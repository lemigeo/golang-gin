package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"golang-gin/internal/domain"
	"golang-gin/internal/repository/mysql/sqlc"
)

// CustomerSocialRepository handles the customer_social table.
type CustomerSocialRepository struct {
	q *sqlc.Queries
}

func NewCustomerSocialRepository(db DB) *CustomerSocialRepository {
	return &CustomerSocialRepository{q: sqlc.New(db)}
}

// WithTx returns a repository bound to db, for use inside a TxRunner.
func (r *CustomerSocialRepository) WithTx(db DB) *CustomerSocialRepository {
	return NewCustomerSocialRepository(db)
}

func (r *CustomerSocialRepository) Create(ctx context.Context, customerID uint64, mediaName, mediaID string) (uint64, error) {
	res, err := r.q.CreateCustomerSocial(ctx, sqlc.CreateCustomerSocialParams{
		CustomerID: customerID,
		MediaName:  mediaName,
		MediaID:    mediaID,
	})
	if err != nil {
		// Either the account is linked to someone else, or this customer
		// already has an account on that media. Both are the same conflict
		// to the caller.
		if isDuplicate(err, "") {
			return 0, domain.ErrSocialAlreadyTaken
		}
		return 0, fmt.Errorf("create customer social: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("create customer social: last insert id: %w", err)
	}
	if id < 0 {
		// AUTO_INCREMENT is unsigned in the schema, so this cannot
		// happen; the check is what makes the conversion provably safe.
		return 0, fmt.Errorf("create customer social: negative insert id %d", id)
	}
	return uint64(id), nil
}

func (r *CustomerSocialRepository) FindByID(ctx context.Context, id uint64) (*domain.CustomerSocial, error) {
	row, err := r.q.GetCustomerSocial(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrSocialNotFound
		}
		return nil, fmt.Errorf("get customer social: %w", err)
	}
	return toDomainCustomerSocial(row), nil
}

func (r *CustomerSocialRepository) FindByMedia(ctx context.Context, mediaName, mediaID string) (*domain.CustomerSocial, error) {
	row, err := r.q.GetCustomerSocialByMedia(ctx, sqlc.GetCustomerSocialByMediaParams{
		MediaName: mediaName,
		MediaID:   mediaID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrSocialNotFound
		}
		return nil, fmt.Errorf("get customer social by media: %w", err)
	}
	return toDomainCustomerSocial(row), nil
}

func (r *CustomerSocialRepository) ListByCustomer(ctx context.Context, customerID uint64) ([]domain.CustomerSocial, error) {
	rows, err := r.q.ListCustomerSocialsByCustomer(ctx, customerID)
	if err != nil {
		return nil, fmt.Errorf("list customer socials: %w", err)
	}

	out := make([]domain.CustomerSocial, 0, len(rows))
	for _, row := range rows {
		out = append(out, *toDomainCustomerSocial(row))
	}
	return out, nil
}

func (r *CustomerSocialRepository) UpdateMediaID(ctx context.Context, id uint64, mediaID string) error {
	res, err := r.q.UpdateCustomerSocialMediaID(ctx, sqlc.UpdateCustomerSocialMediaIDParams{
		MediaID: mediaID,
		ID:      id,
	})
	if err != nil {
		if isDuplicate(err, "") {
			return domain.ErrSocialAlreadyTaken
		}
		return fmt.Errorf("update customer social media id: %w", err)
	}
	return requireAffected(res, domain.ErrSocialNotFound)
}

func (r *CustomerSocialRepository) Delete(ctx context.Context, id uint64) error {
	res, err := r.q.DeleteCustomerSocial(ctx, id)
	if err != nil {
		return fmt.Errorf("delete customer social: %w", err)
	}
	return requireAffected(res, domain.ErrSocialNotFound)
}

// DeleteByCustomer removes every link for a customer. Deleting nothing is not
// an error here: a customer with no linked accounts is a valid state.
func (r *CustomerSocialRepository) DeleteByCustomer(ctx context.Context, customerID uint64) (int64, error) {
	res, err := r.q.DeleteCustomerSocialsByCustomer(ctx, customerID)
	if err != nil {
		return 0, fmt.Errorf("delete customer socials: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected: %w", err)
	}
	return n, nil
}

func toDomainCustomerSocial(row sqlc.CustomerSocial) *domain.CustomerSocial {
	return &domain.CustomerSocial{
		ID:         row.ID,
		CustomerID: row.CustomerID,
		MediaName:  row.MediaName,
		MediaID:    row.MediaID,
		CreatedAt:  row.CreatedAt,
		UpdatedAt:  row.UpdatedAt,
	}
}
