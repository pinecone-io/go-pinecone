# Integrated embedding

An index with integrated embedding converts text to vectors for you, using an embedding model hosted by Pinecone. You
upsert and search with plain text, and Pinecone embeds it on the server. You can also rerank results as part of a
search.

You create an index with integrated embedding with `CreateIndexForModel`, choosing the embedding model at creation (see
[Create an index with integrated embedding](../index-management/indexes.md#create-an-index-with-integrated-embedding)).
You read and write its data through the Records API, using `UpsertRecords` and `SearchRecords`. The examples below use
an index created with `FieldMap: map[string]interface{}{"text": "chunk_text"}`, so each record's `chunk_text` field is
embedded.

The examples below assume an `IndexConnection` named `idxConnection`. For how to create one, see
[Targeting an index](../client-configuration.md#targeting-an-index).

## Upsert records

`UpsertRecords` embeds your text and upserts the records into the connection's namespace. If the namespace doesn't
exist, it's created. Each record:

- Has exactly one of an `_id` or `id` field, which identifies the record in the namespace.
- Has the field named in the index's `FieldMap`, holding the text to embed.
- Can have other fields, which are stored as metadata. You can return them in search results or use them to filter.

A request can contain at most 96 records, which is the max batch size of the hosted embedding models.

```go
records := []*pinecone.IntegratedRecord{
	{
		"_id":        "rec1",
		"chunk_text": "Apple's first product, the Apple I, was released in 1976 and was hand-built by co-founder Steve Wozniak.",
		"category":   "product",
	},
	{
		"_id":        "rec2",
		"chunk_text": "Apples are a great source of dietary fiber, which supports digestion and helps maintain a healthy gut.",
		"category":   "nutrition",
	},
	{
		"_id":        "rec3",
		"chunk_text": "Apples originated in Central Asia and have been cultivated for thousands of years, with over 7,500 varieties available today.",
		"category":   "cultivation",
	},
	{
		"_id":        "rec4",
		"chunk_text": "Rich in vitamin C and other antioxidants, apples contribute to immune health and may reduce the risk of chronic diseases.",
		"category":   "nutrition",
	},
}

err := idxConnection.UpsertRecords(ctx, records)
if err != nil {
	log.Fatalf("Failed to upsert records: %v", err)
}
```

Pinecone embeds upserted text as passages and search text as queries. You can't choose the input type per request.

## Search records

`SearchRecords` embeds the query text and returns the `TopK` records (1 to 10,000) most similar to it, with their
similarity scores. By default, each hit includes all of the record's fields. Set `Fields` to return only some of them.

```go
res, err := idxConnection.SearchRecords(ctx, &pinecone.SearchRecordsRequest{
	Query: pinecone.SearchRecordsQuery{
		TopK:   5,
		Inputs: &map[string]interface{}{"text": "Disease prevention"},
	},
	Fields: &[]string{"chunk_text", "category"},
})
if err != nil {
	log.Fatalf("Failed to search records: %v", err)
}
for _, hit := range res.Result.Hits {
	fmt.Printf("%s (%.3f): %v\n", hit.Id, hit.Score, hit.Fields["chunk_text"])
}
```

`SearchRecordsQuery` also accepts a metadata `Filter`. Instead of `Inputs`, you can search by a stored record's `Id` or
by a `Vector`.

### Rerank search results

To rerank the initial results by relevance to the query, set `Rerank` with a
[reranking model](https://docs.pinecone.io/guides/search/rerank-results#reranking-models) and the fields to rank on.
`TopN` sets how many results to return after reranking, and defaults to `TopK`. The following example finds the 5
records most similar to "Disease prevention", then reranks them and returns the 2 most relevant:

```go
topN := int32(2)

res, err := idxConnection.SearchRecords(ctx, &pinecone.SearchRecordsRequest{
	Query: pinecone.SearchRecordsQuery{
		TopK:   5,
		Inputs: &map[string]interface{}{"text": "Disease prevention"},
	},
	Rerank: &pinecone.SearchRecordsRerank{
		Model:      "bge-reranker-v2-m3",
		TopN:       &topN,
		RankFields: []string{"chunk_text"},
	},
	Fields: &[]string{"chunk_text", "category"},
})
if err != nil {
	log.Fatalf("Failed to search records: %v", err)
}
```

`RankFields` must name at least one field, and every returned record must contain each field it names. How many fields
you can rank on depends on the model. When you search by `Vector` or `Id`, also set `Rerank.Query`, since there's no
query text to rerank against.

## Limitations

Indexes created with `CreateIndexForModel` don't support updating records with text or
[bulk import](../data-operations/bulk-import.md).
