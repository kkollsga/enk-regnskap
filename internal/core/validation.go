package core

import "strings"

// ValidationError samler feltspesifikke valideringsfeil slik at handlerne kan
// vise dem inline ved riktig felt.
type ValidationError struct {
	Fields map[string]string
}

func newValidation() *ValidationError {
	return &ValidationError{Fields: map[string]string{}}
}

func (e *ValidationError) add(field, msg string) {
	e.Fields[field] = msg
}

func (e *ValidationError) has() bool { return len(e.Fields) > 0 }

func (e *ValidationError) Error() string {
	parts := make([]string, 0, len(e.Fields))
	for f, m := range e.Fields {
		parts = append(parts, f+": "+m)
	}
	return strings.Join(parts, "; ")
}

// AsValidation forsoker å hente ut en *ValidationError fra en feil.
func AsValidation(err error) (*ValidationError, bool) {
	ve, ok := err.(*ValidationError)
	return ve, ok
}
