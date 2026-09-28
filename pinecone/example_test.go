package pinecone_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/pinecone-io/go-pinecone/v7/pinecone"
)

// The examples in this file have no Output comment, so go test compiles them but never runs
// them. They call the live Pinecone API and are here to be shown on pkg.go.dev and kept
// compiling, not to test behavior.

// ExampleNewClient_withRetries enables automatic retries with exponential backoff.
// A RetryPolicy applies to both the REST (control/data/inference) and gRPC (data
// plane) clients: 429 and gRPC RESOURCE_EXHAUSTED/UNAVAILABLE are retried; REST 5xx
// and transport errors are retried only for idempotent methods (GET, PUT, DELETE).
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

// Example_fullTextSearch creates an index with two full-text-search fields, upserts documents,
// and ranks them by BM25 relevance to a query. It mirrors the full-text search quickstart in
// the README.
func Example_fullTextSearch() {
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

	// Index creation returns before the index is ready to serve requests.
	var idx *pinecone.Index
	for {
		idx, err = pc.DescribeIndex(ctx, "articles")
		if err != nil {
			log.Fatalf("Failed to describe index: %v", err)
		}
		if idx.Status != nil && idx.Status.Ready {
			break
		}
		time.Sleep(5 * time.Second)
	}

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

// Example_bringYourOwnVectors creates a vector index, upserts records with vectors you've
// generated, and queries by similarity. It mirrors the bring-your-own-vectors quickstart in the
// README.
func Example_bringYourOwnVectors() {
	ctx := context.Background()

	pc, err := pinecone.NewClient(pinecone.NewClientParams{})
	if err != nil {
		log.Fatalf("Failed to create Client: %v", err)
	}

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

	var idx *pinecone.Index
	for {
		idx, err = pc.DescribeIndex(ctx, "docs-example")
		if err != nil {
			log.Fatalf("Failed to describe index: %v", err)
		}
		if idx.Status != nil && idx.Status.Ready {
			break
		}
		time.Sleep(5 * time.Second)
	}

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
}

// Example_integratedEmbedding creates an index with integrated embedding, which embeds text with
// a model hosted by Pinecone, then upserts and searches with plain text. It mirrors the
// integrated embedding quickstart in the README.
func Example_integratedEmbedding() {
	ctx := context.Background()

	pc, err := pinecone.NewClient(pinecone.NewClientParams{})
	if err != nil {
		log.Fatalf("Failed to create Client: %v", err)
	}

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

	var idx *pinecone.Index
	for {
		idx, err = pc.DescribeIndex(ctx, "example-integrated-index")
		if err != nil {
			log.Fatalf("Failed to describe index: %v", err)
		}
		if idx.Status != nil && idx.Status.Ready {
			break
		}
		time.Sleep(5 * time.Second)
	}

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
}
