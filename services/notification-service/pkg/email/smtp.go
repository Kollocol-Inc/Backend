package email

import (
	"bytes"
	"fmt"
	"net/smtp"

	"notification-service/config"
	"notification-service/pkg/lang"
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

func (c *SMTPClient) renderAndSend(to string, l lang.Lang, tmplName string, data any, subject string) error {
	tmpl := Template(l, tmplName)
	if tmpl == nil {
		return fmt.Errorf("template %q not found for language %q", tmplName, l)
	}
	var body bytes.Buffer
	if err := tmpl.Execute(&body, data); err != nil {
		return fmt.Errorf("failed to execute template %q: %w", tmplName, err)
	}
	return c.SendEmail(EmailData{
		To:      to,
		Subject: subject,
		Body:    body.String(),
	})
}

func (c *SMTPClient) SendAuthCode(emailAddr, code string, l lang.Lang) error {
	return c.renderAndSend(emailAddr, l, TmplAuthCode,
		map[string]string{"Code": code},
		Subject(l, TmplAuthCode))
}

func (c *SMTPClient) SendGroupInvite(emailAddr, groupName, inviterName string, l lang.Lang) error {
	return c.renderAndSend(emailAddr, l, TmplGroupInvite,
		map[string]string{"GroupName": groupName, "InviterName": inviterName},
		Subject(l, TmplGroupInvite, groupName))
}

func (c *SMTPClient) SendQuizCreated(emailAddr, quizTitle, creatorName string, l lang.Lang) error {
	return c.renderAndSend(emailAddr, l, TmplQuizCreated,
		map[string]string{"QuizTitle": quizTitle, "CreatorName": creatorName},
		Subject(l, TmplQuizCreated, quizTitle))
}

func (c *SMTPClient) SendQuizResults(emailAddr, quizTitle string, score, maxScore int, l lang.Lang) error {
	return c.renderAndSend(emailAddr, l, TmplQuizResults,
		map[string]any{"QuizTitle": quizTitle, "Score": score, "MaxScore": maxScore},
		Subject(l, TmplQuizResults, quizTitle, score, maxScore))
}

func (c *SMTPClient) SendGradeChanged(emailAddr, quizTitle string, score, maxScore int, l lang.Lang) error {
	return c.renderAndSend(emailAddr, l, TmplGradeChanged,
		map[string]any{"QuizTitle": quizTitle, "Score": score, "MaxScore": maxScore},
		Subject(l, TmplGradeChanged, quizTitle, score, maxScore))
}

func (c *SMTPClient) SendDeadlineReminder(emailAddr, quizTitle, deadline, remainingTime string, l lang.Lang) error {
	return c.renderAndSend(emailAddr, l, TmplDeadlineReminder,
		map[string]string{"QuizTitle": quizTitle, "Deadline": deadline, "RemainingTime": remainingTime},
		Subject(l, TmplDeadlineReminder, quizTitle, remainingTime))
}
