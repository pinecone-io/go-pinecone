# Indexes

An index is defined by its **schema**, which lists the fields it declares, and its **deployment**, which says where it
runs. The deployment is serverless, Bring Your Own Cloud (BYOC), or pod-based (for existing indexes only). The schema
determines which data-plane API the index uses:

- A **document index** declares named fields, such as full-text-search fields and dense or sparse vector fields. You
  read and write it through the [Documents API](../data-operations/documents.md). Create one with `CreateIndex`.
- A **vector index** stores vectors under the reserved field names `_values` (dense), `_sparse_values` (sparse), or
  both. You read and write it through the [Vectors API](../data-operations/vectors.md). Create one with
  `CreateServerlessIndex`, `CreateBYOCIndex`, or `CreateIndex` with a schema made only of the reserved fields.
- An index created with `CreateIndexForModel` has **integrated embedding**, which embeds text for you. You read and
  write it through the [Records API](../inference/integrated-embedding.md).

The examples below assume a `Client` named `pc`. For how to create one, see
[Client configuration](../client-configuration.md).

## Create a document index

`CreateIndex` creates an index from an explicit `IndexSchema`. You can declare these field types:

- `DenseVectorField`: Fixed-dimension dense vectors, with a `Dimension` and `Metric`. At most one per index.
- `SparseVectorField`: Sparse vectors, which take no dimension or metric. At most one per index.
- `StringField` with `FullTextSearch` set: Text indexed for full-text search. At most 100 per index.

You don't declare metadata fields. They're indexed automatically when you upsert data. Field names can be up to 64
bytes and can't start with `$` or `_`.

The following example creates a document index with two full-text-search fields:

```go
idx, err := pc.CreateIndex(ctx, &pinecone.CreateIndexRequest{
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
fmt.Printf("Created index %s\n", idx.Name)
```

A schema can combine text and vector fields. This index stores a dense vector, a sparse vector, and a
full-text-search field on each document:

```go
idx, err := pc.CreateIndex(ctx, &pinecone.CreateIndexRequest{
	Name: "example-articles",
	Schema: pinecone.IndexSchema{
		Fields: map[string]pinecone.IndexSchemaField{
			"embedding":    {DenseVector: &pinecone.DenseVectorField{Dimension: 1536, Metric: pinecone.IndexMetricDotproduct}},
			"sparse_terms": {SparseVector: &pinecone.SparseVectorField{}},
			"body":         {String: &pinecone.StringField{FullTextSearch: &pinecone.FullTextSearchConfig{}}},
		},
	},
})
```

