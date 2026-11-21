package server

import (
	"strings"
	"testing"
)

func TestBuildChatReparentsAndPicksLeaf(t *testing.T) {
	ic := importedChat{title: "t", current: "a2", nodes: []node{
		{id: "root"},
		{id: "sys", parent: "root", role: "system", content: ""},
		{id: "u1", parent: "sys", role: "user", content: "q1", created: 1},
		{id: "a1", parent: "u1", role: "assistant", content: "r1", created: 2},
		{id: "u2", parent: "sys", role: "user", content: "q2", created: 3},
		{id: "a2", parent: "u2", role: "assistant", content: "r2", created: 4},
		{id: "tool", parent: "a2", role: "tool", content: "x", created: 5},
	}}
	c, msgs := buildChat(ic)
	if len(msgs) != 4 {
		t.Fatalf("want 4 messages, got %d", len(msgs))
	}
	byContent := map[string]string{}
	ids := map[string]string{}
	for _, m := range msgs {
		ids[m.ID] = m.Content
		byContent[m.Content] = m.ParentID
	}
	if byContent["q1"] != "" || byContent["q2"] != "" {
		t.Errorf("user messages should be roots after dropping system node: %v", byContent)
	}
	if ids[byContent["r2"]] != "q2" {
		t.Errorf("r2 should hang off q2")
	}
	if ids[c.LeafID] != "r2" {
		t.Errorf("leaf = %q, want r2", ids[c.LeafID])
	}
}

func TestHistoryTextSummarizesTools(t *testing.T) {
	in := "<think>secret</think>\n\nLet me look.\n\n<tool name=\"search\" server=\"S\" status=\"ok\">\n" +
		`{"args":{"q":"x"},"result":"found </tool> it"}` + "\n</tool>\n\nDone."
	out := historyText(in)
	if strings.Contains(out, "secret") || strings.Contains(out, "<tool") {
		t.Errorf("not cleaned: %q", out)
	}
	if !strings.Contains(out, `[Called tool search with {"q":"x"}. Result: found </tool> it]`) || !strings.HasSuffix(out, "Done.") {
		t.Errorf("unexpected: %q", out)
	}
}

func TestParseImportDetectsFormats(t *testing.T) {
	cases := map[string]string{
		`{"format":"litechat","chats":[]}`:                   "litechat",
		`[{"title":"x","mapping":{}}]`:                       "chatgpt",
		`[{"title":"x","chat":{"history":{"messages":{}}}}]`: "open-webui",
	}
	for in, want := range cases {
		_, got, err := parseImport([]byte(in))
		if err != nil || got != want {
			t.Errorf("%s: got %q, %v; want %q", in, got, err, want)
		}
	}
	if _, _, err := parseImport([]byte(`{"foo":1}`)); err == nil {
		t.Error("expected error for unknown format")
	}
}
