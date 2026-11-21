package server

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"litechat/internal/llm"
)

// indexer embeds chat exchanges in the background for meaning-based search.
// It is off until an embedding model is chosen in settings.
type indexer struct {
	s    *Server
	kick func()

	mu      sync.Mutex
	running bool
	lastErr string
}

func newIndexer(s *Server) *indexer {
	ix := &indexer{s: s}
	ch := make(chan struct{}, 1)
	ix.kick = func() {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
	go func() {
		t := time.NewTicker(15 * time.Minute)
		defer t.Stop()
		for {
			select {
			case <-ch:
				time.Sleep(3 * time.Second) // let titles land and bursts settle
			case <-t.C:
			}
			ix.run()
		}
	}()
	ix.kick() // index anything left over from a previous run
	return ix
}

// embedder resolves the configured "<provider_id>/<model>" key.
func (s *Server) embedder() (llm.Embedder, string, string, error) {
	_, p := s.loadPrefs()
	if !p.SemanticSearch || p.EmbeddingModel == "" {
		return nil, "", "", errSemanticOff
	}
	c, model, err := s.clientFor(p.EmbeddingModel)
	if err != nil {
		return nil, "", "", err
	}
	e, ok := c.(llm.Embedder)
	if !ok {
		return nil, "", "", llm.ErrNoEmbeddings
	}
	return e, model, p.EmbeddingModel, nil
}

const errSemanticOff = constErr("semantic search is off; pick an embedding model in Settings → General")

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	// don't cut a UTF-8 sequence in half
	for n > 0 && n < len(s) && s[n]&0xC0 == 0x80 {
		n--
	}
	return s[:n]
}

func embedText(title, question, answer string) string {
	question = fileRe.ReplaceAllStringFunc(question, func(f string) string {
		if i := strings.Index(f, `">`); i > 0 {
			return f[:i+2] + "…</file> "
		}
		return ""
	})
	return "Chat: " + title + "\n\nUser: " + clip(strings.TrimSpace(question), 1200) +
		"\n\nAssistant: " + clip(historyText(answer), 1800)
}

func (ix *indexer) run() {
	ix.mu.Lock()
	if ix.running {
		ix.mu.Unlock()
		return
	}
	ix.running = true
	ix.mu.Unlock()
	defer func() {
		ix.mu.Lock()
		ix.running = false
		ix.mu.Unlock()
	}()

	emb, model, key, err := ix.s.embedder()
	if err != nil {
		ix.setErr("")
		return
	}
	if err := ix.s.db.DropEmbeddingsExcept(key); err != nil {
		ix.setErr(err.Error())
		return
	}
	for {
		batch, err := ix.s.db.PendingEmbeds(key, ix.s.hub.active(), 32)
		if err != nil {
			ix.setErr(err.Error())
			return
		}
		if len(batch) == 0 {
			ix.setErr("")
			return
		}
		texts := make([]string, len(batch))
		for i, p := range batch {
			texts[i] = embedText(p.Title, p.Question, p.Answer)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		vecs, err := emb.Embed(ctx, model, texts)
		cancel()
		if err != nil {
			log.Printf("semantic index: %v", err)
			ix.setErr(err.Error())
			return
		}
		for i, p := range batch {
			if err := ix.s.db.SaveEmbedding(p.MessageID, p.ChatID, key, vecs[i]); err != nil {
				ix.setErr(err.Error())
				return
			}
		}
	}
}

func (ix *indexer) setErr(e string) {
	ix.mu.Lock()
	ix.lastErr = e
	ix.mu.Unlock()
}

func (s *Server) semanticStatus(w http.ResponseWriter, r *http.Request) {
	_, p := s.loadPrefs()
	out := map[string]any{"enabled": p.SemanticSearch && p.EmbeddingModel != "", "model": p.EmbeddingModel}
	if p.EmbeddingModel != "" {
		indexed, total, err := s.db.EmbedCounts(p.EmbeddingModel)
		if err != nil {
			dbErr(w, err)
			return
		}
		out["indexed"], out["total"] = indexed, total
	}
	s.indexer.mu.Lock()
	out["running"], out["error"] = s.indexer.running, s.indexer.lastErr
	s.indexer.mu.Unlock()
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) semanticReindex(w http.ResponseWriter, r *http.Request) {
	s.indexer.kick()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) semanticSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	emb, model, key, err := s.embedder()
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, errSemanticOff) {
			status = http.StatusConflict
		}
		httpErr(w, status, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	vecs, err := emb.Embed(ctx, model, []string{q})
	if err != nil {
		httpErr(w, http.StatusBadGateway, "embedding failed: "+err.Error())
		return
	}
	hits, err := s.db.SemanticSearch(key, vecs[0], 12)
	if err != nil {
		dbErr(w, err)
		return
	}
	// drop the long tail of weak matches
	if len(hits) > 0 {
		cut := hits[0].Score * 0.7
		n := 0
		for _, h := range hits {
			if h.Score >= cut {
				hits[n] = h
				n++
			}
		}
		hits = hits[:n]
	}
	for i := range hits {
		hits[i].Snippet = clip(strings.Join(strings.Fields(fileRe.ReplaceAllString(hits[i].Snippet, "")), " "), 180)
	}
	writeJSON(w, http.StatusOK, hits)
}
