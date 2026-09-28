# Migrating to the 2026-07 Pinecone API

This release moves every part of the SDK — control plane, data plane (gRPC and REST), inference,
and admin — to Pinecone API version `2026-07`. The `X-Pinecone-Api-Version` header is sent as
`2026-07` on every request.

## Breaking changes at a glance

- `CreatePodIndex` and `CreatePodIndexRequest` are removed; pod-based indexes can't be created on
  `2026-07`. [Details](#pod-based-indexes)
- `SourceCollection` and `Schema` on the legacy create requests, and `ConfigureIndexParams.Embed`
  (replaced by `ConfigureIndexParams.Schema`), now return an error.
  [Details](#parameters-that-are-no-longer-supported)
- A dense `IndexMetricDotproduct` index created with `CreateServerlessIndex` or `CreateBYOCIndex` no longer
  accepts sparse vectors. [Details](#hybrid-indexes-check-indexmetricdotproduct)
- `ListImports` takes a `*ListImportsRequest` (nil for defaults) instead of separate `limit` and
  `paginationToken` arguments: `ListImports(ctx, &limit, token)` becomes
  `ListImports(ctx, &pinecone.ListImportsRequest{Limit: &limit, PaginationToken: token})`.
- `QueryByVectorIdRequest.SparseValues` is removed. The API has never accepted a sparse vector
  alongside a query `id`; query by ID alone, or pass the sparse vector to `QueryByVectorValues`.
- Enum constants for `Cloud`, `IndexMetric`, `IndexStatusState`, `ImportStatus`, and
  `ImportErrorMode` are renamed with their type as a prefix (`Aws` → `CloudAWS`, `Cosine` →
  `IndexMetricCosine`, …). [Details](#renamed-enum-constants)
- `Backup` field types changed. [Details](#type-changes)
- Some invalid requests now fail locally instead of at the server.
  [Details](#new-client-side-validation)

## The index model: schema + deployment

Every index is now described by a **schema** (its typed fields) and a **deployment** (serverless,
pod-based, or BYOC), instead of a top-level dimension, metric, and vector type plus a `spec`.

`Index` gains `Schema`, `Deployment`, `ReadCapacity`, `SourceCollection`, `SourceBackupId`, and
`CmekId`. The old fields — `Metric`, `VectorType`, `Dimension`, `Spec`, `Embed` — are still
populated, derived from the schema and deployment, but are deprecated. Existing code that reads
them keeps working.

## Creating indexes

### Existing create methods keep working

`CreateServerlessIndex`, `CreateBYOCIndex`, and `CreateIndexForModel` keep their signatures. The SDK
translates the dimension, metric, and vector type into a schema that uses the reserved field names
`_values` (dense) and `_sparse_values` (sparse). The result is the same kind of vector index that
earlier API versions created, and it works with the vector operations (`UpsertVectors`,
`QueryByVectorValues`, ...).

### Creating an index from a schema (new)

`Client.CreateIndex` creates an index from an explicit `IndexSchema` and, optionally, an
`IndexDeployment` (defaults to serverless on AWS in `us-east-1`). At creation you can declare dense
vector, sparse vector, and full-text-search string fields; metadata fields don't need to be
declared.

A schema containing only `_values` and/or `_sparse_values` creates a vector index. Any other schema
creates a **document index**, which is used with the [document operations](#document-operations-new).

```go
idx, err := pc.CreateIndex(ctx, &pinecone.CreateIndexRequest{
	Name: "hybrid",
	Schema: pinecone.IndexSchema{
		Fields: map[string]pinecone.IndexSchemaField{
			"embedding":    {DenseVector: &pinecone.DenseVectorField{Dimension: 1536, Metric: pinecone.IndexMetricDotproduct}},
			"sparse_terms": {SparseVector: &pinecone.SparseVectorField{}},
		},
	},
})
```

### Pod-based indexes

**`CreatePodIndex` and `CreatePodIndexRequest` have been removed**, including the request's
`ReplicaCount`, `ShardCount`, and `TotalCount` methods. API version `2026-07` does not support
creating pod-based indexes (the server responds with "deployment_type 'pod' is not supported on this
API version"), so code that calls them now fails to compile rather than failing at runtime.

Existing pod-based indexes keep working: data operations, `DescribeIndex`, `ListIndexes`,
`DeleteIndex`, collections, and pod scaling with `ConfigureIndex` (`PodType`, `Replicas`).

To create a new index, use `CreateServerlessIndex` or `CreateIndex`. To create a pod-based index,
use an earlier SDK release that targets API version `2026-04`.

### Parameters that are no longer supported

These parameters are no longer supported. Setting them returns an error before any request is sent:

- `SourceCollection` on `CreateServerlessIndex`: this API version doesn't support creating an
  index from a collection. Use `CreateIndexFromBackup` to restore a backup into a new index.
- `Schema` on `CreateServerlessIndex` and `CreateBYOCIndex`: metadata fields are indexed
  automatically when you upsert data, so they don't need to be declared. (`CreateIndexForModel`
  still accepts its `Schema` parameter.)
- `ConfigureIndexParams.Embed`: use the new `ConfigureIndexParams.Schema` instead (see
  [Updating an integrated index](#updating-an-integrated-index)). `ConfigureIndexEmbed` is
  deprecated.

### Updating an integrated index

An index created with `CreateIndexForModel` reports its embedding configuration as a semantic text
field in its schema, named after the text field in its `FieldMap`. The model and field map can't be
changed after the index is created. The model's read and write parameters can be updated with the
new `ConfigureIndexParams.Schema`, which replaces `ConfigureIndexParams.Embed`:

```go
// Before
pc.ConfigureIndex(ctx, "my-integrated-index", pinecone.ConfigureIndexParams{
	Embed: &pinecone.ConfigureIndexEmbed{
		ReadParameters: &map[string]interface{}{"input_type": "query"},
	},
})

// After
pc.ConfigureIndex(ctx, "my-integrated-index", pinecone.ConfigureIndexParams{
	Schema: &pinecone.ConfigureIndexSchema{
		Fields: map[string]pinecone.ConfigureSemanticTextField{
			"chunk_text": {ReadParameters: &map[string]interface{}{"input_type": "query"}},
		},
	},
})
```

Converting an existing index into an integrated index is no longer supported; create a new index
with `CreateIndexForModel` instead.

### Hybrid indexes: check `IndexMetricDotproduct`

In earlier versions, setting `Metric: IndexMetricDotproduct` on a dense index was enough to store both dense
and sparse vectors. In `2026-07`, sparse vectors require the schema to declare a sparse vector
field, and `CreateServerlessIndex` and `CreateBYOCIndex` only declare a dense one. A dense
`IndexMetricDotproduct` index created with them will reject sparse writes, with no error when the index is
created, and the sparse field can't be added later.

To store dense and sparse vectors in one index, create it with `CreateIndex` and declare both a
`DenseVectorField` and a `SparseVectorField` (see the example above).

## Document operations (new)

Document indexes are used with document operations instead of the vector operations.
`IndexConnection` gains `UpsertDocuments`, `SearchDocuments`, `FetchDocuments`, `UpdateDocuments`,
`DeleteDocuments`, and `ListDocuments`, which work with `Document` (`map[string]interface{}` with an
`"_id"` key). Writing vectors to a document index returns an error from the server: "This index has
a document schema, so writes must go through the documents API."

## Renamed enum constants

Every enum constant now carries its type name as a prefix, as `CollectionStatus`,
`DeletionProtection`, `InviteStatus`, `PrincipalType`, and `ResourceType` already did. The bare
names collided across types (`Failed` was taken by `ImportStatus`, so `IndexStatusState` couldn't
gain one). The string values sent to and received from the API are unchanged.

| Type | Old | New |
|------|-----|-----|
| `Cloud` | `Aws`, `Azure`, `Gcp` | `CloudAWS`, `CloudAzure`, `CloudGCP` |
| `IndexMetric` | `Cosine`, `Dotproduct`, `Euclidean` | `IndexMetricCosine`, `IndexMetricDotproduct`, `IndexMetricEuclidean` |
| `IndexStatusState` | `InitializationFailed`, `Initializing`, `Ready`, `ScalingDown`, `ScalingUp`, `ScalingUpPodSize`, `Terminating` | `IndexStatusStateInitializationFailed`, `IndexStatusStateInitializing`, `IndexStatusStateReady`, `IndexStatusStateScalingDown`, `IndexStatusStateScalingUp`, `IndexStatusStateScalingUpPodSize`, `IndexStatusStateTerminating` |
| `ImportStatus` | `Cancelled`, `Completed`, `Failed`, `InProgress`, `Pending` | `ImportStatusCancelled`, `ImportStatusCompleted`, `ImportStatusFailed`, `ImportStatusInProgress`, `ImportStatusPending` |
| `ImportErrorMode` | `Abort`, `Continue` | `ImportErrorModeAbort`, `ImportErrorModeContinue` |

`IndexStatusState` also gains `IndexStatusStateFailed` and `IndexStatusStateDisabled`, which the API
reports, and drops `ScalingDownPodSize`, which it never does (pod size can only be increased).

## Type changes

- `Backup`: `Schema` is now `*IndexSchema` (was `*MetadataSchema`); `NamespaceCount`,
  `RecordCount`, and `SizeBytes` are `*int64` (were `*int`); new `SourceIndexDeletedAt *time.Time`;
  `Dimension` and `Metric` are deprecated and derived from the schema's dense vector field.
  `CreatedAt` now keeps fractional seconds when the API returns them.
- `Index`: see [the index model](#the-index-model-schema--deployment). Field order in marshaled JSON
  changed.
- `CollectionStatus` gains `CollectionStatusTerminated`.
- `RestoreJob.PercentComplete` is reported by the API only as `100` once complete (no intermediate
  progress).

## New client-side validation

Requests the server would always refuse now fail locally, before anything is sent:

- `QueryByVectorValues` / `QueryByVectorId`: `TopK` must be 1–10000.
- `FetchVectors`: at least one ID; IDs (and `ListVectors` prefixes) must be 1–512 characters with
  no NUL byte.
- `ListVectors` / `ListImports`: `Limit` 1–100. `FetchVectorsByMetadata`: `Limit` 1–10000, non-empty `Filter`.
- `DeleteVectorsByFilter` / `UpdateVectorsByMetadata`: empty filters are rejected; use
  `DeleteAllVectorsInNamespace` to delete everything.
- `UpsertVectors` / `UpdateVector` / `UpdateVectorsByMetadata`: metadata values must be a string,
  number, boolean, or list of strings. Null values are rejected.
- `UpsertRecords`: a record must carry exactly one of `_id` or `id`.
- `SearchRecords`: a `Rerank` must name at least one `RankFields` entry.
- Dedicated read capacity at create time requires `NodeType` plus manual `Replicas` and `Shards`.

## What you don't need to change

Vector operations against existing indexes are unaffected. Inference (`Embed`, `Rerank`,
`ListModels`, `GetModel`) and Admin keep their signatures; the `2026-07` admin changes only add new
functionality. `ConfigureIndex` pod scaling, tags, deletion protection, and read capacity all keep
working; the `Embed` parameter is replaced by `Schema`.
