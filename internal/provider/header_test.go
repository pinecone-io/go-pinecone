package provider

import (
	"context"
	"net/http"
	"testing"
)

func TestCustomHeaderIntercept(t *testing.T) {
	expectedName := "X-Custom-Header"
	expectedValue := "Custom-Value"
	header := NewHeaderProvider(expectedName, expectedValue)

	req, err := http.NewRequest("GET", "https://example.com", nil)
	if err != nil {
		t.Fatalf("Failed to create HTTP request: %v", err)
	}

	ctx := context.Background()

	// Call the Intercept method (method being tested)
	err = header.Intercept(ctx, req)
	if err != nil {
		t.Errorf("Intercept failed: %v", err)
	}

	// Verify that the custom header is set correctly
	if req.Header.Get(expectedName) != expectedValue {
		t.Errorf("Expected header '%s' to have value '%s', got '%s'", expectedName, expectedValue,
			req.Header.Get(expectedName))
	}
}
