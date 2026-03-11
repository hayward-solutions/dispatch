package github

import (
	"encoding/base64"
	"testing"
)

func TestEncryptSecret_ValidKey(t *testing.T) {
	// Generate a valid 32-byte key
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	publicKeyB64 := base64.StdEncoding.EncodeToString(key)

	result, err := encryptSecret(publicKeyB64, "my-secret-value")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == "" {
		t.Fatal("expected non-empty encrypted value")
	}

	// Result should be valid base64
	_, err = base64.StdEncoding.DecodeString(result)
	if err != nil {
		t.Errorf("result is not valid base64: %v", err)
	}
}

func TestEncryptSecret_InvalidBase64Key(t *testing.T) {
	_, err := encryptSecret("not-valid-base64!!!", "secret")
	if err == nil {
		t.Fatal("expected error for invalid base64 key")
	}
}

func TestEncryptSecret_WrongKeyLength(t *testing.T) {
	key := make([]byte, 16)
	publicKeyB64 := base64.StdEncoding.EncodeToString(key)

	_, err := encryptSecret(publicKeyB64, "secret")
	if err == nil {
		t.Fatal("expected error for wrong key length")
	}
}

func TestEncryptSecret_EmptyValue(t *testing.T) {
	key := make([]byte, 32)
	publicKeyB64 := base64.StdEncoding.EncodeToString(key)

	result, err := encryptSecret(publicKeyB64, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Fatal("expected non-empty result even for empty secret")
	}
}

func TestEncryptSecret_DifferentEachTime(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	publicKeyB64 := base64.StdEncoding.EncodeToString(key)

	r1, _ := encryptSecret(publicKeyB64, "same-secret")
	r2, _ := encryptSecret(publicKeyB64, "same-secret")

	if r1 == r2 {
		t.Error("expected different encrypted values due to random nonce")
	}
}
