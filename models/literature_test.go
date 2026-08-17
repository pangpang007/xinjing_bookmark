package models

import "testing"

func TestNormalizeStyle(t *testing.T) {
	if NormalizeStyle("warm") != "warm" {
		t.Fatal("warm")
	}
	if NormalizeStyle("unknown") != "nostalgic" {
		t.Fatal("fallback")
	}
}
