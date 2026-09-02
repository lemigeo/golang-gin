package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"golang-gin/internal/domain"
	"golang-gin/internal/repository/mysql/sqlc"
)

// CustomerPasswordRepository handles the customer_password table.
//
// The stored value is an Argon2id hash; nothing here ever sees a plaintext
// password.
type CustomerPasswordRepository struct {
	q *sqlc.Queries
}

func NewCustomerPasswordRepository(db DB) *CustomerPasswordRepository {
	return &CustomerPasswordRepository{q: sqlc.New(db)}
}

// WithTx returns a repository bound to db, for use inside a TxRunner.
func (r *CustomerPasswordRepository) WithTx(db DB) *CustomerPasswordRepository {
	return NewCustomerPasswordRepository(db)
}

func (r *CustomerPasswordRepository) Create(ctx context.Context, customerID uint64, hash string) error {
	_, err := r.q.CreateCustomerPassword(ctx, sqlc.CreateCustomerPasswordParams{
		CustomerID:   customerID,
		PasswordHash: hash,
	})
	if err != nil {
		if isDuplicate(err, "") {
			return domain.ErrPasswordAlreadySet
		}
		return fmt.Errorf("create customer password: %w", err)
	}
	return nil
}

// FindHash returns the stored hash for a customer. The value stays a string on
// purpose: it is passed straight to the verifier and never rendered.
func (r *CustomerPasswordRepository) FindHash(ctx context.Context, customerID uint64) (string, error) {
	row, err := r.q.GetCustomerPassword(ctx, customerID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", domain.ErrPasswordNotSet
		}
		return "", fmt.Errorf("get customer password: %w", err)
	}
	return row.PasswordHash, nil
}

func (r *CustomerPasswordRepository) Update(ctx context.Context, customerID uint64, hash string) error {
	res, err := r.q.UpdateCustomerPassword(ctx, sqlc.UpdateCustomerPasswordParams{
		PasswordHash: hash,
		CustomerID:   customerID,
	})
	if err != nil {
		return fmt.Errorf("update customer password: %w", err)
	}
	return requireAffected(res, domain.ErrPasswordNotSet)
}

func (r *CustomerPasswordRepository) Delete(ctx context.Context, customerID uint64) error {
	res, err := r.q.DeleteCustomerPassword(ctx, customerID)
	if err != nil {
		return fmt.Errorf("delete customer password: %w", err)
	}
	return requireAffected(res, domain.ErrPasswordNotSet)
}
