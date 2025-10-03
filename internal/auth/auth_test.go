package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestHashPassword(t *testing.T) {
	password := "testpassword123"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if hash == "" {
		t.Fatal("HashPassword returned empty hash")
	}
	if hash == password {
		t.Fatal("HashPassword returned plaintext password")
	}
}

func TestCheckPasswordHash(t *testing.T) {
	password := "testpassword123"
	hash, _ := HashPassword(password)

	match, err := CheckPasswordHash(password, hash)
	if err != nil {
		t.Fatalf("CheckPasswordHash failed: %v", err)
	}
	if !match {
		t.Fatal("CheckPasswordHash should match correct password")
	}

	wrongMatch, err := CheckPasswordHash("wrongpassword", hash)
	if err != nil {
		t.Fatalf("CheckPasswordHash failed: %v", err)
	}
	if wrongMatch {
		t.Fatal("CheckPasswordHash should not match incorrect password")
	}
}

func TestMakeJWT(t *testing.T) {
	userID := uuid.New()
	secret := "test-secret"
	expiresIn := time.Hour

	token, err := MakeJWT(userID, secret, expiresIn)
	if err != nil {
		t.Fatalf("MakeJWT failed: %v", err)
	}
	if token == "" {
		t.Fatal("MakeJWT returned empty token")
	}
}

func TestValidateJWT(t *testing.T) {
	userID := uuid.New()
	secret := "test-secret"
	expiresIn := time.Hour

	token, _ := MakeJWT(userID, secret, expiresIn)

	validatedID, err := ValidateJWT(token, secret)
	if err != nil {
		t.Fatalf("ValidateJWT failed: %v", err)
	}
	if validatedID != userID {
		t.Fatalf("ValidateJWT returned wrong ID: got %v, want %v", validatedID, userID)
	}
}

func TestValidateJWTExpired(t *testing.T) {
	userID := uuid.New()
	secret := "test-secret"
	expiresIn := -time.Hour

	token, _ := MakeJWT(userID, secret, expiresIn)

	_, err := ValidateJWT(token, secret)
	if err == nil {
		t.Fatal("ValidateJWT should reject expired token")
	}
}

func TestValidateJWTWrongSecret(t *testing.T) {
	userID := uuid.New()
	secret := "test-secret"
	wrongSecret := "wrong-secret"
	expiresIn := time.Hour

	token, _ := MakeJWT(userID, secret, expiresIn)

	_, err := ValidateJWT(token, wrongSecret)
	if err == nil {
		t.Fatal("ValidateJWT should reject token with wrong secret")
	}
}
