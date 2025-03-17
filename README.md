# gitm - Git Repository Manager

`gitm` is a CLI tool for managing multiple git repositories from different hosts (GitHub, GitLab, etc.) with a structured folder hierarchy.

## Features

- **Structured Repository Organization**: Automatically organizes repositories in a structured directory hierarchy (`<root-directory>/<host>/<organization>/<repository>`).
- **Multi-Repository Management**: Manage multiple git repositories with a single command.
- **Repository Status**: Check the status of repositories, showing uncommitted changes, branch status, and other important information.
- **Repository Updates**: Update repositories by fetching and optionally pulling the latest changes.
- **Configuration Management**: Easily configure and customize the behavior of the tool.

## Installation

### Using Homebrew

You can install `gitm` using Homebrew:

```bash
brew install alexdouze/tap/gitm
```

### From Source

1. Clone the repository:
   ```bash
   git clone https://github.com/alexDouze/gitm.git
   cd gitm
   ```

2. Build the binary:
   ```bash
   make build
   ```

3. Install the binary (optional):
   ```bash
   # Copy the binary to a directory in your PATH
   cp bin/gitm /usr/local/bin/
   ```

### Cross-Platform Builds

The Makefile includes targets for building on different platforms:

- Linux: `make build-linux`
- Windows: `make build-windows`
- macOS: `make build-macos`

### Releases

Releases are automatically built and published using GitHub Actions and GoReleaser when a new tag is pushed to the repository.

To create a new release:

1. Tag the commit with a semantic version:
   ```bash
   git tag -a v1.0.0 -m "Release v1.0.0"
   ```

2. Push the tag to GitHub:
   ```bash
   git push origin v1.0.0
   ```

3. GitHub Actions will automatically build the binaries for all supported platforms and create a new release on GitHub with the artifacts.

You can also run GoReleaser locally for testing (without publishing):

```bash
goreleaser release --snapshot --clean
```

## Configuration

`gitm` uses a configuration file located at `$HOME/.gitm.yaml` by default. You can initialize the configuration with:

```bash
gitm config init
```

### Configuration Options

- `rootDirectory`: The root directory where repositories are stored (default: `$HOME/Codebase`)
- `clone.defaultOptions`: Default options for the git clone command (default: `--recurse-submodules`)

### Viewing and Modifying Configuration

```bash
# View all configuration
gitm config get

# View a specific configuration value
gitm config get rootDirectory

# Set a configuration value
gitm config set rootDirectory /path/to/your/codebase
gitm config set clone.defaultOptions "--depth 1"
```

## Usage

### Cloning Repositories

Clone a repository into the structured directory hierarchy:

```bash
gitm clone https://github.com/username/repository.git
```

This will clone the repository to `<root-directory>/github.com/username/repository`.

You can also specify a different root directory for a specific clone operation:

```bash
gitm clone https://github.com/username/repository.git --root-dir /path/to/directory
```

### Checking Repository Status

Check the status of repositories:

```bash
# Check all repositories
gitm status --all

# Filter by host
gitm status --host github.com

# Filter by organization
gitm status --org username

# Filter by repository name
gitm status --repo repository

# Filter by path
gitm status --path /path/to/repository
```

The status command shows:
- Uncommitted changes
- Branch information (current branch, remote tracking)
- Branches that are ahead/behind their remote counterparts (with commit count)
- Branches with remote gone (with branch names)
- Branches without remote tracking (with branch names)
- Stash information

### Updating Repositories

Update repositories by fetching and optionally pulling the latest changes:

```bash
# Update all repositories (fetch only)
gitm update --all

# Update with pruning remote-tracking branches
gitm update --all --prune

# Filter by host, organization, or repository name
gitm update --host github.com --org username --repo repository
```

### Pruning Branches

Prune local branches that meet specified criteria (gone remotes or merged):

```bash
# Show branches that would be pruned in all repositories (dry run)
gitm prune --all --gone-only

# Prune branches with gone remotes only (actually delete)
gitm prune --all --gone-only --no-dry-run

# Prune merged branches only
gitm prune --all --merged-only

# Prune both gone and merged branches
gitm prune --all --gone-only --merged-only

# Filter by host, organization, or repository name
gitm prune --host github.com --org username --repo repository --gone-only
```

## Project Structure

```
gitm/
├── cmd/                # Command implementations
│   ├── clone.go        # Clone command
│   ├── config.go       # Configuration command
│   ├── gh-clone.go     # GitHub clone command
│   ├── prune.go        # Branch pruning command
│   ├── root.go         # Root command
│   ├── status.go       # Status command
│   ├── update.go       # Update command
│   └── version.go      # Version command
├── pkg/                # Package code
│   ├── config/         # Configuration handling
│   ├── git/            # Git operations
│   └── tui/            # Terminal UI components
├── main.go             # Entry point
└── Makefile            # Build instructions
```
