package rhttp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

type clientFn (func(*Client, *url.URL) *request)

type requestFn (func(*request) *request)

func encodeJSON(v interface{}) requestFn {
	return func(r *request) *request {
		return r.EncodeJSON(v)
	}
}

type requestCheckFn func(*http.Request, *testing.T)

func checkRequestMethod(expectedMethod string) requestCheckFn {
	return func(req *http.Request, t *testing.T) {
		actualMethod := req.Method
		if diff := cmp.Diff(expectedMethod, actualMethod); diff != "" {
			t.Errorf("Actual method diverges from expectation (-want +got): %s", diff)
		}
	}
}

func checkRequestBody(expectedBody string) requestCheckFn {
	return func(req *http.Request, t *testing.T) {
		actualBody, err := ioutil.ReadAll(req.Body)
		if err != nil {
			t.Errorf("Failed to read request body: %v", err)
		}
		if diff := cmp.Diff(expectedBody, string(actualBody)); diff != "" {
			t.Errorf("Actual body diverges from expectation (-want +got): %s", diff)
		}
	}
}

type doFn (func(*http.Request) (*http.Response, error))

func respondWith(
	statusCode int,
	body []byte,
	err error,
) doFn {
	return func(*http.Request) (*http.Response, error) {
		if err != nil {
			return nil, err
		}

		return &http.Response{
			StatusCode: statusCode,
			Body:       io.NopCloser(bytes.NewBuffer(body)),
		}, nil
	}
}

func respondWithNil(*http.Request) (*http.Response, error) {
	return nil, nil
}

var _ httpClientInterface = &mock{}

type mock struct {
	requestCheckFns []requestCheckFn
	t               *testing.T
	doFn            doFn
}

func (m *mock) Do(req *http.Request) (*http.Response, error) {
	for _, requestCheckFn := range m.requestCheckFns {
		requestCheckFn(req, m.t)
	}

	return m.doFn(req)
}

func textPayload(text string) []byte {
	return []byte(text)
}

type payload struct {
	Val1 int
	Val2 string
}

type unencodablePayload struct{}

var errUnencodable = fmt.Errorf("Type ('%v') cannot be encoded to JSON", reflect.TypeOf(&unencodablePayload{}))

func (u *unencodablePayload) MarshalJSON() ([]byte, error) {
	return nil, errUnencodable
}

func jsonPayload(v interface{}, t *testing.T) []byte {
	buf, err := json.Marshal(v)
	if err != nil {
		t.Errorf("Failed to marshal '%v': %v", v, err)
	}

	return buf
}

type resultCheckFn (func(*result, *testing.T))

func checkResultRawBytes(
	expectedBuf []byte,
	expectedErr error,
) resultCheckFn {
	return func(result *result, t *testing.T) {
		_, actualBuf, actualErr := result.RawBytes()

		if diff := cmp.Diff(expectedBuf, actualBuf); diff != "" {
			t.Errorf("Actual body diverges from expectation (-want +got): %s", diff)
		}

		if diff := cmp.Diff(expectedErr, actualErr, cmpopts.EquateErrors()); diff != "" {
			t.Errorf("Actual error diverges from expectation (-want +got): %s", diff)
		}
	}
}

func checkResultDecodeJSON(
	expectedV payload,
	expectedErr error,
) resultCheckFn {
	return func(result *result, t *testing.T) {
		var actualV payload
		_, actualErr := result.DecodeJSON(&actualV)

		if diff := cmp.Diff(expectedV, actualV); diff != "" {
			t.Errorf("Actual payload diverges from expectation (-want +got): %s", diff)
		}

		if diff := cmp.Diff(expectedErr, actualErr, cmpopts.EquateErrors()); diff != "" {
			t.Errorf("Actual error diverges from expectation (-want +got): %s", diff)
		}
	}
}

func checkResultDecodeJSONWithNilDest() resultCheckFn {
	return func(result *result, t *testing.T) {
		expectedErr := cmpopts.AnyError
		_, actualErr := result.DecodeJSON(nil)
		if diff := cmp.Diff(expectedErr, actualErr, cmpopts.EquateErrors()); diff != "" {
			t.Errorf("Actual error diverges from expectation (-want +got): %s", diff)
		}
	}
}

