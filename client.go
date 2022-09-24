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

// GET initializes an HTTP GET `*Request` targeting the provided url. The
// caller can now chain request preparation functions.
func (c *Client) GET(u *url.URL) *Request {
	return c.NewRequest(http.MethodGet, u)
}

// HEAD initializes an HTTP HEAD `*Request` targeting the provided url. The
// caller can now chain request preparation functions.
func (c *Client) HEAD(u *url.URL) *Request {
	return c.NewRequest(http.MethodHead, u)
}

// POST initializes an HTTP POST `*Request` targeting the provided url. The
// caller can now chain request preparation functions.
func (c *Client) POST(u *url.URL) *Request {
	return c.NewRequest(http.MethodPost, u)
}

// PUT initializes an HTTP PUT `*Request` targeting the provided url. The
// caller can now chain request preparation functions.
func (c *Client) PUT(u *url.URL) *Request {
	return c.NewRequest(http.MethodPut, u)
}

// PATCH initializes an HTTP PATCH `*Request` targeting the provided url. The
// caller can now chain request preparation functions.
func (c *Client) PATCH(u *url.URL) *Request {
	return c.NewRequest(http.MethodPatch, u)
}

// DELETE initializes an HTTP DELETE `*Request` targeting the provided url. The
// caller can now chain request preparation functions.
func (c *Client) DELETE(u *url.URL) *Request {
	return c.NewRequest(http.MethodDelete, u)
}

// NewRequest initialize an HTTP `*Request` ready to use the provided request
// method and targeting the provided url. The caller can now chain request
// preparation functions.
func (c *Client) NewRequest(method string, u *url.URL) *Request {
	c.lazyInitialize()
	return makeRequest(c.ci, method, u)
}

// Request holds the details necessary to later prepare an `*http.Request` and
// also a reference to the `httpClientInterface` that will ultimately `Do()`
// it. However, the Request may fail to become prepared, in which case there
// is a non-nil `err`. The first error encountered is stored and once the err
// is non-nil, all subsequent calls on the `*Request` do nothing.
type Request struct {
	ci  httpClientInterface
	err error

	method  string
	u       *url.URL
	reqbody io.ReadCloser

	prepareCB func(*http.Request) error
}

// makeRequest is a convenience function for instantiating a `*Request`
func makeRequest(
	ci httpClientInterface,
	method string,
	u *url.URL,
) *Request {
	return &Request{
		ci:     ci,
		method: method,
		u:      u,
	}
}

// WithRequestBody allows the consumer to specify any request body
func (r *Request) WithRequestBody(reqbody io.ReadCloser) *Request {
	// do nothing if there is already an error preparing this request
	if r.err != nil {
		return r
	}

	r.reqbody = reqbody

	return r
}

// EncodeJSON encodes the provided `reqbody` struct to JSON and sets it as the
// reqbody of the HTTP request
func (r *Request) EncodeJSON(reqbody interface{}) *Request {
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
func (r *Request) Prepare(prepareCB func(*http.Request) error) *Request {
	// do nothing if there is already an error preparing this request
	if r.err != nil {
		return nil
	}

	r.prepareCB = prepareCB
	return r
}

// Do the `*Request` embodied within, returning a `*Result` for the caller to
// consume
func (r *Request) Do() *Result {
	if r.err != nil {
		return &Result{
			request:  r,
			response: nil,
			err:      r.err,
		}
	}

	urlstr := r.u.String()
	req, err := http.NewRequest(r.method, urlstr, r.reqbody)
	if err != nil {
		return &Result{
			request:  r,
			response: nil,
			err:      fmt.Errorf("failed to prepare request for '%s %s': %w", r.method, urlstr, err),
		}
	}

	if req == nil {
		return &Result{
			request:  r,
			response: nil,
			err:      fmt.Errorf("expected a non-nil request for '%s %s'", r.method, urlstr),
		}
	}

	if r.prepareCB != nil {
		err = r.prepareCB(req)
		if err != nil {
			return &Result{
				request:  r,
				response: nil,
				err:      fmt.Errorf("failed to execute the prepare callback for '%s %s': %w", r.method, urlstr, err),
			}
		}
	}

	resp, err := r.ci.Do(req)
	if err != nil {
		return &Result{
			request:  r,
			response: nil,
			err:      fmt.Errorf("non-protocol request error for '%s %v': %w", r.method, req.URL, err),
		}
	}

	return &Result{
		request:  r,
		response: resp,
		err:      nil,
	}
}

// Result contains the output of executing `Do()` on a `*Request`. There may
// have been an error doing the request, or perhaps an error further upstream,
// so the `response` ptr is non-nil if and only if `err` is nil
type Result struct {
	request  *Request // back-pointer to the originating request
	response *http.Response
	err      error
}

// Response returns the underlying HTTP response, which is useful
// to consumers who wish to handle the response body in a way that
// is not otherwise supported by this library. The caller is
// responsible for closing the request body. This method
// terminates the call chain.
func (r *Result) Response() (*http.Response, error) {
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
func (r *Result) RawBytes() (*http.Response, []byte, error) {
	if r.err != nil {
		return nil, nil, r.err
	}

	buf := &bytes.Buffer{}
	response, err := r.StreamResponse(buf)
	if err != nil {
		return response, nil, err
	}
	return response, buf.Bytes(), err
}

// StreamResponse streams the response body into the supplied destination
// writer. It also returns the underlying http response. This method therefore
// reads and closes the response body. If there was an error anywhere in the
// chain, it is returned. As long as an HTTP response was generated, it will be
// returned. This method terminates a call chain.
func (r *Result) StreamResponse(dst io.Writer) (*http.Response, error) {
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

	_, err := io.Copy(dst, r.response.Body)
	if err != nil {
		return r.response, fmt.Errorf("failed to copy response to destination for '%s %s': %w", r.request.method, r.request.u, err)
	}

	return r.response, nil
}

// DecodeJSON attempts to decode the response body into the provided `v`. This
// method therefore reads and closes the response body. If there was an error
// anywhere in the chain, it is returned. As long as an HTTP response was
// generated, it will be returned. This method terminates a call chain.
func (r *Result) DecodeJSON(v interface{}) (*http.Response, error) {
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
	request *Request,
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
