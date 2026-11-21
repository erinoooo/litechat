package server

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"litechat/internal/discover"
)

type discovered struct {
	discover.Server
	// ProviderID is set when this server is already added as a provider.
	ProviderID string `json:"provider_id,omitempty"`
}

// discoverServers finds local AI servers. ?network=1 also scans the LAN.
func (s *Server) discoverServers(w http.ResponseWriter, r *http.Request) {
	network := r.URL.Query().Get("network") == "1"
	timeout := 8 * time.Second
	if network {
		timeout = 25 * time.Second
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()
	res := discover.Scan(ctx, discover.Options{Network: network, SkipPort: s.opt.ListenPort})

	providers, err := s.db.ListProviders()
	if err != nil {
		dbErr(w, err)
		return
	}
	out := make([]discovered, 0, len(res.Servers))
	for _, srv := range res.Servers {
		d := discovered{Server: srv}
		for _, p := range providers {
			if sameEndpoint(p.BaseURL, srv) {
				d.ProviderID = p.ID
				break
			}
		}
		out = append(out, d)
	}
	writeJSON(w, http.StatusOK, map[string]any{"servers": out, "scanned": res.Scanned, "took_ms": res.TookMS, "network": network})
}

// sameEndpoint reports whether a provider URL points at a discovered server.
// Loopback names are treated as equal, and so is the Docker host for local servers.
func sameEndpoint(base string, srv discover.Server) bool {
	u, err := url.Parse(base)
	if err != nil {
		return false
	}
	port := u.Port()
	if port == "" {
		port = map[string]string{"http": "80", "https": "443"}[u.Scheme]
	}
	if port != strconv.Itoa(srv.Port) {
		return false
	}
	h := strings.ToLower(u.Hostname())
	if h == srv.Host {
		return true
	}
	if srv.Local && isLoopback(h) {
		return true
	}
	if ips, err := net.LookupHost(h); err == nil {
		for _, ip := range ips {
			if ip == srv.Host || (srv.Local && isLoopback(ip)) {
				return true
			}
		}
	}
	return false
}

func isLoopback(h string) bool {
	if h == "localhost" || h == "0.0.0.0" {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}
