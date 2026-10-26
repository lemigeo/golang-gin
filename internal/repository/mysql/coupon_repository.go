package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"golang-gin/internal/domain"
	"golang-gin/internal/repository/mysql/sqlc"
)

// CouponRepository handles the coupon table.
type CouponRepository struct {
	q *sqlc.Queries
}

func NewCouponRepository(db DB) *CouponRepository {
	return &CouponRepository{q: sqlc.New(db)}
}

// WithTx returns a repository bound to db, for use inside a TxRunner.
func (r *CouponRepository) WithTx(db DB) *CouponRepository {
	return NewCouponRepository(db)
}

func (r *CouponRepository) Create(ctx context.Context, c *domain.Coupon) (uint64, error) {
	res, err := r.q.CreateCoupon(ctx, sqlc.CreateCouponParams{
		Name:                c.Name,
		Description:         c.Description,
		ThumbnailUrl:        nullString(c.ThumbnailURL),
		Type:                uint8(c.Type),
		DiscountType:        uint8(c.DiscountType),
		DiscountValue:       c.DiscountValue,
		IssueLimitPerPerson: nullInt32(c.IssueLimitPerPerson),
		IssueLimit:          nullInt32(c.IssueLimit),
		Code:                nullString(c.Code),
		MinPrice:            nullInt32(c.MinPrice),
		ValidPeriod:         nullInt32(c.ValidPeriod),
		Status:              uint8(c.Status),
		StartedAt:           nullTime(c.StartedAt),
		EndedAt:             nullTime(c.EndedAt),
	})
	if err != nil {
		// The CHECK constraints are the real guarantee; the service validates
		// the same rules first so this is the belt to that braces.
		if isCheckViolation(err, "chk_coupon_discount_value") {
			return 0, domain.ErrInvalidDiscountValue
		}
		if isCheckViolation(err, "chk_coupon_period") {
			return 0, domain.ErrInvalidPeriod
		}
		return 0, fmt.Errorf("create coupon: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("create coupon: last insert id: %w", err)
	}
	if id < 0 {
		return 0, fmt.Errorf("create coupon: negative insert id %d", id)
	}
	return uint64(id), nil
}

// Update overwrites every writable column of one coupon.
func (r *CouponRepository) Update(ctx context.Context, id uint64, c *domain.Coupon) error {
	res, err := r.q.UpdateCoupon(ctx, sqlc.UpdateCouponParams{
		ID:                  id,
		Name:                c.Name,
		Description:         c.Description,
		ThumbnailUrl:        nullString(c.ThumbnailURL),
		Type:                uint8(c.Type),
		DiscountType:        uint8(c.DiscountType),
		DiscountValue:       c.DiscountValue,
		IssueLimitPerPerson: nullInt32(c.IssueLimitPerPerson),
		IssueLimit:          nullInt32(c.IssueLimit),
		Code:                nullString(c.Code),
		MinPrice:            nullInt32(c.MinPrice),
		ValidPeriod:         nullInt32(c.ValidPeriod),
		Status:              uint8(c.Status),
		StartedAt:           nullTime(c.StartedAt),
		EndedAt:             nullTime(c.EndedAt),
	})
	if err != nil {
		if isCheckViolation(err, "chk_coupon_discount_value") {
			return domain.ErrInvalidDiscountValue
		}
		if isCheckViolation(err, "chk_coupon_period") {
			return domain.ErrInvalidPeriod
		}
		return fmt.Errorf("update coupon: %w", err)
	}
	return requireAffected(res, domain.ErrCouponNotFound)
}

// UpdateStatus flips a coupon between active and inactive, touching nothing
// else. Deactivating is how a coupon is retired: the row stays, so issued
// coupons and reports keep pointing at something real.
func (r *CouponRepository) UpdateStatus(ctx context.Context, id uint64, status domain.CouponStatus) error {
	res, err := r.q.UpdateCouponStatus(ctx, sqlc.UpdateCouponStatusParams{
		Status: uint8(status),
		ID:     id,
	})
	if err != nil {
		return fmt.Errorf("update coupon status: %w", err)
	}
	return requireAffected(res, domain.ErrCouponNotFound)
}

func (r *CouponRepository) FindByID(ctx context.Context, id uint64) (*domain.Coupon, error) {
	row, err := r.q.GetCoupon(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrCouponNotFound
		}
		return nil, fmt.Errorf("get coupon: %w", err)
	}
	return toDomainCoupon(row), nil
}

// FindByIDAndCode requires both, so a client cannot claim a coupon by id alone
// with somebody else's code.
func (r *CouponRepository) FindByIDAndCode(ctx context.Context, id uint64, code string) (*domain.Coupon, error) {
	row, err := r.q.GetCouponByCode(ctx, sqlc.GetCouponByCodeParams{
		ID:   id,
		Code: sql.NullString{String: code, Valid: true},
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrCouponNotFound
		}
		return nil, fmt.Errorf("get coupon by code: %w", err)
	}
	return toDomainCoupon(row), nil
}

func toDomainCoupon(row sqlc.Coupon) *domain.Coupon {
	return &domain.Coupon{
		ID:                  row.ID,
		Name:                row.Name,
		Description:         row.Description,
		ThumbnailURL:        stringPtr(row.ThumbnailUrl),
		Type:                domain.CouponType(row.Type),
		DiscountType:        domain.DiscountType(row.DiscountType),
		DiscountValue:       row.DiscountValue,
		IssueLimitPerPerson: uint32Ptr(row.IssueLimitPerPerson),
		IssueLimit:          uint32Ptr(row.IssueLimit),
		Code:                stringPtr(row.Code),
		MinPrice:            uint32Ptr(row.MinPrice),
		ValidPeriod:         uint32Ptr(row.ValidPeriod),
		Status:              domain.CouponStatus(row.Status),
		StartedAt:           timePtr(row.StartedAt),
		EndedAt:             timePtr(row.EndedAt),
		CreatedAt:           row.CreatedAt,
		UpdatedAt:           row.UpdatedAt,
	}
}
