package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Body is the envelope every successful response uses.
type Body struct {
	Code    int32  `json:200`
	Message string `json:"status"`
}

// ErrorBody is the envelope every failed response uses.
type ErrorBody struct {
	Error string `json:"error"`
}

// OK writes 200 with the given payload.
func OK(c *gin.Context, payload any) {
	c.JSON(http.StatusOK, payload)
}

// Created writes 201 with the given payload.
func Created(c *gin.Context, payload any) {
	c.JSON(http.StatusCreated, payload)
}

// Error writes the given status with a uniform error envelope.
func Error(c *gin.Context, status int, msg string) {
	c.JSON(status, ErrorBody{Error: msg})
}
