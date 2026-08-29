# Migrating to the 2026-07 Pinecone API

This release moves every surface of the SDK — control plane, data plane (gRPC and REST), inference,
and admin — to Pinecone API version `2026-07`. The `X-Pinecone-Api-Version` header is sent as
`2026-07` on every request.

## The index model: schema + deployment

Under 2026-07 every index carries a persisted **schema** (typed fields) and a **deployment**
(managed, pod, or byoc) instead of top-level `dimension`/`metric`/`vector_type` and a `spec`.

`Index` gains `Schema`, `Deployment`, `ReadCapacity`, `SourceCollection`, `SourceBackupId`, and
`CmekId`. The old fields — `Metric`, `VectorType`, `Dimension`, `Spec`, `Embed` — are still
populated, computed from the schema and deployment, but are deprecated. Existing code that reads
them keeps working.

## Index creation keeps working (with translation)

`CreateServerlessIndex`, `CreateBYOCIndex`, and `CreateIndexForModel` keep their
signatures. Dimension/metric/vector-type declarations are translated to a schema using the reserved
field names the vectors API addresses classic data by (`_values` for dense, `_sparse_values` for
sparse), so the created index is identical on the backend to one created by an earlier API version
and is still served by the vector operations (`UpsertVectors`, `QueryByVectorValues`, ...).

**`CreatePodIndex` is a hard break.** The 2026-07 backend rejects pod index creation outright
("deployment_type 'pod' is not supported on this API version"), so `CreatePodIndex` now returns a
guided error without sending a request. Existing pod indexes remain fully usable — data operations
and `ConfigureIndex` pod scaling keep working.

A few parameters have no 2026-07 equivalent and now return a guided error before any request:

- `SourceCollection` (all create methods): the backend rejects creating an index from a collection.
  Use `CreateIndexFromBackup` to restore a backup.
- `MetadataConfig` on `CreatePodIndex` and `Schema` (metadata schema) on `CreateServerlessIndex` /
  `CreateBYOCIndex`: metadata fields are indexed automatically at upsert; nothing is declared at
  create time. (`CreateIndexForModel` still accepts its `Schema` parameter.)
- `ConfigureIndexParams.Embed`: the convert-to-integrated flow was removed from the API. Embedding
  configuration is set at creation time via `CreateIndexForModel`.

**Hybrid indexes: audit `Dotproduct`.** In earlier versions, `Metric: Dotproduct` on a dense index
was the whole hybrid declaration. In 2026-07, sparse traffic requires the schema to declare a
sparse vector field, and the legacy create translation produces a dense-only (`_values`) schema.
A dense `Dotproduct` index created through `CreateServerlessIndex` will refuse sparse writes with
no error at create time. There is no way to add the sparse field later.

## Documents API (new)

Indexes created natively with a 2026-07 document schema are served by the documents API, not the
vector operations. `IndexConnection` gains `UpsertDocuments`, `SearchDocuments`, `FetchDocuments`,
`UpdateDocuments`, `DeleteDocuments`, and `ListDocuments`, operating on `Document`
(`map[string]interface{}` with an `"_id"` key). A vectors-API write against a document-schema index
is refused by the server with "This index has a document schema, so writes must go through the
documents API."

## Type changes

- `Backup`: `Schema` is now a typed `*IndexSchema` (was `*MetadataSchema`); `NamespaceCount`,
  `RecordCount`, `SizeBytes` are `*int64` (were `*int`); new `SourceIndexDeletedAt *time.Time`;
  `Dimension`/`Metric` are deprecated and computed from the schema's dense vector field.
- `Index`: see above. Field order in marshaled JSON changed.
- `CollectionStatus` gains `CollectionStatusTerminated`.
- `RestoreJob.PercentComplete` is reported by the API only as `100` once complete (no intermediate
  progress).

## New client-side validation

Requests the server was always going to refuse now fail locally, before anything is sent:

- `QueryByVectorValues` / `QueryByVectorId`: `TopK` must be 1–10000.
- `FetchVectors`: at least one ID; IDs (and `ListVectors` prefixes) must be 1–512 characters with
  no NUL byte.
- `ListVectors`: `Limit` 1–100. `FetchVectorsByMetadata`: `Limit` 1–10000, non-empty `Filter`.
- `DeleteVectorsByFilter` / `UpdateVectorsByMetadata`: empty filters are rejected; use
  `DeleteAllVectorsInNamespace` to delete everything.
- `UpsertVectors` / `UpdateVector`: metadata values must be a string, number, boolean, or list of
  strings (null values are accepted; the server strips them on write).
- Dedicated read capacity at create time requires `NodeType` plus manual `Replicas` and `Shards`.

## What you don't need to change

Vector operations against indexes created under earlier API versions are unaffected. Inference
(`Embed`, `Rerank`, `ListModels`, `GetModel`) and Admin keep their signatures — the 2026-07 admin
surface is purely additive. `ConfigureIndex` pod scaling, tags, deletion protection, and read
capacity all keep working; only the `Embed` parameter is gone.
