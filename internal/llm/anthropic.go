package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
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

func (a *anthropic) Stream(ctx context.Context, r Request, onDelta func(string)) error {
	msgs := make([]map[string]any, 0, len(r.Messages))
	for _, m := range r.Messages {
		var blocks []map[string]any
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
		if m.Content != "" || len(blocks) == 0 {
			blocks = append(blocks, map[string]any{"type": "text", "text": m.Content})
		}
		msgs = append(msgs, map[string]any{"role": m.Role, "content": blocks})
	}
	body := map[string]any{
		"model":      r.Model,
		"max_tokens": 16000,
		"messages":   msgs,
		"stream":     true,
	}
	if r.System != "" {
		body["system"] = r.System
	}

	resp, err := a.do(ctx, "POST", "/v1/messages", body, streamHTTP)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	tw := &thinkWrapper{emit: onDelta}
	defer tw.close()
	return sseData(resp.Body, func(event, data string) (bool, error) {
		var ev struct {
			Type  string `json:"type"`
			Delta struct {
				Type     string `json:"type"`
				Text     string `json:"text"`
				Thinking string `json:"thinking"`
			} `json:"delta"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			return false, nil
		}
		switch ev.Type {
		case "content_block_delta":
			switch ev.Delta.Type {
			case "text_delta":
				tw.text(ev.Delta.Text)
			case "thinking_delta":
				tw.reasoning(ev.Delta.Thinking)
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
}
