package service

import (
	"crm-core/internal/domain"
	"crm-core/internal/worker"
	"time"

	"github.com/google/uuid"
)

type CustomerService struct {
	repo       domain.CustomerRepository
	workerPool *worker.LogWorkerPool
}

func NewCustomerService(repo domain.CustomerRepository, wp *worker.LogWorkerPool) *CustomerService {
	return &CustomerService{repo: repo, workerPool: wp}
}

func (s *CustomerService) RegisterCustomer(name, email string) (*domain.Customer, error) {
	c := &domain.Customer{
		ID:        uuid.New().String(),
		Name:      name,
		Email:     email,
		CreatedAt: time.Now(),
	}

	if err := s.repo.Create(c); err != nil {
		return nil, err
	}

	s.workerPool.Enqueue(domain.LogEntry{
		UserID:    c.ID,
		Action:    "CUSTOMER_CREATED",
		Details:   "Customer registered successfully",
		CreatedAt: time.Now(),
	})

	return c, nil
}

func (s *CustomerService) GetCustomerProfile(id string) (*domain.Customer, error) {
	return s.repo.GetByID(id)
}
