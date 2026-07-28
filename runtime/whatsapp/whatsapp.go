// Package whatsapp sends approved WhatsApp Business Platform templates through
// Meta's Cloud API. It keeps delivery explicit and never persists credentials
// or message parameters.
package whatsapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	// ErrUnknownDriver means Options.Driver is not supported.
	ErrUnknownDriver = errors.New("whatsapp: unknown driver")
	// ErrInvalidRecipient means the destination is not an E.164 phone number.
	ErrInvalidRecipient = errors.New("whatsapp: recipient must be an E.164 phone number")
	// ErrTemplateRequired means a message was sent without an approved template.
	ErrTemplateRequired = errors.New("whatsapp: template name and language are required")
	// ErrDeliveryFailed hides Cloud API response bodies from callers and logs.
	ErrDeliveryFailed = errors.New("whatsapp: Cloud API delivery failed")
)

const defaultAPIBaseURL = "https://graph.facebook.com"

// Options configures a WhatsApp client. Driver is "log" by default or
// "cloud" for Meta's WhatsApp Cloud API.
type Options struct {
	Driver        string
	AccessToken   string
	PhoneNumberID string
	APIVersion    string
	Timeout       time.Duration
	Logger        *slog.Logger
	// APIBaseURL defaults to Meta's Graph API. It is useful for an approved
	// proxy or isolated integration tests; applications normally leave it empty.
	APIBaseURL string
}

// Client sends WhatsApp template messages.
type Client interface {
	Send(context.Context, *Message) error
	Close() error
}

// TemplateParameter is one WhatsApp template parameter. Type is normally
// "text" or "payload". For another Cloud API parameter type (such as
// "currency", "image", or "document"), Data is encoded under the same key
// as Type and can hold the exact approved-template value.
type TemplateParameter struct {
	Type    string `json:"type"`
	Text    string `json:"text,omitempty"`
	Payload string `json:"payload,omitempty"`
	Data    any    `json:"-"`
}

// MarshalJSON matches Meta's dynamic template-parameter shape. Text and
// payload are common scalar forms; other parameter types carry their value in
// a property named after Type.
func (p TemplateParameter) MarshalJSON() ([]byte, error) {
	type parameter struct {
		Type    string `json:"type"`
		Text    string `json:"text,omitempty"`
		Payload string `json:"payload,omitempty"`
	}
	if p.Data == nil || p.Type == "text" || p.Type == "payload" {
		return json.Marshal(parameter{Type: p.Type, Text: p.Text, Payload: p.Payload})
	}
	return json.Marshal(map[string]any{
		"type": p.Type,
		p.Type: p.Data,
	})
}

// TemplateComponent describes one component in an approved template.
// Type is usually "header", "body", or "button". SubType and Index are
// used for button components when required by Meta.
type TemplateComponent struct {
	Type       string              `json:"type"`
	SubType    string              `json:"sub_type,omitempty"`
	Index      string              `json:"index,omitempty"`
	Parameters []TemplateParameter `json:"parameters,omitempty"`
}

// Message is an outbound, approved WhatsApp template message.
type Message struct {
	recipient  string
	template   string
	language   string
	components []TemplateComponent
}

// NewMessage creates an empty message builder.
func NewMessage() *Message { return &Message{} }

// To sets the destination to an E.164 phone number. A leading + is accepted.
func (m *Message) To(recipient string) *Message {
	m.recipient = recipient
	return m
}

// Template selects an approved WhatsApp template and its language code.
func (m *Message) Template(name, language string) *Message {
	m.template = name
	m.language = language
	return m
}

// Components replaces the message's template components.
func (m *Message) Components(components ...TemplateComponent) *Message {
	m.components = append([]TemplateComponent(nil), components...)
	return m
}

// NewAuthenticationMessage builds an AUTHENTICATION template message using a
// text body parameter for the one-time code. The template must already be
// approved in the sender's WhatsApp Business Account.
func NewAuthenticationMessage(recipient, template, language, code string) *Message {
	return NewMessage().
		To(recipient).
		Template(template, language).
		Components(TemplateComponent{
			Type:       "body",
			Parameters: []TemplateParameter{{Type: "text", Text: code}},
		})
}

