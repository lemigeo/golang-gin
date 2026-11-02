package testutil

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"golang-gin/internal/handler"
	"golang-gin/internal/repository/mysql"
)

// App is the assembled application plus a small HTTP client.
//
// Tests go through the real router, so routing, middleware, binding, status
// codes and JSON encoding are all exercised — not just the handler function.
type App struct {
	t      *testing.T
	engine *gin.Engine
	token  string
}

// As returns a copy of the app that sends a bearer token on every request.
func (a *App) As(token string) *App {
	clone := *a
	clone.token = token
	return &clone
}

// NewApp builds the engine exactly as production does, on top of db.
func NewApp(t *testing.T, db mysql.DB) *App {
	return NewAppWithConfig(t, db, handler.Config{})
}

// NewAppWithConfig is NewApp with the wiring config, so a test can point the
// OAuth client at a stub provider instead of the real one.
func NewAppWithConfig(t *testing.T, db mysql.DB, cfg handler.Config) *App {
	t.Helper()
	gin.SetMode(gin.TestMode)

	// Session wiring is the same for every test unless one overrides it, so
	// individual tests do not have to know Redis exists.
	if cfg.Redis == nil {
		cfg.Redis = testRedis
	}
	if len(cfg.SessionSecret) == 0 {
		cfg.SessionSecret = []byte("test-secret-that-is-long-enough-32b")
	}
	if cfg.SessionTTL == 0 {
		cfg.SessionTTL = 20 * time.Minute
	}

	return &App{t: t, engine: handler.NewEngine(db, cfg)}
}

func (a *App) GET(path string) *httptest.ResponseRecorder {
	return a.do(http.MethodGet, path, "")
}

func (a *App) POST(path, body string) *httptest.ResponseRecorder {
	return a.do(http.MethodPost, path, body)
}

func (a *App) PUT(path, body string) *httptest.ResponseRecorder {
	return a.do(http.MethodPut, path, body)
}

func (a *App) PATCH(path, body string) *httptest.ResponseRecorder {
	return a.do(http.MethodPatch, path, body)
}

func (a *App) do(method, path, body string) *httptest.ResponseRecorder {
	a.t.Helper()

	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, http.NoBody)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}

	if a.token != "" {
		req.Header.Set("Authorization", "Bearer "+a.token)
	}

	rec := httptest.NewRecorder()
	a.engine.ServeHTTP(rec, req)
	return rec
}
