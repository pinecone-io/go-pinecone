# Contributing

## Prereqs

1. [Go](https://go.dev/doc/install) 1.25 or later (the minimum in `go.mod`)
2. The [just](https://github.com/casey/just?tab=readme-ov-file#installation) command runner
3. The [protobuf-compiler](https://grpc.io/docs/protoc-installation/)

Then, execute `just bootstrap` to install the necessary Go packages. The packages installed when bootstrapping allow regenerating client code from spec files, and generating Go documentation.

## Environment Setup

The `just test` and `just test-unit` commands load environment variables from a `.env` file in the root of the project. The integration tests need all three of these:

```
PINECONE_API_KEY=...
PINECONE_CLIENT_ID=...
PINECONE_CLIENT_SECRET=...
```

`PINECONE_CLIENT_ID` and `PINECONE_CLIENT_SECRET` are a service account's credentials, used by the Admin API tests. To skip a suite, set `PINECONE_SKIP_ADMIN=true` or `PINECONE_SKIP_SERVERLESS=true`.

If `PINECONE_API_KEY` is available in your environment, the `Client` struct can be created with `NewClient` without any additional configuration parameters. Alternatively, you can pass `ApiKey` as a configuration directly through `NewClientParams`.

### API Definitions submodule

The API Definitions are in a private submodule at `codegen/apis`. To checkout or update the submodules, execute the following command in the root of the project:

```shell
git submodule update --init --recursive
```

For working with submodules, see the [Git Submodules](https://git-scm.com/book/en/v2/Git-Tools-Submodules)
documentation. Note that since the current submodule is private to `pinecone-io`, you will not be able to work directly
with the submodule.

## Just commands

`just build` : Builds all packages and runs `go vet`

`just test` : Executes all tests (unit & integration) for the pinecone package

`just test-unit` : Executes unit tests only for the pinecone package

`just gen` : Generates Go client code from the API definitions

`just docs` : Generates Go docs and starts http server on localhost

`just bootstrap` : Installs necessary go packages for gen and docs

## Testing

The `go-pinecone` codebase includes both unit and integration tests. These tests are kept within the same files, but are
constructed differently. See `/pinecone/index_connection_test.go` and `/pinecone/client_test.go` for examples. They are divided into sections with `// Integration tests:` near the top, and `// Unit tests:` near the bottom of the file.

For running tests you can use `just test` to run all tests, and `just test-unit` to only run unit tests.

### Unit tests

Unit tests are generally written using Go's built-in support. You can find a [brief walkthrough](https://go.dev/doc/tutorial/add-a-test) detailing how to write a test. You can also refer to [go.dev/doc/code#Testing](https://go.dev/doc/code#Testing).

When adding unit tests, make sure to postfix `"Unit"` to the test function name in order for the test to be picked up by the `just test-unit` command. For example:

```Go
func TestNewClientParamsSetUnit(t *testing.T) {
	apiKey := "test-api-key"
	client, err := NewClient(NewClientParams{ApiKey: apiKey})

	require.NoError(t, err)
	require.Empty(t, client.baseParams.SourceTag, "Expected client to have empty sourceTag")
	require.NotNil(t, client.baseParams.Headers, "Expected client headers to not be nil")
	apiKeyHeader, ok := client.baseParams.Headers["Api-Key"]
	require.True(t, ok, "Expected client to have an 'Api-Key' header")
	require.Equal(t, apiKey, apiKeyHeader, "Expected 'Api-Key' header to match provided ApiKey")
	require.Equal(t, 3, len(client.restClient.RequestEditors), "Expected client to have correct number of request editors")
}
```

`pinecone/conformance_2026_07_test.go` holds unit tests that check the requests the SDK sends against the `2026-07` API shapes.

### Integration Tests

For integration tests we use the `stretchr/testify` module, specifically for the `suite`, `assert`, and `require` packages. You can find the source code and documentation on GitHub: [https://github.com/stretchr/testify](https://github.com/stretchr/testify).

There are two files that define the integration test suite, and include code that manages setup and teardown of external Index resources before and after the integration suites execute.

- `./pinecone/test_suite.go`
- `./pinecone/suite_runner_test.go`

`test_suite.go` includes the definitions of the `integrationTests` and `adminIntegrationTests` structs, which embed `suite.Suite` from testify. This file also includes `SetupSuite` and `TearDownSuite` methods, along with utility functions for things like index creation and upserting vectors.

`suite_runner_test.go` is the primary entrypoint for the integration tests being run:

```Go
// This is the entry point for all integration tests
// This test function is picked up by go test and triggers the suite runs
func TestRunSuites(t *testing.T) {
	RunSuites(t)
}
```

In `RunSuites` we create a serverless index and run two suites: an `integrationTests` suite against that index, and an `adminIntegrationTests` suite for the Admin API. There's no pod-based suite, because API version `2026-07` can't create pod-based indexes.

As mentioned above, integration tests are written in the same files as unit tests. However, integration tests must be defined as methods on the `integrationTests` (or `adminIntegrationTests`) struct:

```Go
type integrationTests struct {
	suite.Suite
	apiKey    string
	client    *Client
	host      string
	dimension *int32
	idxName   string
	idxConn   *IndexConnection
	// ...
}

// Integration tests:
func (ts *integrationTests) TestListIndexes() {
	indexes, err := ts.client.ListIndexes(context.Background())
	require.NoError(ts.T(), err)
	require.Greater(ts.T(), len(indexes), 0, "Expected at least one index to exist")
}
```

### Other tests

Two more suites are behind build tags, so `go test ./pinecone` doesn't run them. CI runs both on every pull request (see `.github/workflows/ci.yaml`).

- **Local server tests** (`pinecone/local_test.go`, `localServer` tag) run the data plane against [Pinecone Local](https://docs.pinecone.io/guides/operations/local-development) index containers. Start `ghcr.io/pinecone-io/pinecone-index` containers as in `ci.yaml`, then run:

  ```shell
  PINECONE_INDEX_URL_POD=http://localhost:5082 PINECONE_INDEX_URL_SERVERLESS=http://localhost:5081 PINECONE_DIMENSION=1536 \
    go test -count=1 -v ./pinecone -run TestRunLocalIntegrationSuite -tags=localServer
  ```

- **Mocked smoke test** (`smoke/`, `smoke` tag) runs the connect, upsert, and query path against in-process mocks, with no API key. See `smoke/README.md`.

### Examples

`pinecone/example_test.go` holds the examples shown on pkg.go.dev, including the README quickstarts. They have no `// Output:` comment, so `go test` compiles them but doesn't run them. When you change a README quickstart, update the matching example too.

## Documentation

User-facing documentation lives in `README.md` and the [`guides/`](./guides/README.md) directory, with the API reference generated from GoDoc comments. The guides and GoDoc comments follow the terminology of the [Pinecone docs](https://docs.pinecone.io/), such as the Documents API, Vectors API, and Records API, "integrated embedding", and "an index that stores dense vectors" rather than "a dense index". Keep examples in the guides compiling against the current SDK.
