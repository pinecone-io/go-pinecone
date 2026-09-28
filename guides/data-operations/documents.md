# Working with documents

A document index is an index with a document schema, created with `CreateIndex` (see
[Create a document index](../index-management/indexes.md#create-a-document-index)). You read and write
its data through the Documents API, using these methods on `IndexConnection`:

| Method | Description |
|--------|-------------|
| `UpsertDocuments` | Write documents, replacing any with the same `_id` |
| `SearchDocuments` | Rank documents with full-text, query-string, dense vector, or sparse vector scoring |
| `FetchDocuments` | Retrieve documents by ID or by filter |
| `UpdateDocuments` | Patch fields on documents by ID, or on every document that matches a filter |
| `DeleteDocuments` | Delete documents by ID, by filter, or all at once |
| `ListDocuments` | List document IDs, optionally by prefix |

Vector indexes use the [Vectors API](./vectors.md) instead. The two APIs don't cross over, so writing vectors to a
document index returns an error.

The examples below assume an `IndexConnection` named `idxConnection` that targets a document index. For how to create
one, see [Targeting an index](../client-configuration.md#targeting-an-index). Each operation runs against the
connection's namespace.

## Documents

A `Document` is a `map[string]interface{}`. Every document needs an `"_id"` field and at least one field declared in
the index schema. Any field that isn't in the schema is stored on the document as metadata and indexed automatically
for filtering. Metadata values must be strings, numbers, booleans, or lists of strings.

The examples on this page use an index with two full-text-search fields:

```go
schema := pinecone.IndexSchema{
	Fields: map[string]pinecone.IndexSchemaField{
		"title": {String: &pinecone.StringField{FullTextSearch: &pinecone.FullTextSearchConfig{}}},
		"body":  {String: &pinecone.StringField{FullTextSearch: &pinecone.FullTextSearchConfig{}}},
	},
}
```

## Upsert documents

`UpsertDocuments` writes documents into the namespace. Upserting a document with an existing `_id` replaces the whole
document. To change only some fields, use [`UpdateDocuments`](#update-documents).

```go
res, err := idxConnection.UpsertDocuments(ctx, &pinecone.UpsertDocumentsRequest{
	Documents: []pinecone.Document{
		{"_id": "doc-1", "title": "Apple orchards", "body": "Apple trees are grown in orchards across the world.", "year": 2021},
		{"_id": "doc-2", "title": "Citrus groves", "body": "Oranges and lemons grow in warm climates.", "year": 2023},
		{"_id": "doc-3", "title": "Orchard pests", "body": "Codling moths are a common pest in apple orchards.", "year": 2024},
	},
})
if err != nil {
	log.Fatalf("Failed to upsert documents: %v", err)
}
fmt.Printf("Upserted %d documents\n", res.UpsertedCount)
```

These limits apply to upserts:

| Limit | Value |
|-------|-------|
| Documents per request | 1,000 |
| Request size | 2 MB |
| Document size | 2 MB |
| `_id` length | 512 characters |
| Size per full-text-search field value | 100 KB |
| Tokens per full-text-search field value | 10,000 |
| Filterable metadata per document | 40 KB (full-text-search fields don't count) |

Documents are indexed asynchronously, so newly upserted documents may not be searchable right away. It can take up to
a minute, or longer with many indexed fields. If you need to read your writes, for example in a test, poll with a
bounded timeout until the expected results appear. To confirm that an index holds the documents you expect, see
[Check data freshness](https://docs.pinecone.io/guides/index-data/check-data-freshness).

## Search documents

`SearchDocuments` returns the `TopK` documents (1 to 10,000) ranked by the scoring methods in `ScoreBy`. Each
`DocumentScoringMethod` has a `Type`:

| Type | Scores by | Requires |
|------|-----------|----------|
| `"text"` | BM25 over full-text-search fields | `Fields` (one or more) and `Query` |
| `"query_string"` | A Lucene query string | `Query`, with `Fields` left empty |
| `"dense_vector"` | Dense vector similarity | `Fields` (exactly one dense vector field) and `Values` |
| `"sparse_vector"` | Sparse vector similarity | `Fields` (exactly one sparse vector field) and `SparseValues` |

You can combine several methods only when every one is `"text"` or `"query_string"`. A vector method must be the only
entry in `ScoreBy`.

By default, a match carries only its `Id` and `Score`. List the fields to return in `IncludeFields`, or pass
`[]string{"*"}` to return every field.

### Full-text search

A `"text"` method ranks documents by BM25 relevance to the query. The query's terms are combined with OR, so a
document that contains any of them can match.

```go
query := "apple orchards"

res, err := idxConnection.SearchDocuments(ctx, &pinecone.SearchDocumentsRequest{
	TopK: 5,
	ScoreBy: []pinecone.DocumentScoringMethod{
		{Type: "text", Fields: []string{"title", "body"}, Query: &query},
	},
	IncludeFields: []string{"title"},
})
if err != nil {
	log.Fatalf("Failed to search documents: %v", err)
}
for _, match := range res.Matches {
	if match.Score != nil {
		fmt.Printf("%s (%.3f): %v\n", match.Id, *match.Score, match.Fields["title"])
	}
}
```

`Score` is nil when the score isn't a finite number.

### Query strings

A `"query_string"` method takes a Lucene query string, which supports operators such as `AND`, `OR`, quoted phrases,
and boosting with `^`. Use field qualifiers (`field:(clause)`) to target a field, or leave them out to search every
full-text-search field. Unqualified terms are combined with OR.

```go
query := `title:(orchard) AND body:("apple orchards" OR pest)`

res, err := idxConnection.SearchDocuments(ctx, &pinecone.SearchDocumentsRequest{
	TopK: 5,
	ScoreBy: []pinecone.DocumentScoringMethod{
		{Type: "query_string", Query: &query},
	},
	IncludeFields: []string{"*"},
})
```

For the full syntax, see [Query syntax](https://docs.pinecone.io/guides/search/full-text-search/query-syntax).

### Vector search

To search by vector similarity, the index needs a dense or sparse vector field, and each document stores its vector
under that field's name:

```go
// The index schema declares "embedding" as a DenseVectorField with Dimension 3.
_, err := idxConnection.UpsertDocuments(ctx, &pinecone.UpsertDocumentsRequest{
	Documents: []pinecone.Document{
		{"_id": "doc-1", "embedding": []float32{0.1, 0.2, 0.3}, "genre": "drama"},
	},
})

queryVector := []float32{0.1, 0.2, 0.3}

res, err := idxConnection.SearchDocuments(ctx, &pinecone.SearchDocumentsRequest{
	TopK: 10,
	ScoreBy: []pinecone.DocumentScoringMethod{
		{Type: "dense_vector", Fields: []string{"embedding"}, Values: &queryVector},
	},
})
```

For a sparse vector field, set `Type` to `"sparse_vector"` and pass `SparseValues`:

```go
res, err := idxConnection.SearchDocuments(ctx, &pinecone.SearchDocumentsRequest{
	TopK: 10,
	ScoreBy: []pinecone.DocumentScoringMethod{
		{
			Type:   "sparse_vector",
			Fields: []string{"sparse_terms"},
			SparseValues: &pinecone.SparseValues{
				Indices: []uint32{10, 45, 16},
				Values:  []float32{0.5, 0.5, 0.2},
			},
		},
	},
})
```

### Filter search results

`Filter` narrows the documents searched before they're scored. It accepts the metadata operators `$eq`, `$ne`, `$gt`,
`$gte`, `$lt`, `$lte`, `$in`, `$nin`, `$exists`, `$and`, `$or`, and `$not`. Top-level keys are combined with AND, and
each `$in` or `$nin` accepts up to 10,000 values.

```go
query := "orchard"

res, err := idxConnection.SearchDocuments(ctx, &pinecone.SearchDocumentsRequest{
	TopK: 5,
	ScoreBy: []pinecone.DocumentScoringMethod{
		{Type: "text", Fields: []string{"body"}, Query: &query},
	},
	Filter: map[string]interface{}{
		"year": map[string]interface{}{"$gte": 2022},
	},
})
```

Filters on search and fetch requests also accept three text-match operators on full-text-search fields:

| Operator | Matches documents where the field |
|----------|-----------------------------------|
| `$match_phrase` | Contains the exact phrase |
| `$match_all` | Contains every token, in any order |
| `$match_any` | Contains at least one token |

Text-match operators reuse the field's tokenizer and stemmer, and treat their value as literal text. They compose with
metadata operators under `$and`, `$or`, and `$not`. A common pattern is to rank by a dense vector and require a phrase
with a filter:

```go
res, err := idxConnection.SearchDocuments(ctx, &pinecone.SearchDocumentsRequest{
	TopK: 10,
	ScoreBy: []pinecone.DocumentScoringMethod{
		{Type: "dense_vector", Fields: []string{"embedding"}, Values: &queryVector},
	},
	Filter: map[string]interface{}{
		"$and": []interface{}{
			map[string]interface{}{"body": map[string]interface{}{"$match_phrase": "apple orchards"}},
			map[string]interface{}{"body": map[string]interface{}{"$not": map[string]interface{}{"$match_any": "citrus lemon"}}},
		},
	},
})
```

Filtered updates and deletes don't accept text-match operators. To update or delete documents by their text, search or
fetch the matching IDs first.

For more on filters, see [Filter by metadata](https://docs.pinecone.io/guides/search/filter-by-metadata).

## Fetch documents

`FetchDocuments` retrieves documents by ID (up to 1,000 per request) or by filter. Set exactly one of `Ids` or `Filter`.
`IncludeFields` limits the fields returned. When it's empty, every field is returned.

```go
res, err := idxConnection.FetchDocuments(ctx, &pinecone.FetchDocumentsRequest{
	Ids: []string{"doc-1", "doc-2"},
})
if err != nil {
	log.Fatalf("Failed to fetch documents: %v", err)
}
for id, doc := range res.Documents {
	fmt.Printf("%s: %v\n", id, doc["title"])
}
```

A fetch by filter returns one page at a time, with 100 documents per page by default and 10,000 at most (set `Limit`
to change it). When more documents match, the response has a `Pagination.Next` token. Pass it as `PaginationToken` to
get the next page:

```go
filter := map[string]interface{}{"year": map[string]interface{}{"$gte": 2022}}
var token *string

for {
	res, err := idxConnection.FetchDocuments(ctx, &pinecone.FetchDocumentsRequest{
		Filter:          filter,
		PaginationToken: token,
	})
	if err != nil {
		log.Fatalf("Failed to fetch documents: %v", err)
	}
	for id := range res.Documents {
		fmt.Println(id)
	}
	if res.Pagination == nil || res.Pagination.Next == "" {
		break
	}
	token = &res.Pagination.Next
}
```

## Update documents

`UpdateDocuments` patches documents in one of two ways, which you can't combine in one request.

To update documents by ID, pass up to 1,000 partial documents. Each one names a document by `_id`. Its other entries
set those fields, and an optional `"_remove_fields"` entry lists fields to remove. Fields you don't mention are left
unchanged, and an update to an ID that doesn't exist has no effect. Null values are rejected, so list a field in
`"_remove_fields"` to delete it.

```go
_, err := idxConnection.UpdateDocuments(ctx, &pinecone.UpdateDocumentsRequest{
	Documents: []pinecone.Document{
		{"_id": "doc-1", "year": 2022, "_remove_fields": []string{"draft"}},
	},
})
if err != nil {
	log.Fatalf("Failed to update documents: %v", err)
}
```

To update every document that matches a filter, pass `Filter` with `SetFields`, `RemoveFields`, or both. A filtered
update can change metadata fields only. It rejects fields declared in the schema, so patch those by ID. The response's
`MatchedRecords` reports how many documents matched the filter when the update was accepted:

```go
res, err := idxConnection.UpdateDocuments(ctx, &pinecone.UpdateDocumentsRequest{
	Filter:    map[string]interface{}{"year": map[string]interface{}{"$lt": 2023}},
	SetFields: map[string]interface{}{"archived": true},
})
if err != nil {
	log.Fatalf("Failed to update documents: %v", err)
}
if res.MatchedRecords != nil {
	fmt.Printf("Matched %d documents\n", *res.MatchedRecords)
}
```

Updates are applied asynchronously.

## Delete documents

`DeleteDocuments` deletes documents by ID (up to 1,000 per request), by filter, or all documents in the namespace. Set
exactly one of `Ids`, `Filter`, or `DeleteAll`. Deletes are applied asynchronously.

```go
// Delete by ID.
_, err := idxConnection.DeleteDocuments(ctx, &pinecone.DeleteDocumentsRequest{
	Ids: []string{"doc-1"},
})
if err != nil {
	log.Fatalf("Failed to delete documents: %v", err)
}

// Delete every document that matches a filter.
res, err := idxConnection.DeleteDocuments(ctx, &pinecone.DeleteDocumentsRequest{
	Filter: map[string]interface{}{"archived": map[string]interface{}{"$eq": true}},
})
if err == nil && res.MatchedRecords != nil {
	fmt.Printf("Matched %d documents\n", *res.MatchedRecords)
}

// Delete every document in the namespace.
_, err = idxConnection.DeleteDocuments(ctx, &pinecone.DeleteDocumentsRequest{
	DeleteAll: true,
})
```

Only a filtered delete reports `MatchedRecords`. An empty `Filter` is rejected rather than matching every document, so
use `DeleteAll` to clear the namespace.

## List documents

`ListDocuments` lists document IDs in the namespace in sorted order, optionally limited to IDs that start with
`Prefix`. Results are paginated the same way as a fetch by filter:

```go
prefix := "doc-"
var token *string

for {
	res, err := idxConnection.ListDocuments(ctx, &pinecone.ListDocumentsRequest{
		Prefix:          &prefix,
		PaginationToken: token,
	})
	if err != nil {
		log.Fatalf("Failed to list documents: %v", err)
	}
	for _, doc := range res.Documents {
		fmt.Println(doc.Id)
	}
	if res.Pagination == nil || res.Pagination.Next == "" {
		break
	}
	token = &res.Pagination.Next
}
```
