package email

import (
	"bytes"
	"fmt"
	"html/template"
	"net/smtp"

	"notification-service/config"
)

type SMTPClient struct {
	config *config.SMTPConfig
}

func NewSMTPClient(cfg *config.SMTPConfig) *SMTPClient {
	return &SMTPClient{
		config: cfg,
	}
}

type EmailData struct {
	To      string
	Subject string
	Body    string
}

func (c *SMTPClient) SendEmail(data EmailData) error {
	var auth smtp.Auth
	if c.config.Username != "" || c.config.Password != "" {
		auth = smtp.PlainAuth("", c.config.Username, c.config.Password, c.config.Host)
	}

	msg := c.buildMessage(data)

	addr := fmt.Sprintf("%s:%d", c.config.Host, c.config.Port)

	err := smtp.SendMail(addr, auth, c.config.From, []string{data.To}, []byte(msg))
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

func (c *SMTPClient) buildMessage(data EmailData) string {
	msg := fmt.Sprintf("From: %s\r\n", c.config.From)
	msg += fmt.Sprintf("To: %s\r\n", data.To)
	msg += fmt.Sprintf("Subject: %s\r\n", data.Subject)
	msg += "MIME-Version: 1.0\r\n"
	msg += "Content-Type: text/html; charset=UTF-8\r\n"
	msg += "\r\n"
	msg += data.Body

	return msg
}

func (c *SMTPClient) SendAuthCode(email, code string) error {
	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .code { font-size: 32px; font-weight: bold; color: #007bff; letter-spacing: 5px; margin: 20px 0; }
        .footer { margin-top: 30px; font-size: 12px; color: #666; }
    </style>
</head>
<body>
    <div class="container">
        <h2>Kollocol - Verification Code</h2>
        <p>Your verification code is:</p>
        <div class="code">{{.Code}}</div>
        <p>This code will expire in 5 minutes.</p>
        <div class="footer">
            <p>If you didn't request this code, please ignore this email.</p>
        </div>
    </div>
</body>
</html>
`

	t, err := template.New("auth_code").Parse(tmpl)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	var body bytes.Buffer
	if err := t.Execute(&body, map[string]string{"Code": code}); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return c.SendEmail(EmailData{
		To:      email,
		Subject: "Kollocol - Your Verification Code",
		Body:    body.String(),
	})
}

func (c *SMTPClient) SendGroupInvite(email, groupName, inviterName string) error {
	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .highlight { color: #007bff; font-weight: bold; }
        .footer { margin-top: 30px; font-size: 12px; color: #666; }
    </style>
</head>
<body>
    <div class="container">
        <h2>Kollocol - Group Invitation</h2>
        <p><span class="highlight">{{.InviterName}}</span> has invited you to join the group <span class="highlight">{{.GroupName}}</span>.</p>
        <p>Log in to Kollocol to accept the invitation and start collaborating!</p>
        <div class="footer">
            <p>This is an automated message from Kollocol.</p>
        </div>
    </div>
</body>
</html>
`

	t, err := template.New("group_invite").Parse(tmpl)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	var body bytes.Buffer
	data := map[string]string{
		"GroupName":   groupName,
		"InviterName": inviterName,
	}
	if err := t.Execute(&body, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return c.SendEmail(EmailData{
		To:      email,
		Subject: fmt.Sprintf("Kollocol - Invitation to join %s", groupName),
		Body:    body.String(),
	})
}

func (c *SMTPClient) SendQuizCreated(email, quizTitle, creatorName string) error {
	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .highlight { color: #007bff; font-weight: bold; }
        .footer { margin-top: 30px; font-size: 12px; color: #666; }
    </style>
</head>
<body>
    <div class="container">
        <h2>Kollocol - New Quiz Available</h2>
        <p><span class="highlight">{{.CreatorName}}</span> has created a new quiz: <span class="highlight">{{.QuizTitle}}</span></p>
        <p>Log in to Kollocol to participate!</p>
        <div class="footer">
            <p>This is an automated message from Kollocol.</p>
        </div>
    </div>
</body>
</html>
`

	t, err := template.New("quiz_created").Parse(tmpl)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	var body bytes.Buffer
	data := map[string]string{
		"QuizTitle":   quizTitle,
		"CreatorName": creatorName,
	}
	if err := t.Execute(&body, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return c.SendEmail(EmailData{
		To:      email,
		Subject: fmt.Sprintf("Kollocol - New Quiz: %s", quizTitle),
		Body:    body.String(),
	})
}

func (c *SMTPClient) SendQuizResults(email, quizTitle string) error {
	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .highlight { color: #007bff; font-weight: bold; }
        .footer { margin-top: 30px; font-size: 12px; color: #666; }
    </style>
</head>
<body>
    <div class="container">
        <h2>Kollocol - Quiz Results Ready</h2>
        <p>The results for <span class="highlight">{{.QuizTitle}}</span> are now available.</p>
        <p>Log in to Kollocol to view your results!</p>
        <div class="footer">
            <p>This is an automated message from Kollocol.</p>
        </div>
    </div>
</body>
</html>
`

	t, err := template.New("quiz_results").Parse(tmpl)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	var body bytes.Buffer
	data := map[string]string{
		"QuizTitle": quizTitle,
	}
	if err := t.Execute(&body, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return c.SendEmail(EmailData{
		To:      email,
		Subject: fmt.Sprintf("Kollocol - Results for %s", quizTitle),
		Body:    body.String(),
	})
}