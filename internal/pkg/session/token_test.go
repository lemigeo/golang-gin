package session

import (
	"strings"
	"testing"
)

func TestMintAndParse_RoundTrips(t *testing.T) {
	secret := []byte("a-test-secret-long-enough-for-hmac")

	token, err := mint(secret, 42)
	if err != nil {
		t.Fatalf("mint: %v", err)
	}

	got, err := parse(secret, token)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got != 42 {
		t.Fatalf("customer id = %d, want 42", got)
	}
}

func TestMint_SameCustomerGetsDifferentTokens(t *testing.T) {
	secret := []byte("a-test-secret-long-enough-for-hmac")

	first, err := mint(secret, 1)
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	second, err := mint(secret, 1)
	if err != nil {
		t.Fatalf("mint: %v", err)
	}

	// The nonce is what makes a token unguessable. Without it, two logins by
	// the same customer would produce the same string.
	if first == second {
		t.Fatal("two tokens for the same customer are identical")
	}
}

func TestParse_RejectsForgeryAndTampering(t *testing.T) {
	secret := []byte("a-test-secret-long-enough-for-hmac")
	other := []byte("a-different-secret-of-the-same-len")

	valid, err := mint(secret, 7)
	if err != nil {
		t.Fatalf("mint: %v", err)
	}

	// Swap the customer id but keep the signature.
	_, body, _ := strings.Cut(valid, ".")
	tampered := "8." + body

	signedElsewhere, err := mint(other, 7)
	if err != nil {
		t.Fatalf("mint: %v", err)
	}

	cases := map[string]string{
		"empty":             "",
		"no separator":      "garbage",
		"tampered id":       tampered,
		"signed with other": signedElsewhere,
		"truncated":         valid[:len(valid)-2],
	}

	for name, token := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := parse(secret, token); err == nil {
				t.Fatal("parse accepted an invalid token")
			}
		})
	}
}
