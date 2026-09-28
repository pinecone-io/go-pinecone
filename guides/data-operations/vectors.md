# Working with vectors

A vector index stores records, each made of an ID, a vector, and optional metadata. You create one with
`CreateServerlessIndex`, `CreateBYOCIndex`, or `CreateIndex` with a schema of `_values` (dense), `_sparse_values`
(sparse), or both (see [Indexes](../index-management/indexes.md)). You read and write its data through the Vectors
API, which runs over gRPC.

Document indexes use the [Documents API](./documents.md) instead.

The examples below assume an `IndexConnection` named `idxConnection`. For how to create one, see
[Targeting an index](../client-configuration.md#targeting-an-index). Each operation runs against the connection's
namespace.

## Metadata and filters

Record metadata and metadata filters are `*structpb.Struct` values from
`google.golang.org/protobuf/types/known/structpb`, aliased as `pinecone.Metadata` and `pinecone.MetadataFilter`.
Metadata values must be strings, numbers, booleans, or lists of strings. Null values are rejected. Each record can have
up to 40 KB of metadata.

```go
import "google.golang.org/protobuf/types/known/structpb"

metadata, err := structpb.NewStruct(map[string]interface{}{
	"genre": "documentary",
	"year":  2019,
})
if err != nil {
	log.Fatalf("Failed to create metadata: %v", err)
}

filter, err := structpb.NewStruct(map[string]interface{}{
	"genre": map[string]interface{}{"$eq": "documentary"},
	"year":  map[string]interface{}{"$gte": 2019},
})
```

Filters accept the operators `$eq`, `$ne`, `$gt`, `$gte`, `$lt`, `$lte`, `$in`, `$nin`, `$exists`, `$and`, and `$or`.
Each `$in` or `$nin` accepts up to 10,000 values. For details, see
[Filter by metadata](https://docs.pinecone.io/guides/search/filter-by-metadata).

## Upsert vectors

`UpsertVectors` writes records into the namespace, replacing any record with the same ID. If the namespace doesn't
exist, it's created. Each record needs `Values`, `SparseValues`, or both. `UpsertVectors` sends a single request and
doesn't batch for you. A request can contain up to 1,000 records and 2 MB, and each record ID can be up to 512
characters.

```go
vectors := []*pinecone.Vector{
	{Id: "A", Values: &[]float32{0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1}, Metadata: metadata},
	{Id: "B", Values: &[]float32{0.2, 0.2, 0.2, 0.2, 0.2, 0.2, 0.2, 0.2}, Metadata: metadata},
	{Id: "C", Values: &[]float32{0.3, 0.3, 0.3, 0.3, 0.3, 0.3, 0.3, 0.3}, Metadata: metadata},
}

count, err := idxConnection.UpsertVectors(ctx, vectors)
if err != nil {
	log.Fatalf("Failed to upsert vectors: %v", err)
}
fmt.Printf("Upserted %d records\n", count)
```

In an index that stores sparse vectors, set `SparseValues` instead of `Values`:

```go
vectors := []*pinecone.Vector{
	{
		Id:       "A",
		Metadata: metadata,
		SparseValues: &pinecone.SparseValues{
			Indices: []uint32{0, 1, 2, 3, 4, 5, 6, 7},
			Values:  []float32{1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0},
		},
	},
}

count, err := idxConnection.UpsertVectors(ctx, vectors)
```

Writes are applied asynchronously, so a new record may not appear in queries right away. For more, see
[Check data freshness](https://docs.pinecone.io/guides/index-data/check-data-freshness).

## Query an index

### Query by vector values

`QueryByVectorValues` returns the `TopK` records (1 to 10,000) most similar to a query vector. The following example
queries with dense vector values and a metadata filter:

```go
res, err := idxConnection.QueryByVectorValues(ctx, &pinecone.QueryByVectorValuesRequest{
	Vector:          []float32{0.3, 0.3, 0.3, 0.3, 0.3, 0.3, 0.3, 0.3},
	TopK:            3,
	MetadataFilter:  filter,
	IncludeMetadata: true,
})
if err != nil {
	log.Fatalf("Failed to query by vector values: %v", err)
}
for _, match := range res.Matches {
	fmt.Printf("%s: %f\n", match.Vector.Id, match.Score)
}
```

Set `IncludeValues` only when you need the vector values in the response, since returning them makes results larger
and slower. Query results are limited to 4 MB.

In an index that stores sparse vectors, set `SparseValues` and leave `Vector` nil. In an index that stores both dense
and sparse vectors, set both for a hybrid query. The dense field must use the `dotproduct` metric. To weight the dense
and sparse parts, scale the query vectors before you send them. For details, see
[Hybrid search](https://docs.pinecone.io/guides/search/hybrid-search/single-index).

### Query by vector ID

`QueryByVectorId` uses a stored record's vector as the query:

```go
res, err := idxConnection.QueryByVectorId(ctx, &pinecone.QueryByVectorIdRequest{
	VectorId: "A",
	TopK:     3,
})
if err != nil {
	log.Fatalf("Failed to query by vector ID: %v", err)
}
```

## Fetch vectors

`FetchVectors` retrieves records by ID, up to 1,000 per request:

```go
res, err := idxConnection.FetchVectors(ctx, []string{"A", "B"})
if err != nil {
	log.Fatalf("Failed to fetch vectors: %v", err)
}
for id, vector := range res.Vectors {
	fmt.Printf("%s: %v\n", id, *vector.Values)
}
```

`FetchVectorsByMetadata` retrieves the records that match a non-empty metadata filter. It returns one page at a time,
with 100 records per page by default and 10,000 at most (set `Limit` to change it). When more records match, the
response has a `Pagination.Next` token. Pass it as `PaginationToken` to get the next page:

```go
res, err := idxConnection.FetchVectorsByMetadata(ctx, &pinecone.FetchVectorsByMetadataRequest{
	Filter: filter,
})
if err != nil {
	log.Fatalf("Failed to fetch vectors by metadata: %v", err)
}
if res.Pagination != nil {
	fmt.Printf("Next page token: %s\n", res.Pagination.Next)
}
```

## Update vectors

`UpdateVector` updates one record's values, sparse values, or metadata by ID. New dense values must have the same
dimension as the index. Metadata keys you don't list keep their current values, and an update never removes a key.

```go
err := idxConnection.UpdateVector(ctx, &pinecone.UpdateVectorRequest{
	Id:     "A",
	Values: []float32{0.9, 0.9, 0.9, 0.9, 0.9, 0.9, 0.9, 0.9},
})
if err != nil {
	log.Fatalf("Failed to update vector: %v", err)
}
```

`UpdateVectorsByMetadata` sets metadata on every record that matches a non-empty filter, up to 100,000 records per
request. Set `DryRun` to count the matching records without updating them. If more than 100,000 records match, repeat
the request.

```go
newMetadata, err := structpb.NewStruct(map[string]interface{}{"reviewed": true})
if err != nil {
	log.Fatalf("Failed to create metadata: %v", err)
}

res, err := idxConnection.UpdateVectorsByMetadata(ctx, &pinecone.UpdateVectorsByMetadataRequest{
	Filter:   filter,
	Metadata: newMetadata,
})
if err != nil {
	log.Fatalf("Failed to update vectors: %v", err)
}
fmt.Printf("Matched %d records\n", res.MatchedRecords)
```

## Delete vectors

You can delete records by ID (up to 1,000 per request), by metadata filter, or all records in the namespace:

```go
// Delete by ID.
err := idxConnection.DeleteVectorsById(ctx, []string{"A", "B"})
if err != nil {
	log.Fatalf("Failed to delete vectors: %v", err)
}

// Delete every record that matches a metadata filter. An empty filter is rejected.
err = idxConnection.DeleteVectorsByFilter(ctx, filter)

// Delete every record in the connection's namespace.
err = idxConnection.DeleteAllVectorsInNamespace(ctx)
```

## List vectors

`ListVectors` lists record IDs, optionally limited to IDs that start with a prefix. It's supported only on serverless
indexes. It returns up to 100 IDs per page, which is also the default. If you give records IDs such as `doc1#chunk1`,
you can use a prefix to list every chunk of one document.

```go
prefix := "doc1#"
var token *string

for {
	res, err := idxConnection.ListVectors(ctx, &pinecone.ListVectorsRequest{
		Prefix:          &prefix,
		PaginationToken: token,
	})
	if err != nil {
		log.Fatalf("Failed to list vectors: %v", err)
	}
	for _, id := range res.VectorIds {
		fmt.Println(*id)
	}
	if res.NextPaginationToken == nil {
		break
	}
	token = res.NextPaginationToken
}
```

## Describe index statistics

`DescribeIndexStats` returns the index's total record count and the record count of each namespace.

```go
stats, err := idxConnection.DescribeIndexStats(ctx)
if err != nil {
	log.Fatalf("Failed to describe index stats: %v", err)
}
fmt.Printf("Total records: %d\n", stats.TotalVectorCount)
for name, summary := range stats.Namespaces {
	fmt.Printf("%s: %d records\n", name, summary.VectorCount)
}
```

On pod-based indexes, `DescribeIndexStatsFiltered` returns per-namespace counts for the records that match a metadata
filter. Serverless indexes reject a non-empty filter.
