# Pinecone Go SDK guides

These guides cover the Pinecone Go SDK in more detail than the top-level [README](../README.md). For the full API
reference, see [pkg.go.dev](https://pkg.go.dev/github.com/pinecone-io/go-pinecone/v7/pinecone).

## Getting started

- [Client configuration](./client-configuration.md): API keys, custom headers, retries, and targeting an index

## Index management

- [Indexes](./index-management/indexes.md): schemas, deployments, read capacity, and creating, configuring, and
  deleting indexes
- [Backups](./index-management/backups.md): backing up serverless indexes and restoring them into new indexes
- [Collections](./index-management/collections.md): static copies of existing pod-based indexes

## Data operations

- [Working with documents](./data-operations/documents.md): upsert, search, fetch, update, delete, and list documents
  in document indexes
- [Working with vectors](./data-operations/vectors.md): upsert, query, fetch, update, delete, and list records in
  vector indexes
- [Namespaces](./data-operations/namespaces.md): creating, listing, describing, and deleting namespaces
- [Bulk import](./data-operations/bulk-import.md): importing records and documents from object storage

## Inference

- [Inference API](./inference/inference-api.md): embeddings, reranking, and hosted models
- [Integrated embedding](./inference/integrated-embedding.md): upserting and searching text in indexes that embed it
  for you

## Administration

- [Admin API](./admin.md): projects, organizations, API keys, service accounts, role bindings, invites, and users

## Upgrading

- [v7 migration guide](./migration/v7.md): changes for Pinecone API version `2026-07`
