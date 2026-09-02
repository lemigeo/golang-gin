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

// SignUpRequest is the JSON body accepted at POST /auth/signup.
//
// The password bounds are deliberate: a minimum keeps trivially guessable
// values out, and a maximum stops a huge input from turning Argon2id into a
// denial-of-service vector. Argon2id has no 72-byte truncation issue, so the
// ceiling is generous.
type SignUpRequest struct {
	Email    string `json:"email" binding:"required,email,max=255"`
	Password string `json:"password" binding:"required,min=8,max=128"`
	Name     string `json:"name" binding:"required,max=64"`
}
