package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"golang-gin/internal/pkg/response"
)

// Health reports that the process is up. It touches no dependency on purpose:
// a failure here means the app itself is broken, not that the DB is down.
//
// @Summary  Liveness probe
// @Tags     system
// @Produce  json
// @Success  200 {object} response.Body
// @Router   /health [get]
func Health(c *gin.Context) {
	response.OK(c, response.Body{Code: http.StatusOK, Message: "ok"})
}
