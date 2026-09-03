// Package session issues and resolves the opaque tokens a client sends back as
// its session.
//
// A token is customerID.nonce.signature, where the signature is an HMAC over
// the first two parts:
//
//	42.k7Fv1s0Q9m2xUuQ0Yx3Ttg.9Yc7T1s...
//
// The nonce is what makes the token unguessable — the customer id alone, or an
// id plus a timestamp, would be trivially predictable. The HMAC lets a forged
// or corrupted token be rejected without touching Redis, and it means an
// attacker cannot mint a token for another customer without the secret.
//
// The signature is not what makes a session valid, though. Redis is: a token
// whose key has expired or been revoked is dead no matter how well it is
// signed.
package session

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrInvalidToken means the token is malformed or its signature does not match.
var ErrInvalidToken = errors.New("invalid session token")

const nonceBytes = 16

var enc = base64.RawURLEncoding

// mint builds a signed token for a customer.
func mint(secret []byte, customerID uint64) (string, error) {
	nonce := make([]byte, nonceBytes)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("read nonce: %w", err)
	}

	body := strconv.FormatUint(customerID, 10) + "." + enc.EncodeToString(nonce)
	return body + "." + sign(secret, body), nil
}

// parse checks the signature and returns the customer id the token claims.
//
// A valid signature only proves the token was issued by this service. The
// caller must still look the token up in Redis to know the session is alive.
func parse(secret []byte, token string) (uint64, error) {
	idx := strings.LastIndex(token, ".")
	if idx < 0 {
		return 0, ErrInvalidToken
	}
	body, sig := token[:idx], token[idx+1:]

	// Constant time: a byte-by-byte comparison would leak how much of a forged
	// signature is correct.
	if !hmac.Equal([]byte(sig), []byte(sign(secret, body))) {
		return 0, ErrInvalidToken
	}

	rawID, _, ok := strings.Cut(body, ".")
	if !ok {
		return 0, ErrInvalidToken
	}
	customerID, err := strconv.ParseUint(rawID, 10, 64)
	if err != nil {
		return 0, ErrInvalidToken
	}
	return customerID, nil
}

func sign(secret []byte, body string) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(body))
	return enc.EncodeToString(mac.Sum(nil))
}
