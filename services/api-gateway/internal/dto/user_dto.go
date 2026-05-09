package dto


type RegisterRequest struct {
	FirstName string `json:"first_name" binding:"required,min=2,max=50" example:"John"`
	LastName  string `json:"last_name" binding:"required,min=2,max=50" example:"Doe"`
}

type UpdateProfileRequest struct {
	FirstName string  `json:"first_name,omitempty" binding:"omitempty,min=2,max=50" example:"John"`
	LastName  string  `json:"last_name,omitempty" binding:"omitempty,min=2,max=50" example:"Doe"`
	Language  *string `json:"language,omitempty" binding:"omitempty,oneof=ru en" enums:"ru,en" example:"ru"`
}

type UpdateNotificationSettingsRequest struct {
	NewQuizzes   *bool `json:"new_quizzes,omitempty" example:"true"`
	QuizResults  *bool `json:"quiz_results,omitempty" example:"true"`
	GroupInvites *bool `json:"group_invites,omitempty" example:"true"`
	GroupKicked  *bool `json:"group_kicked,omitempty" example:"true"`
	DeadlineReminder *string `json:"deadline_reminder,omitempty" enums:"never,1h,3h,6h,12h,24h" example:"24h"`
}


type CreateGroupRequest struct {
	Name         string   `json:"name" binding:"required,min=3,max=100" example:"Study Group"`
	Description  string   `json:"description,omitempty" binding:"omitempty,max=500" example:"Group for studying together"`
	AvatarURL    string   `json:"avatar_url,omitempty" example:"https://storage.example.com/group-avatars/abc.jpg"`
	MemberEmails []string `json:"member_emails,omitempty" binding:"omitempty,max=100,dive,email" example:"user1@example.com,user2@example.com"`
}

type UpdateGroupRequest struct {
	Name        *string `json:"name,omitempty" binding:"omitempty,min=3,max=100" example:"Study Group"`
	Description *string `json:"description,omitempty" binding:"omitempty,max=500" example:"Group for studying together"`
	AvatarURL   *string `json:"avatar_url,omitempty" example:"https://storage.example.com/group-avatars/abc.jpg"`
}

type InviteGroupMembersRequest struct {
	Emails []string `json:"emails" binding:"required,min=1,max=100,dive,email" example:"user1@example.com,user2@example.com"`
}

type KickGroupMembersRequest struct {
	Emails []string `json:"emails" binding:"required,min=1,max=100,dive,email" example:"user3@example.com"`
}

type GroupAvatarUploadResponse struct {
	AvatarURL string `json:"avatar_url" example:"https://storage.example.com/group-avatars/abc.jpg"`
}


type UserDTO struct {
	ID        string `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Email     string `json:"email" example:"user@example.com"`
	FirstName string `json:"first_name" example:"John"`
	LastName  string `json:"last_name" example:"Doe"`
	AvatarURL string `json:"avatar_url,omitempty" example:"https://storage.example.com/avatars/user123.jpg"`
	Language  string `json:"language" enums:"ru,en" example:"ru"`
	CreatedAt string `json:"created_at" example:"2024-01-15T10:30:00Z"`
	UpdatedAt string `json:"updated_at" example:"2024-01-15T10:30:00Z"`
}

type NotificationSettingsDTO struct {
	UserID       string `json:"user_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	NewQuizzes   bool   `json:"new_quizzes" example:"true"`
	QuizResults  bool   `json:"quiz_results" example:"true"`
	GroupInvites bool   `json:"group_invites" example:"true"`
	GroupKicked  bool   `json:"group_kicked" example:"true"`
	DeadlineReminder string `json:"deadline_reminder" enums:"never,1h,3h,6h,12h,24h" example:"24h"`
}


type GroupDTO struct {
	ID           string `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name         string `json:"name" example:"Study Group"`
	Description  string `json:"description" example:"Group for studying together"`
	AvatarURL    string `json:"avatar_url,omitempty" example:"https://storage.example.com/group-avatars/abc.jpg"`
	OwnerID      string `json:"owner_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	MemberCount  int32  `json:"member_count" example:"5"`
	PendingCount int32  `json:"pending_count" example:"2"`
	CreatedAt    string `json:"created_at" example:"2024-01-15T10:30:00Z"`
	UpdatedAt    string `json:"updated_at" example:"2024-01-15T10:30:00Z"`
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
	ID           string           `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name         string           `json:"name" example:"Study Group"`
	Description  string           `json:"description" example:"Group for studying together"`
	AvatarURL    string           `json:"avatar_url,omitempty" example:"https://storage.example.com/group-avatars/abc.jpg"`
	OwnerID      string           `json:"owner_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	MemberCount  int32            `json:"member_count" example:"5"`
	PendingCount int32            `json:"pending_count" example:"2"`
	Members      []GroupMemberDTO `json:"members"`
	InvitedUsers []GroupMemberDTO `json:"invited_users"`
	CreatedAt    string           `json:"created_at" example:"2024-01-15T10:30:00Z"`
	UpdatedAt    string           `json:"updated_at" example:"2024-01-15T10:30:00Z"`
}

type GetGroupsResponse struct {
	Groups []GroupDTO `json:"groups"`
}
