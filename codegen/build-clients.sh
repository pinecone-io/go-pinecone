#!/bin/bash

set -eux -o pipefail

version=$1 # e.g. 2024-07

# data_destination must align with the option go_package: 
# https://github.com/pinecone-io/apis/blob/e9b47c76f649656002f4911946ca6c4c4a6f04fc/src/release/data/data.proto#L3
data_destination="internal/gen/data"
control_destination="internal/gen/control"

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)

echo "Script dir: $script_dir"

update_apis_repo() {
    echo "Updating apis repo"
    pushd codegen/apis
            git fetch
            git checkout main
            git pull
            just build
    popd
}

verify_spec_version() {
    local version=$1
    echo "Verifying spec version has been provided: $version"
    if [ -z "$version" ]; then
        echo "Version is required"
        exit 1
    fi 
}

generate_oas_client() {
    oas_file="codegen/apis/_build/${version}/control_${version}.oas.yaml"

    oapi-codegen --package=control \
    --generate types,client \
    "${oas_file}" > "${control_destination}/control_plane.oas.go"
}

generate_proto_client() {
    proto_file="codegen/apis/_build/${version}/data_${version}.proto"

    protoc --experimental_allow_proto3_optional \
    --proto_path=codegen/apis/vendor/protos \
    --proto_path=codegen/apis/_build/${version} \
    --go_opt=module="github.com/pinecone-io/go-pinecone" \
    --go-grpc_opt=module="github.com/pinecone-io/go-pinecone" \
    --go_out=. \
    --go-grpc_out=. \
    "${proto_file}" 
}

update_apis_repo
verify_spec_version $version

# Generate control plane client code
rm -rf "${control_destination}"
mkdir -p "${control_destination}"

generate_oas_client

# Generate data plane client code
generate_proto_client
rm -rf "${data_destination}"
mkdir -p "${data_destination}"

generate_proto_client