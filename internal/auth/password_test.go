package auth_test

import (
	"testing"

	"stylelab/internal/auth"
)

func TestHashPasswordAndCheck(t *testing.T) {
	hash, err := auth.HashPassword("correct-horse")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "" || hash == "correct-horse" {
		t.Fatalf("expected bcrypt hash, got %q", hash)
	}
	if !auth.CheckPassword(hash, "correct-horse") {
		t.Fatal("expected matching password to succeed")
	}
	if auth.CheckPassword(hash, "wrong-password") {
		t.Fatal("expected wrong password to fail")
	}
}
