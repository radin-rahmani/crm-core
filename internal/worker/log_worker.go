package worker

import (
	"crm-core/internal/domain"
	"fmt"
	"log"
	"time"
)

type LogWorkerPool struct {
	logChannel    chan domain.LogEntry
	repo          domain.LogRepository
	batchSize     int
	flushInterval time.Duration
	workerCount   int
}

func NewLogWorkerPool(repo domain.LogRepository, bufferSize, batchSize, workerCount int, flushInterval time.Duration) *LogWorkerPool {
	return &LogWorkerPool{
		logChannel:    make(chan domain.LogEntry, bufferSize),
		repo:          repo,
		batchSize:     batchSize,
		flushInterval: flushInterval,
		workerCount:   workerCount,
	}
}

func (wp *LogWorkerPool) Enqueue(entry domain.LogEntry) {
	wp.logChannel <- entry
}

func (wp *LogWorkerPool) Start() {
	for i := 0; i < wp.workerCount; i++ {
		go wp.worker()
	}
}

func (wp *LogWorkerPool) worker() {
	var batch []domain.LogEntry
	ticker := time.NewTicker(wp.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case logEntry := <-wp.logChannel:
			batch = append(batch, logEntry)
			if len(batch) >= wp.batchSize {
				wp.flushWithRetry(batch)
				batch = make([]domain.LogEntry, 0, wp.batchSize)
			}

		case <-ticker.C:
			if len(batch) > 0 {
				wp.flushWithRetry(batch)
				batch = make([]domain.LogEntry, 0, wp.batchSize)
			}
		}
	}
}

func (wp *LogWorkerPool) flushWithRetry(batch []domain.LogEntry) {
	for {
		err := wp.repo.BulkInsert(batch)
		if err == nil {
			fmt.Printf("[Worker] Batch of %d logs inserted successfully.\n", len(batch))
			return
		}

		log.Printf("[Worker Alert] DB unavailable. Retrying in 2s... Error: %v\n", err)
		time.Sleep(2 * time.Second)
	}
}
