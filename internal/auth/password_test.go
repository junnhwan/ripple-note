package auth_test

import (
	"testing"

	"ripple-note/internal/auth"
)

func TestPasswordHasherHashesAndVerifiesPassword(t *testing.T) {
	t.Parallel()

	hasher := auth.NewBcryptPasswordHasher()

	hash, err := hasher.Hash("secret123")
	if err != nil {
		t.Fatalf("expected hash to succeed, got error: %v", err)
	}
	if hash == "" {
		t.Fatal("expected hash to be present")
	}
	if hash == "secret123" {
		t.Fatal("expected hash to differ from raw password")
	}

	if err := hasher.Compare(hash, "secret123"); err != nil {
		t.Fatalf("expected password to verify, got error: %v", err)
	}
	if err := hasher.Compare(hash, "wrong-password"); err == nil {
		t.Fatal("expected wrong password to fail verification")
	}
}
