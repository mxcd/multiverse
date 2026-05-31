package brain

import (
	"fmt"
	"strings"
)

// NotFoundError is returned when a query resolves to no note.
type NotFoundError struct{ Query string }

func (e *NotFoundError) Error() string { return fmt.Sprintf("no note matches %q", e.Query) }

// AmbiguousError is returned when a bare-name query resolves to multiple notes.
type AmbiguousError struct {
	Query   string
	Matches []string
}

func (e *AmbiguousError) Error() string {
	return fmt.Sprintf("%q is ambiguous (%d matches): %s — pass a full path",
		e.Query, len(e.Matches), strings.Join(e.Matches, ", "))
}
