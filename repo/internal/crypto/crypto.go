package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"

	"golang.org/x/crypto/argon2"
)

// Argon2id password hashing

type Argon2Params struct {
	Memory      uint32
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

var DefaultArgon2Params = &Argon2Params{
	Memory:      64 * 1024,
	Iterations:  3,
	Parallelism: 2,
	SaltLength:  16,
	KeyLength:   32,
}

func HashPassword(password string) (string, error) {
	p := DefaultArgon2Params
	salt := make([]byte, p.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	hash := argon2.IDKey([]byte(password), salt, p.Iterations, p.Memory, p.Parallelism, p.KeyLength)

	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		p.Memory, p.Iterations, p.Parallelism,
		hex.EncodeToString(salt),
		hex.EncodeToString(hash),
	), nil
}

func VerifyPassword(password, encoded string) (bool, error) {
	var memory uint32
	var iterations uint32
	var parallelism uint8
	var saltHex, hashHex string

	_, err := fmt.Sscanf(encoded, "$argon2id$v=19$m=%d,t=%d,p=%d$%s", &memory, &iterations, &parallelism, &saltHex)
	if err != nil {
		return false, fmt.Errorf("invalid hash format: %w", err)
	}

	// Split saltHex which contains "salt$hash"
	parts := splitDollar(saltHex)
	if len(parts) != 2 {
		return false, fmt.Errorf("invalid hash format: expected salt$hash")
	}
	saltHex = parts[0]
	hashHex = parts[1]

	salt, err := hex.DecodeString(saltHex)
	if err != nil {
		return false, err
	}
	expectedHash, err := hex.DecodeString(hashHex)
	if err != nil {
		return false, err
	}

	hash := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, uint32(len(expectedHash)))

	if len(hash) != len(expectedHash) {
		return false, nil
	}
	// constant time compare
	var diff byte
	for i := range hash {
		diff |= hash[i] ^ expectedHash[i]
	}
	return diff == 0, nil
}

func splitDollar(s string) []string {
	var parts []string
	start := 0
	for i, c := range s {
		if c == '$' {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}

// AES-GCM encryption for sensitive fields

func Encrypt(plaintext []byte, keyHex string) (ciphertext, nonce []byte, err error) {
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, nil, fmt.Errorf("decode key: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}

	nonce = make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}

	ciphertext = gcm.Seal(nil, nonce, plaintext, nil)
	return ciphertext, nonce, nil
}

func Decrypt(ciphertext, nonce []byte, keyHex string) ([]byte, error) {
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, fmt.Errorf("decode key: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}
