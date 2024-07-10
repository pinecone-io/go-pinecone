package utils

import (
	"context"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// MetadataInterceptor is a grpc.UnaryClientInterceptor that extracts the gRPC metadata
// from the outgoing RPC request context so we can assert on it
func MetadataInterceptor(t *testing.T, expectedMetadata map[string]string) grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req any,
		reply any,
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		metadata, _ := metadata.FromOutgoingContext(ctx)
		metadataString := mdToString(metadata)

		// Check that the outgoing context has the metadata we expect
		for key, value := range expectedMetadata {
			if !strings.Contains(metadataString, key) || !strings.Contains(metadataString, value) {
				t.Fatalf("MetadataInterceptor: expected to find key %s with value %s in metadata, but found %s", key, value, metadataString)
			}
		}

		return nil
	}
}

func mdToString(md metadata.MD) string {
	builder := strings.Builder{}
	for key, values := range md {
		builder.WriteString(key + ": [")
		builder.WriteString(strings.Join(values, ", "))
		builder.WriteString("]\n")
	}

	return builder.String()
}
