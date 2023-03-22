# rhttp
Sugar for the golang net/http package

## Streamlined HTTP Client calls
An `*rhttp.Client` can wrap the golang standard library `*http.Client` - or
anything that provides the following function signature:
`Do(*http.Request) (*http.Response, error)`.

In doing so, one gains a bit of syntatic sugar to more succinctly initialize,
prepare, execute, and response-handle typical golang HTTP client requests.

Here is an example of a GET request:
```
c := &rhttp.Client{}
u := &url.URL{
	Scheme: "https",
	Host:   "HOSTNAME",
	Path:   "RESOURCE-PATH",
}

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
resp, err := c.POST(u).
	EncodeJSON(&requestPayload).
	Do().
	DecodeJSON(&responsePayload)
```

### Request Life Cycle
The features of this package is best understood through knowledge of the
underlying sequence of phases that take place in the life cycle of a request:
0) Request Initialization Phase
1) Request Preparation Phase
2) Request Execution Phase
3) Response Handling Phase

#### 0) Request Initialization Phase
With an `*rhttp.Client{}` in-hand (see [Client
Initialization](#Client-Initialization) the caller invokes `GET`, `HEAD`,
`POST`, `PUT`, `PATCH`, `DELETE` - whichever verb corresponds to the HTTP verb
needed for the request - and provides a target url, including all requisite
information, such as scheme, host, path, query parameters, etc.

The provided url will oftentimes be finalized by this point, but one could
modify it using the [`Prepare` callback](#Prepare Callback) during the [Request
Preparation Phase](#Request-Preparation-Phase).

I opted to explicitly support the HTTP method I most commonly see used in modern
web applications - particularly those used in RESTful APIs. If you seek to use
http methods beyond these, then use the generic `NewRequest` function instead.

#### 1) Request Preparation Phase
With a request now initialized, the caller can now chain any number of request
preparation functions, according to their needs.

- `WithRequestBody` assigns a generic request body to send inside the request
- `EncodeJSON` assings a request body that shall be encoded to JSON and sent
  inside the request
- `Prepare` assigns a callback function that can mutate the request prior to its
  dispatch.

Note that `WithRequestBody` and `EncodeJSON` conflict with themselves and with
one another, since they each mutate the underlying request body. The last one in
the chain will win, because it will be the last one to set the request body.

Furthermore, note that the `Prepare` callback will also be invoked last, just
prior to request execution, regardless of its placement in the request
preparation function chain.

#### 2) Request Execution Phase
Now that the request is prepared, add `Do()` to the function chain. Proceed to
the response handling phase.


#### 3) Response Handling Phase
With the request now executed, choose exactly one method to process the
response.

- `Response()` yields only the underlying `*http.Response`. The caller must
  close the `Body`.
- `RawBytes()` yields a buffer containing the raw reponse body and closes the
  response body, in addition to returning the underlying `*http.Response`
- `StreamResponse(dst io.Writer)` copies the `*http.Response.Body` into the dst
  `io.Writer` and closes the response body, in addition to the returning
the underlying `*http.Response`
- `DecodeJSON(interface{})` decodes the response body into the provided
  parameter, in addition to returning the underlying `*http.Response`

### Client Initialization
The zero-value for an `rhttp.Client` struct is a ready-to-use client. The
underlying `http.Client` used will be lazily initialized as the zero-value
`http.Client{}` from the `net/http` package. If the consumer would like to
configure and customize the underlying http client, they should construct their
`rhttp.Client` using the `NewClient` constructor.

### Errors
TL;DR: Wherever a non-nil error is encountered in any phase of the request life
cycle, it is immediately returned. Subsequent functions & phases do not occur.

An error could occur at a variety of points during the phases of the request
life cycle. During request preparation, there could be an error while encoding
the request body. During the request execution, there could be an error finding
the network or diraling the host specified in the request url. During
response handling there could be an error decoding the response body.

Since these steps have a predefined linear sequence, and since each step is
predicated upon the success of the previous step, the first error encountered is
immediately returned and subsequent steps are canceled.
