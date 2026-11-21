package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"litechat/internal/db"
	"litechat/internal/llm"
)

type sendBody struct {
	ParentID   string   `json:"parent_id"`  // message to attach to ("" = root)
	Content    string   `json:"content"`
	Images     []string `json:"images"`
	Model      string   `json:"model"`      // "<provider_id>/<model_id>"
	Regenerate bool     `json:"regenerate"` // parent_id is a user message; reply to it again
}

type sse struct {
	w http.ResponseWriter
	f http.Flusher
}

func (s *sse) send(event string, v any) {
	b, _ := json.Marshal(v)
	fmt.Fprintf(s.w, "event: %s\ndata: %s\n\n", event, b)
	s.f.Flush()
}

// sendMessage stores the user's message (unless regenerating), then streams
// the assistant's reply back as server-sent events:
//
//	meta  {user, assistant}  the stored message rows
//	delta {t}                a chunk of reply text
//	done  {content, error?}  final text; the reply is saved
//	title {title}            new chat title (first exchange only)
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
		history = append(history, llm.Message{Role: m.Role, Content: stripThink(m.Content), Images: m.ImageList})
	}

	// 4. stream
	flusher, ok := w.(http.Flusher)
	if !ok {
		httpErr(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering
	w.WriteHeader(http.StatusOK)
	out := &sse{w, flusher}
	out.send("meta", map[string]any{"user": user, "assistant": asst})

	var sb strings.Builder
	streamErr := client.Stream(r.Context(), llm.Request{
		Model: modelID, System: chat.SystemPrompt, Messages: history,
	}, func(t string) {
		sb.WriteString(t)
		out.send("delta", map[string]string{"t": t})
	})

	content := sb.String()
	errText := ""
	if streamErr != nil && !errors.Is(streamErr, context.Canceled) {
		errText = streamErr.Error()
		log.Printf("stream error (%s): %v", body.Model, streamErr)
	}
	// save even if the client went away, so a stopped reply keeps its partial text
	if err := s.db.FinishMessage(asst.ID, content, errText); err != nil {
		log.Printf("save reply: %v", err)
	}
	if r.Context().Err() != nil {
		return
	}
	done := map[string]string{"content": content}
	if errText != "" {
		done["error"] = errText
	}
	out.send("done", done)

	// 5. title the chat after its first exchange
	if chat.Title == "New chat" && errText == "" && content != "" {
		_, p := s.loadPrefs()
		title := ""
		if p.AutoTitle == nil || *p.AutoTitle {
			title = s.generateTitle(r.Context(), client, modelID, user.Content, content)
		}
		if title == "" {
			title = fallbackTitle(user.Content)
		}
		if title != "" {
			_ = s.db.UpdateChat(chatID, map[string]any{"title": title}, false)
			out.send("title", map[string]string{"title": title})
		}
	}
}

var thinkRe = regexp.MustCompile(`(?s)<think>.*?(</think>|$)`)

func stripThink(s string) string { return strings.TrimSpace(thinkRe.ReplaceAllString(s, "")) }

func (s *Server) generateTitle(ctx context.Context, c llm.Client, model, question, answer string) string {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if len(answer) > 1500 {
		answer = answer[:1500]
	}
	out, err := llm.Complete(ctx, c, llm.Request{
		Model: model,
		Messages: []llm.Message{{Role: "user", Content: "Write a short title (3-6 words) for this conversation. " +
			"Reply with the title only: no quotes, no punctuation at the end.\n\nUser: " + question +
			"\n\nAssistant: " + stripThink(answer)}},
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

func fallbackTitle(q string) string {
	q = strings.Join(strings.Fields(q), " ")
	r := []rune(q)
	if len(r) > 50 {
		return string(r[:50]) + "…"
	}
	return q
}
