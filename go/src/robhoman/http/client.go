package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
)

// httpClientInterface defines the interface that this package depends upon to
// wrap it into a `Client`
type httpClientInterface interface {
	Do(*http.Request) (*http.Response, error)
}

// Client wraps an inner http client to provide easily chainable request
// preparation, execution, and response parsing operations. If `Client` is
// instantiated as a struct literal or a zero value - or if the inner client is
// nil or otherwise unspecified - a generic golang `http.Client` is lazily
// instantiated as the inner http client
type Client struct {
	ci httpClientInterface
}

// NewClient vends a `*Client` that wraps the provided `httpClientInterface`
func NewClient(c httpClientInterface) *Client {
	return &Client{
		ci: c,
	}
}

// lazyInitialize instatiates a generic golang `http.Client` to wrap if none is
// set already
func (c *Client) lazyInitialize() {
	if c.ci == nil {
		c.ci = &http.Client{}
	}
}

// GET generates an HTTP GET `*request` that the caller may customize and
// ultimately `Do()`
func (c *Client) GET(u *url.URL) *request {
	c.lazyInitialize()
	return makeRequest(c.ci, http.MethodGet, u)
}

// POST generates an HTTP POST `*request` that the caller may customize and
// ultimately `Do()`
func (c *Client) POST(u *url.URL) *request {
	c.lazyInitialize()
	return makeRequest(c.ci, http.MethodPost, u)
}

// PUT generates an HTTP PUT `*request` that the caller may customize and
// ultimately `Do()`
func (c *Client) PUT(u *url.URL) *request {
	c.lazyInitialize()
	return makeRequest(c.ci, http.MethodPut, u)
}

// PATCH generates an HTTP PATCH `*request` that the caller may customize and
// ultimately `Do()`
func (c *Client) PATCH(u *url.URL) *request {
	c.lazyInitialize()
	return makeRequest(c.ci, http.MethodPatch, u)
}

// DELETE generates an HTTP DELETE `*request` that the caller may customize and
// ultimately `Do()`
func (c *Client) DELETE(u *url.URL) *request {
	c.lazyInitialize()
	return makeRequest(c.ci, http.MethodDelete, u)
}

// request holds the details necessary to later prepare an `*http.Request` and
// also a reference to the `httpClientInterface` that will ultimately `Do()`
// it.  However, the request may fail to become prepared, in which case there
// is a non-nil `err`. The first error encountered is stored and once the err
// is non-nil, all subsequent calls on the `*request` do nothing.
type request struct {
	ci  httpClientInterface
	err error

	method  string
	u       *url.URL
	reqbody io.ReadCloser

	prepareCB func(*http.Request) error
}

// makeRequest is a convenience function for instantiating a `*request`
func makeRequest(
	ci httpClientInterface,
	method string,
	u *url.URL,
) *request {
	return &request{
		ci:     ci,
		method: method,
		u:      u,
	}
}

func (r *request) EncodeBodyJSON(reqbody interface{}) *request {
	// do nothing if there is already an error preparing this request
	if r.err != nil {
		return r
	}

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(reqbody)
	if err != nil {
		r.err = fmt.Errorf("Failed to encode body for '%s %s': %w", r.method, r.u.String(), err)
		return r
	}

	r.reqbody = io.NopCloser(&buf)

	return r
}

// Prepare defines a callback that will be invoked during the preparation
// phase, i.e. just before `Do()` is invoked on the inner `httpClientInterface`
func (r *request) Prepare(prepareCB func(*http.Request) error) *request {
	r.prepareCB = prepareCB
	return r
}

// Do the `*request` embodied within, returning a `*result` for the caller to
// consume
func (r *request) Do() *result {
	if r.err != nil {
		return &result{
			request:  r,
			response: nil,
			err:      r.err,
		}
	}

	urlstr := r.u.String()
	req, err := http.NewRequest(r.method, urlstr, r.reqbody)
	if err != nil {
		return &result{
			request:  r,
			response: nil,
			err:      fmt.Errorf("Failed to prepare request for '%s %s': %w", r.method, urlstr, err),
		}
	}

	if req == nil {
		return &result{
			request:  r,
			response: nil,
			err:      fmt.Errorf("Expected a non-nil request for '%s %s'", r.method, urlstr),
		}
	}

	if r.prepareCB != nil {
		err = r.prepareCB(req)
		if err != nil {
			return &result{
				request:  r,
				response: nil,
				err:      fmt.Errorf("Failed to execute the prepare callback for '%s %s': %w", r.method, urlstr, err),
			}
		}
	}

	resp, err := r.ci.Do(req)
	if err != nil {
		return &result{
			request:  r,
			response: nil,
			err:      fmt.Errorf("Non-protocol request error for '%s %v': %w", r.method, req.URL, err),
		}
	}

	return &result{
		request:  r,
		response: resp,
		err:      nil,
	}
}

// result contains the output of executing `Do()` on a `*request`. There may
// have been an error doing the request, or perhaps an error further upstream,
// so the `*response` is non-nil if and only if `err` is nil
type result struct {
	request  *request // back-pointer to the originating request
	response *http.Response
	err      error
}

// RawBytes reads the entire response body into a slice of bytes and returns
// it unless there was an error, in which case that error is returned instead
func (r *result) RawBytes() ([]byte, error) {
	if r.err != nil {
		return nil, r.err
	}

	if r.response == nil {
		return nil, fmt.Errorf("Expected a non-nil response for '%s %s'", r.request.method, r.request.u)
	}

	defer r.response.Body.Close()

	if err := checkStatus(r.request, r.response); err != nil {
		return nil, err
	}

	respbody, err := ioutil.ReadAll(r.response.Body)
	if err != nil {
		return nil, fmt.Errorf("Failed to read response body for '%s %s': %w", r.request.method, r.request.u, err)
	}

	return respbody, nil
}

// DecodeBodyJSON attempts to decode the response body into the provided `v`
func (r *result) DecodeJSON(v interface{}) error {
	if r.err != nil {
		return r.err
	}

	if r.response == nil {
		return fmt.Errorf("Expected a non-nil response for '%s %s'", r.request.method, r.request.u)
	}

	defer r.response.Body.Close()

	if err := checkStatus(r.request, r.response); err != nil {
		return err
	}

	if v == nil {
		return fmt.Errorf("Decode destination was nil for '%s %s'", r.request.method, r.request.u)
	}

	err := json.NewDecoder(r.response.Body).Decode(v)
	if err != nil {
		return fmt.Errorf("Failed to decode the response body for '%s %s': %w", r.request.method, r.request.u, err)
	}

	return nil
}

// checkStatus inspects for status codes greater than or equal to 400. If it
// sees such a status code, it translates the data into a typed http error, as
// defined by this package
func checkStatus(
	request *request,
	response *http.Response,
) error {
	if response.StatusCode >= http.StatusBadRequest {
		message, err := ioutil.ReadAll(response.Body)
		if err != nil {
			message = []byte(
				fmt.Sprintf(
					"Failed to read response body for '%s %s': %v",
					request.method,
					request.u,
					err,
				),
			)
		}

		return NewError(
			response.StatusCode,
			string(message),
		)
	}

	return nil
}
