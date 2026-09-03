package domain

import "errors"

var (
	ErrCustomerNotFound   = errors.New("customer not found")
	ErrEmailTaken         = errors.New("email already registered")
	ErrInvalidStatus      = errors.New("invalid customer status")
	ErrSocialNotFound     = errors.New("customer social not found")
	ErrSocialAlreadyTaken = errors.New("social account already linked")
	ErrPasswordNotSet     = errors.New("password not set")
	ErrPasswordAlreadySet = errors.New("password already set")
	ErrInvalidCredential  = errors.New("invalid email or password")
	ErrSocialVerification = errors.New("social token verification failed")
	ErrMediaIDMismatch    = errors.New("media id does not match the verified profile")
	ErrUnsupportedMedia   = errors.New("unsupported media name")
	ErrCustomerInactive   = errors.New("customer is not active")
)
