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
- `pkg/tui/` holds Lip Gloss line-output styles for the scriptable subcommands (`status`/`prune`/`update`); `pkg/tui/app/` is the interactive Bubble Tea v2 app launched by bare `gitm`
- Bare `gitm` opens the interactive TUI; the `status`/`prune`/`update`/`clone`/`gh-clone`/`config` subcommands stay scriptable (including `--json`)

### Interactive TUI (`pkg/tui/app/`)
- Bubble Tea v2 (Elm architecture): `Init`/`Update`/`View`; `View()` returns a `tea.View` with `AltScreen = true`
- Slow git operations NEVER run inside `Update` — each is a `tea.Cmd` closure in `commands.go` that returns a result `Msg` (see `messages.go`); `Update` only mutates model state and returns commands
- Charm imports use the canonical `charm.land/...` module paths, not `github.com/charmbracelet/...` (the one exception is `github.com/charmbracelet/colorprofile`, which has no `charm.land` alias)
- Bubble Tea v2 reports the space key as `"space"` (not `" "`) — bind it accordingly in `key.NewBinding`
- On the repo-list screen, capital `U`/`P` are bulk variants of `u`/`p` (update/prune every repo via `workerpool.Map`, one aggregate `bulkOpDoneMsg`); a repo with an action in flight shows a busy label (e.g. "updating…") on its row in place of status badges

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