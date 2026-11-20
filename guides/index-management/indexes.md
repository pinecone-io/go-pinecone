# Indexes

An index is defined by its **schema**, which lists the fields it declares, and its **deployment**, which says where it
runs. The deployment is serverless, Bring Your Own Cloud (BYOC), or pod-based (for existing indexes only). The schema
determines which data-plane API the index uses:

- A **document index** declares named fields, such as full-text-search fields, dense or sparse vector fields, and
  fields with integrated embedding. You read and write it through the [Documents API](../data-operations/documents.md).
  Create one with `CreateIndex`.
- A **vector index** stores vectors under the reserved field names `_values` (dense), `_sparse_values` (sparse), or
  both. You read and write it through the [Vectors API](../data-operations/vectors.md). Create one with
  `CreateServerlessIndex`, `CreateBYOCIndex`, or `CreateIndex` with a schema made only of the reserved fields.
- A **legacy integrated index**, created with `CreateIndexForModel`, embeds text for you. Legacy integrated indexes
  require the [legacy Records API](../inference/integrated-embedding.md). For new work, use a document index with
  [integrated embedding](#create-a-document-index-with-integrated-embedding) instead.

The examples below assume a `Client` named `pc`. For how to create one, see
[Client configuration](../client-configuration.md).

## Create a document index

`CreateIndex` creates an index from an explicit `IndexSchema`. You can declare these field types:

- `DenseVectorField`: Fixed-dimension dense vectors, with a `Dimension` and `Metric`. At most one per index.
- `SparseVectorField`: Sparse vectors, which take no dimension or metric. At most one per index.
- `StringField` with `FullTextSearch` set: Text indexed for full-text search. At most 100 per index.
- `StringField` with `Embed` or `SparseEmbed` set: Text that Pinecone embeds with a hosted model (integrated
  embedding). A field that sets `Embed` takes the index's one dense slot, and one that sets `SparseEmbed` takes its one
  sparse slot. See [Create a document index with integrated embedding](#create-a-document-index-with-integrated-embedding).

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
			"your_vector":  {DenseVector: &pinecone.DenseVectorField{Dimension: 1536, Metric: pinecone.IndexMetricDotproduct}},
			"sparse_terms": {SparseVector: &pinecone.SparseVectorField{}},
			"body":         {String: &pinecone.StringField{FullTextSearch: &pinecone.FullTextSearchConfig{}}},
		},
	},
})
if err != nil {
	log.Fatalf("Failed to create index: %v", err)
}
fmt.Printf("Created index %s\n", idx.Name)
```

Each search of a document index ranks by one kind of scoring, so a single search can't combine the dense and sparse
fields. To combine them, run a search for each and merge the results, for example with
[reciprocal rank fusion](https://docs.pinecone.io/guides/search/reciprocal-rank-fusion).

Document indexes have these limitations:

- They run only on serverless deployments.
- You can't change the schema after the index is created.

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
if err != nil {
	log.Fatalf("Failed to create index: %v", err)
}
fmt.Printf("Created index %s\n", idx.Name)
```

