package domain

import "time"

// CouponType mirrors the coupon.type column.
type CouponType uint8

const (
	CouponTypePublic  CouponType = 1 // 다운로드
	CouponTypePrivate CouponType = 2 // 보상
)

// DiscountType mirrors the coupon.discount_type column and decides how
// DiscountValue is read: an amount, or a percentage.
type DiscountType uint8

const (
	DiscountTypeAmount DiscountType = 1
	DiscountTypeRate   DiscountType = 2
)

// CouponStatus mirrors the coupon.status column.
type CouponStatus uint8

const (
	CouponStatusInactive CouponStatus = 0
	CouponStatusActive   CouponStatus = 1
)

// MaxDiscountRate is the ceiling the chk_coupon_discount_value constraint
// enforces for percentage coupons.
const MaxDiscountRate = 100

// Coupon is the entity behind the coupon table. The nullable columns are
// pointers so that "no limit" and "zero" stay distinguishable.
type Coupon struct {
	ID                  uint64
	Name                string
	Description         string
	ThumbnailURL        *string
	Type                CouponType
	DiscountType        DiscountType
	DiscountValue       uint32
	IssueLimitPerPerson *uint32
	IssueLimit          *uint32
	Code                *string
	MinPrice            *uint32
	ValidPeriod         *uint32
	Status              CouponStatus
	StartedAt           *time.Time
	EndedAt             *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// CouponInput is the writable part of a coupon: everything a client may set,
// with the binding rules that mirror the column definitions.
//
// Create and update share it so the two endpoints cannot drift apart — a rule
// added here applies to both.
//
// The binding tags mirror the column definitions: lengths match the VARCHAR
// widths, the enum fields match the values the column comments document, and
// the nullable columns are pointers so that omitting a field means NULL rather
// than zero.
//
// The ceiling on the numeric fields is MaxInt32 rather than MaxUint32 because
// a nullable INT UNSIGNED arrives from the driver as sql.NullInt32; a larger
// value would not survive the round trip.
type CouponInput struct {
	Name        string `json:"name" binding:"required,max=512"`
	Description string `json:"description" binding:"required,max=60000"`

	// 2048 is the practical ceiling browsers and proxies agree on; the column
	// is sized to match. `url` rejects a value that is not a usable absolute
	// URL, which a plain length check would let through.
	ThumbnailURL *string `json:"thumbnailUrl" binding:"omitempty,url,max=2048"`

	Type        CouponType   `json:"type" binding:"required,oneof=1 2"`
	DiscountType DiscountType `json:"discountType" binding:"required,oneof=1 2"`
	DiscountValue uint32     `json:"discountValue" binding:"required,min=1,max=2147483647"`

	// Nullable columns. Omitting one stores NULL, which every one of these
	// reads as "no limit".
	IssueLimitPerPerson *uint32 `json:"issueLimitPerPerson" binding:"omitempty,min=1,max=2147483647"`
	IssueLimit          *uint32 `json:"issueLimit" binding:"omitempty,min=1,max=2147483647"`
	MinPrice            *uint32 `json:"minPrice" binding:"omitempty,min=1,max=2147483647"`
	ValidPeriod         *uint32 `json:"validPeriod" binding:"omitempty,min=1,max=2147483647"`

	// The column is ascii-only, so a non-ascii code would be mangled or
	// rejected by MySQL rather than stored.
	Code *string `json:"code" binding:"omitempty,min=1,max=256,printascii"`

	Status    *CouponStatus `json:"status" binding:"omitempty,oneof=0 1"`
	StartedAt *time.Time    `json:"startedAt"`
	EndedAt   *time.Time    `json:"endedAt"`
}

// Validate covers what the binding tags cannot: the two CHECK constraints on
// the table. Catching them here turns a driver error into a 400 that names the
// problem, but the constraints remain the real guarantee.
func (r CouponInput) Validate() error {
	if r.DiscountType == DiscountTypeRate && r.DiscountValue > MaxDiscountRate {
		return ErrInvalidDiscountValue
	}
	if r.StartedAt != nil && r.EndedAt != nil && r.EndedAt.Before(*r.StartedAt) {
		return ErrInvalidPeriod
	}
	return nil
}

// StatusOrDefault returns the requested status, or the column default.
func (r CouponInput) StatusOrDefault() CouponStatus {
	if r.Status == nil {
		return CouponStatusActive
	}
	return *r.Status
}

// CreateCouponRequest is the JSON body accepted at POST /coupon.
type CreateCouponRequest struct {
	CouponInput
}

// UpdateCouponRequest is the JSON body accepted at PUT /coupon/{id}.
//
// PUT replaces: every writable column is overwritten with what the body says,
// so a field left out becomes NULL rather than keeping its old value. That is
// what makes the result depend only on the request, not on what was there
// before.
type UpdateCouponRequest struct {
	CouponInput
}

// UpdateCouponStatusRequest is the JSON body accepted at
// PATCH /coupon/{id}/status.
//
// Status is a pointer because 0 (inactive) is a meaningful value: with a plain
// CouponStatus the `required` rule could not tell "deactivate" apart from
// "field omitted".
type UpdateCouponStatusRequest struct {
	Status *CouponStatus `json:"status" binding:"required,oneof=0 1"`
}

// CouponResponse is the API shape of a coupon.
type CouponResponse struct {
	ID                  uint64     `json:"id"`
	Name                string     `json:"name"`
	Description         string     `json:"description"`
	ThumbnailURL        *string    `json:"thumbnail_url"`
	Type                uint8      `json:"type"`
	DiscountType        uint8      `json:"discount_type"`
	DiscountValue       uint32     `json:"discount_value"`
	IssueLimitPerPerson *uint32    `json:"issue_limit_per_person"`
	IssueLimit          *uint32    `json:"issue_limit"`
	Code                *string    `json:"code"`
	MinPrice            *uint32    `json:"min_price"`
	ValidPeriod         *uint32    `json:"valid_period"`
	Status              uint8      `json:"status"`
	StartedAt           *time.Time `json:"started_at"`
	EndedAt             *time.Time `json:"ended_at"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

func NewCouponResponse(c *Coupon) CouponResponse {
	return CouponResponse{
		ID:                  c.ID,
		Name:                c.Name,
		Description:         c.Description,
		ThumbnailURL:        c.ThumbnailURL,
		Type:                uint8(c.Type),
		DiscountType:        uint8(c.DiscountType),
		DiscountValue:       c.DiscountValue,
		IssueLimitPerPerson: c.IssueLimitPerPerson,
		IssueLimit:          c.IssueLimit,
		Code:                c.Code,
		MinPrice:            c.MinPrice,
		ValidPeriod:         c.ValidPeriod,
		Status:              uint8(c.Status),
		StartedAt:           c.StartedAt,
		EndedAt:             c.EndedAt,
		CreatedAt:           c.CreatedAt,
		UpdatedAt:           c.UpdatedAt,
	}
}
