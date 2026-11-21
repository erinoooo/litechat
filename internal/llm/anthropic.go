package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"sort"
)

// anthropic speaks the native Claude Messages API.
type anthropic struct{ base, key string }

const anthropicVersion = "2023-06-01"

func (a *anthropic) do(ctx context.Context, method, path string, body any, client *http.Client) (*http.Response, error) {
	var buf []byte
	if body != nil {
		var err error
		if buf, err = json.Marshal(body); err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, a.base+path, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.key)
	req.Header.Set("anthropic-version", anthropicVersion)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		defer resp.Body.Close()
		return nil, readError(resp)
	}
	return resp, nil
}

func (a *anthropic) ListModels(ctx context.Context) ([]Model, error) {
	resp, err := a.do(ctx, "GET", "/v1/models?limit=100", nil, shortHTTP)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var body struct {
		Data []struct {
			ID          string `json:"id"`
			DisplayName string `json:"display_name"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	out := make([]Model, 0, len(body.Data))
	for _, m := range body.Data { // API returns newest first; keep that order
		name := m.DisplayName
		if name == "" {
			name = m.ID
		}
		out = append(out, Model{ID: m.ID, Name: name})
	}
	return out, nil
}

// toAnthropic converts the generic history to Claude content blocks.
// Consecutive tool results are merged into a single user turn.
func toAnthropic(in []Message) []map[string]any {
	var out []map[string]any
	for _, m := range in {
		var blocks []map[string]any
		role := m.Role
		switch {
		case m.Role == "tool":
			role = "user"
			blocks = []map[string]any{{"type": "tool_result", "tool_use_id": m.ToolCallID, "content": m.Content}}
			if n := len(out); n > 0 && out[n-1]["role"] == "user" {
				if prev, ok := out[n-1]["content"].([]map[string]any); ok && len(prev) > 0 && prev[0]["type"] == "tool_result" {
					out[n-1]["content"] = append(prev, blocks...)
					continue
				}
			}
		case m.Role == "assistant":
			if m.Content != "" {
				blocks = append(blocks, map[string]any{"type": "text", "text": m.Content})
			}
			for _, c := range m.ToolCalls {
				blocks = append(blocks, map[string]any{"type": "tool_use", "id": c.ID, "name": c.Name, "input": json.RawMessage(argsOrEmpty(c.Args))})
			}
		default:
			for _, img := range m.Images {
				mt, data, ok := splitDataURL(img)
				if !ok {
					continue
				}
				blocks = append(blocks, map[string]any{
					"type":   "image",
					"source": map[string]any{"type": "base64", "media_type": mt, "data": data},
				})
			}
			if m.Content != "" {
				blocks = append(blocks, map[string]any{"type": "text", "text": m.Content})
			}
		}
		if len(blocks) == 0 {
			blocks = []map[string]any{{"type": "text", "text": "…"}}
		}
		out = append(out, map[string]any{"role": role, "content": blocks})
	}
	return out
}

func (a *anthropic) Stream(ctx context.Context, r Request, onDelta func(string)) (Result, error) {
	body := map[string]any{
		"model":      r.Model,
		"max_tokens": 16000,
		"messages":   toAnthropic(r.Messages),
		"stream":     true,
	}
	if r.System != "" {
		body["system"] = r.System
	}
	if len(r.Tools) > 0 {
		tools := make([]map[string]any, len(r.Tools))
		for i, t := range r.Tools {
			tools[i] = map[string]any{"name": t.Name, "description": t.Description, "input_schema": schemaOrEmpty(t.Schema)}
		}
		body["tools"] = tools
	}

	resp, err := a.do(ctx, "POST", "/v1/messages", body, streamHTTP)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()

	tw := &thinkWrapper{emit: onDelta}
	calls := map[int]*ToolCall{}
	err = sseData(resp.Body, func(event, data string) (bool, error) {
		var ev struct {
			Type         string `json:"type"`
			Index        int    `json:"index"`
			ContentBlock struct {
				Type string `json:"type"`
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"content_block"`
			Delta struct {
				Type        string `json:"type"`
				Text        string `json:"text"`
				Thinking    string `json:"thinking"`
				PartialJSON string `json:"partial_json"`
			} `json:"delta"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			return false, nil
		}
		switch ev.Type {
		case "content_block_start":
			if ev.ContentBlock.Type == "tool_use" {
				calls[ev.Index] = &ToolCall{ID: ev.ContentBlock.ID, Name: ev.ContentBlock.Name}
			}
		case "content_block_delta":
			switch ev.Delta.Type {
			case "text_delta":
				tw.text(ev.Delta.Text)
			case "thinking_delta":
				tw.reasoning(ev.Delta.Thinking)
			case "input_json_delta":
				if c := calls[ev.Index]; c != nil {
					c.Args += ev.Delta.PartialJSON
				}
			}
		case "message_stop":
			return true, nil
		case "error":
			if ev.Error != nil {
				return true, &providerErr{ev.Error.Message}
			}
		}
		return false, nil
	})
	tw.close()

	var res Result
	idx := make([]int, 0, len(calls))
	for i := range calls {
		idx = append(idx, i)
	}
	sort.Ints(idx)
	for _, i := range idx {
		c := calls[i]
		c.Args = argsOrEmpty(c.Args)
		res.ToolCalls = append(res.ToolCalls, *c)
	}
	return res, err
}
