# Pinecone Go SDK &middot; ![License](https://img.shields.io/github/license/pinecone-io/go-pinecone?color=orange) [![Go Reference](https://pkg.go.dev/badge/github.com/pinecone-io/go-pinecone.svg)](https://pkg.go.dev/github.com/pinecone-io/go-pinecone@main/pinecone) [![Go Report Card](https://goreportcard.com/badge/github.com/pinecone-io/go-pinecone)](https://goreportcard.com/report/github.com/pinecone-io/go-pinecone)

This is the official Go SDK for [Pinecone](https://www.pinecone.io).

## Documentation

To see the latest documentation for `main`, visit https://pkg.go.dev/github.com/pinecone-io/go-pinecone@main/pinecone.

To see the latest versioned-release's documentation,
visit https://pkg.go.dev/github.com/pinecone-io/go-pinecone/v5/pinecone.

## Features

go-pinecone contains

- gRPC bindings for [Data Plane](https://docs.pinecone.io/reference/api/2025-10/data-plane) operations
- REST bindings for [Control Plane](https://docs.pinecone.io/reference/api/2025-10/control-plane)
  operations
- REST bindings for [Admin API](https://docs.pinecone.io/reference/api/2025-10/admin/)

See the [Pinecone API Docs](https://docs.pinecone.io/reference/) for more information.

## Upgrading the SDK

To upgrade the SDK to the latest version, run:

```shell
go get -u github.com/pinecone-io/go-pinecone/v5/pinecone@latest
```

## Prerequisites

`go-pinecone` requires a Go version with [modules](https://go.dev/wiki/Modules) support.

## Installation

To install the Pinecone Go SDK, run the following in your terminal:

```shell
go get github.com/pinecone-io/go-pinecone/v5/pinecone
```

For more information on setting up a Go project, see the [Go documentation](https://golang.org/doc/).

## Usage

### Initializing a Client

**Authenticating via an API key**

When initializing a `Client` with a Pinecone API key, you must construct a `NewClientParams` object and pass it to the
`NewClient` function which returns `Client`.

It's recommended that you set your Pinecone API key as an environment variable (`"PINECONE_API_KEY"`) and access it that
way. Alternatively, you can pass it in your code directly.

```go
package main

import (
	"fmt"
	"github.com/pinecone-io/go-pinecone/v5/pinecone"
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
	"github.com/pinecone-io/go-pinecone/v5/pinecone"
	"log"
)

func main() {
	clientParams := pinecone.NewClientBaseParams{
		Headers: map[string]string{
			"Authorization": "Bearer " + "<your OAuth token>",
			"X-Project-Id":  "<Your Pinecone project ID>",
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

### Initializing an AdminClient (Admin API)

When initializing an `AdminClient` you must construct a `NewAdminClientParams` object and pass it to the
`NewAdminClient` or `NewAdminClientWithContext` functions which return `AdminClient`.

`AdminClient` is a struct used for accessing the Pinecone Admin API. A prerequisite for using this class is to have a [service account](https://docs.pinecone.io/guides/organizations/manage-service-accounts). To create a service
account, visit the [Pinecone web console](https://app.pinecone.io) and navigate to the `Access > Service Accounts` section.

**Authenticating via client ID and secret**

After creating a service account, you will be provided with a client ID and secret. These values can be passed via the `NewAdminClientParams` struct, or by setting the `PINECONE_CLIENT_ID` and `PINECONE_CLIENT_SECRET` environment variables. The `NewAdminClient` function handles the authentication handshake, and returns an authenticated `AdminClient`.

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/pinecone-io/go-pinecone/v5/pinecone"
)

func main() {
	ctx := context.Background()

	// Create an AdminClient using your credentials
	adminClient, err := pinecone.NewAdminClient(pinecone.NewAdminClientParams{
		ClientId:     "YOUR_CLIENT_ID",
		ClientSecret: "YOUR_CLIENT_SECRET",
	})
	if err != nil {
		log.Fatalf("failed to create AdminClient: %v", err)
	}

	// Create a new project
	project, err := adminClient.Project.Create(ctx, &pinecone.CreateProjectParams{
		Name: "example-project",
	})
	if err != nil {
		log.Fatalf("failed to create project: %v", err)
	}
	fmt.Printf("Created project: %s\n", project.Name)

	// Create a new API within that project
	apiKey, err := adminClient.APIKey.Create(ctx, project.Id, &pinecone.CreateAPIKeyParams{
		Name: "example-api-key",
	})
	if err != nil {
		log.Fatalf("failed to create API key: %v", err)
	}
	fmt.Printf("Created API key: %s\n", apiKey.Id)

	// List all projects
	projects, err := adminClient.Project.List(ctx)
	if err != nil {
		log.Fatalf("failed to list projects: %v", err)
	}
	fmt.Printf("You have %d project(s)\n", len(projects))

	// List API keys for the created project
	apiKeys, err := adminClient.APIKey.List(ctx, project.Id)
	if err != nil {
		log.Fatalf("failed to list API keys: %v", err)
	}
	fmt.Printf("Project '%s' has %d API key(s)\n", project.Name, len(apiKeys))
}
```

## Indexes

### Create indexes

**Create a serverless index**

The following example creates a `dense` serverless index in the `us-east-1`
region of AWS. For more information on serverless and regional availability,
see [Understanding indexes](https://docs.pinecone.io/guides/indexes/understanding-indexes#serverless-indexes).

```go
package main

import (
	"context"
	"fmt"
	"github.com/pinecone-io/go-pinecone/v5/pinecone"
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

	indexName := "my-serverless-index"
	metric := pinecone.Cosine
	dimension := int32(3)

	idx, err := pc.CreateServerlessIndex(ctx, &pinecone.CreateServerlessIndexRequest{
		Name:      indexName,
		Cloud:     pinecone.Aws,
		Region:    "us-east-1",
		Metric:    &metric,
		Dimension: &dimension,
		Tags:      &pinecone.IndexTags{"environment": "development"},
	})

	if err != nil {
		log.Fatalf("Failed to create serverless index: %v", err)
	} else {
		fmt.Printf("Successfully created serverless index: %s", idx.Name)
	}
}
```

You can also create `sparse` only serverless indexes. These indexes enable direct indexing and retrieval of sparse vectors, supporting traditional methods like BM25 and learned sparse models such as [pinecone-sparse-english-v0](https://docs.pinecone.io/models/pinecone-sparse-english-v0). A `sparse` index must have a distance metric of `dotproduct` and does not require a specified dimension. `dotproduct` will be defaulted for sparse indexes when
a Metric is not provided:

```go
package main

import (
	"context"
	"fmt"
	"github.com/pinecone-io/go-pinecone/v5/pinecone"
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

	indexName := "my-serverless-index"
	vectorType := "sparse"
	metric := pinecone.Dotproduct

	idx, err := pc.CreateServerlessIndex(ctx, &pinecone.CreateServerlessIndexRequest{
		Name:       indexName,
		Cloud:      pinecone.Aws,
		Region:     "us-east-1",
		Metric:     &metric,
		VectorType: &vectorType,
		Tags:       &pinecone.IndexTags{"environment": "development"},
	})

	if err != nil {
		log.Fatalf("Failed to create serverless index: %v", err)
	} else {
		fmt.Printf("Successfully created serverless index: %s", idx.Name)
	}
}
```

**Create a serverless integrated index**

Integrated inference requires a serverless index configured for a specific embedding model. You can either create a new index for a model, or configure an existing index for a model. To create an index that accepts source text and converts it to vectors automatically using an embedding model hosted by Pinecone, use the `Client.CreateIndexForModel` method:

```go
package main

import (
	"context"
	"fmt"
	"github.com/pinecone-io/go-pinecone/v5/pinecone"
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

	index, err := pc.CreateIndexForModel(ctx, &pinecone.CreateIndexForModelRequest{
		Name:   "my-integrated-index",
		Cloud:  pinecone.Aws,
		Region: "us-east-1",
		Embed: pinecone.CreateIndexForModelEmbed{
			Model:    "multilingual-e5-large",
			FieldMap: map[string]interface{}{"text": "chunk_text"},
		},
	})

	if err != nil {
		log.Fatalf("Failed to create serverless integrated index: %v", err)
	} else {
		fmt.Printf("Successfully created serverless integrated index: %s", index.Name)
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
	"github.com/pinecone-io/go-pinecone/v5/pinecone"
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

	indexName := "my-pod-index"
	metric := pinecone.Cosine

	podIndexMetadata := &pinecone.PodSpecMetadataConfig{
		Indexed: &[]string{"title", "description"},
	}

	idx, err := pc.CreatePodIndex(ctx, &pinecone.CreatePodIndexRequest{
		Name:           indexName,
		Dimension:      3,
		Environment:    "us-west1-gcp",
		PodType:        "s1",
		MetadataConfig: podIndexMetadata,
		Metric:         &metric,
		Tags:           &pinecone.IndexTags{"environment": "development"},
	})

	if err != nil {
		log.Fatalf("Failed to create pod index: %v", err)
	} else {
		fmt.Printf("Successfully created pod index: %s", idx.Name)
	}

}
```

### List indexes

The following example lists all indexes in your Pinecone project.

```go
package main

import (
	"context"
	"fmt"
	"github.com/pinecone-io/go-pinecone/v5/pinecone"
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

### Describe an index

The following example describes an index by name.

```go
package main

import (
	"context"
	"fmt"
	"github.com/pinecone-io/go-pinecone/v5/pinecone"
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

### Delete an index

The following example deletes an index by name. Note: only indexes not protected by deletion protection
may be deleted.

```go
package main

import (
	"context"
	"fmt"
	"github.com/pinecone-io/go-pinecone/v5/pinecone"
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

### Configure an index

There are multiple ways to configure Pinecone indexes. You are able to configure Deletion Protection and Tags for both pod-based and Serverless indexes. Additionally, you can configure the size of your pods and the number of replicas for pod-based indexes. Examples for each of these configurations are provided below.

```go
package main

import (
	"context"
	"fmt"
	"github.com/pinecone-io/go-pinecone/v5/pinecone"
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

	// To scale the size of your pods-based index from "x2" to "x4":
	_, err := pc.ConfigureIndex(ctx,
		"my-pod-index",
		pinecone.ConfigureIndexParams{
			PodType: "p1.x4",
		},
	)
	if err != nil {
		fmt.Printf("Failed to configure index: %v\n", err)
	}

	// To scale the number of replicas to 4:
	_, err := pc.ConfigureIndex(ctx,
		"my-pod-index",
		pinecone.ConfigureIndexParams{
			Replicas: 4,
		},
	)
	if err != nil {
		fmt.Printf("Failed to configure index: %v\n", err)
	}

	// To scale both the size of your pods and the number of replicas:
	_, err := pc.ConfigureIndex(ctx,
		"my-pod-index",
		pinecone.ConfigureIndexParams{
			PodType: "p1.x4",
			Replicas: 4,
		},
	)
	if err != nil {
		fmt.Printf("Failed to configure index: %v\n", err)
	}

	// To add or remove IndexTags
	_, err := pc.ConfigureIndex(ctx,
		"my-pod-index",
		pinecone.ConfigureIndexParams{
			Tags: pinecone.IndexTags{
				"environment": "development",
				"source":  "",
			},
		},
	)

	// To enable deletion protection:
	_, err := pc.ConfigureIndex(ctx, "my-index", pinecone.ConfigureIndexParams{DeletionProtection: "enabled"})
	if err != nil {
		fmt.Printf("Failed to configure index: %v\n", err)
	}

	// To convert an existing serverless index into an integrated index
	model := "multilingual-e5-large"
	_, err := pc.ConfigureIndex(ctx, "my-serverless-index", pinecone.ConfigureIndexParams{
		Embed: &pinecone.ConfigureIndexEmbed{
			FieldMap: &map[string]interface{}{
				"text": "my-text-field",
			},
			Model: &model,
		},
	})
}
```

### Describe index statistics

The following example describes the statistics of an index by name.

```go
package main

import (
	"context"
	"fmt"
	"github.com/pinecone-io/go-pinecone/v5/pinecone"
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
		desc := fmt.Sprintf("Description: \n  Name: %s\n  Dimension: %d\n  Host: %s\n  Metric: %s\n"+
			"  DeletionProtection"+
			": %s\n"+
			"  Spec: %+v"+
			"\n  Status: %+v\n",
			idx.Name, idx.Dimension, idx.Host, idx.Metric, idx.DeletionProtection, idx.Spec, idx.Status)
		fmt.Println(desc)
	}
}
```

## Index Operations

Pinecone indexes support working with vector data using operations such as upsert, query, fetch, and delete.

### Targeting an index

To perform data operations on an index, you target it using the `Index` method on a `Client` object which returns a pointer to an `IndexConnection`. Calling `Index` will create and dial the index via a new gRPC connection. You can target a specific `Namespace` when calling `Index`, but if you want to reuse the connection with different namespaces, you can call `IndexConnection.WithNamespace`. If no `Namespace` is provided when establishing a new
`IndexConnection`, the default of `"__default__"` will be used.

You will need your index's `Host` value, which you can retrieve via `DescribeIndex` or `ListIndexes`.

```go
package main

import (
	"context"
	"fmt"
	"github.com/pinecone-io/go-pinecone/v5/pinecone"
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

	idx, err := pc.DescribeIndex(ctx, "pinecone-index")
	if err != nil {
		log.Fatalf("Failed to describe index \"%v\": %v", idx.Name, err)
	}

	idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host})
	if err != nil {
		log.Fatalf("Failed to create IndexConnection for Host: %v: %v", idx.Host, err)
	}
}
```

### Working with namespaces

Within an index, records are partitioned into namespaces, and all upserts, queries, and other data operations always target one namespace. You can read more about [namespaces here](https://docs.pinecone.io/guides/index-data/indexing-overview#namespaces).

You can list all namespaces in an index in a paginated format, describe a specific namespace, or delete a namespace. NOTE: Deleting a namespace will delete all record information partitioned in that namespace.

```go
	ctx := context.Background()

	pc, err := pinecone.NewClient(pinecone.NewClientParams{
		ApiKey:    "YOUR_API_KEY",
	})
	if err != nil {
		log.Fatalf("Failed to create Client: %v", err)
	}

	idx, err := pc.DescribeIndex(ctx, "example-index")
	if err != nil {
		log.Fatalf("Failed to describe index \"%v\": %v", idx.Name, err)
	}

	idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host})
	if err != nil {
		log.Fatalf("Failed to create IndexConnection for Host: %v: %v", idx.Host, err)
	}

	// list namespaces
	limit := uint32(10)
	namespaces, err := idxConnection.ListNamespaces(ctx, &pinecone.ListNamespacesParams{
		Limit: &limit,
	})
	if err != nil {
		log.Fatalf("Failed to list namespaces for Host: %v: %v", idx.Host, err)
	}

	// describe a namespace
	namespace1, err := idxConnection.DescribeNamespace(ctx, "my-namespace-1")
	if err != nil {
		log.Fatalf("Failed to describe namespace: %v: %v", "my-namespace-1", err)
	}

	// delete a namespace
	err = idxConnection.DeleteNamespace(ctx, "my-namespace-1")
	if err != nil {
		log.Fatalf("Failed to delete namespace: %v: %v", "my-namespace-1", err)
	}
```

### Upsert vectors

The following example upserts dense vectors and metadata to `example-index` in the namespace `my-namespace`. Upserting to a specific `Namespace` will implicitly create the namespace if it does not exist already.

```go
package main

import (
	"context"
	"fmt"
	"github.com/pinecone-io/go-pinecone/v5/pinecone"
	"google.golang.org/protobuf/types/known/structpb"
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

	idx, err := pc.DescribeIndex(ctx, "example-index")
	if err != nil {
		log.Fatalf("Failed to describe index \"%v\": %v", idx.Name, err)
	}

	idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host, Namespace: "my-namespace"})
	if err != nil {
		log.Fatalf("Failed to create IndexConnection for Host: %v: %v", idx.Host, err)
	}

	metadataMap := map[string]interface{}{
		"genre": "classical",
	}
	metadata, err := structpb.NewStruct(metadataMap)

	vectors := []*pinecone.Vector{
		{
			Id:           "A",
			Values:       []float32{0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1},
			Metadata:     metadata,
		},
		{
			Id:           "B",
			Values:       []float32{0.2, 0.2, 0.2, 0.2, 0.2, 0.2, 0.2, 0.2},
			Metadata:     metadata,
		},
		{
			Id:           "C",
			Values:       []float32{0.3, 0.3, 0.3, 0.3, 0.3, 0.3, 0.3, 0.3},
			Metadata:     metadata,
		},
		{
			Id:           "D",
			Values:       []float32{0.4, 0.4, 0.4, 0.4, 0.4, 0.4, 0.4, 0.4},
			Metadata:     metadata,
		},
	}

	count, err := idxConnection.UpsertVectors(ctx, vectors)
	if err != nil {
		log.Fatalf("Failed to upsert vectors: %v", err)
	} else {
		fmt.Printf("Successfully upserted %d vector(s)", count)
	}
}
```

The following example upserts sparse vectors and metadata to `example-sparse-index`.

```go
package main

import (
	"context"
	"fmt"
	"github.com/pinecone-io/go-pinecone/v5/pinecone"
	"google.golang.org/protobuf/types/known/structpb"
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

	idx, err := pc.DescribeIndex(ctx, "example-sparse-index")
	if err != nil {
		log.Fatalf("Failed to describe index \"%v\": %v", idx.Name, err)
	}

	idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host})
	if err != nil {
		log.Fatalf("Failed to create IndexConnection for Host: %v: %v", idx.Host, err)
	}

	metadataMap := map[string]interface{}{
		"genre": "classical",
	}
	metadata, err := structpb.NewStruct(metadataMap)

	vectors := []*pinecone.Vector{
		{
			Id:           "A",
			Metadata:     metadata,
			SparseValues: &pinecone.SparseValues{
				Indices: []uint32{0, 1, 2, 3, 4, 5, 6, 7},
				Values:  []float32{1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0},
			},
		},
		{
			Id:           "B",
			Metadata:     metadata,
			SparseValues: &pinecone.SparseValues{
				Indices: []uint32{0, 1, 2, 3, 4, 5, 6, 7},
				Values:  []float32{3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 8.0},
			},
		},
		{
			Id:           "C",
			Metadata:     metadata,
			SparseValues: &pinecone.SparseValues{
				Indices: []uint32{0, 1, 2, 3, 4, 5, 6, 7},
				Values:  []float32{4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 8.0, 7.0},
			},
		},
		{
			Id:           "D",
			Metadata:     metadata,
			SparseValues: &pinecone.SparseValues{
				Indices: []uint32{0, 1, 2, 3, 4, 5, 6, 7},
				Values:  []float32{5.0, 6.0, 7.0, 8.0, 9.0, 8.0, 7.0, 6.0},
			},
		},
	}

	count, err := idxConnection.UpsertVectors(ctx, vectors)
	if err != nil {
		log.Fatalf("Failed to upsert vectors: %v", err)
	} else {
		fmt.Printf("Successfully upserted %d vector(s)", count)
	}
}
```

### Import vectors from object storage

You can now [import vectors en masse](https://docs.pinecone.io/guides/data/understanding-imports) from object
storage. `Import` is a long-running, asynchronous operation that imports large numbers of records into a Pinecone
serverless index.

In order to import vectors from object storage, they must be stored in Parquet files and adhere to the necessary
[file format](https://docs.pinecone.io/guides/data/understanding-imports#parquet-file-format). Your object storage
must also adhere to the necessary [directory structure](https://docs.pinecone.io/guides/data/understanding-imports#directory-structure).

The following example imports vectors from an Amazon S3 bucket into a Pinecone serverless index:

```go
    ctx := context.Background()

    clientParams := pinecone.NewClientParams{
        ApiKey: os.Getenv("PINECONE_API_KEY"),
    }

    pc, err := pinecone.NewClient(clientParams)

    if err != nil {
        log.Fatalf("Failed to create Client: %v", err)
    }

    indexName := "sample-index"
    dimension := int32(3)
    metric := pinecone.Cosine

    idx, err := pc.CreateServerlessIndex(ctx, &pinecone.CreateServerlessIndexRequest{
        Name:      indexName,
        Dimension: &dimension,
        Metric:    &metric,
        Cloud:     pinecone.Aws,
        Region:    "us-east-1",
    })

    if err != nil {
        log.Fatalf("Failed to create serverless index: %v", err)
    }

    idx, err = pc.DescribeIndex(ctx, "pinecone-index")

	if err != nil {
        log.Fatalf("Failed to describe index \"%v\": %v", idx.Name, err)
    }

    idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host})
    if err != nil {
        log.Fatalf("Failed to create IndexConnection for Host: %v: %v", idx.Host, err)
    }

    storageURI := "s3://my-bucket/my-directory/"

    errorMode := "abort" // Will abort if error encountered; other option: "continue"

    importRes, err := idxConnection.StartImport(ctx, storageURI, nil, &errorMode)

	if err != nil {
        log.Fatalf("Failed to start import: %v", err)
    }

    fmt.Printf("import started with ID: %s", importRes.Id)
