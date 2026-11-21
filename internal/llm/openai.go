package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"sort"
)

// openAI speaks the OpenAI Chat Completions API, which Ollama, llama.cpp,
// LM Studio, vLLM, OpenRouter, Groq, Together, etc. all implement.
type openAI struct{ base, key string }

func (o *openAI) do(ctx context.Context, method, path string, body any, client *http.Client) (*http.Response, error) {
	var rd *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req, err := http.NewRequestWithContext(ctx, method, o.base+path, rd)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if o.key != "" {
		req.Header.Set("Authorization", "Bearer "+o.key)
	}
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

func (o *openAI) ListModels(ctx context.Context) ([]Model, error) {
	resp, err := o.do(ctx, "GET", "/models", nil, shortHTTP)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var body struct {
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	out := make([]Model, 0, len(body.Data))
	for _, m := range body.Data {
		name := m.Name
		if name == "" {
			name = m.ID
		}
		out = append(out, Model{ID: m.ID, Name: name})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (o *openAI) Stream(ctx context.Context, r Request, onDelta func(string)) error {
	type part struct {
		Type     string `json:"type"`
		Text     string `json:"text,omitempty"`
		ImageURL *struct {
			URL string `json:"url"`
		} `json:"image_url,omitempty"`
	}
	msgs := []map[string]any{}
	if r.System != "" {
		msgs = append(msgs, map[string]any{"role": "system", "content": r.System})
	}
	for _, m := range r.Messages {
		if len(m.Images) == 0 {
			msgs = append(msgs, map[string]any{"role": m.Role, "content": m.Content})
			continue
		}
		parts := []part{{Type: "text", Text: m.Content}}
		for _, img := range m.Images {
			p := part{Type: "image_url"}
			p.ImageURL = &struct {
				URL string `json:"url"`
			}{URL: img}
			parts = append(parts, p)
		}
		msgs = append(msgs, map[string]any{"role": m.Role, "content": parts})
	}

	resp, err := o.do(ctx, "POST", "/chat/completions", map[string]any{
		"model":    r.Model,
		"messages": msgs,
		"stream":   true,
	}, streamHTTP)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	tw := &thinkWrapper{emit: onDelta}
	defer tw.close()
	return sseData(resp.Body, func(_, data string) (bool, error) {
		if data == "[DONE]" {
			return true, nil
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content          string `json:"content"`
					Reasoning        string `json:"reasoning"`
					ReasoningContent string `json:"reasoning_content"`
				} `json:"delta"`
			} `json:"choices"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return false, nil // skip malformed keep-alives
		}
		if chunk.Error != nil {
			return true, &providerErr{chunk.Error.Message}
		}
		for _, c := range chunk.Choices {
			tw.reasoning(c.Delta.ReasoningContent)
			tw.reasoning(c.Delta.Reasoning)
			tw.text(c.Delta.Content)
		}
		return false, nil
	})
}

type providerErr struct{ msg string }

func (e *providerErr) Error() string { return e.msg }
