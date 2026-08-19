package services

import (
	"image/color"
	"testing"
)

func TestFormatCitation(t *testing.T) {
	cases := []struct {
		book, author, want string
	}{
		{"故乡", "鲁迅", "《故乡》 鲁迅"},
		{"《故乡》", "鲁迅", "《故乡》 鲁迅"},
		{"《《故乡》》", "鲁迅", "《故乡》 鲁迅"},
		{"  故乡  ", " 鲁迅 ", "《故乡》 鲁迅"},
	}
	for _, tc := range cases {
		if got := formatCitation(tc.book, tc.author); got != tc.want {
			t.Fatalf("formatCitation(%q,%q)=%q want %q", tc.book, tc.author, got, tc.want)
		}
	}
}

func TestBackgroundForStyle(t *testing.T) {
	want := map[string]color.RGBA{
		"warm":       {R: 0xC4, G: 0x7A, B: 0x4A, A: 255},
		"melancholy": {R: 0x3D, G: 0x4A, B: 0x6B, A: 255},
		"nostalgic":  {R: 0x8B, G: 0x73, B: 0x55, A: 255},
		"hopeful":    {R: 0x5B, G: 0x8F, B: 0x6B, A: 255},
	}
	for style, c := range want {
		got := backgroundForStyle(style)
		if got != c {
			t.Fatalf("%s=%v want %v", style, got, c)
		}
	}
	if backgroundForStyle("unknown") != want["nostalgic"] {
		t.Fatal("invalid style should use nostalgic")
	}
	if backgroundForStyle("") != want["nostalgic"] {
		t.Fatal("empty style should use nostalgic")
	}
	if backgroundForStyle("warm") == backgroundForStyle("hopeful") {
		t.Fatal("warm and hopeful must differ")
	}
	if backgroundForStyle("melancholy") == backgroundForStyle("hopeful") {
		t.Fatal("melancholy must not be green")
	}
}

func TestDecodeAvatarBase64(t *testing.T) {
	const png1x1 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="
	if decodeAvatarBase64("", 72) != nil {
		t.Fatal("empty")
	}
	if decodeAvatarBase64("not-base64", 72) != nil {
		t.Fatal("invalid")
	}
	if img := decodeAvatarBase64(png1x1, 72); img == nil {
		t.Fatal("raw base64")
	}
	if img := decodeAvatarBase64("data:image/png;base64,"+png1x1, 72); img == nil {
		t.Fatal("data uri")
	}
}

func TestResolveAvatarPrefersBase64(t *testing.T) {
	const png1x1 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="
	s := &ImageService{}
	img := s.resolveAvatar(ShareImageInput{
		AvatarBase64: png1x1,
		AvatarURL:    "http://127.0.0.1:1/missing.png",
	}, 48)
	if img == nil {
		t.Fatal("expected base64 avatar")
	}
	if s.resolveAvatar(ShareImageInput{}, 48) != nil {
		t.Fatal("empty should be hollow circle (nil image)")
	}
}
