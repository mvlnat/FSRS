package handler

import (
	"net/mail"
	"strings"
	"testing"
)

func TestNewSMTPAuthEmailSenderParsesDisplayNameFromAddress(t *testing.T) {
	t.Parallel()

	sender, err := NewSMTPAuthEmailSender("smtp.example.com", 587, "mailer", "secret", "FSRS <noreply@example.com>")
	if err != nil {
		t.Fatalf("NewSMTPAuthEmailSender returned error: %v", err)
	}

	fromHeader, err := mail.ParseAddress(sender.fromHeader)
	if err != nil {
		t.Fatalf("from header is not a valid email address: %v", err)
	}
	if fromHeader.Name != "FSRS" || fromHeader.Address != "noreply@example.com" {
		t.Fatalf("unexpected parsed from header: %#v", fromHeader)
	}
	if sender.envelopeFrom != "noreply@example.com" {
		t.Fatalf("unexpected envelope from value: %q", sender.envelopeFrom)
	}
}

func TestNewSMTPAuthEmailSenderRejectsInvalidFromAddress(t *testing.T) {
	t.Parallel()

	_, err := NewSMTPAuthEmailSender("smtp.example.com", 587, "", "", "not-an-email")
	if err == nil {
		t.Fatal("expected invalid from address error")
	}
	if err.Error() != "smtp from address is invalid" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildVerificationEmailContent(t *testing.T) {
	t.Parallel()

	verificationURL := "https://fsrs.example.com/verify-email?token=abc123&next=study"
	content, err := buildVerificationEmailContent(verificationURL)
	if err != nil {
		t.Fatalf("buildVerificationEmailContent returned error: %v", err)
	}

	if content.subject != "Verify your email for FSRS" {
		t.Fatalf("unexpected subject: %q", content.subject)
	}
	if !strings.Contains(content.textBody, "Verify Email:\r\n"+verificationURL) {
		t.Fatalf("plain text body missing action URL:\n%s", content.textBody)
	}
	if !strings.Contains(content.htmlBody, "Verify your email address") {
		t.Fatalf("html body missing title:\n%s", content.htmlBody)
	}
	if !strings.Contains(content.htmlBody, "https://fsrs.example.com/verify-email?token=abc123&amp;next=study") {
		t.Fatalf("html body did not escape url query params:\n%s", content.htmlBody)
	}
}

func TestBuildPasswordResetEmailContent(t *testing.T) {
	t.Parallel()

	resetURL := "https://fsrs.example.com/reset-password?token=reset123"
	content, err := buildPasswordResetEmailContent(resetURL)
	if err != nil {
		t.Fatalf("buildPasswordResetEmailContent returned error: %v", err)
	}

	if content.subject != "Reset your FSRS password" {
		t.Fatalf("unexpected subject: %q", content.subject)
	}
	if !strings.Contains(content.textBody, "Reset Password:\r\n"+resetURL) {
		t.Fatalf("plain text body missing reset link:\n%s", content.textBody)
	}
	if !strings.Contains(content.htmlBody, "Account Security") {
		t.Fatalf("html body missing eyebrow:\n%s", content.htmlBody)
	}
}

func TestBuildMultipartEmailMessage(t *testing.T) {
	t.Parallel()

	message, err := buildMultipartEmailMessage("learner@example.com", "FSRS <noreply@example.com>", actionEmailContent{
		subject:  "Verify your email for FSRS",
		textBody: "Plain text body",
		htmlBody: "<strong>HTML body</strong>",
	})
	if err != nil {
		t.Fatalf("buildMultipartEmailMessage returned error: %v", err)
	}

	messageText := string(message)
	requiredSnippets := []string{
		"To: learner@example.com\r\n",
		"From: FSRS <noreply@example.com>\r\n",
		"Subject: Verify your email for FSRS\r\n",
		`Content-Type: multipart/alternative; boundary="`,
		"Content-Type: text/plain; charset=UTF-8\r\n",
		"Content-Type: text/html; charset=UTF-8\r\n",
		"Content-Transfer-Encoding: quoted-printable\r\n",
	}

	for _, snippet := range requiredSnippets {
		if !strings.Contains(messageText, snippet) {
			t.Fatalf("message missing snippet %q:\n%s", snippet, messageText)
		}
	}

	if strings.Contains(messageText, "\nPlain text body\n") || strings.Contains(messageText, "\n<strong>HTML body</strong>\n") {
		t.Fatalf("message body contains bare LF line endings:\n%s", messageText)
	}
}

func TestNormalizeEmailLineEndings(t *testing.T) {
	t.Parallel()

	normalized := normalizeEmailLineEndings("line one\nline two\r\nline three\rline four")
	if normalized != "line one\r\nline two\r\nline three\r\nline four" {
		t.Fatalf("unexpected normalized value: %q", normalized)
	}
}
