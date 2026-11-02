package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"golang-gin/internal/domain"
	"golang-gin/internal/middleware"
	"golang-gin/internal/pkg/response"
	"golang-gin/internal/service"
)

// CustomerCouponHandler serves the endpoints a logged-in customer uses on
// coupons.
type CustomerCouponHandler struct {
	customerCouponService *service.CustomerCouponService
}

func NewCustomerCouponHandler(s *service.CustomerCouponService) *CustomerCouponHandler {
	return &CustomerCouponHandler{customerCouponService: s}
}

// Why download looks the way it does.
//
// Separated from the swag block by a blank line so it stays out of the
// published API description.
//
//   - The customer comes from the session, never from the request. There is no
//     customerId field to send: the token decides who is downloading, so one
//     customer cannot claim a coupon into somebody else's account.
//
//   - The whole issue — status, period, total limit, per-person limit — is one
//     INSERT ... SELECT. Checking first and inserting after would leave a gap
//     where a concurrent request takes the last slot, and the last slot of a
//     limited coupon is exactly when requests arrive together.
//
//   - The read before it is for the error message only. It can say "not found"
//     or "not active" precisely; it is never treated as permission, because by
//     the time the insert runs the answer may have changed.
//
//   - A refusal is 409, not 400. The request was well formed; the state of the
//     world is what refused it, and retrying later may well succeed.
//
//   - expired_at is computed in SQL as NOW() + valid_period minutes, so the
//     lifetime starts from the moment the database wrote the row rather than
//     from whenever the application happened to read the clock.

// DownloadCoupon handles POST /coupon/{id}/download.
//
// @Summary      Download a coupon into the signed-in customer's wallet
// @Tags         coupon
// @Produce      json
// @Security     BearerAuth
// @Param        id  path     int true "coupon id"
// @Success      201 {object} domain.CustomerCouponResponse
// @Failure      400 {object} response.ErrorBody "invalid coupon id"
// @Failure      401 {object} response.ErrorBody "missing or expired session"
// @Failure      404 {object} response.ErrorBody "coupon not found"
// @Failure      409 {object} response.ErrorBody "inactive, outside its period, or an issue limit is reached"
// @Failure      500 {object} response.ErrorBody
// @Router       /coupon/{id}/download [post]
func (h *CustomerCouponHandler) DownloadCoupon(c *gin.Context) {
	customerID, ok := middleware.CustomerID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "invalid or expired session")
		return
	}

	couponID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || couponID == 0 {
		response.Error(c, http.StatusBadRequest, "invalid coupon id")
		return
	}

	issued, err := h.customerCouponService.Download(c.Request.Context(), customerID, couponID)
	if err != nil {
		response.FromError(c, err)
		return
	}

	response.Created(c, domain.NewCustomerCouponResponse(issued))
}
