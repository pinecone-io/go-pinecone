# Pinecone Go SDK - AI Assistant Guide

**Project:** Pinecone Go SDK — Vector database client for AI applications
**Go:** 1.21+
**Module:** `github.com/pinecone-io/go-pinecone/v5`

## Project Overview

The Pinecone Go SDK provides a client for interacting with Pinecone vector databases. Built for type safety, correctness, and idiomatic Go.

**Entry points:**
- `Client` — Control plane client for managing indexes, collections, backups, and inference
- `AdminClient` — Admin operations (projects, orgs, API keys)
- `IndexConnection` — Data plane client for vector operations on a specific index

**Key technologies:**
- Standard `net/http` for REST API calls (Control Plane, Inference, Admin)
- `google.golang.org/grpc` for Data Plane operations
- Auto-generated clients from OpenAPI/Protobuf specs via `oapi-codegen` and `protoc-gen-go`
- `testify` for testing (assert, require, suite)

## Architecture

Three-plane design, each with its own client struct:

**Public API (`pinecone/`):**
- `client.go` — `Client`: Control Plane (create/list/describe/delete indexes, collections, backups) + Inference
- `admin_client.go` — `AdminClient`: Admin operations (projects, orgs, API keys)
- `index_connection.go` — `IndexConnection`: Data Plane over gRPC (upsert, query, fetch, delete, update vectors; namespace management)
- `models.go` — All shared types, constants, and enums (`IndexMetric`, `Cloud`, `IndexStatus`, etc.)
- `errors.go` — `PineconeError` error type

**Internal (`internal/`):**
- `internal/gen/` — Auto-generated client code. **Never edit manually.** Regenerate with `just gen`.
  - `db_control/` — REST client for Control Plane
  - `db_data/grpc/` — gRPC bindings for Data Plane
  - `db_data/rest/` — REST bindings for Data Plane
  - `inference/` — Inference API client
  - `admin/` — Admin API client
- `internal/provider/` — Auth/header provider utilities
- `internal/useragent/` — User-agent string construction
- `internal/utils/` — Internal utility functions

**Code Generation:**
Generated code under `internal/gen/` is produced by `codegen/build-clients.sh` from API specs in `codegen/apis/` (a private git submodule). Run `just gen` to regenerate after spec changes. The current API version is `2025-10`.

## Build & Test Commands

**Setup:**
```bash
# Install required codegen tools (protoc-gen-go, oapi-codegen, godoc)
just bootstrap

# Initialize git submodules (internal Pinecone employees only)
git submodule update --init --recursive
```

**Note:** The `codegen/apis/` submodule contains internal OpenAPI/Protobuf specifications accessible only to Pinecone employees. It is not required for SDK development.

**Build:**
```bash
just build        # go build -v ./... && go vet ./...
just build-clean  # clean build with -a flag
```

**Test:**
```bash
# Unit tests only (fast, no external dependencies)
just test-unit    # go test -v -run Unit ./pinecone

# All tests (requires .env with credentials)
just test         # go test -count=1 -v ./pinecone

# Single test by name
go test -v -run TestNameHere ./pinecone/...
```

**Integration tests require `.env`:**
```
PINECONE_API_KEY=...
PINECONE_CLIENT_ID=...
PINECONE_CLIENT_SECRET=...
```

**Documentation:**
```bash
just docs   # godoc server at http://localhost:6060/pkg/github.com/pinecone-io/go-pinecone/v5/pinecone/
```

**Code generation:**
```bash
just gen    # regenerate internal/gen/ from API specs
```

## Code Style & Conventions

**Quick summary:**

- **Formatting:** Always run `gofmt` (or `go fmt ./...`). All Go code must be gofmt-compliant; non-negotiable.
- **Vetting:** `go vet ./...` before committing. Zero warnings allowed.
- **Naming:** `PascalCase` for exported identifiers; `camelCase` for unexported. Acronyms stay uppercase (`URL`, `HTTP`, `API`, `ID`). Short contextual names in function bodies (`ctx`, `err`, `idx`) are idiomatic.
- **Comments:** All exported types, functions, constants, and variables require GoDoc comments. The comment must start with the name of the symbol (e.g., `// Client holds the parameters...`).
- **Errors:** Return errors rather than panicking. Use `fmt.Errorf("context: %w", err)` to wrap with context. Use `errors.Is()` and `errors.As()` for inspection.
- **Context:** Always accept `context.Context` as the first parameter in functions that perform I/O or long-running operations.
- **Testing:** Unit tests use `*testing.T` with `testify/assert` and `testify/require`. Unit test names must end with `Unit` (e.g., `TestNewClientParamsSetUnit`). Integration tests are methods on the `integrationTests` suite.
- **Generated code:** Never modify `internal/gen/` directly. Regenerate with `just gen`.

