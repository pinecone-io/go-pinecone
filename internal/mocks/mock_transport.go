package mocks

import (
	"bytes"
	"io"
	"net/http"
)

type MockTransport struct {
	Req *http.Request
	Resp *http.Response
	Err error
}

func (m *MockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	m.Req = req
	return m.Resp, m.Err
}

func CreateMockClient(jsonBody string) *http.Client {
	return &http.Client {
		Transport: &MockTransport{
			Resp: &http.Response{
				StatusCode: 200,
				Body: io.NopCloser(bytes.NewReader([]byte(jsonBody))),
				Header: make(http.Header),
			},
		},
	}
}

var jsonYes = `{"message": "success"}`