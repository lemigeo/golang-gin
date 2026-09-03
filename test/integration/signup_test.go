package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"golang-gin/internal/domain"
	"golang-gin/internal/handler"
	"golang-gin/internal/testutil"
)

func TestSignUpEmail_Success_Returns201AndPersists(t *testing.T) {
	tx := testutil.Tx(t)
	app := testutil.NewApp(t, tx)

	res := app.POST("/auth/signup",
		`{"email":"a@b.com","password":"correct horse","name":"remi","mediaName":"email"}`)

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
	require.NotContains(t, hash, "correct horse", "password must never be stored in the clear")
	require.Contains(t, hash, "$argon2id$")
}

func TestSignUpEmail_DuplicateEmail_Returns409(t *testing.T) {
	app := testutil.NewApp(t, testutil.Tx(t))

	first := app.POST("/auth/signup",
		`{"email":"dup@b.com","password":"correct horse","name":"first","mediaName":"email"}`)
	require.Equal(t, http.StatusCreated, first.Code)

	second := app.POST("/auth/signup",
		`{"email":"dup@b.com","password":"another one","name":"second","mediaName":"email"}`)
	require.Equal(t, http.StatusConflict, second.Code)
}

func TestSignUp_InvalidBody_Returns400(t *testing.T) {
	app := testutil.NewApp(t, testutil.Tx(t))

	cases := []struct {
		name string
		body string
	}{
		{"no email", `{"password":"correct horse","name":"remi","mediaName":"email"}`},
		{"malformed email", `{"email":"nope","password":"correct horse","name":"remi","mediaName":"email"}`},
		{"no media name", `{"email":"a@b.com","password":"correct horse","name":"remi"}`},
		{"unknown media", `{"email":"a@b.com","name":"remi","mediaName":"line","mediaId":"1","mediaToken":"t"}`},
		{"no password", `{"email":"a@b.com","name":"remi","mediaName":"email"}`},
		{"short password", `{"email":"a@b.com","password":"short","name":"remi","mediaName":"email"}`},
		{"no name", `{"email":"a@b.com","password":"correct horse","mediaName":"email"}`},
		{"social without media id", `{"email":"a@b.com","mediaName":"kakao","mediaToken":"t"}`},
		{"social without token", `{"email":"a@b.com","mediaName":"kakao","mediaId":"1"}`},
		{"not json at all", `nope`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := app.POST("/auth/signup", tc.body)
			require.Equal(t, http.StatusBadRequest, res.Code)
		})
	}
}

// stubKakao stands in for kapi.kakao.com. The client takes its base URL as
// configuration, which is why no provider interface is needed to test this.
func stubKakao(t *testing.T, status int, payload string) string {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// require would call FailNow from the server goroutine, which is not
		// allowed. Report and let the assertions after the request decide.
		if r.URL.Path != "/v2/user/me" {
			t.Errorf("kakao stub: unexpected path %q", r.URL.Path)
		}
		if r.Header.Get("Authorization") == "" {
			t.Error("kakao stub: missing Authorization header")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(payload))
	}))
	t.Cleanup(srv.Close)

	return srv.URL
}

func TestSignUpSocial_Kakao_UsesProfileNameAndSkipsPassword(t *testing.T) {
	tx := testutil.Tx(t)
	base := stubKakao(t, http.StatusOK, `{
		"id": 4815162342,
		"properties": {"nickname": "카카오이름"},
		"kakao_account": {"has_email": true, "email": "from-kakao@b.com"}
	}`)
	app := testutil.NewAppWithConfig(t, tx, handler.Config{KakaoBaseURL: base})

	res := app.POST("/auth/signup",
		`{"email":"ignored@b.com","name":"보낸이름","mediaName":"kakao","mediaId":"4815162342","mediaToken":"tok"}`)

	require.Equal(t, http.StatusCreated, res.Code)

	var body domain.CustomerResponse
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &body))
	require.Equal(t, "카카오이름", body.Name, "name must come from the verified profile")
	require.Equal(t, "from-kakao@b.com", body.Email, "email must come from the verified profile")

	// The social link is written...
	var mediaID string
	require.NoError(t, tx.QueryRowContext(context.Background(),
		"SELECT media_id FROM customer_social WHERE customer_id = ? AND media_name = ?",
		body.ID, "kakao").Scan(&mediaID))
	require.Equal(t, "4815162342", mediaID)

	// ...and no password row exists: the provider is the credential.
	var n int
	require.NoError(t, tx.QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM customer_password WHERE customer_id = ?", body.ID).Scan(&n))
	require.Equal(t, 0, n)
}

