// Package llm talks to model providers. Every provider streams plain text
// deltas; model "thinking" is wrapped in <think>…</think> so the UI has a
// single format to render, whatever the provider.
package llm

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Message struct {
	Role    string   // user | assistant
	Content string
	Images  []string // data URLs
}

type Model struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Request struct {
	Model    string
	System   string
	Messages []Message
}

type Client interface {
	ListModels(ctx context.Context) ([]Model, error)
	// Stream calls onDelta for every chunk of text. It returns when the
	// response is complete, the context is cancelled, or an error occurs.
	Stream(ctx context.Context, req Request, onDelta func(string)) error
}

func New(kind, baseURL, apiKey string) (Client, error) {
	baseURL = strings.TrimRight(baseURL, "/")
	switch kind {
	case "openai":
		return &openAI{base: baseURL, key: apiKey}, nil
	case "anthropic":
		if baseURL == "" {
			baseURL = "https://api.anthropic.com"
		}
		return &anthropic{base: baseURL, key: apiKey}, nil
	}
	return nil, fmt.Errorf("unknown provider kind %q", kind)
}

// Complete is a convenience wrapper that collects a full streamed response.
func Complete(ctx context.Context, c Client, req Request) (string, error) {
	var sb strings.Builder
	err := c.Stream(ctx, req, func(s string) { sb.WriteString(s) })
	return sb.String(), err
}

// streamHTTP has no overall timeout (responses can take minutes);
// cancellation comes from the request context.
var streamHTTP = &http.Client{Transport: &http.Transport{
	Proxy:                 http.ProxyFromEnvironment,
	ResponseHeaderTimeout: 5 * time.Minute,
	IdleConnTimeout:       90 * time.Second,
}}

var shortHTTP = &http.Client{Timeout: 15 * time.Second}

func readError(resp *http.Response) error {
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	msg := strings.TrimSpace(string(b))
	if msg == "" {
		msg = resp.Status
	}
	return fmt.Errorf("provider returned %d: %s", resp.StatusCode, msg)
}

// sseData yields the payload of each `data:` line in an SSE stream.
func sseData(r io.Reader, fn func(event, data string) (stop bool, err error)) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 16*1024*1024)
	event := ""
	for sc.Scan() {
		line := sc.Text()
		switch {
		case line == "":
			event = ""
		case strings.HasPrefix(line, "event:"):
			event = strings.TrimSpace(line[6:])
		case strings.HasPrefix(line, "data:"):
			stop, err := fn(event, strings.TrimSpace(line[5:]))
			if err != nil || stop {
				return err
			}
		}
	}
	return sc.Err()
}

// thinkWrapper turns separate reasoning/text deltas into one text stream
// with the reasoning wrapped in <think> tags.
type thinkWrapper struct {
	emit     func(string)
	inThink  bool
}

func (t *thinkWrapper) reasoning(s string) {
	if s == "" {
		return
	}
	if !t.inThink {
		t.emit("<think>")
		t.inThink = true
	}
	t.emit(s)
}

func (t *thinkWrapper) text(s string) {
	if s == "" {
		return
	}
	t.close()
	t.emit(s)
}

func (t *thinkWrapper) close() {
	if t.inThink {
		t.emit("</think>\n\n")
		t.inThink = false
	}
}

func splitDataURL(u string) (mediaType, data string, ok bool) {
	// data:image/png;base64,XXXX
	if !strings.HasPrefix(u, "data:") {
		return "", "", false
	}
	comma := strings.IndexByte(u, ',')
	if comma < 0 {
		return "", "", false
	}
	meta := u[5:comma]
	mediaType = strings.TrimSuffix(meta, ";base64")
	return mediaType, u[comma+1:], true
}
