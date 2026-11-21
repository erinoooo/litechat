// Package server wires HTTP routes to the database and LLM providers.
package server

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"litechat/internal/db"
)

type Server struct {
	db     *db.DB
	static fs.FS
	mux    *http.ServeMux

	modelsMu    sync.Mutex
	modelsCache map[string]modelsEntry // provider id -> cached models

	loginMu   sync.Mutex
	loginFail map[string][]time.Time // ip -> recent failures
}

func New(d *db.DB, static fs.FS) *Server {
	s := &Server{
		db:          d,
		static:      static,
		mux:         http.NewServeMux(),
		modelsCache: map[string]modelsEntry{},
		loginFail:   map[string][]time.Time{},
	}
	s.routes()
	go func() {
		for range time.Tick(time.Hour) {
			d.PruneSessions()
		}
	}()
	return s
}

func (s *Server) routes() {
	m := s.mux
	// public
	m.HandleFunc("GET /api/auth/status", s.authStatus)
	m.HandleFunc("POST /api/auth/setup", s.csrf(s.authSetup))
	m.HandleFunc("POST /api/auth/login", s.csrf(s.authLogin))
	m.HandleFunc("POST /api/auth/logout", s.csrf(s.authLogout))

	// authenticated
	a := func(pattern string, h http.HandlerFunc) { m.HandleFunc(pattern, s.requireAuth(s.csrf(h))) }
	a("POST /api/auth/password", s.authChangePassword)

	a("GET /api/providers", s.listProviders)
	a("POST /api/providers", s.saveProvider)
	a("PUT /api/providers/{id}", s.saveProvider)
	a("DELETE /api/providers/{id}", s.deleteProvider)
	a("GET /api/models", s.listModels)

	a("GET /api/chats", s.listChats)
	a("POST /api/chats", s.createChat)
	a("GET /api/chats/{id}", s.getChat)
	a("PATCH /api/chats/{id}", s.updateChat)
	a("DELETE /api/chats/{id}", s.deleteChat)
	a("POST /api/chats/{id}/messages", s.sendMessage)

	a("GET /api/presets", s.listPresets)
	a("POST /api/presets", s.savePreset)
	a("PUT /api/presets/{id}", s.savePreset)
	a("DELETE /api/presets/{id}", s.deletePreset)

	a("GET /api/settings", s.getSettings)
	a("PUT /api/settings", s.putSettings)

	a("GET /api/search", s.search)

	m.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		httpErr(w, http.StatusNotFound, "not found")
	})
	m.Handle("/", s.staticHandler())
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "same-origin")
	w.Header().Set("X-Frame-Options", "DENY")
	s.mux.ServeHTTP(w, r)
}

// csrf: state-changing requests must carry a custom header, which a
// cross-site form or image can't add without a CORS preflight.
func (s *Server) csrf(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Header.Get("X-LiteChat") != "1" {
			httpErr(w, http.StatusForbidden, "missing X-LiteChat header")
			return
		}
		h(w, r)
	}
}

// ---------- helpers ----------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func httpErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<20) // images can be big
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		httpErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return false
	}
	return true
}

func dbErr(w http.ResponseWriter, err error) {
	if errors.Is(err, db.ErrNotFound) {
		httpErr(w, http.StatusNotFound, "not found")
		return
	}
	log.Printf("db error: %v", err)
	httpErr(w, http.StatusInternalServerError, "internal error")
}

func clientIP(r *http.Request) string {
	ip := r.RemoteAddr
	if i := strings.LastIndexByte(ip, ':'); i > 0 {
		ip = ip[:i]
	}
	return ip
}