func TestSignUpSocial_MediaIDMismatch_Returns401(t *testing.T) {
	base := stubKakao(t, http.StatusOK, `{"id": 111, "properties": {"nickname": "n"}}`)
	app := testutil.NewAppWithConfig(t, testutil.Tx(t), handler.Config{KakaoBaseURL: base})

	// The token is valid but belongs to a different account than claimed.
	res := app.POST("/auth/signup",
		`{"email":"a@b.com","name":"n","mediaName":"kakao","mediaId":"999","mediaToken":"tok"}`)

	require.Equal(t, http.StatusUnauthorized, res.Code)
}

func TestSignUpSocial_ProviderRejectsToken_Returns401(t *testing.T) {
	base := stubKakao(t, http.StatusUnauthorized, `{"msg":"invalid token","code":-401}`)
	app := testutil.NewAppWithConfig(t, testutil.Tx(t), handler.Config{KakaoBaseURL: base})

	res := app.POST("/auth/signup",
		`{"email":"a@b.com","name":"n","mediaName":"kakao","mediaId":"1","mediaToken":"bad"}`)

	require.Equal(t, http.StatusUnauthorized, res.Code)
}

func TestSignUpSocial_SameSocialAccountTwice_Returns409(t *testing.T) {
	base := stubKakao(t, http.StatusOK, `{"id": 777, "properties": {"nickname": "n"}}`)
	app := testutil.NewAppWithConfig(t, testutil.Tx(t), handler.Config{KakaoBaseURL: base})

	first := app.POST("/auth/signup",
		`{"email":"one@b.com","name":"n","mediaName":"kakao","mediaId":"777","mediaToken":"tok"}`)
	require.Equal(t, http.StatusCreated, first.Code)

	second := app.POST("/auth/signup",
		`{"email":"two@b.com","name":"n","mediaName":"kakao","mediaId":"777","mediaToken":"tok"}`)
	require.Equal(t, http.StatusConflict, second.Code)
}

func TestSignUpSocial_Naver_Success(t *testing.T) {
	tx := testutil.Tx(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/nid/me" {
			t.Errorf("naver stub: unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"resultcode":"00","message":"success",
			"response":{"id":"naver-1","email":"n@b.com","name":"네이버이름"}}`))
	}))
	t.Cleanup(srv.Close)

	app := testutil.NewAppWithConfig(t, tx, handler.Config{NaverBaseURL: srv.URL})

	res := app.POST("/auth/signup",
		`{"email":"ignored@b.com","name":"보낸이름","mediaName":"naver","mediaId":"naver-1","mediaToken":"tok"}`)

	require.Equal(t, http.StatusCreated, res.Code)

	var body domain.CustomerResponse
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &body))
	require.Equal(t, "네이버이름", body.Name)
	require.Equal(t, "n@b.com", body.Email)
}

// Naver answers 200 with a result code even when the token is bad, so a naive
// status-code check would let a forged signup through.
func TestSignUpSocial_NaverErrorResultCode_Returns401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"resultcode":"024","message":"Authentication failed"}`))
	}))
	t.Cleanup(srv.Close)

	app := testutil.NewAppWithConfig(t, testutil.Tx(t), handler.Config{NaverBaseURL: srv.URL})

	res := app.POST("/auth/signup",
		`{"email":"a@b.com","name":"n","mediaName":"naver","mediaId":"naver-1","mediaToken":"tok"}`)

	require.Equal(t, http.StatusUnauthorized, res.Code)
}
