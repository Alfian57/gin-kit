package validation

import (
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"
)

func TestStructReturnsDetailedJSONFieldErrors(t *testing.T) {
	type address struct {
		City string `json:"city" validate:"required"`
	}
	type request struct {
		Email   string  `json:"email" validate:"required,email"`
		Role    string  `json:"role" validate:"oneof=admin member"`
		Address address `json:"address" validate:"required"`
	}
	v := New()
	err := v.Struct(request{Role: "owner"})
	var failures *Errors
	if !strings.Contains(err.Error(), "3 field") {
		t.Fatalf("unexpected error: %v", err)
	}
	failures = err.(*Errors)
	if got := failures.Fields["email"][0].Message; got != "The email field is required." {
		t.Fatalf("unexpected message: %q", got)
	}
	if got := failures.Fields["role"][0].Parameters["allowed"]; got != "admin member" {
		t.Fatalf("unexpected parameter: %q", got)
	}
	if _, ok := failures.Fields["address.city"]; !ok {
		t.Fatalf("nested JSON field missing: %#v", failures.Fields)
	}
}

func TestCustomRuleMessageAndTranslator(t *testing.T) {
	v := New()
	if err := v.RegisterRule("slug", func(fl validator.FieldLevel) bool {
		return !strings.Contains(fl.Field().String(), " ")
	}); err != nil {
		t.Fatal(err)
	}
	v.RegisterMessage("slug", "{field} cannot contain spaces")
	type input struct {
		Name string `json:"name" validate:"slug"`
	}
	err := v.Struct(input{Name: "not valid"}).(*Errors)
	if got := err.Fields["name"][0].Message; got != "name cannot contain spaces" {
		t.Fatalf("unexpected custom message: %q", got)
	}

	v = New()
	v.SetTranslator(func(c Context) string { return "translated:" + c.Field })
	type required struct {
		Name string `json:"name" validate:"required"`
	}
	err = v.Struct(required{}).(*Errors)
	if got := err.Fields["name"][0].Message; got != "translated:name" {
		t.Fatalf("unexpected translated message: %q", got)
	}
}
