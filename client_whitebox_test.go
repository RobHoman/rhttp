package rhttp

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestClient(t *testing.T) {
	fnMap := map[string](func(*Client, *url.URL) *Request){
		http.MethodGet: func(c *Client, u *url.URL) *Request {
			return c.GET(u)
		},
		http.MethodHead: func(c *Client, u *url.URL) *Request {
			return c.HEAD(u)
		},
		http.MethodPost: func(c *Client, u *url.URL) *Request {
			return c.POST(u)
		},
		http.MethodPut: func(c *Client, u *url.URL) *Request {
			return c.PUT(u)
		},
		http.MethodPatch: func(c *Client, u *url.URL) *Request {
			return c.PATCH(u)
		},
		http.MethodDelete: func(c *Client, u *url.URL) *Request {
			return c.DELETE(u)
		},
		"USER_SPECIFIED_REQUEST_METHOD": func(c *Client, u *url.URL) *Request {
			return c.NewRequest("USER_SPECIFIED_REQUEST_METHOD", u)
		},
	}

	t.Run("IntializesLazily", func(t *testing.T) {
		for _, fn := range fnMap {
			c := &Client{}
			expected := httpClientInterface(nil)
			actual := c.ci

			if diff := cmp.Diff(expected, actual); diff != "" {
				t.Errorf("Actual result diverges from expectation (-want +got): %s", diff)
			}

			fn(c, &url.URL{})

			expected = &http.Client{}
			actual = c.ci

			if diff := cmp.Diff(expected, actual); diff != "" {
				t.Errorf("Actual result diverges from expectation (-want +got): %s", diff)
			}
		}
	})
}
