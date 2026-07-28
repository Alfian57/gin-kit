package cli

import "fmt"

// Diagnostic is a stable, actionable CLI error. Codes are safe for automation.
type Diagnostic struct {
	// Code store data used by this type.
	Code string
	// Phase store data used by this type.
	Phase string
	// Path store data used by this type.
	Path string
	// Cause store data used by this type.
	Cause error
	// Recovery store data used by this type.
	Recovery string
}

// Error performs this package operation.
func (e *Diagnostic) Error() string {
	message := fmt.Sprintf("[%s] %s failed", e.Code, e.Phase)
	if e.Path != "" {
		message += " for " + e.Path
	}
	if e.Cause != nil {
		message += ": " + e.Cause.Error()
	}
	if e.Recovery != "" {
		message += "\nRecovery: " + e.Recovery
	}
	return message
}

// Unwrap performs this package operation.
func (e *Diagnostic) Unwrap() error { return e.Cause }

// diagnostic performs this package operation.
func diagnostic(code, phase, path string, cause error, recovery string) error {
	return &Diagnostic{Code: code, Phase: phase, Path: path, Cause: cause, Recovery: recovery}
}
