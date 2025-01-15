package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestHashPassword(t *testing.T) {
	password := "password"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hash) == 0 {
		t.Fatalf("expected hash to have length > 0")
	}
	want := true
	got := CheckPasswordHash(password, hash)
	if got != want {
		t.Fatalf("expected hash to match password")
	}
}

func TestJWTCreateValidate(t *testing.T) {
	userID := uuid.New()
	tokenSecret := userID.String()
	expiresIn := time.Minute * 2
	token, err := MakeJWT(userID, tokenSecret, expiresIn)
	if err != nil {
		t.Fatalf("unable to make token: %v", err)
	}
	got, err := ValidateJWT(token, tokenSecret)
	if err != nil {
		t.Fatalf("unable to validate token: %v", err)
	}
	if got != userID {
		t.Fatalf("expected userID to match")
	}
}

func TestJWTCreateValidateExpired(t *testing.T) {
	userID := uuid.New()
	tokenSecret := userID.String()
	expiresIn := time.Nanosecond
	token, err := MakeJWT(userID, tokenSecret, expiresIn)
	if err != nil {
		t.Fatalf("unable to make token: %v", err)
	}
	time.Sleep(time.Millisecond)
	_, err = ValidateJWT(token, tokenSecret)
	if err == nil {
		t.Fatalf("expected error for expired token")
	}
}

func TestJWTCreateValidateWrongSecret(t *testing.T) {
	userID := uuid.New()
	tokenSecret := userID.String()
	expiresIn := time.Minute * 2
	token, err := MakeJWT(userID, tokenSecret, expiresIn)
	if err != nil {
		t.Fatalf("unable to make token: %v", err)
	}
	_, err = ValidateJWT(token, "wrongsecret")
	if err == nil {
		t.Fatalf("expected error for wrong secret")
	}
}
