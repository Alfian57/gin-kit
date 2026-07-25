package validation

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

// FieldError describes one failed validation rule without exposing the submitted value.
type FieldError struct {
	Rule       string            `json:"rule"`
	Message    string            `json:"message"`
	Parameters map[string]string `json:"parameters"`
}

// Errors groups failures by their JSON field path.
type Errors struct {
	Fields map[string][]FieldError `json:"fields"`
}

func (e *Errors) Error() string {
	if e == nil {
		return "validation failed"
	}
	return fmt.Sprintf("validation failed for %d field(s)", len(e.Fields))
}

// Context is supplied to message translators.
type Context struct {
	Field     string
	Rule      string
	Parameter string
}

// Translator converts a failed rule into a public, human-readable message.
type Translator func(Context) string

// Validator is an explicit wrapper around go-playground/validator.
type Validator struct {
	engine    *validator.Validate
	messages  map[string]string
	translate Translator
}

// New creates a validator which reports JSON/form field names and English messages.
func New() *Validator {
	engine := validator.New()
	engine.RegisterTagNameFunc(func(field reflect.StructField) string {
		for _, tag := range []string{"json", "form"} {
			name := strings.SplitN(field.Tag.Get(tag), ",", 2)[0]
			if name != "" && name != "-" {
				return name
			}
		}
		return field.Name
	})
	return &Validator{
		engine:    engine,
		messages:  make(map[string]string),
		translate: englishMessage,
	}
}

// Default is safe for concurrent validation after application startup.
var Default = New()

// RegisterRule registers an application-specific validation tag.
func (v *Validator) RegisterRule(name string, fn validator.Func) error {
	return v.engine.RegisterValidation(name, fn)
}

// RegisterMessage overrides a message for a rule. The placeholders {field} and
// {parameter} are expanded when the error is rendered.
func (v *Validator) RegisterMessage(rule, message string) {
	v.messages[rule] = message
}

// SetTranslator replaces the default English translator.
func (v *Validator) SetTranslator(translator Translator) {
	if translator != nil {
		v.translate = translator
	}
}

// Struct validates a request DTO and returns nil or structured field failures.
func (v *Validator) Struct(value any) error {
	if v == nil {
		v = Default
	}
	err := v.engine.Struct(value)
	if err == nil {
		return nil
	}
	var invalid *validator.InvalidValidationError
	if errors.As(err, &invalid) {
		return fmt.Errorf("validate value: %w", err)
	}
	var failures validator.ValidationErrors
	if !errors.As(err, &failures) {
		return err
	}

	result := &Errors{Fields: make(map[string][]FieldError)}
	for _, failure := range failures {
		field := fieldPath(failure.Namespace())
		context := Context{Field: field, Rule: failure.Tag(), Parameter: failure.Param()}
		message := v.messages[failure.Tag()]
		if message == "" {
			message = v.translate(context)
		} else {
			message = strings.NewReplacer(
				"{field}", field,
				"{parameter}", failure.Param(),
			).Replace(message)
		}
		parameters := map[string]string{}
		if failure.Param() != "" {
			parameters[parameterName(failure.Tag())] = failure.Param()
		}
		result.Fields[field] = append(result.Fields[field], FieldError{
			Rule:       failure.Tag(),
			Message:    message,
			Parameters: parameters,
		})
	}
	return result
}

func fieldPath(namespace string) string {
	parts := strings.Split(namespace, ".")
	if len(parts) > 1 {
		parts = parts[1:]
	}
	return strings.Join(parts, ".")
}

func parameterName(rule string) string {
	switch rule {
	case "min":
		return "min"
	case "max":
		return "max"
	case "len":
		return "length"
	case "oneof":
		return "allowed"
	default:
		return "value"
	}
}

func englishMessage(c Context) string {
	switch c.Rule {
	case "required":
		return fmt.Sprintf("The %s field is required.", c.Field)
	case "email":
		return fmt.Sprintf("The %s field must be a valid email address.", c.Field)
	case "min":
		return fmt.Sprintf("The %s field must be at least %s.", c.Field, c.Parameter)
	case "max":
		return fmt.Sprintf("The %s field must not be greater than %s.", c.Field, c.Parameter)
	case "len":
		return fmt.Sprintf("The %s field must contain exactly %s items.", c.Field, c.Parameter)
	case "oneof":
		return fmt.Sprintf("The selected %s is invalid.", c.Field)
	default:
		return fmt.Sprintf("The %s field is invalid.", c.Field)
	}
}
