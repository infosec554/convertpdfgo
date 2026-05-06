package mailer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type BrevoMailer struct {
	APIKey string
	Sender string
}

type BrevoRequest struct {
	Sender      SendInfo   `json:"sender"`
	To          []SendInfo `json:"to"`
	Subject     string     `json:"subject"`
	HtmlContent string     `json:"htmlContent"`
	TextContent string     `json:"textContent,omitempty"`
}

type SendInfo struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

type BrevoResponse struct {
	MessageID uint64 `json:"messageId,omitempty"`
	Error     string `json:"errors,omitempty"`
}

func NewBrevo(apiKey, sender string) *BrevoMailer {
	return &BrevoMailer{
		APIKey: apiKey,
		Sender: sender,
	}
}

func (b *BrevoMailer) Send(to string, subject string, body string) error {
	if b.APIKey == "" {
		return fmt.Errorf("brevo api key not configured")
	}

	req := BrevoRequest{
		Sender: SendInfo{
			Email: b.Sender,
			Name:  "convertpdfgo",
		},
		To: []SendInfo{
			{
				Email: to,
			},
		},
		Subject:     subject,
		HtmlContent: body,
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", "https://api.brevo.com/v3/smtp/email", bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("api-key", b.APIKey)

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}
	defer resp.Body.Close()

	body_resp, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("brevo error: status %d, body: %s", resp.StatusCode, string(body_resp))
	}

	var result BrevoResponse
	if err := json.Unmarshal(body_resp, &result); err != nil {
		// Some responses may not have JSON, that's OK if status is success
		if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
			return err
		}
	}

	return nil
}
