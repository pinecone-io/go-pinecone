# Contributing

## Prereqs

1. A [current version of Go](https://go.dev/doc/install) (recommended 1.21+)
2. The [just](https://github.com/casey/just?tab=readme-ov-file#installation) command runner
3. The [protobuf-compiler](https://grpc.io/docs/protoc-installation/)

Then, execute `just bootstrap` to install the necessary Go packages

## Environment Setup

At a minimum, you will need to declare a `PINECONE_API_KEY` variable in your environment in order to interact with Pinecone services, and
run the integration tests locally. If `PINECONE_API_KEY` is available in you environment, the `Client` struct can be created with `NewClient`
without any additional configuration parameters. Alternatively, you can pass `ApiKey` as a configuration directly through `NewClientParams`.

````shell
### API Definitions submodule

The API Definitions are in a private submodule. To checkout or update the submodules, execute the following command in the root of the project:

```shell
git submodule update --init --recursive
````

For working with submodules, see the [Git Submodules](https://git-scm.com/book/en/v2/Git-Tools-Submodules)
documentation.

## Just commands

`just test` : Executes all tests (unit & integration) for the pinecone package

`jest test-unit` : Executes unit tests only for the pinecone package

`just gen` : Generates Go client code from the API definitions

`just docs` : Generates Go docs and starts http server on localhost

`just bootstrap` : Installs necessary go packages for gen and docs
