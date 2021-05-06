# go-pinecone
Pinecone Go Client

## Features
go-pinecone supports all Pinecone dataplane operations: upsert, fetch, query, delete, and info.

It notably does *not* support service management (creating, deleting Pinecone services and routers). 

## Installation
go-pinecone requires a Go version with [modules](https://github.com/golang/go/wiki/Modules) support.

```shell
go get github.com/pinecone-io/go-pinecone
```

## Usage
See examples/app.go for a usage sample.