Each search of a document index ranks by one kind of scoring, so a single search can't combine the dense and sparse
fields. To combine them, run a search for each and merge the results, for example with
[reciprocal rank fusion](https://docs.pinecone.io/guides/search/reciprocal-rank-fusion).

Document indexes have these limitations:

- They run only on serverless deployments.
- You can't change the schema after the index is created.
- They don't support [backups](./backups.md).
- You can't switch them from dedicated read capacity back to on-demand.

### Configure text processing

`FullTextSearchConfig` controls how a field's text is analyzed. You can't change these settings after the index is
created.

- `Language`: The language for text analysis. Defaults to `"en"`.
- `Stemming`: Reduces words to their root form, so "moths" matches "moth". Off by default.
- `StopWords`: Filters out common words such as "the". Off by default, and requires `Stemming`. Stop word lists aren't
  available for Arabic, Greek, Romanian, Tamil, or Turkish.
- `Ngram`: Splits the field into character n-grams for substring or prefix (autocomplete) matching. `MaxGram` can be at
  most 10. You can't combine `Ngram` with `Stemming` or `StopWords`.

```go
stemming := true
prefixOnly := true

schema := pinecone.IndexSchema{
	Fields: map[string]pinecone.IndexSchemaField{
		"body": {String: &pinecone.StringField{
			FullTextSearch: &pinecone.FullTextSearchConfig{Stemming: &stemming},
		}},
		"product_name": {String: &pinecone.StringField{
			FullTextSearch: &pinecone.FullTextSearchConfig{
				Ngram: &pinecone.NgramConfig{MinGram: 2, MaxGram: 5, PrefixOnly: &prefixOnly},
			},
		}},
	},
}
```

For details, see [Text processing](https://docs.pinecone.io/guides/search/full-text-search/text-processing).

### Choose a cloud and region

`Deployment` defaults to a serverless index on AWS in `us-east-1`. To choose another cloud or region, set a
`ManagedDeployment`:

```go
idx, err := pc.CreateIndex(ctx, &pinecone.CreateIndexRequest{
	Name:   "articles-gcp",
	Schema: schema,
	Deployment: &pinecone.IndexDeployment{
		Managed: &pinecone.ManagedDeployment{Cloud: pinecone.CloudGCP, Region: "us-central1"},
	},
})
```

For the available regions, see [Create an index](https://docs.pinecone.io/guides/index-data/create-an-index).

`CreateIndexRequest` also accepts `ReadCapacity` (see [Dedicated read capacity](#dedicated-read-capacity)), `CmekId`,
`DeletionProtection`, and `Tags`. `Name` is optional. If it's empty, Pinecone generates one. Set a name if you need to
retry the request safely, because a retry with the same name fails with a conflict instead of creating a second index.

## Create a vector index

`CreateServerlessIndex` creates a vector index from a dimension and metric. The following example creates a serverless
index that stores dense vectors, in the `us-east-1` region of AWS:

```go
metric := pinecone.IndexMetricCosine
dimension := int32(1536)

idx, err := pc.CreateServerlessIndex(ctx, &pinecone.CreateServerlessIndexRequest{
	Name:      "docs-example",
	Cloud:     pinecone.CloudAWS,
	Region:    "us-east-1",
	Metric:    &metric,
	Dimension: &dimension,
	Tags:      &pinecone.IndexTags{"environment": "development"},
})
if err != nil {
	log.Fatalf("Failed to create serverless index: %v", err)
}
```

You can also create an index that stores only sparse vectors, for
[sparse-vector search](https://docs.pinecone.io/guides/search/lexical-search) with a learned sparse model such as
[pinecone-sparse-english-v0](https://docs.pinecone.io/models/pinecone-sparse-english-v0). It must use the `dotproduct`
metric, which is the default when `Metric` isn't set, and takes no dimension:

```go
vectorType := "sparse"

idx, err := pc.CreateServerlessIndex(ctx, &pinecone.CreateServerlessIndexRequest{
	Name:       "example-sparse-vectors",
	Cloud:      pinecone.CloudAWS,
	Region:     "us-east-1",
	VectorType: &vectorType,
})
```

A vector index can store both dense and sparse vectors. To run hybrid queries that combine them, the dense vectors
must use the `dotproduct` metric. An index created with `CreateServerlessIndex` or `CreateBYOCIndex` and
`IndexMetricDotproduct` supports this. So does an index created with `CreateIndex` and both reserved fields, which is
the same index:

```go
idx, err := pc.CreateIndex(ctx, &pinecone.CreateIndexRequest{
	Name: "example-hybrid",
	Schema: pinecone.IndexSchema{
		Fields: map[string]pinecone.IndexSchemaField{
			"_values":        {DenseVector: &pinecone.DenseVectorField{Dimension: 1536, Metric: pinecone.IndexMetricDotproduct}},
			"_sparse_values": {SparseVector: &pinecone.SparseVectorField{}},
		},
	},
})
```

### Create a BYOC index

To create a vector index in a BYOC environment, use `CreateBYOCIndex`. Some BYOC environments support only dedicated
read capacity and reject on-demand. In those environments, set `ReadCapacity` (see
[Dedicated read capacity](#dedicated-read-capacity)), as in this example:

```go
metric := pinecone.IndexMetricCosine
dimension := int32(1536)
nodeType := "b1"
replicas := int32(1)
shards := int32(1)

idx, err := pc.CreateBYOCIndex(ctx, &pinecone.CreateBYOCIndexRequest{
	Name:        "example-byoc-index",
	Environment: "YOUR_BYOC_ENVIRONMENT",
	Metric:      &metric,
	Dimension:   &dimension,
	ReadCapacity: &pinecone.ReadCapacityParams{
		Dedicated: &pinecone.ReadCapacityDedicatedConfig{
			NodeType: &nodeType,
			Scaling: &pinecone.ReadCapacityScaling{
				Manual: &pinecone.ReadCapacityManualScaling{Replicas: &replicas, Shards: &shards},
			},
		},
	},
})
```

For more on BYOC, see [Bring your own cloud](https://docs.pinecone.io/guides/production/bring-your-own-cloud).

## Create an index with integrated embedding

An index with integrated embedding uses an embedding model hosted by Pinecone to convert text to vectors for you. You
choose the model when you create the index, and you can't change it afterwards. You can update the model's read and
write parameters with `ConfigureIndex` (see [Configure an index](#configure-an-index)). Create one with
`CreateIndexForModel`:

```go
idx, err := pc.CreateIndexForModel(ctx, &pinecone.CreateIndexForModelRequest{
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
```

`FieldMap` names the record field whose text is embedded. To upsert and search text, see
[Integrated embedding](../inference/integrated-embedding.md).

## Wait for an index to be ready

Index creation returns before the index can serve requests. Before you write data, poll `DescribeIndex` until
`Status.Ready` is true:

```go
for {
	desc, err := pc.DescribeIndex(ctx, "articles")
	if err != nil {
		log.Fatalf("Failed to describe index: %v", err)
	}
	if desc.Status != nil && desc.Status.Ready {
		break
	}
	time.Sleep(5 * time.Second)
}
```

For an index with dedicated read capacity, also wait until `ReadCapacity.Dedicated.Status.State` is `"Ready"`.

## Dedicated read capacity

Serverless indexes use on-demand read capacity by default. To provision dedicated read nodes instead, set
`ReadCapacity` when you create the index. You need to set `NodeType` (`"b1"` or `"t1"`, which has more processing power
and memory) and manual `Replicas` and `Shards`:

```go
nodeType := "t1"
replicas := int32(1)
shards := int32(1)

idx, err := pc.CreateIndex(ctx, &pinecone.CreateIndexRequest{
	Name:   "articles-dedicated",
	Schema: schema,
	ReadCapacity: &pinecone.ReadCapacityParams{
		Dedicated: &pinecone.ReadCapacityDedicatedConfig{
			NodeType: &nodeType,
			Scaling: &pinecone.ReadCapacityScaling{
				Manual: &pinecone.ReadCapacityManualScaling{Replicas: &replicas, Shards: &shards},
			},
		},
	},
})
```

Each shard provides 250 GB of storage, and an index needs at least one. Setting `Replicas` to 0 pauses the index. An
index with dedicated read nodes supports only one namespace. For details, see
[Dedicated read nodes](https://docs.pinecone.io/guides/index-data/dedicated-read-nodes/overview).

You can change read capacity later with `ConfigureIndex`. You can make one change every 10 minutes, and each change can
take up to 30 minutes to complete. Reads and writes continue normally while it's applied.

## List indexes

The following example lists all indexes in your project:

```go
idxs, err := pc.ListIndexes(ctx)
if err != nil {
	log.Fatalf("Failed to list indexes: %v", err)
}
for _, idx := range idxs {
	fmt.Printf("- %s\n", idx.Name)
}
```

## Describe an index

The following example describes an index by name. The returned `Index` includes its `Host`, `Status`, `Schema`, and
`Deployment`.

```go
idx, err := pc.DescribeIndex(ctx, "articles")
if err != nil {
	log.Fatalf("Failed to describe index: %v", err)
}
fmt.Printf("host: %s, ready: %v\n", idx.Host, idx.Status.Ready)
for name, field := range idx.Schema.Fields {
	fmt.Printf("field %s: %+v\n", name, field)
}
```

The `Metric`, `VectorType`, `Dimension`, `Spec`, and `Embed` fields on `Index` are deprecated. They're still
populated, derived from the schema and deployment.

## Delete an index

The following example deletes an index by name:

```go
err := pc.DeleteIndex(ctx, "articles")
if err != nil {
	log.Fatalf("Failed to delete index: %v", err)
}
```

You can't delete an index that has deletion protection enabled. Disable it with `ConfigureIndex` first.

## Configure an index

`ConfigureIndex` changes an existing index's deletion protection, tags, read capacity, and the read and write
parameters of an integrated embedding model.

```go
// Enable deletion protection.
_, err := pc.ConfigureIndex(ctx, "articles", pinecone.ConfigureIndexParams{
	DeletionProtection: pinecone.DeletionProtectionEnabled,
})
if err != nil {
	log.Fatalf("Failed to configure index: %v", err)
}

// Add or update tags. Setting a tag's value to "" removes it.
_, err = pc.ConfigureIndex(ctx, "articles", pinecone.ConfigureIndexParams{
	Tags: pinecone.IndexTags{
		"environment": "production",
		"source":      "",
	},
})

// Switch a vector index to on-demand read capacity.
_, err = pc.ConfigureIndex(ctx, "docs-example", pinecone.ConfigureIndexParams{
	ReadCapacity: &pinecone.ReadCapacityParams{OnDemand: &pinecone.ReadCapacityOnDemandConfig{}},
})

// Update the read parameters of an integrated embedding model. The key is the name of the
// semantic text field in the index's schema, which is the text field from its FieldMap.
_, err = pc.ConfigureIndex(ctx, "example-integrated-index", pinecone.ConfigureIndexParams{
	Schema: &pinecone.ConfigureIndexSchema{
		Fields: map[string]pinecone.ConfigureSemanticTextField{
			"chunk_text": {
				ReadParameters: &map[string]interface{}{"input_type": "query", "truncate": "NONE"},
			},
		},
	},
})
```

An index can have up to 20 tags. Tag keys can be up to 80 characters, and values up to 120.

To move an index from on-demand to dedicated read capacity, set `NodeType`, `Scaling.Manual.Replicas`, and
`Scaling.Manual.Shards`. When you change an existing dedicated configuration, fields you leave out keep their current
values.

Deletion protection prevents deleting the index, but not deleting its namespaces or records.

## Existing pod-based indexes

Pinecone API version `2026-07` doesn't support creating pod-based indexes. Existing pod-based indexes keep working. You
can list, describe, and delete them, run data operations against them, create [collections](./collections.md) from
them, and scale them with `ConfigureIndex`. You can only increase the pod size, and you can't change the pod family (such as `p1`).

```go
// Scale the pod size from "x2" to "x4" and the number of replicas to 4.
_, err := pc.ConfigureIndex(ctx, "example-pod-index", pinecone.ConfigureIndexParams{
	PodType:  "p1.x4",
	Replicas: 4,
})
if err != nil {
	log.Fatalf("Failed to configure index: %v", err)
}
```

To create a new pod-based index, use an earlier SDK release that targets API version `2026-04`. For details, see the
[v7 migration guide](../migration/v7.md).
