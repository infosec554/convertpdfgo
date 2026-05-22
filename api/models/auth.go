package models

// === Registration & email/password login ===

type RegisterRequest struct {
	Name     string `json:"name" binding:"required,min=2"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

type RegisterResponse struct {
	OtpID   string `json:"otp_id"`
	Message string `json:"message"`
}

// RegisterConfirmRequest finalises a sign-up after the user enters the OTP.
type RegisterConfirmRequest struct {
	OtpID string `json:"otp_id" binding:"required"`
	Code  string `json:"code" binding:"required,len=6"`
}

type AuthTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type EmailPasswordLoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// === Refresh / logout ===

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// === Password reset ===

type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type ResetPasswordRequest struct {
	OtpID       string `json:"otp_id" binding:"required"`
	Code        string `json:"code" binding:"required,len=6"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=6"`
}

// === Profile ===

type UpdateProfileRequest struct {
	Name string `json:"name" binding:"required,min=2"`
}
