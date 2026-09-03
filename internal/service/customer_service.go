package service

import (
	"context"
	"errors"
	"fmt"

	"golang-gin/internal/domain"
	"golang-gin/internal/pkg/oauth"
	"golang-gin/internal/pkg/password"
	"golang-gin/internal/pkg/session"
	"golang-gin/internal/repository/mysql"
)

// CustomerService holds the customer use cases. Repositories are concrete
// types, not interfaces.
type CustomerService struct {
	tx           *mysql.TxRunner
	customerRepo *mysql.CustomerRepository
	passwordRepo *mysql.CustomerPasswordRepository
	socialRepo   *mysql.CustomerSocialRepository
	hasher       *password.Hasher
	oauth        *oauth.Client
	sessions     *session.Store
}

func NewCustomerService(
	tx *mysql.TxRunner,
	customerRepo *mysql.CustomerRepository,
	passwordRepo *mysql.CustomerPasswordRepository,
	socialRepo *mysql.CustomerSocialRepository,
	hasher *password.Hasher,
	oauthClient *oauth.Client,
	sessions *session.Store,
) *CustomerService {
	return &CustomerService{
		tx:           tx,
		customerRepo: customerRepo,
		passwordRepo: passwordRepo,
		socialRepo:   socialRepo,
		hasher:       hasher,
		oauth:        oauthClient,
		sessions:     sessions,
	}
}

// SignUp registers a customer through whichever flow the request asks for.
func (s *CustomerService) SignUp(ctx context.Context, req domain.SignUpRequest) (*domain.Customer, error) {
	if req.MediaName.IsSocial() {
		return s.signUpSocial(ctx, req)
	}
	return s.signUpEmail(ctx, req)
}