func TestBlackbox(t *testing.T) {
	var clientFns = []struct {
		method          string
		fn              clientFn
		requestCheckFns []requestCheckFn
	}{
		{
			method: http.MethodGet,
			fn: func(c *Client, u *url.URL) *request {
				return c.GET(u)
			},
			requestCheckFns: []requestCheckFn{checkRequestMethod(http.MethodGet)},
		},
		{
			method: http.MethodPost,
			fn: func(c *Client, u *url.URL) *request {
				return c.POST(u)
			},
			requestCheckFns: []requestCheckFn{checkRequestMethod(http.MethodPost)},
		},
		{
			method: http.MethodPut,
			fn: func(c *Client, u *url.URL) *request {
				return c.PUT(u)
			},
			requestCheckFns: []requestCheckFn{checkRequestMethod(http.MethodPut)},
		},
		{
			method: http.MethodPatch,
			fn: func(c *Client, u *url.URL) *request {
				return c.PATCH(u)
			},
			requestCheckFns: []requestCheckFn{checkRequestMethod(http.MethodPatch)},
		},
		{
			method: http.MethodDelete,
			fn: func(c *Client, u *url.URL) *request {
				return c.DELETE(u)
			},
			requestCheckFns: []requestCheckFn{checkRequestMethod(http.MethodDelete)},
		},
	}

	var requestFns = []struct {
		name            string
		fn              requestFn
		requestCheckFns []requestCheckFn
		resultCheckFn   resultCheckFn
	}{
		{
			name:            "encodeSaneJSON",
			fn:              encodeJSON(payload{1, "a"}),
			requestCheckFns: []requestCheckFn{checkRequestBody("{\"Val1\":1,\"Val2\":\"a\"}\n")},
		},
		{
			name:          "failToEncodeJSON",
			fn:            encodeJSON(&unencodablePayload{}),
			resultCheckFn: checkResultRawBytes(nil, errUnencodable),
		},
		// TODO(rob(h)) add possibilities here for using prepareCb. Also, figure
		// out how to allow a multitude of requestFns to be used, since mutliple
		// could be used together
	}

	var doFns = []struct {
		name          string
		fn            doFn
		resultCheckFn resultCheckFn
	}{
		{
			name: "respondWithEmptyBody",
			fn: respondWith(
				http.StatusOK,
				[]byte{},
				nil,
			),
			resultCheckFn: checkResultRawBytes([]byte{}, nil),
		},
		{
			name: "respondWithTextPayload",
			fn: respondWith(
				http.StatusOK,
				[]byte("text payload"),
				nil,
			),
			resultCheckFn: checkResultRawBytes([]byte("text payload"), nil),
		},
		{
			name: "respondWithJSONPayload",
			fn: respondWith(
				http.StatusOK,
				jsonPayload(payload{1, "a"}, t),
				nil,
			),
			resultCheckFn: checkResultDecodeJSON(
				payload{1, "a"},
				nil,
			),
		},
		{
			name: "respondWithMalformedJSONPayload",
			fn: respondWith(
				http.StatusOK,
				[]byte("{\"a\": 1"), // intentionally malformed json
				nil,
			),
			resultCheckFn: checkResultDecodeJSON(
				payload{},
				cmpopts.AnyError,
			),
		},
		{
			name:          "respondWithNil",
			fn:            respondWithNil,
			resultCheckFn: checkResultRawBytes(nil, cmpopts.AnyError),
		},
		{
			name:          "respondWithErr",
			fn:            respondWith(0, nil, fmt.Errorf("injected error")),
			resultCheckFn: checkResultRawBytes(nil, cmpopts.AnyError),
		},
		{
			name: "respondWith400",
			fn: respondWith(
				http.StatusBadRequest,
				[]byte(http.StatusText(http.StatusBadRequest)),
				nil,
			),
			resultCheckFn: checkResultRawBytes(
				nil,
				NewError(http.StatusBadRequest, http.StatusText(http.StatusBadRequest)),
			),
		},
		{
			name: "respondWith500",
			fn: respondWith(
				http.StatusInternalServerError,
				[]byte(http.StatusText(http.StatusInternalServerError)),
				nil,
			),
			resultCheckFn: checkResultRawBytes(
				nil,
				NewError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)),
			),
		},
	}

	type testCase struct {
		testName        string
		clientFn        clientFn
		requestFnChain  []requestFn
		requestCheckFns []requestCheckFn
		doFn            doFn
		resultCheckFn   resultCheckFn
	}

	var tcs []testCase
	for _, doFn := range doFns {
		for _, reqFn := range requestFns {
			for _, clientFn := range clientFns {
				var resultCheckFn resultCheckFn
				if reqFn.resultCheckFn != nil {
					resultCheckFn = reqFn.resultCheckFn
				} else {
					resultCheckFn = doFn.resultCheckFn
				}

				tc := testCase{
					testName: strings.Join([]string{
						clientFn.method, reqFn.name, doFn.name,
					}, "-"),
					clientFn:        clientFn.fn,
					requestFnChain:  []requestFn{reqFn.fn},
					requestCheckFns: append(clientFn.requestCheckFns, reqFn.requestCheckFns...),
					doFn:            doFn.fn,
					resultCheckFn:   resultCheckFn,
				}
				tcs = append(tcs, tc)
			}
		}
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			client := NewClient(&mock{
				requestCheckFns: tc.requestCheckFns,
				t:               t,
				doFn:            tc.doFn,
			})

			u, err := url.Parse(fmt.Sprintf("http://test.test.test/%s", tc.testName))
			if err != nil {
				t.Errorf("Failed to parse url: %v", err)
			}

			request := tc.clientFn(client, u)
			for _, requestFn := range tc.requestFnChain {
				request = requestFn(request)
			}
			result := request.Do()
			tc.resultCheckFn(result, t)
		})
	}
}
