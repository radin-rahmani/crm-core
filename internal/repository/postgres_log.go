package repository

import (
	"crm-core/internal/domain"
	"database/sql"
	"fmt"
	"strings"
)

type PostgresLogRepo struct {
	db *sql.DB
}

func NewPostgresLogRepo(db *sql.DB) *PostgresLogRepo {
	return &PostgresLogRepo{db: db}
}

func (r *PostgresLogRepo) BulkInsert(logs []domain.LogEntry) error {
	if len(logs) == 0 {
		return nil
	}

	valueStrings := make([]string, 0, len(logs))
	valueArgs := make([]interface{}, 0, len(logs)*4)

	for i, entry := range logs {
		n := i * 4
		valueStrings = append(valueStrings, fmt.Sprintf("($%d, $%d, $%d, $%d)", n+1, n+2, n+3, n+4))
		valueArgs = append(valueArgs, entry.UserID, entry.Action, entry.Details, entry.CreatedAt)
	}

	stmt := fmt.Sprintf("INSERT INTO logs (user_id, action, details, created_at) VALUES %s", strings.Join(valueStrings, ","))
	_, err := r.db.Exec(stmt, valueArgs...)
	return err
}
