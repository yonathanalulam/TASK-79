package crypto

import (
	"testing"
)

func TestHashAndVerifyPassword(t *testing.T) {
	password := "password123"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	if hash == "" {
		t.Fatal("hash should not be empty")
	}

	// Verify correct password
	valid, err := VerifyPassword(password, hash)
	if err != nil {
		t.Fatalf("VerifyPassword failed: %v", err)
	}
	if !valid {
		t.Error("expected valid password verification")
	}

	// Verify wrong password
	valid, err = VerifyPassword("wrongpassword", hash)
	if err != nil {
		t.Fatalf("VerifyPassword failed: %v", err)
	}
	if valid {
		t.Error("expected invalid password verification")
	}
}

func TestHashPasswordUniqueSalts(t *testing.T) {
	password := "samepassword"
	hash1, _ := HashPassword(password)
	hash2, _ := HashPassword(password)

	if hash1 == hash2 {
		t.Error("hashes with different salts should not be equal")
	}

	// Both should still verify
	valid1, _ := VerifyPassword(password, hash1)
	valid2, _ := VerifyPassword(password, hash2)
	if !valid1 || !valid2 {
		t.Error("both hashes should verify correctly")
	}
}

func TestEncryptDecrypt(t *testing.T) {
	keyHex := "0123456789abcdef0123456789abcdef" // 16 bytes = AES-128
	plaintext := []byte("sensitive phone number: 555-1234")

	ciphertext, nonce, err := Encrypt(plaintext, keyHex)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	if len(ciphertext) == 0 {
		t.Fatal("ciphertext should not be empty")
	}
	if len(nonce) == 0 {
		t.Fatal("nonce should not be empty")
	}

	// Decrypt
	decrypted, err := Decrypt(ciphertext, nonce, keyHex)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("decrypted text mismatch: got %q, want %q", decrypted, plaintext)
	}
}

func TestDecryptWithWrongKey(t *testing.T) {
	keyHex := "0123456789abcdef0123456789abcdef"
	wrongKey := "abcdef0123456789abcdef0123456789"
	plaintext := []byte("secret data")

	ciphertext, nonce, _ := Encrypt(plaintext, keyHex)

	_, err := Decrypt(ciphertext, nonce, wrongKey)
	if err == nil {
		t.Error("expected error when decrypting with wrong key")
	}
}

func TestEncryptWithInvalidKey(t *testing.T) {
	_, _, err := Encrypt([]byte("test"), "invalidhex")
	if err == nil {
		t.Error("expected error with invalid hex key")
	}
}
