package server

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"litechat/internal/db"
	"litechat/internal/mcp"
)

const masked = "••••••"

func (s *Server) mcpConfigs() []mcp.Config {
	rows, err := s.db.ListMCPServers()
	if err != nil {
		return nil
	}
	out := make([]mcp.Config, 0, len(rows))
	for _, r := range rows {
		var c mcp.Config
		if json.Unmarshal(r.Config, &c) == nil {
			c.ID = r.ID
			out = append(out, c)
		}
	}
	return out
}

// secrets in headers/env are never sent back to the browser
func maskMap(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		if v != "" {
			v = masked
		}
		out[k] = v
	}
	return out
}

func (s *Server) listMCP(w http.ResponseWriter, r *http.Request) {
	cfgs := s.mcpConfigs()
	for i := range cfgs {
		cfgs[i].Headers = maskMap(cfgs[i].Headers)
		cfgs[i].Env = maskMap(cfgs[i].Env)
	}
	writeJSON(w, http.StatusOK, map[string]any{"servers": cfgs, "stdio_allowed": s.mcp.AllowStdio})
}

func (s *Server) saveMCP(w http.ResponseWriter, r *http.Request) {
	var c mcp.Config
	if !readJSON(w, r, &c) {
		return
	}
	id := r.PathValue("id")
	var old mcp.Config
	if id != "" {
		row, err := s.db.GetMCPServer(id)
		if err != nil {
			dbErr(w, err)
			return
		}
		_ = json.Unmarshal(row.Config, &old)
	}
	// keep previously stored secret values the UI only saw masked
	keep := func(next, prev map[string]string) map[string]string {
		for k, v := range next {
			if v == masked {
				next[k] = prev[k]
			}
		}
		return next
	}
	c.Headers = keep(c.Headers, old.Headers)
	c.Env = keep(c.Env, old.Env)
	c.Name = strings.TrimSpace(c.Name)
	c.URL = strings.TrimSpace(c.URL)
	c.Command = strings.TrimSpace(c.Command)

	switch c.Transport {
	case "http":
		if u, err := url.Parse(c.URL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			httpErr(w, http.StatusBadRequest, "URL must be an http(s) URL")
			return
		}
	case "stdio":
		if c.Command == "" {
			httpErr(w, http.StatusBadRequest, "command is required")
			return
		}
	default:
		httpErr(w, http.StatusBadRequest, "transport must be http or stdio")
		return
	}
	if c.Name == "" {
		httpErr(w, http.StatusBadRequest, "name is required")
		return
	}
	c.ID = ""
	raw, _ := json.Marshal(c)
	row := db.MCPServer{ID: id, Config: raw}
	if err := s.db.SaveMCPServer(&row); err != nil {
		dbErr(w, err)
		return
	}
	s.mcp.Forget(row.ID)
	c.ID = row.ID
	c.Headers, c.Env = maskMap(c.Headers), maskMap(c.Env)
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) deleteMCP(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.db.DeleteMCPServer(id); err != nil {
		dbErr(w, err)
		return
	}
	s.mcp.Forget(id)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// mcpStatus connects to each enabled server and lists its tools.
func (s *Server) mcpStatus(w http.ResponseWriter, r *http.Request) {
	st := s.mcp.Status(r.Context(), s.mcpConfigs(), r.URL.Query().Get("refresh") == "1")
	writeJSON(w, http.StatusOK, st)
}