// signUpEmail writes the customer row and the password row in one
// transaction: a customer that exists without a password could never log in
// and could never register that email again.
func (s *CustomerService) signUpEmail(ctx context.Context, req domain.SignUpRequest) (*domain.Customer, error) {
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

		if err := s.ensureEmailFree(ctx, customerRepo, req.Email); err != nil {
			return err
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

// signUpSocial verifies the provider token first, then writes the customer and
// the social link. No password row is created: for these customers the
// provider is the credential.
func (s *CustomerService) signUpSocial(ctx context.Context, req domain.SignUpRequest) (*domain.Customer, error) {
	// The network call happens before the transaction: it can take seconds and
	// must not hold database locks.
	profile, err := s.oauth.Profile(ctx, req.MediaName, req.MediaToken)
	if err != nil {
		return nil, err
	}

	// The token proves who the caller is; mediaId is only their claim about it.
	// Without this check anyone holding a valid token could register an account
	// bound to somebody else's social id.
	if profile.ID != req.MediaID {
		return nil, domain.ErrMediaIDMismatch
	}

	// The provider is authoritative for both. The request values are a fallback
	// for providers that withhold them — Kakao omits the email unless the user
	// granted that scope.
	name := req.Name
	if profile.Name != "" {
		name = profile.Name
	}
	email := req.Email
	if profile.Email != "" {
		email = profile.Email
	}
	if name == "" {
		return nil, fmt.Errorf("%w: provider returned no name and none was supplied", domain.ErrSocialVerification)
	}

	var customer *domain.Customer
	runErr := s.tx.Run(ctx, func(db mysql.DB) error {
		customerRepo := s.customerRepo.WithTx(db)
		socialRepo := s.socialRepo.WithTx(db)

		// Already linked to someone: the unique index on
		// (media_name, media_id) is what actually enforces this.
		_, socialErr := socialRepo.FindByMedia(ctx, string(req.MediaName), profile.ID)
		switch {
		case socialErr == nil:
			return domain.ErrSocialAlreadyTaken
		case !errors.Is(socialErr, domain.ErrSocialNotFound):
			return socialErr
		}

		if err := s.ensureEmailFree(ctx, customerRepo, email); err != nil {
			return err
		}

		id, createErr := customerRepo.Create(ctx, email, name, domain.CustomerStatusActive)
		if createErr != nil {
			return createErr
		}
		if _, linkErr := socialRepo.Create(ctx, id, string(req.MediaName), profile.ID); linkErr != nil {
			return linkErr
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

// ensureEmailFree gives a clear answer for the common case. The unique index
// is what actually guarantees it — two concurrent signups both pass this
// check, and the loser fails on insert.
func (s *CustomerService) ensureEmailFree(ctx context.Context, repo *mysql.CustomerRepository, email string) error {
	_, err := repo.FindByEmail(ctx, email)
	switch {
	case err == nil:
		return domain.ErrEmailTaken
	case !errors.Is(err, domain.ErrCustomerNotFound):
		return err
	default:
		return nil
	}
}

// Login authenticates a customer and issues a session.
func (s *CustomerService) Login(ctx context.Context, req domain.LoginRequest) (*domain.Customer, *session.Session, error) {
	var (
		customer *domain.Customer
		err      error
	)
	if req.MediaName.IsSocial() {
		customer, err = s.loginSocial(ctx, req)
	} else {
		customer, err = s.loginEmail(ctx, req)
	}
	if err != nil {
		return nil, nil, err
	}

	if customer.Status != domain.CustomerStatusActive {
		return nil, nil, domain.ErrCustomerInactive
	}

	issued, err := s.sessions.Issue(ctx, customer.ID)
	if err != nil {
		return nil, nil, err
	}
	return customer, issued, nil
}

// loginEmail checks the password against the stored Argon2id hash.
//
// A missing customer and a wrong password both return the same error on
// purpose: distinguishing them would turn this endpoint into a way to find out
// which email addresses are registered.
func (s *CustomerService) loginEmail(ctx context.Context, req domain.LoginRequest) (*domain.Customer, error) {
	customer, hash, err := s.customerRepo.FindWithPasswordByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, domain.ErrCustomerNotFound) {
			return nil, domain.ErrInvalidCredential
		}
		return nil, err
	}

	if err := s.hasher.Verify(req.Password, hash); err != nil {
		if errors.Is(err, password.ErrMismatch) {
			return nil, domain.ErrInvalidCredential
		}
		return nil, fmt.Errorf("verify password: %w", err)
	}

	// Parameters were raised since this hash was made, so replace it while the
	// plaintext is in hand. A failure here must not fail the login: the
	// password was correct.
	if s.hasher.NeedsRehash(hash) {
		if fresh, hashErr := s.hasher.Hash(req.Password); hashErr == nil {
			_ = s.passwordRepo.Update(ctx, customer.ID, fresh)
		}
	}

	return customer, nil
}

// loginSocial verifies the provider token the same way sign-up does, then
// finds the customer through the social link.
//
// The profile is authoritative for name and email, so a change made at the
// provider lands here on the next login.
func (s *CustomerService) loginSocial(ctx context.Context, req domain.LoginRequest) (*domain.Customer, error) {
	profile, err := s.oauth.Profile(ctx, req.MediaName, req.MediaToken)
	if err != nil {
		return nil, err
	}
	if profile.ID != req.MediaID {
		return nil, domain.ErrMediaIDMismatch
	}

	customer, err := s.customerRepo.FindByMedia(ctx, string(req.MediaName), profile.ID)
	if err != nil {
		if errors.Is(err, domain.ErrCustomerNotFound) {
			// Verified, but never signed up. Say so rather than pretending the
			// credentials were wrong: the client should send them to sign-up.
			return nil, domain.ErrCustomerNotFound
		}
		return nil, err
	}

	name, email := customer.Name, customer.Email
	if profile.Name != "" {
		name = profile.Name
	}
	if profile.Email != "" {
		email = profile.Email
	}
	if name == customer.Name && email == customer.Email {
		return customer, nil
	}

	if err := s.customerRepo.UpdateProfile(ctx, customer.ID, name, email); err != nil {
		// Someone else already owns that email. The login itself is valid, so
		// let it through with the profile unchanged rather than locking the
		// customer out over a rename.
		if errors.Is(err, domain.ErrEmailTaken) {
			return customer, nil
		}
		return nil, err
	}

	customer.Name, customer.Email = name, email
	return customer, nil
}

// GetCustomer returns one customer by id.
func (s *CustomerService) GetCustomer(ctx context.Context, id uint64) (*domain.Customer, error) {
	return s.customerRepo.FindByID(ctx, id)
}
