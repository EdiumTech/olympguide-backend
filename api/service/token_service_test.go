package service

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

func TestTokensRejectUnexpectedSigningMethod(t *testing.T) {
	key := strings.Repeat("t", 64)
	t.Setenv("TOKEN_SECRET", key)
	svc := NewTokenService()
	token := jwt.NewWithClaims(jwt.SigningMethodHS512, jwt.MapClaims{
		"user_id": 42, "email": "test@example.org", "exp": time.Now().Add(time.Minute).Unix(),
	})
	signed, err := token.SignedString([]byte(key))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ValidateIDToken(signed); err == nil {
		t.Fatal("ID token signed with HS512 was accepted")
	}
	if _, err := svc.ValidateEmailToken(signed); err == nil {
		t.Fatal("email token signed with HS512 was accepted")
	}
}

func TestTokenRoundTrips(t *testing.T) {
	t.Setenv("TOKEN_SECRET", strings.Repeat("t", 64))
	svc := NewTokenService()
	idToken, err := svc.GenerateIDToken(42)
	if err != nil {
		t.Fatal(err)
	}
	id, err := svc.ValidateIDToken(idToken)
	if err != nil || id != 42 {
		t.Fatalf("ID token round trip: %d, %v", id, err)
	}
	emailToken, err := svc.GenerateEmailToken("test@example.org")
	if err != nil {
		t.Fatal(err)
	}
	email, err := svc.ValidateEmailToken(emailToken)
	if err != nil || email != "test@example.org" {
		t.Fatalf("email token round trip: %s, %v", email, err)
	}
}
