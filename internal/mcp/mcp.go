// Package mcp is a small Model Context Protocol client: enough to list a
// server's tools and call them, over Streamable HTTP or stdio.
package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const protocolVersion = "2025-06-18"

type Config struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Transport string            `json:"transport"` // http | stdio
	URL       string            `json:"url"`
	Headers   map[string]string `json:"headers"`
	Command   string            `json:"command"`
	Args      []string          `json:"args"`
	Env       map[string]string `json:"env"`
	Enabled   bool              `json:"enabled"`
}

type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *rpcError) Error() string { return fmt.Sprintf("MCP error %d: %s", e.Code, e.Message) }

type rpcResponse struct {
	ID     *int64          `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *rpcError       `json:"error"`
}

// transport sends one JSON-RPC request (id != 0) or notification (id == 0).
type transport interface {
	call(ctx context.Context, id int64, method string, params any) (json.RawMessage, error)
	close()
}

// Client is a connected, initialized MCP session.
type Client struct {
	t      transport
	nextID atomic.Int64
}

func Connect(ctx context.Context, cfg Config, allowStdio bool) (*Client, error) {
	var t transport
	switch cfg.Transport {
	case "http":
		t = &httpTransport{url: cfg.URL, headers: cfg.Headers}
	case "stdio":
		if !allowStdio {
			return nil, errors.New("stdio MCP servers are disabled; start liteChat with LITECHAT_MCP_STDIO=1 to allow them")
		}
		st, err := startStdio(cfg)
		if err != nil {
			return nil, err
		}
		t = st
	default:
		return nil, fmt.Errorf("unknown transport %q", cfg.Transport)
	}
	c := &Client{t: t}
	_, err := c.request(ctx, "initialize", map[string]any{
		"protocolVersion": protocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]string{"name": "liteChat", "version": "0.2"},
	})
	if err != nil {
		t.close()
		return nil, fmt.Errorf("initialize: %w", err)
	}
	if ht, ok := t.(*httpTransport); ok {
		ht.initialized = true
	}
	_, _ = t.call(ctx, 0, "notifications/initialized", nil)
	return c, nil
}

func (c *Client) request(ctx context.Context, method string, params any) (json.RawMessage, error) {
	return c.t.call(ctx, c.nextID.Add(1), method, params)
}

func (c *Client) Close() { c.t.close() }

func (c *Client) ListTools(ctx context.Context) ([]Tool, error) {
	var all []Tool
	cursor := ""
	for range 20 { // page limit
		params := map[string]any{}
		if cursor != "" {
			params["cursor"] = cursor
		}
		raw, err := c.request(ctx, "tools/list", params)
		if err != nil {
			return nil, err
		}
		var res struct {
			Tools      []Tool `json:"tools"`
			NextCursor string `json:"nextCursor"`
		}
		if err := json.Unmarshal(raw, &res); err != nil {
			return nil, err
		}
		all = append(all, res.Tools...)
		if res.NextCursor == "" {
			break
		}
		cursor = res.NextCursor
	}
	return all, nil
}

const maxResult = 20000

// CallTool runs a tool and flattens its result to text.
func (c *Client) CallTool(ctx context.Context, name string, args json.RawMessage) (text string, isError bool, err error) {
	if len(args) == 0 {
		args = json.RawMessage("{}")
	}
	raw, err := c.request(ctx, "tools/call", map[string]any{"name": name, "arguments": args})
	if err != nil {
		return "", true, err
	}
	var res struct {
		Content []struct {
			Type     string `json:"type"`
			Text     string `json:"text"`
			MimeType string `json:"mimeType"`
			Resource *struct {
				URI  string `json:"uri"`
				Text string `json:"text"`
			} `json:"resource"`
		} `json:"content"`
		StructuredContent json.RawMessage `json:"structuredContent"`
		IsError           bool            `json:"isError"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return "", true, err
	}
	var parts []string
	for _, p := range res.Content {
		switch p.Type {
		case "text":
			parts = append(parts, p.Text)
		case "resource":
			if p.Resource != nil {
				parts = append(parts, fmt.Sprintf("[resource %s]\n%s", p.Resource.URI, p.Resource.Text))
			}
		default:
			parts = append(parts, fmt.Sprintf("[%s content omitted: %s]", p.Type, p.MimeType))
		}
	}
	if len(parts) == 0 && len(res.StructuredContent) > 0 {
		parts = append(parts, string(res.StructuredContent))
	}
	text = strings.Join(parts, "\n\n")
	if len(text) > maxResult {
		text = text[:maxResult] + "\n…[truncated]"
	}
	return text, res.IsError, nil
}

// ---------- Streamable HTTP ----------

type httpTransport struct {
	url         string
	headers     map[string]string
	mu          sync.Mutex
	session     string
	initialized bool
}

var mcpHTTP = &http.Client{Timeout: 5 * time.Minute}

