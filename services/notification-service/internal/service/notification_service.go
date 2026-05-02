package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"time"

	"notification-service/internal/repository"
	"notification-service/pkg/email"
	"notification-service/pkg/errors"
	"notification-service/pkg/lang"
	pb "notification-service/proto"

	"google.golang.org/grpc/codes"
)

type NotificationRepo interface {
	CreateNotification(ctx context.Context, notification *repository.Notification) error
	GetNotifications(ctx context.Context, userID string, limit, offset int) ([]*repository.Notification, int, error)
	MarkAsRead(ctx context.Context, notificationIDs []string, userID string) error
	MarkAsReadByType(ctx context.Context, userID, notifType, relatedEntityID string) error
	DeleteNotification(ctx context.Context, notificationIDs []string, userID string) error
	DeleteAllForUser(ctx context.Context, userID string) error
}

type EmailSender interface {
	SendEmail(data email.EmailData) error
	SendAuthCode(emailAddr, code string, l lang.Lang) error
	SendGroupInvite(emailAddr, groupName, inviterName string, l lang.Lang) error
	SendQuizCreated(emailAddr, quizTitle, creatorName string, l lang.Lang) error
	SendQuizResults(emailAddr, quizTitle string, score, maxScore int, l lang.Lang) error
	SendGradeChanged(emailAddr, quizTitle string, score, maxScore int, l lang.Lang) error
	SendDeadlineReminder(emailAddr, quizTitle, deadline, remainingTime string, l lang.Lang) error
}

type NotificationService struct {
	pb.UnimplementedNotificationServiceServer
	repo       NotificationRepo
	smtpClient EmailSender
}

func NewNotificationService(db *sql.DB, smtpClient *email.SMTPClient) *NotificationService {
	return &NotificationService{
		repo:       repository.NewNotificationRepository(db),
		smtpClient: smtpClient,
	}
}

func NewNotificationServiceWithDeps(repo NotificationRepo, smtpClient EmailSender) *NotificationService {
	return &NotificationService{
		repo:       repo,
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
			Id:              n.ID,
			UserId:          n.UserID,
			Type:            n.Type,
			Title:           n.Title,
			Content:         n.Content,
			IsRead:          n.IsRead,
			CreatedAt:       n.CreatedAt.Format(time.RFC3339),
			RelatedEntityId: n.RelatedEntityID,
		})
	}

	return &pb.GetNotificationsResponse{
		Notifications: pbNotifications,
		Total:         int32(total),
	}, nil
}

func (s *NotificationService) MarkAsRead(ctx context.Context, req *pb.MarkAsReadRequest) (*pb.MarkAsReadResponse, error) {
	if err := s.repo.MarkAsRead(ctx, req.NotificationIds, req.UserId); err != nil {
		log.Printf("Failed to mark notifications as read: %v", err)
		return nil, errors.New(codes.Internal, errors.ReasonNotificationMarkFailed, "Failed to mark notifications as read", nil)
	}

	return &pb.MarkAsReadResponse{}, nil
}

func (s *NotificationService) MarkAsReadByType(ctx context.Context, req *pb.MarkAsReadByTypeRequest) (*pb.MarkAsReadByTypeResponse, error) {
	if req.UserId == "" || req.Type == "" || req.RelatedEntityId == "" {
		return nil, errors.New(codes.InvalidArgument, errors.ReasonUserIDRequired, "user_id, type and related_entity_id are required", nil)
	}

	if err := s.repo.MarkAsReadByType(ctx, req.UserId, req.Type, req.RelatedEntityId); err != nil {
		log.Printf("Failed to mark notifications as read by type: %v", err)
		return nil, errors.New(codes.Internal, errors.ReasonNotificationMarkFailed, "Failed to mark notifications as read", nil)
	}

	return &pb.MarkAsReadByTypeResponse{}, nil
}

func (s *NotificationService) DeleteNotification(ctx context.Context, req *pb.DeleteNotificationRequest) (*pb.DeleteNotificationResponse, error) {
	if err := s.repo.DeleteNotification(ctx, req.NotificationIds, req.UserId); err != nil {
		log.Printf("Failed to delete notifications: %v", err)
		return nil, errors.New(codes.Internal, errors.ReasonNotificationDeleteFailed, "Failed to delete notifications", nil)
	}

	return &pb.DeleteNotificationResponse{}, nil
}

