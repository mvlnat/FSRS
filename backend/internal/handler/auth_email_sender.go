package handler

import (
	"bytes"
	"context"
	"html/template"
	"log"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strconv"
	"strings"
)

type SMTPAuthEmailSender struct {
	fromHeader   string
	envelopeFrom string
	addr         string
	auth         smtp.Auth
}

type actionEmailContent struct {
	subject  string
	textBody string
	htmlBody string
}

type actionEmailTemplateData struct {
	Preview       string
	Eyebrow       string
	Title         string
	Intro         string
	Details       string
	ActionLabel   string
	ActionURL     string
	FallbackLabel string
	SafetyNote    string
}

var actionEmailHTMLTemplate = template.Must(template.New("action-email").Parse(`<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>{{.Title}}</title>
  </head>
  <body style="margin:0;padding:0;background:#f5f5f7;color:#1d1d1f;font-family:-apple-system,BlinkMacSystemFont,'SF Pro Display','SF Pro Text','Helvetica Neue',Helvetica,Arial,sans-serif;">
    <div style="display:none;max-height:0;overflow:hidden;opacity:0;color:transparent;">
      {{.Preview}}
    </div>
    <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%" style="background:#f5f5f7;">
      <tr>
        <td align="center" style="padding:32px 16px;">
          <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%" style="max-width:600px;">
            <tr>
              <td style="padding-bottom:16px;text-align:center;">
                <span style="display:inline-block;padding:8px 14px;border-radius:999px;background:rgba(0,113,227,0.08);color:#0071e3;font-size:12px;font-weight:600;letter-spacing:0.08em;text-transform:uppercase;">
                  FSRS
                </span>
              </td>
            </tr>
            <tr>
              <td style="background:#ffffff;border:1px solid rgba(0,0,0,0.08);border-radius:24px;padding:40px 36px;box-shadow:0 8px 24px rgba(0,0,0,0.08);">
                <p style="margin:0 0 12px;font-size:12px;line-height:1.4;font-weight:600;letter-spacing:0.08em;text-transform:uppercase;color:#6e6e73;">
                  {{.Eyebrow}}
                </p>
                <h1 style="margin:0 0 18px;font-size:30px;line-height:1.15;font-weight:600;letter-spacing:-0.024em;color:#1d1d1f;">
                  {{.Title}}
                </h1>
                <p style="margin:0 0 12px;font-size:17px;line-height:1.55;color:#1d1d1f;">
                  {{.Intro}}
                </p>
                <p style="margin:0 0 28px;font-size:17px;line-height:1.55;color:#6e6e73;">
                  {{.Details}}
                </p>
                <table role="presentation" cellpadding="0" cellspacing="0" border="0" style="margin-bottom:24px;">
                  <tr>
                    <td align="center" bgcolor="#0071e3" style="border-radius:999px;">
                      <a href="{{.ActionURL}}" style="display:inline-block;padding:14px 24px;font-size:16px;font-weight:600;line-height:1;color:#ffffff;text-decoration:none;">
                        {{.ActionLabel}}
                      </a>
                    </td>
                  </tr>
                </table>
                <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%" style="margin-bottom:20px;">
                  <tr>
                    <td style="background:#fbfbfd;border:1px solid rgba(0,0,0,0.08);border-radius:18px;padding:16px 18px;">
                      <p style="margin:0 0 10px;font-size:13px;line-height:1.5;font-weight:600;color:#6e6e73;">
                        {{.FallbackLabel}}
                      </p>
                      <p style="margin:0;font-size:13px;line-height:1.6;word-break:break-all;">
                        <a href="{{.ActionURL}}" style="color:#0071e3;text-decoration:none;">{{.ActionURL}}</a>
                      </p>
                    </td>
                  </tr>
                </table>
                <p style="margin:0;font-size:14px;line-height:1.6;color:#6e6e73;">
                  {{.SafetyNote}}
                </p>
              </td>
            </tr>
            <tr>
              <td style="padding:16px 12px 0;text-align:center;">
                <p style="margin:0;font-size:12px;line-height:1.6;color:#86868b;">
                  FSRS account email
                </p>
              </td>
            </tr>
          </table>
        </td>
      </tr>
    </table>
  </body>
</html>`))

var ErrEmailSenderNotConfigured = authValidationError("email sender is not configured")

func NewSMTPAuthEmailSender(host string, port int, username, password, from string) (*SMTPAuthEmailSender, error) {
	host = strings.TrimSpace(host)
	from = strings.TrimSpace(from)
	if host == "" || port <= 0 || from == "" {
		return nil, authValidationError("smtp host, port, and from address are required")
	}

	fromAddress, err := mail.ParseAddress(from)
	if err != nil {
		return nil, authValidationError("smtp from address is invalid")
	}

	var auth smtp.Auth
	if strings.TrimSpace(username) != "" {
		auth = smtp.PlainAuth("", username, password, host)
	}

	return &SMTPAuthEmailSender{
		fromHeader:   fromAddress.String(),
		envelopeFrom: fromAddress.Address,
		addr:         net.JoinHostPort(host, strconv.Itoa(port)),
		auth:         auth,
	}, nil
}

