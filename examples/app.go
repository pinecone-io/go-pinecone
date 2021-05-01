package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/binary"
	"flag"
	"github.com/pinecone-io/go-pinecone/pinecone"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"log"
	"math"
	"math/rand"
	"time"
)

var (
	serverAddr = flag.String("server_addr", "localhost:10000", "The server address in the format of host:port")
	serviceName = flag.String("service_name", "example-service", "The name that uniquely identifies the Pinecone service")
	apiKey = flag.String("api_key", "", "The Pinecone API Key")
)

func floatArrToNdArray(arr [][]float32) (*pinecone.NdArray, error) {
	var buf bytes.Buffer

	for i := range arr {
		for j := range arr[i] {
			err := binary.Write(&buf, binary.LittleEndian, arr[i][j])
			if err != nil {
				return nil, err
			}
		}
	}

	return &pinecone.NdArray{
		Buffer: buf.Bytes(),
		Shape: []uint32{uint32(len(arr)), uint32(len(arr[0]))},
		Dtype: "float32",
	}, nil
}

func floatArrToNdArrayLogErr(arr [][]float32) *pinecone.NdArray {
	result, err := floatArrToNdArray(arr)
	if err != nil {
		log.Fatalf("failed to convert arr; got error: %v", err)
	}
	return result
}

func floatNdArrayToArr(array *pinecone.NdArray) ([][]float32, error) {
	var buf bytes.Buffer
	buf.Write(array.Buffer)

	var vectorCount, vectorDim uint32
	if len(array.Shape) == 1 {
		vectorCount, vectorDim = 1, array.Shape[0]
	} else {
		vectorCount, vectorDim = array.Shape[0], array.Shape[1]
	}

	result := make([][]float32, vectorCount)
	for i := range result {
		result[i] = make([]float32, vectorDim)
	}

	for i := range result {
		for j := range result[i] {
			bits := binary.LittleEndian.Uint32(buf.Next(4))
			result[i][j] = math.Float32frombits(bits)
		}
	}
	return result, nil
}

func floatNdArrayToArrLogErr(array *pinecone.NdArray) [][]float32 {
	result, err := floatNdArrayToArr(array)
	if err != nil {
		log.Fatal("failed to convert NdArray; got error: %v", err)
	}
	return result
}

func stringNdArrayToArr(array *pinecone.NdArray, itemsize int) ([][]string, error) {
	var buf bytes.Buffer
	buf.Write(array.Buffer)

	var vectorCount, vectorDim uint32
	log.Print(array.Shape)
	if len(array.Shape) == 1 {
		vectorCount, vectorDim = 1, array.Shape[0]
	} else {
		vectorCount, vectorDim = array.Shape[0], array.Shape[1]
	}

	result := make([][]string, vectorCount)
	for i := range result {
		result[i] = make([]string, vectorDim)
	}

	for i := range result {
		for j := range result[i] {
			result[i][j] = string(buf.Next(itemsize))
		}
	}
	return result, nil
}

func stringNdArrayToArrLogErr(array *pinecone.NdArray, itemsize int) [][]string {
	result, err := stringNdArrayToArr(array, itemsize)
	if err != nil {
		log.Fatal("failed to convert NdArray; go error: %v", err)
	}
	return result
}

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
				Data: floatArrToNdArrayLogErr([][]float32{
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
		log.Printf("fetched vector: %v", floatNdArrayToArrLogErr((*reqFetch).Fetch.Vectors[0]))
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
				Data:        floatArrToNdArrayLogErr([][]float32{
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
			stringNdArrayToArrLogErr((*reqQuery).Query.Matches[0].Ids, 4),
			floatNdArrayToArrLogErr((*reqQuery).Query.Matches[0].Data))
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