package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/big"
	"quiz-service/internal/constants"
	"quiz-service/pkg/database"
	"time"

	"github.com/google/uuid"
)

type ParticipantSession struct {
	UserID     string
	Status     string
	Score      int
	Answers    string // JSON
	StartedAt  time.Time
	FinishedAt sql.NullTime
}

type SessionAnswer struct {
	QuestionID  string `json:"question_id"`
	Answer      string `json:"answer"`
	IsCorrect   bool   `json:"is_correct"`
	Score       int    `json:"score"`
	TimeSpentMs int64  `json:"time_spent_ms"`
	IsReviewed  bool   `json:"is_reviewed"`
}

type InstanceRepository struct {
	db database.DBTX
}

func NewInstanceRepository(db database.DBTX) *InstanceRepository {
	return &InstanceRepository{db: db}
}

func (r *InstanceRepository) q(ctx context.Context) database.DBTX {
	return database.Querier(ctx, r.db)
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

	exec := func(ctx context.Context, q database.DBTX) error {
		insert := `
			INSERT INTO quiz_instances (id, template_id, title, access_code, status, group_id, created_by, created_at, start_time, deadline, quiz_type, settings, total_time, total_questions)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		`
		if _, err := q.ExecContext(ctx, insert,
			instance.ID, instance.TemplateID, instance.Title, instance.AccessCode,
			instance.Status, instance.GroupID, instance.CreatedBy, instance.CreatedAt,
			instance.StartTime, instance.Deadline, instance.QuizType, instance.Settings,
			instance.TotalTime, instance.TotalQuestions,
		); err != nil {
			return err
		}
		if instance.TemplateID.Valid {
			if _, err := q.ExecContext(ctx, `
				INSERT INTO instance_questions (instance_id, question_id, order_index)
				SELECT $1, question_id, order_index
				FROM template_questions
				WHERE template_id = $2
			`, instance.ID, instance.TemplateID.String); err != nil {
				return fmt.Errorf("failed to copy questions: %w", err)
			}
		}
		return nil
	}

	if existing := database.TxFromContext(ctx); existing != nil {
		return exec(ctx, existing)
	}
	db, ok := r.db.(*sql.DB)
	if !ok {
		return fmt.Errorf("CreateInstance requires *sql.DB; got %T", r.db)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := exec(ctx, tx); err != nil {
		return err
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
	err := r.q(ctx).QueryRowContext(ctx, query, instanceID).Scan(
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

	rows, err := r.q(ctx).QueryContext(ctx, query, args...)
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

	rows, err := r.q(ctx).QueryContext(ctx, query, instanceID)
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
		err = r.q(ctx).QueryRowContext(ctx, query, code).Scan(&exists)
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

	rows, err := r.q(ctx).QueryContext(ctx, query, args...)
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
	err := r.q(ctx).QueryRowContext(ctx, query, accessCode).Scan(
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

func (r *InstanceRepository) GetInstanceParticipants(ctx context.Context, instanceID, excludeUserID string) ([]*ParticipantSession, error) {
	query := `
		SELECT gs.user_id, gs.status, gs.score, gs.answers, gs.started_at, gs.finished_at
		FROM game_sessions gs
		WHERE gs.instance_id = $1 AND gs.user_id != $2
		ORDER BY gs.started_at ASC
	`
	rows, err := r.q(ctx).QueryContext(ctx, query, instanceID, excludeUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*ParticipantSession
	for rows.Next() {
		s := &ParticipantSession{}
		if err := rows.Scan(&s.UserID, &s.Status, &s.Score, &s.Answers, &s.StartedAt, &s.FinishedAt); err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

func (r *InstanceRepository) GetParticipantAnswers(ctx context.Context, instanceID, userID string) (*ParticipantSession, error) {
	query := `
		SELECT gs.user_id, gs.status, gs.score, gs.answers, gs.started_at, gs.finished_at
		FROM game_sessions gs
		WHERE gs.instance_id = $1 AND gs.user_id = $2
	`
	s := &ParticipantSession{}
	err := r.q(ctx).QueryRowContext(ctx, query, instanceID, userID).Scan(
		&s.UserID, &s.Status, &s.Score, &s.Answers, &s.StartedAt, &s.FinishedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("participant not found")
	}
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (r *InstanceRepository) GradeAnswer(ctx context.Context, instanceID, userID, questionID string, score int) (int, error) {
	var oldScore int

	exec := func(ctx context.Context, q database.DBTX) error {
		var answersJSON string
		const lockQuery = `SELECT answers FROM game_sessions WHERE instance_id = $1 AND user_id = $2 FOR UPDATE`
		if err := q.QueryRowContext(ctx, lockQuery, instanceID, userID).Scan(&answersJSON); err != nil {
			return fmt.Errorf("participant not found: %w", err)
		}

		var answers []SessionAnswer
		if err := json.Unmarshal([]byte(answersJSON), &answers); err != nil {
			return fmt.Errorf("failed to parse answers: %w", err)
		}

		found := false
		for i, a := range answers {
			if a.QuestionID == questionID {
				oldScore = a.Score
				answers[i].Score = score
				answers[i].IsCorrect = score > 0
				answers[i].IsReviewed = true
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("answer for question not found")
		}

		updatedJSON, err := json.Marshal(answers)
		if err != nil {
			return fmt.Errorf("failed to marshal answers: %w", err)
		}

		const updateQuery = `UPDATE game_sessions SET answers = $1 WHERE instance_id = $2 AND user_id = $3`
		_, err = q.ExecContext(ctx, updateQuery, string(updatedJSON), instanceID, userID)
		return err
	}

	if existing := database.TxFromContext(ctx); existing != nil {
		return oldScore, exec(ctx, existing)
	}
	db, ok := r.db.(*sql.DB)
	if !ok {
		return 0, fmt.Errorf("GradeAnswer requires *sql.DB; got %T", r.db)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()
	if err := exec(ctx, tx); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit tx: %w", err)
	}
	return oldScore, nil
}

func (r *InstanceRepository) DeleteInstance(ctx context.Context, instanceID, createdBy string) error {
	query := `DELETE FROM quiz_instances WHERE id = $1 AND created_by = $2`
	result, err := r.q(ctx).ExecContext(ctx, query, instanceID, createdBy)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("instance not found or not owned by user")
	}
	return nil
}

func (r *InstanceRepository) UpdateInstanceStatus(ctx context.Context, instanceID, status string) error {
	query := `UPDATE quiz_instances SET status = $1 WHERE id = $2`
	result, err := r.q(ctx).ExecContext(ctx, query, status, instanceID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("instance not found")
	}
	return nil
}