For the available regions, see [Create an index](https://docs.pinecone.io/guides/index-data/create-an-index).

`CreateIndexRequest` also accepts `ReadCapacity` (see [Dedicated read capacity](#dedicated-read-capacity)), `CmekId`,
`DeletionProtection`, and `Tags`. `Name` is optional. If it's empty, Pinecone generates one. Set a name if you need to
retry the request safely, because a retry with the same name fails with a conflict instead of creating a second index.

### Create a document index with integrated embedding

Document indexes support fields with integrated embedding. Set `Embed` (dense) or `SparseEmbed` (sparse) on a
`StringField`, and Pinecone embeds the field's text with a hosted model when you upsert, and embeds query text when you
search:

```go
idx, err := pc.CreateIndex(ctx, &pinecone.CreateIndexRequest{
	Name: "embedded-articles",
	Schema: pinecone.IndexSchema{
		Fields: map[string]pinecone.IndexSchemaField{
			"body": {String: &pinecone.StringField{
				Embed:       &pinecone.EmbedConfig{},
				SparseEmbed: &pinecone.SparseEmbedConfig{},
			}},
		},
	},
})
if err != nil {
	log.Fatalf("Failed to create index: %v", err)
}
fmt.Printf("Created index %s\n", idx.Name)
```

Every field of `EmbedConfig` and `SparseEmbedConfig` is optional, so an empty config selects the default model. The
configuration is fixed at creation. The text is stored and indexed for full-text search by default; set `StoreText` to
`false` to embed it without storing it. For searching these fields, unstored text, and the 96-document request limit,
see [Integrated embedding](../data-operations/documents.md#integrated-embedding).

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
fmt.Printf("Created index %s\n", idx.Name)
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
if err != nil {
	log.Fatalf("Failed to create index: %v", err)
}
fmt.Printf("Created index %s\n", idx.Name)
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
if err != nil {
	log.Fatalf("Failed to create index: %v", err)
}
fmt.Printf("Created index %s\n", idx.Name)
```

### Create a BYOC index

To create a vector index in a BYOC environment, use `CreateBYOCIndex`:

```go
metric := pinecone.IndexMetricCosine
dimension := int32(1536)

idx, err := pc.CreateBYOCIndex(ctx, &pinecone.CreateBYOCIndexRequest{
	Name:        "example-byoc-index",
	Environment: "YOUR_BYOC_ENVIRONMENT",
	Metric:      &metric,
	Dimension:   &dimension,
})
if err != nil {
	log.Fatalf("Failed to create index: %v", err)
}
fmt.Printf("Created index %s\n", idx.Name)
```

Like a serverless index, a BYOC index uses on-demand read capacity by default. To provision dedicated read nodes
instead, set `ReadCapacity` (see [Dedicated read capacity](#dedicated-read-capacity)). Some BYOC environments support
only dedicated read capacity; there, on-demand, including the default, fails, so set `ReadCapacity.Dedicated`.

For more on BYOC, see [Bring your own cloud](https://docs.pinecone.io/guides/production/bring-your-own-cloud).

<a id="create-an-index-with-integrated-embedding"></a>

## Create a legacy integrated index

A legacy integrated index, created with `CreateIndexForModel`, uses an embedding model hosted by Pinecone to convert
the text of one mapped field to vectors for you. Legacy integrated indexes require the legacy Records API. For new
work, create a [document index with integrated embedding](#create-a-document-index-with-integrated-embedding) instead.

You choose the model when you create the index, and you can't change it afterwards. You can update the model's read and
write parameters with `ConfigureIndex` (see [Configure an index](#configure-an-index)):

```go
idx, err := pc.CreateIndexForModel(ctx, &pinecone.CreateIndexForModelRequest{
	Name:   "example-legacy-integrated-index",
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
fmt.Printf("Created index %s\n", idx.Name)
```

`FieldMap` names the record field whose text is embedded. To upsert and search text, see
[Legacy integrated indexes](../inference/integrated-embedding.md).

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

For an index with dedicated read capacity, also wait until `ReadCapacity.Dedicated.Status.State` is `"Ready"`. If it's
`"Error"`, `Status.ErrorMessage`, if set, says why; stop waiting rather than loop. Read capacity in `"Error"` can still
be configured, so fix the cause and call `ConfigureIndex` again.

## Dedicated read capacity

Serverless and BYOC indexes use on-demand read capacity by default, except in a BYOC environment that supports only
dedicated read capacity. To provision dedicated read nodes instead, set `ReadCapacity` when you create the index. You
need to set `NodeType` (`"b1"` or `"t1"`, which has more processing power and memory), `Shards`, and a scaling strategy.
With manual scaling, you set a fixed number of `Replicas` per shard. Dedicated read capacity is billed, so the examples
use the smallest shape. An index with dedicated read capacity can't switch back to on-demand: `ConfigureIndex` returns a
`PineconeError` with the error code `INVALID_ARGUMENT`. To pause it, set `Replicas` to 0. The [Dedicated read
nodes](https://docs.pinecone.io/guides/index-data/dedicated-read-nodes/overview) overview has more on these settings.

```go
nodeType := "b1"
shards := int32(1)
replicas := int32(1)

idx, err := pc.CreateIndex(ctx, &pinecone.CreateIndexRequest{
	Name:   "articles-dedicated",
	Schema: schema,
	ReadCapacity: &pinecone.ReadCapacityParams{
		Dedicated: &pinecone.ReadCapacityDedicatedConfig{
			NodeType: &nodeType,
			Shards:   &shards,
			Scaling: &pinecone.ReadCapacityScaling{
				Manual: &pinecone.ReadCapacityManualScaling{Replicas: &replicas},
			},
		},
	},
})
if err != nil {
	log.Fatalf("Failed to create index: %v", err)
}
fmt.Printf("Created index %s\n", idx.Name)
```

Each shard provides 250 GB of storage, and an index needs at least one. Setting `Replicas` to 0 pauses the index. An
index with dedicated read nodes supports only one namespace. For details, see
[Dedicated read nodes](https://docs.pinecone.io/guides/index-data/dedicated-read-nodes/overview).

Code written for v7 that sets `Scaling.Manual.Shards` still works: the SDK sends it as `Shards`. That field is
deprecated. If you set both, `Shards` wins and the deprecated value is ignored, so a `Scaling` block copied from a
describe response, which fills in the deprecated field for a manually scaled index, doesn't undo a change to `Shards`.

### Replica autoscaling

With `Scaling.Auto`, Pinecone moves the number of replicas per shard between `MinReplicas` and `MaxReplicas` as load
changes. The autoscaler never changes the shard count; change it yourself with `ConfigureIndex`. Your organization must
be enabled for replica autoscaling; otherwise the request fails with a `PineconeError` whose error code is `FORBIDDEN`.
On a plan without dedicated read capacity, any dedicated request fails with `INVALID_ARGUMENT` instead.

Against per-index and per-project read node limits, an index counts as `Shards` × `Replicas` read nodes, or as
`Shards` × `MaxReplicas` when it autoscales, even while it runs fewer. A request that would go over a limit also fails
with `FORBIDDEN`, so show the error's message rather than assuming which case it is.

A program can't tell the two `FORBIDDEN` cases apart from the error code, so an automatic fallback from `Scaling.Auto`
to `Scaling.Manual` can hide a limit error. The safest choice is not to fall back automatically: report the error and
let a person decide. If you do fall back, request `Replicas` equal to `MaxReplicas`. That counts against the limits like
the Auto request, so a limit error fails again instead of quietly shrinking the index. The index then runs, and is
billed for, `MaxReplicas` replicas per shard until you lower `Replicas` with `ConfigureIndex`.

```go
nodeType := "b1"
shards := int32(1)

idx, err := pc.CreateIndex(ctx, &pinecone.CreateIndexRequest{
	Name:   "articles-autoscaling",
	Schema: schema,
	ReadCapacity: &pinecone.ReadCapacityParams{
		Dedicated: &pinecone.ReadCapacityDedicatedConfig{
			NodeType: &nodeType,
			Shards:   &shards,
			Scaling: &pinecone.ReadCapacityScaling{
				Auto: &pinecone.ReadCapacityAutoScaling{MinReplicas: 1, MaxReplicas: 2},
			},
		},
	},
})
if err != nil {
	log.Fatalf("Failed to create index: %v", err)
}
fmt.Printf("Created index %s\n", idx.Name)
```

When you describe an autoscaled index, `ReadCapacity.Dedicated.ScalingStrategy` is
`pinecone.ReadCapacityScalingStrategyAuto`, `Scaling.Auto` holds the bounds, and `Scaling.Manual` is nil. The live
number of replicas per shard is in `ReadCapacity.Dedicated.Status.CurrentReplicas`. The autoscaler changes the replica
count without leaving the `"Ready"` state, so it doesn't block your own changes.

`MinReplicas` must be at least 1. To pause an autoscaled index, switch it to `Scaling.Manual` with `Replicas` set to 0.

### Change read capacity

You can change read capacity later with `ConfigureIndex`; see [Configure an index](#configure-an-index). A change can
take up to 30 minutes to complete, and reads and writes continue normally while it's applied. While it's applied,
`ReadCapacity.Dedicated.Status.State` is `"Scaling"` or `"Migrating"`, and another read capacity change fails with the
error code `ABORTED`. That includes the initial provisioning right after you create an index with dedicated read
capacity. Wait until the state is `"Ready"`, then try again. The client's `RetryPolicy` doesn't retry `ABORTED`.

The whole `ConfigureIndex` call fails, so tags or deletion protection set in the same call aren't applied either. To
change them while read capacity is being applied, use a separate call without `ReadCapacity`.

`PineconeError.ErrorCode` holds the error code. This example retries on `ABORTED` and stops on any other error:

```go
params := pinecone.ConfigureIndexParams{
	ReadCapacity: &pinecone.ReadCapacityParams{
		Dedicated: &pinecone.ReadCapacityDedicatedConfig{
			Scaling: &pinecone.ReadCapacityScaling{
				Auto: &pinecone.ReadCapacityAutoScaling{MinReplicas: 1, MaxReplicas: 3},
			},
		},
	},
}
for {
	_, err := pc.ConfigureIndex(ctx, "articles-autoscaling", params)
	if err == nil {
		break
	}
	var pcErr *pinecone.PineconeError
	if !errors.As(err, &pcErr) || pcErr.ErrorCode != "ABORTED" {
		log.Fatalf("Failed to configure index: %v", err)
	}
	// Another read capacity change is still being applied. Try again once it settles.
	time.Sleep(time.Minute)
}
```

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

If an index uses a feature that this SDK's API version can't represent, this SDK can't describe it, so `ListIndexes`
leaves it out. `ListIncompatibleIndexes` lists those indexes. Each entry's `MinimumAPIVersion` is the earliest generally
available API version that can describe the index, and every later one can too; `PreviewAPIVersion` is the earliest
early-access API version that can. Either is `nil` when there's no such version, and when both are, no public API
version can describe the index yet. `DescribeIndex`, `ConfigureIndex`, `CreateCollection`, and `CreateBackup` refuse
such an index with the error code `FAILED_PRECONDITION` and the reason `RESOURCE_INCOMPATIBLE_WITH_API_VERSION`, which
the SDK names `pinecone.ReasonResourceIncompatibleWithAPIVersion`; `PineconeError.Details()` carries the same versions.
`DeleteIndex` works unless deletion protection is on, which this SDK can't turn off for such an index. To work with it,
upgrade to an SDK that supports an API version that can describe it, when there is one. The error message's advice to
set the `X-Pinecone-Api-Version` header is for direct API calls; don't do that with this SDK (for example through
`Headers`), because its request and response types are built for one API version and the difference goes unnoticed.

```go
incompatible, err := pc.ListIncompatibleIndexes(ctx)
if err != nil {
	log.Fatalf("Failed to list indexes: %v", err)
}
for _, idx := range incompatible {
	switch {
	case idx.MinimumAPIVersion != nil:
		fmt.Printf("- %s (use API version %s or later)\n", idx.Name, *idx.MinimumAPIVersion)
	case idx.PreviewAPIVersion != nil:
		fmt.Printf("- %s (only on early-access API version %s)\n", idx.Name, *idx.PreviewAPIVersion)
	default:
		fmt.Printf("- %s (no public API version yet)\n", idx.Name)
	}
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

You can't delete an index that has deletion protection enabled. Disable it with `ConfigureIndex` first. For an index
listed by `ListIncompatibleIndexes` that isn't possible with this SDK; see [List indexes](#list-indexes).

Deletion is asynchronous. Until it finishes, the index still appears in `ListIndexes` with `Status.State`
`"Terminating"`, along with its read capacity configuration.

## Configure an index

`ConfigureIndex` changes an existing index's deletion protection, tags, read capacity, and the read and write
parameters of a legacy integrated index's embedding model. The integrated embedding configuration of a document index
is fixed at creation.

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


// Update the read parameters of a legacy integrated index's embedding model. The key is the name
// of the semantic text field in the index's schema, which is the text field from its FieldMap.
// ConfigureIndex returns an error for any other key, because the API would rename the field.
_, err = pc.ConfigureIndex(ctx, "example-legacy-integrated-index", pinecone.ConfigureIndexParams{
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

To move an index from on-demand to dedicated read capacity, set `NodeType`, `Shards`, and a scaling strategy
(`Scaling.Manual` with `Replicas`, or `Scaling.Auto`). When you change an existing dedicated configuration, fields you
leave out keep their current values. Changing only `NodeType` or `Shards` keeps the current scaling strategy, including
the bounds of an autoscaled index.

```go
// Turn on replica autoscaling, or change the bounds of an autoscaled index.
_, err = pc.ConfigureIndex(ctx, "articles-dedicated", pinecone.ConfigureIndexParams{
	ReadCapacity: &pinecone.ReadCapacityParams{
		Dedicated: &pinecone.ReadCapacityDedicatedConfig{
			Scaling: &pinecone.ReadCapacityScaling{
				Auto: &pinecone.ReadCapacityAutoScaling{MinReplicas: 1, MaxReplicas: 2},
			},
		},
	},
})

// Switch back to a fixed number of replicas per shard.
replicas := int32(1)
_, err = pc.ConfigureIndex(ctx, "articles-dedicated", pinecone.ConfigureIndexParams{
	ReadCapacity: &pinecone.ReadCapacityParams{
		Dedicated: &pinecone.ReadCapacityDedicatedConfig{
			Scaling: &pinecone.ReadCapacityScaling{
				Manual: &pinecone.ReadCapacityManualScaling{Replicas: &replicas},
			},
		},
	},
})
```

A `Scaling.Manual` block with neither `Replicas` nor the deprecated `Shards` returns an error without calling the API:
the SDK can't send a manual configuration without a replica count, and sending 0 would pause the index.

Deletion protection prevents deleting the index, but not deleting its namespaces or records.

## Existing pod-based indexes

Pinecone API versions `2026-07` and later don't support creating pod-based indexes. Existing pod-based indexes keep
working. You can list, describe, and delete them, run data operations against them, create
[collections](./collections.md) from them, and scale them with `ConfigureIndex`. You can only increase the pod size, and
you can't change the pod family (such as `p1`).

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
