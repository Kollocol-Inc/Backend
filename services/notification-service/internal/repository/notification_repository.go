package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Notification struct {
	ID        string
	UserID    string
	Type      string
	Title     string
	Content   string
	IsRead    bool
	CreatedAt time.Time
}

type NotificationRepository struct {
	db *sql.DB
}

func NewNotificationRepository(db *sql.DB) *NotificationRepository {
	return &NotificationRepository{
		db: db,
	}
}

func (r *NotificationRepository) CreateNotification(ctx context.Context, notification *Notification) error {
	if notification.ID == "" {
		notification.ID = uuid.New().String()
	}
	if notification.CreatedAt.IsZero() {
		notification.CreatedAt = time.Now()
	}

	query := `
		INSERT INTO notifications (id, user_id, type, title, content, is_read, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := r.db.ExecContext(ctx, query,
		notification.ID,
		notification.UserID,
		notification.Type,
		notification.Title,
		notification.Content,
		notification.IsRead,
		notification.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create notification: %w", err)
	}

	return nil
}

func (r *NotificationRepository) GetNotifications(ctx context.Context, userID string, limit, offset int) ([]*Notification, int, error) {
	countQuery := `SELECT COUNT(*) FROM notifications WHERE user_id = $1`
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count notifications: %w", err)
	}

	query := `
		SELECT id, user_id, type, title, content, is_read, created_at
		FROM notifications
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get notifications: %w", err)
	}
	defer rows.Close()

	var notifications []*Notification
	for rows.Next() {
		n := &Notification{}
		if err := rows.Scan(&n.ID, &n.UserID, &n.Type, &n.Title, &n.Content, &n.IsRead, &n.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("failed to scan notification: %w", err)
		}
		notifications = append(notifications, n)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating notifications: %w", err)
	}

	return notifications, total, nil
}

func (r *NotificationRepository) MarkAsRead(ctx context.Context, notificationID, userID string) error {
	query := `
		UPDATE notifications
		SET is_read = true
		WHERE id = $1 AND user_id = $2
	`

	result, err := r.db.ExecContext(ctx, query, notificationID, userID)
	if err != nil {
		return fmt.Errorf("failed to mark notification as read: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("notification not found")
	}

	return nil
}

func (r *NotificationRepository) DeleteNotification(ctx context.Context, notificationID, userID string) error {
	query := `DELETE FROM notifications WHERE id = $1 AND user_id = $2`

	result, err := r.db.ExecContext(ctx, query, notificationID, userID)
	if err != nil {
		return fmt.Errorf("failed to delete notification: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("notification not found")
	}

	return nil
}