func (s *NotificationService) DeleteAllForUser(ctx context.Context, req *pb.DeleteAllForUserRequest) (*pb.DeleteAllForUserResponse, error) {
	if req.UserId == "" {
		return nil, errors.New(codes.InvalidArgument, errors.ReasonUserIDRequired, "User ID is required", nil)
	}

	if err := s.repo.DeleteAllForUser(ctx, req.UserId); err != nil {
		log.Printf("DeleteAllForUser: failed for user %s: %v", req.UserId, err)
		return nil, errors.New(codes.Internal, errors.ReasonNotificationDeleteFailed, "Failed to delete notifications", map[string]string{"user_id": req.UserId})
	}

	return &pb.DeleteAllForUserResponse{}, nil
}

func (s *NotificationService) HandleSendAuthCode(ctx context.Context, data []byte) error {
	var event struct {
		Email    string `json:"email"`
		Code     string `json:"code"`
		Language string `json:"language"`
	}

	if err := json.Unmarshal(data, &event); err != nil {
		return err
	}

	l := lang.Parse(event.Language)
	log.Printf("Sending auth code to %s [lang=%s]", event.Email, l)
	return s.smtpClient.SendAuthCode(event.Email, event.Code, l)
}

func (s *NotificationService) HandleGroupInvite(ctx context.Context, data []byte) error {
	var event struct {
		GroupID       string `json:"group_id"`
		GroupName     string `json:"group_name"`
		InviterName   string `json:"inviter_name"`
		InviteeEmail  string `json:"invitee_email"`
		InviteeUserID string `json:"invitee_user_id"`
		Language      string `json:"language"`
	}

	if err := json.Unmarshal(data, &event); err != nil {
		return err
	}

	l := lang.Parse(event.Language)
	log.Printf("Processing group_invite: group=%s, invitee=%s, lang=%s", event.GroupName, event.InviteeEmail, l)

	if event.InviteeUserID != "" {
		notification := &repository.Notification{
			UserID:          event.InviteeUserID,
			Type:            email.TmplGroupInvite,
			Title:           email.InAppTitle(l, email.TmplGroupInvite),
			Content:         email.InAppContent(l, email.TmplGroupInvite, event.InviterName, event.GroupName),
			IsRead:          false,
			RelatedEntityID: event.GroupID,
		}
		if err := s.repo.CreateNotification(ctx, notification); err != nil {
			log.Printf("HandleGroupInvite: failed to create in-app notification for user %s: %v", event.InviteeUserID, err)
		}
	}

	if event.InviteeEmail == "" {
		return nil
	}
	return s.smtpClient.SendGroupInvite(event.InviteeEmail, event.GroupName, event.InviterName, l)
}

func (s *NotificationService) HandleQuizCreated(ctx context.Context, data []byte) error {
	var event struct {
		InstanceID   string `json:"instance_id"`
		Title        string `json:"title"`
		GroupID      string `json:"group_id"`
		CreatorID    string `json:"creator_id"`
		CreatorName  string `json:"creator_name"`
		Deadline     string `json:"deadline"`
		Participants []struct {
			UserID   string `json:"user_id"`
			Email    string `json:"email"`
			Language string `json:"language"`
		} `json:"participants"`
	}

	if err := json.Unmarshal(data, &event); err != nil {
		return err
	}

	log.Printf("Processing quiz_created event for instance %s, %d participants", event.InstanceID, len(event.Participants))

	for _, p := range event.Participants {
		l := lang.Parse(p.Language)
		notification := &repository.Notification{
			UserID:  p.UserID,
			Type:    email.TmplQuizCreated,
			Title:   email.InAppTitle(l, email.TmplQuizCreated),
			Content: event.Title,
			IsRead:  false,
		}
		if err := s.repo.CreateNotification(ctx, notification); err != nil {
			log.Printf("HandleQuizCreated: failed to create in-app notification for user %s: %v", p.UserID, err)
		}

		if p.Email == "" {
			continue
		}
		if err := s.smtpClient.SendQuizCreated(p.Email, event.Title, event.CreatorName, l); err != nil {
			log.Printf("HandleQuizCreated: failed to send email to %s: %v", p.Email, err)
		}
	}

	return nil
}

