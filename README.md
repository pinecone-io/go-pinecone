> **Warning**
> 
> **Under active development** This SDK is pre-1.0 and should be considered unstable. Before a 1.0 release, there are
> no guarantees of backward compatibility between minor versions.

# go-pinecone

Official Pinecone Go Client

## Features
go-pinecone contains gRPC bindings for all Pinecone vector plane operations: list, upsert, fetch, query, delete, and info.

It notably does *not* yet support Index management (creating, deleting Pinecone indexes.) 

## Installation
go-pinecone requires a Go version with [modules](https://github.com/golang/go/wiki/Modules) support.

To add a dependency on go-pinecone:
```shell
go get github.com/pinecone-io/go-pinecone
```

## Support
To get help using go-pinecone, reach out to support@pinecone.io.

## Development

### Prereqs

1. A [current version of Go](https://go.dev/doc/install) (recommended 1.21+) 
2. The [just](https://github.com/casey/just?tab=readme-ov-file#installation) command runner
3. The [protobuf-compiler](https://grpc.io/docs/protoc-installation/)

Then, execute `just bootstrap` to install the necessary Go packages

### .env Setup

To avoid race conditions or having to wait for index creation, the tests require a project with at least one pod index
and one serverless index. Copy the api key and index names to a `.env` file. See `.env.example` for a template.

### Just commands

`just test` : Executes all tests for the pinecone package

`just gen` : Generates Go client code from the API definitions

`just docs` : Generates Go docs and starts http server on localhost

`just bootstrap` : Installs necessary go packages for gen and docs
