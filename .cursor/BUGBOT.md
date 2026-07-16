# Code Review Guidance

## Quick Reference

Verify: code builds, unit tests pass, `go vet` clean, GoDoc updated, no generated code edits, proper error handling, input validation, adequate test coverage.

## Code Quality Review

### Documentation
- Update GoDoc comments near changed code to ensure they are up to date
- All exported types, functions, constants, and variables require GoDoc comments
- Comment must start with the name of the symbol (e.g., `// Client holds the parameters...`)
- Include usage examples in GoDoc using `Example` functions for public APIs
- Document struct field semantics and whether they are required vs. optional

### Error Handling
- Return errors; do not panic except for truly unrecoverable states
- Wrap errors with context using `fmt.Errorf("doing X: %w", err)` — use `%w` (not `%v`) to preserve the error chain
- Inspect errors with `errors.Is()` and `errors.As()`; never match errors by string comparison
- Do not swallow errors with empty `if err != nil {}` blocks
- Use `PineconeError` for SDK-level errors surfaced to users

### Validation
- Validate input parameters early in public methods, returning descriptive errors
- Error messages should include the invalid value and what was expected
- Do not validate API responses — trust the API

### Code Generation
- Never manually edit files in `internal/gen/` — these are auto-generated
- If changes are needed, update the source OpenAPI/Proto files in `codegen/` and run `just gen`

### Testing
- Unit tests: use `*testing.T` with `testify/require`/`testify/assert`, no external dependencies
- Unit test function names must end with `Unit` (e.g., `TestNewClientParamsSetUnit`)
- Integration tests: implement as methods on the `integrationTests` suite
- Test both success and failure cases, including edge cases
- Use descriptive test names that clearly indicate what is being tested

### Concurrency
- All public methods accept `context.Context` as the first parameter for cancellation/deadlines
- Do not store `context.Context` in struct fields — pass it as a parameter
- Client methods are safe for concurrent use; document if any are not

### Resource Management
- Close gRPC connections (`IndexConnection`) when done
- Use `defer` for cleanup in functions that acquire resources
- Ensure proper cleanup runs in error paths

## Pull Request Review

### PR Title and Description
- Title: Use Conventional Commits format (e.g., `fix: handle nil pointer in query method`)
- Description: Clear problem and solution, link to related issue, Go code examples showing usage

### Code Changes
- Scope: Single concern per PR
- Backward compatibility: Maintain when possible; document and justify breaking changes
- Dependencies: No unnecessary additions; justify new `go get` entries and run `go mod tidy`

## Common Issues

- Missing GoDoc on exported symbols
- Using `panic` instead of returning an error
- Wrapping errors with `%v` instead of `%w` (loses error chain for `errors.Is()`/`errors.As()`)
- String-matching errors instead of using `errors.Is()`/`errors.As()`
- Missing `context.Context` as first parameter in I/O-performing functions
- Editing `internal/gen/` directly instead of regenerating with `just gen`
- Missing `Unit` suffix on unit test function names
- Integration tests not using the `integrationTests` suite pattern
- Unused variables or imports (code will not compile in Go)
- Exported types/functions missing GoDoc comments
- Not running `go mod tidy` after adding/removing dependencies
- Credentials or API keys in log statements or error messages
- `errors.New(fmt.Sprintf(...))` instead of `fmt.Errorf(...)`

## Review Focus by Change Type

**Bug Fixes**: Address root cause, not symptoms. No regressions. Handle related edge cases. Tests demonstrate fix.

**New Features**: Complete and functional. Well-designed public API with GoDoc. Tests cover happy path and errors. No breaking changes unless intentional.

**Refactoring**: Behavior unchanged (verify with tests). More idiomatic Go. No performance regressions. Update GoDoc if needed.

**Performance Improvements**: Measurable and significant. No correctness regressions. Tests verify improvement. Document trade-offs.