```

You can [start, cancel, and check the status](https://docs.pinecone.io/guides/data/import-data) of all or one import operation(s).

### Query an index

#### Query by vector values

The following example queries the index `example-index` with dense vector values and metadata filtering.

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/pinecone-io/go-pinecone/v5/pinecone"
	"google.golang.org/protobuf/types/known/structpb"
	"log"
	"os"
)

func prettifyStruct(obj interface{}) string {
	bytes, _ := json.MarshalIndent(obj, "", "  ")
	return string(bytes)
}

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

	idx, err := pc.DescribeIndex(ctx, "example-index")
	if err != nil {
		log.Fatalf("Failed to describe index \"%v\": %v", idx.Name, err)
	}

	idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host, Namespace: "example-namespace"})
	if err != nil {
		log.Fatalf("Failed to create IndexConnection for Host %v: %v", idx.Host, err)
	}

	queryVector := []float32{0.3, 0.3, 0.3, 0.3, 0.3, 0.3, 0.3, 0.3}

	metadataMap := map[string]interface{}{
		"genre": map[string]interface{}{
			"$eq": "documentary",
		},
		"year": 2019,
	}

	metadataFilter, err := structpb.NewStruct(metadataMap)
	if err != nil {
		log.Fatalf("Failed to create metadataFilter: %v", err)
	}

	res, err := idxConnection.QueryByVectorValues(ctx, &pinecone.QueryByVectorValuesRequest{
		Vector:         queryVector,
		TopK:           3,
		MetadataFilter: metadataFilter,
		IncludeValues:  true,
	})
	if err != nil {
		log.Fatalf("Error encountered when querying by vector: %v", err)
	} else {
		fmt.Printf(prettifyStruct(res))
	}
}

// Returns:
// {
//   "matches": [
//     {
//       "vector": {
//         "id": "B",
//         "values": [
//           0.2,
//           0.2,
//           0.2,
//           0.2,
//           0.2,
//           0.2,
//           0.2,
//           0.2
//         ]
//       },
//       "score": 1
//     },
//     {
//       "vector": {
//         "id": "C",
//         "values": [
//           0.3,
//           0.3,
//           0.3,
//           0.3,
//           0.3,
//           0.3,
//           0.3,
//           0.3
//         ]
//       },
//       "score": 1
//     },
//     {
//       "vector": {
//         "id": "A",
//         "values": [
//           0.1,
//           0.1,
//           0.1,
//           0.1,
//           0.1,
//           0.1,
//           0.1,
//           0.1
//         ]
//       },
//       "score": 1
//     }
//   ],
//   "usage": {
//     "read_units": 6
//   }
// }
```

