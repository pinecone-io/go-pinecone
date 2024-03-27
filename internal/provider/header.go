package provider

import (
	"context"
	"net/http"
)

type customHeader struct {
	name  string
	value string
}

func NewHeaderProvider(name string, value string) *customHeader {
	return &customHeader{name: name, value: value}
}

func (h *customHeader) Intercept(ctx context.Context, req *http.Request) error {
	req.Header.Set(h.name, h.value)
	return nil
}
