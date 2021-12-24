package http

import (
	"fmt"
	"net/http"
)

// Package-defined generic errors for common HTTP 4xx- & 5xx-series errors
var (
	ErrBadRequest         = newGenericError(http.StatusBadRequest)
	ErrUnauthorized       = newGenericError(http.StatusUnauthorized)
	ErrForbidden          = newGenericError(http.StatusForbidden)
	ErrNotFound           = newGenericError(http.StatusNotFound)
	ErrConflict           = newGenericError(http.StatusConflict)
	ErrInternalServer     = newGenericError(http.StatusInternalServerError)
	ErrServiceUnavailable = newGenericError(http.StatusServiceUnavailable)
	ErrNotImplemented     = newGenericError(http.StatusNotImplemented)
)

// Error represents the combination of an HTTP status code and message. It
// meets the standard golang Error interface
type Error struct {
	StatusCode int
	Message    string
}

var _ error = &Error{}

// NewError creates an *http.Error with the provided status code and message.
func NewError(status int, message string) *Error {
	return &Error{
		StatusCode: status,
		Message:    message,
	}
}

// newGenericError creates an *http.Error with a `Message` that matches generic
// status text for the given status code, as registered with IANA (see
// http.StatusText).
func newGenericError(statusCode int) *Error {
	return &Error{StatusCode: statusCode, Message: http.StatusText(statusCode)}
}

// New creates an *http.Error that with the same status code and the provided
// message
func (e *Error) New(message string) *Error {
	return NewError(e.StatusCode, message)
}

// Newf creates an *http.Error that with the same status code and the provided
// messages, using the golang convention for format strings and arguments
func (e *Error) Newf(format string, args ...interface{}) *Error {
	return NewError(e.StatusCode, fmt.Sprintf(format, args...))
}

// Error returns the underlying error message
func (e *Error) Error() string {
	return fmt.Sprintf("(%d) %s", e.StatusCode, e.Message)
}

// Is returns true if and only if the provided error has the same status code
func (e *Error) Is(err error) bool {
	inst, castSuccess := err.(*Error)
	return castSuccess && inst != nil && inst.StatusCode == e.StatusCode
}

// HasStatusCode returns true if and only if the error has the provided status code
func (e *Error) HasStatusCode(statusCode int) bool {
	return e.StatusCode == statusCode
}
