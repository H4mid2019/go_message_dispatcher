package repository

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"

	"github.com/go-message-dispatcher/internal/domain"
)

type PostgreSQLMessageRepository struct {
	db *sql.DB
}

func NewPostgreSQLMessageRepository(db *sql.DB) *PostgreSQLMessageRepository {
	return &PostgreSQLMessageRepository{db: db}
}

func (r *PostgreSQLMessageRepository) GetUnsentMessages(ctx context.Context, limit int) ([]*domain.Message, error) {
	query := `
		SELECT id, phone_number, content, sent, created_at 
		FROM messages 
		WHERE sent = FALSE 
		AND phone_number IS NOT NULL 
		AND phone_number != '' 
		AND content IS NOT NULL 
		AND content != '' 
		AND LENGTH(phone_number) BETWEEN 10 AND 20
		AND LENGTH(content) <= 160
		ORDER BY created_at ASC, id ASC 
		LIMIT $1 
		FOR UPDATE SKIP LOCKED`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query unsent messages: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var messages []*domain.Message
	for rows.Next() {
		message := &domain.Message{}
		scanErr := rows.Scan(
			&message.ID,
			&message.PhoneNumber,
			&message.Content,
			&message.Sent,
			&message.CreatedAt,
		)
		if scanErr != nil {
			return nil, fmt.Errorf("failed to scan message row: %w", scanErr)
		}
		messages = append(messages, message)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error during row iteration: %w", err)
	}

	return messages, nil
}

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
	defer func() { _ = rows.Close() }()

	var messages []*domain.Message
	for rows.Next() {
		message := &domain.Message{}
		scanErr := rows.Scan(
			&message.ID,
			&message.PhoneNumber,
			&message.Content,
			&message.Sent,
			&message.CreatedAt,
		)
		if scanErr != nil {
			return nil, fmt.Errorf("failed to scan message row: %w", scanErr)
		}
		messages = append(messages, message)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error during row iteration: %w", err)
	}

	return messages, nil
}

func (r *PostgreSQLMessageRepository) CreateMessage(ctx context.Context, phoneNumber, content string) (*domain.Message, error) {
	// Validate content length before insertion
	if len(content) > 160 {
		return nil, fmt.Errorf("content exceeds maximum length of 160 characters (got %d)", len(content))
	}
	if content == "" {
		return nil, fmt.Errorf("content cannot be empty")
	}

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

func (r *PostgreSQLMessageRepository) CheckConnection(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

func (r *PostgreSQLMessageRepository) CheckHealth(ctx context.Context) error {
	return r.db.PingContext(ctx)
}
