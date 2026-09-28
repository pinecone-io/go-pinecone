# Pinecone Go SDK &middot; ![License](https://img.shields.io/github/license/pinecone-io/go-pinecone?color=orange) [![Go Reference](https://pkg.go.dev/badge/github.com/pinecone-io/go-pinecone/v7.svg)](https://pkg.go.dev/github.com/pinecone-io/go-pinecone/v7/pinecone)

The Pinecone Go SDK is the official Go client for [Pinecone Database](https://www.pinecone.io).

## Features

- **Documents API**: Full-text search, query strings, and vector search over indexes with a document schema.
- **Vectors API**: Upsert, query, fetch, update, delete, and list records in vector indexes over gRPC.
- **Records API**: Upsert and search text in indexes with integrated embedding.
- **Index management**: Create, configure, and delete indexes, and manage backups, collections, and namespaces.
- **Inference API**: Generate embeddings and rerank results with models hosted by Pinecone.
- **Admin API**: Manage projects, organizations, API keys, service accounts, role bindings, invites, and users.
- **Retries**: Optional automatic retries with exponential backoff.

The SDK targets Pinecone API version `2026-07`, covering the control plane, data plane, Inference, and Admin APIs. For
the API reference, see [Pinecone API](https://docs.pinecone.io/reference/api/introduction).

## Documentation

- [Guides](./guides/README.md): detailed usage guides for each area of the SDK
- [API reference](https://pkg.go.dev/github.com/pinecone-io/go-pinecone/v7/pinecone) for the latest release, or
  [for `main`](https://pkg.go.dev/github.com/pinecone-io/go-pinecone/v7@main/pinecone)
- [Pinecone documentation](https://docs.pinecone.io/)

If you're upgrading from v6 or earlier, see the [v7 migration guide](./guides/migration/v7.md). v7 moves to API
version `2026-07`, which adds document indexes and removes the ability to create pod-based indexes.

## Prerequisites

- Go 1.25 or later
- A Pinecone API key. Sign up at [app.pinecone.io](https://app.pinecone.io), then set the key as the
  `PINECONE_API_KEY` environment variable.

## Installation

```shell
go get github.com/pinecone-io/go-pinecone/v7/pinecone
```

To upgrade to the latest version:

```shell
go get -u github.com/pinecone-io/go-pinecone/v7/pinecone@latest
```

## Quickstart

These quickstarts are also on [pkg.go.dev](https://pkg.go.dev/github.com/pinecone-io/go-pinecone/v7/pinecone#pkg-examples)
as package examples, which are compiled with the SDK's tests.

### Full-text search

The following program creates a document index with two full-text-search fields, upserts documents, and searches
them:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/pinecone-io/go-pinecone/v7/pinecone"
)

func main() {
	ctx := context.Background()

	// Reads the API key from the PINECONE_API_KEY environment variable.
	pc, err := pinecone.NewClient(pinecone.NewClientParams{})
	if err != nil {
		log.Fatalf("Failed to create Client: %v", err)
	}

	_, err = pc.CreateIndex(ctx, &pinecone.CreateIndexRequest{
		Name: "articles",
		Schema: pinecone.IndexSchema{
			Fields: map[string]pinecone.IndexSchemaField{
				"title": {String: &pinecone.StringField{FullTextSearch: &pinecone.FullTextSearchConfig{}}},
				"body":  {String: &pinecone.StringField{FullTextSearch: &pinecone.FullTextSearchConfig{}}},
			},
		},
	})
	if err != nil {
		log.Fatalf("Failed to create index: %v", err)
	}

	idx := waitUntilReady(ctx, pc, "articles")

	idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host, Namespace: "example-namespace"})
	if err != nil {
		log.Fatalf("Failed to create IndexConnection: %v", err)
	}
	defer idxConnection.Close()

	_, err = idxConnection.UpsertDocuments(ctx, &pinecone.UpsertDocumentsRequest{
		Documents: []pinecone.Document{
			{"_id": "doc-1", "title": "Apple orchards", "body": "Apple trees are grown in orchards across the world.", "year": 2021},
			{"_id": "doc-2", "title": "Citrus groves", "body": "Oranges and lemons grow in warm climates.", "year": 2023},
			{"_id": "doc-3", "title": "Orchard pests", "body": "Codling moths are a common pest in apple orchards.", "year": 2024},
		},
	})
	if err != nil {
		log.Fatalf("Failed to upsert documents: %v", err)
	}

	// Documents are indexed asynchronously and can take up to a minute to become searchable.
	time.Sleep(30 * time.Second)

	query := "apple orchards"
	res, err := idxConnection.SearchDocuments(ctx, &pinecone.SearchDocumentsRequest{
		TopK: 3,
		ScoreBy: []pinecone.DocumentScoringMethod{
			{Type: "text", Fields: []string{"title", "body"}, Query: &query},
		},
		IncludeFields: []string{"title"},
	})
	if err != nil {
		log.Fatalf("Failed to search documents: %v", err)
	}
	for _, match := range res.Matches {
		fmt.Printf("%s: %v\n", match.Id, match.Fields["title"])
	}
}

// waitUntilReady polls DescribeIndex until the index is ready to serve requests.
func waitUntilReady(ctx context.Context, pc *pinecone.Client, name string) *pinecone.Index {
	for {
		idx, err := pc.DescribeIndex(ctx, name)
		if err != nil {
			log.Fatalf("Failed to describe index: %v", err)
		}
		if idx.Status != nil && idx.Status.Ready {
			return idx
		}
		time.Sleep(5 * time.Second)
	}
}
```

For query strings, vector fields, filtering, and the other document operations, see
[Working with documents](./guides/data-operations/documents.md).

### Bring your own vectors

The following example creates a vector index, upserts records with vectors you've generated, and queries by
similarity. It reuses the client setup and `waitUntilReady` from the example above.

```go
metric := pinecone.IndexMetricCosine
dimension := int32(3)

_, err = pc.CreateServerlessIndex(ctx, &pinecone.CreateServerlessIndexRequest{
	Name:      "docs-example",
	Cloud:     pinecone.CloudAWS,
	Region:    "us-east-1",
	Metric:    &metric,
	Dimension: &dimension,
})
if err != nil {
	log.Fatalf("Failed to create index: %v", err)
}

idx := waitUntilReady(ctx, pc, "docs-example")

idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host, Namespace: "example-namespace"})
if err != nil {
	log.Fatalf("Failed to create IndexConnection: %v", err)
}
defer idxConnection.Close()

_, err = idxConnection.UpsertVectors(ctx, []*pinecone.Vector{
	{Id: "A", Values: &[]float32{0.1, 0.2, 0.3}},
	{Id: "B", Values: &[]float32{0.3, 0.2, 0.1}},
})
if err != nil {
	log.Fatalf("Failed to upsert vectors: %v", err)
}

res, err := idxConnection.QueryByVectorValues(ctx, &pinecone.QueryByVectorValuesRequest{
	Vector: []float32{0.1, 0.2, 0.3},
	TopK:   2,
})
if err != nil {
	log.Fatalf("Failed to query: %v", err)
}
for _, match := range res.Matches {
	fmt.Printf("%s: %f\n", match.Vector.Id, match.Score)
}
```

For metadata, filtering, and the other vector operations, see
[Working with vectors](./guides/data-operations/vectors.md).

### Integrated embedding

The following example creates an index with integrated embedding, which embeds text for you with a model hosted by
Pinecone. You upsert and search with plain text.

```go
_, err = pc.CreateIndexForModel(ctx, &pinecone.CreateIndexForModelRequest{
	Name:   "example-integrated-index",
	Cloud:  pinecone.CloudAWS,
	Region: "us-east-1",
	Embed: pinecone.CreateIndexForModelEmbed{
		Model:    "multilingual-e5-large",
		FieldMap: map[string]interface{}{"text": "chunk_text"},
	},
})
if err != nil {
	log.Fatalf("Failed to create index: %v", err)
}

idx := waitUntilReady(ctx, pc, "example-integrated-index")

idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host, Namespace: "example-namespace"})
if err != nil {
	log.Fatalf("Failed to create IndexConnection: %v", err)
}
defer idxConnection.Close()

