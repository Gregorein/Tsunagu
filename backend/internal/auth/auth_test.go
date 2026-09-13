package auth_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"tsunagu/backend/internal/auth"
	tdb "tsunagu/backend/internal/db"
	"tsunagu/backend/internal/db/sqlcgen"
)

func newManager(t *testing.T) *auth.Manager {
	t.Helper()
	conn, err := tdb.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return auth.New(sqlcgen.New(conn))
}

func TestPasswordAndLoginFlow(t *testing.T) {
	ctx := context.Background()
	m := newManager(t)

	if m.PasswordSet(ctx) {
		t.Fatal("expected no password set initially")
	}

	if _, _, err := m.Login(ctx, "whatever"); err != auth.ErrNoPasswordSet {
		t.Fatalf("expected ErrNoPasswordSet, got %v", err)
	}

	if err := m.SetPassword(ctx, ""); err == nil {
		t.Fatal("expected error for empty password")
	}

	if err := m.SetPassword(ctx, "correct horse battery"); err != nil {
		t.Fatalf("set password: %v", err)
	}
	if !m.PasswordSet(ctx) {
		t.Fatal("expected password to be set")
	}

	if err := m.SetPassword(ctx, "new password here"); err != nil {
		t.Fatalf("rotate password: %v", err)
	}

	if _, _, err := m.Login(ctx, "correct horse battery"); err != auth.ErrWrongPassword {
		t.Fatalf("expected old password to be rejected, got %v", err)
	}

	token, exp, err := m.Login(ctx, "new password here")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if exp.Before(time.Now()) {
		t.Fatal("expected expiry in the future")
	}
	if !m.VerifySession(ctx, token) {
		t.Fatal("expected freshly issued token to verify")
	}
	if m.VerifySession(ctx, token+"tampered") {
		t.Fatal("expected tampered token to be rejected")
	}
	if m.VerifySession(ctx, "garbage") {
		t.Fatal("expected garbage token to be rejected")
	}

	if err := m.DisablePassword(ctx); err != nil {
		t.Fatalf("disable password: %v", err)
	}
	if m.PasswordSet(ctx) {
		t.Fatal("expected password to be unset after disabling")
	}
	if _, _, err := m.Login(ctx, "new password here"); err != auth.ErrNoPasswordSet {
		t.Fatalf("expected ErrNoPasswordSet after disabling, got %v", err)
	}
	if m.VerifySession(ctx, token) {
		t.Fatal("expected old session to stop verifying once password is disabled")
	}
}

func TestSessionAcrossManagerInstances(t *testing.T) {
	ctx := context.Background()
	conn, err := tdb.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer conn.Close()
	q := sqlcgen.New(conn)

	m1 := auth.New(q)
	if err := m1.SetPassword(ctx, "first password!"); err != nil {
		t.Fatalf("set password: %v", err)
	}
	token, _, err := m1.Login(ctx, "first password!")
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	m2 := auth.New(q)
	if !m2.VerifySession(ctx, token) {
		t.Fatal("expected a fresh Manager sharing the same DB to verify the same session secret/token")
	}
}
