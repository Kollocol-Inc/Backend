package model

type UserInfo struct {
	ID           string
	Email        string
	FirstName    string
	LastName     string
	IsRegistered bool
}

type NotificationSettings struct {
	NewQuizzes       bool
	QuizResults      bool
	GroupInvites     bool
	DeadlineReminder string
}
