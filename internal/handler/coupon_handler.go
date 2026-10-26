package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"golang-gin/internal/domain"
	"golang-gin/internal/pkg/response"
	"golang-gin/internal/service"
)

// CouponHandler serves the coupon endpoints.
type CouponHandler struct {
	couponService *service.CouponService
}

func NewCouponHandler(couponService *service.CouponService) *CouponHandler {
	return &CouponHandler{couponService: couponService}
}

// Why coupon creation validates where it does.
//
// Separated from the swag block by a blank line so it stays out of the
// published API description.
//
//   - The binding tags on domain.CreateCouponRequest mirror the column
//     definitions: max=512 is the VARCHAR width, oneof=1 2 is what the column
//     comments document, and the nullable columns are pointers so omitting a
//     field stores NULL rather than zero. A field-level rule belongs there and
//     nowhere else.
//
//   - The two CHECK constraints span fields, which binding tags cannot
//     express, so CreateCouponRequest.Validate covers them: a rate coupon
//     between 1 and 100 percent, and ended_at not before started_at.
//
//   - Validating in Go does not make the constraints redundant. Go rejects
//     early and can say which rule failed; the constraints are what hold when
//     a row arrives by some other path. The repository maps a violated
//     constraint back to the same domain error, so both routes produce the
//     same 400.
//
//   - The numeric ceilings are MaxInt32, not MaxUint32, because a nullable
//     INT UNSIGNED comes back from the driver as sql.NullInt32. A larger value
//     would store but not survive the read.

// CreateCoupon handles POST /coupon.
//
// @Summary      Create a coupon
// @Tags         coupon
// @Accept       json
// @Produce      json
// @Param        request body     domain.CreateCouponRequest true "coupon to create"
// @Success      201     {object} domain.CouponResponse
// @Failure      400     {object} response.ErrorBody "validation failed"
// @Failure      500     {object} response.ErrorBody
// @Router       /coupon [post]
func (h *CouponHandler) CreateCoupon(c *gin.Context) {
	var req domain.CreateCouponRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := req.Validate(); err != nil {
		response.FromError(c, err)
		return
	}

	coupon, err := h.couponService.Create(c.Request.Context(), req)
	if err != nil {
		response.FromError(c, err)
		return
	}

	response.Created(c, domain.NewCouponResponse(coupon))
}

// UpdateCoupon handles PUT /coupon/{id}.
//
// PUT replaces: a field the body leaves out becomes NULL, it does not keep its
// previous value. A partial change needs PATCH, which this API does not have
// yet.
//
// @Summary      Replace a coupon
// @Tags         coupon
// @Accept       json
// @Produce      json
// @Param        id      path     int                        true "coupon id"
// @Param        request body     domain.UpdateCouponRequest true "the coupon as it should be after the call"
// @Success      200     {object} domain.CouponResponse
// @Failure      400     {object} response.ErrorBody "invalid id, or validation failed"
// @Failure      404     {object} response.ErrorBody "coupon not found"
// @Failure      500     {object} response.ErrorBody
// @Router       /coupon/{id} [put]
func (h *CouponHandler) UpdateCoupon(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.Error(c, http.StatusBadRequest, "invalid coupon id")
		return
	}

	var req domain.UpdateCouponRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		response.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	if validErr := req.Validate(); validErr != nil {
		response.FromError(c, validErr)
		return
	}

	coupon, err := h.couponService.Update(c.Request.Context(), id, req)
	if err != nil {
		response.FromError(c, err)
		return
	}

	response.OK(c, domain.NewCouponResponse(coupon))
}

// UpdateCouponStatus handles PATCH /coupon/{id}/status.
//
// Activating and deactivating is the one change an operator makes often and
// under time pressure, so it gets its own endpoint: unlike PUT, this one
// cannot wipe a field by leaving it out of the body.
//
// @Summary      Activate or deactivate a coupon
// @Tags         coupon
// @Accept       json
// @Produce      json
// @Param        id      path     int                              true "coupon id"
// @Param        request body     domain.UpdateCouponStatusRequest true "0: inactive, 1: active"
// @Success      200     {object} domain.CouponResponse
// @Failure      400     {object} response.ErrorBody "invalid id, or unknown status"
// @Failure      404     {object} response.ErrorBody "coupon not found"
// @Failure      500     {object} response.ErrorBody
// @Router       /coupon/{id}/status [patch]
func (h *CouponHandler) UpdateCouponStatus(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.Error(c, http.StatusBadRequest, "invalid coupon id")
		return
	}

	var req domain.UpdateCouponStatusRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		response.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	coupon, err := h.couponService.UpdateStatus(c.Request.Context(), id, *req.Status)
	if err != nil {
		response.FromError(c, err)
		return
	}

	response.OK(c, domain.NewCouponResponse(coupon))
}
