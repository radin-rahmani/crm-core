package repository

import (
	"crm-core/internal/domain"
	"database/sql"
)

type PostgresCustomerRepo struct {
	db *sql.DB
}

func NewPostgresCustomerRepo(db *sql.DB) *PostgresCustomerRepo {
	return &PostgresCustomerRepo{db: db}
}

func (r *PostgresCustomerRepo) Create(c *domain.Customer) error {
	query := `INSERT INTO customers (id, name, email, created_at) VALUES ($1, $2, $3, $4)`
	_, err := r.db.Exec(query, c.ID, c.Name, c.Email, c.CreatedAt)
	return err
}

func (r *PostgresCustomerRepo) GetByID(id string) (*domain.Customer, error) {
	query := `SELECT id, name, email, created_at FROM customers WHERE id = $1`
	row := r.db.QueryRow(query, id)

	var c domain.Customer
	if err := row.Scan(&c.ID, &c.Name, &c.Email, &c.CreatedAt); err != nil {
		return nil, err
	}
	return &c, nil
}
