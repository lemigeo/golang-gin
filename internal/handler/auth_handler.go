package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"golang-gin/internal/domain"
	"golang-gin/internal/pkg/response"
	"golang-gin/internal/service"
)

// AuthHandler serves the sign-up and sign-in endpoints.
type AuthHandler struct {
	customerService *service.CustomerService
}

func NewAuthHandler(customerService *service.CustomerService) *AuthHandler {
	return &AuthHandler{customerService: customerService}
}

// SignUp handles POST /auth/signup.
func (h *AuthHandler) SignUp(c *gin.Context) {
	var req domain.SignUpRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	customer, err := h.customerService.SignUp(c.Request.Context(), req)
	if err != nil {
		response.FromError(c, err)
		return
	}

	response.Created(c, domain.NewCustomerResponse(customer))
}
