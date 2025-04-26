package mailer

import (
	"context"
	"fmt"
	"strings"
	"testing"
	// "net/smtp" // Import needed if mocking smtp.SendMail
)

func TestNewMailerSMTP_Valid(t *testing.T) {
	validConf := MailerSMTPConf{
		SMTPHost: "smtp.example.com",
		SMTPPort: 587,
		Username: "user@example.com",
		Password: "password123",
	}

	mailer, err := NewMailerSMTP(validConf)
	if err != nil {
		t.Fatalf("Expected no error for valid config, but got: %v", err)
	}

	if mailer == nil {
		t.Fatal("Expected mailer instance, but got nil")
	}

	// Check if fields are set correctly
	if mailer.smtpHost != validConf.SMTPHost {
		t.Errorf("Expected smtpHost '%s', got '%s'", validConf.SMTPHost, mailer.smtpHost)
	}
	if mailer.smtpPort != validConf.SMTPPort {
		t.Errorf("Expected smtpPort %d, got %d", validConf.SMTPPort, mailer.smtpPort)
	}
	if mailer.username != validConf.Username {
		t.Errorf("Expected username '%s', got '%s'", validConf.Username, mailer.username)
	}
	if mailer.password != validConf.Password {
		t.Errorf("Expected password '%s', got '%s'", validConf.Password, mailer.password)
	}
}

func TestNewMailerSMTP_Invalid(t *testing.T) {
	validConf := func() MailerSMTPConf {
		return MailerSMTPConf{
			SMTPHost: "smtp.example.com",
			SMTPPort: 587,
			Username: "user@example.com",
			Password: "password123",
		}
	}

	testCases := []struct {
		name          string
		modifier      func(*MailerSMTPConf)
		expectedError string
	}{
		{
			name:          "SMTPHost too short",
			modifier:      func(c *MailerSMTPConf) { c.SMTPHost = "a.b" },
			expectedError: fmt.Sprintf("SMTPHost must be between %d and %d characters", ValidMinSMTPHostLength, ValidMaxSMTPHostLength),
		},
		{
			name:          "SMTPHost too long",
			modifier:      func(c *MailerSMTPConf) { c.SMTPHost = strings.Repeat("a", ValidMaxSMTPHostLength+1) },
			expectedError: fmt.Sprintf("SMTPHost must be between %d and %d characters", ValidMinSMTPHostLength, ValidMaxSMTPHostLength),
		},
		{
			name:          "Invalid SMTPPort",
			modifier:      func(c *MailerSMTPConf) { c.SMTPPort = 26 },
			expectedError: fmt.Sprintf("SMTPPort must be one of the following: %s", ValidSMTPPorts),
		},
		{
			name:          "Username too short",
			modifier:      func(c *MailerSMTPConf) { c.Username = "" },
			expectedError: fmt.Sprintf("Username must be between %d and %d characters", ValidMinUsernameLength, ValidMaxUsernameLength),
		},
		{
			name:          "Username too long",
			modifier:      func(c *MailerSMTPConf) { c.Username = strings.Repeat("a", ValidMaxUsernameLength+1) },
			expectedError: fmt.Sprintf("Username must be between %d and %d characters", ValidMinUsernameLength, ValidMaxUsernameLength),
		},
		{
			name:          "Password too short",
			modifier:      func(c *MailerSMTPConf) { c.Password = "pw" },
			expectedError: fmt.Sprintf("Password must be between %d and %d characters", ValidMinPasswordLength, ValidMaxPasswordLength),
		},
		{
			name:          "Password too long",
			modifier:      func(c *MailerSMTPConf) { c.Password = strings.Repeat("a", ValidMaxPasswordLength+1) },
			expectedError: fmt.Sprintf("Password must be between %d and %d characters", ValidMinPasswordLength, ValidMaxPasswordLength),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			conf := validConf()
			tc.modifier(&conf)
			_, err := NewMailerSMTP(conf)

			if err == nil {
				t.Fatalf("Expected error '%s', but got nil", tc.expectedError)
			}

			mailerErr, ok := err.(*MailerError)
			if !ok {
				t.Fatalf("Expected error type *MailerError, but got %T", err)
			}

			if mailerErr.Message != tc.expectedError {
				t.Errorf("Expected error message '%s', but got '%s'", tc.expectedError, mailerErr.Message)
			}
		})
	}
}

// TestSendBasic checks the Send method structure but does not actually send an email.
// Proper testing of Send requires mocking net/smtp.SendMail or using a test SMTP server.
func TestSendBasic(t *testing.T) {
	conf := MailerSMTPConf{
		SMTPHost: "smtp.example.com", // Use a dummy host
		SMTPPort: 587,
		Username: "user",
		Password: "password",
	}
	mailer, err := NewMailerSMTP(conf)
	if err != nil {
		t.Fatalf("Failed to create mailer for basic Send test: %v", err)
	}

	contentBuilder := MailContentBuilder{}
	content, err := contentBuilder.
		WithFromName("Test Sender").
		WithFromAddress("sender@example.com").
		WithToName("Test Recipient").
		WithToAddress("recipient@example.com").
		WithMimeType("text/plain").
		WithSubject("Basic Send Test").
		WithBody("This is a basic test.").
		Build()
	if err != nil {
		t.Fatalf("Failed to build mail content for basic Send test: %v", err)
	}

	// We expect Send to fail because it can't connect to "smtp.example.com",
	// but we are checking that it doesn't panic and returns a MailerError.
	err = mailer.Send(context.Background(), content)

	if err == nil {
		t.Error("Expected an error from Send due to invalid host/credentials, but got nil")
	} else {
		_, ok := err.(*MailerError)
		if !ok {
			t.Errorf("Expected error type *MailerError from Send, but got %T (%v)", err, err)
		} else {
			// Optionally check if the error message indicates a failure to send
			if !strings.Contains(err.Error(), "Failed to send email") {
				t.Errorf("Expected error message to contain 'Failed to send email', but got: %s", err.Error())
			}
		}
	}

	// Note: To truly test the Send logic (formatting, auth), mocking smtp.SendMail is necessary.
	// Example (conceptual):
	// mockSendMail := func(addr string, a smtp.Auth, from string, to []string, msg []byte) error {
	//     // Add checks here for addr, auth, from, to, msg content
	//     fmt.Println("Mock SendMail called!")
	//     return nil // or return an error for testing error handling
	// }
	// // Inject mockSendMail (requires modifying MailerSMTP or using interfaces/dependency injection)
}
