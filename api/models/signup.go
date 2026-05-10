package models

type SignupRequest struct {
	OtpConfirmationToken string `json:"otp_confirmation_token"`
	Email                string `json:"email" binding:"required,email"`
	Password             string `json:"password"`
	Name                 string `json:"name"`
}
type SignupResponse struct {
	ID    string `json:"id"`        
	Token string `json:"token"`      
}
type OTPConfirmationClaims struct {
	OtpID string `json:"otp_id"`
	Email string `json:"email"`
}
