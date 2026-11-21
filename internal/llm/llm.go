// Package llm talks to model providers. Every provider streams plain text
// deltas; model "thinking" is wrapped in <think>…</think> so the UI has a
// single format to render, whatever the provider. Tool calls requested by
// the model are returned in Result for the caller to execute.
package llm

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type Message struct {
	Role       string // user | assistant | tool
	Content    string
	Images     []string   // data URLs (user)
	ToolCalls  []ToolCall // assistant: tools it asked for
	ToolCallID string     // tool: which call this answers
}

type ToolCall struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Args string `json:"args"` // raw JSON object
}

type Tool struct {
	Name        string
	Description string
	Schema      json.RawMessage // JSON Schema for the arguments
}

type Model struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Request struct {
	Model    string
	System   string
	Messages []Message
	Tools    []Tool
}

type Result struct {
	ToolCalls []ToolCall
	// Truncated is set when the model stopped because it ran out of
	// output or context length, rather than finishing its answer.
	Truncated bool
}

type Client interface {
	ListModels(ctx context.Context) ([]Model, error)
	// Stream calls onDelta for every chunk of text. It returns when the
	// response is complete, the context is cancelled, or an error occurs.
	Stream(ctx context.Context, req Request, onDelta func(string)) (Result, error)
}

// Embedder is implemented by providers with an embeddings endpoint.
type Embedder interface {
	Embed(ctx context.Context, model string, inputs []string) ([][]float32, error)
}

var ErrNoEmbeddings = errors.New("this provider has no embeddings API")

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
	_, err := c.Stream(ctx, req, func(s string) { sb.WriteString(s) })
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

var embedHTTP = &http.Client{Timeout: 2 * time.Minute}

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
	emit    func(string)
	inThink bool
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

func schemaOrEmpty(s json.RawMessage) json.RawMessage {
	if len(s) == 0 || string(s) == "null" {
		return json.RawMessage(`{"type":"object","properties":{}}`)
	}
	return s
}

func argsOrEmpty(a string) string {
	if strings.TrimSpace(a) == "" {
		return "{}"
	}
	return a
}

type providerErr struct{ msg string }

func (e *providerErr) Error() string { return e.msg }

var thinkTokRe = regexp.MustCompile(`<think>|</think>|\n</tool>`)

// NormalizeThink adds the opening <think> that some models never write.
// Chat templates such as GLM's end the prompt with "<think>", so the model's
// own output starts mid-reasoning and only closes it: "reasoning</think>answer".
// Servers that parse reasoning out (llama.cpp's reasoning parser, vLLM's
// --reasoning-parser) send it separately; the rest pass it through like this.
// Each unmatched </think> gets a <think> at the start of its stretch of text:
// the start of the reply, or the end of the previous </think> or tool block.
func NormalizeThink(s string) string {
	var inserts []int
	open, segStart := false, 0
	for _, m := range thinkTokRe.FindAllStringIndex(s, -1) {
		switch s[m[0]:m[1]] {
		case "<think>":
			open = true
		case "</think>":
			if !open {
				start := segStart
				for start < m[0] && strings.ContainsRune(" \t\r\n", rune(s[start])) {
					start++
				}
				inserts = append(inserts, start)
			}
			open, segStart = false, m[1]
		default: // end of a tool block
			if !open {
				segStart = m[1]
			}
		}
	}
	if len(inserts) == 0 {
		return s
	}
	var b strings.Builder
	prev := 0
	for _, at := range inserts {
		b.WriteString(s[prev:at])
		b.WriteString("<think>")
		prev = at
	}
	b.WriteString(s[prev:])
	return b.String()
}
