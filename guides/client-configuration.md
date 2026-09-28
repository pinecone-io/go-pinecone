# Client configuration

The examples in these guides assume a `*pinecone.Client` named `pc` and a `context.Context` named `ctx`. Examples of
data operations also assume a `*pinecone.IndexConnection` named `idxConnection`. This guide shows how to create each of
them.

## Initializing a Client

### Authenticate with an API key

Construct a `NewClientParams` and pass it to `NewClient`. If `ApiKey` is empty, the client reads it from the
`PINECONE_API_KEY` environment variable.

```go
package main

import (
	"context"
	"log"
	"os"

	"github.com/pinecone-io/go-pinecone/v7/pinecone"
)

func main() {
	ctx := context.Background()

	pc, err := pinecone.NewClient(pinecone.NewClientParams{
		ApiKey: os.Getenv("PINECONE_API_KEY"),
	})
	if err != nil {
		log.Fatalf("Failed to create Client: %v", err)
	}

	idxs, err := pc.ListIndexes(ctx)
	if err != nil {
		log.Fatalf("Failed to list indexes: %v", err)
	}
	log.Printf("Found %d indexes", len(idxs))
}
```

`NewClientParams` also accepts:

- `Headers`: extra HTTP headers sent with each REST request. Headers from the `PINECONE_ADDITIONAL_HEADERS`
  environment variable (a JSON object) are merged in, and values in `Headers` take precedence. These headers apply to
  REST requests only. To send extra metadata with gRPC data-plane calls, set `AdditionalMetadata` on
  `NewIndexConnParams`.
- `Host`: the control-plane and inference host. Defaults to the `PINECONE_CONTROLLER_HOST` environment variable, or
  `https://api.pinecone.io`.
- `RestClient`: a custom `*http.Client`.
- `SourceTag`: a string used to help Pinecone attribute API activity.
- `RetryPolicy`: see [Configuring retries](#configuring-retries).

### Authenticate with custom headers

To authenticate with custom headers (for example, for OAuth), construct a `NewClientBaseParams` and pass it to
`NewClientBase`. You must include the `"X-Project-Id"` header with your Pinecone project ID.

```go
pc, err := pinecone.NewClientBase(pinecone.NewClientBaseParams{
	Headers: map[string]string{
		"Authorization": "Bearer YOUR_OAUTH_TOKEN",
		"X-Project-Id":  "YOUR_PROJECT_ID",
	},
})
if err != nil {
	log.Fatalf("Failed to create Client: %v", err)
}
```

## Configuring retries

By default the SDK does not retry failed requests. To enable automatic retries with
exponential backoff, set a `RetryPolicy`. When set, it applies to both the REST
(control/data/inference) and gRPC (data plane) clients.

Retries cover rate-limit responses (HTTP 429 or gRPC `RESOURCE_EXHAUSTED`) and transient errors (HTTP 5xx or gRPC
`UNAVAILABLE`). Other 4xx errors are never retried.

- **REST**: 429 is always retried. 5xx and transport errors are retried only for idempotent methods (GET, PUT, and
  DELETE), to avoid repeating a write. So POST calls, such as document and record operations, imports, `Embed`, and
  `Rerank`, are retried only on 429. A `Retry-After` header sets the wait before the next attempt, capped at
  `MaxDelay`.
- **gRPC**: Data-plane calls are retried on `RESOURCE_EXHAUSTED` and `UNAVAILABLE` for every method, including writes,
  so a retried write may be applied twice. gRPC retries don't use `Retry-After`.

A 429 can also mean your project has reached a monthly usage limit, which retrying won't fix. For details, see
[Rate limits](https://docs.pinecone.io/reference/api/database-limits/rate-limits).

```go
pc, err := pinecone.NewClient(pinecone.NewClientParams{
	ApiKey:      os.Getenv("PINECONE_API_KEY"),
	RetryPolicy: pinecone.DefaultRetryPolicy(), // 3 retries, 500ms base, 30s cap, 2x backoff
})
```

To customize the policy, construct a `RetryPolicy` directly:

```go
pc, err := pinecone.NewClient(pinecone.NewClientParams{
	ApiKey: os.Getenv("PINECONE_API_KEY"),
	RetryPolicy: &pinecone.RetryPolicy{
		MaxRetries:        5,
		BaseDelay:         time.Second,
		MaxDelay:          time.Minute,
		BackoffMultiplier: 2,
	},
})
```

## Targeting an index

To perform data operations on an index, call `Client.Index`, which returns an `*IndexConnection`. You need the index's
`Host`, which you can get from the Pinecone console, `DescribeIndex`, or `ListIndexes`. In production, we recommend
looking up the host once and passing it in directly, rather than calling `DescribeIndex` every time your application
starts. For details, see [Target an index](https://docs.pinecone.io/guides/manage-data/target-an-index).

Calling `Index` creates and dials a new gRPC connection, so reuse the `IndexConnection` and call `Close` when you're done
with it.

Each `IndexConnection` targets one namespace. Set `Namespace` when calling `Index`, or call
`IndexConnection.WithNamespace` to get a copy that targets a different namespace and shares the same gRPC connection.
If no `Namespace` is provided, the
default namespace is used. The API reports it as `"__default__"`, and either `""` or `"__default__"` can be passed
wherever a namespace is expected.

```go
idx, err := pc.DescribeIndex(ctx, "docs-example")
if err != nil {
	log.Fatalf("Failed to describe index: %v", err)
}

idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host, Namespace: "example-namespace"})
if err != nil {
	log.Fatalf("Failed to create IndexConnection for Host %v: %v", idx.Host, err)
}
defer idxConnection.Close()

// Reuse the same connection for another namespace.
otherNamespace := idxConnection.WithNamespace("example-namespace-2")
```

The same `IndexConnection` is used for both [document operations](./data-operations/documents.md) and
[vector operations](./data-operations/vectors.md).
