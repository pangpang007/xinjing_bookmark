package utils

import "testing"

func TestExtractJSON(t *testing.T) {
	raw := "```json\n{\"a\":1}\n```"
	got := ExtractJSON(raw)
	if got != `{"a":1}` {
		t.Fatalf("got %q", got)
	}
}
