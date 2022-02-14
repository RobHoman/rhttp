package rhttp

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestClient(t *testing.T) {
	fnMap := map[string](func(*Client, *url.URL) *request){
		http.MethodGet: func(c *Client, u *url.URL) *request {
			return c.GET(u)
		},
		http.MethodPost: func(c *Client, u *url.URL) *request {
			return c.POST(u)
		},
		http.MethodPut: func(c *Client, u *url.URL) *request {
			return c.PUT(u)
		},
		http.MethodPatch: func(c *Client, u *url.URL) *request {
			return c.PATCH(u)
		},
		http.MethodDelete: func(c *Client, u *url.URL) *request {
			return c.DELETE(u)
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
