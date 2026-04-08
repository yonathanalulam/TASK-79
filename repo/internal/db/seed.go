package db

import (
	"context"
	"log/slog"

	"fleetcommerce/internal/crypto"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RunSeeds updates seeded users with real Argon2id password hashes and
// encrypts sample phone numbers. It is idempotent — safe to call on every boot.
func RunSeeds(ctx context.Context, pool *pgxpool.Pool, encryptionKey string) {
	seedPasswords(ctx, pool)
	seedPhoneEncryption(ctx, pool, encryptionKey)
}

func seedPasswords(ctx context.Context, pool *pgxpool.Pool) {
	// Check if admin already has a working hash (salt is random, so a real
	// hash will always be longer than the placeholder used in the migration).
	var currentHash string
	err := pool.QueryRow(ctx, `SELECT password_hash FROM users WHERE username='admin'`).Scan(&currentHash)
	if err != nil {
		return // no users yet — migrations haven't seeded
	}

	// Quick probe: try to verify against the known seed password.
	// If it already works, skip re-hashing.
	if ok, _ := crypto.VerifyPassword("password123", currentHash); ok {
		slog.Info("seed passwords already set, skipping")
		return
	}

	hash, err := crypto.HashPassword("password123")
	if err != nil {
		slog.Error("seed: hash password", "error", err)
		return
	}

	usernames := []string{"admin", "inventory", "sales", "auditor"}
	for _, u := range usernames {
		if _, err := pool.Exec(ctx, `UPDATE users SET password_hash=$1 WHERE username=$2`, hash, u); err != nil {
			slog.Error("seed: update password", "user", u, "error", err)
		}
	}
	slog.Info("seed passwords updated for demo users")
}

func seedPhoneEncryption(ctx context.Context, pool *pgxpool.Pool, encryptionKey string) {
	// Skip if already encrypted
	var encrypted []byte
	err := pool.QueryRow(ctx, `SELECT contact_phone_encrypted FROM customer_accounts WHERE account_code='CUST-001'`).Scan(&encrypted)
	if err == nil && len(encrypted) > 0 {
		slog.Info("seed phone encryption already set, skipping")
		return
	}

	phones := map[string]string{
		"CUST-001": "212-555-5678",
		"CUST-002": "310-555-9012",
		"CUST-003": "312-555-3456",
		"CUST-004": "214-555-7890",
	}

	for code, phone := range phones {
		ciphertext, nonce, err := crypto.Encrypt([]byte(phone), encryptionKey)
		if err != nil {
			slog.Error("seed: encrypt phone", "account", code, "error", err)
			continue
		}
		masked := "***-***-" + phone[len(phone)-4:]
		pool.Exec(ctx, `UPDATE customer_accounts SET contact_phone_encrypted=$1, contact_phone_nonce=$2, contact_phone_masked=$3 WHERE account_code=$4`,
			ciphertext, nonce, masked, code)
	}
	slog.Info("seed phone encryption applied")
}
