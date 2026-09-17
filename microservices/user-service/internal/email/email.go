package email

import (
	"context"
	"fmt"
	"net/smtp"

	"github.com/saurabhkr78/sudowallet/microservices/shared/logger"
)

type EmailSender interface {
	SendEmail(ctx context.Context, to string, subject string, body string) error
}

type smtpEmailSender struct {
	Host string
	Port string
	From string
}

func NewSMTPEmailSender(host, port, from string) EmailSender {
	return &smtpEmailSender{Host: host, Port: port, From: from}
}

// SendEmail sends an unauthenticated SMTP message (MailHog for local dev).
func (s *smtpEmailSender) SendEmail(ctx context.Context, to string, subject string, body string) error {
	msg := []byte(fmt.Sprintf("To: %s\r\nSubject: %s\r\n\r\n%s\r\n", to, subject, body))
	addr := s.Host + ":" + s.Port
	if err := smtp.SendMail(addr, nil, s.From, []string{to}, msg); err != nil {
		logger.Error(ctx, "failed to send email", "to", to, "error", err)
		return err
	}
	return nil
}
