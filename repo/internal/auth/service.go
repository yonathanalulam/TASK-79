package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"fleetcommerce/internal/crypto"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrAccountDisabled    = errors.New("account is disabled")
	ErrSessionExpired     = errors.New("session expired")
	ErrSessionNotFound    = errors.New("session not found")
)

type User struct {
	ID          int
	Username    string
	FullName    string
	IsActive    bool
	CreatedAt   time.Time
	LastLoginAt *time.Time
}

type Session struct {
	ID        string
	UserID    int
	ExpiresAt time.Time
}

type Service struct {
	pool           *pgxpool.Pool
	sessionMaxAge  time.Duration
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{
		pool:          pool,
		sessionMaxAge: 24 * time.Hour,
	}
}

func (s *Service) Login(ctx context.Context, username, password string) (*Session, *User, error) {
	var user struct {
		ID           int
		Username     string
		PasswordHash string
		FullName     string
		IsActive     bool
	}

	err := s.pool.QueryRow(ctx,
		`SELECT id, username, password_hash, full_name, is_active FROM users WHERE username = $1`,
		username,
	).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.FullName, &user.IsActive)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, ErrInvalidCredentials
		}
		return nil, nil, fmt.Errorf("query user: %w", err)
	}

	if !user.IsActive {
		return nil, nil, ErrAccountDisabled
	}

	valid, err := crypto.VerifyPassword(password, user.PasswordHash)
	if err != nil || !valid {
		return nil, nil, ErrInvalidCredentials
	}

	// Update last login
	_, _ = s.pool.Exec(ctx, `UPDATE users SET last_login_at = NOW() WHERE id = $1`, user.ID)

	// Create session
	sessionID, err := generateSessionID()
	if err != nil {
		return nil, nil, fmt.Errorf("generate session: %w", err)
	}

	expiresAt := time.Now().Add(s.sessionMaxAge)
	_, err = s.pool.Exec(ctx,
		`INSERT INTO sessions (id, user_id, expires_at) VALUES ($1, $2, $3)`,
		sessionID, user.ID, expiresAt,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create session: %w", err)
	}

	return &Session{
			ID:        sessionID,
			UserID:    user.ID,
			ExpiresAt: expiresAt,
		}, &User{
			ID:       user.ID,
			Username: user.Username,
			FullName: user.FullName,
			IsActive: user.IsActive,
		}, nil
}

func (s *Service) ValidateSession(ctx context.Context, sessionID string) (*User, error) {
	var session struct {
		UserID    int
		ExpiresAt time.Time
	}

	err := s.pool.QueryRow(ctx,
		`SELECT user_id, expires_at FROM sessions WHERE id = $1`,
		sessionID,
	).Scan(&session.UserID, &session.ExpiresAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}

	if time.Now().After(session.ExpiresAt) {
		_ = s.DestroySession(ctx, sessionID)
		return nil, ErrSessionExpired
	}

	var user User
	err = s.pool.QueryRow(ctx,
		`SELECT id, username, full_name, is_active, last_login_at FROM users WHERE id = $1`,
		session.UserID,
	).Scan(&user.ID, &user.Username, &user.FullName, &user.IsActive, &user.LastLoginAt)

	if err != nil {
		return nil, err
	}

	if !user.IsActive {
		_ = s.DestroySession(ctx, sessionID)
		return nil, ErrAccountDisabled
	}

	return &user, nil
}

func (s *Service) DestroySession(ctx context.Context, sessionID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE id = $1`, sessionID)
	return err
}

func (s *Service) CleanExpiredSessions(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at < NOW()`)
	return err
}

func generateSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
