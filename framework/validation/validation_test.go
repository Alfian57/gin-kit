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

func TestEnglishMessagesCoverCommonRules(t *testing.T) {
	type profile struct {
		Age      int    `json:"age" validate:"gte=18,lte=120"`
		Score    int    `json:"score" validate:"gt=0,lt=100"`
		Website  string `json:"website" validate:"url"`
		OwnerID  string `json:"owner_id" validate:"uuid"`
		Code     string `json:"code" validate:"numeric"`
		Handle   string `json:"handle" validate:"alphanum"`
		BornAt   string `json:"born_at" validate:"datetime=2006-01-02"`
		Confirm  string `json:"confirm" validate:"eqfield=Handle"`
		Status   string `json:"status" validate:"ne=banned"`
		Slug     string `json:"slug" validate:"startswith=go-"`
		Filename string `json:"filename" validate:"endswith=.txt"`
	}
	failures := New().Struct(profile{
		Age:      12,
		Score:    -1,
		Website:  "not a url",
		OwnerID:  "not-a-uuid",
		Code:     "abc",
		Handle:   "a b",
		BornAt:   "yesterday",
		Confirm:  "different",
		Status:   "banned",
		Slug:     "kit",
		Filename: "notes.md",
	}).(*Errors)
	for field, expected := range map[string]struct {
		message   string
		parameter string
		value     string
	}{
		"age":      {"The age field must be at least 18.", "min", "18"},
		"score":    {"The score field must be greater than 0.", "greater_than", "0"},
		"website":  {"The website field must be a valid URL.", "", ""},
		"owner_id": {"The owner_id field must be a valid UUID.", "", ""},
		"code":     {"The code field must be a number.", "", ""},
		"handle":   {"The handle field must only contain letters and numbers.", "", ""},
		"born_at":  {"The born_at field must match the 2006-01-02 format.", "layout", "2006-01-02"},
		"confirm":  {"The confirm field must match the Handle field.", "other", "Handle"},
		"status":   {"The status field must not be banned.", "disallowed", "banned"},
		"slug":     {"The slug field must start with go-.", "prefix", "go-"},
		"filename": {"The filename field must end with .txt.", "suffix", ".txt"},
	} {
		entries := failures.Fields[field]
		if len(entries) == 0 {
			t.Fatalf("no failure recorded for %s: %#v", field, failures.Fields)
		}
		if entries[0].Message != expected.message {
			t.Errorf("%s message = %q, want %q", field, entries[0].Message, expected.message)
		}
		if expected.parameter != "" && entries[0].Parameters[expected.parameter] != expected.value {
			t.Errorf("%s parameters = %#v, want %s=%s", field, entries[0].Parameters, expected.parameter, expected.value)
		}
	}
}
