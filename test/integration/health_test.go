package integration

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"golang-gin/internal/pkg/response"
	"golang-gin/internal/testutil"
)

func TestDefaultEndpoints_ReturnOK(t *testing.T) {
	app := testutil.NewApp(t, testutil.Tx(t))

	for _, path := range []string{"/", "/health"} {
		t.Run(path, func(t *testing.T) {
			res := app.GET(path)

			require.Equal(t, http.StatusOK, res.Code)

			var body response.Body
			require.NoError(t, json.Unmarshal(res.Body.Bytes(), &body))
			require.Equal(t, http.StatusOK, body.Code)
			require.Equal(t, "ok", body.Message)
		})
	}
}

func TestUnknownPath_Returns404(t *testing.T) {
	app := testutil.NewApp(t, testutil.Tx(t))

	res := app.GET("/nope")

	require.Equal(t, http.StatusNotFound, res.Code)
}
