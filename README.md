> **Warning**
> 
> **Production use not recommended.** This release is in a pre-alpha state and we have temporarily paused development to concentrate resources on our other clients. The currently supported way to interact with Pinecone from a Golang app is via our public [REST API](https://docs.pinecone.io/reference/create_index).

# go-pinecone
Pinecone Go Client

## Features
go-pinecone contains basic GRPC bindings for all Pinecone vector plane operations: upsert, fetch, query, delete, and info.

It notably does *not* support Index management (creating, deleting Pinecone indexes) or OpenAPI-based Pinecone APIs. 

## Installation
go-pinecone requires a Go version with [modules](https://github.com/golang/go/wiki/Modules) support.

To add a dependency on go-pinecone:
```shell
go get github.com/pinecone-io/go-pinecone
```

## Usage
See examples/app.go for a usage sample.

## Support
To get help using go-pinecone, reach out to support@pinecone.io.

## Development
Clone with submodules:
```
git clone --recursive git@github.com:pinecone-io/go-pinecone.git
```

Generate code: `make gen`

Run tests: `make test`

View docs: `godoc -http=:6060` then open http://localhost:6060/pkg/github.com/pinecone-io/go-pinecone/pinecone/ (requires installing godoc - https://github.com/golang/tools#downloadinstall)
