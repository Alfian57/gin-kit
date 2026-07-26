// Package mail provides transactional email with a fluent message
// builder, an SMTP driver, and a development log driver.
package mail

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"strings"
	"time"

	gomail "github.com/wneessen/go-mail"
)

var (
	ErrNoRecipients  = errors.New("mail: message has no recipients")
	ErrNoBody        = errors.New("mail: message has no body")
	ErrUnknownDriver = errors.New("mail: unknown driver")
)

// Encryption selects the SMTP transport security.
type Encryption string

const (
	EncryptionNone     Encryption = "none"     // plain, port 25
	EncryptionTLS      Encryption = "tls"      // implicit TLS, port 465
	EncryptionSTARTTLS Encryption = "starttls" // mandatory STARTTLS, port 587
)

type Options struct {
	// Driver selects the transport: "log" (default, renders messages into
	// the logger) or "smtp".
	Driver     string
	Host       string
	Port       int // defaults per encryption: 587 starttls, 465 tls, 25 none
	Username   string
	Password   string
	Encryption Encryption // defaults to starttls
	// FromAddress is the default sender, required for the smtp driver.
	FromAddress string
	FromName    string
	Timeout     time.Duration // dial and send, defaults to 15s
	Logger      *slog.Logger  // log driver sink, defaults to slog.Default()
}

// Mailer sends messages.
type Mailer interface {
	Send(ctx context.Context, message *Message) error
	Close() error
}

// New builds a Mailer for the configured driver.
func New(options Options) (Mailer, error) {
	applyOptionDefaults(&options)
	switch options.Driver {
	case "log":
		return &logMailer{options: options}, nil
	case "smtp":
		if options.Host == "" {
			return nil, errors.New("mail: the smtp driver requires a host")
		}
		if options.FromAddress == "" {
			return nil, errors.New("mail: the smtp driver requires a from address")
		}
		return newSMTPMailer(options)
	default:
		return nil, fmt.Errorf("%w %q", ErrUnknownDriver, options.Driver)
	}
}

func applyOptionDefaults(options *Options) {
	if options.Driver == "" {
		options.Driver = "log"
	}
	if options.Encryption == "" {
		options.Encryption = EncryptionSTARTTLS
	}
	if options.Port == 0 {
		switch options.Encryption {
		case EncryptionTLS:
			options.Port = 465
		case EncryptionNone:
			options.Port = 25
		default:
			options.Port = 587
		}
	}
	if options.Timeout <= 0 {
		options.Timeout = 15 * time.Second
	}
	if options.Logger == nil {
		options.Logger = slog.Default()
	}
}

type attachment struct {
	filename    string
	contentType string
	content     []byte
}

// Message is a fluent mail builder. Builder errors (bad template, failed
// attachment read) are deferred and surface at Send.
type Message struct {
	fromAddress string
	fromName    string
	to          []string
	cc          []string
	bcc         []string
	replyTo     string
	subject     string
	text        string
	html        string
	attachments []attachment
	err         error
}

func NewMessage() *Message { return &Message{} }

// From overrides the mailer's default sender for this message.
func (m *Message) From(address, name string) *Message {
	m.fromAddress, m.fromName = address, name
	return m
}

func (m *Message) To(addresses ...string) *Message {
	m.to = append(m.to, addresses...)
	return m
}

func (m *Message) Cc(addresses ...string) *Message {
	m.cc = append(m.cc, addresses...)
	return m
}

func (m *Message) Bcc(addresses ...string) *Message {
	m.bcc = append(m.bcc, addresses...)
	return m
}

func (m *Message) ReplyTo(address string) *Message {
	m.replyTo = address
	return m
}

func (m *Message) Subject(subject string) *Message {
	m.subject = subject
	return m
}

func (m *Message) Text(body string) *Message {
	m.text = body
	return m
}

func (m *Message) HTML(body string) *Message {
	m.html = body
	return m
}

// HTMLTemplate renders the named template with data as the HTML body.
func (m *Message) HTMLTemplate(t *template.Template, name string, data any) *Message {
	body, err := Render(t, name, data)
	if err != nil {
		m.err = errors.Join(m.err, err)
		return m
	}
	m.html = body
	return m
}

// Attach adds an attachment read from r.
func (m *Message) Attach(filename, contentType string, r io.Reader) *Message {
	content, err := io.ReadAll(r)
	if err != nil {
		m.err = errors.Join(m.err, fmt.Errorf("mail: read attachment %s: %w", filename, err))
		return m
	}
	m.attachments = append(m.attachments, attachment{filename: filename, contentType: contentType, content: content})
	return m
}

// Render executes the named template with data and returns the HTML.
func Render(t *template.Template, name string, data any) (string, error) {
	var body strings.Builder
	if err := t.ExecuteTemplate(&body, name, data); err != nil {
		return "", fmt.Errorf("mail: render template %s: %w", name, err)
	}
	return body.String(), nil
}

// build converts a Message into a go-mail message, applying option defaults.
// Both drivers use it so the log driver renders exactly what SMTP would send.
func build(message *Message, options Options) (*gomail.Msg, error) {
	if message.err != nil {
		return nil, message.err
	}
	if len(message.to) == 0 && len(message.cc) == 0 && len(message.bcc) == 0 {
		return nil, ErrNoRecipients
	}
	if message.text == "" && message.html == "" {
		return nil, ErrNoBody
	}
	msg := gomail.NewMsg()
	fromAddress, fromName := message.fromAddress, message.fromName
	if fromAddress == "" {
		fromAddress, fromName = options.FromAddress, options.FromName
	}
	if fromAddress != "" {
		if fromName != "" {
			if err := msg.FromFormat(fromName, fromAddress); err != nil {
				return nil, fmt.Errorf("mail: invalid from address: %w", err)
			}
		} else if err := msg.From(fromAddress); err != nil {
			return nil, fmt.Errorf("mail: invalid from address: %w", err)
		}
	}
	if err := msg.To(message.to...); err != nil {
		return nil, fmt.Errorf("mail: invalid recipient: %w", err)
	}
	if len(message.cc) > 0 {
		if err := msg.Cc(message.cc...); err != nil {
			return nil, fmt.Errorf("mail: invalid cc recipient: %w", err)
		}
	}
	if len(message.bcc) > 0 {
		if err := msg.Bcc(message.bcc...); err != nil {
			return nil, fmt.Errorf("mail: invalid bcc recipient: %w", err)
		}
	}
	if message.replyTo != "" {
		if err := msg.ReplyTo(message.replyTo); err != nil {
			return nil, fmt.Errorf("mail: invalid reply-to address: %w", err)
		}
	}
	msg.Subject(message.subject)
	switch {
	case message.html != "" && message.text != "":
		msg.SetBodyString(gomail.TypeTextPlain, message.text)
		msg.AddAlternativeString(gomail.TypeTextHTML, message.html)
	case message.html != "":
		msg.SetBodyString(gomail.TypeTextHTML, message.html)
	default:
		msg.SetBodyString(gomail.TypeTextPlain, message.text)
	}
	for _, item := range message.attachments {
		attachOptions := []gomail.FileOption{}
		if item.contentType != "" {
			attachOptions = append(attachOptions, gomail.WithFileContentType(gomail.ContentType(item.contentType)))
		}
		msg.AttachReader(item.filename, strings.NewReader(string(item.content)), attachOptions...)
	}
	return msg, nil
}
