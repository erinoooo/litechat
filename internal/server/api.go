package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"litechat/internal/db"
	"litechat/internal/llm"
)

// ---------- providers ----------

func (s *Server) listProviders(w http.ResponseWriter, r *http.Request) {
	ps, err := s.db.ListProviders()
	if err != nil {
		dbErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ps)
}

func (s *Server) saveProvider(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name    string  `json:"name"`
		Kind    string  `json:"kind"`
		BaseURL string  `json:"base_url"`
		APIKey  *string `json:"api_key"` // nil = keep existing key
		Enabled *bool   `json:"enabled"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	p := db.Provider{Enabled: true}
	if id := r.PathValue("id"); id != "" {
		var err error
		if p, err = s.db.GetProvider(id); err != nil {
			dbErr(w, err)
			return
		}
	}
	if body.Name != "" {
		p.Name = strings.TrimSpace(body.Name)
	}
	if body.Kind != "" {
		p.Kind = body.Kind
	}
	if body.BaseURL != "" || p.ID == "" {
		p.BaseURL = strings.TrimRight(strings.TrimSpace(body.BaseURL), "/")
	}
	if body.APIKey != nil {
		p.APIKey = strings.TrimSpace(*body.APIKey)
	}
	if body.Enabled != nil {
		p.Enabled = *body.Enabled
	}

	if p.Kind != "openai" && p.Kind != "anthropic" {
		httpErr(w, http.StatusBadRequest, "kind must be openai or anthropic")
		return
	}
	if p.Kind == "anthropic" && p.BaseURL == "" {
		p.BaseURL = "https://api.anthropic.com"
	}
	if u, err := url.Parse(p.BaseURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		httpErr(w, http.StatusBadRequest, "base URL must be an http(s) URL")
		return
	}
	if p.Name == "" {
		p.Name = p.BaseURL
	}
	if err := s.db.SaveProvider(&p); err != nil {
		dbErr(w, err)
		return
	}
	s.invalidateModels(p.ID)
	p.HasKey = p.APIKey != ""
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) deleteProvider(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.db.DeleteProvider(id); err != nil {
		dbErr(w, err)
		return
	}
	s.invalidateModels(id)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ---------- models ----------

type modelsEntry struct {
	models []llm.Model
	err    string
	at     time.Time
}

type modelOut struct {
	Key          string `json:"key"` // "<provider_id>/<model_id>"
	ID           string `json:"id"`
	Name         string `json:"name"`
	ProviderID   string `json:"provider_id"`
	ProviderName string `json:"provider_name"`
}

func (s *Server) invalidateModels(providerID string) {
	s.modelsMu.Lock()
	delete(s.modelsCache, providerID)
	s.modelsMu.Unlock()
}

func (s *Server) listModels(w http.ResponseWriter, r *http.Request) {
	ps, err := s.db.ListProviders()
	if err != nil {
		dbErr(w, err)
		return
	}
	refresh := r.URL.Query().Get("refresh") == "1"

	results := make([]modelsEntry, len(ps))
	var wg sync.WaitGroup
	for i, p := range ps {
		if !p.Enabled {
			continue
		}
		s.modelsMu.Lock()
		e, ok := s.modelsCache[p.ID]
		s.modelsMu.Unlock()
		if ok && !refresh && time.Since(e.at) < 5*time.Minute {
			results[i] = e
			continue
		}
		wg.Add(1)
		go func(i int, p db.Provider) {
			defer wg.Done()
			e := modelsEntry{at: time.Now()}
			c, err := llm.New(p.Kind, p.BaseURL, p.APIKey)
			if err == nil {
				ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
				e.models, err = c.ListModels(ctx)
				cancel()
			}
			if err != nil {
				e.err = err.Error()
				e.at = time.Now().Add(-4 * time.Minute) // retry errors sooner
			}
			s.modelsMu.Lock()
			s.modelsCache[p.ID] = e
			s.modelsMu.Unlock()
			results[i] = e
		}(i, p)
	}
	wg.Wait()

	out := struct {
		Models []modelOut        `json:"models"`
		Errors map[string]string `json:"errors"`
	}{Models: []modelOut{}, Errors: map[string]string{}}
	for i, p := range ps {
		if !p.Enabled {
			continue
		}
		if results[i].err != "" {
			out.Errors[p.ID] = results[i].err
		}
		for _, m := range results[i].models {
			out.Models = append(out.Models, modelOut{
				Key: p.ID + "/" + m.ID, ID: m.ID, Name: m.Name, ProviderID: p.ID, ProviderName: p.Name,
			})
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// clientFor resolves a "<provider_id>/<model_id>" key.
func (s *Server) clientFor(key string) (llm.Client, string, error) {
	pid, model, ok := strings.Cut(key, "/")
	if !ok || model == "" {
		return nil, "", errBadModel
	}
	p, err := s.db.GetProvider(pid)
	if err != nil {
		return nil, "", errBadModel
	}
	if !p.Enabled {
		return nil, "", errProviderOff
	}
	c, err := llm.New(p.Kind, p.BaseURL, p.APIKey)
	return c, model, err
}

type constErr string

func (e constErr) Error() string { return string(e) }

const (
	errBadModel    = constErr("unknown model; pick one from the model menu")
	errProviderOff = constErr("that model's provider is disabled")
)

// ---------- chats ----------

func (s *Server) listChats(w http.ResponseWriter, r *http.Request) {
	cs, err := s.db.ListChats()
	if err != nil {
		dbErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cs)
}

func (s *Server) createChat(w http.ResponseWriter, r *http.Request) {
	var c db.Chat
	if !readJSON(w, r, &c) {
		return
	}
	c.LeafID, c.Pinned = "", false
	if err := s.db.CreateChat(&c); err != nil {
		dbErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) getChat(w http.ResponseWriter, r *http.Request) {
	c, err := s.db.GetChat(r.PathValue("id"))
	if err != nil {
		dbErr(w, err)
		return
	}
	ms, err := s.db.ListMessages(c.ID)
	if err != nil {
		dbErr(w, err)
		return
	}
	generating := ""
	if g := s.hub.forChat(c.ID); g != nil {
		generating = g.msgID
	}
	writeJSON(w, http.StatusOK, map[string]any{"chat": c, "messages": ms, "generating": generating})
}

func (s *Server) updateChat(w http.ResponseWriter, r *http.Request) {
	var fields map[string]any
	if !readJSON(w, r, &fields) {
		return
	}
	// metadata edits don't reorder the sidebar; only new messages do
	if err := s.db.UpdateChat(r.PathValue("id"), fields, false); err != nil {
		dbErr(w, err)
		return
	}
	c, err := s.db.GetChat(r.PathValue("id"))
	if err != nil {
		dbErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) deleteChat(w http.ResponseWriter, r *http.Request) {
	if g := s.hub.forChat(r.PathValue("id")); g != nil {
		g.cancel()
	}
	if err := s.db.DeleteChat(r.PathValue("id")); err != nil {
		dbErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) search(w http.ResponseWriter, r *http.Request) {
	hits, err := s.db.Search(r.URL.Query().Get("q"))
	if err != nil {
		dbErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, hits)
}

// ---------- presets ----------

func (s *Server) listPresets(w http.ResponseWriter, r *http.Request) {
	ps, err := s.db.ListPresets()
	if err != nil {
		dbErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ps)
}

func (s *Server) savePreset(w http.ResponseWriter, r *http.Request) {
	var p db.Preset
	if !readJSON(w, r, &p) {
		return
	}
	p.ID = r.PathValue("id")
	if strings.TrimSpace(p.Name) == "" {
		httpErr(w, http.StatusBadRequest, "name is required")
		return
	}
	if err := s.db.SavePreset(&p); err != nil {
		dbErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) deletePreset(w http.ResponseWriter, r *http.Request) {
	if err := s.db.DeletePreset(r.PathValue("id")); err != nil {
		dbErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ---------- user preferences (opaque JSON owned by the UI) ----------

type prefs struct {
	DefaultModel   string `json:"default_model"`
	AutoTitle      *bool  `json:"auto_title"`
	SemanticSearch bool   `json:"semantic_search"`
	EmbeddingModel string `json:"embedding_model"` // "<provider_id>/<model>"
}

func (s *Server) loadPrefs() (json.RawMessage, prefs) {
	raw, err := s.db.GetSetting("prefs")
	if err != nil {
		raw = "{}"
	}
	var p prefs
	_ = json.Unmarshal([]byte(raw), &p)
	return json.RawMessage(raw), p
}

func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	raw, _ := s.loadPrefs()
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(raw)
}

func (s *Server) putSettings(w http.ResponseWriter, r *http.Request) {
	var v map[string]any
	if !readJSON(w, r, &v) {
		return
	}
	b, _ := json.Marshal(v)
	if err := s.db.SetSetting("prefs", string(b)); err != nil {
		dbErr(w, err)
		return
	}
	s.indexer.kick() // embedding settings may have changed
	writeJSON(w, http.StatusOK, v)
}
