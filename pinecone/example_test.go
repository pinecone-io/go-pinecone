package pinecone_test

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/pinecone-io/go-pinecone/v6/pinecone"
)

// ExampleNewClient_withRetries enables automatic retries with exponential backoff.
// A RetryPolicy applies to both the REST (control/data/inference) and gRPC (data
// plane) clients: 429 and transient 5xx / gRPC UNAVAILABLE responses are retried,
// other 4xx errors are not.
func ExampleNewClient_withRetries() {
	pc, err := pinecone.NewClient(pinecone.NewClientParams{
		ApiKey:      os.Getenv("PINECONE_API_KEY"),
		RetryPolicy: pinecone.DefaultRetryPolicy(), // 3 retries, 500ms base, 30s cap, 2x
	})
	if err != nil {
		log.Fatalf("failed to create Client: %v", err)
	}

	// Requests made through pc now retry rate-limited and transient failures.
	_, err = pc.ListIndexes(context.Background())
	if err != nil {
		log.Fatalf("failed to list indexes: %v", err)
	}
}

// ExampleRetryPolicy configures a custom retry policy instead of the default.
func ExampleRetryPolicy() {
	policy := &pinecone.RetryPolicy{
		MaxRetries:        5,
		BaseDelay:         time.Second,
		MaxDelay:          time.Minute,
		BackoffMultiplier: 2,
	}

	pc, err := pinecone.NewClient(pinecone.NewClientParams{
		ApiKey:      os.Getenv("PINECONE_API_KEY"),
		RetryPolicy: policy,
	})
	if err != nil {
		log.Fatalf("failed to create Client: %v", err)
	}
	_ = pc
}
