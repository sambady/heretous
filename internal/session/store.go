package session

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var (
	ErrUnauthenticated = errors.New("session is missing or expired")
	ErrInvalidName     = errors.New("name must contain between 1 and 32 characters")
)

type User struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	IsGuest bool   `json:"is_guest"`
}

type Session struct {
	User      User      `json:"user"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Store keeps all authentication state in PostgreSQL, shared by every instance.
type Store struct {
	db  *sql.DB
	ttl time.Duration
}

func NewStore(db *sql.DB, ttl time.Duration) *Store {
	return &Store{db: db, ttl: ttl}
}

func (s *Store) CreateGuest(ctx context.Context, name, previousToken string) (Session, string, error) {
	if !utf8.ValidString(name) {
		return Session{}, "", ErrInvalidName
	}
	for _, ch := range name {
		if unicode.IsControl(ch) || unicode.Is(unicode.Cf, ch) {
			return Session{}, "", ErrInvalidName
		}
	}
	name = strings.Join(strings.Fields(name), " ")
	if length := utf8.RuneCountInString(name); length < 1 || length > 32 {
		return Session{}, "", ErrInvalidName
	}

	var secret [32]byte
	if _, err := rand.Read(secret[:]); err != nil {
		return Session{}, "", err
	}
	token := base64.RawURLEncoding.EncodeToString(secret[:])
	hash := sha256.Sum256(secret[:])

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Session{}, "", err
	}
	defer tx.Rollback()

	// Replacing a guest session also revokes the previous credential atomically.
	if previousHash, ok := tokenHash(previousToken); ok {
		if _, err := tx.ExecContext(ctx, "DELETE FROM app.sessions WHERE token_hash = $1", previousHash); err != nil {
			return Session{}, "", err
		}
	}
	var current Session
	err = tx.QueryRowContext(ctx,
		"INSERT INTO app.users (name) VALUES ($1) RETURNING id, name, is_guest", name,
	).Scan(&current.User.ID, &current.User.Name, &current.User.IsGuest)
	if err != nil {
		return Session{}, "", err
	}
	err = tx.QueryRowContext(ctx, `
		INSERT INTO app.sessions (token_hash, user_id, expires_at)
		VALUES ($1, $2, CURRENT_TIMESTAMP + ($3::bigint * INTERVAL '1 second'))
		RETURNING expires_at`, hash[:], current.User.ID, int64(s.ttl/time.Second),
	).Scan(&current.ExpiresAt)
	if err != nil {
		return Session{}, "", err
	}
	if err := tx.Commit(); err != nil {
		return Session{}, "", err
	}
	return current, token, nil
}

func (s *Store) Current(ctx context.Context, token string) (Session, error) {
	hash, ok := tokenHash(token)
	if !ok {
		return Session{}, ErrUnauthenticated
	}
	var current Session
	err := s.db.QueryRowContext(ctx, `
		SELECT u.id, u.name, u.is_guest, s.expires_at
		FROM app.sessions s JOIN app.users u ON u.id = s.user_id
		WHERE s.token_hash = $1 AND s.expires_at > CURRENT_TIMESTAMP`, hash,
	).Scan(&current.User.ID, &current.User.Name, &current.User.IsGuest, &current.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrUnauthenticated
	}
	return current, err
}

func (s *Store) Revoke(ctx context.Context, token string) error {
	hash, ok := tokenHash(token)
	if !ok {
		return nil
	}
	_, err := s.db.ExecContext(ctx, "DELETE FROM app.sessions WHERE token_hash = $1", hash)
	return err
}

func tokenHash(token string) ([]byte, bool) {
	if len(token) != 43 {
		return nil, false
	}
	secret, err := base64.RawURLEncoding.Strict().DecodeString(token)
	if err != nil || len(secret) != 32 {
		return nil, false
	}
	hash := sha256.Sum256(secret)
	return hash[:], true
}
