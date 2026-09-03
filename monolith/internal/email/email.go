package email

import (
	"context"
	"fmt"
	"github.com/saurabhkr78/sudowallet/monolith/internal/logger"
	"net/smtp"
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
	return &smtpEmailSender{
		Host: host,
		Port: port,
		From: from,
	}
}

func (s *smtpEmailSender) SendEmail(ctx context.Context, to string, subject string, body string) error {
	//prepare the payload to sent to the smtp server
	//this structure is required by the smtp server to send the email
	// SMTP/email protocols mein lines traditionally CRLF (\r\n) se separate hoti hain.
	// Isliye hum yahan \r\n ka use kar rahe hain.
	// fmt.Sprintf ka use se hamein foramtted string milta hai
	/* ye produce karega
		To: user@gmail.com\r\n
	Subject: Verify your email\r\n
	\r\n
	Your OTP is 123456\r\n
		actual aisa dekhega:
		To: user@gmail.com
	Subject: Verify your email

	Your OTP is 123456

	smtp.SendMail function expect karta hai byte slice as message, isliye humne msg ko []byte() mein convert kiya hai.
	*/
	msg := []byte(fmt.Sprintf("To: %s\r\nSubject: %s\r\n\r\n%s\r\n", to, subject, body))

	//for local testing, we can use a local SMTP server like MailHog or Papercut and send the mail without authentication. In production, you would need to use a real SMTP server with authentication.
	addr := s.Host + ":" + s.Port
	err := smtp.SendMail(addr, nil, s.From, []string{to}, msg)
	if err != nil {

		logger.Log.Error("failed to send the mail ", "to", to, "error", err)
		return err
	}

	return nil
}
