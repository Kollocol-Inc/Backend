package dto


type RegisterRequest struct {
	FirstName string `json:"first_name" binding:"required,min=2,max=50" example:"John"`
	LastName  string `json:"last_name" binding:"required,min=2,max=50" example:"Doe"`
}

type UpdateProfileRequest struct {
	FirstName string `json:"first_name,omitempty" binding:"omitempty,min=2,max=50" example:"John"`
	LastName  string `json:"last_name,omitempty" binding:"omitempty,min=2,max=50" example:"Doe"`
}

type UpdateNotificationSettingsRequest struct {
	NewQuizzes       *bool   `json:"new_quizzes,omitempty" example:"true"`
	QuizResults      *bool   `json:"quiz_results,omitempty" example:"true"`
	GroupInvites     *bool   `json:"group_invites,omitempty" example:"true"`
	DeadlineReminder *string `json:"deadline_reminder,omitempty" example:"24h"`
}


type CreateGroupRequest struct {
	Name         string   `json:"name" binding:"required,min=3,max=100" example:"Study Group"`
	MemberEmails []string `json:"member_emails,omitempty" example:"user1@example.com,user2@example.com"`
}

type UpdateGroupRequest struct {
	Name         string   `json:"name,omitempty" binding:"omitempty,min=3,max=100" example:"Study Group"`
	MemberEmails []string `json:"member_emails,omitempty" example:"user3@example.com"`
}


type UserDTO struct {
	ID        string `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Email     string `json:"email" example:"user@example.com"`
	FirstName string `json:"first_name" example:"John"`
	LastName  string `json:"last_name" example:"Doe"`
	AvatarURL string `json:"avatar_url,omitempty" example:"https://storage.example.com/avatars/user123.jpg"`
	CreatedAt string `json:"created_at" example:"2024-01-15T10:30:00Z"`
	UpdatedAt string `json:"updated_at" example:"2024-01-15T10:30:00Z"`
}

type NotificationSettingsDTO struct {
	UserID           string `json:"user_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	NewQuizzes       bool   `json:"new_quizzes" example:"true"`
	QuizResults      bool   `json:"quiz_results" example:"true"`
	GroupInvites     bool   `json:"group_invites" example:"true"`
	DeadlineReminder string `json:"deadline_reminder" example:"24h"`
}


type GroupDTO struct {
	ID          string `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name        string `json:"name" example:"Study Group"`
	OwnerID     string `json:"owner_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	MemberCount int32  `json:"member_count" example:"5"`
	CreatedAt   string `json:"created_at" example:"2024-01-15T10:30:00Z"`
	UpdatedAt   string `json:"updated_at" example:"2024-01-15T10:30:00Z"`
}

type GroupMemberDTO struct {
	UserID    string `json:"user_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Email     string `json:"email" example:"user@example.com"`
	FirstName string `json:"first_name" example:"John"`
	LastName  string `json:"last_name" example:"Doe"`
	AvatarURL string `json:"avatar_url,omitempty" example:"https://storage.example.com/avatars/user123.jpg"`
	JoinedAt  string `json:"joined_at" example:"2024-01-15T10:30:00Z"`
}

type GroupWithMembersDTO struct {
	ID        string           `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name      string           `json:"name" example:"Study Group"`
	OwnerID   string           `json:"owner_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Members   []GroupMemberDTO `json:"members"`
	CreatedAt string           `json:"created_at" example:"2024-01-15T10:30:00Z"`
	UpdatedAt string           `json:"updated_at" example:"2024-01-15T10:30:00Z"`
}

type GetGroupsResponse struct {
	Groups []GroupDTO `json:"groups"`
}
