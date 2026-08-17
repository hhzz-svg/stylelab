package auth_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"stylelab/internal/auth"
	"stylelab/internal/ids"
	"stylelab/internal/store"
)

func newTestService(t *testing.T) (*auth.Service, *store.Store) {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() {
		if err := st.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	return auth.NewService(st), st
}

func TestRegisterLoginAndToken(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	userID, err := svc.Register(ctx, "reader@example.com", "password1")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if !ids.Valid(userID, "usr_") {
		t.Fatalf("user id: %s", userID)
	}

	token, loginID, err := svc.Login(ctx, "reader@example.com", "password1")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if loginID != userID {
		t.Fatalf("login user id: got %s want %s", loginID, userID)
	}
	if len(token) != 64 {
		t.Fatalf("token length: got %d want 64", len(token))
	}
	for _, c := range token {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			t.Fatalf("token is not hex: %s", token)
		}
	}

	got, err := svc.UserIDFromToken(ctx, token)
	if err != nil {
		t.Fatalf("UserIDFromToken: %v", err)
	}
	if got != userID {
		t.Fatalf("UserIDFromToken: got %s want %s", got, userID)
	}
}

func TestRegisterValidation(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	cases := []struct {
		name     string
		email    string
		password string
		want     error
		setup    func()
	}{
		{name: "missing at", email: "not-an-email", password: "password1", want: auth.ErrInvalid},
		{name: "short password", email: "ok@example.com", password: "short", want: auth.ErrInvalid},
		{name: "short unicode password", email: "ok@example.com", password: "密码密码", want: auth.ErrInvalid},
		{
			name:     "duplicate email",
			email:    "dup@example.com",
			password: "password1",
			want:     auth.ErrConflict,
			setup: func() {
				if _, err := svc.Register(ctx, "dup@example.com", "password1"); err != nil {
					t.Fatalf("setup Register: %v", err)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup()
			}
			_, err := svc.Register(ctx, tc.email, tc.password)
			if !errors.Is(err, tc.want) {
				t.Fatalf("Register(%s): got %v want %v", tc.name, err, tc.want)
			}
		})
	}
}

func TestLoginRejectsWrongPassword(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	if _, err := svc.Register(ctx, "reader@example.com", "password1"); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if _, _, err := svc.Login(ctx, "reader@example.com", "wrong-password"); !errors.Is(err, auth.ErrUnauthorized) {
		t.Fatalf("Login: got %v want %v", err, auth.ErrUnauthorized)
	}
}

func TestExpiredSessionRejected(t *testing.T) {
	svc, st := newTestService(t)
	ctx := context.Background()
	userID, err := svc.Register(ctx, "reader@example.com", "password1")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	raw := strings.Repeat("ab", 32)
	_, err = st.DB().Exec(
		`INSERT INTO sessions (id, user_id, token_hash, expires_at) VALUES (?, ?, ?, ?)`,
		ids.New("ses_"),
		userID,
		auth.HashToken(raw),
		time.Now().UTC().Add(-time.Hour).Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("insert expired session: %v", err)
	}

	if _, err := svc.UserIDFromToken(ctx, raw); !errors.Is(err, auth.ErrUnauthorized) {
		t.Fatalf("UserIDFromToken: got %v want %v", err, auth.ErrUnauthorized)
	}
}

func TestLogoutInvalidatesToken(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	if _, err := svc.Register(ctx, "reader@example.com", "password1"); err != nil {
		t.Fatalf("Register: %v", err)
	}
	token, _, err := svc.Login(ctx, "reader@example.com", "password1")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if err := svc.Logout(ctx, token); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if _, err := svc.UserIDFromToken(ctx, token); !errors.Is(err, auth.ErrUnauthorized) {
		t.Fatalf("UserIDFromToken after logout: got %v want %v", err, auth.ErrUnauthorized)
	}
}
