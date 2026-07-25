package cli

import "fmt"

// Diagnostic is a stable, actionable CLI error. Codes are safe for automation.
type Diagnostic struct {
	Code     string
	Phase    string
	Path     string
	Cause    error
	Recovery string
}

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

func (e *Diagnostic) Unwrap() error { return e.Cause }

func diagnostic(code, phase, path string, cause error, recovery string) error {
	return &Diagnostic{Code: code, Phase: phase, Path: path, Cause: cause, Recovery: recovery}
}