func (s *SMTPAuthEmailSender) SendVerificationEmail(ctx context.Context, email, verificationURL string) error {
	content, err := buildVerificationEmailContent(verificationURL)
	if err != nil {
		return err
	}

	return s.send(ctx, email, content)
}

func (s *SMTPAuthEmailSender) SendPasswordResetEmail(ctx context.Context, email, resetURL string) error {
	content, err := buildPasswordResetEmailContent(resetURL)
	if err != nil {
		return err
	}

	return s.send(ctx, email, content)
}

func (s *SMTPAuthEmailSender) send(_ context.Context, email string, content actionEmailContent) error {
	message, err := buildMultipartEmailMessage(email, s.fromHeader, content)
	if err != nil {
		return err
	}

	return smtp.SendMail(s.addr, s.auth, s.envelopeFrom, []string{email}, message)
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

func buildVerificationEmailContent(verificationURL string) (actionEmailContent, error) {
	return buildActionEmailContent(actionEmailTemplateData{
		Preview:       "Confirm your email address to finish setting up your FSRS account.",
		Eyebrow:       "Account Setup",
		Title:         "Verify your email address",
		Intro:         "Thanks for signing up for FSRS.",
		Details:       "Confirm this email address to activate your account and start studying with your decks.",
		ActionLabel:   "Verify Email",
		ActionURL:     verificationURL,
		FallbackLabel: "If the button does not work, copy and paste this link into your browser:",
		SafetyNote:    "If you did not create an FSRS account, you can safely ignore this email.",
	}, "Verify your email for FSRS")
}

func buildPasswordResetEmailContent(resetURL string) (actionEmailContent, error) {
	return buildActionEmailContent(actionEmailTemplateData{
		Preview:       "Use the secure link below to choose a new password for your FSRS account.",
		Eyebrow:       "Account Security",
		Title:         "Reset your password",
		Intro:         "We received a request to reset the password for your FSRS account.",
		Details:       "Use the secure link below to choose a new password and get back to studying.",
		ActionLabel:   "Reset Password",
		ActionURL:     resetURL,
		FallbackLabel: "If the button does not work, copy and paste this link into your browser:",
		SafetyNote:    "If you did not request a password reset, you can ignore this email and your password will stay unchanged.",
	}, "Reset your FSRS password")
}

func buildActionEmailContent(data actionEmailTemplateData, subject string) (actionEmailContent, error) {
	var htmlBody bytes.Buffer
	if err := actionEmailHTMLTemplate.Execute(&htmlBody, data); err != nil {
		return actionEmailContent{}, err
	}

	textBody := strings.Join([]string{
		data.Title,
		"",
		data.Intro,
		data.Details,
		"",
		data.ActionLabel + ":",
		data.ActionURL,
		"",
		data.SafetyNote,
		"",
		"FSRS",
	}, "\n")

	return actionEmailContent{
		subject:  subject,
		textBody: normalizeEmailLineEndings(textBody),
		htmlBody: normalizeEmailLineEndings(htmlBody.String()),
	}, nil
}

func buildMultipartEmailMessage(to, from string, content actionEmailContent) ([]byte, error) {
	var message bytes.Buffer
	writer := multipart.NewWriter(&message)
	boundary := writer.Boundary()

	message.WriteString("To: " + to + "\r\n")
	message.WriteString("From: " + from + "\r\n")
	message.WriteString("Subject: " + content.subject + "\r\n")
	message.WriteString("MIME-Version: 1.0\r\n")
	message.WriteString(`Content-Type: multipart/alternative; boundary="` + boundary + `"` + "\r\n")
	message.WriteString("\r\n")

	if err := writeQuotedPrintablePart(writer, "text/plain", content.textBody); err != nil {
		return nil, err
	}
	if err := writeQuotedPrintablePart(writer, "text/html", content.htmlBody); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	return message.Bytes(), nil
}

func writeQuotedPrintablePart(writer *multipart.Writer, contentType, body string) error {
	header := textproto.MIMEHeader{}
	header.Set("Content-Type", contentType+"; charset=UTF-8")
	header.Set("Content-Transfer-Encoding", "quoted-printable")

	part, err := writer.CreatePart(header)
	if err != nil {
		return err
	}

	encoder := quotedprintable.NewWriter(part)
	if _, err := encoder.Write([]byte(body)); err != nil {
		_ = encoder.Close()
		return err
	}

	return encoder.Close()
}

func normalizeEmailLineEndings(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	return strings.ReplaceAll(value, "\n", "\r\n")
}
