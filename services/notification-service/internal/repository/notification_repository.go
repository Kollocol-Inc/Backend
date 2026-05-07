package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type Notification struct {
	ID              string
	UserID          string
	Type            string
	Title           string
	Content         string
	IsRead          bool
	CreatedAt       time.Time
	RelatedEntityID string
	RequiresAction  bool
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
		INSERT INTO notifications (id, user_id, type, title, content, is_read, created_at, related_entity_id, requires_action)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	var relatedEntityID sql.NullString
	if notification.RelatedEntityID != "" {
		relatedEntityID = sql.NullString{String: notification.RelatedEntityID, Valid: true}
	}

	_, err := r.db.ExecContext(ctx, query,
		notification.ID,
		notification.UserID,
		notification.Type,
		notification.Title,
		notification.Content,
		notification.IsRead,
		notification.CreatedAt,
		relatedEntityID,
		notification.RequiresAction,
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
		SELECT id, user_id, type, title, content, is_read, created_at, related_entity_id, requires_action
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
		var relatedEntityID sql.NullString
		if err := rows.Scan(&n.ID, &n.UserID, &n.Type, &n.Title, &n.Content, &n.IsRead, &n.CreatedAt, &relatedEntityID, &n.RequiresAction); err != nil {
			return nil, 0, fmt.Errorf("failed to scan notification: %w", err)
		}
		if relatedEntityID.Valid {
			n.RelatedEntityID = relatedEntityID.String
		}
		notifications = append(notifications, n)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating notifications: %w", err)
	}

	return notifications, total, nil
}

func (r *NotificationRepository) MarkAsRead(ctx context.Context, notificationIDs []string, userID string) error {
	if len(notificationIDs) == 0 {
		return nil
	}

	query := `
		UPDATE notifications
		SET is_read = true
		WHERE id = ANY($1) AND user_id = $2 AND requires_action = false
	`

	if _, err := r.db.ExecContext(ctx, query, pq.Array(notificationIDs), userID); err != nil {
		return fmt.Errorf("failed to mark notifications as read: %w", err)
	}

	return nil
}

func (r *NotificationRepository) MarkAsReadByType(ctx context.Context, userID, notifType, relatedEntityID string) error {
	query := `
		UPDATE notifications
		SET is_read = true
		WHERE user_id = $1
		  AND type = $2
		  AND related_entity_id = $3
		  AND is_read = false
	`

	if _, err := r.db.ExecContext(ctx, query, userID, notifType, relatedEntityID); err != nil {
		return fmt.Errorf("failed to mark notifications as read by type: %w", err)
	}

	return nil
}

func (r *NotificationRepository) DeleteNotification(ctx context.Context, notificationIDs []string, userID string) error {
	if len(notificationIDs) == 0 {
		return nil
	}

	query := `DELETE FROM notifications WHERE id = ANY($1) AND user_id = $2`

	if _, err := r.db.ExecContext(ctx, query, pq.Array(notificationIDs), userID); err != nil {
		return fmt.Errorf("failed to delete notifications: %w", err)
	}

	return nil
}

func (r *NotificationRepository) DeleteAllForUser(ctx context.Context, userID string) error {
	query := `DELETE FROM notifications WHERE user_id = $1`

	if _, err := r.db.ExecContext(ctx, query, userID); err != nil {
		return fmt.Errorf("failed to delete all notifications for user: %w", err)
	}

	return nil
}
