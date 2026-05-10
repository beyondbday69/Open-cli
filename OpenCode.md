# OpenCode Project Memory

## Commands
- Build: `go build -o opencode .`
- Test All: `go test ./...`
- Single Test: `go test -v -run TestName ./path/to/package`
- Lint/Format: `go fmt ./... && go vet ./...`

## Code Style
- **Formatting**: Follow standard `gofmt` style.
- **Imports**: Group standard library first, then internal project imports.
- **Naming**: Use `PascalCase` for exported and `camelCase` for unexported identifiers.
- **Error Handling**: Wrap errors using `fmt.Errorf("...: %w", err)`.
- **Logging**: Use the `internal/logging` package for all application logs.
- **Architecture**: Use the Service pattern (interfaces in `internal/...`) for dependencies.
- **Testing**: 
  - Use `github.com/stretchr/testify/assert` and `require`.
  - Use `t.Parallel()` for independent tests and `t.TempDir()` for filesystem tests.
  - Mark helper functions with `t.Helper()`.