#### Query by vector id

The following example queries the index `example-index` with a vector id value.

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/pinecone-io/go-pinecone/v5/pinecone"
	"log"
	"os"
)

func prettifyStruct(obj interface{}) string {
	bytes, _ := json.MarshalIndent(obj, "", "  ")
	return string(bytes)
}

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

	idx, err := pc.DescribeIndex(ctx, "example-index")
	if err != nil {
		log.Fatalf("Failed to describe index \"%v\": %v", idx.Name, err)
	}

	idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host, Namespace: "example-namespace"})
	if err != nil {
		log.Fatalf("Failed to create IndexConnection for Host %v: %v", idx.Host, err)
	}

	vectorId := "vector-id"
	res, err := idxConnection.QueryByVectorId(ctx, &pinecone.QueryByVectorIdRequest{
		VectorId:      vectorId,
		TopK:          3,
		IncludeValues: true,
	})
	if err != nil {
		log.Fatalf("Error encountered when querying by vector ID `%v`: %v", vectorId, err)
	} else {
		fmt.Printf(prettifyStruct(res.Matches))
	}
}
```

### Delete vectors

#### Delete vectors by ID

The following example deletes a vector by its ID value from `example-index` and `example-namespace`. You can pass a
slice of vector IDs to `DeleteVectorsById`.

```go
package main

