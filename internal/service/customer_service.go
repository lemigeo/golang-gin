package service

import (
	"context"
	"errors"
	"fmt"

	"golang-gin/internal/domain"
	"golang-gin/internal/pkg/password"
	"golang-gin/internal/repository/mysql"
)

// CustomerService holds the customer use cases. Repositories are concrete
// types, not interfaces.
type CustomerService struct {
	tx           *mysql.TxRunner
	customerRepo *mysql.CustomerRepository
	passwordRepo *mysql.CustomerPasswordRepository
	hasher       *password.Hasher
}

func NewCustomerService(
	tx *mysql.TxRunner,
	customerRepo *mysql.CustomerRepository,
	passwordRepo *mysql.CustomerPasswordRepository,
	hasher *password.Hasher,
) *CustomerService {
	return &CustomerService{
		tx:           tx,
		customerRepo: customerRepo,
		passwordRepo: passwordRepo,
		hasher:       hasher,
	}
}

// SignUp registers a customer with an email and a password.
//
// The customer row and the password row are written in one transaction: a
// customer that exists without a password could never log in and could never
// register that email again.
func (s *CustomerService) SignUp(ctx context.Context, req domain.SignUpRequest) (*domain.Customer, error) {
	// Hash before opening the transaction. Argon2id deliberately takes tens of
	// milliseconds, and holding a transaction open for it would pin a
	// connection and lock rows for no reason.
	hash, err := s.hasher.Hash(req.Password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	var customer *domain.Customer
	runErr := s.tx.Run(ctx, func(db mysql.DB) error {
		customerRepo := s.customerRepo.WithTx(db)
		passwordRepo := s.passwordRepo.WithTx(db)

		// A friendly early answer for the common case. The unique index is
		// what actually guarantees it — two concurrent signups both pass this
		// check, and the loser fails on insert below.
		_, lookupErr := customerRepo.FindByEmail(ctx, req.Email)
		switch {
		case lookupErr == nil:
			return domain.ErrEmailTaken
		case !errors.Is(lookupErr, domain.ErrCustomerNotFound):
			return lookupErr
		}

		id, createErr := customerRepo.Create(ctx, req.Email, req.Name, domain.CustomerStatusActive)
		if createErr != nil {
			return createErr
		}
		if pwErr := passwordRepo.Create(ctx, id, hash); pwErr != nil {
			return pwErr
		}

		var findErr error
		customer, findErr = customerRepo.FindByID(ctx, id)
		return findErr
	})
	if runErr != nil {
		return nil, runErr
	}
	return customer, nil
}

// GetCustomer returns one customer by id.
func (s *CustomerService) GetCustomer(ctx context.Context, id uint64) (*domain.Customer, error) {
	return s.customerRepo.FindByID(ctx, id)
}
