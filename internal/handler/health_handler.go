package handler

import (
	"github.com/gin-gonic/gin"

	"golang-gin/internal/pkg/response"
)

// Health reports that the process is up. It touches no dependency on purpose:
// a failure here means the app itself is broken, not that the DB is down.
func Health(c *gin.Context) {
	response.OK(c, response.Body{Code: 200, Message: "ok"})
}
