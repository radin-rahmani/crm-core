package domain

import "time"

type LogEntry struct {
	UserID    string
	Action    string
	Details   string
	CreatedAt time.Time
}

type LogRepository interface {
	BulkInsert(logs []LogEntry) error
}
