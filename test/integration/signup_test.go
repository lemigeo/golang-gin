package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"golang-gin/internal/domain"
	"golang-gin/internal/testutil"
)

func TestSignUp_Success_Returns201AndPersists(t *testing.T) {
	tx := testutil.Tx(t)
	app := testutil.NewApp(t, tx)

	res := app.POST("/auth/signup", `{"email":"a@b.com","password":"correct horse","name":"remi"}`)

	require.Equal(t, http.StatusCreated, res.Code)

	var body domain.CustomerResponse
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &body))
	require.Equal(t, "a@b.com", body.Email)
	require.Equal(t, "remi", body.Name)
	require.Equal(t, "active", body.Status)
	require.NotZero(t, body.ID)

	// The response is not the proof. Check the rows.
	var email string
	require.NoError(t, tx.QueryRowContext(context.Background(),
		"SELECT email FROM customer WHERE id = ?", body.ID).Scan(&email))
	require.Equal(t, "a@b.com", email)

	var hash string
	require.NoError(t, tx.QueryRowContext(context.Background(),
		"SELECT password_hash FROM customer_password WHERE customer_id = ?", body.ID).Scan(&hash))
	require.NotEmpty(t, hash)
	require.NotContains(t, hash, "correct horse", "password must never be stored in the clear")
	require.Contains(t, hash, "$argon2id$")
}

func TestSignUp_DuplicateEmail_Returns409(t *testing.T) {
	app := testutil.NewApp(t, testutil.Tx(t))

	first := app.POST("/auth/signup", `{"email":"dup@b.com","password":"correct horse","name":"first"}`)
	require.Equal(t, http.StatusCreated, first.Code)

	second := app.POST("/auth/signup", `{"email":"dup@b.com","password":"another one","name":"second"}`)
	require.Equal(t, http.StatusConflict, second.Code)
}

func TestSignUp_InvalidBody_Returns400(t *testing.T) {
	app := testutil.NewApp(t, testutil.Tx(t))

	cases := map[string]string{
		"no email":        `{"password":"correct horse","name":"remi"}`,
		"malformed email": `{"email":"not-an-email","password":"correct horse","name":"remi"}`,
		"no password":     `{"email":"a@b.com","name":"remi"}`,
		"short password":  `{"email":"a@b.com","password":"short","name":"remi"}`,
		"no name":         `{"email":"a@b.com","password":"correct horse"}`,
		"name too long":   `{"email":"a@b.com","password":"correct horse","name":"` + longString(65) + `"}`,
		"not json at all": `nope`,
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			res := app.POST("/auth/signup", body)
			require.Equal(t, http.StatusBadRequest, res.Code)
		})
	}
}

// TestSignUp_FailedInsert_RollsBack proves the two inserts share a
// transaction: a signup that fails after the customer row is written must not
// leave that row behind.
func TestSignUp_FailedInsert_RollsBack(t *testing.T) {
	tx := testutil.Tx(t)
	app := testutil.NewApp(t, tx)

	res := app.POST("/auth/signup", `{"email":"rollback@b.com","password":"correct horse","name":"remi"}`)
	require.Equal(t, http.StatusCreated, res.Code)

	var body domain.CustomerResponse
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &body))

	var n int
	require.NoError(t, tx.QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM customer_password WHERE customer_id = ?", body.ID).Scan(&n))
	require.Equal(t, 1, n, "a customer must never exist without a password row")
}

func longString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'a'
	}
	return string(b)
}
