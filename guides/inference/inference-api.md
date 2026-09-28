# Inference API

`Client.Inference` is an `*InferenceService` for the Pinecone Inference API. It gives you access to embedding and
reranking models hosted by Pinecone. For the available models, see
[embedding models](https://docs.pinecone.io/guides/index-data/create-an-index#embedding-models) and
[reranking models](https://docs.pinecone.io/guides/search/rerank-results#reranking-models).

To have an index embed text for you when you upsert and search, use an index with
[integrated embedding](./integrated-embedding.md) instead.

The examples below assume a `Client` named `pc`. For how to create one, see
[Client configuration](../client-configuration.md).

## Create embeddings

`Embed` generates vector embeddings for text. Set `"input_type"` to `"passage"` for text you store and to `"query"` for
search queries. A request can contain up to 96 inputs, and `multilingual-e5-large` accepts up to 507 tokens per input.
Set `"truncate"` to `"END"` to truncate longer inputs instead of returning an error.

```go
docs, err := pc.Inference.Embed(ctx, &pinecone.EmbedRequest{
	Model: "multilingual-e5-large",
	TextInputs: []string{
		"Turkey is a classic meat to eat at American Thanksgiving.",
		"Many people enjoy the beautiful mosques in Turkey.",
	},
	Parameters: pinecone.EmbedParameters{
		"input_type": "passage",
		"truncate":   "END",
	},
})
if err != nil {
	log.Fatalf("Failed to embed documents: %v", err)
}
for _, embedding := range docs.Data {
	fmt.Printf("%d dimensions\n", len(embedding.DenseEmbedding.Values))
}

query, err := pc.Inference.Embed(ctx, &pinecone.EmbedRequest{
	Model:      "multilingual-e5-large",
	TextInputs: []string{"How should I prepare my turkey?"},
	Parameters: pinecone.EmbedParameters{
		"input_type": "query",
		"truncate":   "END",
	},
})
if err != nil {
	log.Fatalf("Failed to embed query: %v", err)
}
```

Each `Embedding` has a `DenseEmbedding` or a `SparseEmbedding`, depending on the model.

## Rerank documents

`Rerank` orders documents by relevance to a query. Each result's `Score` is between 0 and 1, and scores closer to 1
mean higher relevance. `bge-reranker-v2-m3` accepts up to 100 documents and 1,024 tokens per query and document pair.
Its `"truncate"` parameter defaults to `"NONE"`, so set it to `"END"` to truncate longer pairs instead of returning an
error.

```go
topN := 3
returnDocuments := true
rankFields := []string{"body"}

res, err := pc.Inference.Rerank(ctx, &pinecone.RerankRequest{
	Model: "bge-reranker-v2-m3",
	Query: "What are some good Turkey dishes for Thanksgiving?",
	Documents: []pinecone.Document{
		{"title": "Turkey Sandwiches", "body": "Turkey is a classic meat to eat at American Thanksgiving."},
		{"title": "Lemon Turkey", "body": "A lemon brined Turkey with apple sausage stuffing is a classic Thanksgiving main course."},
		{"title": "Thanksgiving", "body": "My favorite Thanksgiving dish is pumpkin pie"},
		{"title": "Protein Sources", "body": "Turkey is a great source of protein."},
	},
	TopN:            &topN,
	ReturnDocuments: &returnDocuments,
	RankFields:      &rankFields,
	Parameters:      &map[string]interface{}{"truncate": "END"},
})
if err != nil {
	log.Fatalf("Failed to rerank documents: %v", err)
}
for _, ranked := range res.Data {
	fmt.Printf("%d (%.3f): %v\n", ranked.Index, ranked.Score, (*ranked.Document)["title"])
}
```

## Hosted models

`ListModels` lists the models hosted by Pinecone. You can filter by `Type` (`"embed"` or `"rerank"`) and, for
embedding models, by `VectorType` (`"dense"` or `"sparse"`).

```go
embed := "embed"

models, err := pc.Inference.ListModels(ctx, &pinecone.ListModelsParams{Type: &embed})
if err != nil {
	log.Fatalf("Failed to list models: %v", err)
}
for _, model := range *models.Models {
	fmt.Println(model.Model)
}
```

`DescribeModel` describes a single model by name:

```go
model, err := pc.Inference.DescribeModel(ctx, "multilingual-e5-large")
if err != nil {
	log.Fatalf("Failed to describe model: %v", err)
}
fmt.Printf("%+v\n", model)
```
