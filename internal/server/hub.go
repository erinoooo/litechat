package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
)

// A generation is one assistant reply being produced in the background.
// It outlives the HTTP request that started it: closing the tab doesn't stop
// it, and any client can re-attach to watch the rest of it stream.
type generation struct {
	chatID, msgID string
	cancel        context.CancelFunc

	mu      sync.Mutex
	content strings.Builder
	tool    map[string]any // tool currently running, if any
	subs    map[chan event]struct{}
	done    bool
	final   []event // done (+ title) events, replayed to late subscribers
}

type event struct {
	name string
	data any
}

type hub struct {
	mu     sync.Mutex
	byMsg  map[string]*generation
	byChat map[string]*generation
}

func newHub() *hub {
	return &hub{byMsg: map[string]*generation{}, byChat: map[string]*generation{}}
}

func (h *hub) start(chatID, msgID string, cancel context.CancelFunc) *generation {
	g := &generation{chatID: chatID, msgID: msgID, cancel: cancel, subs: map[chan event]struct{}{}}
	h.mu.Lock()
	h.byMsg[msgID] = g
	h.byChat[chatID] = g
	h.mu.Unlock()
	return g
}

func (h *hub) finish(g *generation) {
	h.mu.Lock()
	delete(h.byMsg, g.msgID)
	if h.byChat[g.chatID] == g {
		delete(h.byChat, g.chatID)
	}
	h.mu.Unlock()
}

func (h *hub) forChat(chatID string) *generation {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.byChat[chatID]
}

// active returns the message ids currently being generated.
func (h *hub) active() map[string]bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make(map[string]bool, len(h.byMsg))
	for id := range h.byMsg {
		out[id] = true
	}
	return out
}

func (g *generation) publish(e event) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if e.name == "delta" {
		g.content.WriteString(e.data.(map[string]string)["t"])
	}
	switch e.name {
	case "tool_start":
		g.tool = e.data.(map[string]any)
	case "delta":
		// a tool block arriving in the text means the tool finished
		if strings.Contains(e.data.(map[string]string)["t"], "<tool ") {
			g.tool = nil
		}
	case "done", "title":
		g.final = append(g.final, e)
		if e.name == "done" {
			g.tool = nil
		}
	}
	for ch := range g.subs {
		select {
		case ch <- e:
		default:
			// slow client: drop it; it will see the stream end and re-attach
			close(ch)
			delete(g.subs, ch)
		}
	}
}

// close marks the generation finished and ends every subscription.
func (g *generation) close() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.done = true
	for ch := range g.subs {
		close(ch)
		delete(g.subs, ch)
	}
}

// subscribe returns a snapshot of the reply so far plus a channel of later events.
func (g *generation) subscribe() (snapshot string, tool map[string]any, ch chan event, final []event) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.done {
		return g.content.String(), nil, nil, append([]event(nil), g.final...)
	}
	ch = make(chan event, 1024)
	g.subs[ch] = struct{}{}
	return g.content.String(), g.tool, ch, nil
}

func (g *generation) unsubscribe(ch chan event) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, ok := g.subs[ch]; ok {
		delete(g.subs, ch)
		close(ch)
	}
}

func (g *generation) text() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.content.String()
}

// ---------- SSE plumbing ----------

type sse struct {
	w http.ResponseWriter
	f http.Flusher
}

func startSSE(w http.ResponseWriter) (*sse, bool) {
	f, ok := w.(http.Flusher)
	if !ok {
		httpErr(w, http.StatusInternalServerError, "streaming unsupported")
		return nil, false
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering
	w.WriteHeader(http.StatusOK)
	return &sse{w, f}, true
}

func (s *sse) send(event string, v any) {
	b, _ := json.Marshal(v)
	fmt.Fprintf(s.w, "event: %s\ndata: %s\n\n", event, b)
	s.f.Flush()
}

// follow streams a generation's events to the client until it finishes or
// the client leaves. With sync=true it first sends the reply so far.
func follow(r *http.Request, out *sse, g *generation, sync bool) {
	snap, tool, ch, final := g.subscribe()
	if sync {
		out.send("sync", map[string]any{"message_id": g.msgID, "content": snap, "tool": tool})
	}
	if ch == nil { // already finished
		for _, e := range final {
			out.send(e.name, e.data)
		}
		return
	}
	defer g.unsubscribe(ch)
	for {
		select {
		case e, ok := <-ch:
			if !ok {
				return
			}
			out.send(e.name, e.data)
		case <-r.Context().Done():
			return
		}
	}
}
