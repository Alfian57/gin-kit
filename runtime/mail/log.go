package mail

import (
	"context"
	"strings"
)

// logMailer renders the full message and writes it to the logger instead of
// sending it — the development default.
type logMailer struct {
	options Options
}

func (m *logMailer) Send(ctx context.Context, message *Message) error {
	msg, err := build(message, m.options)
	if err != nil {
		return err
	}
	var rendered strings.Builder
	if _, err := msg.WriteTo(&rendered); err != nil {
		return err
	}
	m.options.Logger.InfoContext(ctx, "mail message (log driver)",
		"to", strings.Join(message.to, ", "),
		"subject", message.subject,
		"message", rendered.String(),
	)
	return nil
}

func (m *logMailer) Close() error { return nil }
