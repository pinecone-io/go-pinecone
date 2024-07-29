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

To upgrade your client to the latest version, run:

```shell
go get -u github.com/pinecone-io/go-pinecone/pinecone@latest
```

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

The following example creates a serverless index in the `us-east-1`
region of AWS. For more information on serverless and regional availability,
see [Understanding indexes](https://docs.pinecone.io/guides/indexes/understanding-indexes#serverless-indexes).

```go
package main

import (
	"context"
	"fmt"
	"github.com/pinecone-io/go-pinecone/pinecone"
	"log"
	"os"
)

func main() {
	ctx := context.Background()

	clientParams := pinecone.NewClientParams{
		ApiKey: os.Getenv("PINECONE_API_KEY"),
	}

	pc, err := pinecone.NewClient(clientParams)
	if err != nil {
		log.Fatalf("Failed to create Client: %v", err)
	} else {
		fmt.Println("Successfully created a new Client object!")
	}

	idx, err := pc.CreateServerlessIndex(ctx, &pinecone.CreateServerlessIndexRequest{
		Name:      "my-serverless-index",
		Dimension: 3,
		Metric:    pinecone.Cosine,
		Cloud:     pinecone.Aws,
		Region:    "us-east-1",
	})

	if err != nil {
		log.Fatalf("Failed to create serverless index: %s", idx.Name)
	} else {
		fmt.Printf("Successfully created serverless index: %s", idx.Name)
	}
}
```

**Create a pod-based index**
The following example creates a pod-based index with a metadata configuration. If no metadata configuration is
provided, all metadata fields are automatically indexed.

```go
package main

import (
	"context"
	"fmt"
	"github.com/pinecone-io/go-pinecone/pinecone"
	"log"
	"os"
)

func main() {
	ctx := context.Background()

	clientParams := pinecone.NewClientParams{
		ApiKey: os.Getenv("PINECONE_API_KEY"),
	}

	pc, err := pinecone.NewClient(clientParams)
	if err != nil {
		log.Fatalf("Failed to create Client: %v", err)
	} else {
		fmt.Println("Successfully created a new Client object!")
	}

	podIndexMetadata := &pinecone.PodSpecMetadataConfig{
		Indexed: &[]string{"title", "description"},
	}

	idx, err := pc.CreatePodIndex(ctx, &pinecone.CreatePodIndexRequest{
		Name:           "my-pod-index",
		Dimension:      3,
		Metric:         pinecone.Cosine,
		Environment:    "us-west1-gcp",
		PodType:        "s1",
		MetadataConfig: podIndexMetadata,
	})

	if err != nil {
		log.Fatalf("Failed to create pod index: %v", err)
	} else {
		fmt.Printf("Successfully created pod index: %s", idx.Name)
	}

}
```

#### List indexes

The following example lists all indexes in your Pinecone project.

```Go
package main

import (
	"context"
	"fmt"
	"github.com/pinecone-io/go-pinecone/pinecone"
	"log"
	"os"
)

func main() {
	ctx := context.Background()

	clientParams := pinecone.NewClientParams{
		ApiKey: os.Getenv("PINECONE_API_KEY"),
	}

	pc, err := pinecone.NewClient(clientParams)
	if err != nil {
		log.Fatalf("Failed to create Client: %v", err)
	} else {
		fmt.Println("Successfully created a new Client object!")
	}

	idxs, err := pc.ListIndexes(ctx)
	if err != nil {
		log.Fatalf("Failed to list indexes: %v", err)
	} else {
		fmt.Println("Your project has the following indexes:")
		for _, idx := range idxs {
			fmt.Printf("- \"%s\"\n", idx.Name)
		}
	}
}
```

#### Describe an index

The following example describes an index by name.

```Go
package main

import (
	"context"
	"fmt"
	"github.com/pinecone-io/go-pinecone/pinecone"
	"log"
	"os"
)

func main() {
	ctx := context.Background()

	clientParams := pinecone.NewClientParams{
		ApiKey: os.Getenv("PINECONE_API_KEY"),
	}

	pc, err := pinecone.NewClient(clientParams)
	if err != nil {
		log.Fatalf("Failed to create Client: %v", err)
	} else {
		fmt.Println("Successfully created a new Client object!")
	}

	indexName := "the-name-of-my-index"

	idx, err := pc.DescribeIndex(ctx, indexName)
	if err != nil {
		log.Fatalf("Failed to describe index: %s", err)
	} else {
		fmt.Printf("%+v", *idx)
	}
}
  ```

#### Delete an index

The following example deletes an index by name. Note: only indexes not protected by deletion protection
may be deleted.

```Go
package main

import (
	"context"
	"fmt"
	"github.com/pinecone-io/go-pinecone/pinecone"
	"log"
	"os"
)

func main() {
	ctx := context.Background()

	clientParams := pinecone.NewClientParams{
		ApiKey: os.Getenv("PINECONE_API_KEY"),
	}

	pc, err := pinecone.NewClient(clientParams)
	if err != nil {
		log.Fatalf("Failed to create Client: %v", err)
	} else {
		fmt.Println("Successfully created a new Client object!")
	}

	indexName := "the-name-of-my-index"

	err = pc.DeleteIndex(ctx, indexName)
	if err != nil {
		log.Fatalf("Error: %v", err)
	} else {
		fmt.Printf("Index \"%s\" deleted successfully", indexName)
	}
}
```

#### Configure an index

There are multiple ways to configure Pinecone indexes. You are able to configure Deletion Protection for both
pod-based and Serverless indexes. Additionally, you can configure the size of your pods and the number of replicas
for pod-based indexes. Examples for each of these configurations are provided below.

```Go
package main

import (
	"context"
	"fmt"
	"github.com/pinecone-io/go-pinecone/pinecone"
	"log"
	"os"
)

func main() {
	ctx := context.Background()

	clientParams := pinecone.NewClientParams{
		ApiKey: os.Getenv("PINECONE_API_KEY"),
	}

	pc, err := pinecone.NewClient(clientParams)
	if err != nil {
		log.Fatalf("Failed to create Client: %v", err)
	} else {
		fmt.Println("Successfully created a new Client object!")
	}

	// To scale the size of your pods from "x2" to "x4" (only applicable to pod-based indexes):
	_, err := pc.ConfigureIndex(ctx, "my-index", "p1.x4", nil)
	if err != nil {
		fmt.Printf("Failed to configure index: %v\n", err)
	}

	// To scale the number of replicas from 2 to 4 (only applicable to pod-based indexes):
	_, err := pc.ConfigureIndex(ctx, "my-index", nil, 4)
	if err != nil {
		fmt.Printf("Failed to configure index: %v\n", err)
	}

	// To configure deletion protection (applicable to both pod-based and serverless indexes):
	_, err := pc.ConfigureIndex(ctx, "my-index", nil, nil, true)
	if err != nil {
		fmt.Printf("Failed to configure index: %v\n", err)
	}
}
```

#### Describe index statistics

The following examlpe describes the statistics of an index by name.

```Go
package main

import (
	"context"
	"fmt"
	"github.com/pinecone-io/go-pinecone/pinecone"
	"log"
	"os"
)

func main() {
	ctx := context.Background()

	clientParams := pinecone.NewClientParams{
		ApiKey: os.Getenv("PINECONE_API_KEY"),
	}

	pc, err := pinecone.NewClient(clientParams)
	if err != nil {
		log.Fatalf("Failed to create Client: %v", err)
	} else {
		fmt.Println("Successfully created a new Client object!")
	}

	indexName := "the-name-of-my-index"

	idx, err := pc.DescribeIndex(ctx, indexName)
	if err != nil {
		log.Fatalf("Failed to describe index \"%s\". Error:%s", idx.Name, err)
	} else {
		fmt.Printf("Successfully found the \"%s\" index!\n", idx.Name)
	}
}
```

#### Upsert vectors

#### Query an index

#### Delete vectors

#### Fetch vectors

#### Update vectors

#### List vectors

### Collections

[A collection is a static copy of an index](https://docs.pinecone.io/guides/indexes/understanding-collections).
Collections are only available for pod-based indexes.

#### Create a collection

The following example creates a collection from a source index.

```Go
package main

import (
	"context"
	"fmt"
	"github.com/pinecone-io/go-pinecone/pinecone"
	"log"
	"os"
)

func main() {
	ctx := context.Background()

	clientParams := pinecone.NewClientParams{
		ApiKey: os.Getenv("PINECONE_API_KEY"),
	}

	pc, err := pinecone.NewClient(clientParams)
	if err != nil {
		log.Fatalf("Failed to create Client: %v", err)
	} else {
		fmt.Println("Successfully created a new Client object!")
	}

	collection, err := pc.CreateCollection(ctx, &pinecone.CreateCollectionRequest{
		Name:   "my-collection",
		Source: "my-source-index",
	})
	if err != nil {
		log.Fatalf("Failed to create collection: %v", err)
	} else {
		fmt.Printf("Successfully created collection \"%s\".", collection.Name)
	}
}
```

#### List collections

The following example lists all collections in your Pinecone project.

```Go
package main

import (
	"context"
	"fmt"
	"github.com/pinecone-io/go-pinecone/pinecone"
	"log"
	"os"
)

func main() {
	ctx := context.Background()

	clientParams := pinecone.NewClientParams{
		ApiKey: os.Getenv("PINECONE_API_KEY"),
	}

	pc, err := pinecone.NewClient(clientParams)
	if err != nil {
		log.Fatalf("Failed to create Client: %v", err)
	} else {
		fmt.Println("Successfully created a new Client object!")
	}

	collections, err := pc.ListCollections(ctx)
	if err != nil {
		log.Fatalf("Failed to list collections: %v", err)
	} else {
		if len(collections) == 0 {
			fmt.Printf("No collections found in project")
		} else {
			fmt.Println("Collections in project:")
			for _, collection := range collections {
				fmt.Printf("- %s\n", collection.Name)
			}
		}
	}
}
```

#### Describe a collection

The following example describes a collection by name.

```Go
package main

import (
	"context"
	"fmt"
	"github.com/pinecone-io/go-pinecone/pinecone"
	"log"
	"os"
)

func main() {
	ctx := context.Background()

	clientParams := pinecone.NewClientParams{
		ApiKey: os.Getenv("PINECONE_API_KEY"),
	}

	pc, err := pinecone.NewClient(clientParams)
	if err != nil {
		log.Fatalf("Failed to create Client: %v", err)
	} else {
		fmt.Println("Successfully created a new Client object!")
	}

	collection, err := pc.DescribeCollection(ctx, "my-collection")
	if err != nil {
		log.Fatalf("Error describing collection: %v", err)
	} else {
		fmt.Printf("Collection: %+v\n", *collection)
	}
}
```

#### Delete a collection

The following example deletes a collection by name.

```Go
package main

import (
	"context"
	"fmt"
	"github.com/pinecone-io/go-pinecone/pinecone"
	"log"
	"os"
)

func main() {
	ctx := context.Background()

	clientParams := pinecone.NewClientParams{
		ApiKey: os.Getenv("PINECONE_API_KEY"),
	}

	pc, err := pinecone.NewClient(clientParams)
	if err != nil {
		log.Fatalf("Failed to create Client: %v", err)
	} else {
		fmt.Println("Successfully created a new Client object!")
	}

	collectionName := "my-collection"

	err = pc.DeleteCollection(ctx, collectionName)
	if err != nil {
		log.Fatalf("Failed to create collection: %s\n", err)
	} else {
		log.Printf("Successfully deleted collection \"%s\"\n", collectionName)
	}
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

`just test-unit`: Executes only unit tests for the pinecone package

`just gen` : Generates Go client code from the API definitions

`just docs` : Generates Go docs and starts http server on localhost

`just bootstrap` : Installs necessary go packages for gen and docs
