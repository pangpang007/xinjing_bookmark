package services

import "testing"

func TestShareImageFilename(t *testing.T) {
	const name = "550e8400-e29b-41d4-a716-446655440000.jpg"
	cases := []struct {
		in   string
		want string
	}{
		{name, name},
		{"bookmark/" + name, name},
		{"https://r2.soupcircle.xyz/bookmark/" + name, name},
		{"https://api.soupcircle.xyz/bookmark/share-images/" + name, name},
		{"https://r2.soupcircle.xyz/bookmark/550E8400-E29B-41D4-A716-446655440000.JPG", name},
		{"", ""},
		{"https://r2.soupcircle.xyz/bookmark/../secret.jpg", ""},
		{"not-a-uuid.jpg", ""},
	}
	for _, tc := range cases {
		if got := ShareImageFilename(tc.in); got != tc.want {
			t.Fatalf("ShareImageFilename(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}
