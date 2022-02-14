# rhttp
Sugar for the golang net/http package

## Streamlined HTTP Client calls
An `*rhttp.Client` can wrap the golang standard library `*http.Client` - or
anything that provides the following function signature:
`Do(*http.Request) (*http.Response, error)`.

In doing so, one gains a bit of syntatic sugar to more succinctly create,
dispatch, and error handle typical golang HTTP client requests.

Here is an example of a GET request:
```
c := &rhttp.Client{}
u := &url.URL{
	Scheme: "https",
	Host:   "HOSTNAME",
	Path:   "RESOURCE-PATH",
}

var v interface{}
resp, err := c.GET(u).Do().Response()
```

Here is an example of a POST request
```
c := &rhttp.Client{}
u := &url.URL{
	Scheme: "https",
	Host:   "HOSTNAME",
	Path:   "RESOURCE-PATH",
}

var requestPayload interface{}
var responsePayload interface{}
err := c.POST(u).
	EncodeJSON(&requestPayload).
	Do().
	DecodeJSON(&responsePayload)
```

If an error is returned by any of these one-liner HTTP requests, it will be the
first error encountered. For example, an error can occurred during request
prepartion (e.g. failing to encode the request body) or an error can occur during the
request dispatch phase (e.g. failing to find a network or the host specified in
the url) or an error can occur during the response-processing phase (e.g.
failing to decode the response body).

If the entire process is successful on an HTTP-protocol level, this library
inspects the HTTP response code. 4xx- & 5xx-series HTTP responses are converted
into errors. The consumer can easily check for these errors. See [HTTP
Errors](#http-errors).

The caller may need to make other manipulations to the HTTP request prior to its
dispatch. They should use the `Prepare` method to provide a callback.

The caller must eventual determine which function they will use to terminate the
call chain. `Response()` is the most generic way to do so, and it will give the
caller access to the underlying `*http.Response` or it will return the first
error encountered in the call chain or a typed `*rhttp.Error`. `RawBytes` and
`DecodeJSON` are similar, but they also assist the caller in processing the
response bodies. Callers that want to inspect the underlying `*http.Response` in
conjunction with the convenience of terminating the chain with `RawBytes` or
`DecodeJSON` should use the `HandleResponse` method.

The zero-value for an `rhttp.Client` struct is a ready-to-use client. The
underlying `http.Client` used will be lazily initialized as the zero-value
`http.Client{}` from the `net/http` package. If the consumer would like to
configure and customize the underlying http client, they should construct their
`rhttp.Client` using the `NewClient` constructor.

## HTTP Errors
The `rhttp.Error` type holds an HTTP status code and a message, and it meets the
golang `error` interface. The `Is` and `HasStatusCode` methods allow the
consumer to easily check the error for its underlying status code and handle it
accordingly.
