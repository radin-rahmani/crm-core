package main

import (
	"crm-core/internal/delivery"
	"crm-core/internal/repository"
	"crm-core/internal/service"
	"crm-core/internal/worker"
	pb "crm-core/proto"
	"database/sql"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	_ "github.com/lib/pq"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:password@postgres:5432/crm_db?sslmode=disable"
	}

	var db *sql.DB
	var err error

	for i := 1; i <= 10; i++ {
		db, err = sql.Open("postgres", dbURL)
		if err == nil {
			err = db.Ping()
		}
		if err == nil {
			break
		}
		log.Printf("Waiting for database connection... attempt %d/10 (error: %v)", i, err)
		time.Sleep(2 * time.Second)
	}

	if err != nil {
		log.Fatalf("Could not connect to database after retries: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("Error closing database: %v", err)
		}
	}()

	if err := initDB(db); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	redisAddr := os.Getenv("REDIS_HOST")
	if redisAddr == "" {
		redisAddr = os.Getenv("REDIS_URL")
	}
	if redisAddr == "" {
		redisAddr = "redis:6379"
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	logRepo := repository.NewPostgresLogRepo(db)
	postgresCustomerRepo := repository.NewPostgresCustomerRepo(db)
	cachedCustomerRepo := repository.NewRedisCustomerAdapter(postgresCustomerRepo, rdb)

	workerPool := worker.NewLogWorkerPool(logRepo, 20000, 500, 5, 2*time.Second)
	workerPool.Start()

	customerService := service.NewCustomerService(cachedCustomerRepo, workerPool)

	httpHandler := delivery.NewHTTPHandler(customerService)
	grpcHandler := delivery.NewGRPCHandler(workerPool)

	go func() {
		http.HandleFunc("/customers", httpHandler.CreateCustomer)
		http.HandleFunc("/customers/", httpHandler.GetCustomer)
		log.Println("HTTP Server running on :8080")
		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen on :50051: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterLoggerServiceServer(grpcServer, grpcHandler)

	reflection.Register(grpcServer)

	log.Println("gRPC Server running on :50051")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("gRPC server failed: %v", err)
	}
}

func initDB(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS customers (id VARCHAR(36) PRIMARY KEY, name VARCHAR(100), email VARCHAR(100) UNIQUE, created_at TIMESTAMP);`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS logs (id SERIAL PRIMARY KEY, user_id VARCHAR(36), action VARCHAR(50), details TEXT, created_at TIMESTAMP);`)
	return err
}