func (h *httpTransport) call(ctx context.Context, id int64, method string, params any) (json.RawMessage, error) {
	msg := map[string]any{"jsonrpc": "2.0", "method": method}
	if params != nil {
		msg["params"] = params
	}
	if id != 0 {
		msg["id"] = id
	}
	body, _ := json.Marshal(msg)
	req, err := http.NewRequestWithContext(ctx, "POST", h.url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	for k, v := range h.headers {
		req.Header.Set(k, v)
	}
	h.mu.Lock()
	if h.session != "" {
		req.Header.Set("Mcp-Session-Id", h.session)
	}
	if h.initialized {
		req.Header.Set("MCP-Protocol-Version", protocolVersion)
	}
	h.mu.Unlock()

	resp, err := mcpHTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if s := resp.Header.Get("Mcp-Session-Id"); s != "" {
		h.mu.Lock()
		h.session = s
		h.mu.Unlock()
	}
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("MCP server returned %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	if id == 0 {
		return nil, nil
	}
	if strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		return readSSEResponse(resp.Body, id)
	}
	var r rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}
	if r.Error != nil {
		return nil, r.Error
	}
	return r.Result, nil
}

func readSSEResponse(body io.Reader, id int64) (json.RawMessage, error) {
	sc := bufio.NewScanner(body)
	sc.Buffer(make([]byte, 64*1024), 32*1024*1024)
	var data strings.Builder
	flush := func() (json.RawMessage, bool, error) {
		defer data.Reset()
		if data.Len() == 0 {
			return nil, false, nil
		}
		var r rpcResponse
		if json.Unmarshal([]byte(data.String()), &r) != nil || r.ID == nil || *r.ID != id {
			return nil, false, nil // a notification or unrelated message
		}
		if r.Error != nil {
			return nil, true, r.Error
		}
		return r.Result, true, nil
	}
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			if res, done, err := flush(); done {
				return res, err
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			data.WriteString(strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
	}
	if res, done, err := flush(); done {
		return res, err
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return nil, errors.New("MCP stream ended without a response")
}

func (h *httpTransport) close() {
	h.mu.Lock()
	s := h.session
	h.mu.Unlock()
	if s == "" {
		return
	}
	req, err := http.NewRequest("DELETE", h.url, nil)
	if err != nil {
		return
	}
	req.Header.Set("Mcp-Session-Id", s)
	for k, v := range h.headers {
		req.Header.Set(k, v)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if resp, err := mcpHTTP.Do(req.WithContext(ctx)); err == nil {
		resp.Body.Close()
	}
}

// ---------- stdio ----------

type stdioTransport struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	writeMu sync.Mutex
	mu      sync.Mutex
	pending map[int64]chan rpcResponse
	done    chan struct{}
	stderr  *tailBuffer
}

func startStdio(cfg Config) (*stdioTransport, error) {
	if cfg.Command == "" {
		return nil, errors.New("no command set")
	}
	cmd := exec.Command(cfg.Command, cfg.Args...)
	cmd.Env = os.Environ()
	for k, v := range cfg.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	st := &stdioTransport{cmd: cmd, stdin: stdin, pending: map[int64]chan rpcResponse{}, done: make(chan struct{}), stderr: &tailBuffer{}}
	cmd.Stderr = st.stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	go func() {
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 64*1024), 32*1024*1024)
		for sc.Scan() {
			var r rpcResponse
			if json.Unmarshal(sc.Bytes(), &r) != nil || r.ID == nil {
				continue // notifications, logs
			}
			st.mu.Lock()
			ch := st.pending[*r.ID]
			delete(st.pending, *r.ID)
			st.mu.Unlock()
			if ch != nil {
				ch <- r
			}
		}
		close(st.done)
		_ = cmd.Wait()
	}()
	return st, nil
}

func (s *stdioTransport) call(ctx context.Context, id int64, method string, params any) (json.RawMessage, error) {
	msg := map[string]any{"jsonrpc": "2.0", "method": method}
	if params != nil {
		msg["params"] = params
	}
	var ch chan rpcResponse
	if id != 0 {
		msg["id"] = id
		ch = make(chan rpcResponse, 1)
		s.mu.Lock()
		s.pending[id] = ch
		s.mu.Unlock()
		defer func() {
			s.mu.Lock()
			delete(s.pending, id)
			s.mu.Unlock()
		}()
	}
	b, _ := json.Marshal(msg)
	s.writeMu.Lock()
	_, err := s.stdin.Write(append(b, '\n'))
	s.writeMu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("write to MCP server: %w%s", err, s.stderr.hint())
	}
	if id == 0 {
		return nil, nil
	}
	select {
	case r := <-ch:
		if r.Error != nil {
			return nil, r.Error
		}
		return r.Result, nil
	case <-s.done:
		return nil, fmt.Errorf("MCP server exited%s", s.stderr.hint())
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *stdioTransport) close() {
	_ = s.stdin.Close()
	select {
	case <-s.done:
	case <-time.After(2 * time.Second):
		if s.cmd.Process != nil {
			_ = s.cmd.Process.Kill()
		}
	}
}

// tailBuffer keeps the last few KB of a server's stderr for error messages.
type tailBuffer struct {
	mu  sync.Mutex
	buf []byte
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.mu.Lock()
	t.buf = append(t.buf, p...)
	if len(t.buf) > 4096 {
		t.buf = t.buf[len(t.buf)-4096:]
	}
	t.mu.Unlock()
	return len(p), nil
}

func (t *tailBuffer) hint() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	s := strings.TrimSpace(string(t.buf))
	if s == "" {
		return ""
	}
	if i := strings.LastIndexByte(s, '\n'); i >= 0 && len(s)-i < 300 {
		s = s[i+1:]
	}
	if len(s) > 300 {
		s = s[len(s)-300:]
	}
	return ": " + s
}
