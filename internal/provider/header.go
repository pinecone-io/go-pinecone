package provider

import (
	"context"
	"net/http"
)

type CustomHeader struct {
	name  string
	value string
}

func NewHeaderProvider(name string, value string) *CustomHeader {
	return &CustomHeader{name: name, value: value}
}

func (h *CustomHeader) Intercept(ctx context.Context, req *http.Request) error {
	req.Header.Set(h.name, h.value)
	return nil
}
