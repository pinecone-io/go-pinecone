package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"github.com/pinecone-io/go-pinecone/pinecone_grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"log"
	"math/rand"
	"time"
)

var (
	indexName = flag.String("index", "", "The pinecone index name.")
	projectName = flag.String("project", "", "The pinecone project name.")
	pineconeEnv = flag.String("environment", "us-west1-gcp", "The pinecone environment name.")
	apiKey = flag.String("api-key", "", "The Pinecone API Key.")
)

func main() {
	flag.Parse()
	rand.Seed(time.Now().UTC().UnixNano())

	config := &tls.Config{}

	ctx, cancel := context.WithTimeout(context.Background(), 5 * time.Second)
	defer cancel()

	ctx = metadata.AppendToOutgoingContext(ctx, "api-key", *apiKey)
	target := fmt.Sprintf("%s-%s.svc.%s.pinecone.io:443", *indexName, *projectName, *pineconeEnv)
	log.Printf("connecting to %v", target)
	conn, err := grpc.DialContext(
		ctx,
		target,
		grpc.WithTransportCredentials(credentials.NewTLS(config)),
		grpc.WithAuthority(target),
		grpc.WithBlock(),
	)
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
	}
	defer conn.Close()

	client := pinecone_grpc.NewVectorServiceClient(conn)

	// upsert
	log.Print("upserting data...")
	upsertResult, upsertErr := client.Upsert(ctx, &pinecone_grpc.UpsertRequest{
		Vectors: []*pinecone_grpc.Vector{
			{
				Id: "example-vector-1",
				Values: []float32{0.01, 0.01, 0.01, 0.01, 0.01, 0.01, 0.01, 0.01},
			},
			{
				Id: "example-vector-2",
				Values: []float32{0.02, 0.02, 0.02, 0.02, 0.02, 0.02, 0.02, 0.02},
			},
		},
		Namespace: "example-namespace",
	})
	if upsertErr != nil {
		log.Fatalf("upsert error: %v", upsertErr)
	} else {
		log.Printf("upsert result: %v", upsertResult)
	}

	// fetch
	log.Print("fetching vector...")
	fetchResult, fetchErr := client.Fetch(ctx, &pinecone_grpc.FetchRequest{
		Ids:     []string{"example-vector-1", "example-vector-2"},
		Namespace:   "example-namespace",
	})
	if fetchErr != nil {
		log.Fatalf("fetch error: %v", fetchErr)
	} else {
		log.Printf("fetch result: %v", fetchResult)
	}

	// query
	log.Print("querying data...")
	queryResult, queryErr := client.Query(ctx, &pinecone_grpc.QueryRequest{
		Queries: []*pinecone_grpc.QueryVector{
			{Values: []float32{0.01, 0.01, 0.01, 0.01, 0.01, 0.01, 0.01, 0.01},},
			{Values: []float32{0.02, 0.02, 0.02, 0.02, 0.02, 0.02, 0.02, 0.02},},
		},
		TopK: 3,
		IncludeValues: true,
		Namespace: "example-namespace",
	})
	if queryErr != nil {
		log.Fatalf("query error: %v", queryErr)
	} else {
		log.Printf("query result: %v", queryResult)
	}

	// delete
	log.Print("deleting vectors...")
	deleteResult, deleteErr := client.Delete(ctx, &pinecone_grpc.DeleteRequest{
		Ids:     []string{"example-vector-1", "example-vector-2"},
		Namespace:   "example-namespace",
	})
	if deleteErr != nil {
		log.Fatalf("delete error: %v", deleteErr)
	} else {
		log.Printf("delete result: %v", deleteResult)
	}

	// describeIndexStats
	log.Print("describing index statistics...")
	describeIndexStatsResult, describeIndexStatsErr := client.DescribeIndexStats(ctx, &pinecone_grpc.DescribeIndexStatsRequest{})
	if describeIndexStatsErr != nil {
		log.Fatalf("describeIndexStats error: %v", describeIndexStatsErr)
	} else {
		log.Printf("describeIndexStats result: %v", describeIndexStatsResult)
	}
	log.Print("done!")
}