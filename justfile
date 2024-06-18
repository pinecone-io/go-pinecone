test:
  #!/usr/bin/env bash
  set -o allexport
  source .env
  set +o allexport
  go test -count=1 -v ./pinecone
bootstrap:
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
    go install github.com/deepmap/oapi-codegen/v2/cmd/oapi-codegen@v2.2.0
    go install golang.org/x/tools/cmd/godoc@latest
gen:
  protoc --experimental_allow_proto3_optional --proto_path=apis/proto --go_opt=module="github.com/pinecone-io/go-pinecone" --go-grpc_opt=module="github.com/pinecone-io/go-pinecone" --go_out=. --go-grpc_out=. apis/proto/pinecone/data/v1/vector_service.proto
  oapi-codegen --package=control --generate types,client apis/openapi/control/v1/control_v1.yaml > internal/gen/control/control_plane.oas.go
docs:
  @echo "Serving docs at http://localhost:6060/pkg/github.com/pinecone-io/go-pinecone/pinecone/"
  @godoc -http=:6060 >/dev/null