err = idxConnection.UpsertRecords(ctx, []*pinecone.IntegratedRecord{
	{"_id": "rec1", "chunk_text": "Apples are a great source of dietary fiber.", "category": "nutrition"},
	{"_id": "rec2", "chunk_text": "The Apple I was released in 1976.", "category": "product"},
})
if err != nil {
	log.Fatalf("Failed to upsert records: %v", err)
}

res, err := idxConnection.SearchRecords(ctx, &pinecone.SearchRecordsRequest{
	Query: pinecone.SearchRecordsQuery{
		TopK:   2,
		Inputs: &map[string]interface{}{"text": "healthy snacks"},
	},
})
if err != nil {
	log.Fatalf("Failed to search records: %v", err)
}
for _, hit := range res.Result.Hits {
	fmt.Printf("%s: %f\n", hit.Id, hit.Score)
}
```

For reranking and more, see [Integrated embedding](./guides/inference/integrated-embedding.md).

## Guides

- [Client configuration](./guides/client-configuration.md): API keys, custom headers, retries, and targeting an index
- Index management
  - [Indexes](./guides/index-management/indexes.md): schemas, deployments, read capacity, and configuring indexes
  - [Backups](./guides/index-management/backups.md)
  - [Collections](./guides/index-management/collections.md)
- Data operations
  - [Working with documents](./guides/data-operations/documents.md)
  - [Working with vectors](./guides/data-operations/vectors.md)
  - [Namespaces](./guides/data-operations/namespaces.md)
  - [Bulk import](./guides/data-operations/bulk-import.md)
- Inference
  - [Inference API](./guides/inference/inference-api.md): embeddings, reranking, and hosted models
  - [Integrated embedding](./guides/inference/integrated-embedding.md)
- [Admin API](./guides/admin.md)
- [v7 migration guide](./guides/migration/v7.md)

## Support

To get help with the Pinecone Go SDK, file an issue on [GitHub](https://github.com/pinecone-io/go-pinecone/issues),
visit the [community forum](https://community.pinecone.io/), or
[open a support ticket](https://app.pinecone.io/organizations/-/settings/support/ticket).

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md) for development setup and guidelines.
