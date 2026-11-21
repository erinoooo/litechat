package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"litechat/internal/db"
	"litechat/internal/llm"
	"litechat/internal/mcp"
)

type sendBody struct {
	ParentID   string   `json:"parent_id"` // message to attach to ("" = root)
	Content    string   `json:"content"`
	Images     []string `json:"images"`
	Model      string   `json:"model"`      // "<provider_id>/<model_id>"
	Regenerate bool     `json:"regenerate"` // parent_id is a user message; reply to it again
	Tools      bool     `json:"tools"`      // offer MCP tools to the model
}

const maxToolRounds = 10

// sendMessage stores the user's message (unless regenerating), starts the
// reply in the background, and streams it back as server-sent events:
//
//	meta       {user, assistant}  the stored message rows
//	delta      {t}                a chunk of reply text (tool results arrive as <tool> blocks)
//	tool_start {name, server, args}
//	done       {content, error?}  final text; the reply is saved
//	title      {title}            new chat title (first exchange only)
func (s *Server) sendMessage(w http.ResponseWriter, r *http.Request) {
	chatID := r.PathValue("id")
	var body sendBody
	if !readJSON(w, r, &body) {
		return
	}
	chat, err := s.db.GetChat(chatID)
	if err != nil {
		dbErr(w, err)
		return
	}
	if g := s.hub.forChat(chatID); g != nil {
		httpErr(w, http.StatusConflict, "a reply is already being generated in this chat")
		return
	}
	all, err := s.db.ListMessages(chatID)
	if err != nil {
		dbErr(w, err)
		return
	}
	byID := map[string]db.Message{}
	for _, m := range all {
		byID[m.ID] = m
	}
	if body.ParentID != "" {
		if _, ok := byID[body.ParentID]; !ok {
			httpErr(w, http.StatusBadRequest, "parent message not found")
			return
		}
	}
	client, modelID, err := s.clientFor(body.Model)
	if err != nil {
		httpErr(w, http.StatusBadRequest, err.Error())
		return
	}

	// 1. the user message
	var user db.Message
	if body.Regenerate {
		user = byID[body.ParentID]
		if user.Role != "user" {
			httpErr(w, http.StatusBadRequest, "can only regenerate a reply to a user message")
			return
		}
	} else {
		if strings.TrimSpace(body.Content) == "" && len(body.Images) == 0 {
			httpErr(w, http.StatusBadRequest, "message is empty")
			return
		}
		user = db.Message{ChatID: chatID, ParentID: body.ParentID, Role: "user", Content: body.Content, ImageList: body.Images}
		if err := s.db.InsertMessage(&user); err != nil {
			dbErr(w, err)
			return
		}
		all = append(all, user)
	}

	// 2. placeholder for the reply
	asst := db.Message{ChatID: chatID, ParentID: user.ID, Role: "assistant", Model: body.Model, ImageList: []string{}}
	if err := s.db.InsertMessage(&asst); err != nil {
		dbErr(w, err)
		return
	}
	_ = s.db.UpdateChat(chatID, map[string]any{"leaf_id": asst.ID, "model": body.Model}, true)

	// 3. history = path from root to the user message
	var history []llm.Message
	for _, m := range db.Path(all, user.ID) {
		if m.Role == "assistant" && m.Content == "" {
			continue // failed or empty reply
		}
		history = append(history, llm.Message{Role: m.Role, Content: historyText(m.Content), Images: m.ImageList})
	}

	// 4. run in the background, independent of this request
	ctx, cancel := context.WithCancel(context.Background())
	g := s.hub.start(chatID, asst.ID, cancel)
	go s.generate(ctx, g, chat, client, modelID, history, body.Tools, user.Content)

	out, ok := startSSE(w)
	if !ok {
		return
	}
	out.send("meta", map[string]any{"user": user, "assistant": asst})
	follow(r, out, g, false)
}

