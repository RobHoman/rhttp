package rhttp

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

// GET initializes an HTTP GET `*request` targeting the provided url. The
// caller can now chain request preparation functions.
func (c *Client) GET(u *url.URL) *request {
	return c.NewRequest(http.MethodGet, u)
}

// HEAD initializes an HTTP HEAD `*request` targeting the provided url. The
// caller can now chain request preparation functions.
func (c *Client) HEAD(u *url.URL) *request {
	return c.NewRequest(http.MethodHead, u)
}

// POST initializes an HTTP POST `*request` targeting the provided url. The
// caller can now chain request preparation functions.
func (c *Client) POST(u *url.URL) *request {
	return c.NewRequest(http.MethodPost, u)
}

// PUT initializes an HTTP PUT `*request` targeting the provided url. The
// caller can now chain request preparation functions.
func (c *Client) PUT(u *url.URL) *request {
	return c.NewRequest(http.MethodPut, u)
}

// PATCH initializes an HTTP PATCH `*request` targeting the provided url. The
// caller can now chain request preparation functions.
func (c *Client) PATCH(u *url.URL) *request {
	return c.NewRequest(http.MethodPatch, u)
}

// DELETE initializes an HTTP DELETE `*request` targeting the provided url. The
// caller can now chain request preparation functions.
func (c *Client) DELETE(u *url.URL) *request {
	return c.NewRequest(http.MethodDelete, u)
}

// NewRequest initialize an HTTP `*request` ready to use the provided request
// method and targeting the provided url. The caller can now chain request
// preparation functions.
func (c *Client) NewRequest(method string, u *url.URL) *request {
	c.lazyInitialize()
	return makeRequest(c.ci, method, u)
}

// request holds the details necessary to later prepare an `*http.Request` and
// also a reference to the `httpClientInterface` that will ultimately `Do()`
// it. However, the request may fail to become prepared, in which case there
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

// WithRequestBody allows the consumer to specify any request body
func (r *request) WithRequestBody(reqbody io.ReadCloser) *request {
	// do nothing if there is already an error preparing this request
	if r.err != nil {
		return r
	}

	r.reqbody = reqbody

	return r
}

// EncodeJSON encodes the provided `reqbody` struct to JSON and sets it as the
// reqbody of the HTTP request
func (r *request) EncodeJSON(reqbody interface{}) *request {
	// do nothing if there is already an error preparing this request
	if r.err != nil {
		return r
	}

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(reqbody)
	if err != nil {
		r.err = fmt.Errorf("failed to encode body for '%s %s': %w", r.method, r.u.String(), err)
		return r
	}

	r.reqbody = io.NopCloser(&buf)

	return r
}

// Prepare defines a callback that will be invoked during the preparation
// phase, i.e. just before `Do()` is invoked on the inner
// `httpClientInterface`. It is recommended that the consumer does not
// manipulate the request body during this callback.
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
			err:      fmt.Errorf("failed to prepare request for '%s %s': %w", r.method, urlstr, err),
		}
	}

	if req == nil {
		return &result{
			request:  r,
			response: nil,
			err:      fmt.Errorf("expected a non-nil request for '%s %s'", r.method, urlstr),
		}
	}

	if r.prepareCB != nil {
		err = r.prepareCB(req)
		if err != nil {
			return &result{
				request:  r,
				response: nil,
				err:      fmt.Errorf("failed to execute the prepare callback for '%s %s': %w", r.method, urlstr, err),
			}
		}
	}

	resp, err := r.ci.Do(req)
	if err != nil {
		return &result{
			request:  r,
			response: nil,
			err:      fmt.Errorf("non-protocol request error for '%s %v': %w", r.method, req.URL, err),
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

// Response returns the underlying HTTP response, which is useful
// to consumers who wish to handle the response body in a way that
// is not otherwise supported by this library. The caller is
// responsible for closing the request body. This method
// terminates the call chain.
func (r *result) Response() (*http.Response, error) {
	if r.err != nil {
		return nil, r.err
	}

	if r.response == nil {
		return nil, fmt.Errorf("expected a non-nil response for '%s %s'", r.request.method, r.request.u)
	}

	if err := checkStatus(r.request, r.response); err != nil {
		return nil, err
	}

	return r.response, nil
}

// RawBytes reads the entire response body into a slice of bytes
// and returns it, along with the underlying response. This method therefore
// reads and closes the response body. If there was an error anywhere in the
// chain, it is returned. As long as an HTTP response was generated, it will be
// returned. This method terminates a call chain.
func (r *result) RawBytes() (*http.Response, []byte, error) {
	if r.err != nil {
		return nil, nil, r.err
	}

	if r.response == nil {
		return nil, nil, fmt.Errorf("expected a non-nil response for '%s %s'", r.request.method, r.request.u)
	}

	defer r.response.Body.Close()

	if err := checkStatus(r.request, r.response); err != nil {
		return r.response, nil, err
	}

	respbody, err := ioutil.ReadAll(r.response.Body)
	if err != nil {
		return r.response, nil, fmt.Errorf("failed to read response body for '%s %s': %w", r.request.method, r.request.u, err)
	}

	return r.response, respbody, nil
}

// DecodeJSON attempts to decode the response body into the provided `v`. This
// method therefore reads and closes the response body. If there was an error
// anywhere in the chain, it is returned. As long as an HTTP response was
// generated, it will be returned. This method terminates a call chain.
func (r *result) DecodeJSON(v interface{}) (*http.Response, error) {
	if r.err != nil {
		return nil, r.err
	}

	if r.response == nil {
		return nil, fmt.Errorf("expected a non-nil response for '%s %s'", r.request.method, r.request.u)
	}

	defer r.response.Body.Close()

	if err := checkStatus(r.request, r.response); err != nil {
		return r.response, err
	}

	if v == nil {
		return r.response, fmt.Errorf("decode destination was nil for '%s %s'", r.request.method, r.request.u)
	}

	err := json.NewDecoder(r.response.Body).Decode(v)
	if err != nil {
		return r.response, fmt.Errorf("failed to decode the response body for '%s %s': %w", r.request.method, r.request.u, err)
	}

	return r.response, nil
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
