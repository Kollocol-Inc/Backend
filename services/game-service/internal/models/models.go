package models

import (
	"database/sql"
	"time"
)

type GameSession struct {
	InstanceID           string
	UserID               string
	Status               string // "joined", "in_progress", "finished"
	CurrentQuestionIndex int
	Score                int
	Answers              string // JSON
	StartedAt            time.Time
	FinishedAt           sql.NullTime
}

type Answer struct {
	QuestionID  string `json:"question_id"`
	Answer      string `json:"answer"`
	IsCorrect   bool   `json:"is_correct"`
	Score       int    `json:"score"`
	TimeSpentMs int64  `json:"time_spent_ms"`
	IsReviewed  bool   `json:"is_reviewed"`
}

type QuizData struct {
	QuizType   string     `json:"quiz_type"`
	CreatedBy  string     `json:"created_by"`
	Questions  []Question `json:"questions"`
	TemplateID string     `json:"template_id"`
	Title      string     `json:"title"`
	Settings   Settings   `json:"settings"`
	DeadlineMs int64      `json:"deadline_ms,omitempty"`
}

type Question struct {
	ID            string   `json:"id"`
	Text          string   `json:"text"`
	Type          string   `json:"type"`
	Options       []string `json:"options"`
	CorrectAnswer string   `json:"correct_answer"`
	OrderIndex    int      `json:"order_index"`
	MaxScore      int      `json:"max_score"`
	TimeLimitSec  int      `json:"time_limit_sec"`
}

type Settings struct {
	QuestionsRandomOrder bool `json:"questions_random_order"`
}

type LeaderboardEntry struct {
	Rank   int    `json:"rank"`
	UserID string `json:"user_id"`
	Score  int    `json:"score"`
}