package dto

type LoginRequest struct {
	Email string `json:"email" binding:"required,email" example:"user@example.com"`
}

type VerifyCodeRequest struct {
	Email string `json:"email" binding:"required,email" example:"user@example.com"`
	Code  string `json:"code" binding:"required,len=4" example:"1234"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
}

type LoginResponse struct {
	Success bool   `json:"success" example:"true"`
	Message string `json:"message" example:"Verification code sent to email"`
}

type VerifyCodeResponse struct {
	Success      bool   `json:"success" example:"true"`
	AccessToken  string `json:"access_token,omitempty" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	RefreshToken string `json:"refresh_token,omitempty" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	IsRegistered bool   `json:"is_registered" example:"true"`
	UserID       string `json:"user_id,omitempty" example:"6695dde6-8f6e-4973-905f-077ff7d3e2f8"`
	Message      string `json:"message,omitempty" example:"Login successful"`
}

type RefreshTokenResponse struct {
	Success      bool   `json:"success" example:"true"`
	AccessToken  string `json:"access_token,omitempty" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	RefreshToken string `json:"refresh_token,omitempty" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	Message      string `json:"message,omitempty" example:"Token refreshed successfully"`
}

type LogoutResponse struct {
	Success bool   `json:"success" example:"true"`
	Message string `json:"message,omitempty" example:"Logged out successfully"`
}
