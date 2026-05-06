package service

import "convertpdfgo/pkg/mailer"

type MailerService interface {
	Send(to, subject, body string) error
}

type mailerService struct {
	mailer mailer.IMailer
}

func NewMailerService(m mailer.IMailer) MailerService {
	return &mailerService{mailer: m}
}

func (m *mailerService) Send(to, subject, body string) error {
	return m.mailer.Send(to, subject, body)
}
