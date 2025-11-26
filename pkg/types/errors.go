package types

import (
	"errors"
	"fmt"
)

var (
	ErrPluginNotFound      = errors.New("plugin not found")
	ErrPluginAlreadyLoaded = errors.New("plugin already loaded")
	ErrPluginNotEnabled    = errors.New("plugin not enabled")
	ErrPluginDisabled      = errors.New("plugin is disabled")
	ErrInvalidManifest     = errors.New("invalid plugin manifest")
	ErrInvalidWASM         = errors.New("invalid WASM module")
	ErrMissingExport       = errors.New("missing required WASM export")
	ErrPermissionDenied    = errors.New("permission denied")
	ErrDependencyNotMet    = errors.New("dependency not met")
	ErrEventHandlerFailed  = errors.New("event handler failed")
	ErrHostCallFailed      = errors.New("host call failed")
	ErrPluginTimeout       = errors.New("plugin operation timed out")
	ErrPluginPanic         = errors.New("plugin panicked")
	ErrMemoryLimitExceeded = errors.New("memory limit exceeded")
	ErrUnsupportedEvent    = errors.New("unsupported event type")
	ErrPlayerNotFound      = errors.New("player not found")
	ErrWorldNotFound       = errors.New("world not found")
	ErrInvalidPosition     = errors.New("invalid position")
)

type PluginError struct {
	PluginID PluginID
	Op       string
	Err      error
}

func (e *PluginError) Error() string {
	if e.PluginID == "" {
		return fmt.Sprintf("%s: %v", e.Op, e.Err)
	}
	return fmt.Sprintf("plugin %s: %s: %v", e.PluginID, e.Op, e.Err)
}

func (e *PluginError) Unwrap() error { return e.Err }

func NewPluginError(id PluginID, op string, err error) *PluginError {
	return &PluginError{PluginID: id, Op: op, Err: err}
}

type EventError struct {
	EventID   EventID
	EventType EventType
	PluginID  PluginID
	Err       error
}

func (e *EventError) Error() string {
	return fmt.Sprintf("event %s (%s) plugin %s: %v", e.EventID, e.EventType, e.PluginID, e.Err)
}

func (e *EventError) Unwrap() error { return e.Err }

func NewEventError(eventID EventID, eventType EventType, pluginID PluginID, err error) *EventError {
	return &EventError{EventID: eventID, EventType: eventType, PluginID: pluginID, Err: err}
}

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation: %s: %s", e.Field, e.Message)
}

type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return "no validation errors"
	}
	if len(e) == 1 {
		return e[0].Error()
	}
	msg := fmt.Sprintf("%d validation errors:", len(e))
	for _, err := range e {
		msg += "\n  - " + err.Error()
	}
	return msg
}

func (e *ValidationErrors) Add(field, message string) {
	*e = append(*e, ValidationError{Field: field, Message: message})
}

func (e ValidationErrors) HasErrors() bool { return len(e) > 0 }
