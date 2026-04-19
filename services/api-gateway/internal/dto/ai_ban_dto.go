package dto

type CreateAIBanRequest struct {
	UserID string `json:"user_id" binding:"required" example:"user-123"`
	Reason string `json:"reason" example:"Abuse of AI features"`
}

type AIBanDTO struct {
	UserID    string `json:"user_id" example:"user-123"`
	Reason    string `json:"reason" example:"Abuse of AI features"`
	BannedBy  string `json:"banned_by" example:"admin-456"`
	CreatedAt int64  `json:"created_at" example:"1700000000"`
}

type CreateAIBanResponse struct {
	Ban AIBanDTO `json:"ban"`
}

type GetAIBanResponse struct {
	Ban AIBanDTO `json:"ban"`
}

type ListAIBansResponse struct {
	Bans []AIBanDTO `json:"bans"`
}
