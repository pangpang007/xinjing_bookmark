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

func TestNewQuota(t *testing.T) {
	if NewQuota(1, 0) != nil {
		t.Fatal("unlimited")
	}
	q := NewQuota(2, 3)
	if q == nil || q.Used != 2 || q.Limit != 3 || q.Remaining != 1 {
		t.Fatalf("%+v", q)
	}
	q = NewQuota(5, 3)
	if q.Remaining != 0 {
		t.Fatalf("remaining=%d", q.Remaining)
	}
}
