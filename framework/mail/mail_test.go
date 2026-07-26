package mail

import (
	"context"
	"errors"
	"html/template"
	"log/slog"
	"strings"
	"testing"
)

func TestNewValidatesOptions(t *testing.T) {
	for _, test := range []struct {
		name    string
		options Options
	}{
		{"unknown driver", Options{Driver: "carrier-pigeon"}},
		{"smtp without host", Options{Driver: "smtp", FromAddress: "a@b.c"}},
		{"smtp without from", Options{Driver: "smtp", Host: "smtp.example.com"}},
		{"smtp with bad encryption", Options{Driver: "smtp", Host: "h", FromAddress: "a@b.c", Encryption: "quantum"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := New(test.options); err == nil {
				t.Fatal("expected constructor error")
			}
		})
	}
	if _, err := New(Options{}); err != nil {
		t.Fatalf("log driver default rejected: %v", err)
	}
}

func TestPortDefaultsFollowEncryption(t *testing.T) {
	for encryption, wantPort := range map[Encryption]int{
		EncryptionSTARTTLS: 587,
		EncryptionTLS:      465,
		EncryptionNone:     25,
	} {
		options := Options{Encryption: encryption}
		applyOptionDefaults(&options)
		if options.Port != wantPort {
			t.Fatalf("%s default port = %d, want %d", encryption, options.Port, wantPort)
		}
	}
}

func TestBuildProducesFullMIMEMessage(t *testing.T) {
	message := NewMessage().
		To("to@example.com").
		Cc("cc@example.com").
		ReplyTo("reply@example.com").
		Subject("Monthly report").
		Text("plain body").
		HTML("<h1>html body</h1>").
		Attach("report.txt", "text/plain", strings.NewReader("attached content"))
	msg, err := build(message, Options{FromAddress: "sender@example.com", FromName: "Sender"})
	if err != nil {
		t.Fatal(err)
	}
	var rendered strings.Builder
	if _, err := msg.WriteTo(&rendered); err != nil {
		t.Fatal(err)
	}
	output := rendered.String()
	for _, expected := range []string{
		"To: <to@example.com>", "Cc: <cc@example.com>", "Reply-To: <reply@example.com>",
		"Subject: Monthly report", "plain body", "html body", "report.txt",
		`"Sender" <sender@example.com>`,
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("rendered message missing %q:\n%s", expected, output)
		}
	}
}

func TestBuildRejectsIncompleteMessages(t *testing.T) {
	if _, err := build(NewMessage().Text("body"), Options{}); !errors.Is(err, ErrNoRecipients) {
		t.Fatalf("missing recipients accepted: %v", err)
	}
	if _, err := build(NewMessage().To("a@b.c"), Options{}); !errors.Is(err, ErrNoBody) {
		t.Fatalf("missing body accepted: %v", err)
	}
}

func TestHTMLTemplateDefersRenderErrors(t *testing.T) {
	tmpl := template.Must(template.New("welcome").Parse("Hello {{.Name}}"))
	message := NewMessage().To("a@b.c").HTMLTemplate(tmpl, "absent", nil)
	if _, err := build(message, Options{}); err == nil {
		t.Fatal("missing template name did not surface at build")
	}

	ok := NewMessage().To("a@b.c").HTMLTemplate(tmpl, "welcome", map[string]string{"Name": "gin-kit"})
	msg, err := build(ok, Options{})
	if err != nil {
		t.Fatal(err)
	}
	var rendered strings.Builder
	if _, err := msg.WriteTo(&rendered); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rendered.String(), "Hello gin-kit") {
		t.Fatalf("template not rendered:\n%s", rendered.String())
	}
}

func TestLogDriverWritesRenderedMessage(t *testing.T) {
	var output strings.Builder
	mailer, err := New(Options{Logger: slog.New(slog.NewTextHandler(&output, nil)), FromAddress: "dev@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	message := NewMessage().To("user@example.com").Subject("Welcome aboard").Text("Hi there")
	if err := mailer.Send(context.Background(), message); err != nil {
		t.Fatal(err)
	}
	logged := output.String()
	if !strings.Contains(logged, "user@example.com") || !strings.Contains(logged, "Welcome aboard") {
		t.Fatalf("log driver output incomplete:\n%s", logged)
	}
	if err := mailer.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestRenderTable(t *testing.T) {
	tmpl := template.Must(template.New("greet").Parse("Hi {{.Name}}"))
	body, err := Render(tmpl, "greet", map[string]string{"Name": "dev"})
	if err != nil || body != "Hi dev" {
		t.Fatalf("render: %q err=%v", body, err)
	}
	if _, err := Render(tmpl, "absent", nil); err == nil {
		t.Fatal("missing template accepted")
	}
}
