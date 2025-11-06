api_version := "2025-10"

default:
    @just --list

build:
  go build -v ./...
  go vet ./...

build-clean:
  go build -a -v ./...
  go vet ./...

build-pinecone:
  go build -v ./pinecone
  go vet ./pinecone

test:
  #!/usr/bin/env bash
  set -o allexport
  source .env
  set +o allexport
  go test -count=1 -v ./pinecone

test-unit:
    #!/usr/bin/env bash
    set -o allexport
    source .env
    set +o allexport
    go test -v -run Unit ./pinecone

bootstrap:
    go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.32
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.3
    go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.3.0
    go install golang.org/x/tools/cmd/godoc@latest

gen:
  ./codegen/build-clients.sh {{api_version}}

docs:
  @echo "Serving docs at http://localhost:6060/pkg/github.com/pinecone-io/go-pinecone/v4/pinecone/"
  @godoc -http=:6060 >/dev/null
