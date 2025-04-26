package mailer

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

const (
	ValidMinFromNameLength    = 1
	ValidMaxFromNameLength    = 100
	ValidMinFromAddressLength = 5
	ValidMaxFromAddressLength = 50
	ValidMinToNameLength      = 1
	ValidMaxToNameLength      = 100
	ValidMinToAddressLength   = 5
	ValidMaxToAddressLength   = 50
	ValidMimeType             = "text/plain|text/html"
	ValidMinSubjectLength     = 1
	ValidMaxSubjectLength     = 255
	ValidMinBodyLength        = 1
	ValidMaxBodyLength        = 20000
)

type MimeType string

const (
	MimeTypeTextPlain MimeType = "text/plain"
	MimeTypeTextHTML  MimeType = "text/html"
)

func (m MimeType) String() string {
	return string(m)
}

func (m MimeType) IsValid() bool {
	return m == MimeTypeTextPlain || m == MimeTypeTextHTML
}

type MailerError struct {
	Message string
}

func (e *MailerError) Error() string {
	return e.Message
}

type MailContent struct {
	// fromName: The name of the sender
	// e.g. "John Doe"
	fromName string

	// fromAddress: The email address of the sender
	// e.g. "john.doe@example.com"
	fromAddress string

	// toName: The name of the recipient
	// e.g. "Jane Doe"
	toName string

	// toAddress: The email address of the recipient
	// e.g. "jane.doe@example.com"
	toAddress string

	// mimeType: The MIME type of the email
	// e.g. "text/plain" or "text/html"
	mimeType string

	// subject is the subject of the email
	subject string

	// Body is the body of the email
	body string
}

type MailContentBuilder struct {
	MailContent MailContent
}

func (b *MailContentBuilder) WithFromName(name string) *MailContentBuilder {
	b.MailContent.fromName = name
	return b
}

func (b *MailContentBuilder) WithFromAddress(address string) *MailContentBuilder {
	b.MailContent.fromAddress = address
	return b
}

func (b *MailContentBuilder) WithToName(name string) *MailContentBuilder {
	b.MailContent.toName = name
	return b
}

func (b *MailContentBuilder) WithToAddress(address string) *MailContentBuilder {
	b.MailContent.toAddress = address
	return b
}

func (b *MailContentBuilder) WithMimeType(mimeType MimeType) *MailContentBuilder {
	// Remove the defaulting logic. Validation happens in Build().
	b.MailContent.mimeType = mimeType.String()
	return b
}

func (b *MailContentBuilder) WithMimeTypeAsString(mimeType string) *MailContentBuilder {
	return b.WithMimeType(MimeType(mimeType))
}

func (b *MailContentBuilder) WithSubject(subject string) *MailContentBuilder {
	b.MailContent.subject = subject
	return b
}

func (b *MailContentBuilder) WithBody(body string) *MailContentBuilder {
	b.MailContent.body = body
	return b
}

func (b *MailContentBuilder) Build() (MailContent, error) {
	if len(b.MailContent.fromName) < ValidMinFromNameLength || len(b.MailContent.fromName) > ValidMaxFromNameLength {
		return MailContent{}, &MailerError{
			Message: fmt.Sprintf("fromName must be between %d and %d characters", ValidMinFromNameLength, ValidMaxFromNameLength),
		}
	}

	if len(b.MailContent.toName) < ValidMinToNameLength || len(b.MailContent.toName) > ValidMaxToNameLength {
		return MailContent{}, &MailerError{
			Message: fmt.Sprintf("toName must be between %d and %d characters", ValidMinToNameLength, ValidMaxToNameLength),
		}
	}

	if len(b.MailContent.toAddress) < ValidMinToAddressLength || len(b.MailContent.toAddress) > ValidMaxToAddressLength {
		return MailContent{}, &MailerError{
			Message: fmt.Sprintf("toAddress must be between %d and %d characters", ValidMinToAddressLength, ValidMaxToAddressLength),
		}
	}

	if !slices.Contains(strings.Split(ValidMimeType, "|"), b.MailContent.mimeType) {
		return MailContent{}, &MailerError{
			Message: fmt.Sprintf("mimeType must be one of the following: %s", ValidMimeType),
		}
	}

	if len(b.MailContent.subject) < ValidMinSubjectLength || len(b.MailContent.subject) > ValidMaxSubjectLength {
		return MailContent{}, &MailerError{
			Message: fmt.Sprintf("subject must be between %d and %d characters", ValidMinSubjectLength, ValidMaxSubjectLength),
		}
	}

	if len(b.MailContent.body) < ValidMinBodyLength || len(b.MailContent.body) > ValidMaxBodyLength {
		return MailContent{}, &MailerError{
			Message: fmt.Sprintf("body must be between %d and %d characters", ValidMinBodyLength, ValidMaxBodyLength),
		}
	}

	return b.MailContent, nil
}

// MailerService is an interface must be implemented by any mailer service
// that is used to send emails.
type MailerService interface {
	Send(ctx context.Context, content MailContent) error
}
