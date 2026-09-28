# Collections

[A collection is a static copy of an index](https://docs.pinecone.io/guides/indexes/understanding-collections).
Collections are available only for pod-based indexes. Pinecone API version `2026-07` can't create pod-based indexes or
create an index from a collection, so collections are mainly useful for keeping copies of existing pod-based indexes.
A collection can't be moved to a different project. For serverless indexes, use [backups](./backups.md) instead.

The examples below assume a `Client` named `pc`. For how to create one, see
[Client configuration](../client-configuration.md).

## Create a collection

The following example creates a collection from a pod-based source index.

```go
collection, err := pc.CreateCollection(ctx, &pinecone.CreateCollectionRequest{
	Name:   "example-collection",
	Source: "example-pod-index",
})
if err != nil {
	log.Fatalf("Failed to create collection: %v", err)
}
fmt.Printf("Created collection %s\n", collection.Name)
```

## List collections

```go
collections, err := pc.ListCollections(ctx)
if err != nil {
	log.Fatalf("Failed to list collections: %v", err)
}
for _, collection := range collections {
	fmt.Printf("- %s\n", collection.Name)
}
```

## Describe a collection

```go
collection, err := pc.DescribeCollection(ctx, "example-collection")
if err != nil {
	log.Fatalf("Failed to describe collection: %v", err)
}
fmt.Printf("%s: %s, %d vectors\n", collection.Name, collection.Status, collection.VectorCount)
```

## Delete a collection

```go
err := pc.DeleteCollection(ctx, "example-collection")
if err != nil {
	log.Fatalf("Failed to delete collection: %v", err)
}
```
