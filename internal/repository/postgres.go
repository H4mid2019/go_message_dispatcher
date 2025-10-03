// Package repository provides data access implementations for PostgreSQL and Redis.
// It implements the domain interfaces for persistent data storage and caching.
package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/go-message-dispatcher/internal/domain"
	_ "github.com/lib/pq" // PostgreSQL driver
)

// PostgreSQLMessageRepository implements MessageRepository using PostgreSQL
type PostgreSQLMessageRepository struct {
	db *sql.DB
}

// NewPostgreSQLMessageRepository creates a new PostgreSQL message repository
func NewPostgreSQLMessageRepository(db *sql.DB) *PostgreSQLMessageRepository {
	return &PostgreSQLMessageRepository{db: db}
}

// GetUnsentMessages retrieves unsent messages in FIFO order (oldest first)
// This ensures fairest processing and prevents message starvation
func (r *PostgreSQLMessageRepository) GetUnsentMessages(ctx context.Context, limit int) ([]*domain.Message, error) {
	query := `
		SELECT id, phone_number, content, sent, created_at 
		FROM messages 
		WHERE sent = FALSE 
		ORDER BY created_at ASC 
		LIMIT $1`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query unsent messages: %w", err)
	}
	defer rows.Close()

	var messages []*domain.Message
	for rows.Next() {
		message := &domain.Message{}
		err := rows.Scan(
			&message.ID,
			&message.PhoneNumber,
			&message.Content,
			&message.Sent,
			&message.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan message row: %w", err)
		}
		messages = append(messages, message)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error during row iteration: %w", err)
	}

	return messages, nil
}

// MarkAsSent updates a message's sent status to true
// Uses optimistic locking to prevent race conditions in concurrent processing
func (r *PostgreSQLMessageRepository) MarkAsSent(ctx context.Context, messageID int) error {
	query := `UPDATE messages SET sent = TRUE WHERE id = $1 AND sent = FALSE`

	result, err := r.db.ExecContext(ctx, query, messageID)
	if err != nil {
		return fmt.Errorf("failed to mark message as sent: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("message %d was not found or already sent", messageID)
	}

	return nil
}

// GetSentMessages retrieves all messages that have been successfully sent
// Ordered by creation time for consistent API responses
func (r *PostgreSQLMessageRepository) GetSentMessages(ctx context.Context) ([]*domain.Message, error) {
	query := `
		SELECT id, phone_number, content, sent, created_at 
		FROM messages 
		WHERE sent = TRUE 
		ORDER BY created_at ASC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query sent messages: %w", err)
	}
	defer rows.Close()

	var messages []*domain.Message
	for rows.Next() {
		message := &domain.Message{}
		err := rows.Scan(
			&message.ID,
			&message.PhoneNumber,
			&message.Content,
			&message.Sent,
			&message.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan message row: %w", err)
		}
		messages = append(messages, message)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error during row iteration: %w", err)
	}

	return messages, nil
}

// CreateMessage adds a new message to the database queue
// Primarily used for testing and administrative purposes
func (r *PostgreSQLMessageRepository) CreateMessage(ctx context.Context, phoneNumber, content string) (*domain.Message, error) {
	query := `
		INSERT INTO messages (phone_number, content) 
		VALUES ($1, $2) 
		RETURNING id, phone_number, content, sent, created_at`

	message := &domain.Message{}
	err := r.db.QueryRowContext(ctx, query, phoneNumber, content).Scan(
		&message.ID,
		&message.PhoneNumber,
		&message.Content,
		&message.Sent,
		&message.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create message: %w", err)
	}

	return message, nil
}

// CheckConnection verifies the database connection is healthy
func (r *PostgreSQLMessageRepository) CheckConnection(ctx context.Context) error {
	return r.db.PingContext(ctx)
}
