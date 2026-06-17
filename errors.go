package convex

import (
	"fmt"

	"github.com/cervantesh/convex-go/internal/core"
)

const statusCodeUDFFailed = 560

// ConvexError is an application-level error thrown by a Convex function with
// ConvexError on the server.
type ConvexError = core.ConvexError

// HTTPError describes a non-Convex-function HTTP failure returned by the
// deployment endpoint.
type HTTPError struct {
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("convex: http request failed with status %d", e.StatusCode)
	}
	return fmt.Sprintf("convex: http request failed with status %d: %s", e.StatusCode, e.Body)
}

// FunctionError describes an error returned by a Convex query, mutation, or
// action.
type FunctionError = core.FunctionError
