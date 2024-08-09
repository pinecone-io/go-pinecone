# Contributing

## Prereqs

1. A [current version of Go](https://go.dev/doc/install) (recommended 1.21+)
2. The [just](https://github.com/casey/just?tab=readme-ov-file#installation) command runner
3. The [protobuf-compiler](https://grpc.io/docs/protoc-installation/)

Then, execute `just bootstrap` to install the necessary Go packages. The packages installed when bootstrapping allow regenerating client code from spec files, and generating Go documentation.

## Environment Setup

At a minimum, you will need to declare a `PINECONE_API_KEY` variable in your environment in order to interact with Pinecone services, and run integration tests locally. If `PINECONE_API_KEY` is available in your environment, the `Client` struct can be created with `NewClient` without any additional configuration parameters. Alternatively, you can pass `ApiKey` as a configuration directly through `NewClientParams`.

````shell
### API Definitions submodule

The API Definitions are in a private submodule. To checkout or update the submodules, execute the following command in the root of the project:

```shell
git submodule update --init --recursive
````

For working with submodules, see the [Git Submodules](https://git-scm.com/book/en/v2/Git-Tools-Submodules)
documentation. Note that since the current submodule is private to `pinecone-io`, you will not be able to work directly
with the submodule.

## Just commands

`just test` : Executes all tests (unit & integration) for the pinecone package

`jest test-unit` : Executes unit tests only for the pinecone package

`just gen` : Generates Go client code from the API definitions

`just docs` : Generates Go docs and starts http server on localhost

`just bootstrap` : Installs necessary go packages for gen and docs

## Testing

The `go-pinecone` codebase includes both unit and integration tests. These tests are kept within the same files, but are
constructed differently. See `/pinecone/index_connection_test.go` and `/pinecone/client_test.go` for examples. They are divided into sections with `// Integration tests: ` near the top, and `// Unit tests:` near the bottom of the file.

For running tests you can use `just test` to run all tests, and `just test-unit` to only run unit tests.

### Unit tests

Unit tests are generally written using Go's built-in support. You can find a [brief walkthrough](https://go.dev/doc/tutorial/add-a-test) detailing how to write a test. You can also refer to [go.dev/doc/code#Testing](https://go.dev/doc/code#Testing).

When adding unit tests, make sure to postfix `"Unit"` to the test function name in order for the test to be picked up by the `just test-unit` command. For example:

```Go
func TestNewClientParamsSetUnit(t *testing.T) {
	apiKey := "test-api-key"
	client, err := NewClient(NewClientParams{ApiKey: apiKey})

	require.NoError(t, err)
	require.Empty(t, client.sourceTag, "Expected client to have empty sourceTag")
	require.NotNil(t, client.headers, "Expected client headers to not be nil")
	apiKeyHeader, ok := client.headers["Api-Key"]
	require.True(t, ok, "Expected client to have an 'Api-Key' header")
	require.Equal(t, apiKey, apiKeyHeader, "Expected 'Api-Key' header to match provided ApiKey")
	require.Equal(t, 3, len(client.restClient.RequestEditors), "Expected client to have correct number of request editors")
}
```

### Integration Tests

For integration tests we use the `stretchr/testify` module, specifically for the `suite`, `assert`, and `require` packages. You can find the source code and documentation on GitHub: [https://github.com/stretchr/testify](https://github.com/stretchr/testify).

There are two files that define the integration test suite, and include code that manages setup and teardown of external Index resources before and after the integration suites execute.

- `./pinecone/test_suite.go`
- `./pinecone/suite_runner_test.go`

`test_suite.go` includes the definition of the `IntegrationTests` struct which embeds `suite.Suite` from testify. This file also includes `SetupSuite` and `TearDownSuite` methods, along with utility functions for things like index creation and upserting vectors.

`suite_runner_test.go` is the primary entrypoint for the integration tests being run:

```Go
// This is the entry point for all integration tests
// This test function is picked up by go test and triggers the suite runs
func TestRunSuites(t *testing.T) {
	RunSuites(t)
}
```

In `RunSuites` we create two different `IntegrationTests` for pod and serverless indexes.

As mentioned above, integration tests are written in the same files as unit tests. However, integration tests must be defined as methods on the `IntegrationTests` struct:

```Go
type IntegrationTests struct {
	suite.Suite
	apiKey         string
	client         *Client
	host           string
	dimension      int32
	indexType      string
	vectorIds      []string
	idxName        string
	idxConn        *IndexConnection
	collectionName string
	sourceTag      string
}

// Integration tests:
func (ts *IntegrationTests) TestListIndexes() {
	indexes, err := ts.client.ListIndexes(context.Background())
	require.NoError(ts.T(), err)
	require.Greater(ts.T(), len(indexes), 0, "Expected at least one index to exist")
}
```
