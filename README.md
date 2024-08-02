# Pinecone Go Client &middot; ![License](https://img.shields.io/github/license/pinecone-io/go-pinecone?color=orange) [![Go Reference](https://pkg.go.dev/badge/github.com/pinecone-io/go-pinecone.svg)](https://pkg.go.dev/github.com/pinecone-io/go-pinecone@main/pinecone) [![Go Report Card](https://goreportcard.com/badge/github.com/pinecone-io/go-pinecone)](https://goreportcard.com/report/github.com/pinecone-io/go-pinecone)

This is the official Go client for [Pinecone](https://www.pinecone.io).

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

To install the Pinecone Go client, run the following in your terminal:

```shell
go get github.com/pinecone-io/go-pinecone/pinecone
```

For more information on setting up a Go project, see the [Go documentation](https://golang.org/doc/).

## Usage

### Initializing the client

**Authenticating via an API key**

When initializing the client with a Pinecone API key, you must construct a `NewClientParams` object and pass it to the
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

## Indexes

### Create indexes

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

### List indexes

The following example lists all indexes in your Pinecone project.

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

### Delete an index

The following example deletes an index by name. Note: only indexes not protected by deletion protection
may be deleted.

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

There are multiple ways to configure Pinecone indexes. You are able to configure Deletion Protection for both
pod-based and Serverless indexes. Additionally, you can configure the size of your pods and the number of replicas
for pod-based indexes. Examples for each of these configurations are provided below.

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

	// To scale the size of your pods-based index from "x2" to "x4":
	_, err := pc.ConfigureIndex(ctx, "my-pod-index", pinecone.ConfigureIndexParams{PodType: "p1.x4"})
	if err != nil {
		fmt.Printf("Failed to configure index: %v\n", err)
	}

	// To scale the number of replicas to 4:
	_, err := pc.ConfigureIndex(ctx, "my-pod-index", pinecone.ConfigureIndexParams{Replicas: 4})
	if err != nil {
		fmt.Printf("Failed to configure index: %v\n", err)
	}

	// To scale both the size of your pods and the number of replicas:
	_, err := pc.ConfigureIndex(ctx, "my-pod-index", pinecone.ConfigureIndexParams{PodType: "p1.x4", Replicas: 4})
	if err != nil {
		fmt.Printf("Failed to configure index: %v\n", err)
	}

	// To enable deletion protection:
	_, err := pc.ConfigureIndex(ctx, "my-index", pinecone.ConfigureIndexParams{DeletionProtection: "enabled"})
	if err != nil {
		fmt.Printf("Failed to configure index: %v\n", err)
	}
}
```

### Describe index statistics

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

To perform data operations on an index, you target it using the `Index` method on a `Client` object. You will
need your index's `Host` value, which you can retrieve via `DescribeIndex` or `ListIndexes`.

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

### Upsert vectors

The following example upserts
vectors ([both dense and sparse](https://docs.pinecone.io/guides/data/upsert-sparse-dense-vectors)) and metadata
to `example-index`.

```go
package main

import (
	"context"
	"fmt"
	"github.com/pinecone-io/go-pinecone/pinecone"
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

	idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host})
	if err != nil {
		log.Fatalf("Failed to create IndexConnection for Host: %v: %v", idx.Host, err)
	}

	metadataMap := map[string]interface{}{
		"genre": "classical",
	}

	metadata, err := structpb.NewStruct(metadataMap)

	sparseValues := pinecone.SparseValues{
		Indices: []uint32{0, 1, 2, 3, 4, 5, 6, 7},
		Values:  []float32{1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0},
	}

	vectors := []*pinecone.Vector{
		{
			Id:           "A",
			Values:       []float32{0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1},
			Metadata:     metadata,
			SparseValues: &sparseValues,
		},
		{
			Id:           "B",
			Values:       []float32{0.2, 0.2, 0.2, 0.2, 0.2, 0.2, 0.2, 0.2},
			Metadata:     metadata,
			SparseValues: &sparseValues,
		},
		{
			Id:           "C",
			Values:       []float32{0.3, 0.3, 0.3, 0.3, 0.3, 0.3, 0.3, 0.3},
			Metadata:     metadata,
			SparseValues: &sparseValues,
		},
		{
			Id:           "D",
			Values:       []float32{0.4, 0.4, 0.4, 0.4, 0.4, 0.4, 0.4, 0.4},
			Metadata:     metadata,
			SparseValues: &sparseValues,
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

### Query an index

#### Query by vector values

The following example queries the index `example-index` with vector values and metadata filtering. Note: you can
also query by sparse values;
see [sparse-dense documentation](https://docs.pinecone.io/guides/data/query-sparse-dense-vectors)
for examples.

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/pinecone-io/go-pinecone/pinecone"
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

	metadataFilter, err := structpb.NewStruct(map[string]interface{}{
		"genre": {"$eq": "documentary"},
		"year":  2019,
	})
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
	"github.com/pinecone-io/go-pinecone/pinecone"
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

```Go
package main

import (
	"context"
	"fmt"
	"github.com/pinecone-io/go-pinecone/pinecone"
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
	"github.com/pinecone-io/go-pinecone/pinecone"
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
	"github.com/pinecone-io/go-pinecone/pinecone"
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

### List collections

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

### Describe a collection

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

### Delete a collection

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
