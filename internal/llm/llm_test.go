package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAnthropicStreamToolUse(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "k" || r.Header.Get("anthropic-version") == "" {
			t.Errorf("missing auth headers")
		}
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		w.Header().Set("Content-Type", "text/event-stream")
		events := []string{
			`{"type":"message_start"}`,
			`{"type":"content_block_start","index":0,"content_block":{"type":"thinking"}}`,
			`{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"hmm"}}`,
			`{"type":"content_block_start","index":1,"content_block":{"type":"text"}}`,
			`{"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"Checking."}}`,
			`{"type":"content_block_start","index":2,"content_block":{"type":"tool_use","id":"tu_1","name":"search"}}`,
			`{"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"{\"q\":"}}`,
			`{"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"\"go\"}"}}`,
			`{"type":"message_stop"}`,
		}
		for _, e := range events {
			fmt.Fprintf(w, "event: x\ndata: %s\n\n", e)
		}
	}))
	defer srv.Close()

	c, _ := New("anthropic", srv.URL, "k")
	var text strings.Builder
	res, err := c.Stream(context.Background(), Request{
		Model:  "claude-x",
		System: "be brief",
		Tools:  []Tool{{Name: "search", Description: "d"}},
		Messages: []Message{
			{Role: "user", Content: "hi"},
			{Role: "assistant", Content: "a", ToolCalls: []ToolCall{{ID: "t0", Name: "search", Args: `{"q":"x"}`}, {ID: "t1", Name: "search", Args: ""}}},
			{Role: "tool", ToolCallID: "t0", Content: "r0"},
			{Role: "tool", ToolCallID: "t1", Content: "r1"},
		},
	}, func(s string) { text.WriteString(s) })
	if err != nil {
		t.Fatal(err)
	}
	if text.String() != "<think>hmm</think>\n\nChecking." {
		t.Errorf("text = %q", text.String())
	}
	if len(res.ToolCalls) != 1 || res.ToolCalls[0].ID != "tu_1" || res.ToolCalls[0].Args != `{"q":"go"}` {
		t.Errorf("tool calls = %+v", res.ToolCalls)
	}
	msgs := got["messages"].([]any)
	if len(msgs) != 3 {
		t.Fatalf("want 3 messages (tool results merged), got %d: %v", len(msgs), msgs)
	}
	results := msgs[2].(map[string]any)["content"].([]any)
	if len(results) != 2 || results[1].(map[string]any)["tool_use_id"] != "t1" {
		t.Errorf("tool results not merged: %v", results)
	}
	asst := msgs[1].(map[string]any)["content"].([]any)
	if in := asst[2].(map[string]any)["input"]; fmt.Sprint(in) != "map[]" {
		t.Errorf("empty args should become {}: %v", in)
	}
	if got["system"] != "be brief" || got["tools"] == nil {
		t.Errorf("system/tools missing: %v", got)
	}
}

func TestOpenAIStreamToolCalls(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		for _, d := range []string{
			`{"choices":[{"delta":{"reasoning":"r"}}]}`,
			`{"choices":[{"delta":{"content":"ok"}}]}`,
			`{"choices":[{"delta":{"tool_calls":[{"index":1,"id":"b","function":{"name":"two","arguments":"{}"}}]}}]}`,
			`{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"a","function":{"name":"one","arguments":"{\"x\""}}]}}]}`,
			`{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":":1}"}}]}}]}`,
		} {
			fmt.Fprintf(w, "data: %s\n\n", d)
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	c, _ := New("openai", srv.URL, "")
	var text strings.Builder
	res, err := c.Stream(context.Background(), Request{Model: "m", Messages: []Message{
		{Role: "user", Content: "hi", Images: []string{"data:image/png;base64,AAAA"}},
		{Role: "assistant", ToolCalls: []ToolCall{{ID: "z", Name: "one", Args: "{}"}}},
		{Role: "tool", ToolCallID: "z", Content: "res"},
	}}, func(s string) { text.WriteString(s) })
	if err != nil {
		t.Fatal(err)
	}
	if text.String() != "<think>r</think>\n\nok" {
		t.Errorf("text = %q", text.String())
	}
	if len(res.ToolCalls) != 2 || res.ToolCalls[0].Name != "one" || res.ToolCalls[0].Args != `{"x":1}` || res.ToolCalls[1].ID != "b" {
		t.Errorf("tool calls = %+v", res.ToolCalls)
	}
	msgs := got["messages"].([]any)
	if m := msgs[1].(map[string]any); m["content"] != nil || m["tool_calls"] == nil {
		t.Errorf("assistant tool-call message wrong: %v", m)
	}
	if m := msgs[2].(map[string]any); m["role"] != "tool" || m["tool_call_id"] != "z" {
		t.Errorf("tool message wrong: %v", m)
	}
}
