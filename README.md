# go-pinecone
Pinecone Go Client

## Features
go-pinecone supports all Pinecone dataplane operations: upsert, fetch, query, delete, and info.

It notably does *not* support service management (creating, deleting Pinecone services and routers). 

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
Generate code: `make gen`

Run tests: `make test`

View docs: `godoc -http=:6060` then open http://localhost:6060/pkg/github.com/pinecone-io/go-pinecone/pinecone/ (requires installing godoc - https://github.com/golang/tools#downloadinstall)
