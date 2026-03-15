package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"math/big"
	"quiz-service/internal/constants"
	"time"

	"github.com/google/uuid"
)

type InstanceRepository struct {
	db *sql.DB
}

func NewInstanceRepository(db *sql.DB) *InstanceRepository {
	return &InstanceRepository{db: db}
}

type Instance struct {
	ID             string
	TemplateID     sql.NullString
	Title          string
	AccessCode     string
	Status         string
	GroupID        sql.NullString
	CreatedBy      string
	CreatedAt      time.Time
	StartTime      sql.NullTime
	Deadline       sql.NullTime
	QuizType       string
	Settings       string // JSON
	TotalTime      uint64
	TotalQuestions uint64
}

type InstanceWithQuestions struct {
	Instance  *Instance
	Questions []*Question
}

type ParticipatingInstance struct {
	Instance      *Instance
	SessionStatus string
}

func (r *InstanceRepository) CreateInstance(ctx context.Context, instance *Instance) error {
	instance.ID = uuid.New().String()
	instance.CreatedAt = time.Now()

	if instance.QuizType == constants.QuizTypeAsync {
		instance.Status = constants.InstanceStatusActive
	} else {
		instance.Status = constants.InstanceStatusWaiting
	}

	var err error
	instance.AccessCode, err = r.generateUniqueAccessCode(ctx)
	if err != nil {
		return fmt.Errorf("failed to generate access code: %w", err)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `
		INSERT INTO quiz_instances (id, template_id, title, access_code, status, group_id, created_by, created_at, start_time, deadline, quiz_type, settings, total_time, total_questions)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`

	_, err = tx.ExecContext(ctx, query,
		instance.ID,
		instance.TemplateID,
		instance.Title,
		instance.AccessCode,
		instance.Status,
		instance.GroupID,
		instance.CreatedBy,
		instance.CreatedAt,
		instance.StartTime,
		instance.Deadline,
		instance.QuizType,
		instance.Settings,
		instance.TotalTime,
		instance.TotalQuestions,
	)
	if err != nil {
		return err
	}

	if instance.TemplateID.Valid {
		queryCopyQuestions := `
			INSERT INTO instance_questions (instance_id, question_id, order_index)
			SELECT $1, question_id, order_index
			FROM template_questions
			WHERE template_id = $2
		`
		_, err = tx.ExecContext(ctx, queryCopyQuestions, instance.ID, instance.TemplateID.String)
		if err != nil {
			return fmt.Errorf("failed to copy questions: %w", err)
		}
	}

	return tx.Commit()
}

func (r *InstanceRepository) GetInstanceByID(ctx context.Context, instanceID string) (*Instance, error) {
	query := `
		SELECT id, template_id, title, access_code, status, group_id, created_by, created_at, start_time, deadline, quiz_type, settings, total_time, total_questions
		FROM quiz_instances
		WHERE id = $1
	`

	instance := &Instance{}
	err := r.db.QueryRowContext(ctx, query, instanceID).Scan(
		&instance.ID,
		&instance.TemplateID,
		&instance.Title,
		&instance.AccessCode,
		&instance.Status,
		&instance.GroupID,
		&instance.CreatedBy,
		&instance.CreatedAt,
		&instance.StartTime,
		&instance.Deadline,
		&instance.QuizType,
		&instance.Settings,
		&instance.TotalTime,
		&instance.TotalQuestions,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("instance not found")
	}
	if err != nil {
		return nil, err
	}

	return instance, nil
}

func (r *InstanceRepository) GetHostingInstances(ctx context.Context, userID, status string) ([]*Instance, error) {
	query := `
		SELECT id, template_id, title, access_code, status, group_id, created_by, created_at, start_time, deadline, quiz_type, settings, total_time, total_questions
		FROM quiz_instances
		WHERE created_by = $1
	`

	args := []any{userID}

	if status != "" {
		query += " AND status = $2"
		args = append(args, status)
	}

	query += " ORDER BY created_at DESC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var instances []*Instance
	for rows.Next() {
		instance := &Instance{}
		err := rows.Scan(
			&instance.ID,
			&instance.TemplateID,
			&instance.Title,
			&instance.AccessCode,
			&instance.Status,
			&instance.GroupID,
			&instance.CreatedBy,
			&instance.CreatedAt,
			&instance.StartTime,
			&instance.Deadline,
			&instance.QuizType,
			&instance.Settings,
			&instance.TotalTime,
			&instance.TotalQuestions,
		)
		if err != nil {
			return nil, err
		}
		instances = append(instances, instance)
	}

	return instances, rows.Err()
}

func (r *InstanceRepository) GetInstanceWithQuestions(ctx context.Context, instanceID string) (*InstanceWithQuestions, error) {
	query := `
		SELECT
			i.id, i.template_id, i.title, i.access_code, i.status, i.group_id, i.created_by, i.created_at, i.start_time, i.deadline, i.quiz_type, i.settings, i.total_time, i.total_questions,
			q.id, q.text, q.type, q.correct_answer, iq.order_index, q.max_score, q.time_limit_sec
		FROM quiz_instances i
		LEFT JOIN instance_questions iq ON i.id = iq.instance_id
		LEFT JOIN questions q ON iq.question_id = q.id
		WHERE i.id = $1
		ORDER BY iq.order_index ASC
	`

	rows, err := r.db.QueryContext(ctx, query, instanceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result *InstanceWithQuestions
	var questions []*Question

	for rows.Next() {
		if result == nil {
			result = &InstanceWithQuestions{
				Instance: &Instance{},
			}
		}

		var qID sql.NullString
		var qText, qType, qCorrectAnswer sql.NullString
		var qOrderIndex, qMaxScore, qTimeLimitSec sql.NullInt32

		err := rows.Scan(
			&result.Instance.ID,
			&result.Instance.TemplateID,
			&result.Instance.Title,
			&result.Instance.AccessCode,
			&result.Instance.Status,
			&result.Instance.GroupID,
			&result.Instance.CreatedBy,
			&result.Instance.CreatedAt,
			&result.Instance.StartTime,
			&result.Instance.Deadline,
			&result.Instance.QuizType,
			&result.Instance.Settings,
			&result.Instance.TotalTime,
			&result.Instance.TotalQuestions,
			&qID,
			&qText,
			&qType,
			&qCorrectAnswer,
			&qOrderIndex,
			&qMaxScore,
			&qTimeLimitSec,
		)
		if err != nil {
			return nil, err
		}

		if qID.Valid {
			question := &Question{
				ID:            qID.String,
				TemplateID:    result.Instance.ID,
				Text:          qText.String,
				Type:          qType.String,
				CorrectAnswer: qCorrectAnswer.String,
				OrderIndex:    int(qOrderIndex.Int32),
				MaxScore:      int(qMaxScore.Int32),
				TimeLimitSec:  int(qTimeLimitSec.Int32),
			}
			questions = append(questions, question)
		}
	}

	if result == nil {
		return nil, fmt.Errorf("instance not found")
	}

	result.Questions = questions
	return result, nil
}

func (r *InstanceRepository) generateUniqueAccessCode(ctx context.Context) (string, error) {
	maxAttempts := 10
	for range maxAttempts {
		n, err := rand.Int(rand.Reader, big.NewInt(900000))
		if err != nil {
			return "", err
		}
		code := fmt.Sprintf("%06d", n.Int64()+100000)

		var exists bool
		query := `SELECT EXISTS(SELECT 1 FROM quiz_instances WHERE access_code = $1)`
		err = r.db.QueryRowContext(ctx, query, code).Scan(&exists)
		if err != nil {
			return "", err
		}

		if !exists {
			return code, nil
		}
	}

	return "", fmt.Errorf("failed to generate unique access code after %d attempts", maxAttempts)
}

func (r *InstanceRepository) GetParticipatingInstances(ctx context.Context, userID, sessionStatus string) ([]*ParticipatingInstance, error) {
	query := `
		SELECT * FROM (
			SELECT
				i.id, i.template_id, i.title, i.access_code, i.status, i.group_id, i.created_by, i.created_at, i.start_time, i.deadline, i.quiz_type, i.settings, i.total_time, i.total_questions,
				COALESCE(gs.status, 'not_started') as session_status
			FROM quiz_instances i
			LEFT JOIN game_sessions gs ON i.id = gs.instance_id AND gs.user_id = $1
			WHERE i.created_by != $1
			  AND (
			    gs.user_id IS NOT NULL
			    OR EXISTS (
			      SELECT 1 FROM group_members gm
			      WHERE gm.group_id = i.group_id AND gm.user_id = $1
			    )
			  )
		) AS participating
	`

	args := []any{userID}

	if sessionStatus != "" {
		query += " WHERE session_status = $2"
		args = append(args, sessionStatus)
	} else {
		query += " WHERE session_status != 'finished'"
	}

	query += " ORDER BY created_at DESC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*ParticipatingInstance
	for rows.Next() {
		instance := &Instance{}
		result := &ParticipatingInstance{Instance: instance}

		err := rows.Scan(
			&instance.ID,
			&instance.TemplateID,
			&instance.Title,
			&instance.AccessCode,
			&instance.Status,
			&instance.GroupID,
			&instance.CreatedBy,
			&instance.CreatedAt,
			&instance.StartTime,
			&instance.Deadline,
			&instance.QuizType,
			&instance.Settings,
			&instance.TotalTime,
			&instance.TotalQuestions,
			&result.SessionStatus,
		)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}

	return results, rows.Err()
}

func (r *InstanceRepository) GetInstanceByAccessCode(ctx context.Context, accessCode string) (*Instance, error) {
	query := `
		SELECT id, template_id, title, access_code, status, group_id, created_by, created_at, start_time, deadline, quiz_type, settings, total_time, total_questions
		FROM quiz_instances
		WHERE access_code = $1
	`

	instance := &Instance{}
	err := r.db.QueryRowContext(ctx, query, accessCode).Scan(
		&instance.ID,
		&instance.TemplateID,
		&instance.Title,
		&instance.AccessCode,
		&instance.Status,
		&instance.GroupID,
		&instance.CreatedBy,
		&instance.CreatedAt,
		&instance.StartTime,
		&instance.Deadline,
		&instance.QuizType,
		&instance.Settings,
		&instance.TotalTime,
		&instance.TotalQuestions,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("instance not found")
	}
	if err != nil {
		return nil, err
	}

	return instance, nil
}
