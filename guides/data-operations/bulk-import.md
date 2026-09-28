# Bulk import

[Importing from object storage](https://docs.pinecone.io/guides/index-data/import-data) is the most efficient way to
load large numbers of records into a serverless index. An import is a long-running, asynchronous operation that reads
files from a directory in Amazon S3, Google Cloud Storage, or Azure Blob Storage.

The files you import depend on the index:

- **Vector indexes**: Parquet files.
- **Indexes with a document schema**: [JSON Lines (JSONL)](https://jsonlines.org/) files, with the `.jsonl` or
  `.jsonl.gz` extension.

Your files must follow the required format and directory structure. For details, see
[Prepare your data](https://docs.pinecone.io/guides/index-data/import-data#3-prepare-your-data).

The examples below assume an `IndexConnection` named `idxConnection`. For how to create one, see
[Targeting an index](../client-configuration.md#targeting-an-index).

## Start an import

`StartImport` takes three arguments:

- The URI of the directory to import from: `s3://BUCKET/DIR`, `gs://BUCKET/DIR`, or
  `https://ACCOUNT.blob.core.windows.net/CONTAINER/DIR`. You can import from S3 only into an index hosted on AWS.
- The ID of a [storage integration](https://docs.pinecone.io/guides/operations/integrations/manage-storage-integrations),
  or nil for a publicly readable bucket.
- An error mode, or nil for the default, `ImportErrorModeAbort`. `ImportErrorModeAbort` stops the import at the first
  record that fails, and `ImportErrorModeContinue` skips records that fail. With `ImportErrorModeContinue`, the import
  doesn't report which records were skipped.

```go
errorMode := string(pinecone.ImportErrorModeContinue)

importRes, err := idxConnection.StartImport(ctx, "s3://BUCKET_NAME/PATH/TO/DIR", nil, &errorMode)
if err != nil {
	log.Fatalf("Failed to start import: %v", err)
}
fmt.Printf("Import started with ID: %s\n", importRes.Id)
```

## Check an import

`DescribeImport` returns an import's status and progress. Each import takes at least 10 minutes.

```go
imp, err := idxConnection.DescribeImport(ctx, importRes.Id)
if err != nil {
	log.Fatalf("Failed to describe import: %v", err)
}
fmt.Printf("Status: %s, %.0f%% complete, %d records imported\n", imp.Status, imp.PercentComplete, imp.RecordsImported)
```

`Status` is an `ImportStatus`: `ImportStatusPending`, `ImportStatusInProgress`, `ImportStatusCompleted`,
`ImportStatusFailed`, or `ImportStatusCancelled`. An import can stay `ImportStatusInProgress` at 100% while
indexing finishes.

## List imports

`ListImports` takes a `*ListImportsRequest`, or nil for the defaults:

```go
limit := int32(10)

res, err := idxConnection.ListImports(ctx, &pinecone.ListImportsRequest{Limit: &limit})
if err != nil {
	log.Fatalf("Failed to list imports: %v", err)
}
for _, imp := range res.Imports {
	fmt.Printf("%s: %s\n", imp.Id, imp.Status)
}
```

To get the next page, pass `res.NextPaginationToken` as `PaginationToken`.

## Cancel an import

```go
err := idxConnection.CancelImport(ctx, importRes.Id)
if err != nil {
	log.Fatalf("Failed to cancel import: %v", err)
}
```

## Import limits

| Limit | Value |
|-------|-------|
| Namespaces per import | 10,000 |
| Files per import | 100,000 |
| Size per file | 10 GB |
| Total input size (on-demand indexes) | 1 TB |

These rules also apply:

- You can't import into an existing namespace. To import into the `__default__` namespace, it must be empty.
- You can't import into an index created with `CreateIndexForModel`. To add records to one, use
  [`UpsertRecords`](../inference/integrated-embedding.md#upsert-records).
