// Package middleware holds the cross-cutting HTTP concerns.
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"golang-gin/internal/pkg/response"
	"golang-gin/internal/pkg/session"
)

// customerIDKey is unexported so nothing can plant a customer id in the
// context except this middleware.
const customerIDKey = "auth.customerID"

// Auth resolves the bearer token into a customer id and stores it on the
// request.
//
// Every endpoint that acts on behalf of a customer takes the id from here, not
// from the request body: a body is whatever the caller typed, while this comes
// from a token the server issued and Redis still holds.
func Auth(sessions *session.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := bearerToken(c.GetHeader("Authorization"))
		if token == "" {
			response.Error(c, http.StatusUnauthorized, "missing bearer token")
			c.Abort()
			return
		}

		customerID, err := sessions.Resolve(c.Request.Context(), token)
		if err != nil {
			// A forged token and an expired one are the same answer here:
			// saying which would tell an attacker their forgery was well
			// formed.
			response.Error(c, http.StatusUnauthorized, "invalid or expired session")
			c.Abort()
			return
		}

		c.Set(customerIDKey, customerID)
		c.Next()
	}
}

// CustomerID returns the authenticated customer. It is only set on routes that
// went through Auth, so a false second value means the route is misconfigured
// rather than that the caller is anonymous.
func CustomerID(c *gin.Context) (uint64, bool) {
	v, ok := c.Get(customerIDKey)
	if !ok {
		return 0, false
	}
	id, ok := v.(uint64)
	return id, ok
}

func bearerToken(header string) string {
	const prefix = "Bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(header[len(prefix):])
}
