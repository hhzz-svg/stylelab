package auth

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"stylelab/internal/ids"
	"stylelab/internal/store"
)

var (
	ErrInvalid      = errors.New("invalid")
	ErrConflict     = errors.New("conflict")
	ErrUnauthorized = errors.New("unauthorized")
)

const sessionTTL = 14 * 24 * time.Hour

type Service struct {
	st *store.Store
}

func NewService(st *store.Store) *Service {
	return &Service{st: st}
}

func (s *Service) Register(ctx context.Context, email, password string) (string, error) {
	if !strings.Contains(email, "@") {
		return "", ErrInvalid
	}
	if utf8.RuneCountInString(password) < 8 {
		return "", ErrInvalid
	}

	var existing string
	err := s.st.DB().QueryRowContext(ctx, `SELECT id FROM users WHERE email = ?`, email).Scan(&existing)
	if err == nil {
		return "", ErrConflict
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}

	hash, err := HashPassword(password)
	if err != nil {
		return "", err
	}
	userID := ids.New("usr_")
	_, err = s.st.DB().ExecContext(
		ctx,
		`INSERT INTO users (id, email, password_hash, created_at) VALUES (?, ?, ?, ?)`,
		userID,
		email,
		hash,
		time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		if isUniqueEmail(err) {
			return "", ErrConflict
		}
		return "", err
	}
	return userID, nil
}

func (s *Service) Login(ctx context.Context, email, password string) (string, string, error) {
	var userID, hash string
	err := s.st.DB().QueryRowContext(ctx, `SELECT id, password_hash FROM users WHERE email = ?`, email).Scan(&userID, &hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", ErrUnauthorized
		}
		return "", "", err
	}
	if !CheckPassword(hash, password) {
		return "", "", ErrUnauthorized
	}

	raw, err := newRawToken()
	if err != nil {
		return "", "", err
	}
	_, err = s.st.DB().ExecContext(
		ctx,
		`INSERT INTO sessions (id, user_id, token_hash, expires_at) VALUES (?, ?, ?, ?)`,
		ids.New("ses_"),
		userID,
		HashToken(raw),
		time.Now().UTC().Add(sessionTTL).Format(time.RFC3339),
	)
	if err != nil {
		return "", "", err
	}
	return raw, userID, nil
}

func (s *Service) Logout(ctx context.Context, rawToken string) error {
	_, err := s.st.DB().ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, HashToken(rawToken))
	return err
}

func (s *Service) UserIDFromToken(ctx context.Context, rawToken string) (string, error) {
	var userID, expiresAt string
	err := s.st.DB().QueryRowContext(
		ctx,
		`SELECT user_id, expires_at FROM sessions WHERE token_hash = ?`,
		HashToken(rawToken),
	).Scan(&userID, &expiresAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrUnauthorized
		}
		return "", err
	}
	exp, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return "", ErrUnauthorized
	}
	if !exp.After(time.Now().UTC()) {
		return "", ErrUnauthorized
	}
	return userID, nil
}

func isUniqueEmail(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed: users.email")
}
