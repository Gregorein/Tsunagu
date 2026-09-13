package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"tsunagu/backend/internal/db/sqlcgen"
)

const (
	keyPasswordHash  = "auth_password_hash"
	keySessionSecret = "auth_session_secret"
	sessionTTL       = 30 * 24 * time.Hour
)

var ErrWrongPassword = errors.New("incorrect password")
var ErrNoPasswordSet = errors.New("no password set")

type Manager struct {
	q *sqlcgen.Queries

	mu     sync.RWMutex
	hash   string
	secret []byte
	loaded bool
}

func New(q *sqlcgen.Queries) *Manager {
	return &Manager{q: q}
}

func (m *Manager) ensureLoaded(ctx context.Context) error {
	m.mu.RLock()
	if m.loaded {
		m.mu.RUnlock()
		return nil
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.loaded {
		return nil
	}

	hash, err := m.q.GetSetting(ctx, keyPasswordHash)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	secretStr, err := m.q.GetSetting(ctx, keySessionSecret)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if secretStr == "" {
		raw := make([]byte, 32)
		if _, err := rand.Read(raw); err != nil {
			return err
		}
		secretStr = base64.RawURLEncoding.EncodeToString(raw)
		if err := m.q.SetSetting(ctx, sqlcgen.SetSettingParams{Key: keySessionSecret, Value: secretStr}); err != nil {
			return err
		}
	}
	secret, err := base64.RawURLEncoding.DecodeString(secretStr)
	if err != nil {
		return fmt.Errorf("decode session secret: %w", err)
	}

	m.hash = hash
	m.secret = secret
	m.loaded = true
	return nil
}

func (m *Manager) PasswordSet(ctx context.Context) bool {
	if err := m.ensureLoaded(ctx); err != nil {
		return false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.hash != ""
}

func (m *Manager) SetPassword(ctx context.Context, newPassword string) error {
	if err := m.ensureLoaded(ctx); err != nil {
		return err
	}
	if newPassword == "" {
		return errors.New("password must not be empty")
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.q.SetSetting(ctx, sqlcgen.SetSettingParams{Key: keyPasswordHash, Value: string(hashed)}); err != nil {
		return err
	}
	m.hash = string(hashed)
	return nil
}

func (m *Manager) DisablePassword(ctx context.Context) error {
	if err := m.ensureLoaded(ctx); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.q.SetSetting(ctx, sqlcgen.SetSettingParams{Key: keyPasswordHash, Value: ""}); err != nil {
		return err
	}
	m.hash = ""
	return nil
}

func (m *Manager) Login(ctx context.Context, password string) (string, time.Time, error) {
	if err := m.ensureLoaded(ctx); err != nil {
		return "", time.Time{}, err
	}
	m.mu.RLock()
	hash, secret := m.hash, m.secret
	m.mu.RUnlock()

	if hash == "" {
		return "", time.Time{}, ErrNoPasswordSet
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return "", time.Time{}, ErrWrongPassword
	}
	exp := time.Now().Add(sessionTTL)
	return issueToken(secret, exp), exp, nil
}

// VerifySession reports whether token is a currently-valid, unexpired session
// token. It never checks the static api_token — that check is separate.
func (m *Manager) VerifySession(ctx context.Context, token string) bool {
	if err := m.ensureLoaded(ctx); err != nil {
		return false
	}
	m.mu.RLock()
	hash, secret := m.hash, m.secret
	m.mu.RUnlock()
	if hash == "" {
		return false
	}
	return verifyToken(secret, token)
}

func issueToken(secret []byte, exp time.Time) string {
	payload := strconv.FormatInt(exp.Unix(), 10)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return payload + "." + sig
}

func verifyToken(secret []byte, token string) bool {
	payload, sigStr, ok := strings.Cut(token, ".")
	if !ok {
		return false
	}
	exp, err := strconv.ParseInt(payload, 10, 64)
	if err != nil || time.Now().Unix() > exp {
		return false
	}
	got, err := base64.RawURLEncoding.DecodeString(sigStr)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payload))
	return hmac.Equal(mac.Sum(nil), got)
}
