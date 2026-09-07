package main

import (
	"context"
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
	"os/signal"
	"syscall"
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
	healthHandler := delivery.NewHealthHandler(db, rdb)

	mux := http.NewServeMux()
	mux.HandleFunc("/customers", httpHandler.CreateCustomer)
	mux.HandleFunc("/customers/", httpHandler.GetCustomer)
	mux.HandleFunc("/healthz", healthHandler.Liveness)
	mux.HandleFunc("/readyz", healthHandler.Readiness)

	httpServer := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	go func() {
		log.Println("HTTP Server running on :8080")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
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

	go func() {
		log.Println("gRPC Server running on :50051")
		if err := grpcServer.Serve(lis); err != nil {
			log.Printf("gRPC server stopped: %v", err)
		}
	}()

	// Block until we receive a termination signal (e.g. from `docker stop`
	// or a Kubernetes pod eviction).
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	log.Printf("Received signal %v, shutting down gracefully...", sig)

	shutdown(httpServer, grpcServer, workerPool, db, rdb)
	log.Println("Shutdown complete.")
}

// shutdown drains the service in dependency order: stop accepting new work
// first, then let in-flight work finish, then flush buffered async work,
// and finally close the underlying connections.
func shutdown(httpServer *http.Server, grpcServer *grpc.Server, workerPool *worker.LogWorkerPool, db *sql.DB, rdb *redis.Client) {
	// 1. Stop accepting new HTTP requests; let in-flight ones finish (bounded wait).
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	// 2. Stop accepting new gRPC streams; let in-flight ones finish.
	grpcServer.GracefulStop()

	// 3. Drain the async log worker pool so buffered audit logs aren't lost.
	log.Println("Draining log worker pool...")
	workerPool.Stop()

	// 4. Close downstream connections last, now that nothing needs them.
	if err := db.Close(); err != nil {
		log.Printf("Error closing database: %v", err)
	}
	if err := rdb.Close(); err != nil {
		log.Printf("Error closing redis: %v", err)
	}
}

// initDB creates the schema if it doesn't exist yet. Since multiple replicas
// of this service can start concurrently against a fresh database (e.g. a
// Kubernetes Deployment with replicas > 1), plain "CREATE TABLE IF NOT
// EXISTS" statements are not safe: two replicas can both pass the "does this
// exist" check before either finishes creating the table, causing a
// duplicate-key error on Postgres' internal catalog (pg_type).
//
// A Postgres session-level advisory lock (pg_advisory_lock) serializes this
// setup step across replicas: whichever replica gets the lock first creates
// the tables while the others block, then each of the others acquires the
// lock in turn and finds the tables already exist (a no-op).
//
// Session-level advisory locks are tied to a single backend connection, so
// we must pin one dedicated *sql.Conn for the lock/create/unlock sequence
// instead of using db.Exec (which can pull a different pooled connection
// for each call, in which case the lock wouldn't actually serialize
// anything).
const initDBLockKey = 727142

func initDB(db *sql.DB) error {
	ctx := context.Background()

	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, initDBLockKey); err != nil {
		return err
	}
	defer conn.ExecContext(ctx, `SELECT pg_advisory_unlock($1)`, initDBLockKey)

	_, err = conn.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS customers (id VARCHAR(36) PRIMARY KEY, name VARCHAR(100), email VARCHAR(100) UNIQUE, created_at TIMESTAMP);`)
	if err != nil {
		return err
	}

	_, err = conn.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS logs (id SERIAL PRIMARY KEY, user_id VARCHAR(36), action VARCHAR(50), details TEXT, created_at TIMESTAMP);`)
	return err
}
