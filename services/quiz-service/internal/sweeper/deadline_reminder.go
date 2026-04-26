package sweeper

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"time"

	"quiz-service/internal/model"
)

type ReminderPublisher interface {
	Publish(ctx context.Context, queueName string, body []byte) error
}

type ReminderUserClient interface {
	GetGroupMemberIDs(ctx context.Context, groupID string) ([]string, error)
	GetUsersByIDs(ctx context.Context, userIDs []string) (map[string]*model.UserInfo, error)
	GetNotificationSettingsBatch(ctx context.Context, userIDs []string) (map[string]model.NotificationSettings, error)
}

type DeadlineReminderSweeper struct {
	db         *sql.DB
	publisher  ReminderPublisher
	userClient ReminderUserClient
	interval   time.Duration
}

func NewDeadlineReminderSweeper(
	db *sql.DB,
	publisher ReminderPublisher,
	userClient ReminderUserClient,
	interval time.Duration,
) *DeadlineReminderSweeper {
	return &DeadlineReminderSweeper{
		db:         db,
		publisher:  publisher,
		userClient: userClient,
		interval:   interval,
	}
}

func (s *DeadlineReminderSweeper) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.sweep(ctx)
		}
	}
}

type upcomingInstance struct {
	ID       string
	Title    string
	GroupID  sql.NullString
	Deadline time.Time
}

func (s *DeadlineReminderSweeper) sweep(ctx context.Context) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, title, group_id, deadline
		FROM quiz_instances
		WHERE quiz_type = 'async'
		  AND deadline IS NOT NULL
		  AND deadline > NOW()
		  AND status NOT IN ('pending_review', 'reviewed', 'published_results')
	`)
	if err != nil {
		log.Printf("deadline_reminder sweeper: failed to query instances: %v", err)
		return
	}
	defer rows.Close()

	var instances []upcomingInstance
	for rows.Next() {
		var inst upcomingInstance
		if err := rows.Scan(&inst.ID, &inst.Title, &inst.GroupID, &inst.Deadline); err != nil {
			log.Printf("deadline_reminder sweeper: scan error: %v", err)
			continue
		}
		instances = append(instances, inst)
	}
	if err := rows.Err(); err != nil {
		log.Printf("deadline_reminder sweeper: rows error: %v", err)
		return
	}

	now := time.Now()
	for _, inst := range instances {
		s.processInstance(ctx, inst, now)
	}
}

type dueOffset struct {
	label    string
	duration time.Duration
}

func (s *DeadlineReminderSweeper) processInstance(ctx context.Context, inst upcomingInstance, now time.Time) {
	if !inst.GroupID.Valid {
		return
	}

	remaining := inst.Deadline.Sub(now)

	window := s.interval

	var dueOffsets []dueOffset
	for _, candidate := range []dueOffset{
		{"1h", time.Hour},
		{"24h", 24 * time.Hour},
	} {
		diff := remaining - candidate.duration
		if diff < 0 {
			diff = -diff
		}
		if diff <= window {
			dueOffsets = append(dueOffsets, candidate)
		}
	}

	if len(dueOffsets) == 0 {
		return
	}

	memberIDs, err := s.userClient.GetGroupMemberIDs(ctx, inst.GroupID.String)
	if err != nil {
		log.Printf("deadline_reminder sweeper: GetGroupMemberIDs failed for instance %s: %v", inst.ID, err)
		return
	}
	if len(memberIDs) == 0 {
		return
	}

	usersMap, err := s.userClient.GetUsersByIDs(ctx, memberIDs)
	if err != nil {
		log.Printf("deadline_reminder sweeper: GetUsersByIDs failed for instance %s: %v", inst.ID, err)
		usersMap = map[string]*model.UserInfo{}
	}

	settingsBatch, err := s.userClient.GetNotificationSettingsBatch(ctx, memberIDs)
	if err != nil {
		log.Printf("deadline_reminder sweeper: GetNotificationSettingsBatch failed for instance %s: %v", inst.ID, err)
		settingsBatch = map[string]model.NotificationSettings{}
	}

	for _, offset := range dueOffsets {
		s.sendReminders(ctx, inst, offset.label, memberIDs, usersMap, settingsBatch, now)
	}
}

type reminderParticipant struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
}

type deadlineReminderEvent struct {
	InstanceID     string                `json:"instance_id"`
	Title          string                `json:"title"`
	Deadline       string                `json:"deadline"`
	ReminderOffset string                `json:"reminder_offset"`
	Participants   []reminderParticipant `json:"participants"`
}

func (s *DeadlineReminderSweeper) sendReminders(
	ctx context.Context,
	inst upcomingInstance,
	offsetLabel string,
	memberIDs []string,
	usersMap map[string]*model.UserInfo,
	settingsBatch map[string]model.NotificationSettings,
	now time.Time,
) {
	var toSend []reminderParticipant
	for _, uid := range memberIDs {
		settings, hasSettings := settingsBatch[uid]
		if hasSettings {
			if settings.DeadlineReminder == "never" {
				continue
			}
			if settings.DeadlineReminder != offsetLabel {
				continue
			}
		} else {
			if offsetLabel != "24h" {
				continue
			}
		}

		alreadySent, err := s.checkSent(ctx, inst.ID, uid, offsetLabel)
		if err != nil {
			log.Printf("deadline_reminder sweeper: checkSent error for user %s, instance %s: %v", uid, inst.ID, err)
			continue
		}
		if alreadySent {
			continue
		}

		email := ""
		if info, ok := usersMap[uid]; ok && info.IsRegistered {
			email = info.Email
		}
		toSend = append(toSend, reminderParticipant{UserID: uid, Email: email})
	}

	if len(toSend) == 0 {
		return
	}

	event := deadlineReminderEvent{
		InstanceID:     inst.ID,
		Title:          inst.Title,
		Deadline:       inst.Deadline.Format(time.RFC3339),
		ReminderOffset: offsetLabel,
		Participants:   toSend,
	}

	eventJSON, err := json.Marshal(event)
	if err != nil {
		log.Printf("deadline_reminder sweeper: marshal error: %v", err)
		return
	}

	if err := s.publisher.Publish(ctx, "quiz.deadline_reminder", eventJSON); err != nil {
		log.Printf("deadline_reminder sweeper: publish failed for instance %s offset %s: %v", inst.ID, offsetLabel, err)
		return
	}

	for _, p := range toSend {
		if err := s.recordSent(ctx, inst.ID, p.UserID, offsetLabel, now); err != nil {
			log.Printf("deadline_reminder sweeper: recordSent error for user %s, instance %s: %v", p.UserID, inst.ID, err)
		}
	}
}

func (s *DeadlineReminderSweeper) checkSent(ctx context.Context, instanceID, userID, offsetLabel string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM quiz_deadline_reminders_sent
			WHERE instance_id = $1 AND user_id = $2 AND reminder_offset = $3
		)
	`, instanceID, userID, offsetLabel).Scan(&exists)
	return exists, err
}

func (s *DeadlineReminderSweeper) recordSent(ctx context.Context, instanceID, userID, offsetLabel string, sentAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO quiz_deadline_reminders_sent (instance_id, user_id, reminder_offset, sent_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (instance_id, user_id, reminder_offset) DO NOTHING
	`, instanceID, userID, offsetLabel, sentAt)
	return err
}
