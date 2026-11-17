# Backups

A backup is a static copy of a serverless index that only consumes storage. You can't query a backup, but you can
restore it into a new serverless index. To learn more, see
[Backups overview](https://docs.pinecone.io/guides/manage-data/backups-overview).

Backups have these limitations:

- They're available only on the Standard and Enterprise plans, with up to 500 and 1,000 backups per project.
- They're stored in the same project, cloud, and region as the source index.
- They include only records written at least 15 minutes before the backup was taken.
- They're supported for vector indexes, document indexes, and legacy integrated indexes.

The examples below assume a `Client` named `pc`. For how to create one, see
[Client configuration](../client-configuration.md).

## Create a backup

```go
name := "example-backup"
description := "Monthly backup of docs-example"

backup, err := pc.CreateBackup(ctx, &pinecone.CreateBackupParams{
	IndexName:   "docs-example",
	Name:        &name,
	Description: &description,
})
if err != nil {
	log.Fatalf("Failed to create backup: %v", err)
}
fmt.Printf("Created backup %s\n", backup.BackupId)
```

## Describe, list, and delete backups

A backup's `Status` is `"Initializing"`, `"Ready"`, or `"InitializationFailed"`.

```go
backup, err := pc.DescribeBackup(ctx, backupId)
if err != nil {
	log.Fatalf("Failed to describe backup: %v", err)
}
fmt.Printf("Backup status: %s\n", backup.Status)

indexName := "docs-example"
limit := 10

backups, err := pc.ListBackups(ctx, &pinecone.ListBackupsParams{
	IndexName: &indexName,
	Limit:     &limit,
})
if err != nil {
	log.Fatalf("Failed to list backups: %v", err)
}
for _, b := range backups.Data {
	fmt.Printf("%s: %s\n", b.BackupId, b.Status)
}

err = pc.DeleteBackup(ctx, backupId)
```

To list every backup in the project, leave out `IndexName`. To also list backups of deleted indexes that had that name,
set `IncludeDeleted`, which requires `IndexName`.

A backup that uses a feature from a later API version than this SDK uses can't be described by this SDK, so
`ListBackups` reports it in `IncompatibleBackups` instead of `Data`, with its `BackupId` and the oldest API version that
can describe it. Each one counts toward `Limit`, so a page can hold fewer than `Limit` backups in `Data` while
`Pagination` still points to more. `DescribeBackup` and `CreateIndexFromBackup` refuse such a backup with the error code
`FAILED_PRECONDITION`; `DeleteBackup` still works. To work with it, upgrade to an SDK that supports that API version;
don't override the `X-Pinecone-Api-Version` header (see [List indexes](indexes.md#list-indexes)).

```go
backups, err := pc.ListBackups(ctx, nil)
if err != nil {
	log.Fatalf("Failed to list backups: %v", err)
}
for _, b := range backups.IncompatibleBackups {
	fmt.Printf("%s (needs API version %s or later)\n", b.BackupId, b.MinAPIVersion)
}
```

## Restore a backup into a new index

Wait for a backup's `Status` to be `"Ready"` before you restore it. `CreateIndexFromBackup` creates a new index in the
backup's project and starts a restore job. The new index keeps the source index's configuration, except read capacity:
it's on-demand unless you set `ReadCapacity`. You can set its `Tags`, `DeletionProtection`, `ReadCapacity`, and
`Deployment`. Dedicated read capacity needs enough shards to hold the backup's data; if it has too few, the request
fails with the error code `INVALID_ARGUMENT` and a message that names the minimum number of shards.

```go
tags := pinecone.IndexTags{"restored_on": time.Now().Format("2006-01-02")}

res, err := pc.CreateIndexFromBackup(ctx, &pinecone.CreateIndexFromBackupParams{
	BackupId: backupId,
	Name:     "docs-example-restored",
	Tags:     &tags,
})
if err != nil {
	log.Fatalf("Failed to create index from backup: %v", err)
}

job, err := pc.DescribeRestoreJob(ctx, res.RestoreJobId)
if err != nil {
	log.Fatalf("Failed to describe restore job: %v", err)
}
fmt.Printf("Restore job status: %s\n", job.Status)
```

A restore job's `Status` is `"Pending"`, `"Completed"`, `"Failed"`, or `"Cancelled"`. The restore is finished when it's
`"Completed"`. `PercentComplete` is reported
only as 100, once the job completes. To list the project's restore jobs, use `ListRestoreJobs`. Like `ListBackups`, it
reports a restore job this SDK can't describe in `IncompatibleRestoreJobs` instead of `Data`, and `DescribeRestoreJob`
refuses such a job with the error code `FAILED_PRECONDITION`.

### Restore into another region

By default, the index is created where the backup was taken: the same cloud and region for a serverless backup, or the
same environment for a BYOC backup. To restore a serverless backup into another region of the same cloud, set
`Deployment` to a `ManagedDeployment` in that region:

```go
res, err := pc.CreateIndexFromBackup(ctx, &pinecone.CreateIndexFromBackupParams{
	BackupId: backupId,
	Name:     "docs-example-us-west-2",
	Deployment: &pinecone.IndexDeployment{
		Managed: &pinecone.ManagedDeployment{Cloud: pinecone.CloudAWS, Region: "us-west-2"},
	},
})
if err != nil {
	log.Fatalf("Failed to create index from backup: %v", err)
}
```

Cross-region restore isn't yet available from every source region; the API returns `412` when it isn't. Cross-cloud
restore is not supported, a serverless backup can't be restored into a BYOC environment, and a BYOC backup can only be
restored into the environment it was taken in.
