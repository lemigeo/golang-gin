package service

import (
	"context"

	"golang-gin/internal/domain"
	"golang-gin/internal/repository/mysql"
)

// CouponService holds the coupon use cases.
type CouponService struct {
	couponRepo *mysql.CouponRepository
}

func NewCouponService(couponRepo *mysql.CouponRepository) *CouponService {
	return &CouponService{couponRepo: couponRepo}
}

// Create stores a coupon.
//
// The request has already been validated against the column rules; this reads
// back the stored row so the response carries what the database actually holds
// — the defaults and the timestamps it filled in, not what was sent.
func (s *CouponService) Create(ctx context.Context, req domain.CreateCouponRequest) (*domain.Coupon, error) {
	coupon := toCoupon(req.CouponInput)

	id, err := s.couponRepo.Create(ctx, coupon)
	if err != nil {
		return nil, err
	}
	return s.couponRepo.FindByID(ctx, id)
}

// Update replaces a coupon and returns the stored row.
//
// Reading it back matters more here than on create: the row that comes back is
// proof of what the replacement actually left behind, including the fields the
// request omitted and the updated_at the database set.
func (s *CouponService) Update(ctx context.Context, id uint64, req domain.UpdateCouponRequest) (*domain.Coupon, error) {
	if err := s.couponRepo.Update(ctx, id, toCoupon(req.CouponInput)); err != nil {
		return nil, err
	}
	return s.couponRepo.FindByID(ctx, id)
}

// UpdateStatus activates or deactivates a coupon.
//
// It is separate from Update because the two are different operations with
// different risks: this one cannot lose a field by omission, so it is safe to
// call from a list screen with a toggle.
func (s *CouponService) UpdateStatus(ctx context.Context, id uint64, status domain.CouponStatus) (*domain.Coupon, error) {
	if err := s.couponRepo.UpdateStatus(ctx, id, status); err != nil {
		return nil, err
	}
	return s.couponRepo.FindByID(ctx, id)
}

// Get returns one coupon by id.
func (s *CouponService) Get(ctx context.Context, id uint64) (*domain.Coupon, error) {
	return s.couponRepo.FindByID(ctx, id)
}

func toCoupon(in domain.CouponInput) *domain.Coupon {
	return &domain.Coupon{
		Name:                in.Name,
		Description:         in.Description,
		ThumbnailURL:        in.ThumbnailURL,
		Type:                in.Type,
		DiscountType:        in.DiscountType,
		DiscountValue:       in.DiscountValue,
		IssueLimitPerPerson: in.IssueLimitPerPerson,
		IssueLimit:          in.IssueLimit,
		Code:                in.Code,
		MinPrice:            in.MinPrice,
		ValidPeriod:         in.ValidPeriod,
		Status:              in.StatusOrDefault(),
		StartedAt:           in.StartedAt,
		EndedAt:             in.EndedAt,
	}
}
