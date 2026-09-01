package handler

import "github.com/gin-gonic/gin"

// NewEngine wires the whole application and returns the router.
// Assembly lives here rather than in main so integration tests build the
// engine through the exact same path production does.
func NewEngine() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	r.GET("/", Health)
	r.GET("/health", Health)

	return r
}
