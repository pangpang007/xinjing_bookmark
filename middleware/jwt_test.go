package middleware

import (
	"testing"
	"time"
)

func TestJWTGenerateParse(t *testing.T) {
	auth := NewJWTAuth("test-secret", time.Hour)
	token, err := auth.Generate(42)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := auth.Parse(token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.UserID != 42 {
		t.Fatalf("uid=%d", claims.UserID)
	}
}
