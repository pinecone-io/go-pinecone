gen:
	protoc --proto_path . --proto_path thirdparty/api-common-protos --proto_path thirdparty/grpc-gateway --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative pinecone_grpc/vector_service.proto

test:
	go test ./...
