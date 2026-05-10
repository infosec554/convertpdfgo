package models

type SendOtpRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type SendOtpResponse struct {
	OtpID string `json:"otp_id"`
}

type ConfirmOtpRequest struct {
	OtpID string `json:"otp_id"`
	Code  string `json:"code"`
}

type ConfirmOtpResponse struct {
	Token string `json:"token"` // otp_confirmation_token
}
