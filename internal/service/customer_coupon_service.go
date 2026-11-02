package service

import (
	"context"
	"errors"
	"time"

	"golang-gin/internal/domain"
	"golang-gin/internal/repository/mysql"
)

// CustomerCouponService holds the issuing use cases.
type CustomerCouponService struct {
	couponRepo         *mysql.CouponRepository
	customerCouponRepo *mysql.CustomerCouponRepository
}

func NewCustomerCouponService(
	couponRepo *mysql.CouponRepository,
	customerCouponRepo *mysql.CustomerCouponRepository,
) *CustomerCouponService {
	return &CustomerCouponService{
		couponRepo:         couponRepo,
		customerCouponRepo: customerCouponRepo,
	}
}

// Download issues a coupon to a customer.
//
// Two passes, for two different jobs. The first reads the coupon so that an
// unusable one gets an answer that says why — not found, inactive, outside its
// period. The second is the atomic insert, which is what actually decides:
// between the read and the write a coupon can be deactivated or the last slot
// taken, and only the insert's own conditions see that.
//
// So the read must never be treated as permission. It exists for the error
// message; the insert is the gate.
func (s *CustomerCouponService) Download(ctx context.Context, customerID, couponID uint64) (*domain.CustomerCoupon, error) {
	coupon, err := s.couponRepo.FindByID(ctx, couponID)
	if err != nil {
		return nil, err
	}
	if coupon.Status != domain.CouponStatusActive {
		return nil, domain.ErrCouponNotActive
	}
	if !withinPeriod(coupon, time.Now()) {
		return nil, domain.ErrCouponNotInPeriod
	}

	id, err := s.customerCouponRepo.Download(ctx, customerID, couponID)
	if err != nil {
		if errors.Is(err, domain.ErrCouponNotIssuable) {
			return nil, s.explainRefusal(ctx, coupon, customerID)
		}
		return nil, err
	}

	return s.customerCouponRepo.FindByID(ctx, id)
}

// explainRefusal turns "the insert matched nothing" into the reason.
//
// It runs only on the failure path, so the counting costs nothing in the
// common case. The counts are read after the fact and may have moved again;
// they decide the wording of an error, never whether the coupon was issued.
func (s *CustomerCouponService) explainRefusal(ctx context.Context, coupon *domain.Coupon, customerID uint64) error {
	if coupon.IssueLimitPerPerson != nil {
		mine, err := s.customerCouponRepo.CountByCouponAndCustomer(ctx, coupon.ID, customerID)
		if err != nil {
			return err
		}
		if mine >= int64(*coupon.IssueLimitPerPerson) {
			return domain.ErrPersonalLimitReached
		}
	}

	if coupon.IssueLimit != nil {
		total, err := s.customerCouponRepo.CountByCoupon(ctx, coupon.ID)
		if err != nil {
			return err
		}
		if total >= int64(*coupon.IssueLimit) {
			return domain.ErrIssueLimitReached
		}
	}

	// The coupon changed between the read and the insert — deactivated, or its
	// period ended.
	return domain.ErrCouponNotIssuable
}

func withinPeriod(c *domain.Coupon, now time.Time) bool {
	if c.StartedAt != nil && now.Before(*c.StartedAt) {
		return false
	}
	if c.EndedAt != nil && now.After(*c.EndedAt) {
		return false
	}
	return true
}
