package auth

import (
	"encoding/base64"
	"testing"
)

func validKey() []byte {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	return key
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	key := validKey()
	plaintext := "ghp_test_token_12345"

	encrypted, err := EncryptToken(plaintext, key)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	if encrypted == plaintext {
		t.Fatal("encrypted text should differ from plaintext")
	}

	decrypted, err := DecryptToken(encrypted, key)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("expected %q, got %q", plaintext, decrypted)
	}
}

func TestEncryptDecrypt_EmptyString(t *testing.T) {
	key := validKey()

	encrypted, err := EncryptToken("", key)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	decrypted, err := DecryptToken(encrypted, key)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}

	if decrypted != "" {
		t.Errorf("expected empty string, got %q", decrypted)
	}
}

func TestEncrypt_InvalidKeySize(t *testing.T) {
	// AES accepts 16, 24, or 32 byte keys. Test with an invalid size.
	badKey := make([]byte, 15)
	_, err := EncryptToken("test", badKey)
	if err == nil {
		t.Fatal("expected error for 15-byte key")
	}
}

func TestDecrypt_InvalidBase64(t *testing.T) {
	key := validKey()
	_, err := DecryptToken("not-valid-base64!!!", key)
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
}

func TestDecrypt_TamperedCiphertext(t *testing.T) {
	key := validKey()

	encrypted, err := EncryptToken("secret", key)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	// Decode, tamper, re-encode
	raw, _ := base64.StdEncoding.DecodeString(encrypted)
	raw[len(raw)-1] ^= 0xFF
	tampered := base64.StdEncoding.EncodeToString(raw)

	_, err = DecryptToken(tampered, key)
	if err == nil {
		t.Fatal("expected error for tampered ciphertext")
	}
}

func TestDecrypt_ShortCiphertext(t *testing.T) {
	key := validKey()
	// Encode just a few bytes (shorter than nonce size)
	short := base64.StdEncoding.EncodeToString([]byte{1, 2, 3})

	_, err := DecryptToken(short, key)
	if err == nil {
		t.Fatal("expected error for short ciphertext")
	}
}

func TestDecrypt_WrongKey(t *testing.T) {
	key1 := validKey()
	key2 := make([]byte, 32)
	for i := range key2 {
		key2[i] = byte(i + 100)
	}

	encrypted, err := EncryptToken("secret", key1)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	_, err = DecryptToken(encrypted, key2)
	if err == nil {
		t.Fatal("expected error for wrong key")
	}
}

func TestEncrypt_DifferentCiphertextEachTime(t *testing.T) {
	key := validKey()

	e1, _ := EncryptToken("same", key)
	e2, _ := EncryptToken("same", key)

	if e1 == e2 {
		t.Error("expected different ciphertexts due to random nonce")
	}
}