func (s *NotificationService) HandleQuizDeadlineReminder(ctx context.Context, data []byte) error {
	var event struct {
		InstanceID     string `json:"instance_id"`
		Title          string `json:"title"`
		Deadline       string `json:"deadline"`
		ReminderOffset string `json:"reminder_offset"`
		Participants   []struct {
			UserID   string `json:"user_id"`
			Email    string `json:"email"`
			Language string `json:"language"`
		} `json:"participants"`
	}

	if err := json.Unmarshal(data, &event); err != nil {
		return err
	}

	log.Printf("Processing deadline_reminder for instance %s, offset %s, %d participants",
		event.InstanceID, event.ReminderOffset, len(event.Participants))

	for _, p := range event.Participants {
		l := lang.Parse(p.Language)
		notification := &repository.Notification{
			UserID:  p.UserID,
			Type:    email.TmplDeadlineReminder,
			Title:   email.InAppTitle(l, email.TmplDeadlineReminder),
			Content: email.InAppContent(l, email.TmplDeadlineReminder, event.Title, event.ReminderOffset),
			IsRead:  false,
		}
		if err := s.repo.CreateNotification(ctx, notification); err != nil {
			log.Printf("HandleQuizDeadlineReminder: failed to create in-app notification for user %s: %v", p.UserID, err)
		}

		if p.Email == "" {
			continue
		}
		if err := s.smtpClient.SendDeadlineReminder(p.Email, event.Title, event.Deadline, event.ReminderOffset, l); err != nil {
			log.Printf("HandleQuizDeadlineReminder: failed to send email to %s: %v", p.Email, err)
		}
	}

	return nil
}

func (s *NotificationService) HandleQuizResultsReady(ctx context.Context, data []byte) error {
	var event struct {
		InstanceID   string `json:"instance_id"`
		Title        string `json:"title"`
		Participants []struct {
			UserID   string `json:"user_id"`
			Email    string `json:"email"`
			Score    int    `json:"score"`
			MaxScore int    `json:"max_score"`
			Language string `json:"language"`
		} `json:"participants"`
	}

	if err := json.Unmarshal(data, &event); err != nil {
		return err
	}

	log.Printf("Processing quiz_results_ready event for instance %s", event.InstanceID)

	for _, p := range event.Participants {
		l := lang.Parse(p.Language)
		notification := &repository.Notification{
			UserID:  p.UserID,
			Type:    email.TmplQuizResults,
			Title:   email.InAppTitle(l, email.TmplQuizResults),
			Content: event.Title,
			IsRead:  false,
		}

		if err := s.repo.CreateNotification(ctx, notification); err != nil {
			log.Printf("Failed to create notification for user %s: %v", p.UserID, err)
		}

		if p.Email == "" {
			continue
		}
		if err := s.smtpClient.SendQuizResults(p.Email, event.Title, p.Score, p.MaxScore, l); err != nil {
			log.Printf("Failed to send quiz_results email to %s: %v", p.Email, err)
		}
	}

	return nil
}

func (s *NotificationService) HandleGradeChanged(ctx context.Context, data []byte) error {
	var event struct {
		InstanceID       string `json:"instance_id"`
		ParticipantID    string `json:"participant_id"`
		ParticipantEmail string `json:"participant_email"`
		Title            string `json:"title"`
		Score            int    `json:"score"`
		MaxScore         int    `json:"max_score"`
		Language         string `json:"language"`
	}

	if err := json.Unmarshal(data, &event); err != nil {
		return err
	}

	l := lang.Parse(event.Language)
	log.Printf("Processing quiz.grade_changed event for instance %s, participant %s, lang=%s", event.InstanceID, event.ParticipantID, l)

	notification := &repository.Notification{
		UserID:  event.ParticipantID,
		Type:    email.TmplGradeChanged,
		Title:   email.InAppTitle(l, email.TmplGradeChanged),
		Content: event.Title,
		IsRead:  false,
	}

	if err := s.repo.CreateNotification(ctx, notification); err != nil {
		log.Printf("Failed to create grade_changed notification for user %s: %v", event.ParticipantID, err)
	}

	if event.ParticipantEmail != "" {
		if err := s.smtpClient.SendGradeChanged(event.ParticipantEmail, event.Title, event.Score, event.MaxScore, l); err != nil {
			log.Printf("Failed to send grade_changed email to %s: %v", event.ParticipantEmail, err)
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
