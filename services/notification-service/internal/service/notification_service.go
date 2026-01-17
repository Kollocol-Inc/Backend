package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"time"

	"notification-service/internal/repository"
	"notification-service/pkg/email"
	pb "notification-service/proto"
)

type NotificationService struct {
	pb.UnimplementedNotificationServiceServer
	repo       *repository.NotificationRepository
	smtpClient *email.SMTPClient
}

func NewNotificationService(db *sql.DB, smtpClient *email.SMTPClient) *NotificationService {
	return &NotificationService{
		repo:       repository.NewNotificationRepository(db),
		smtpClient: smtpClient,
	}
}

func (s *NotificationService) GetNotifications(ctx context.Context, req *pb.GetNotificationsRequest) (*pb.GetNotificationsResponse, error) {
	limit := max(req.Limit, 1)
	offset := max(req.Offset, 0)

	notifications, total, err := s.repo.GetNotifications(ctx, req.UserId, int(limit), int(offset))
	if err != nil {
		log.Printf("Failed to get notifications: %v", err)
		return &pb.GetNotificationsResponse{
			Notifications: []*pb.Notification{},
			Total:         0,
		}, nil
	}

	pbNotifications := make([]*pb.Notification, 0, len(notifications))
	for _, n := range notifications {
		pbNotifications = append(pbNotifications, &pb.Notification{
			Id:        n.ID,
			UserId:    n.UserID,
			Type:      n.Type,
			Title:     n.Title,
			Content:   n.Content,
			IsRead:    n.IsRead,
			CreatedAt: n.CreatedAt.Format(time.RFC3339),
		})
	}

	return &pb.GetNotificationsResponse{
		Notifications: pbNotifications,
		Total:         int32(total),
	}, nil
}

func (s *NotificationService) MarkAsRead(ctx context.Context, req *pb.MarkAsReadRequest) (*pb.MarkAsReadResponse, error) {
	if err := s.repo.MarkAsRead(ctx, req.NotificationId, req.UserId); err != nil {
		log.Printf("Failed to mark notification as read: %v", err)
		return &pb.MarkAsReadResponse{
			Success: false,
			Message: "Failed to mark notification as read",
		}, nil
	}

	return &pb.MarkAsReadResponse{
		Success: true,
		Message: "Notification marked as read",
	}, nil
}

func (s *NotificationService) DeleteNotification(ctx context.Context, req *pb.DeleteNotificationRequest) (*pb.DeleteNotificationResponse, error) {
	if err := s.repo.DeleteNotification(ctx, req.NotificationId, req.UserId); err != nil {
		log.Printf("Failed to delete notification: %v", err)
		return &pb.DeleteNotificationResponse{
			Success: false,
			Message: "Failed to delete notification",
		}, nil
	}

	return &pb.DeleteNotificationResponse{
		Success: true,
		Message: "Notification deleted",
	}, nil
}

func (s *NotificationService) HandleSendAuthCode(ctx context.Context, data []byte) error {
	var event struct {
		Email string `json:"email"`
		Code  string `json:"code"`
	}

	if err := json.Unmarshal(data, &event); err != nil {
		return err
	}

	log.Printf("Sending auth code to %s", event.Email)
	return s.smtpClient.SendAuthCode(event.Email, event.Code)
}

func (s *NotificationService) HandleGroupInvite(ctx context.Context, data []byte) error {
	var event struct {
		GroupID      string `json:"group_id"`
		GroupName    string `json:"group_name"`
		InviterName  string `json:"inviter_name"`
		InviteeEmail string `json:"invitee_email"`
	}

	if err := json.Unmarshal(data, &event); err != nil {
		return err
	}

	log.Printf("Sending group invite to %s for group %s", event.InviteeEmail, event.GroupName)
	return s.smtpClient.SendGroupInvite(event.InviteeEmail, event.GroupName, event.InviterName)
}

func (s *NotificationService) HandleQuizCreated(ctx context.Context, data []byte) error {
	var event struct {
		InstanceID   string   `json:"instance_id"`
		Title        string   `json:"title"`
		GroupID      string   `json:"group_id"`
		CreatorID    string   `json:"creator_id"`
		Deadline     string   `json:"deadline"`
		Participants []string `json:"participants"`
	}

	if err := json.Unmarshal(data, &event); err != nil {
		return err
	}

	log.Printf("Processing quiz_created event for instance %s", event.InstanceID)

	// Create in-app notifications for participants
	for _, userID := range event.Participants {
		notification := &repository.Notification{
			UserID:  userID,
			Type:    "quiz_created",
			Title:   "New Quiz Available",
			Content: event.Title,
			IsRead:  false,
		}

		if err := s.repo.CreateNotification(ctx, notification); err != nil {
			log.Printf("Failed to create notification for user %s: %v", userID, err)
		}
	}

	return nil
}

func (s *NotificationService) HandleQuizResultsReady(ctx context.Context, data []byte) error {
	var event struct {
		InstanceID     string   `json:"instance_id"`
		ParticipantIDs []string `json:"participant_ids"`
		Title          string   `json:"title"`
	}

	if err := json.Unmarshal(data, &event); err != nil {
		return err
	}

	log.Printf("Processing quiz_results_ready event for instance %s", event.InstanceID)

	// Create in-app notifications for participants
	for _, userID := range event.ParticipantIDs {
		notification := &repository.Notification{
			UserID:  userID,
			Type:    "quiz_results",
			Title:   "Quiz Results Ready",
			Content: event.Title,
			IsRead:  false,
		}

		if err := s.repo.CreateNotification(ctx, notification); err != nil {
			log.Printf("Failed to create notification for user %s: %v", userID, err)
		}
	}

	return nil
}

func (s *NotificationService) HandleSendEmail(ctx context.Context, data []byte) error {
	var event struct {
		To       string `json:"to"`
		Subject  string `json:"subject"`
		Body     string `json:"body"`
		Template string `json:"template"`
	}

	if err := json.Unmarshal(data, &event); err != nil {
		return err
	}

	log.Printf("Sending email to %s with subject: %s", event.To, event.Subject)
	return s.smtpClient.SendEmail(email.EmailData{
		To:      event.To,
		Subject: event.Subject,
		Body:    event.Body,
	})
}

func (s *NotificationService) HandleCreateNotification(ctx context.Context, data []byte) error {
	var event struct {
		UserID  string `json:"user_id"`
		Type    string `json:"type"`
		Title   string `json:"title"`
		Content string `json:"content"`
	}

	if err := json.Unmarshal(data, &event); err != nil {
		return err
	}

	log.Printf("Creating notification for user %s: %s", event.UserID, event.Title)

	notification := &repository.Notification{
		UserID:  event.UserID,
		Type:    event.Type,
		Title:   event.Title,
		Content: event.Content,
		IsRead:  false,
	}

	return s.repo.CreateNotification(ctx, notification)
}
