package main

import (
	"context"
	"crypto/tls"
	"flag"
	"github.com/pinecone-io/go-pinecone/pinecone"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"log"
	"math/rand"
	"time"
)

var (
	serverAddr = flag.String("server_addr", "localhost:10000", "The server address in the format of host:port")
	serviceName = flag.String("service_name", "example-service", "The name that uniquely identifies the Pinecone service")
	apiKey = flag.String("api_key", "", "The Pinecone API Key")
)

func main() {
	flag.Parse()
	config := &tls.Config{}

	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(config)))
	opts = append(opts, grpc.WithAuthority(*serverAddr))
	opts = append(opts, grpc.WithBlock())
	opts = append(opts, grpc.WithTimeout(5 * time.Second))
	log.Printf("connecting to %v", *serverAddr)
	conn, err := grpc.Dial(*serverAddr, opts...)
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
	}
	defer conn.Close()

	client := pinecone.NewRPCClientClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ctx = metadata.AppendToOutgoingContext(ctx, "api-key", *apiKey)

	// upsert
	log.Print("upserting data...")
	upsertResult, upsertErr := client.CallUnary(ctx, &pinecone.Request{
		RequestId: uint64(rand.Intn(10000)),
		Path:      "write",
		Version:   "golang-alpha",
		Body: &pinecone.Request_Index{
			Index: &pinecone.IndexRequest{
				Ids:  []string{"vec1", "vec2"},
				Data: pinecone.FloatArrToNdArrayLogErr([][]float32{
					{0, 1, 2, 3} ,
					{4, 5, 6, 7} ,
					{8, 9, 10, 11},
				}),
			},
		},
		Namespace:   "test-ns-1",
	})
	if upsertErr != nil {
		log.Fatalf("upsert error: %v", upsertErr)
	} else {
		log.Printf("upsert result: %v", upsertResult)
	}

	// fetch
	log.Print("fetching vector...")
	fetchResult, fetchErr := client.CallUnary(ctx, &pinecone.Request{
		RequestId: uint64(rand.Intn(10000)),
		Path:      "read",
		Version:   "golang-alpha",
		Body: &pinecone.Request_Fetch{
			Fetch: &pinecone.FetchRequest{
				Ids:     []string{"vec1", "vec2"},
			},
		},
		Namespace:   "test-ns-1",
	})
	if fetchErr != nil {
		log.Fatalf("fetch error: %v", fetchErr)
	} else {
		log.Printf("fetch result: %v", fetchResult)
		reqBody := fetchResult.Body
		reqFetch := reqBody.(*pinecone.Request_Fetch)
		log.Printf("fetched vector: %v", pinecone.FloatNdArrayToArrLogErr((*reqFetch).Fetch.Vectors[0]))
	}

	// query
	log.Print("querying data...")
	queryResult, queryErr := client.CallUnary(ctx, &pinecone.Request{
		RequestId:         uint64(rand.Intn(10000)),
		Path:              "read",
		Version:           "golang-alpha",
		Body:              &pinecone.Request_Query{
			Query: &pinecone.QueryRequest{
				TopK:        2,
				IncludeData: true,
				Data:        pinecone.FloatArrToNdArrayLogErr([][]float32{
					{0, 1, 2, 4} ,
				}),
			},
		},
		Namespace:         "test-ns-1",
	})
	if queryErr != nil {
		log.Fatalf("query error: %v", queryErr)
	} else {
		log.Printf("query result: %v", queryResult)
		reqBody := queryResult.Body
		reqQuery := reqBody.(*pinecone.Request_Query)
		log.Printf("query #1 results: ids %v data %v",
			pinecone.StringNdArrayToArrLogErr((*reqQuery).Query.Matches[0].Ids, 4),
			pinecone.FloatNdArrayToArrLogErr((*reqQuery).Query.Matches[0].Data))
	}

	// delete
	log.Print("deleting vector...")
	deleteResult, deleteErr := client.CallUnary(ctx, &pinecone.Request{
		RequestId: uint64(rand.Intn(10000)),
		Path:      "write",
		Version:   "golang-alpha",
		Body: &pinecone.Request_Delete{
			Delete: &pinecone.DeleteRequest{
				Ids:     []string{"vec1"},
			},
		},
		Namespace:   "test-ns-1",
	})
	if deleteErr != nil {
		log.Fatalf("delete error: %v", deleteErr)
	} else {
		log.Printf("delete result: %v", deleteResult)
	}

	log.Print("done!")
}