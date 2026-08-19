package config

import "testing"

func TestGetEnvInt(t *testing.T) {
	t.Setenv("INTERPRET_DAILY_LIMIT", "0")
	if getEnvInt("INTERPRET_DAILY_LIMIT", 3) != 0 {
		t.Fatal("0 means unlimited")
	}
	t.Setenv("INTERPRET_DAILY_LIMIT", "3")
	if getEnvInt("INTERPRET_DAILY_LIMIT", 1) != 3 {
		t.Fatal("explicit 3")
	}
	t.Setenv("INTERPRET_DAILY_LIMIT", "")
	if getEnvInt("INTERPRET_DAILY_LIMIT", 3) != 3 {
		t.Fatal("empty uses fallback")
	}
	t.Setenv("INTERPRET_DAILY_LIMIT", "-1")
	if getEnvInt("INTERPRET_DAILY_LIMIT", 3) != 0 {
		t.Fatal("negative becomes 0")
	}
}
