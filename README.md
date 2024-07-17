> **Warning**
>
> **Under active development** This SDK is pre-1.0 and should be considered unstable. Before a 1.0 release, there are
> no guarantees of backward compatibility between minor versions.

# go-pinecone

[![Go Reference](https://pkg.go.dev/badge/github.com/pinecone-io/go-pinecone.svg)](https://pkg.go.dev/github.com/pinecone-io/go-pinecone)

Official Pinecone Go Client

## Features

go-pinecone contains

- gRPC bindings for Data Plane operations on Vectors
- REST bindings for Control Plane operations on Indexes and Collections

See [Pinecone API Docs](https://docs.pinecone.io/reference/) for more info.

## Installation

go-pinecone requires a Go version with [modules](https://go.dev/wiki/Modules) support.

To add a dependency on go-pinecone:

```shell
go get github.com/pinecone-io/go-pinecone/pinecone
```

## Usage

```go
package main

import (
	"context"
	"fmt"
	"github.com/pinecone-io/go-pinecone/pinecone"
)

func main() {
	ctx := context.Background()

	pc, err := pinecone.NewClient(pinecone.NewClientParams{
		ApiKey: "api-key",
	})

	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	idxs, err := pc.ListIndexes(ctx)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	for _, index := range idxs {
		fmt.Println(index)
	}

	idx, err := pc.Index(idxs[0].Host)
	defer idx.Close()

	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	res, err := idx.DescribeIndexStats(ctx)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println(res)
}
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

### API Definitions submodule

The API Definitions are in a private submodule. To checkout or update the submodules execute in the root of the project:

```shell
git submodule update --init --recursive
```

For working with submodules, see the [Git Submodules](https://git-scm.com/book/en/v2/Git-Tools-Submodules) documentation.

### Just commands

`just test` : Executes all tests for the pinecone package

`just gen` : Generates Go client code from the API definitions

`just docs` : Generates Go docs and starts http server on localhost

`just bootstrap` : Installs necessary go packages for gen and docs
