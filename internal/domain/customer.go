package domain

import "time"

// CustomerStatus mirrors the customer.status column.
type CustomerStatus uint8

const (
	CustomerStatusDeleted  CustomerStatus = 0
	CustomerStatusActive   CustomerStatus = 1
	CustomerStatusInactive CustomerStatus = 2
)

// Valid reports whether s is one of the defined states. Anything else means the
// row was written by something that does not know this schema.
func (s CustomerStatus) Valid() bool {
	switch s {
	case CustomerStatusDeleted, CustomerStatusActive, CustomerStatusInactive:
		return true
	default:
		return false
	}
}

func (s CustomerStatus) String() string {
	switch s {
	case CustomerStatusDeleted:
		return "deleted"
	case CustomerStatusActive:
		return "active"
	case CustomerStatusInactive:
		return "inactive"
	default:
		return "unknown"
	}
}

// Customer is the entity behind the customer table.
type Customer struct {
	ID        uint64
	Email     string
	Name      string
	Status    CustomerStatus
	CreatedAt time.Time
	UpdatedAt time.Time
}

// CustomerSocial links a customer to one account on an external service.
type CustomerSocial struct {
	ID         uint64
	CustomerID uint64
	MediaName  string
	MediaID    string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// CreateCustomerRequest is the JSON body accepted when registering a customer.
type CreateCustomerRequest struct {
	Email string `json:"email" binding:"required,email,max=255"`
	Name  string `json:"name" binding:"required,max=64"`
}

// UpdateCustomerRequest is the JSON body accepted when renaming a customer.
type UpdateCustomerRequest struct {
	Name string `json:"name" binding:"required,max=64"`
}

// CustomerResponse is what the API returns. The entity is never serialized
// directly so that adding a column does not silently widen the API.
type CustomerResponse struct {
	ID        uint64    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func NewCustomerResponse(c *Customer) CustomerResponse {
	return CustomerResponse{
		ID:        c.ID,
		Email:     c.Email,
		Name:      c.Name,
		Status:    c.Status.String(),
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}

// CustomerSocialResponse is the API shape of a linked social account.
type CustomerSocialResponse struct {
	ID         uint64    `json:"id"`
	CustomerID uint64    `json:"customer_id"`
	MediaName  string    `json:"media_name"`
	MediaID    string    `json:"media_id"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func NewCustomerSocialResponse(s *CustomerSocial) CustomerSocialResponse {
	return CustomerSocialResponse{
		ID:         s.ID,
		CustomerID: s.CustomerID,
		MediaName:  s.MediaName,
		MediaID:    s.MediaID,
		CreatedAt:  s.CreatedAt,
		UpdatedAt:  s.UpdatedAt,
	}
}

// MediaName is how a customer signed up: with an email and password, or
// through a social provider.
type MediaName string

const (
	MediaEmail MediaName = "email"
	MediaKakao MediaName = "kakao"
	MediaNaver MediaName = "naver"
)

// IsSocial reports whether signing up with m requires verifying a provider
// token instead of storing a password.
func (m MediaName) IsSocial() bool {
	return m == MediaKakao || m == MediaNaver
}

// SocialProfile is what a provider returns once a token is verified. Only the
// fields this service actually uses are kept.
type SocialProfile struct {
	ID    string
	Email string
	Name  string
}

// SignUpRequest is the JSON body accepted at POST /auth/signup.
//
// One endpoint serves both flows, and mediaName decides which fields are
// required: "email" needs a password, a social provider needs the media id and
// the token proving it. The bindings encode exactly that, so an impossible
// combination is rejected before any of it reaches the service.
//
// The password bounds are deliberate: a minimum keeps trivially guessable
// values out, and a maximum stops a huge input from turning Argon2id into a
// denial-of-service vector. Argon2id has no 72-byte truncation issue, so the
// ceiling is generous.
type SignUpRequest struct {
	Email     string    `json:"email" binding:"required,email,max=255"`
	MediaName MediaName `json:"mediaName" binding:"required,oneof=email kakao naver"`

	// Email sign-up only.
	Password string `json:"password" binding:"required_if=MediaName email,omitempty,min=8,max=128"`

	// Social sign-up only. The name comes from the verified profile, so it is
	// required only for the email flow.
	Name       string `json:"name" binding:"required_if=MediaName email,max=64"`
	MediaID    string `json:"mediaId" binding:"required_unless=MediaName email,max=255"`
	MediaToken string `json:"mediaToken" binding:"required_unless=MediaName email"`
}

// LoginRequest is the JSON body accepted at POST /auth/login.
//
// Like sign-up, one endpoint serves both flows and mediaName picks which
// fields are required.
type LoginRequest struct {
	MediaName MediaName `json:"mediaName" binding:"required,oneof=email kakao naver"`

	// Email login only.
	Email    string `json:"email" binding:"required_if=MediaName email,omitempty,email,max=255"`
	Password string `json:"password" binding:"required_if=MediaName email,omitempty,max=128"`

	// Social login only.
	MediaID    string `json:"mediaId" binding:"required_unless=MediaName email,max=255"`
	MediaToken string `json:"mediaToken" binding:"required_unless=MediaName email"`
}

// LoginResponse carries the session the client will send back on later calls.
type LoginResponse struct {
	Token     string           `json:"token"`
	ExpiresAt time.Time        `json:"expires_at"`
	Customer  CustomerResponse `json:"customer"`
}
