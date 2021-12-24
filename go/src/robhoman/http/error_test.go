package http

import (
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestError(t *testing.T) {
	errors := []*Error{
		ErrBadRequest,
		ErrUnauthorized,
		ErrForbidden,
		ErrNotFound,
		ErrConflict,
		ErrInternalServer,
		ErrServiceUnavailable,
		ErrNotImplemented,
	}

	t.Run("PackageErrorsUseGenericMessages", func(_ *testing.T) {
		for _, err := range errors {
			expected := http.StatusText(err.StatusCode)
			actual := err.Message
			if actual != expected {
				t.Errorf("Expected message '%s' but got message '%s'", expected, actual)
			}
		}
	})

	t.Run("NewError", func(_ *testing.T) {
		statusCode := http.StatusBadRequest
		message := "test message"
		expected := &Error{
			StatusCode: statusCode,
			Message:    message,
		}
		output := NewError(statusCode, message)

		if !reflect.DeepEqual(output, expected) {
			t.Errorf("Expected error '%v' but got error '%v'", expected, output)
		}
	})

	t.Run("New", func(_ *testing.T) {
		message := "test message"
		for _, err := range errors {
			expected := &Error{
				StatusCode: err.StatusCode,
				Message:    message,
			}
			output := err.New(message)

			if !reflect.DeepEqual(output, expected) {
				t.Errorf("Expected error '%v' but got error '%v'", expected, output)
			}
		}
	})

	t.Run("Newf", func(_ *testing.T) {
		format := "test format %d %s"
		args := []interface{}{1, "x"}
		for _, err := range errors {
			expected := &Error{
				StatusCode: err.StatusCode,
				Message:    fmt.Sprintf(format, args...),
			}
			output := err.Newf(format, args...)

			if !reflect.DeepEqual(output, expected) {
				t.Errorf("Expected error '%v' but got error '%v'", expected, output)
			}
		}
	})

	t.Run("Error", func(_ *testing.T) {
		for _, err := range errors {
			errStr := err.Error()
			if !strings.Contains(errStr, strconv.Itoa(err.StatusCode)) {
				t.Errorf("Expected error string '%s' to contain the status code '%d'", errStr, err.StatusCode)
			}
		}
	})

	t.Run("Is", func(_ *testing.T) {
		for i, err := range errors {
			for j, other := range errors {
				output := err.Is(other)
				if i == j && !output {
					t.Errorf("Expected error '%v' to match error '%v'", other, err)
				} else if i != j && output {
					t.Errorf("Did not expect error '%v' to match error '%v'", other, err)
				}
			}
		}
	})

	t.Run("HasStatusCode", func(_ *testing.T) {
		for i, err := range errors {
			for j, other := range errors {
				statusCode := other.StatusCode
				output := err.HasStatusCode(statusCode)
				if i == j && !output {
					t.Errorf("Expected error '%v' to have status code '%d'", err, statusCode)
				} else if i != j && output {
					t.Errorf("Did not expect error '%v' to have status code '%d'", err, statusCode)
				}
			}
		}
	})
}
