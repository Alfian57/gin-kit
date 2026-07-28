package session

import (
	"encoding/gob"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// Flash is a one-shot message that survives exactly one redirect.
type Flash struct {
	// Kind store data used by this type.
	Kind string
	// Message store data used by this type.
	Message string
}

// init initializes package-level implementation state.
func init() { gob.Register([]Flash{}) }

// flashKey define package-level implementation state.
const flashKey = "gin-kit.flashes"

// PutFlash stores a flash message, e.g. PutFlash(c, "success", "Task created.").
func PutFlash(c *gin.Context, kind, message string) error {
	current := sessions.Default(c)
	flashes, _ := current.Get(flashKey).([]Flash)
	current.Set(flashKey, append(flashes, Flash{Kind: kind, Message: message}))
	return current.Save()
}

// Flashes returns and clears the stored flash messages.
func Flashes(c *gin.Context) []Flash {
	current := sessions.Default(c)
	flashes, _ := current.Get(flashKey).([]Flash)
	if len(flashes) > 0 {
		current.Delete(flashKey)
		_ = current.Save()
	}
	return flashes
}
