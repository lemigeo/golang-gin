package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"golang-gin/internal/domain"
	"golang-gin/internal/repository/mysql/sqlc"
)

// CustomerRepository is a thin layer over the generated queries: it calls them
// and maps rows and driver errors into domain types. No business rules here.
type CustomerRepository struct {
	q *sqlc.Queries
}

// NewCustomerRepository takes a DBTX so production can pass *sql.DB and tests
// can pass a *sql.Tx that gets rolled back.
func NewCustomerRepository(db DB) *CustomerRepository {
	return &CustomerRepository{q: sqlc.New(db)}
}

// WithTx returns a repository bound to db, for use inside a TxRunner.
func (r *CustomerRepository) WithTx(db DB) *CustomerRepository {
	return NewCustomerRepository(db)
}

func (r *CustomerRepository) Create(ctx context.Context, email, name string, status domain.CustomerStatus) (uint64, error) {
	res, err := r.q.CreateCustomer(ctx, sqlc.CreateCustomerParams{
		Email:  email,
		Name:   name,
		Status: uint8(status),
	})
	if err != nil {
		if isDuplicate(err, "uk_customer_email") {
			return 0, domain.ErrEmailTaken
		}
		return 0, fmt.Errorf("create customer: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("create customer: last insert id: %w", err)
	}
	if id < 0 {
		// AUTO_INCREMENT is unsigned in the schema, so this cannot
		// happen; the check is what makes the conversion provably safe.
		return 0, fmt.Errorf("create customer: negative insert id %d", id)
	}
	return uint64(id), nil
}

func (r *CustomerRepository) FindByID(ctx context.Context, id uint64) (*domain.Customer, error) {
	row, err := r.q.GetCustomer(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrCustomerNotFound
		}
		return nil, fmt.Errorf("get customer: %w", err)
	}
	return toDomainCustomer(row), nil
}

func (r *CustomerRepository) FindByEmail(ctx context.Context, email string) (*domain.Customer, error) {
	row, err := r.q.GetCustomerByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrCustomerNotFound
		}
		return nil, fmt.Errorf("get customer by email: %w", err)
	}
	return toDomainCustomer(row), nil
}

func (r *CustomerRepository) ListByStatus(ctx context.Context, status domain.CustomerStatus, limit, offset int32) ([]domain.Customer, error) {
	rows, err := r.q.ListCustomersByStatus(ctx, sqlc.ListCustomersByStatusParams{
		Status: uint8(status),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list customers: %w", err)
	}

	out := make([]domain.Customer, 0, len(rows))
	for _, row := range rows {
		out = append(out, *toDomainCustomer(row))
	}
	return out, nil
}

func (r *CustomerRepository) CountByStatus(ctx context.Context, status domain.CustomerStatus) (int64, error) {
	n, err := r.q.CountCustomersByStatus(ctx, uint8(status))
	if err != nil {
		return 0, fmt.Errorf("count customers: %w", err)
	}
	return n, nil
}

func (r *CustomerRepository) UpdateName(ctx context.Context, id uint64, name string) error {
	res, err := r.q.UpdateCustomerName(ctx, sqlc.UpdateCustomerNameParams{Name: name, ID: id})
	if err != nil {
		return fmt.Errorf("update customer name: %w", err)
	}
	return requireAffected(res, domain.ErrCustomerNotFound)
}

func (r *CustomerRepository) UpdateStatus(ctx context.Context, id uint64, status domain.CustomerStatus) error {
	res, err := r.q.UpdateCustomerStatus(ctx, sqlc.UpdateCustomerStatusParams{
		Status: uint8(status),
		ID:     id,
	})
	if err != nil {
		return fmt.Errorf("update customer status: %w", err)
	}
	return requireAffected(res, domain.ErrCustomerNotFound)
}

func (r *CustomerRepository) Delete(ctx context.Context, id uint64) error {
	res, err := r.q.DeleteCustomer(ctx, id)
	if err != nil {
		return fmt.Errorf("delete customer: %w", err)
	}
	return requireAffected(res, domain.ErrCustomerNotFound)
}

func toDomainCustomer(row sqlc.Customer) *domain.Customer {
	return &domain.Customer{
		ID:        row.ID,
		Email:     row.Email,
		Name:      row.Name,
		Status:    domain.CustomerStatus(row.Status),
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
}

// requireAffected turns "the statement matched nothing" into notFound, so
// callers do not have to inspect sql.Result themselves.
func requireAffected(res sql.Result, notFound error) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return notFound
	}
	return nil
}