// attach lets a client pick up a reply that is still being generated.
func (s *Server) attach(w http.ResponseWriter, r *http.Request) {
	g := s.hub.forChat(r.PathValue("id"))
	if g == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	out, ok := startSSE(w)
	if !ok {
		return
	}
	follow(r, out, g, true)
}

func (s *Server) stopGeneration(w http.ResponseWriter, r *http.Request) {
	if g := s.hub.forChat(r.PathValue("id")); g != nil {
		g.cancel()
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) generate(ctx context.Context, g *generation, chat db.Chat, client llm.Client, modelID string,
	history []llm.Message, useTools bool, question string) {
	defer s.hub.finish(g)
	defer g.close()
	defer g.cancel()

	emit := func(t string) { g.publish(event{"delta", map[string]string{"t": t}}) }

	// save progress periodically so a reload or restart shows partial text
	saveDone := make(chan struct{})
	go func() {
		t := time.NewTicker(2 * time.Second)
		defer t.Stop()
		last := ""
		for {
			select {
			case <-t.C:
				if c := g.text(); c != last {
					_ = s.db.SaveMessageContent(g.msgID, c)
					last = c
				}
			case <-saveDone:
				return
			}
		}
	}()

	var tools []mcp.Bound
	var llmTools []llm.Tool
	if useTools {
		cfgs := s.mcpConfigs()
		var errs []string
		tools, errs = s.mcp.Tools(ctx, cfgs)
		for _, e := range errs {
			log.Printf("mcp: %s", e)
		}
		for _, t := range tools {
			desc := t.Tool.Description
			if desc == "" {
				desc = t.Tool.Name
			}
			llmTools = append(llmTools, llm.Tool{Name: t.Name, Description: fmt.Sprintf("[%s] %s", t.Server, desc), Schema: t.Tool.InputSchema})
		}
	}

	req := llm.Request{Model: modelID, System: chat.SystemPrompt, Messages: history, Tools: llmTools}
	var streamErr error
	for round := 0; ; round++ {
		var roundText strings.Builder
		res, err := client.Stream(ctx, req, func(t string) {
			roundText.WriteString(t)
			emit(t)
		})
		if err != nil {
			streamErr = err
			break
		}
		if len(res.ToolCalls) == 0 {
			break
		}
		if round >= maxToolRounds {
			emit("\n\n_Stopped: too many tool calls in a row._")
			break
		}
		req.Messages = append(req.Messages, llm.Message{Role: "assistant", Content: stripThink(roundText.String()), ToolCalls: res.ToolCalls})
		for _, call := range res.ToolCalls {
			result, _ := s.runTool(ctx, g, tools, call)
			req.Messages = append(req.Messages, llm.Message{Role: "tool", ToolCallID: call.ID, Content: result})
			if ctx.Err() != nil {
				break
			}
		}
		if ctx.Err() != nil {
			streamErr = ctx.Err()
			break
		}
		if roundText.Len() > 0 || len(res.ToolCalls) > 0 {
			emit("\n\n")
		}
	}
	close(saveDone)

	content := strings.TrimRight(g.text(), "\n")
	errText := ""
	if streamErr != nil && !errors.Is(streamErr, context.Canceled) {
		errText = streamErr.Error()
		log.Printf("stream error (%s): %v", modelID, streamErr)
	}
	if err := s.db.FinishMessage(g.msgID, content, errText); err != nil {
		log.Printf("save reply: %v", err)
	}
	done := map[string]string{"content": content}
	if errText != "" {
		done["error"] = errText
	}
	g.publish(event{"done", done})
	s.indexer.kick()

	// title the chat after its first exchange
	if chat.Title == "New chat" && errText == "" && content != "" {
		_, p := s.loadPrefs()
		title := ""
		if p.AutoTitle == nil || *p.AutoTitle {
			tctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			title = s.generateTitle(tctx, client, modelID, question, content)
			cancel()
		}
		if title == "" {
			title = fallbackTitle(question)
		}
		if title != "" {
			_ = s.db.UpdateChat(chat.ID, map[string]any{"title": title}, false)
			g.publish(event{"title", map[string]string{"title": title}})
		}
	}
}

// runTool executes one tool call and appends a <tool> block to the reply.
func (s *Server) runTool(ctx context.Context, g *generation, tools []mcp.Bound, call llm.ToolCall) (string, bool) {
	var bound *mcp.Bound
	for i := range tools {
		if tools[i].Name == call.Name {
			bound = &tools[i]
			break
		}
	}
	args := json.RawMessage(call.Args)
	if !json.Valid(args) {
		args = json.RawMessage("{}")
	}
	server, label := "", call.Name
	if bound != nil {
		server, label = bound.Server, bound.Tool.Name
	}
	g.publish(event{"tool_start", map[string]any{"name": label, "server": server, "args": args}})

	var result string
	isErr := false
	if bound == nil {
		result, isErr = "Error: unknown tool "+call.Name, true
	} else {
		tctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
		text, toolErr, err := s.mcp.Call(tctx, *bound, args)
		cancel()
		switch {
		case err != nil:
			result, isErr = "Error: "+err.Error(), true
		case toolErr:
			result, isErr = text, true
			if result == "" {
				result = "Error"
			}
		default:
			result = text
		}
	}

	status := "ok"
	if isErr {
		status = "error"
	}
	// json.Marshal escapes <, > and & so the block can't be closed early
	payload, _ := json.Marshal(map[string]any{"args": args, "result": result})
	block := fmt.Sprintf("<tool name=\"%s\" server=\"%s\" status=\"%s\">\n%s\n</tool>",
		html.EscapeString(label), html.EscapeString(server), status, payload)
	g.publish(event{"delta", map[string]string{"t": "\n\n" + block}})
	return result, isErr
}

var (
	thinkRe = regexp.MustCompile(`(?s)<think>.*?(</think>|$)`)
	toolRe  = regexp.MustCompile(`(?s)<tool name="([^"]*)"[^>]*>\n(.*?)\n</tool>`)
)

func stripThink(s string) string { return strings.TrimSpace(thinkRe.ReplaceAllString(s, "")) }

// historyText prepares a stored reply for use as context in later turns:
// thinking is dropped and tool blocks become short plain-text notes.
func historyText(s string) string {
	s = stripThink(s)
	return toolRe.ReplaceAllStringFunc(s, func(block string) string {
		m := toolRe.FindStringSubmatch(block)
		var p struct {
			Args   json.RawMessage `json:"args"`
			Result string          `json:"result"`
		}
		_ = json.Unmarshal([]byte(m[2]), &p)
		res := p.Result
		if len(res) > 1500 {
			res = res[:1500] + "…"
		}
		return fmt.Sprintf("[Called tool %s with %s. Result: %s]", html.UnescapeString(m[1]), p.Args, res)
	})
}

func (s *Server) generateTitle(ctx context.Context, c llm.Client, model, question, answer string) string {
	answer = historyText(answer)
	if len(answer) > 1500 {
		answer = answer[:1500]
	}
	if len(question) > 2000 {
		question = question[:2000]
	}
	out, err := llm.Complete(ctx, c, llm.Request{
		Model: model,
		Messages: []llm.Message{{Role: "user", Content: "Write a short title (3-6 words) for this conversation. " +
			"Reply with the title only: no quotes, no punctuation at the end.\n\nUser: " + question +
			"\n\nAssistant: " + answer}},
	})
	if err != nil {
		return ""
	}
	t := strings.TrimSpace(stripThink(out))
	t = strings.Trim(t, "\"'*#` ")
	if i := strings.IndexByte(t, '\n'); i >= 0 {
		t = t[:i]
	}
	if len([]rune(t)) > 80 {
		return ""
	}
	return t
}

var fileRe = regexp.MustCompile(`(?s)<file name="[^"]*">.*?</file>\s*`)

func fallbackTitle(q string) string {
	q = strings.Join(strings.Fields(fileRe.ReplaceAllString(q, "")), " ")
	r := []rune(q)
	if len(r) > 50 {
		return string(r[:50]) + "…"
	}
	return q
}
