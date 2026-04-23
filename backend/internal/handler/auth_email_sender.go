package handler

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/smtp"
	"strconv"
	"strings"
)

type SMTPAuthEmailSender struct {
	from string
	addr string
	auth smtp.Auth
}

var ErrEmailSenderNotConfigured = authValidationError("email sender is not configured")

func NewSMTPAuthEmailSender(host string, port int, username, password, from string) (*SMTPAuthEmailSender, error) {
	host = strings.TrimSpace(host)
	from = strings.TrimSpace(from)
	if host == "" || port <= 0 || from == "" {
		return nil, authValidationError("smtp host, port, and from address are required")
	}

	var auth smtp.Auth
	if strings.TrimSpace(username) != "" {
		auth = smtp.PlainAuth("", username, password, host)
	}

	return &SMTPAuthEmailSender{
		from: from,
		addr: net.JoinHostPort(host, strconv.Itoa(port)),
		auth: auth,
	}, nil
}

func (s *SMTPAuthEmailSender) SendVerificationEmail(ctx context.Context, email, verificationURL string) error {
	return s.send(ctx, email, "Verify your FSRS email", buildTextEmailBody(
		"Verify your FSRS email",
		"Open this link to verify your email address:",
		verificationURL,
	))
}

func (s *SMTPAuthEmailSender) SendPasswordResetEmail(ctx context.Context, email, resetURL string) error {
	return s.send(ctx, email, "Reset your FSRS password", buildTextEmailBody(
		"Reset your FSRS password",
		"Open this link to reset your password:",
		resetURL,
	))
}

func (s *SMTPAuthEmailSender) send(_ context.Context, email, subject, body string) error {
	message := strings.Join([]string{
		"To: " + email,
		"From: " + s.from,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		body,
	}, "\r\n")

	return smtp.SendMail(s.addr, s.auth, s.from, []string{email}, []byte(message))
}

func (s *SMTPAuthEmailSender) CheckConfig() error {
	return nil
}

type LogAuthEmailSender struct {
	logger interface {
		Printf(format string, v ...any)
	}
}

func NewLogAuthEmailSender(logger *log.Logger) *LogAuthEmailSender {
	if logger == nil {
		logger = log.Default()
	}

	return &LogAuthEmailSender{logger: logger}
}

func (s *LogAuthEmailSender) SendVerificationEmail(_ context.Context, email, verificationURL string) error {
	s.logger.Printf("verification email for %s: %s", email, verificationURL)
	return nil
}

func (s *LogAuthEmailSender) SendPasswordResetEmail(_ context.Context, email, resetURL string) error {
	s.logger.Printf("password reset email for %s: %s", email, resetURL)
	return nil
}

func (s *LogAuthEmailSender) CheckConfig() error {
	return nil
}

type UnavailableAuthEmailSender struct {
	err error
}

func NewUnavailableAuthEmailSender(err error) *UnavailableAuthEmailSender {
	if err == nil {
		err = ErrEmailSenderNotConfigured
	}

	return &UnavailableAuthEmailSender{err: err}
}

func (s *UnavailableAuthEmailSender) SendVerificationEmail(_ context.Context, _ string, _ string) error {
	return s.err
}

func (s *UnavailableAuthEmailSender) SendPasswordResetEmail(_ context.Context, _ string, _ string) error {
	return s.err
}

func (s *UnavailableAuthEmailSender) CheckConfig() error {
	return s.err
}

func buildTextEmailBody(subject, intro, actionURL string) string {
	return fmt.Sprintf("%s\n\n%s\n%s\n", subject, intro, actionURL)
}
