## Contributing and development

### Prereqs

1. A [current version of Go](https://go.dev/doc/install) (recommended 1.21+)
2. The [just](https://github.com/casey/just?tab=readme-ov-file#installation) command runner
3. The [protobuf-compiler](https://grpc.io/docs/protoc-installation/)

Then, execute `just bootstrap` to install the necessary Go packages

### .env Setup

An easy way to keep track of necessary environment variables is to create a `.env` file in the root of the project.
This project comes with a sample `.env` file (`.env.sample`) that you can copy and modify. At the very least, you
will need to include the `PINECONE_API_KEY` variable in your `.env` file for the tests to run locally.

````shell
### API Definitions submodule

The API Definitions are in a private submodule. To checkout or update the submodules execute in the root of the project:

```shell
git submodule update --init --recursive
````

For working with submodules, see the [Git Submodules](https://git-scm.com/book/en/v2/Git-Tools-Submodules)
documentation.

### Just commands

`just test` : Executes all tests for the pinecone package

`just gen` : Generates Go client code from the API definitions

`just docs` : Generates Go docs and starts http server on localhost

`just bootstrap` : Installs necessary go packages for gen and docs
