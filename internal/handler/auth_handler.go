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

// Why sign-up looks the way it does.
//
// These notes are for whoever changes this endpoint next. They sit in their own
// comment block, separated from the swag annotations below by a blank line, so
// swag does not pull them into the published API description: they explain
// decisions, not the contract.
//
//   - One endpoint serves both flows. mediaName picks the flow, and the
//     conditional requirements are expressed as binding tags on
//     domain.SignUpRequest (`required_if=MediaName email`, `required_unless`).
//     An impossible combination is rejected as 400 before it reaches the
//     service, so the service never has to ask "which fields can I trust".
//
//   - A social sign-up MUST check that the id in the verified profile equals
//     the mediaId in the request. The token proves who the caller is; mediaId
//     is only their claim about it. Drop that check and anyone holding a valid
//     provider token can register an account bound to somebody else's social
//     id.
//
//   - Name and email come from the verified profile; the request values are a
//     fallback for providers that withhold them. Kakao omits the email unless
//     the user granted that scope. Trusting the request email instead would let
//     a social caller claim an address they do not own.
//
//   - A social sign-up writes customer + customer_social and NO
//     customer_password row. For these customers the provider is the
//     credential; an empty or placeholder password row would be a login path
//     nobody intended.
//
//   - The provider client has no interface. internal/pkg/oauth.Client takes its
//     base URLs as configuration, so tests point it at an httptest.Server via
//     handler.Config{KakaoBaseURL: ...} instead of a mock.
//
//   - Naver answers 200 with a resultcode even when the token is bad (024
//     authentication failed, 028 missing header). Checking only the status code
//     would let a forged sign-up through, so the client requires
//     resultcode == "00".

// SignUp handles POST /auth/signup.
//
// @Summary      Sign up with an email or a social account
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body     domain.SignUpRequest true "mediaName decides which fields are required"
// @Success      201     {object} domain.CustomerResponse
// @Failure      400     {object} response.ErrorBody "validation failed"
// @Failure      401     {object} response.ErrorBody "social token rejected, or mediaId does not match the verified profile"
// @Failure      409     {object} response.ErrorBody "email or social account already registered"
// @Failure      500     {object} response.ErrorBody
// @Router       /auth/signup [post]
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

// Why login looks the way it does.
//
// Same as sign-up, these notes are separated from the swag block by a blank
// line so they stay out of the published API description.
//
//   - A missing customer and a wrong password return the same 401. Telling
//     them apart would turn this endpoint into a way to discover which email
//     addresses are registered.
//
//   - A social login re-verifies the provider token and re-checks that the
//     profile id equals mediaId, exactly as sign-up does. The social link is
//     then looked up by that verified id, never by the value the client sent.
//
//   - A verified social account that never signed up gets 404, not 401. The
//     credentials were fine; the client should send the user to sign-up.
//
//   - The provider is authoritative for name and email, so a rename at Kakao
//     or Naver lands here on the next login. If the new email is already taken
//     by another customer the login still succeeds with the old profile — a
//     rename must not lock somebody out of their account.
//
//   - The token is opaque to the client. It carries a customer id and a nonce
//     under an HMAC, but Redis is what decides whether a session is alive: an
//     expired or revoked key is dead however well signed the token is.

// Login handles POST /auth/login.
//
// @Summary      Log in with an email or a social account
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body     domain.LoginRequest true "mediaName decides which fields are required"
// @Success      200     {object} domain.LoginResponse
// @Failure      400     {object} response.ErrorBody "validation failed"
// @Failure      401     {object} response.ErrorBody "wrong credentials, or social token rejected"
// @Failure      403     {object} response.ErrorBody "customer is not active"
// @Failure      404     {object} response.ErrorBody "social account verified but not registered"
// @Failure      500     {object} response.ErrorBody
// @Router       /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req domain.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	customer, issued, err := h.customerService.Login(c.Request.Context(), req)
	if err != nil {
		response.FromError(c, err)
		return
	}

	response.OK(c, domain.LoginResponse{
		Token:     issued.Token,
		ExpiresAt: issued.ExpiresAt,
		Customer:  domain.NewCustomerResponse(customer),
	})
}
