// Package password hashes and verifies passwords with Argon2id.
//
// Hashes are stored as PHC strings, e.g.
//
//	$argon2id$v=19$m=19456,t=2,p=1$c29tZXNhbHQ$aGFzaA
//
// The parameters live inside the value, so raising them later does not
// invalidate existing hashes: each one is verified with the parameters it was
// created with, and NeedsRehash reports the ones worth upgrading on next login.
package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// ErrMismatch means the password does not match the hash. It is the expected
// outcome of a wrong password, not a failure of the hashing itself.
var ErrMismatch = errors.New("password does not match")

// ErrInvalidHash means the stored value is not a hash this package can read.
var ErrInvalidHash = errors.New("invalid password hash format")

// Params are the Argon2id cost parameters. Defaults follow the OWASP minimum
// (19 MiB, 2 iterations, 1 lane).
type Params struct {
	Memory      uint32 // KiB
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

func DefaultParams() Params {
	return Params{
		Memory:      19 * 1024,
		Iterations:  2,
		Parallelism: 1,
		SaltLength:  16,
		KeyLength:   32,
	}
}

// Hasher produces and checks Argon2id hashes.
type Hasher struct {
	params Params
}

func NewHasher(p Params) *Hasher { return &Hasher{params: p} }

// Hash returns the PHC-encoded hash of plain.
func (h *Hasher) Hash(plain string) (string, error) {
	salt := make([]byte, h.params.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("read salt: %w", err)
	}

	key := argon2.IDKey(
		[]byte(plain),
		salt,
		h.params.Iterations,
		h.params.Memory,
		h.params.Parallelism,
		h.params.KeyLength,
	)

	b64 := base64.RawStdEncoding
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		h.params.Memory,
		h.params.Iterations,
		h.params.Parallelism,
		b64.EncodeToString(salt),
		b64.EncodeToString(key),
	), nil
}

// Verify reports whether plain produced encoded. It returns ErrMismatch when
// the password is simply wrong, so callers can tell that apart from a broken
// stored value.
func (h *Hasher) Verify(plain, encoded string) error {
	params, salt, want, err := decode(encoded)
	if err != nil {
		return err
	}

	// Bounded before the conversion: a key length outside this range means
	// the stored value is not one this package produced.
	keyLen := len(want)
	if keyLen < 16 || keyLen > 1024 {
		return ErrInvalidHash
	}

	got := argon2.IDKey(
		[]byte(plain),
		salt,
		params.Iterations,
		params.Memory,
		params.Parallelism,
		uint32(keyLen),
	)

	if subtle.ConstantTimeCompare(got, want) != 1 {
		return ErrMismatch
	}
	return nil
}

// NeedsRehash reports whether encoded was made with weaker parameters than the
// hasher now uses. Call it after a successful Verify to upgrade hashes as
// users log in.
func (h *Hasher) NeedsRehash(encoded string) bool {
	params, _, _, err := decode(encoded)
	if err != nil {
		return true
	}
	return params.Memory < h.params.Memory ||
		params.Iterations < h.params.Iterations ||
		params.Parallelism < h.params.Parallelism
}

func decode(encoded string) (p Params, salt, key []byte, err error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return p, nil, nil, ErrInvalidHash
	}

	var version int
	if _, err = fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return p, nil, nil, ErrInvalidHash
	}
	if _, err = fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.Memory, &p.Iterations, &p.Parallelism); err != nil {
		return p, nil, nil, ErrInvalidHash
	}

	b64 := base64.RawStdEncoding
	if salt, err = b64.DecodeString(parts[4]); err != nil {
		return p, nil, nil, ErrInvalidHash
	}
	if key, err = b64.DecodeString(parts[5]); err != nil {
		return p, nil, nil, ErrInvalidHash
	}
	return p, salt, key, nil
}
