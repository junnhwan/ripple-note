package auth_test

import (
	"testing"
	"time"

	"ripple-note/internal/auth"
)

func TestJWTManagerIssuesAndParsesToken(t *testing.T) {
	t.Parallel()

	manager := auth.NewJWTManager(auth.JWTConfig{
		Secret: "test-secret",
		Issuer: "ripple-note-test",
		TTL:    time.Hour,
	})

	token, err := manager.Issue(auth.UserClaims{
		UserID: 42,
		Role:   "admin",
	})
	if err != nil {
		t.Fatalf("expected token issue to succeed, got error: %v", err)
	}
	if token == "" {
		t.Fatal("expected token to be present")
	}

	claims, err := manager.Parse(token)
	if err != nil {
		t.Fatalf("expected token parse to succeed, got error: %v", err)
	}
	if claims.UserID != 42 {
		t.Fatalf("expected user id 42, got %d", claims.UserID)
	}
	if claims.Role != "admin" {
		t.Fatalf("expected role admin, got %q", claims.Role)
	}
}

func TestJWTManagerRejectsTokenSignedWithDifferentSecret(t *testing.T) {
	t.Parallel()

	issuer := auth.NewJWTManager(auth.JWTConfig{
		Secret: "secret-a",
		Issuer: "ripple-note-test",
		TTL:    time.Hour,
	})
	parser := auth.NewJWTManager(auth.JWTConfig{
		Secret: "secret-b",
		Issuer: "ripple-note-test",
		TTL:    time.Hour,
	})

	token, err := issuer.Issue(auth.UserClaims{UserID: 1, Role: "user"})
	if err != nil {
		t.Fatalf("expected token issue to succeed, got error: %v", err)
	}

	if _, err := parser.Parse(token); err == nil {
		t.Fatal("expected parser to reject token signed by a different secret")
	}
}
