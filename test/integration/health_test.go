package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"golang-gin/internal/handler"
	"golang-gin/internal/pkg/response"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	m.Run()
}

func TestDefaultEndpoints_ReturnOK(t *testing.T) {
	engine := handler.NewEngine()

	for _, path := range []string{"/", "/health"} {
		t.Run(path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

			require.Equal(t, http.StatusOK, rec.Code)

			var body response.Body
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			require.Equal(t, http.StatusOK, body.Code)
			require.Equal(t, "ok", body.Message)
		})
	}
}

func TestUnknownPath_Returns404(t *testing.T) {
	engine := handler.NewEngine()

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/nope", nil))

	require.Equal(t, http.StatusNotFound, rec.Code)
}
