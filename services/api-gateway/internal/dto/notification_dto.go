package dto

type NotificationDTO struct {
	ID        string `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	UserID    string `json:"user_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Type      string `json:"type" example:"quiz_created"`
	Title     string `json:"title" example:"New Quiz Available"`
	Content   string `json:"content" example:"Math Quiz"`
	IsRead    bool   `json:"is_read" example:"false"`
	CreatedAt string `json:"created_at" example:"2025-01-15T10:00:00Z"`
}

type GetNotificationsResponse struct {
	Notifications []NotificationDTO `json:"notifications"`
	Total         int32             `json:"total" example:"10"`
}