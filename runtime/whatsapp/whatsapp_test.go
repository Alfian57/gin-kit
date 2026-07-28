package whatsapp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewRejectsInvalidCloudOptions(t *testing.T) {
	for _, options := range []Options{
		{Driver: "carrier-pigeon"},
		{Driver: "cloud"},
		{Driver: "cloud", AccessToken: "token", PhoneNumberID: "123", APIVersion: "latest"},
	} {
		if _, err := New(options); err == nil {
			t.Fatalf("options %+v were accepted", options)
		}
	}
}

func TestCloudSendsAuthenticationTemplate(t *testing.T) {
	var received struct {
		MessagingProduct string `json:"messaging_product"`
		To               string `json:"to"`
		Type             string `json:"type"`
		Template         struct {
			Name     string `json:"name"`
			Language struct {
				Code string `json:"code"`
			} `json:"language"`
			Components []TemplateComponent `json:"components"`
		} `json:"template"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v25.0/123/messages" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatal("missing access token")
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := New(Options{Driver: "cloud", AccessToken: "test-token", PhoneNumberID: "123", APIVersion: "v25.0", APIBaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Send(context.Background(), NewAuthenticationMessage("+62812345678", "login_code", "id", "654321")); err != nil {
		t.Fatal(err)
	}
	if received.MessagingProduct != "whatsapp" || received.To != "62812345678" || received.Type != "template" || received.Template.Name != "login_code" || received.Template.Language.Code != "id" {
		t.Fatalf("unexpected payload: %+v", received)
	}
	if len(received.Template.Components) != 1 || received.Template.Components[0].Parameters[0].Text != "654321" {
		t.Fatalf("OTP component missing: %+v", received.Template.Components)
	}
}

func TestLogDriverRedactsRecipientAndParameters(t *testing.T) {
	var output bytes.Buffer
	client, err := New(Options{Logger: slog.New(slog.NewTextHandler(&output, nil))})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Send(context.Background(), NewAuthenticationMessage("+62812345678", "login_code", "id", "654321")); err != nil {
		t.Fatal(err)
	}
	logged := output.String()
	for _, secret := range []string{"62812345678", "654321"} {
		if strings.Contains(logged, secret) {
			t.Fatalf("log exposed %q: %s", secret, logged)
		}
	}
	if !strings.Contains(logged, "*******5678") || !strings.Contains(logged, "login_code") {
		t.Fatalf("log did not include safe metadata: %s", logged)
	}
}

func TestCloudFailureDoesNotExposeResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":{"message":"test-token must stay private"}}`)
	}))
	defer server.Close()
	client, err := New(Options{Driver: "cloud", AccessToken: "test-token", PhoneNumberID: "123", APIVersion: "v25.0", APIBaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	err = client.Send(context.Background(), NewAuthenticationMessage("62812345678", "login_code", "id", "654321"))
	if !errors.Is(err, ErrDeliveryFailed) || strings.Contains(err.Error(), "test-token") {
		t.Fatalf("unsafe Cloud API error: %v", err)
	}
}

func TestTemplateParameterDataUsesParameterTypeKey(t *testing.T) {
	encoded, err := json.Marshal(TemplateParameter{
		Type: "currency",
		Data: map[string]any{"fallback_value": "Rp10.000", "code": "IDR", "amount_1000": 10000},
	})
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if string(decoded["type"]) != `"currency"` || len(decoded["currency"]) == 0 || len(decoded["data"]) != 0 {
		t.Fatalf("parameter shape = %s", encoded)
	}
}

func TestMessageValidation(t *testing.T) {
	client, err := New(Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Send(context.Background(), NewMessage().To("not-a-phone").Template("template", "en")); !errors.Is(err, ErrInvalidRecipient) {
		t.Fatalf("recipient error = %v", err)
	}
	if err := client.Send(context.Background(), NewMessage().To("62812345678")); !errors.Is(err, ErrTemplateRequired) {
		t.Fatalf("template error = %v", err)
	}
}

func TestDefaultTimeout(t *testing.T) {
	client, err := New(Options{Driver: "cloud", AccessToken: "token", PhoneNumberID: "123", APIVersion: "v25.0"})
	if err != nil {
		t.Fatal(err)
	}
	cloud := client.(*cloudClient)
	if cloud.http.Timeout != 15*time.Second {
		t.Fatalf("timeout = %s", cloud.http.Timeout)
	}
}
