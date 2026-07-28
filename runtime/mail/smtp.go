package mail

import (
	"context"
	"fmt"

	gomail "github.com/wneessen/go-mail"
)

// smtpMailer sends messages through an SMTP server via wneessen/go-mail.
type smtpMailer struct {
	// client store data used by this type.
	client *gomail.Client
	// options store data used by this type.
	options Options
}

// newSMTPMailer performs this package operation.
func newSMTPMailer(options Options) (*smtpMailer, error) {
	clientOptions := []gomail.Option{
		gomail.WithPort(options.Port),
		gomail.WithTimeout(options.Timeout),
	}
	switch options.Encryption {
	case EncryptionNone:
		clientOptions = append(clientOptions, gomail.WithTLSPolicy(gomail.NoTLS))
	case EncryptionTLS:
		clientOptions = append(clientOptions, gomail.WithSSL())
	case EncryptionSTARTTLS:
		clientOptions = append(clientOptions, gomail.WithTLSPolicy(gomail.TLSMandatory))
	default:
		return nil, fmt.Errorf("mail: unknown encryption %q", options.Encryption)
	}
	if options.Username != "" {
		clientOptions = append(clientOptions,
			gomail.WithSMTPAuth(gomail.SMTPAuthAutoDiscover),
			gomail.WithUsername(options.Username),
			gomail.WithPassword(options.Password),
		)
	}
	client, err := gomail.NewClient(options.Host, clientOptions...)
	if err != nil {
		return nil, fmt.Errorf("mail: smtp client: %w", err)
	}
	return &smtpMailer{client: client, options: options}, nil
}

// Send performs this package operation.
func (m *smtpMailer) Send(ctx context.Context, message *Message) error {
	msg, err := build(message, m.options)
	if err != nil {
		return err
	}
	return m.client.DialAndSendWithContext(ctx, msg)
}

// Close performs this package operation.
func (m *smtpMailer) Close() error { return nil }
