package domain

import "time"

type Customer struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type CustomerRepository interface {
	Create(customer *Customer) error
	GetByID(id string) (*Customer, error)
}
