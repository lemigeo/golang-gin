package session

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrNotFound means the token is well-formed but no live session backs it:
// it expired, or it was revoked.
var ErrNotFound = errors.New("session not found")

const keyPrefix = "session:"

// Session is what a successful login hands back to the client.
type Session struct {
	Token     string
	ExpiresAt time.Time
}

// Store keeps sessions in Redis. The key is the token and the value is the
// customer id, so resolving a request is one GET.
type Store struct {
	redis  redis.UniversalClient
	secret []byte
	ttl    time.Duration
}

func NewStore(client redis.UniversalClient, secret []byte, ttl time.Duration) *Store {
	return &Store{redis: client, secret: secret, ttl: ttl}
}

// TTL is how long a freshly issued session lives.
func (s *Store) TTL() time.Duration { return s.ttl }

// Issue mints a token and records it. Existing sessions for the customer are
// left alone, so signing in on a second device does not evict the first.
func (s *Store) Issue(ctx context.Context, customerID uint64) (*Session, error) {
	token, err := mint(s.secret, customerID)
	if err != nil {
		return nil, err
	}

	if err := s.redis.Set(ctx, keyPrefix+token, customerID, s.ttl).Err(); err != nil {
		return nil, fmt.Errorf("store session: %w", err)
	}

	return &Session{
		Token:     token,
		ExpiresAt: time.Now().Add(s.ttl),
	}, nil
}

// Resolve returns the customer behind a token.
//
// The signature is checked first so that garbage never reaches Redis: an
// attacker cannot use this endpoint to generate load with random strings.
func (s *Store) Resolve(ctx context.Context, token string) (uint64, error) {
	signedID, err := parse(s.secret, token)
	if err != nil {
		return 0, err
	}

	storedID, err := s.redis.Get(ctx, keyPrefix+token).Uint64()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, ErrNotFound
		}
		return 0, fmt.Errorf("read session: %w", err)
	}

	// Belt and braces: the signed id and the stored id must agree. They can
	// only differ if the secret leaked or Redis was tampered with.
	if storedID != signedID {
		return 0, ErrInvalidToken
	}
	return storedID, nil
}

// Revoke ends one session. Deleting a token that is already gone is not an
// error: the caller wanted it gone, and it is.
func (s *Store) Revoke(ctx context.Context, token string) error {
	if _, err := parse(s.secret, token); err != nil {
		return err
	}
	if err := s.redis.Del(ctx, keyPrefix+token).Err(); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}
