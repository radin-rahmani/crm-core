package repository

import (
	"context"
	"crm-core/internal/domain"
	"encoding/json"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
)

type RedisCustomerAdapter struct {
	baseRepo    domain.CustomerRepository
	redisClient *redis.Client
}

func NewRedisCustomerAdapter(baseRepo domain.CustomerRepository, rdb *redis.Client) *RedisCustomerAdapter {
	return &RedisCustomerAdapter{
		baseRepo:    baseRepo,
		redisClient: rdb,
	}
}

func (r *RedisCustomerAdapter) Create(c *domain.Customer) error {
	err := r.baseRepo.Create(c)
	if err != nil {
		return err
	}

	data, _ := json.Marshal(c)
	if err := r.redisClient.Set(context.Background(), "customer:"+c.ID, data, 10*time.Minute).Err(); err != nil {
		log.Printf("[Redis Error] Failed to cache customer on create: %v", err)
	}
	return nil
}

func (r *RedisCustomerAdapter) GetByID(id string) (*domain.Customer, error) {
	val, err := r.redisClient.Get(context.Background(), "customer:"+id).Result()
	if err == nil {
		var c domain.Customer
		if err := json.Unmarshal([]byte(val), &c); err == nil {
			log.Println("[Cache Hit] Customer fetched from Redis")
			return &c, nil
		}
	}

	log.Println("[Cache Miss] Fetching customer from Postgres")
	c, err := r.baseRepo.GetByID(id)
	if err != nil {
		return nil, err
	}

	data, _ := json.Marshal(c)
	if err := r.redisClient.Set(context.Background(), "customer:"+c.ID, data, 10*time.Minute).Err(); err != nil {
		log.Printf("[Redis Error] Failed to set cache: %v", err)
	}

	return c, nil
}
