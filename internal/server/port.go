package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"litechat/internal/db"
)

// ---------- export ----------

type exportFile struct {
	Format     string          `json:"format"`
	Version    int             `json:"version"`
	ExportedAt int64           `json:"exported_at"`
	Chats      []db.ChatExport `json:"chats"`
}

func (s *Server) exportAll(w http.ResponseWriter, r *http.Request) {
	chats, err := s.db.ExportAll()
	if err != nil {
		dbErr(w, err)
		return
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="litechat-export-%s.json"`, time.Now().Format("2006-01-02")))
	writeJSON(w, http.StatusOK, exportFile{Format: "litechat", Version: 1, ExportedAt: time.Now().UnixMilli(), Chats: chats})
}

// ---------- import ----------

// node is a format-neutral message used while importing.
type node struct {
	id, parent, role, content, model string
	images                           []string
	created                          int64 // ms
}

type importedChat struct {
	title, system, model string
	pinned               bool
	created, updated     int64
	current              string
	nodes                []node
}

func (s *Server) importChats(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 512<<20)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		httpErr(w, http.StatusRequestEntityTooLarge, "file too large (max 512 MB)")
		return
	}
	chats, format, err := parseImport(raw)
	if err != nil {
		httpErr(w, http.StatusBadRequest, err.Error())
		return
	}
	n := 0
	for _, ic := range chats {
		c, msgs := buildChat(ic)
		if len(msgs) == 0 {
			continue
		}
		if err := s.db.ImportChat(c, msgs); err != nil {
			dbErr(w, err)
			return
		}
		n++
	}
	s.indexer.kick()
	writeJSON(w, http.StatusOK, map[string]any{"imported": n, "format": format})
}

func parseImport(raw []byte) ([]importedChat, string, error) {
	var probe any
	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil, "", fmt.Errorf("not valid JSON: %v", err)
	}
	switch v := probe.(type) {
	case map[string]any:
		if _, ok := v["chats"]; ok {
			return parseLiteChat(raw)
		}
		if _, ok := v["mapping"]; ok { // a single ChatGPT conversation
			return parseChatGPT([]byte("[" + string(raw) + "]"))
		}
	case []any:
		if len(v) == 0 {
			return nil, "", fmt.Errorf("the file has no chats")
		}
		first, _ := v[0].(map[string]any)
		if _, ok := first["mapping"]; ok {
			return parseChatGPT(raw)
		}
		if c, ok := first["chat"].(map[string]any); ok {
			if _, ok := c["history"]; ok {
				return parseOpenWebUI(raw)
			}
		}
	}
	return nil, "", fmt.Errorf("unrecognized format: expected a liteChat, ChatGPT (conversations.json) or Open WebUI export")
}

func parseLiteChat(raw []byte) ([]importedChat, string, error) {
	var f exportFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, "", err
	}
	var out []importedChat
	for _, ce := range f.Chats {
		ic := importedChat{title: ce.Chat.Title, system: ce.Chat.SystemPrompt, model: ce.Chat.Model, pinned: ce.Chat.Pinned,
			created: ce.Chat.CreatedAt, updated: ce.Chat.UpdatedAt, current: ce.Chat.LeafID}
		for _, m := range ce.Messages {
			ic.nodes = append(ic.nodes, node{id: m.ID, parent: m.ParentID, role: m.Role, content: m.Content,
				model: m.Model, images: m.ImageList, created: m.CreatedAt})
		}
		out = append(out, ic)
	}
	return out, "litechat", nil
}

func parseChatGPT(raw []byte) ([]importedChat, string, error) {
	var convs []struct {
		Title       string  `json:"title"`
		CreateTime  float64 `json:"create_time"`
		UpdateTime  float64 `json:"update_time"`
		CurrentNode string  `json:"current_node"`
		Mapping     map[string]struct {
			ID      string `json:"id"`
			Parent  string `json:"parent"`
			Message *struct {
				Author struct {
					Role string `json:"role"`
				} `json:"author"`
				CreateTime float64 `json:"create_time"`
				Content    struct {
					ContentType string `json:"content_type"`
					Parts       []any  `json:"parts"`
					Text        string `json:"text"`
				} `json:"content"`
				Metadata struct {
					ModelSlug string `json:"model_slug"`
				} `json:"metadata"`
			} `json:"message"`
		} `json:"mapping"`
	}
	if err := json.Unmarshal(raw, &convs); err != nil {
		return nil, "", err
	}
	var out []importedChat
	for _, cv := range convs {
		ic := importedChat{title: cv.Title, created: int64(cv.CreateTime * 1000), updated: int64(cv.UpdateTime * 1000), current: cv.CurrentNode}
		for id, n := range cv.Mapping {
			nd := node{id: id, parent: n.Parent}
			if m := n.Message; m != nil {
				nd.role = m.Author.Role
				nd.created = int64(m.CreateTime * 1000)
				nd.model = m.Metadata.ModelSlug
				var parts []string
				for _, p := range m.Content.Parts {
					if s, ok := p.(string); ok && s != "" {
						parts = append(parts, s)
					}
				}
				nd.content = strings.Join(parts, "\n\n")
				if nd.content == "" && m.Content.ContentType == "code" {
					nd.content = "```\n" + m.Content.Text + "\n```"
				}
			}
			ic.nodes = append(ic.nodes, nd)
		}
		out = append(out, ic)
	}
	return out, "chatgpt", nil
}

func parseOpenWebUI(raw []byte) ([]importedChat, string, error) {
	var items []struct {
		Title     string `json:"title"`
		CreatedAt int64  `json:"created_at"`
		UpdatedAt int64  `json:"updated_at"`
		Pinned    bool   `json:"pinned"`
		Chat      struct {
			Title   string `json:"title"`
			System  string `json:"system"`
			History struct {
				CurrentID string `json:"currentId"`
				Messages  map[string]struct {
					ID        string `json:"id"`
					ParentID  string `json:"parentId"`
					Role      string `json:"role"`
					Content   any    `json:"content"`
					Timestamp int64  `json:"timestamp"`
					Model     string `json:"model"`
				} `json:"messages"`
			} `json:"history"`
		} `json:"chat"`
	}
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, "", err
	}
	secs := func(t int64) int64 { // Open WebUI uses seconds (sometimes ns)
		switch {
		case t > 1e17:
			return t / 1e6
		case t > 1e12:
			return t
		default:
			return t * 1000
		}
	}
	var out []importedChat
	for _, it := range items {
		title := it.Title
		if title == "" {
			title = it.Chat.Title
		}
		ic := importedChat{title: title, system: it.Chat.System, pinned: it.Pinned,
			created: secs(it.CreatedAt), updated: secs(it.UpdatedAt), current: it.Chat.History.CurrentID}
		for id, m := range it.Chat.History.Messages {
			content, _ := m.Content.(string)
			ic.nodes = append(ic.nodes, node{id: id, parent: m.ParentID, role: m.Role, content: content,
				model: m.Model, created: secs(m.Timestamp)})
		}
		out = append(out, ic)
	}
	return out, "open-webui", nil
}

// buildChat assigns fresh ids, drops non-chat nodes (system, tool, empty)
// while re-attaching their children, and picks the active branch.
func buildChat(ic importedChat) (db.Chat, []db.Message) {
	byID := map[string]node{}
	for _, n := range ic.nodes {
		byID[n.id] = n
	}
	keep := func(n node) bool {
		return (n.role == "user" || n.role == "assistant") && strings.TrimSpace(n.content) != ""
	}
	// nearest kept ancestor-or-self
	var resolve func(id string, depth int) string
	resolve = func(id string, depth int) string {
		n, ok := byID[id]
		if !ok || depth > 10000 {
			return ""
		}
		if keep(n) {
			return id
		}
		return resolve(n.parent, depth+1)
	}

	var kept []node
	for _, n := range ic.nodes {
		if keep(n) {
			kept = append(kept, n)
		}
	}
	sort.SliceStable(kept, func(i, j int) bool { return kept[i].created < kept[j].created })

	now := time.Now().UnixMilli()
	newID := map[string]string{}
	for _, n := range kept {
		newID[n.id] = db.NewID()
	}
	msgs := make([]db.Message, 0, len(kept))
	var latest int64
	for i, n := range kept {
		created := n.created
		if created <= 0 {
			created = ic.created + int64(i)
		}
		if created <= 0 {
			created = now + int64(i)
		}
		latest = max(latest, created)
		imgs := n.images
		if imgs == nil {
			imgs = []string{}
		}
		msgs = append(msgs, db.Message{
			ID: newID[n.id], ParentID: newID[resolve(n.parent, 0)], Role: n.role, Content: n.content,
			ImageList: imgs, Model: n.model, CreatedAt: created,
		})
	}

	c := db.Chat{ID: db.NewID(), Title: strings.TrimSpace(ic.title), SystemPrompt: ic.system, Model: ic.model, Pinned: ic.pinned,
		CreatedAt: ic.created, UpdatedAt: ic.updated}
	if c.Title == "" {
		c.Title = "Imported chat"
	}
	if c.CreatedAt <= 0 {
		c.CreatedAt = now
	}
	if c.UpdatedAt <= 0 {
		c.UpdatedAt = max(latest, c.CreatedAt)
	}
	c.LeafID = newID[resolve(ic.current, 0)]
	if c.LeafID == "" && len(msgs) > 0 {
		c.LeafID = msgs[len(msgs)-1].ID
	}
	// if the leaf isn't actually a leaf, follow the newest child down
	children := map[string][]string{}
	for _, m := range msgs {
		children[m.ParentID] = append(children[m.ParentID], m.ID)
	}
	for kids := children[c.LeafID]; len(kids) > 0; kids = children[c.LeafID] {
		c.LeafID = kids[len(kids)-1]
	}
	return c, msgs
}
