package email

import (
	"fmt"
	"log"
	"os"
	"strings"

	resend "github.com/resend/resend-go/v2"
)

type Sender struct {
	client *resend.Client
	from   string
}

func NewSender(apiKey, fromEmail string) *Sender {
	s := &Sender{from: fromEmail}
	if apiKey != "" {
		s.client = resend.NewClient(apiKey)
	}
	return s
}

func (s *Sender) IsConfigured() bool {
	return s.client != nil
}

func (s *Sender) SendWaitlistConfirmation(toEmail string) error {
	if s.client == nil {
		log.Printf("Skipping waitlist confirmation email to %s: Resend not configured", toEmail)
		return nil
	}

	html := `<div style="font-family:-apple-system,system-ui,sans-serif;max-width:480px;margin:0 auto;padding:32px 24px">
<h2 style="font-size:20px;margin:0 0 16px">You're on the waitlist</h2>
<p style="color:#444;line-height:1.6;margin:0 0 12px">Thanks for signing up for Haraesume. We'll review your request and let you know as soon as your account is ready.</p>
<p style="color:#444;line-height:1.6;margin:0">In the meantime, have your LaTeX resume handy — you'll be able to start tailoring it the moment you're in.</p>
<hr style="border:none;border-top:1px solid #eee;margin:28px 0 16px">
<p style="color:#999;font-size:13px;margin:0">— Haraesume</p>
</div>`

	_, err := s.client.Emails.Send(&resend.SendEmailRequest{
		From:    s.from,
		To:      []string{toEmail},
		Subject: "You're on the waitlist — Haraesume",
		Html:    html,
	})
	if err != nil {
		return fmt.Errorf("failed to send waitlist confirmation to %s: %w", toEmail, err)
	}
	log.Printf("Sent waitlist confirmation email to %s", toEmail)
	return nil
}

func (s *Sender) SendApprovalEmail(toEmail string) error {
	if s.client == nil {
		log.Printf("Skipping approval email to %s: Resend not configured", toEmail)
		return nil
	}

	appURL := strings.TrimSpace(os.Getenv("APP_URL"))
	if appURL == "" {
		appURL = "https://haraesume.com"
	}

	html := fmt.Sprintf(`<div style="font-family:-apple-system,system-ui,sans-serif;max-width:480px;margin:0 auto;padding:32px 24px">
<h2 style="font-size:20px;margin:0 0 16px">You've been approved!</h2>
<p style="color:#444;line-height:1.6;margin:0 0 20px">Your Haraesume account is now active. Sign in to start tailoring your resume for every role you apply to.</p>
<a href="%s" style="display:inline-block;padding:10px 28px;background:#111;color:#fff;text-decoration:none;border-radius:4px;font-size:14px;font-weight:500">Sign in</a>
<hr style="border:none;border-top:1px solid #eee;margin:28px 0 16px">
<p style="color:#999;font-size:13px;margin:0">— Haraesume</p>
</div>`, appURL)

	_, err := s.client.Emails.Send(&resend.SendEmailRequest{
		From:    s.from,
		To:      []string{toEmail},
		Subject: "You're in — Haraesume",
		Html:    html,
	})
	if err != nil {
		return fmt.Errorf("failed to send approval email to %s: %w", toEmail, err)
	}
	log.Printf("Sent approval email to %s", toEmail)
	return nil
}
