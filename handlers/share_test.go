package handlers

import "testing"

func TestResolveShareNickname(t *testing.T) {
	cases := []struct {
		req, user, want string
	}{
		{"用户昵称", "库里的名", "用户昵称"},
		{"  用户昵称  ", "库里的名", "用户昵称"},
		{"", "库里的名", "库里的名"},
		{"", "", "心境用户"},
		{"   ", "  ", "心境用户"},
	}
	for _, tc := range cases {
		if got := resolveShareNickname(tc.req, tc.user); got != tc.want {
			t.Fatalf("resolveShareNickname(%q,%q)=%q want %q", tc.req, tc.user, got, tc.want)
		}
	}
}

func TestFirstNonEmpty(t *testing.T) {
	if got := firstNonEmpty("", " https://a.example/x.png ", "https://b"); got != "https://a.example/x.png" {
		t.Fatalf("got %q", got)
	}
	if got := firstNonEmpty("", "  "); got != "" {
		t.Fatalf("got %q", got)
	}
}
