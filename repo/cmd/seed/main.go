package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"fleetcommerce/internal/app"
	"fleetcommerce/internal/crypto"
	"fleetcommerce/internal/db"
)

func main() {
	cfg := app.LoadConfig()

	pool, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatal("connect:", err)
	}
	defer pool.Close()

	ctx := context.Background()

	// Hash passwords for demo users
	password := "password123"
	hash, err := crypto.HashPassword(password)
	if err != nil {
		log.Fatal("hash:", err)
	}

	users := []struct {
		Username string
		FullName string
	}{
		{"admin", "System Administrator"},
		{"inventory", "Inventory Manager"},
		{"sales", "Sales Associate"},
		{"auditor", "Auditor User"},
	}

	for _, u := range users {
		_, err := pool.Exec(ctx, `UPDATE users SET password_hash = $1 WHERE username = $2`, hash, u.Username)
		if err != nil {
			log.Printf("update %s: %v", u.Username, err)
		} else {
			fmt.Printf("Updated password for %s\n", u.Username)
		}
	}

	// Encrypt sample phone numbers
	phones := map[string]string{
		"CUST-001": "212-555-5678",
		"CUST-002": "310-555-9012",
		"CUST-003": "312-555-3456",
		"CUST-004": "214-555-7890",
	}

	for code, phone := range phones {
		ciphertext, nonce, err := crypto.Encrypt([]byte(phone), cfg.EncryptionKey)
		if err != nil {
			log.Printf("encrypt %s: %v", code, err)
			continue
		}
		masked := "***-***-" + phone[len(phone)-4:]
		_, err = pool.Exec(ctx, `UPDATE customer_accounts SET contact_phone_encrypted=$1, contact_phone_nonce=$2, contact_phone_masked=$3 WHERE account_code=$4`,
			ciphertext, nonce, masked, code)
		if err != nil {
			log.Printf("update phone %s: %v", code, err)
		} else {
			fmt.Printf("Encrypted phone for %s\n", code)
		}
	}

	fmt.Println("\nSeed complete! Demo credentials:")
	fmt.Println("  admin / password123 (Administrator)")
	fmt.Println("  inventory / password123 (Inventory Manager)")
	fmt.Println("  sales / password123 (Sales Associate)")
	fmt.Println("  auditor / password123 (Auditor)")
}

func init() {
	if os.Getenv("DATABASE_URL") == "" {
		os.Setenv("DATABASE_URL", "postgres://fleet:fleet@localhost:5432/fleetcommerce?sslmode=disable")
	}
}
