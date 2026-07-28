// Package authz provides explicit, allowlist-style authorization decisions.
//
// Policies return a Decision built with Allow or Deny. Handlers enforce a
// decision with Authorize, which writes the canonical 403 forbidden envelope
// on deny; services convert one into an error with Decision.Err. Deny
// reasons are internal: they are logged and wrapped as error causes, but
// never serialized into HTTP responses.
package authz

import (
	"errors"
	"net/http"

	"github.com/Alfian57/gin-kit/runtime/httpx"
	"github.com/gin-gonic/gin"
)

const (
	// forbiddenCode define package-level implementation state.
	forbiddenCode = "forbidden"
	// forbiddenMessage define package-level implementation state.
	forbiddenMessage = "You are not allowed to perform this action."
)

// Decision is the outcome of a policy check. The zero value denies, so a
// forgotten rule fails closed.
type Decision struct {
	// Allowed store data used by this type.
	Allowed bool
	Reason  string // internal; logged, never serialized
}

// Allow grants the action.
func Allow() Decision { return Decision{Allowed: true} }

// Deny rejects the action. The reason stays internal — it is logged and
// carried as an error cause, never sent to clients.
func Deny(reason string) Decision { return Decision{Reason: reason} }

// Err returns nil when the decision allows the action. Otherwise it returns
// a stable 403 *httpx.Error with code "forbidden"; the deny reason travels
// only as the wrapped cause, so the response body never varies with it.
func (d Decision) Err() error {
	if d.Allowed {
		return nil
	}
	return httpx.WrapError(http.StatusForbidden, forbiddenCode, forbiddenMessage, errors.New(d.denyReason()))
}

// Authorize enforces a decision at the HTTP boundary. When denied it logs
// the internal reason through the request-scoped logger, writes the
// canonical 403 forbidden envelope, aborts the request, and returns false.
// When allowed it writes nothing and returns true.
func Authorize(c *gin.Context, d Decision) bool {
	if d.Allowed {
		return true
	}
	httpx.Logger(c).Warn("authorization denied", "reason", d.denyReason())
	httpx.Fail(c, httpx.NewError(http.StatusForbidden, forbiddenCode, forbiddenMessage))
	return false
}

// denyReason performs this package operation.
func (d Decision) denyReason() string {
	if d.Reason == "" {
		return "denied"
	}
	return d.Reason
}
