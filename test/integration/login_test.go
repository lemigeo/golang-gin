package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"golang-gin/internal/domain"
	"golang-gin/internal/handler"
	"golang-gin/internal/testutil"
)

func TestLoginEmail_Success_IssuesSession(t *testing.T) {
	tx := testutil.Tx(t)
	app := testutil.NewApp(t, tx)

	signup := app.POST("/auth/signup",
		`{"email":"login@b.com","password":"correct horse","name":"remi","mediaName":"email"}`)
	require.Equal(t, http.StatusCreated, signup.Code)

	res := app.POST("/auth/login",
		`{"email":"login@b.com","password":"correct horse","mediaName":"email"}`)
	require.Equal(t, http.StatusOK, res.Code)

	var body domain.LoginResponse
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &body))
	require.NotEmpty(t, body.Token)
	require.Equal(t, "login@b.com", body.Customer.Email)
	require.WithinDuration(t, time.Now().Add(20*time.Minute), body.ExpiresAt, time.Minute)

	// The session is really in Redis, under the token, with a TTL.
	ttl, err := testutil.Redis().TTL(context.Background(), "session:"+body.Token).Result()
	require.NoError(t, err)
	require.Greater(t, ttl, 19*time.Minute)
	require.LessOrEqual(t, ttl, 20*time.Minute)

	storedID, err := testutil.Redis().Get(context.Background(), "session:"+body.Token).Uint64()
	require.NoError(t, err)
	require.Equal(t, body.Customer.ID, storedID)
}

func TestLoginEmail_WrongPassword_Returns401(t *testing.T) {
	app := testutil.NewApp(t, testutil.Tx(t))

	signup := app.POST("/auth/signup",
		`{"email":"wrong@b.com","password":"correct horse","name":"remi","mediaName":"email"}`)
	require.Equal(t, http.StatusCreated, signup.Code)

	res := app.POST("/auth/login",
		`{"email":"wrong@b.com","password":"not the password","mediaName":"email"}`)
	require.Equal(t, http.StatusUnauthorized, res.Code)
}

// An unknown email and a wrong password must be indistinguishable, or this
// endpoint becomes a way to enumerate registered addresses.
func TestLoginEmail_UnknownEmail_LooksTheSameAsWrongPassword(t *testing.T) {
	app := testutil.NewApp(t, testutil.Tx(t))

	signup := app.POST("/auth/signup",
		`{"email":"known@b.com","password":"correct horse","name":"remi","mediaName":"email"}`)
	require.Equal(t, http.StatusCreated, signup.Code)

	unknown := app.POST("/auth/login",
		`{"email":"nobody@b.com","password":"correct horse","mediaName":"email"}`)
	wrongPw := app.POST("/auth/login",
		`{"email":"known@b.com","password":"not the password","mediaName":"email"}`)

	require.Equal(t, http.StatusUnauthorized, unknown.Code)
	require.Equal(t, unknown.Code, wrongPw.Code)
	require.JSONEq(t, unknown.Body.String(), wrongPw.Body.String())
}

func TestLoginEmail_TwoLogins_BothSessionsLive(t *testing.T) {
	app := testutil.NewApp(t, testutil.Tx(t))

	require.Equal(t, http.StatusCreated, app.POST("/auth/signup",
		`{"email":"two@b.com","password":"correct horse","name":"remi","mediaName":"email"}`).Code)

	first := loginToken(t, app, `{"email":"two@b.com","password":"correct horse","mediaName":"email"}`)
	second := loginToken(t, app, `{"email":"two@b.com","password":"correct horse","mediaName":"email"}`)

	require.NotEqual(t, first, second, "each login gets a fresh token")
	for _, token := range []string{first, second} {
		n, err := testutil.Redis().Exists(context.Background(), "session:"+token).Result()
		require.NoError(t, err)
		require.EqualValues(t, 1, n, "signing in on a second device must not evict the first")
	}
}

