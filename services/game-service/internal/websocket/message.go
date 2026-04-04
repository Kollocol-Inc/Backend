package websocket

type MessageType string

const (
	// Client -> Server
	MessageTypeJoin     MessageType = "join"
	MessageTypeStart    MessageType = "start_quiz"
	MessageTypeAnswer   MessageType = "answer"
	MessageTypeContinue MessageType = "continue"
	MessageTypePing     MessageType = "ping"
	MessageTypeKick     MessageType = "kick"

	// Server -> Client
	MessageTypeConnected          MessageType = "connected"
	MessageTypeParticipantsUpdate MessageType = "participants_update"
	MessageTypeParticipantsList   MessageType = "participants_list"
	MessageTypeQuizStarted        MessageType = "quiz_started"
	MessageTypeQuestion           MessageType = "question"
	MessageTypeAnswerResult       MessageType = "answer_result"
	MessageTypeLeaderboard        MessageType = "leaderboard"
	MessageTypeTimeExpired        MessageType = "time_expired"
	MessageTypeWaitingForCreator  MessageType = "waiting_for_creator"
	MessageTypeQuizFinished       MessageType = "quiz_finished"
	MessageTypeAnswerProgress     MessageType = "answer_progress"
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
	User   User   `json:"user"`
	Count  int    `json:"count"`
}

type User struct {
	UserID    string `json:"user_id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url,omitempty"`
	IsCreator bool   `json:"is_creator"`
	IsOnline  bool   `json:"is_online"`
}

type Quiz struct {
	Title string `json:"title"`
}

type ParticipantsListPayload struct {
	Participants []User `json:"participants"`
	Quiz         Quiz   `json:"quiz"`
}

type KickPayload struct {
	Email string `json:"email"`
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

type AnswerOptionStats struct {
	Option string `json:"option"`
	Count  int    `json:"count"`
}

type LeaderboardPayload struct {
	Leaderboard       []LeaderboardEntry  `json:"leaderboard"`
	AnswerOptionStats []AnswerOptionStats `json:"questions_stats,omitempty"`
	CanContinue       bool                `json:"can_continue,omitempty"`
	Question          *QuestionPayload    `json:"question,omitempty"`
}

type LeaderboardEntry struct {
	User       User `json:"user"`
	Rank       int  `json:"rank"`
	Score      int  `json:"score"`
	IsAnswered bool `json:"is_answered"`
}

type TimeExpiredPayload struct {
	QuestionIndex int `json:"question_index"`
}

type WaitingForCreatorPayload struct {
	QuestionIndex int    `json:"question_index"`
	Reason        string `json:"reason,omitempty"`
}

type AnswerProgressPayload struct {
	ParticipantsAnswered int `json:"participants_answered"`
	TotalParticipants    int `json:"total_participants"`
}

type QuizFinishedPayload struct {
	FinalScore int `json:"final_score"`
	Rank       int `json:"rank"`
}

type ErrorPayload struct {
	Message string `json:"message"`
}
