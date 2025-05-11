package mailer

import (
	"context"
	"fmt"
	"net/smtp"
	"slices"
	"strings"
)

const (
	ValidMinSMTPHostLength = 5
	ValidMaxSMTPHostLength = 100
	ValidSMTPPorts         = "25|465|587|1025|8025"
	ValidMinUsernameLength = 1
	ValidMaxUsernameLength = 100
	ValidMinPasswordLength = 3
	ValidMaxPasswordLength = 100
)

type MailerSMTPConf struct {
	// SMTPHost is the hostname of the SMTP server (5-100 chars). *Required*.
	// It must be a valid hostname or IP address.
	SMTPHost string

	// SMTPPort is the port of the SMTP server (must be 25, 465, or 587). *Required*.
	// Common ports are 25 (SMTP), 465 (SMTPS), and 587 (Submission).
	// Note: 1025 and 8025 are also valid for some configurations.
	SMTPPort int

	// Username is the SMTP username for authentication (1-100 chars). *Required*.
	Username string

	// Password is the SMTP password for authentication (3-100 chars). *Required*.
	// It should be kept secret and not logged or exposed.
	Password string
}

type MailerSMTP struct {
	smtpHost string
	smtpPort int
	username string
	password string
}

func NewMailerSMTP(conf MailerSMTPConf) (*MailerSMTP, error) {
	if len(conf.SMTPHost) < ValidMinSMTPHostLength || len(conf.SMTPHost) > ValidMaxSMTPHostLength {
		return nil, &MailerError{
			Message: fmt.Sprintf("SMTPHost must be between %d and %d characters", ValidMinSMTPHostLength, ValidMaxSMTPHostLength),
		}
	}

	if !slices.Contains(strings.Split(ValidSMTPPorts, "|"), fmt.Sprint(conf.SMTPPort)) {
		return nil, &MailerError{
			Message: fmt.Sprintf("SMTPPort must be one of the following: %s", ValidSMTPPorts),
		}
	}

	if len(conf.Username) < ValidMinUsernameLength || len(conf.Username) > ValidMaxUsernameLength {
		return nil, &MailerError{
			Message: fmt.Sprintf("Username must be between %d and %d characters", ValidMinUsernameLength, ValidMaxUsernameLength),
		}
	}

	if len(conf.Password) < ValidMinPasswordLength || len(conf.Password) > ValidMaxPasswordLength {
		return nil, &MailerError{
			Message: fmt.Sprintf("Password must be between %d and %d characters", ValidMinPasswordLength, ValidMaxPasswordLength),
		}
	}

	return &MailerSMTP{
		smtpHost: conf.SMTPHost,
		smtpPort: conf.SMTPPort,
		username: conf.Username,
		password: conf.Password,
	}, nil
}

func (m *MailerSMTP) Send(ctx context.Context, content MailContent) error {
	msgTpl := `From: %s <%s>
To: %s <%s>
Subject: %s
MIME-version: 1.0;
Content-Type: %s;
%s

`
	msg := fmt.Sprintf(
		msgTpl,
		content.fromName,
		content.fromAddress,
		content.toName,
		content.toAddress,
		content.subject,
		content.mimeType,
		content.body,
	)

	if err := smtp.SendMail(
		fmt.Sprintf("%s:%d", m.smtpHost, m.smtpPort),
		smtp.PlainAuth("", m.username, m.password, m.smtpHost),
		m.username,
		[]string{content.toAddress},
		[]byte(msg),
	); err != nil {
		return &MailerError{
			Message: fmt.Sprintf("Failed to send email: %v", err),
		}
	}

	return nil
}
