package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type TemplateRepository struct {
	db *sql.DB
}

func NewTemplateRepository(db *sql.DB) *TemplateRepository {
	return &TemplateRepository{db: db}
}

type Template struct {
	ID          string
	OwnerID     string
	Title       string
	Description string
	QuizType    string
	Settings    string // JSON
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Question struct {
	ID            string
	TemplateID    string
	Text          string
	Type          string
	Options       string // JSON array
	CorrectAnswer string // JSON
	OrderIndex    int
	MaxScore      int
	TimeLimitSec  int
	AIAnswer      sql.NullString // JSON
}

func (r *TemplateRepository) CreateTemplate(ctx context.Context, template *Template) error {
	template.ID = uuid.New().String()
	template.CreatedAt = time.Now()
	template.UpdatedAt = time.Now()

	query := `
		INSERT INTO quiz_templates (id, owner_id, title, description, quiz_type, settings, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := r.db.ExecContext(ctx, query,
		template.ID,
		template.OwnerID,
		template.Title,
		template.Description,
		template.QuizType,
		template.Settings,
		template.CreatedAt,
		template.UpdatedAt,
	)

	return err
}

func (r *TemplateRepository) GetTemplateByID(ctx context.Context, templateID string) (*Template, error) {
	query := `
		SELECT id, owner_id, title, description, quiz_type, settings, created_at, updated_at
		FROM quiz_templates
		WHERE id = $1
	`

	template := &Template{}
	err := r.db.QueryRowContext(ctx, query, templateID).Scan(
		&template.ID,
		&template.OwnerID,
		&template.Title,
		&template.Description,
		&template.QuizType,
		&template.Settings,
		&template.CreatedAt,
		&template.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("template not found")
	}
	if err != nil {
		return nil, err
	}

	return template, nil
}

func (r *TemplateRepository) GetTemplatesByOwner(ctx context.Context, ownerID string) ([]*Template, error) {
	query := `
		SELECT id, owner_id, title, description, quiz_type, settings, created_at, updated_at
		FROM quiz_templates
		WHERE owner_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []*Template
	for rows.Next() {
		template := &Template{}
		err := rows.Scan(
			&template.ID,
			&template.OwnerID,
			&template.Title,
			&template.Description,
			&template.QuizType,
			&template.Settings,
			&template.CreatedAt,
			&template.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		templates = append(templates, template)
	}

	return templates, rows.Err()
}

func (r *TemplateRepository) UpdateTemplate(ctx context.Context, template *Template) error {
	template.UpdatedAt = time.Now()

	query := `
		UPDATE quiz_templates
		SET title = $1, description = $2, settings = $3, updated_at = $4
		WHERE id = $5 AND owner_id = $6
	`

	result, err := r.db.ExecContext(ctx, query,
		template.Title,
		template.Description,
		template.Settings,
		template.UpdatedAt,
		template.ID,
		template.OwnerID,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return fmt.Errorf("template not found or unauthorized")
	}

	return nil
}

func (r *TemplateRepository) DeleteTemplate(ctx context.Context, templateID, ownerID string) error {
	query := `DELETE FROM quiz_templates WHERE id = $1 AND owner_id = $2`

	result, err := r.db.ExecContext(ctx, query, templateID, ownerID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return fmt.Errorf("template not found or unauthorized")
	}

	return nil
}

func (r *TemplateRepository) CreateQuestion(ctx context.Context, question *Question) error {
	question.ID = uuid.New().String()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	queryQuestion := `
		INSERT INTO questions (id, text, type, options, correct_answer, max_score, time_limit_sec)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err = tx.ExecContext(ctx, queryQuestion,
		question.ID,
		question.Text,
		question.Type,
		question.Options,
		question.CorrectAnswer,
		question.MaxScore,
		question.TimeLimitSec,
	)
	if err != nil {
		return err
	}

	queryLink := `
		INSERT INTO template_questions (template_id, question_id, order_index)
		VALUES ($1, $2, $3)
	`

	_, err = tx.ExecContext(ctx, queryLink,
		question.TemplateID,
		question.ID,
		question.OrderIndex,
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (r *TemplateRepository) LinkQuestionToTemplate(ctx context.Context, templateID, questionID string, orderIndex int) error {
	query := `
		INSERT INTO template_questions (template_id, question_id, order_index)
		VALUES ($1, $2, $3)
	`
	_, err := r.db.ExecContext(ctx, query, templateID, questionID, orderIndex)
	return err
}

func (r *TemplateRepository) GetQuestionsByTemplateID(ctx context.Context, templateID string) ([]*Question, error) {
	query := `
		SELECT q.id, tq.template_id, q.text, q.type, q.options, q.correct_answer, tq.order_index, q.max_score, q.time_limit_sec, q.ai_answer
		FROM questions q
		JOIN template_questions tq ON q.id = tq.question_id
		WHERE tq.template_id = $1
		ORDER BY tq.order_index ASC
	`

	rows, err := r.db.QueryContext(ctx, query, templateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var questions []*Question
	for rows.Next() {
		question := &Question{}
		err := rows.Scan(
			&question.ID,
			&question.TemplateID,
			&question.Text,
			&question.Type,
			&question.Options,
			&question.CorrectAnswer,
			&question.OrderIndex,
			&question.MaxScore,
			&question.TimeLimitSec,
			&question.AIAnswer,
		)
		if err != nil {
			return nil, err
		}
		questions = append(questions, question)
	}

	return questions, rows.Err()
}

func (r *TemplateRepository) DeleteQuestionsByTemplateID(ctx context.Context, templateID string) error {
	// TODO: Maybe delete questions that are not used by any other template/instance.
	query := `DELETE FROM template_questions WHERE template_id = $1`
	_, err := r.db.ExecContext(ctx, query, templateID)
	return err
}

func CorrectAnswerToJSON(answer interface{}) (string, error) {
	data, err := json.Marshal(answer)
	if err != nil {
		return "", err
	}
	return string(data), nil
}