import (
	"context"
	"fmt"
	"github.com/pinecone-io/go-pinecone/v5/pinecone"
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

	idx, err := pc.DescribeIndex(ctx, "example-index")
	if err != nil {
		log.Fatalf("Failed to describe index \"%s\". Error:%s", idx.Name, err)
	}

	idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host, Namespace: "example-namespace"})
	if err != nil {
		log.Fatalf("Failed to create IndexConnection for Host: %v. Error: %v", idx.Host, err)
	}

	vectorId := "your-vector-id"
	err = idxConnection.DeleteVectorsById(ctx, []string{vectorId})

	if err != nil {
		log.Fatalf("Failed to delete vector with ID: %s. Error: %s\n", vectorId, err)
	}
}
```

#### Delete vectors by filter

The following example deletes vectors from `example-index` using a metadata filter.

```go
package main

import (
	"context"
	"fmt"
	"github.com/pinecone-io/go-pinecone/v5/pinecone"
	"google.golang.org/protobuf/types/known/structpb"
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

	idx, err := pc.DescribeIndex(ctx, "example-index")
	if err != nil {
		log.Fatalf("Failed to describe index \"%s\". Error:%s", idx.Name, err)
	}

	idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host})
	if err != nil {
		log.Fatalf("Failed to create IndexConnection for Host: %v. Error: %v", idx.Host, err)
	}

	filter, err := structpb.NewStruct(map[string]interface{}{
		"genre": "classical",
	})
	if err != nil {
		log.Fatalf("Failed to create metadata filter. Error: %v", err)
	}

	err = idxConnection.DeleteVectorsByFilter(ctx, filter)

	if err != nil {
		log.Fatalf("Failed to delete vector(s) with filter: %+v. Error: %s\n", filter, err)
	}
}
```

#### Delete all vectors in a namespace

The following example deletes all vectors from `example-index` and `example-namespace`.

```go
package main

