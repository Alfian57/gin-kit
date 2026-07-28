package mail

import (
	"context"
	"fmt"

	gomail "github.com/wneessen/go-mail"
)

// smtpMailer sends messages through an SMTP server via wneessen/go-mail.
type smtpMailer struct {
	client  *gomail.Client
	options Options
}

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

func (m *smtpMailer) Send(ctx context.Context, message *Message) error {
	msg, err := build(message, m.options)
	if err != nil {
		return err
	}
	return m.client.DialAndSendWithContext(ctx, msg)
}

func (m *smtpMailer) Close() error { return nil }
