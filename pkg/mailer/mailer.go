package mailer

// Mailer interface barcha email servislari uchun
type IMailer interface {
	Send(to, subject, body string) error
}

