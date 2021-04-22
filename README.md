# go-pinecone
Pinecone Go Client

This is a simple Go module containing generated client code for the Pinecone GRPC service (based on spec in pinecone/core.proto).

To regenerate the GRPC code (from the repo root directory):
```shell
protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative pinecone/core.proto
```

See the examples dir for example usage.