import (
	"context"
	"fmt"
	"github.com/pinecone-io/go-pinecone/v5/pinecone"
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

	idx, err := pc.DescribeIndex(ctx, "example-index")
	if err != nil {
		log.Fatalf("Failed to describe index \"%s\". Error:%s", idx.Name, err)
	}

	idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host, Namespace: "example-namespace"})
	if err != nil {
		log.Fatalf("Failed to create IndexConnection for Host: %v. Error: %v", idx.Host, err)
	}

	// deletes all vectors in "example-namespace"
	err = idxConnection.DeleteAllVectorsInNamespace(ctx)
	if err != nil {
		log.Fatalf("Failed to delete vectors in namespace: \"%s\". Error: %s", idxConnection.Namespace, err)
	}
}
```

### Fetch vectors

The following example fetches vectors by ID from `example-index` and `example-namespace`.

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/pinecone-io/go-pinecone/v5/pinecone"
	"log"
	"os"
)

func prettifyStruct(obj interface{}) string {
	bytes, _ := json.MarshalIndent(obj, "", "  ")
	return string(bytes)
}

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

	idx, err := pc.DescribeIndex(ctx, "example-index")
	if err != nil {
		log.Fatalf("Failed to describe index \"%v\": %v", idx.Name, err)
	}

	idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host, Namespace: "example-namespace"})
	if err != nil {
		log.Fatalf("Failed to create IndexConnection for Host %v: %v", idx.Host, err)
	}

	res, err := idxConnection.FetchVectors(ctx, []string{"id-1", "id-2"})
	if err != nil {
		log.Fatalf("Failed to fetch vectors: %v", err)
	} else {
		fmt.Printf(prettifyStruct(res))
	}
}

// Response:
// {
//   "vectors": {
//     "id-1": {
//       "id": "id-1",
//       "values": [
//         -0.0089730695,
//         -0.020010853,
//         -0.0042787646,
//         ...
//       ]
//     },
//     "id-2": {
//       "id": "id-2",
//       "values": [
//         -0.005380766,
//         0.00215196,
//         -0.014833462,
//         ...
//       ]
//     }
//   },
//   "usage": {
//     "read_units": 1
//   }
// }
```

