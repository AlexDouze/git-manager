# Git Manager (gitm) Development Guide

## Build Commands
- Build binary: `make build`
- Run tests: `make test`
- Run a single test: `go test ./pkg/git -run TestParseURL`
- Clean build artifacts: `make clean`
- Cross-platform builds: `make build-linux`, `make build-windows`, `make build-macos`

## Code Style

### Organization
- Commands in `cmd/` package
- Core logic in `pkg/` with subpackages: `config/`, `git/`, `tui/`

### Naming Conventions
- Use CamelCase for exported functions/types (public)
- Use camelCase for unexported functions/types (private)
- Prefer descriptive names over abbreviations
- Use consistent naming patterns for similar concepts

### Error Handling
- Use proper error wrapping with `fmt.Errorf("context: %w", err)`
- Return detailed errors with context
- Validate inputs early and return clear error messages

### Testing
- Write table-driven tests using `t.Run` subtest pattern
- Use mock implementations for external dependencies
- Test error conditions explicitly

### Imports
- Group imports: standard library first, then external packages, then internal
- No unused imports or variables