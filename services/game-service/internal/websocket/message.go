package websocket

type MessageType string

const (
	// Client -> Server
	MessageTypeJoin     MessageType = "join"
	MessageTypeStart    MessageType = "start_quiz"
	MessageTypeAnswer   MessageType = "answer"
	MessageTypeContinue MessageType = "continue"
	MessageTypePing     MessageType = "ping"

	// Server -> Client
	MessageTypeConnected          MessageType = "connected"
	MessageTypeParticipantsUpdate MessageType = "participants_update"
	MessageTypeQuizStarted        MessageType = "quiz_started"
	MessageTypeQuestion           MessageType = "question"
	MessageTypeAnswerResult       MessageType = "answer_result"
	MessageTypeLeaderboard        MessageType = "leaderboard"
	MessageTypeTimeExpired        MessageType = "time_expired"
	MessageTypeWaitingForCreator  MessageType = "waiting_for_creator"
	MessageTypeQuizFinished       MessageType = "quiz_finished"
	MessageTypeError              MessageType = "error"
	MessageTypePong               MessageType = "pong"
)

type Message struct {
	Type    MessageType `json:"type"`
	Payload any         `json:"payload,omitempty"`
}

type JoinPayload struct {
	InstanceID string `json:"instance_id,omitempty"`
	AccessCode string `json:"access_code,omitempty"`
}

type AnswerPayload struct {
	QuestionID  string `json:"question_id"`
	Answer      string `json:"answer"`
	TimeSpentMs int64  `json:"time_spent_ms,omitempty"`
}

type ConnectedPayload struct {
	SessionID  string `json:"session_id"`
	QuizType   string `json:"quiz_type"`
	QuizStatus string `json:"quiz_status"`
	IsCreator  bool   `json:"is_creator"`
}

type ParticipantsUpdatePayload struct {
	Action string `json:"action"` // "joined" or "left"
	UserID string `json:"user_id"`
	Count  int    `json:"count"`
}

type QuizStartedPayload struct {
	QuizType string `json:"quiz_type"`
}

type QuestionPayload struct {
	Question       QuestionData `json:"question"`
	QuestionIndex  int          `json:"question_index"`
	TotalQuestions int          `json:"total_questions"`
	TimeLimitMs    int64        `json:"time_limit_ms,omitempty"`
	ServerTime     int64        `json:"server_time"`
}

type QuestionData struct {
	ID           string   `json:"id"`
	Text         string   `json:"text"`
	Type         string   `json:"type"`
	Options      []string `json:"options,omitempty"`
	OrderIndex   int      `json:"order_index"`
	MaxScore     int      `json:"max_score"`
	TimeLimitSec int      `json:"time_limit_sec"`
}

type AnswerResultPayload struct {
	IsCorrect   bool  `json:"is_correct"`
	Score       int   `json:"score"`
	TimeSpentMs int64 `json:"time_spent_ms"`
	TotalScore  int   `json:"total_score"`
}

type LeaderboardPayload struct {
	Leaderboard []LeaderboardEntry `json:"leaderboard"`
}

type LeaderboardEntry struct {
	Rank   int    `json:"rank"`
	UserID string `json:"user_id"`
	Score  int    `json:"score"`
}

type TimeExpiredPayload struct {
	QuestionIndex int `json:"question_index"`
}

type WaitingForCreatorPayload struct {
	QuestionIndex int    `json:"question_index"`
	Reason        string `json:"reason,omitempty"`
}

type QuizFinishedPayload struct {
	FinalScore int `json:"final_score"`
	Rank       int `json:"rank"`
}

type ErrorPayload struct {
	Message string `json:"message"`
}