// New creates a configured WhatsApp client.
func New(options Options) (Client, error) {
	applyDefaults(&options)
	switch options.Driver {
	case "log":
		return &logClient{options: options}, nil
	case "cloud":
		if strings.TrimSpace(options.AccessToken) == "" || strings.TrimSpace(options.PhoneNumberID) == "" || strings.TrimSpace(options.APIVersion) == "" {
			return nil, errors.New("whatsapp: cloud driver requires an access token, phone number ID, and API version")
		}
		if !validAPIVersion(options.APIVersion) {
			return nil, errors.New("whatsapp: API version must look like vNN.N")
		}
		parsed, err := url.ParseRequestURI(options.APIBaseURL)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return nil, errors.New("whatsapp: API base URL must be absolute")
		}
		return &cloudClient{options: options, http: &http.Client{Timeout: options.Timeout}}, nil
	default:
		return nil, fmt.Errorf("%w %q", ErrUnknownDriver, options.Driver)
	}
}

func applyDefaults(options *Options) {
	options.Driver = strings.ToLower(strings.TrimSpace(options.Driver))
	options.AccessToken = strings.TrimSpace(options.AccessToken)
	options.PhoneNumberID = strings.TrimSpace(options.PhoneNumberID)
	if options.Driver == "" {
		options.Driver = "log"
	}
	options.APIVersion = strings.TrimSpace(options.APIVersion)
	options.APIBaseURL = strings.TrimRight(strings.TrimSpace(options.APIBaseURL), "/")
	if options.APIBaseURL == "" {
		options.APIBaseURL = defaultAPIBaseURL
	}
	if options.Timeout <= 0 {
		options.Timeout = 15 * time.Second
	}
	if options.Logger == nil {
		options.Logger = slog.Default()
	}
}

func validateMessage(message *Message) (string, error) {
	if message == nil {
		return "", errors.New("whatsapp: message is required")
	}
	recipient := strings.TrimSpace(message.recipient)
	if strings.HasPrefix(recipient, "+") {
		recipient = recipient[1:]
	}
	if len(recipient) < 8 || len(recipient) > 15 {
		return "", ErrInvalidRecipient
	}
	for _, rune := range recipient {
		if rune < '0' || rune > '9' {
			return "", ErrInvalidRecipient
		}
	}
	if strings.TrimSpace(message.template) == "" || strings.TrimSpace(message.language) == "" {
		return "", ErrTemplateRequired
	}
	return recipient, nil
}

func validAPIVersion(version string) bool {
	if len(version) < 4 || version[0] != 'v' {
		return false
	}
	parts := strings.Split(version[1:], ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return false
	}
	for _, part := range parts {
		for _, rune := range part {
			if rune < '0' || rune > '9' {
				return false
			}
		}
	}
	return true
}

type logClient struct{ options Options }

func (c *logClient) Send(ctx context.Context, message *Message) error {
	recipient, err := validateMessage(message)
	if err != nil {
		return err
	}
	c.options.Logger.InfoContext(ctx, "whatsapp message (log driver)",
		"to", redactRecipient(recipient),
		"template", strings.TrimSpace(message.template),
		"language", strings.TrimSpace(message.language),
		"components", len(message.components),
	)
	return nil
}

func (*logClient) Close() error { return nil }

func redactRecipient(recipient string) string {
	if len(recipient) <= 4 {
		return "***"
	}
	return strings.Repeat("*", len(recipient)-4) + recipient[len(recipient)-4:]
}

type cloudClient struct {
	options Options
	http    *http.Client
}

func (c *cloudClient) Send(ctx context.Context, message *Message) error {
	recipient, err := validateMessage(message)
	if err != nil {
		return err
	}
	payload, err := encodePayload(recipient, message)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint(), payload)
	if err != nil {
		return fmt.Errorf("whatsapp: build Cloud API request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+c.options.AccessToken)
	request.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return fmt.Errorf("%w: request could not be completed", ErrDeliveryFailed)
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("%w: status %d", ErrDeliveryFailed, response.StatusCode)
	}
	return nil
}

func (*cloudClient) Close() error { return nil }
