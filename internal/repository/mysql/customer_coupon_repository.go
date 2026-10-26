package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"golang-gin/internal/domain"
	"golang-gin/internal/repository/mysql/sqlc"
)

// CustomerCouponRepository handles the customer_coupon table.
type CustomerCouponRepository struct {
	q *sqlc.Queries
}

func NewCustomerCouponRepository(db DB) *CustomerCouponRepository {
	return &CustomerCouponRepository{q: sqlc.New(db)}
}

// WithTx returns a repository bound to db, for use inside a TxRunner.
func (r *CustomerCouponRepository) WithTx(db DB) *CustomerCouponRepository {
	return NewCustomerCouponRepository(db)
}

// Download issues a coupon to a customer in one statement.
//
// The INSERT ... SELECT carries every condition — status, period and both
// issue limits — so the check and the write are the same statement: nothing
// can change in between. If any condition fails the SELECT produces no row,
// the INSERT writes nothing, and this returns ErrCouponNotIssuable for the
// caller to narrow down.
func (r *CustomerCouponRepository) Download(ctx context.Context, customerID, couponID uint64) (uint64, error) {
	res, err := r.q.DownloadCoupon(ctx, sqlc.DownloadCouponParams{
		CustomerID: customerID,
		CouponID:   couponID,
	})
	if err != nil {
		return 0, fmt.Errorf("download coupon: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("download coupon: rows affected: %w", err)
	}
	if affected == 0 {
		return 0, domain.ErrCouponNotIssuable
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("download coupon: last insert id: %w", err)
	}
	if id < 0 {
		return 0, fmt.Errorf("download coupon: negative insert id %d", id)
	}
	return uint64(id), nil
}

func (r *CustomerCouponRepository) FindByID(ctx context.Context, id uint64) (*domain.CustomerCoupon, error) {
	row, err := r.q.GetCustomerCoupon(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrCouponNotFound
		}
		return nil, fmt.Errorf("get customer coupon: %w", err)
	}
	return toDomainCustomerCoupon(row), nil
}

// CountByCoupon is how many of this coupon have been issued in total.
func (r *CustomerCouponRepository) CountByCoupon(ctx context.Context, couponID uint64) (int64, error) {
	n, err := r.q.CountCustomerCouponsByCoupon(ctx, couponID)
	if err != nil {
		return 0, fmt.Errorf("count customer coupons: %w", err)
	}
	return n, nil
}

// CountByCouponAndCustomer is how many one customer holds.
func (r *CustomerCouponRepository) CountByCouponAndCustomer(ctx context.Context, couponID, customerID uint64) (int64, error) {
	n, err := r.q.CountCustomerCouponsByCouponAndCustomer(ctx, sqlc.CountCustomerCouponsByCouponAndCustomerParams{
		CouponID:   couponID,
		CustomerID: customerID,
	})
	if err != nil {
		return 0, fmt.Errorf("count customer coupons for customer: %w", err)
	}
	return n, nil
}

func toDomainCustomerCoupon(row sqlc.CustomerCoupon) *domain.CustomerCoupon {
	return &domain.CustomerCoupon{
		ID:            row.ID,
		CustomerID:    row.CustomerID,
		CouponID:      row.CouponID,
		DiscountType:  domain.DiscountType(row.DiscountType),
		DiscountValue: row.DiscountValue,
		Code:          stringPtr(row.Code),
		UsedAt:        timePtr(row.UsedAt),
		ExpiredAt:     timePtr(row.ExpiredAt),
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}
}
