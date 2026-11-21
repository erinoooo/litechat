package discover

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

// fake starts an HTTP server answering the given paths with JSON bodies.
func fake(t *testing.T, routes map[string]string) (host string, port int) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := routes[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	h, p, _ := net.SplitHostPort(srv.Listener.Addr().String())
	port, _ = strconv.Atoi(p)
	return h, port
}

func TestProbeIdentifiesServers(t *testing.T) {
	models := `{"object":"list","data":[{"id":"m1","owned_by":"x"},{"id":"m2"}]}`
	cases := []struct {
		name   string
		routes map[string]string
		want   string
	}{
		{"ollama", map[string]string{"/api/version": `{"version":"0.12.3"}`, "/v1/models": models}, "Ollama"},
		{"lmstudio", map[string]string{"/api/v0/models": `{"data":[{"id":"m","type":"llm"}]}`, "/v1/models": models}, "LM Studio"},
		{"llamacpp", map[string]string{"/props": `{"default_generation_settings":{},"build_info":"b6000"}`, "/v1/models": models}, "llama.cpp"},
		{"kobold", map[string]string{"/api/extra/version": `{"result":"KoboldCpp","version":"1.99"}`, "/v1/models": models}, "KoboldCpp"},
		{"vllm by owned_by", map[string]string{"/v1/models": `{"data":[{"id":"q","owned_by":"vllm"}]}`}, "vLLM"},
		{"generic", map[string]string{"/v1/models": models}, "OpenAI-compatible server"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h, p := fake(t, c.routes)
			s, ok := Probe(context.Background(), h, p)
			if !ok {
				t.Fatal("not detected")
			}
			if s.Kind != c.want {
				t.Errorf("kind = %q, want %q", s.Kind, c.want)
			}
			if s.BaseURL != "http://"+net.JoinHostPort(h, strconv.Itoa(p))+"/v1" || len(s.Models) == 0 {
				t.Errorf("bad server: %+v", s)
			}
		})
	}
}

func TestProbeIgnoresNonAIServers(t *testing.T) {
	// e.g. a web app that serves HTML for every path, like liteChat itself
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<!doctype html><p>hi"))
	}))
	defer srv.Close()
	h, p, _ := net.SplitHostPort(srv.Listener.Addr().String())
	port, _ := strconv.Atoi(p)
	if s, ok := Probe(context.Background(), h, port); ok {
		t.Errorf("should not detect a server, got %+v", s)
	}
}
