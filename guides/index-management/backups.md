# Backups

A backup is a static copy of a serverless index that only consumes storage. You can't query a backup, but you can
restore it into a new serverless index. To learn more, see
[Backups overview](https://docs.pinecone.io/guides/manage-data/backups-overview).

Backups have these limitations:

- They're available only on the Standard and Enterprise plans, with up to 500 and 1,000 backups per project.
- They're stored in the same project, cloud, and region as the source index.
- They include only records written at least 15 minutes before the backup was taken.
- They're supported for vector indexes and indexes with integrated embedding, but not for document indexes.

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

## Restore a backup into a new index

Wait for a backup's `Status` to be `"Ready"` before you restore it. `CreateIndexFromBackup` creates a new serverless
index in the backup's project, cloud, and region, and starts a restore job. The new index keeps the source index's
configuration. You can set its `Tags`, `DeletionProtection`, and `ReadCapacity`.

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

A restore job's `Status` is `"Pending"`, `"Completed"`, `"Failed"`, or `"Cancelled"`. `PercentComplete` is reported
only as 100, once the job completes. To list the project's restore jobs, use `ListRestoreJobs`.
