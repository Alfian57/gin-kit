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
	// ErrNoRecipients define package-level implementation state.
	ErrNoRecipients = errors.New("mail: message has no recipients")
	// ErrNoBody define package-level implementation state.
	ErrNoBody = errors.New("mail: message has no body")
	// ErrUnknownDriver define package-level implementation state.
	ErrUnknownDriver = errors.New("mail: unknown driver")
)

// Encryption selects the SMTP transport security.
type Encryption string

const (
	// EncryptionNone define package-level implementation state.
	EncryptionNone Encryption = "none" // plain, port 25
	// EncryptionTLS define package-level implementation state.
	EncryptionTLS Encryption = "tls" // implicit TLS, port 465
	// EncryptionSTARTTLS define package-level implementation state.
	EncryptionSTARTTLS Encryption = "starttls" // mandatory STARTTLS, port 587
)

// Options defines an implementation type used by this package.
type Options struct {
	// Driver selects the transport: "log" (default, renders messages into
	// the logger) or "smtp".
	Driver string
	// Host store data used by this type.
	Host string
	Port int // defaults per encryption: 587 starttls, 465 tls, 25 none
	// Username store data used by this type.
	Username string
	// Password store data used by this type.
	Password   string
	Encryption Encryption // defaults to starttls
	// FromAddress is the default sender, required for the smtp driver.
	FromAddress string
	// FromName store data used by this type.
	FromName string
	Timeout  time.Duration // dial and send, defaults to 15s
	Logger   *slog.Logger  // log driver sink, defaults to slog.Default()
}

// Mailer sends messages.
type Mailer interface {
	// Send define an operation required by this interface.
	Send(ctx context.Context, message *Message) error
	// Close define an operation required by this interface.
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

// applyOptionDefaults performs this package operation.
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

// attachment defines an implementation type used by this package.
type attachment struct {
	// filename store data used by this type.
	filename string
	// contentType store data used by this type.
	contentType string
	// content store data used by this type.
	content []byte
}

// Message is a fluent mail builder. Builder errors (bad template, failed
// attachment read) are deferred and surface at Send.
type Message struct {
	// fromAddress store data used by this type.
	fromAddress string
	// fromName store data used by this type.
	fromName string
	// to store data used by this type.
	to []string
	// cc store data used by this type.
	cc []string
	// bcc store data used by this type.
	bcc []string
	// replyTo store data used by this type.
	replyTo string
	// subject store data used by this type.
	subject string
	// text store data used by this type.
	text string
	// html store data used by this type.
	html string
	// attachments store data used by this type.
	attachments []attachment
	// err store data used by this type.
	err error
}

// NewMessage performs this package operation.
func NewMessage() *Message { return &Message{} }

// From overrides the mailer's default sender for this message.
func (m *Message) From(address, name string) *Message {
	m.fromAddress, m.fromName = address, name
	return m
}

// To performs this package operation.
func (m *Message) To(addresses ...string) *Message {
	m.to = append(m.to, addresses...)
	return m
}

// Cc performs this package operation.
func (m *Message) Cc(addresses ...string) *Message {
	m.cc = append(m.cc, addresses...)
	return m
}

// Bcc performs this package operation.
func (m *Message) Bcc(addresses ...string) *Message {
	m.bcc = append(m.bcc, addresses...)
	return m
}

// ReplyTo performs this package operation.
func (m *Message) ReplyTo(address string) *Message {
	m.replyTo = address
	return m
}

// Subject performs this package operation.
func (m *Message) Subject(subject string) *Message {
	m.subject = subject
	return m
}

// Text performs this package operation.
func (m *Message) Text(body string) *Message {
	m.text = body
	return m
}

// HTML performs this package operation.
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

// AttachmentInfo describes an attachment without exposing its content.
type AttachmentInfo struct {
	// Filename store data used by this type.
	Filename string `json:"filename"`
	// ContentType store data used by this type.
	ContentType string `json:"content_type"`
	// Size store data used by this type.
	Size int `json:"size"`
}

// Envelope is a read-only snapshot of a message for inspection tooling such
// as the devtools mail outbox. Attachment content is never exposed, only its
// metadata.
type Envelope struct {
	// From store data used by this type.
	From string `json:"from,omitempty"`
	// FromName store data used by this type.
	FromName string `json:"from_name,omitempty"`
	// To store data used by this type.
	To []string `json:"to,omitempty"`
	// Cc store data used by this type.
	Cc []string `json:"cc,omitempty"`
	// Bcc store data used by this type.
	Bcc []string `json:"bcc,omitempty"`
	// ReplyTo store data used by this type.
	ReplyTo string `json:"reply_to,omitempty"`
	// Subject store data used by this type.
	Subject string `json:"subject"`
	// Text store data used by this type.
	Text string `json:"text,omitempty"`
	// HTML store data used by this type.
	HTML string `json:"html,omitempty"`
	// Attachments store data used by this type.
	Attachments []AttachmentInfo `json:"attachments,omitempty"`
}

// Envelope snapshots the message. Slices are copied so later builder calls
// do not mutate the snapshot.
func (m *Message) Envelope() Envelope {
	envelope := Envelope{
		From:     m.fromAddress,
		FromName: m.fromName,
		To:       append([]string(nil), m.to...),
		Cc:       append([]string(nil), m.cc...),
		Bcc:      append([]string(nil), m.bcc...),
		ReplyTo:  m.replyTo,
		Subject:  m.subject,
		Text:     m.text,
		HTML:     m.html,
	}
	for _, item := range m.attachments {
		envelope.Attachments = append(envelope.Attachments, AttachmentInfo{
			Filename:    item.filename,
			ContentType: item.contentType,
			Size:        len(item.content),
		})
	}
	return envelope
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