**Key patterns:**
- `Client` instances named `pc` by convention in examples and tests
- Option parameters use dedicated request structs (e.g., `CreateServerlessIndexRequest`)
- Public API methods accept `context.Context` as first parameter
- Input validation occurs before API calls; return descriptive `PineconeError` with context

**Module management:**
```bash
go get <package>    # add dependency
go mod tidy         # clean unused dependencies and update go.sum
```

## Security Considerations

**Always:**
- Redact API keys in logs and error messages — never include raw credentials in error output
- Keep TLS verification enabled for gRPC and HTTP connections
- Validate user input before API calls; return clear errors with context

**Pre-commit:**
- Run `go build ./...` and `go vet ./...` before committing

## Development Workflows

**Scratch directory for temporary files:**

The `scratch/` directory is gitignored and designated for agent-generated temporary files:
- Test scripts and experimental code
- Debug utilities and proof-of-concepts
- Temporary markdown reports and analysis outputs

**Always place temporary/experimental files in `scratch/` rather than the project root.** This keeps the working tree clean and prevents accidental commits.

**Planning multi-step work:**

Save plans to `scratch/plans/YYYY-MM-DD-<feature-name>.md` with bite-sized tasks, exact file paths, and verification steps.

## Specialized Agent Review

This repository uses specialized agents for domain-specific code review. Suggest invoking agents when relevant to the user's changes.

## Available Agents

Agents are invoked with `@agent-name` syntax. Agent definitions live in `.cursor/agents/`.

### @test-runner
- **Checks:** Build succeeds, unit tests pass, `go vet` clean
- **Suggest when:** After any code change, before committing, when tests are failing

### @godoc-review
- **Checks:** Exported symbols have GoDoc comments, comments start with the symbol name, examples are accurate and compile, struct fields are documented
- **Suggest when:** User adds exported types/functions, changes method signatures, refactors public API

### @error-handling-review
- **Checks:** Errors returned (not panicked), `%w` wrapping used (not `%v`), `PineconeError` used for user-facing errors, clear error messages, no swallowed errors, `errors.Is()`/`errors.As()` for inspection
- **Suggest when:** User adds error handling, changes error paths, adds new I/O functions

### @security-review
- **Checks:** Credential exposure in logs/errors, TLS/gRPC security, input validation, secrets not committed
- **Suggest when:** User adds authentication, changes logging/errors, handles user-provided data

### @breaking-changes-review
- **Checks:** Removed exported symbols, changed function signatures, changed struct field types, changed return types, changed behavior of existing methods
- **Suggest when:** Modifying public APIs, removing features, changing method signatures

### @ux-review
- **Checks:** Context propagation (first param), option structs for complex params, GoDoc quality, consistent naming, clear error messages, idiomatic Go
- **Suggest when:** User adds public APIs, designs new functions, finalizes API changes

### @dependency-review
- **Checks:** New dependency justified, `go mod tidy` run, license compatibility, transitive dependency weight
- **Suggest when:** Adding new `go get` dependencies, updating packages

## Agent Workflows

**Basic:** `@test-runner` before committing

**Feature development:**
1. Implement following conventions
2. `@error-handling-review` (if adding error handling), `@godoc-review` (if changing public API)
3. `@security-review` (if handling credentials or user input)
4. `@test-runner`, `@ux-review`, `@breaking-changes-review` (if modifying public API)
5. Commit

**Bug fix:** `@test-runner` → fix → `@security-review` (if relevant) → `@test-runner` → commit

**New dependency:** `@dependency-review` → get user approval → `go get` → `go mod tidy` → `@test-runner` → commit

## Quick Reference

| Task | Agent |
|------|-------|
| Build and test verification | `@test-runner` |
| GoDoc comment quality | `@godoc-review` |
| Error handling patterns | `@error-handling-review` |
| Security vulnerabilities | `@security-review` |
| Breaking API changes | `@breaking-changes-review` |
| API usability and ergonomics | `@ux-review` |
| New dependencies | `@dependency-review` |

---

*Last audited: February 17, 2026*