### Update vectors

The following example updates vectors by ID in `example-index` and `example-namespace`.

```go
package main

import (
	"context"
	"fmt"
	"github.com/pinecone-io/go-pinecone/v5/pinecone"
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

	idx, err := pc.DescribeIndex(ctx, "pinecone-index")
	if err != nil {
		log.Fatalf("Failed to describe index \"%v\": %v", idx.Name, err)
	}

	idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host, Namespace: "ns1"})
	if err != nil {
		log.Fatalf("Failed to create IndexConnection for Host %v: %v", idx.Host, err)
	}

	id := "id-3"

	err = idxConnection.UpdateVector(ctx, &pinecone.UpdateVectorRequest{
		Id:     id,
		Values: []float32{4.0, 2.0},
	})
	if err != nil {
		log.Fatalf("Failed to update vector with ID %v: %v", id, err)
	}
}
```

### List vectors

The `ListVectors` method can be used to list vector ids matching a particular id prefix.
With clever assignment of vector ids, you can model hierarchical relationships across embeddings within the same
document.

The following example lists all vector ids in `example-index` and `example-namespace`, with the prefix `doc1`.

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/pinecone-io/go-pinecone/v5/pinecone"
	"log"
	"os"
)

func prettifyStruct(obj interface{}) string {
	bytes, _ := json.MarshalIndent(obj, "", "  ")
	return string(bytes)
}

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

	idx, err := pc.DescribeIndex(ctx, "example-index")
	if err != nil {
		log.Fatalf("Failed to describe index \"%v\": %v", idx.Name, err)
	}

	idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host, Namespace: "example-namespace"})
	if err != nil {
		log.Fatalf("Failed to create IndexConnection for Host %v: %v", idx.Host, err)
	}

	limit := uint32(3)
	prefix := "doc1"

	res, err := idxConnection.ListVectors(ctx, &pinecone.ListVectorsRequest{
		Limit:  &limit,
		Prefix: &prefix,
	})
	if len(res.VectorIds) == 0 {
		fmt.Println("No vectors found")
	} else {
		fmt.Printf(prettifyStruct(res))
	}
}

// Response:
// {
//   "vector_ids": [
//     "doc1#chunk1",
//     "doc1#chunk2",
//     "doc1#chunk3"
//   ],
//   "usage": {
//     "read_units": 1
//   },
//   "next_pagination_token": "eyJza2lwX3Bhc3QiOiIwMDBkMTc4OC0zMDAxLTQwZmMtYjZjNC0wOWI2N2I5N2JjNDUiLCJwcmVmaXgiOm51bGx9"
// }
```

## Collections

[A collection is a static copy of an index](https://docs.pinecone.io/guides/indexes/understanding-collections).
Collections are only available for pod-based indexes.

### Create a collection

The following example creates a collection from a source index.

```go
package main

