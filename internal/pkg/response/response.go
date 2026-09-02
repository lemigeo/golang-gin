package response

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"golang-gin/internal/domain"
)

// Body is the envelope every successful response uses.
type Body struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
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

// FromError maps a domain error to a status code. This is the only place that
// mapping lives: services and repositories do not know about HTTP.
//
// Anything unrecognized becomes a 500 with a generic message, so internal
// details never reach the client.
func FromError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrCustomerNotFound),
		errors.Is(err, domain.ErrSocialNotFound),
		errors.Is(err, domain.ErrPasswordNotSet):
		Error(c, http.StatusNotFound, "not found")

	case errors.Is(err, domain.ErrEmailTaken):
		Error(c, http.StatusConflict, "email already registered")

	case errors.Is(err, domain.ErrSocialAlreadyTaken):
		Error(c, http.StatusConflict, "social account already linked")

	case errors.Is(err, domain.ErrPasswordAlreadySet):
		Error(c, http.StatusConflict, "password already set")

	case errors.Is(err, domain.ErrInvalidCredential):
		Error(c, http.StatusUnauthorized, "invalid email or password")

	case errors.Is(err, domain.ErrInvalidStatus):
		Error(c, http.StatusBadRequest, "invalid status")

	default:
		_ = c.Error(err) // recorded for the logging middleware
		Error(c, http.StatusInternalServerError, "internal server error")
	}
}
