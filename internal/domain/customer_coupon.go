package domain

import "time"

// CustomerCoupon is one coupon issued to one customer.
//
// DiscountType and DiscountValue are copies taken at issue time: editing the
// coupon afterwards must not change what an already issued one is worth.
type CustomerCoupon struct {
	ID            uint64
	CustomerID    uint64
	CouponID      uint64
	DiscountType  DiscountType
	DiscountValue uint32
	Code          *string
	UsedAt        *time.Time
	ExpiredAt     *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// CustomerCouponResponse is the API shape of an issued coupon.
type CustomerCouponResponse struct {
	ID            uint64     `json:"id"`
	CustomerID    uint64     `json:"customer_id"`
	CouponID      uint64     `json:"coupon_id"`
	DiscountType  uint8      `json:"discount_type"`
	DiscountValue uint32     `json:"discount_value"`
	Code          *string    `json:"code"`
	UsedAt        *time.Time `json:"used_at"`
	ExpiredAt     *time.Time `json:"expired_at"`
	CreatedAt     time.Time  `json:"created_at"`
}

func NewCustomerCouponResponse(cc *CustomerCoupon) CustomerCouponResponse {
	return CustomerCouponResponse{
		ID:            cc.ID,
		CustomerID:    cc.CustomerID,
		CouponID:      cc.CouponID,
		DiscountType:  uint8(cc.DiscountType),
		DiscountValue: cc.DiscountValue,
		Code:          cc.Code,
		UsedAt:        cc.UsedAt,
		ExpiredAt:     cc.ExpiredAt,
		CreatedAt:     cc.CreatedAt,
	}
}
