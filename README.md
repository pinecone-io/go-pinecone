# go-pinecone

[![Go Reference](https://pkg.go.dev/badge/github.com/pinecone-io/go-pinecone.svg)](https://pkg.go.dev/github.com/pinecone-io/go-pinecone@main/pinecone)

Official Pinecone Go Client

## Documentation

To see the latest documentation for `main`, visit https://pkg.go.dev/github.com/pinecone-io/go-pinecone@main/pinecone.

To see the latest versioned-release's documentation,
visit https://pkg.go.dev/github.com/pinecone-io/go-pinecone/pinecone.

## Features

go-pinecone contains

- gRPC bindings for [Data Plane](https://docs.pinecone.io/reference/api/2024-07/data-plane) operations
- REST bindings for [Control Plane](https://docs.pinecone.io/reference/api/2024-07/control-plane)
  operations

See the [Pinecone API Docs](https://docs.pinecone.io/reference/) for more information.

## Upgrading your client

tktk

## Prerequisites

`go-pinecone` requires a Go version with [modules](https://go.dev/wiki/Modules) support.

## Installation

To install the package, run the following in your terminal:

```shell
go get github.com/pinecone-io/go-pinecone/pinecone
```

For more information on setting up Go project, see the [Go documentation](https://golang.org/doc/).

## Usage

### Initializing the client

**Authenticating via an API key**

When initializing the client with a Pinecone API key, you must construct a `NewClientParams` and pass it to the
`NewClient` function.

It's recommended that you set your Pinecone API key as an environment variable (`"PINECONE_API_KEY"`) and access it that
way. Alternatively, you can pass it in your code directly.

```go
package main

import (
	"fmt"
	"github.com/pinecone-io/go-pinecone/pinecone"
	"log"
	"os"
)

func main() {
	clientParams := pinecone.NewClientParams{
		ApiKey: os.Getenv("PINECONE_API_KEY"),
	}

	pc, err := pinecone.NewClient(clientParams)

	if err != nil {
		log.Fatalf("Failed to create Client: %v", err)
	} else {
		fmt.Println("Successfully created a new Client object!")
	}
}
```

**Authenticating via custom headers**

If you choose to authenticate via custom headers (e.g. for OAuth), you must construct a `NewClientBaseParams` object
and pass it to `NewClientBase`.

Note: you must include the `"X-Project-Id"` header with your Pinecone project ID
when authenticating via custom headers.

```go
package main

import (
	"fmt"
	"github.com/pinecone-io/go-pinecone/pinecone"
	"log"
)

func main() {
	clientParams := pinecone.NewClientBaseParams{
		Headers: map[string]string{
			"Authorization": "Bearer " + "<your OAuth token>"
			"X-Project-Id":  "<Your Pinecone project ID>"
		},
	}

	pc, err := pinecone.NewClientBase(clientParams)

	if err != nil {
		log.Fatalf("Failed to create Client: %v", err)
	} else {
		fmt.Println("Successfully created a new Client object!")
	}
}


```

### Indexes

#### Create indexes

**Create a serverless index**

The following example creates a serverless index in the `us-west-2`
region of AWS. For more information on serverless and regional availability,
see [Understanding indexes](https://docs.pinecone.io/guides/indexes/understanding-indexes#serverless-indexes).

```go
tk
```

**Create a pod-based index**
The following example creates an index without a metadata
configuration. By default, Pinecone indexes all metadata.

#### List indexes

#### Describe an index

#### Delete an index

#### Configure an index

- pods, replicas, deletion protection
-

#### Describe index statistics

#### Upsert vectors

#### Query an index

#### Delete vectors

#### Fetch vectors

#### Update vectors

#### List vectors

### Collections

#### Create a collection

#### List collections

#### Describe a collection

#### Delete a collection

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

## Contributing and development

### Prereqs

1. A [current version of Go](https://go.dev/doc/install) (recommended 1.21+)
2. The [just](https://github.com/casey/just?tab=readme-ov-file#installation) command runner
3. The [protobuf-compiler](https://grpc.io/docs/protoc-installation/)

Then, execute `just bootstrap` to install the necessary Go packages

### .env Setup

An easy way to keep track of necessary environment variables is to create a `.env` file in the root of the project.
This project comes with a sample `.env` file (`.env.sample`) that you can copy and modify. At the very least, you
will need to include the `PINECONE_API_KEY` variable in your `.env` file for the tests to run locally.

```shell
### API Definitions submodule

The API Definitions are in a private submodule. To checkout or update the submodules execute in the root of the project:

```shell
git submodule update --init --recursive
```

For working with submodules, see the [Git Submodules](https://git-scm.com/book/en/v2/Git-Tools-Submodules)
documentation.

### Just commands

`just test` : Executes all tests for the pinecone package

`just gen` : Generates Go client code from the API definitions

`just docs` : Generates Go docs and starts http server on localhost

`just bootstrap` : Installs necessary go packages for gen and docs