import (
	"context"
	"fmt"
	"github.com/pinecone-io/go-pinecone/v5/pinecone"
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

### List collections

The following example lists all collections in your Pinecone project.

```go
package main

import (
	"context"
	"fmt"
	"github.com/pinecone-io/go-pinecone/v5/pinecone"
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

### Describe a collection

The following example describes a collection by name.

```go
package main

import (
	"context"
	"fmt"
	"github.com/pinecone-io/go-pinecone/v5/pinecone"
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

### Delete a collection

The following example deletes a collection by name.

```go
package main

import (
	"context"
	"fmt"
	"github.com/pinecone-io/go-pinecone/v5/pinecone"
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

## Backups

A backup is a static copy of a serverless index that only consumes storage. It is a non-queryable representation of a set of records. You can create a backup of a serverless index, and you can create a new serverless index from a backup. You can optionally apply new `Tags` and `DeletionProtection` configurations for the index when calling `CreateIndexFromBackup`. You can read more about [backups here](https://docs.pinecone.io/guides/manage-data/backups-overview).

```go
	ctx := context.Background()

	clientParams := pinecone.NewClientParams{
		ApiKey: os.Getenv("PINECONE_API_KEY"),
	}

	pc, err := pinecone.NewClient(clientParams)
	if err != nil {
		log.Fatalf("Failed to create Client: %w", err)
	}

	indexName := "my-index"
	backupName := fmt.Sprintf("backup-%s", indexName)
	backupDesc := fmt.Sprintf("Backup created for index %s", indexName)
	fmt.Printf("Creating backup: %s for index: %s\n", backupName, indexName)

	backup, err := pc.CreateBackup(ctx, &pinecone.CreateBackupParams{
		IndexName:   indexName,
		Name:        &backupName,
		Description: &backupDesc,
	})
	if err != nil {
		log.Fatalf("Failed to create backup: %w", err)
	}

	backup, err = pc.DescribeBackup(ctx, backup.BackupId)
	if err != nil {
		log.Fatalf("Failed to describe backup: %w", err)
	}

	// wait for backup to be "Complete" before triggering a restore job
	log.Printf("Backup status: %v", backup.Status)

	limit := 10
	backups, err := pc.ListBackups(ctx, &pinecone.ListBackupsParams{
		Limit: &limit,
		IndexName: &indexName,
	})
	if err != nil {
		log.Fatalf("Failed to list backups: %w", err)
	}

	// create a new serverless index from the backup
	restoredIndexName := indexName + "-from-backup"
	restoredIndexTags := pinecone.IndexTags{"restored_on": time.Now().Format("2006-01-02 15:04")}
	createIndexFromBackupResp, err := pc.CreateIndexFromBackup(ctx, &pinecone.CreateIndexFromBackupParams{
		BackupId: backup.BackupId,
		Name:     restoredIndexName,
		Tags:     &restoredIndexTags,
	})
	if err != nil {
		log.Fatalf("Failed to create index from backup: %w", err)
	}

	// check the status of the index restoration
	restoreJob, err := pc.DescribeRestoreJob(ctx, createIndexFromBackupResp.RestoreJobId)
	if err != nil {
		log.Fatalf("Failed to describe restore job: %w", err)
	}
```

## Inference

The `Client` object has an `Inference` namespace which exposes an `InferenceService` pointer which allows interacting with Pinecone's [Inference API](https://docs.pinecone.io/guides/inference/generate-embeddings).
The Inference API is a service that gives you access to embedding models hosted on Pinecone's infrastructure. Read more at [Understanding Pinecone Inference](https://docs.pinecone.io/guides/inference/understanding-inference).

### Create Embeddings

Send text to Pinecone's inference API to generate embeddings for documents and queries.

```go
	ctx := context.Background()

	pc, err := pinecone.NewClient(pinecone.NewClientParams{
		ApiKey: "YOUR_API_KEY",
	})
	if err !=  nil {
		log.Fatalf("Failed to create Client: %v", err)
	}

	embeddingModel := "multilingual-e5-large"
	documents := []string{
		"Turkey is a classic meat to eat at American Thanksgiving.",
		"Many people enjoy the beautiful mosques in Turkey.",
	}
	docParameters := pinecone.EmbedParameters{
		InputType: "passage",
		Truncate: "END",
	}

	docEmbeddingsResponse, err := pc.Inference.Embed(ctx, &pinecone.EmbedRequest{
		Model: embeddingModel,
		TextInputs: documents,
		Parameters: docParameters,
	})
	if err != nil {
		log.Fatalf("Failed to embed documents: %v", err)
	}
	fmt.Printf("docs embedding response: %+v", docEmbeddingsResponse)

	// << Upsert documents into Pinecone >>

	userQuery := []string{
		"How should I prepare my turkey?"
	}
	queryParameters := pinecone.EmbedParameters{
		InputType: "query",
		Truncate: "END",
	}
	queryEmbeddingsResponse, err := pc.Inference.Embed(ctx, &pinecone.EmbedRequest{
		Model: embeddingModel,
		TextInputs: userQuery,
		Parameters: queryParameters,
	})
	if err != nil {
		log.Fatalf("Failed to embed query: %v", err)
	}
	fmt.Printf("query embedding response: %+v", queryEmbeddingsResponse)

    // << Send query to Pinecone to retrieve similar documents >>
```

### Rerank documents

Rerank documents in descending relevance-order against a query.

**Note:** The `score` represents the absolute measure of relevance of a given query and passage pair. Normalized
between [0, 1], the `score` represents how closely relevant a specific item and query are, with scores closer to 1
indicating higher relevance.

```go
    ctx := context.Background()

    pc, err := pinecone.NewClient(pinecone.NewClientParams{
        ApiKey: "YOUR-API-KEY"
	})

    if err != nil {
        log.Fatalf("Failed to create Client: %v", err)
    }

    rerankModel := "bge-reranker-v2-m3"
    query := "What are some good Turkey dishes for Thanksgiving?"

    documents := []pinecone.Document{
      {"title": "Turkey Sandwiches", "body": "Turkey is a classic meat to eat at American Thanksgiving."},
      {"title": "Lemon Turkey", "body": "A lemon brined Turkey with apple sausage stuffing is a classic Thanksgiving main course."},
      {"title": "Thanksgiving", "body": "My favorite Thanksgiving dish is pumpkin pie"},
      {"title": "Protein Sources", "body": "Turkey is a great source of protein."},
    }

    // Optional arguments
    topN := 3
    returnDocuments := false
    rankFields := []string{"body"}
    modelParams := map[string]string{
      "truncate": "END",
    }

    rerankRequest := pinecone.RerankRequest{
      Model:           rerankModel,
      Query:           query,
      Documents:       documents,
      TopN:            &topN,
      ReturnDocuments: &returnDocuments,
      RankFields:      &rankFields,
      Parameters:      &modelParams,
    }

    rerankResponse, err := pc.Inference.Rerank(ctx, &rerankRequest)

    if err != nil {
      log.Fatalf("Failed to rerank documents: %v", err)
    }

    fmt.Printf("rerank response: %+v", rerankResponse)
```

### Hosted Models

To see available models hosted by Pinecone, you can use the `DescribeModel` and `ListModels` methods on the `InferenceService` struct. This allows you to retrieve detailed information about specific models.

You can list all available models, with the options of filtering by model `Type` (`"embed"`, `"rerank"`), and `VectorType` (`"sparse"`, `"dense"`) for models with `Type` `"embed"`.

```go
	ctx := context.Background()

	pc, err := pinecone.NewClient(pinecone.NewClientParams{
		ApiKey:    "YOUR_API_KEY",
	})
	if err != nil {
		log.Fatalf("Failed to create Client: %v", err)
	}

	embed := "embed"
	rerank := "rerank"

	embedModels, err := pc.Inference.ListModels(ctx, &pinecone.ListModelsParams{
		Type: &embed,
	})
	if err != nil {
		log.Fatalf("Failed to list embedding models: %v", err)
	}

	rerankModels, err := pc.Inference.ListModels(ctx, &pinecone.ListModelsParams{
		Type: &rerank,
	})
	if err != nil {
		log.Fatalf("Failed to list reranking models: %v", err)
	}
```

You can also describe a single model by name:

```go
	ctx := context.Background()

	pc, err := pinecone.NewClient(pinecone.NewClientParams{
		ApiKey:    "YOUR_API_KEY",
	})
	if err != nil {
		log.Fatalf("Failed to create Client: %v", err)
	}

	model, err := pc.Inference.DescribeModel(ctx, "multilingual-e5-large")
	if err != nil {
		log.Fatalf("Failed to get model: %v", err)
	}

	fmt.Printf("Model (multilingual-e5-large): %+v\n", model)
```

### Integrated Inference

When using an index with integrated inference, embedding and reranking operations are tied to index operations and do not require extra steps. This allows working with an index that accepts source text and converts it to vectors automatically using an embedding model hosted by Pinecone.

Integrated inference requires a serverless index configured for a specific embedding model. You can either create a new index for a model or configure an existing index for a model. See **Create a serverless integrated index** above for specifics on creating these indexes.

Once you have an index configured for a specific embedding model, use the `IndexConnection.UpsertRecords` method to convert your source data to embeddings and upsert them into a namespace.

**Upsert integrated records**

Note the following requirements for each record:

- Each record must contain a unique `_id`, which will serve as the record identifier in the index namespace.
- Each record must contain a field with the data for embedding. This field must match the `FieldMap` specified when creating the index.
- Any additional fields in the record will be stored in the index and can be returned in search results or used to filter search results.

```go
	ctx := context.Background()

	clientParams := pinecone.NewClientParams{
		ApiKey:    "YOUR_API_KEY",
	}

	pc, err := pinecone.NewClient(clientParams)

	if err != nil {
		log.Fatalf("Failed to create Client: %v", err)
	}

	idx, err := pc.DescribeIndex(ctx, "your-index-name")

	if err != nil {
		log.Fatalf("Failed to describe index \"%s\". Error:%s", idx.Name, err)
	}

	idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host, Namespace: "my-namespace"})
	if err != nil {
		log.Fatalf("Failed to create IndexConnection: %v", err)
	}

	records := []*pinecone.IntegratedRecord{
			{
				"_id":        "rec1",
				"chunk_text": "Apple's first product, the Apple I, was released in 1976 and was hand-built by co-founder Steve Wozniak.",
				"category":   "product",
			},
			{
				"_id":        "rec2",
				"chunk_text": "Apples are a great source of dietary fiber, which supports digestion and helps maintain a healthy gut.",
				"category":   "nutrition",
			},
			{
				"_id":        "rec3",
				"chunk_text": "Apples originated in Central Asia and have been cultivated for thousands of years, with over 7,500 varieties available today.",
				"category":   "cultivation",
			},
			{
				"_id":        "rec4",
				"chunk_text": "In 2001, Apple released the iPod, which transformed the music industry by making portable music widely accessible.",
				"category":   "product",
			},
			{
				"_id":        "rec5",
				"chunk_text": "Apple went public in 1980, making history with one of the largest IPOs at that time.",
				"category":   "milestone",
			},
			{
				"_id":        "rec6",
				"chunk_text": "Rich in vitamin C and other antioxidants, apples contribute to immune health and may reduce the risk of chronic diseases.",
				"category":   "nutrition",
			},
			{
				"_id":        "rec7",
				"chunk_text": "Known for its design-forward products, Apple's branding and market strategy have greatly influenced the technology sector and popularized minimalist design worldwide.",
				"category":   "influence",
			},
			{
				"_id":        "rec8",
				"chunk_text": "The high fiber content in apples can also help regulate blood sugar levels, making them a favorable snack for people with diabetes.",
				"category":   "nutrition",
			},
		}

	err = idxConnection.UpsertRecords(ctx, records)
	if err != nil {
			log.Fatalf("Failed to upsert vectors. Error: %v", err)
	}
