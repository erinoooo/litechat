package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Manager keeps one live session per configured server and caches tool lists.
type Manager struct {
	AllowStdio bool

	mu    sync.Mutex
	conns map[string]*entry
}

type entry struct {
	cfgKey  string
	client  *Client
	tools   []Tool
	err     string
	at      time.Time
	connect sync.Mutex
}

func NewManager(allowStdio bool) *Manager {
	return &Manager{AllowStdio: allowStdio, conns: map[string]*entry{}}
}

func cfgKey(c Config) string {
	b, _ := json.Marshal(c)
	return string(b)
}

func (m *Manager) entryFor(cfg Config) *entry {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := cfgKey(cfg)
	e := m.conns[cfg.ID]
	if e != nil && e.cfgKey != key {
		if e.client != nil {
			go e.client.Close()
		}
		e = nil
	}
	if e == nil {
		e = &entry{cfgKey: key}
		m.conns[cfg.ID] = e
	}
	return e
}

// Forget closes the session for a server (after it's edited or deleted).
func (m *Manager) Forget(id string) {
	m.mu.Lock()
	e := m.conns[id]
	delete(m.conns, id)
	m.mu.Unlock()
	if e != nil && e.client != nil {
		go e.client.Close()
	}
}

// client returns a connected client, reconnecting if needed.
func (m *Manager) client(ctx context.Context, cfg Config, refresh bool) (*entry, error) {
	e := m.entryFor(cfg)
	e.connect.Lock()
	defer e.connect.Unlock()
	if e.client != nil && !refresh && time.Since(e.at) < 10*time.Minute {
		return e, nil
	}
	if e.client != nil {
		e.client.Close()
		e.client = nil
	}
	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	c, err := Connect(cctx, cfg, m.AllowStdio)
	if err == nil {
		var tools []Tool
		tools, err = c.ListTools(cctx)
		if err == nil {
			e.client, e.tools, e.err, e.at = c, tools, "", time.Now()
			return e, nil
		}
		c.Close()
	}
	e.err = err.Error()
	return e, err
}

type ServerStatus struct {
	Tools []Tool `json:"tools"`
	Error string `json:"error,omitempty"`
}

// Status connects (if needed) and reports each server's tools or error.
func (m *Manager) Status(ctx context.Context, cfgs []Config, refresh bool) map[string]ServerStatus {
	out := make(map[string]ServerStatus, len(cfgs))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, cfg := range cfgs {
		if !cfg.Enabled {
			continue
		}
		wg.Add(1)
		go func(cfg Config) {
			defer wg.Done()
			e, err := m.client(ctx, cfg, refresh)
			st := ServerStatus{Tools: []Tool{}}
			if err != nil {
				st.Error = err.Error()
			} else {
				st.Tools = e.tools
			}
			mu.Lock()
			out[cfg.ID] = st
			mu.Unlock()
		}(cfg)
	}
	wg.Wait()
	return out
}

// Bound is a tool exposed to the model under a unique, provider-safe name.
type Bound struct {
	Name     string // what the model sees, e.g. "github__search_issues"
	Server   string // server display name
	ServerID string
	Tool     Tool
	cfg      Config
}

var unsafeName = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

func slug(s string) string {
	s = strings.Trim(unsafeName.ReplaceAllString(strings.ToLower(s), "_"), "_")
	if s == "" {
		s = "mcp"
	}
	return s
}

// Tools gathers the tools of all enabled servers. Errors for individual
// servers are returned alongside whatever tools did load.
func (m *Manager) Tools(ctx context.Context, cfgs []Config) ([]Bound, []string) {
	var out []Bound
	var errs []string
	seen := map[string]bool{}
	status := m.Status(ctx, cfgs, false)
	for _, cfg := range cfgs {
		st, ok := status[cfg.ID]
		if !ok {
			continue
		}
		if st.Error != "" {
			errs = append(errs, fmt.Sprintf("%s: %s", cfg.Name, st.Error))
			continue
		}
		for _, t := range st.Tools {
			name := slug(cfg.Name) + "__" + slug(t.Name)
			if len(name) > 64 {
				name = name[:64]
			}
			for i := 2; seen[name]; i++ {
				name = fmt.Sprintf("%s_%d", name[:min(len(name), 60)], i)
			}
			seen[name] = true
			out = append(out, Bound{Name: name, Server: cfg.Name, ServerID: cfg.ID, Tool: t, cfg: cfg})
		}
	}
	return out, errs
}

// Call runs a bound tool, reconnecting once if the session went away.
func (m *Manager) Call(ctx context.Context, b Bound, args json.RawMessage) (string, bool, error) {
	e, err := m.client(ctx, b.cfg, false)
	if err != nil {
		return "", true, err
	}
	text, isErr, err := e.client.CallTool(ctx, b.Tool.Name, args)
	if err != nil && ctx.Err() == nil {
		if e, err2 := m.client(ctx, b.cfg, true); err2 == nil {
			return e.client.CallTool(ctx, b.Tool.Name, args)
		}
	}
	return text, isErr, err
}

func (m *Manager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, e := range m.conns {
		if e.client != nil {
			e.client.Close()
		}
		delete(m.conns, id)
	}
}
