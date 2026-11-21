package server

import (
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"

	"litechat/internal/db"
)

const (
	cookieName = "litechat_session"
	sessionTTL = 30 * 24 * time.Hour
	hashKey    = "password_hash"
	minPwLen   = 8
)

// SetPassword stores a bcrypt hash of pw and logs out every session.
func SetPassword(d *db.DB, pw string) error {
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if err := d.SetSetting(hashKey, string(h)); err != nil {
		return err
	}
	d.DeleteAllSessions()
	return nil
}

// PasswordMatches reports whether pw matches the stored password.
func PasswordMatches(d *db.DB, pw string) bool {
	h, err := d.GetSetting(hashKey)
	if err != nil {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(h), []byte(pw)) == nil
}

func (s *Server) hasPassword() bool {
	_, err := s.db.GetSetting(hashKey)
	return err == nil
}

func (s *Server) authed(r *http.Request) bool {
	c, err := r.Cookie(cookieName)
	return err == nil && s.db.ValidSession(c.Value)
}

func (s *Server) requireAuth(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.authed(r) {
			httpErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		h(w, r)
	}
}

func (s *Server) startSession(w http.ResponseWriter, r *http.Request) {
	tok, err := s.db.CreateSession(sessionTTL)
	if err != nil {
		dbErr(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    tok,
		Path:     "/",
		MaxAge:   int(sessionTTL.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
	})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) authStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{
		"setup":  s.hasPassword(),
		"authed": s.authed(r),
	})
}

// authSetup sets the password on first run only.
func (s *Server) authSetup(w http.ResponseWriter, r *http.Request) {
	if s.hasPassword() {
		httpErr(w, http.StatusConflict, "already set up")
		return
	}
	var body struct{ Password string }
	if !readJSON(w, r, &body) {
		return
	}
	if len(body.Password) < minPwLen {
		httpErr(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}
	if err := SetPassword(s.db, body.Password); err != nil {
		dbErr(w, err)
		return
	}
	s.startSession(w, r)
}

func (s *Server) authLogin(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if s.tooManyFailures(ip) {
		httpErr(w, http.StatusTooManyRequests, "too many attempts, try again in a few minutes")
		return
	}
	var body struct{ Password string }
	if !readJSON(w, r, &body) {
		return
	}
	if !PasswordMatches(s.db, body.Password) {
		s.recordFailure(ip)
		time.Sleep(500 * time.Millisecond)
		httpErr(w, http.StatusUnauthorized, "wrong password")
		return
	}
	s.startSession(w, r)
}

func (s *Server) authLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(cookieName); err == nil {
		s.db.DeleteSession(c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", MaxAge: -1})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) authChangePassword(w http.ResponseWriter, r *http.Request) {
	var body struct{ Current, New string }
	if !readJSON(w, r, &body) {
		return
	}
	if !PasswordMatches(s.db, body.Current) {
		httpErr(w, http.StatusUnauthorized, "current password is wrong")
		return
	}
	if len(body.New) < minPwLen {
		httpErr(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}
	if err := SetPassword(s.db, body.New); err != nil {
		dbErr(w, err)
		return
	}
	s.startSession(w, r) // all old sessions were dropped; keep this one logged in
}

// 10 failures per 10 minutes per IP.
func (s *Server) tooManyFailures(ip string) bool {
	s.loginMu.Lock()
	defer s.loginMu.Unlock()
	cutoff := time.Now().Add(-10 * time.Minute)
	recent := s.loginFail[ip][:0]
	for _, t := range s.loginFail[ip] {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}
	s.loginFail[ip] = recent
	return len(recent) >= 10
}

func (s *Server) recordFailure(ip string) {
	s.loginMu.Lock()
	s.loginFail[ip] = append(s.loginFail[ip], time.Now())
	s.loginMu.Unlock()
}