```

**Search integrated records**

Use the `IndexConnection.SearchRecords` method to convert a query to a vector embedding and then search your namespace for the most semantically similar records, along with their similarity scores.

```go
	res, err := idxConnection.SearchRecords(ctx, &pinecone.SearchRecordsRequest{
			Query: pinecone.SearchRecordsQuery{
				TopK: 5,
				Inputs: &map[string]interface{}{
					"text": "Disease prevention",
				},
			},
	})
	if err != nil {
			log.Fatalf("Failed to search records: %v", err)
	}
	fmt.Printf("Search results: %+v\n", res)
```

To rerank initial search results based on relevance to the query, add the rerank parameter, including the [reranking model](https://docs.pinecone.io/guides/inference/understanding-inference#reranking-models) you want to use, the number of reranked results to return, and the fields to use for reranking, if different than the main query.

For example, repeat the search for the 4 documents most semantically related to the query, “Disease prevention”, but this time rerank the results and return only the 2 most relevant documents:

```go
	topN := int32(2)
	res, err := idxConnection.SearchRecords(ctx, &pinecone.SearchRecordsRequest{
			Query: pinecone.SearchRecordsQuery{
				TopK: 5,
				Inputs: &map[string]interface{}{
					"text": "Disease prevention",
				},
			},
			Rerank: &pinecone.SearchRecordsRerank{
				Model:      "bge-reranker-v2-m3",
				TopN:       &topN,
				RankFields: []string{"chunk_text"},
			},
			Fields: &[]string{"chunk_text", "category"},
		})
	if err != nil {
			log.Fatalf("Failed to search records: %v", err)
	}
	fmt.Printf("Search results: %+v\n", res)
```

## Support

To get help using go-pinecone you can file an issue on [GitHub](https://github.com/pinecone-io/go-pinecone/issues),
visit the [community forum](https://community.pinecone.io/),
or reach out to support@pinecone.io.
