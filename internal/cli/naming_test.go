package cli

import "testing"

func TestSnakeCaseHandlesAcronymsAndSeparators(t *testing.T) {
	for input, want := range map[string]string{
		"Ticket":     "ticket",
		"APIKey":     "api_key",
		"HTMLParser": "html_parser",
		"api-key":    "api_key",
		"OrderItem":  "order_item",
		"user_id":    "user_id",
		"HTTPServer": "http_server",
	} {
		if got := snakeCase(input); got != want {
			t.Errorf("snakeCase(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestPascalAndCamelCase(t *testing.T) {
	for input, want := range map[string][2]string{
		"api-key":    {"ApiKey", "apiKey"},
		"order_item": {"OrderItem", "orderItem"},
		"Ticket":     {"Ticket", "ticket"},
	} {
		if got := pascalCase(input); got != want[0] {
			t.Errorf("pascalCase(%q) = %q, want %q", input, got, want[0])
		}
		if got := camelCase(input); got != want[1] {
			t.Errorf("camelCase(%q) = %q, want %q", input, got, want[1])
		}
	}
}

func TestPluralize(t *testing.T) {
	for input, want := range map[string]string{
		"ticket":  "tickets",
		"city":    "cities",
		"box":     "boxes",
		"class":   "classes",
		"dish":    "dishes",
		"day":     "days",
		"invoice": "invoices",
	} {
		if got := pluralize(input); got != want {
			t.Errorf("pluralize(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestValidateGeneratorNameRejectsBadInput(t *testing.T) {
	for _, name := range []string{"", "123abc", "type", "!!"} {
		if _, _, _, err := validateGeneratorName(name); err == nil {
			t.Errorf("name %q accepted", name)
		}
	}
	pascal, camel, snake, err := validateGeneratorName("api-key")
	if err != nil || pascal != "ApiKey" || camel != "apiKey" || snake != "api_key" {
		t.Fatalf("api-key: %q %q %q %v", pascal, camel, snake, err)
	}
}
