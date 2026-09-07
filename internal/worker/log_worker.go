package worker

import (
	"crm-core/internal/domain"
	"fmt"
	"log"
	"sync"
	"time"
)

type LogWorkerPool struct {
	logChannel    chan domain.LogEntry
	repo          domain.LogRepository
	batchSize     int
	flushInterval time.Duration
	workerCount   int
	wg            sync.WaitGroup
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
		wp.wg.Add(1)
		go wp.worker()
	}
}

// Stop closes the intake channel and blocks until every worker has drained
// its remaining buffered entries and flushed them to the repository. Callers
// must ensure no further Enqueue calls happen after Stop is invoked (i.e.
// stop the HTTP/gRPC servers first).
func (wp *LogWorkerPool) Stop() {
	close(wp.logChannel)
	wp.wg.Wait()
}

func (wp *LogWorkerPool) worker() {
	defer wp.wg.Done()

	var batch []domain.LogEntry
	ticker := time.NewTicker(wp.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case logEntry, ok := <-wp.logChannel:
			if !ok {
				// Channel closed (Stop was called): flush whatever is left
				// and exit the worker.
				if len(batch) > 0 {
					wp.flushWithRetry(batch)
				}
				return
			}
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
