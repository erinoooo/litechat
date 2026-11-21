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
	var buf []byte
	if body != nil {
		var err error
		if buf, err = json.Marshal(body); err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, o.base+path, bytes.NewReader(buf))
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

func (o *openAI) Embed(ctx context.Context, model string, inputs []string) ([][]float32, error) {
	resp, err := o.do(ctx, "POST", "/embeddings", map[string]any{"model": model, "input": inputs}, embedHTTP)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var body struct {
		Data []struct {
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	out := make([][]float32, len(inputs))
	for i, d := range body.Data {
		idx := d.Index
		if idx < 0 || idx >= len(out) {
			idx = i
		}
		out[idx] = d.Embedding
	}
	for _, v := range out {
		if len(v) == 0 {
			return nil, &providerErr{"embeddings response was missing vectors"}
		}
	}
	return out, nil
}

type oaFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type oaToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function oaFunc `json:"function"`
}

func (o *openAI) Stream(ctx context.Context, r Request, onDelta func(string)) (Result, error) {
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
		switch {
		case m.Role == "tool":
			msgs = append(msgs, map[string]any{"role": "tool", "tool_call_id": m.ToolCallID, "content": m.Content})
		case len(m.ToolCalls) > 0:
			calls := make([]oaToolCall, len(m.ToolCalls))
			for i, c := range m.ToolCalls {
				calls[i] = oaToolCall{ID: c.ID, Type: "function", Function: oaFunc{Name: c.Name, Arguments: argsOrEmpty(c.Args)}}
			}
			msg := map[string]any{"role": "assistant", "tool_calls": calls, "content": nil}
			if m.Content != "" {
				msg["content"] = m.Content
			}
			msgs = append(msgs, msg)
		case len(m.Images) == 0:
			msgs = append(msgs, map[string]any{"role": m.Role, "content": m.Content})
		default:
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
	}

	body := map[string]any{"model": r.Model, "messages": msgs, "stream": true}
	if len(r.Tools) > 0 {
		tools := make([]map[string]any, len(r.Tools))
		for i, t := range r.Tools {
			tools[i] = map[string]any{"type": "function", "function": map[string]any{
				"name": t.Name, "description": t.Description, "parameters": schemaOrEmpty(t.Schema),
			}}
		}
		body["tools"] = tools
	}

	resp, err := o.do(ctx, "POST", "/chat/completions", body, streamHTTP)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()

	tw := &thinkWrapper{emit: onDelta}
	calls := map[int]*ToolCall{}
	truncated := false
	err = sseData(resp.Body, func(_, data string) (bool, error) {
		if data == "[DONE]" {
			return true, nil
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content          string `json:"content"`
					Reasoning        string `json:"reasoning"`
					ReasoningContent string `json:"reasoning_content"`
					ToolCalls        []struct {
						Index    int    `json:"index"`
						ID       string `json:"id"`
						Function oaFunc `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
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
			if c.FinishReason == "length" {
				truncated = true
			}
			tw.reasoning(c.Delta.ReasoningContent)
			tw.reasoning(c.Delta.Reasoning)
			tw.text(c.Delta.Content)
			for _, tc := range c.Delta.ToolCalls {
				call := calls[tc.Index]
				if call == nil {
					call = &ToolCall{}
					calls[tc.Index] = call
				}
				if tc.ID != "" {
					call.ID = tc.ID
				}
				call.Name += tc.Function.Name
				call.Args += tc.Function.Arguments
			}
		}
		return false, nil
	})
	tw.close()

	res := Result{Truncated: truncated}
	idx := make([]int, 0, len(calls))
	for i := range calls {
		idx = append(idx, i)
	}
	sort.Ints(idx)
	for _, i := range idx {
		c := calls[i]
		if c.Name == "" {
			continue
		}
		if c.ID == "" {
			c.ID = "call_" + c.Name
		}
		c.Args = argsOrEmpty(c.Args)
		res.ToolCalls = append(res.ToolCalls, *c)
	}
	return res, err
}
