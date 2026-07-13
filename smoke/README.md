# Smoke tests

## Mocked critical-path gate (`mocked_critical_path_test.go`)

A key-free smoke gate that runs the critical **connect → upsert → query** path
against fully in-process mocks. It runs on **every pull request** (the
`Smoke (mocked, no key)` job in `.github/workflows/ci.yaml`) with no
`PINECONE_API_KEY`, guarding the request/response plumbing against regressions
before any keyed suite runs.

### What is mocked

The mocks are injected at the transport layer, not at the SDK's public API:

- **Control plane** (`DescribeIndex`) — REST; mocked with `net/http/httptest.Server`
  pointed at via `NewClientParams.Host`. The mock returns a describe-index body
  whose `host` field points at the gRPC data plane below.
- **Data plane** (`UpsertVectors`, `QueryByVectorValues`) — gRPC; mocked with a
  real in-process `grpc.Server` implementing `VectorServiceServer` on a loopback
  TCP port. The SDK dials it insecurely (the `http://` scheme on the resolved
  host tells the SDK to skip TLS).

Everything above the wire — config resolution, request building, gRPC
marshaling, response deserialization — is the real code path. This is the Go
analogue of the Python SDK's respx-backed gate
(`tests/smoke/test_mocked_critical_path_*.py`) and the TS SDK's fetchApi-injected
gate (`src/smoke/mockedCriticalPath.test.ts`).

### Run locally

The `smoke` build tag isolates this suite from the default `go test ./pinecone` run:

```sh
go test -tags smoke -run '^TestMockedCriticalPath$' -v -count=1 ./smoke/...
```

### Zero-collection guard

CI pipes the output through `grep '--- PASS: TestMockedCriticalPath'` after the
test run. Go exits 0 even when zero tests match a `-run` pattern, so this grep
is the guard that turns a "suite accidentally emptied" case into a CI failure
rather than a silent green.

### Real (keyed) suites

The keyed integration tests live in `./pinecone` and run in the `build` job with
`PINECONE_API_KEY`, `PINECONE_CLIENT_ID`, and `PINECONE_CLIENT_SECRET` from
secrets. They are gated separately from this key-free gate.
