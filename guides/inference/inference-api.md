# Inference API

`Client.Inference` is an `*InferenceService` for the Pinecone Inference API. It gives you access to embedding and
reranking models hosted by Pinecone. For the available models, see
[embedding models](https://docs.pinecone.io/guides/index-data/create-an-index#embedding-models) and
[reranking models](https://docs.pinecone.io/guides/search/rerank-results#reranking-models).

To have an index embed text for you when you upsert and search, use a document index with
[integrated embedding](../data-operations/documents.md#integrated-embedding) instead. For indexes created with
`CreateIndexForModel`, see [Legacy integrated indexes](./integrated-embedding.md).

The examples below assume a `Client` named `pc`. For how to create one, see
[Client configuration](../client-configuration.md).

## Create embeddings

`Embed` generates vector embeddings for text. Set `"input_type"` to `"passage"` for text you store and to `"query"` for
search queries. A request can contain up to 96 inputs, and `multilingual-e5-large` accepts up to 507 tokens per input.
Set `"truncate"` to `"END"` to truncate longer inputs instead of returning an error. Parameters are model-specific:
`DescribeModel` reports the keys a model accepts, their allowed values, and their defaults in `SupportedParameters`
(see [Hosted models](#hosted-models)).

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
fmt.Printf("Query: %d dimensions\n", len(query.Data[0].DenseEmbedding.Values))
```

Each `Embedding` has a `DenseEmbedding` or a `SparseEmbedding`, depending on its vector type. If the API returns a kind
of embedding the SDK doesn't recognize, `Embed` still returns the response: `VectorType` names the kind, and those
embeddings have neither field set.

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

`ListModels` lists the models hosted by Pinecone. You can filter by `Type` (`"embed"` or `"rerank"`), by `VectorType`
(`"dense"` or `"sparse"`, which lists only embedding models), and by `Modality`, a kind of input the model accepts
(`"text"`, `"image"`, or `"pdf"`). A model is listed only if it matches every filter you set. Each model's `Modalities`
lists the kinds of input it accepts; text-only models list `["text"]`.

```go
embed := "embed"

models, err := pc.Inference.ListModels(ctx, &pinecone.ListModelsParams{Type: &embed})
if err != nil {
	log.Fatalf("Failed to list models: %v", err)
}
for _, model := range *models.Models {
	fmt.Println(model.Model)
}

image := "image"
imageModels, err := pc.Inference.ListModels(ctx, &pinecone.ListModelsParams{Modality: &image})
if err != nil {
	log.Fatalf("Failed to list models: %v", err)
}
for _, model := range *imageModels.Models {
	if model.Modalities != nil {
		fmt.Println(model.Model, *model.Modalities)
	}
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

`SupportedParameters` lists the parameters a model accepts in `EmbedRequest.Parameters` or `RerankRequest.Parameters`,
with each one's allowed values or range and its default. A `SupportedParameterValue` sets `StringValue`, `IntValue`,
`FloatValue`, or `BoolValue` to match the JSON value. A JSON null sets none of them. An array, an object, or an integer
outside the `int32` range is kept as JSON text in `RawValue`; for such an integer, `FloatValue` holds the nearest
`float32` as well. A `ModelInfo` encodes back to the same JSON with `encoding/json`.

`SupportedOn` lists the APIs and index types a model works with: `"standalone_inference"` (the Inference API's
`Embed` and `Rerank`), `"integrated_index"` (a legacy integrated index created with `CreateIndexForModel`), and
`"document_index"` (an `Embed` or `SparseEmbed` field of a document index schema). It's nil when the API doesn't
report it.