func TestLoginSocial_Success_UpdatesChangedProfile(t *testing.T) {
	tx := testutil.Tx(t)

	profile := `{"id": 4242, "properties": {"nickname": "처음이름"},
		"kakao_account": {"has_email": true, "email": "first@b.com"}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(profile))
	}))
	t.Cleanup(srv.Close)

	app := testutil.NewAppWithConfig(t, tx, handler.Config{KakaoBaseURL: srv.URL})

	signup := app.POST("/auth/signup",
		`{"email":"ignored@b.com","name":"보낸이름","mediaName":"kakao","mediaId":"4242","mediaToken":"tok"}`)
	require.Equal(t, http.StatusCreated, signup.Code)

	// The provider now reports a different name and email.
	profile = `{"id": 4242, "properties": {"nickname": "바뀐이름"},
		"kakao_account": {"has_email": true, "email": "second@b.com"}}`

	res := app.POST("/auth/login", `{"mediaName":"kakao","mediaId":"4242","mediaToken":"tok"}`)
	require.Equal(t, http.StatusOK, res.Code)

	var body domain.LoginResponse
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &body))
	require.Equal(t, "바뀐이름", body.Customer.Name)
	require.Equal(t, "second@b.com", body.Customer.Email)

	// ...and the change is persisted, not just reflected in the response.
	var name, email string
	require.NoError(t, tx.QueryRowContext(context.Background(),
		"SELECT name, email FROM customer WHERE id = ?", body.Customer.ID).Scan(&name, &email))
	require.Equal(t, "바뀐이름", name)
	require.Equal(t, "second@b.com", email)
}

// A verified social account that never signed up is 404, not 401: the
// credentials were fine, the account simply does not exist yet.
func TestLoginSocial_NeverSignedUp_Returns404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id": 5150, "properties": {"nickname": "n"}}`))
	}))
	t.Cleanup(srv.Close)

	app := testutil.NewAppWithConfig(t, testutil.Tx(t), handler.Config{KakaoBaseURL: srv.URL})

	res := app.POST("/auth/login", `{"mediaName":"kakao","mediaId":"5150","mediaToken":"tok"}`)
	require.Equal(t, http.StatusNotFound, res.Code)
}

func TestLoginSocial_MediaIDMismatch_Returns401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id": 111, "properties": {"nickname": "n"}}`))
	}))
	t.Cleanup(srv.Close)

	app := testutil.NewAppWithConfig(t, testutil.Tx(t), handler.Config{KakaoBaseURL: srv.URL})

	res := app.POST("/auth/login", `{"mediaName":"kakao","mediaId":"999","mediaToken":"tok"}`)
	require.Equal(t, http.StatusUnauthorized, res.Code)
}

func TestLogin_InvalidBody_Returns400(t *testing.T) {
	app := testutil.NewApp(t, testutil.Tx(t))

	cases := []struct {
		name string
		body string
	}{
		{"no media name", `{"email":"a@b.com","password":"correct horse"}`},
		{"email without password", `{"email":"a@b.com","mediaName":"email"}`},
		{"email without address", `{"password":"correct horse","mediaName":"email"}`},
		{"social without media id", `{"mediaName":"kakao","mediaToken":"t"}`},
		{"social without token", `{"mediaName":"kakao","mediaId":"1"}`},
		{"unknown media", `{"mediaName":"line","mediaId":"1","mediaToken":"t"}`},
		{"not json at all", `nope`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := app.POST("/auth/login", tc.body)
			require.Equal(t, http.StatusBadRequest, res.Code)
		})
	}
}

func loginToken(t *testing.T, app *testutil.App, body string) string {
	t.Helper()

	res := app.POST("/auth/login", body)
	require.Equal(t, http.StatusOK, res.Code)

	var parsed domain.LoginResponse
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &parsed))
	require.NotEmpty(t, parsed.Token)
	return parsed.Token
}
