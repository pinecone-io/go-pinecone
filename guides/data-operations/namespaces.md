# Namespaces

A namespace is a partition within an index. Every data operation targets one namespace, whether it uses the Documents
API, the Vectors API, or the Records API. An `IndexConnection` targets the namespace you set when you created it, or
the default namespace, `__default__`. For details, see
[Targeting an index](../client-configuration.md#targeting-an-index). To learn more about namespaces, see
[Manage namespaces](https://docs.pinecone.io/guides/manage-data/manage-namespaces).

Writing to a namespace that doesn't exist creates it. You can also create and manage namespaces directly. These
operations are supported only on serverless indexes.

The examples below assume an `IndexConnection` named `idxConnection`.

## Create a namespace

```go
ns, err := idxConnection.CreateNamespace(ctx, &pinecone.CreateNamespaceParams{
	Name: "example-namespace",
})
if err != nil {
	log.Fatalf("Failed to create namespace: %v", err)
}
fmt.Printf("Created namespace %s\n", ns.Name)
```

To index only some metadata fields for filtering, set `Schema` to a `MetadataSchema` that lists up to 50 fields, each
with `Filterable: true`. When `Schema` is nil, the namespace uses the index's metadata configuration. Creating a
namespace that already exists fails.

## List namespaces

`ListNamespaces` returns one page of namespaces at a time, with up to 100 per page (the default). To list only
namespaces whose names start with a string, set `Prefix`.

```go
limit := uint32(10)
var token *string

for {
	res, err := idxConnection.ListNamespaces(ctx, &pinecone.ListNamespacesParams{
		Limit:           &limit,
		PaginationToken: token,
	})
	if err != nil {
		log.Fatalf("Failed to list namespaces: %v", err)
	}
	for _, ns := range res.Namespaces {
		fmt.Printf("%s: %d records\n", ns.Name, ns.RecordCount)
	}
	if res.Pagination == nil || res.Pagination.Next == "" {
		break
	}
	token = &res.Pagination.Next
}
```

## Describe a namespace

```go
ns, err := idxConnection.DescribeNamespace(ctx, "example-namespace")
if err != nil {
	log.Fatalf("Failed to describe namespace: %v", err)
}
fmt.Printf("%s: %d records, %d bytes\n", ns.Name, ns.RecordCount, ns.SizeBytes)
```

`SizeBytes` is approximate. It reads 0 for data written before size tracking, and can still count recently deleted data.

## Delete a namespace

Deleting a namespace deletes every record in it. This can't be undone.

```go
err := idxConnection.DeleteNamespace(ctx, "example-namespace")
if err != nil {
	log.Fatalf("Failed to delete namespace: %v", err)
}
```

To describe or delete the default namespace, pass `"__default__"` or `""`.
