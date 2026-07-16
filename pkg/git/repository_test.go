package git

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// MockGitCommandExecutor is a mock implementation of GitCommandExecutor for testing
type MockGitCommandExecutor struct {
	// ExecuteFunc allows tests to define custom behavior for the Execute method
	ExecuteFunc func(ctx context.Context, repoPath string, stdout bool, args ...string) ([]byte, error)
}

// TestRepository is a wrapper around Repository that overrides filesystem checks for testing
type TestRepository struct {
	*Repository
	pathExists bool
}

// NewTestRepository creates a new test repository with mocked filesystem checks
func NewTestRepository() *TestRepository {
	return &TestRepository{
		Repository: NewRepository(),
		pathExists: true, // Default to true for testing
	}
}

// Status overrides the original Status method to bypass filesystem checks
func (r *TestRepository) Status(ctx context.Context) (*RepositoryStatus, error) {
	status := &RepositoryStatus{
		Repository: r.Repository,
	}

	// Skip the filesystem check that would normally happen in Repository.Status()
	// and only perform it if we explicitly want to test that case
	if !r.pathExists {
		return nil, fmt.Errorf("repository path does not exist: %s", r.Path)
	}

	// Get uncommitted changes
	if err := r.getUncommittedChanges(ctx, status); err != nil {
		return nil, fmt.Errorf("failed to get uncommitted changes: %w", err)
	}

	// Get branch information
	if err := r.getBranchInformation(ctx, status); err != nil {
		return nil, fmt.Errorf("failed to get branch information: %w", err)
	}

	// Check for stashes
	if err := r.getStashInformation(ctx, status); err != nil {
		return nil, fmt.Errorf("failed to get stash information: %w", err)
	}

	return status, nil
}

// Execute calls the mock's ExecuteFunc if defined, or returns a default response
func (m *MockGitCommandExecutor) Execute(ctx context.Context, repoPath string, stdout bool, args ...string) ([]byte, error) {
	if m.ExecuteFunc != nil {
		return m.ExecuteFunc(ctx, repoPath, stdout, args...)
	}
	return []byte("mock response"), nil
}

func TestRepositoryWithMockExecutor(t *testing.T) {
	// Create a new test repository
	repo := NewTestRepository()

	// Create a mock executor with custom behavior
	mockExecutor := &MockGitCommandExecutor{
		ExecuteFunc: func(ctx context.Context, repoPath string, stdout bool, args ...string) ([]byte, error) {
			// Check if the command is "status"
			if len(args) > 0 && args[0] == "status" {
				return []byte("M  README.md"), nil
			}

			// Branch information now comes from for-each-ref
			if len(args) > 0 && args[0] == "for-each-ref" {
				return []byte(refLine("main", "*", "", "", "", "") + "\n" +
					refLine("feature", "", "", "", "", "")), nil
			}

			// Check if the command is "stash list"
			if len(args) > 1 && args[0] == "stash" && args[1] == "list" {
				return []byte(""), nil
			}

			// Simulate an error for other commands
			return nil, errors.New("command not supported in mock")
		},
	}

	// Set the mock executor on the repository
	repo.SetGitCommandExecutor(mockExecutor)

	// Test repository with mock
	repo.Path = "/mock/path"

	// Test Status method with mock
	status, err := repo.Status(context.Background())
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify the mock was used correctly
	if !status.HasUncommittedChanges {
		t.Error("Expected uncommitted changes, got none")
	}

	if len(status.UncommittedChanges) != 1 || status.UncommittedChanges[0] != "M  README.md" {
		t.Errorf("Expected 'M  README.md' as uncommitted change, got: %v", status.UncommittedChanges)
	}
}

func TestRepositoryWithErrorMock(t *testing.T) {
	// Create a new test repository
	repo := NewTestRepository()

	// Create a mock executor that always returns an error
	mockExecutor := &MockGitCommandExecutor{
		ExecuteFunc: func(ctx context.Context, repoPath string, stdout bool, args ...string) ([]byte, error) {
			return nil, errors.New("mock error")
		},
	}

	// Set the mock executor on the repository
	repo.SetGitCommandExecutor(mockExecutor)

	// Test repository with mock
	repo.Path = "/mock/path"

	// Test Status method with mock
	_, err := repo.Status(context.Background())
	if err == nil {
		t.Fatal("Expected an error, got nil")
	}
}

func TestRepositoryWithNonExistentPath(t *testing.T) {
	// Create a new test repository
	repo := NewTestRepository()

	// Set pathExists to false to simulate a non-existent path
	repo.pathExists = false

	// Test repository with mock
	repo.Path = "/non/existent/path"

	// Test Status method with mock
	_, err := repo.Status(context.Background())
	if err == nil {
		t.Fatal("Expected an error for non-existent path, got nil")
	}

	if err.Error() != "repository path does not exist: /non/existent/path" {
		t.Errorf("Expected path not exist error, got: %v", err)
	}
}

func TestParseURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		want    *Repository
		wantErr bool
	}{
		{
			name: "SSH URL",
			url:  "git@github.com:octocat/hello-world.git",
			want: &Repository{
				Host:         "github.com",
				Organization: "octocat",
				Name:         "hello-world",
			},
			wantErr: false,
		},
		{
			name: "SSH URL without .git",
			url:  "git@github.com:octocat/hello-world",
			want: &Repository{
				Host:         "github.com",
				Organization: "octocat",
				Name:         "hello-world",
			},
			wantErr: false,
		},
		{
			name: "HTTPS URL",
			url:  "https://github.com/octocat/hello-world.git",
			want: &Repository{
				Host:         "github.com",
				Organization: "octocat",
				Name:         "hello-world",
			},
			wantErr: false,
		},
		{
			name: "HTTPS URL without .git",
			url:  "https://github.com/octocat/hello-world",
			want: &Repository{
				Host:         "github.com",
				Organization: "octocat",
				Name:         "hello-world",
			},
			wantErr: false,
		},
		{
			name: "HTTP URL",
			url:  "http://github.com/octocat/hello-world",
			want: &Repository{
				Host:         "github.com",
				Organization: "octocat",
				Name:         "hello-world",
			},
			wantErr: false,
		},
		{
			name:    "Invalid SSH URL",
			url:     "git@github.com",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Invalid HTTPS URL",
			url:     "https://github.com",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Unsupported URL format",
			url:     "github.com/octocat/hello-world",
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if got.Host != tt.want.Host {
				t.Errorf("ParseURL() Host = %v, want %v", got.Host, tt.want.Host)
			}
			if got.Organization != tt.want.Organization {
				t.Errorf("ParseURL() Organization = %v, want %v", got.Organization, tt.want.Organization)
			}
			if got.Name != tt.want.Name {
				t.Errorf("ParseURL() Name = %v, want %v", got.Name, tt.want.Name)
			}
		})
	}
}

func TestClone(t *testing.T) {
	// Create a repository for testing
	repo, err := ParseURL("git@github.com:octocat/hello-world.git")
	if err != nil {
		t.Fatalf("Failed to parse URL: %v", err)
	}

	// Test successful clone
	t.Run("Successful clone", func(t *testing.T) {
		mockExecutor := &MockGitCommandExecutor{
			ExecuteFunc: func(ctx context.Context, repoPath string, stdout bool, args ...string) ([]byte, error) {
				// Verify the clone command is correct
				if len(args) < 2 || args[0] != "clone" {
					t.Errorf("Expected clone command, got: %v", args)
				}
				return []byte("Cloning into 'hello-world'..."), nil
			},
		}

		repo.SetGitCommandExecutor(mockExecutor)
		err := repo.Clone(context.Background(), "/tmp", "git@github.com:octocat/hello-world.git", []string{})
		if err != nil {
			t.Errorf("Clone() error = %v, want nil", err)
		}

		// Verify the path is set correctly
		expectedPath := "/tmp/github.com/octocat/hello-world"
		if repo.Path != expectedPath {
			t.Errorf("Clone() path = %v, want %v", repo.Path, expectedPath)
		}
	})

	// Test clone with options
	t.Run("Clone with options", func(t *testing.T) {
		mockExecutor := &MockGitCommandExecutor{
			ExecuteFunc: func(ctx context.Context, repoPath string, stdout bool, args ...string) ([]byte, error) {
				// Verify the clone command includes the options
				if len(args) < 3 || args[0] != "clone" || args[1] != "--depth=1" {
					t.Errorf("Expected clone command with --depth=1 option, got: %v", args)
				}
				return []byte("Cloning into 'hello-world'..."), nil
			},
		}

		repo.SetGitCommandExecutor(mockExecutor)
		err := repo.Clone(context.Background(), "/tmp", "git@github.com:octocat/hello-world.git", []string{"--depth=1"})
		if err != nil {
			t.Errorf("Clone() error = %v, want nil", err)
		}
	})

	// Test clone error
	t.Run("Clone error", func(t *testing.T) {
		mockExecutor := &MockGitCommandExecutor{
			ExecuteFunc: func(ctx context.Context, repoPath string, stdout bool, args ...string) ([]byte, error) {
				return nil, errors.New("clone failed")
			},
		}

		repo.SetGitCommandExecutor(mockExecutor)
		err := repo.Clone(context.Background(), "/tmp", "git@github.com:octocat/hello-world.git", []string{})
		if err == nil {
			t.Error("Clone() error = nil, want error")
		}
	})
}

func TestUpdate(t *testing.T) {
	// All subtests use /tmp so os.Stat passes in Repository.Status().
	// Mocks handle all git commands needed by the real Update() method.
	newRepo := func() *TestRepository {
		r := NewTestRepository()
		r.Path = "/tmp"
		return r
	}

	// stdMock builds a mock executor with configurable responses per command.
	// branchOutput is a for-each-ref formatted string (NUL-separated fields,
	// newline-separated branches) as produced by refLine.
	stdMock := func(
		revParse string,
		fetchErr error,
		statusOutput string,
		branchOutput string,
		pullErr error,
	) *MockGitCommandExecutor {
		return &MockGitCommandExecutor{
			ExecuteFunc: func(ctx context.Context, repoPath string, stdout bool, args ...string) ([]byte, error) {
				if len(args) == 0 {
					return nil, fmt.Errorf("unexpected empty command")
				}
				switch args[0] {
				case "rev-parse":
					return []byte(revParse), nil
				case "fetch":
					return []byte(""), fetchErr
				case "status":
					return []byte(statusOutput), nil
				case "for-each-ref":
					return []byte(branchOutput), nil
				case "stash":
					if len(args) > 1 && args[1] == "list" {
						return []byte(""), nil
					}
				case "pull":
					return []byte(""), pullErr
				case "checkout":
					return []byte(""), nil
				}
				return nil, fmt.Errorf("unexpected command: %v", args)
			},
		}
	}

	t.Run("Fetch only", func(t *testing.T) {
		repo := newRepo()
		repo.SetGitCommandExecutor(stdMock("main", nil, "", refLine("main", "*", "origin/main", "", "", ""), nil))
		result, err := repo.Update(context.Background(), true, false)
		if err != nil {
			t.Errorf("Update() error = %v, want nil", err)
		}
		if result == nil {
			t.Error("Update() result = nil, want non-nil")
		}
		if result != nil && len(result.BranchUpdateResults) != 0 {
			t.Errorf("Update() BranchUpdateResults = %v, want empty (fetch-only)", result.BranchUpdateResults)
		}
	})

	t.Run("Fetch with prune", func(t *testing.T) {
		repo := newRepo()
		var seenPrune, seenAll bool
		repo.SetGitCommandExecutor(&MockGitCommandExecutor{
			ExecuteFunc: func(ctx context.Context, repoPath string, stdout bool, args ...string) ([]byte, error) {
				if args[0] == "fetch" {
					for _, a := range args {
						if a == "--prune" {
							seenPrune = true
						}
						if a == "--all" {
							seenAll = true
						}
					}
					return []byte(""), nil
				}
				return stdMock("main", nil, "", refLine("main", "*", "origin/main", "", "", ""), nil).ExecuteFunc(ctx, repoPath, stdout, args...)
			},
		})
		_, err := repo.Update(context.Background(), true, true)
		if err != nil {
			t.Errorf("Update() error = %v, want nil", err)
		}
		if !seenPrune {
			t.Error("expected fetch to include --prune")
		}
		if !seenAll {
			t.Error("expected fetch to include --all")
		}
	})

	t.Run("Fetch and pull branch behind", func(t *testing.T) {
		repo := newRepo()
		// feature is behind remote — pull should be called
		repo.SetGitCommandExecutor(stdMock("main", nil, "",
			refLine("main", "*", "origin/main", "", "", "")+"\n"+
				refLine("feature", "", "origin/feature", "[behind 2]", "", ""), nil))
		result, err := repo.Update(context.Background(), false, false)
		if err != nil {
			t.Errorf("Update() error = %v, want nil", err)
		}
		if result == nil {
			t.Error("Update() result = nil, want non-nil")
		} else if result.HasErrors {
			t.Errorf("Update() result.HasErrors = true, want false")
		}
	})

	t.Run("Update with uncommitted changes", func(t *testing.T) {
		repo := newRepo()
		repo.SetGitCommandExecutor(stdMock("main", nil, "M  README.md", refLine("main", "*", "origin/main", "", "", ""), nil))
		_, err := repo.Update(context.Background(), false, false)
		if err == nil {
			t.Error("Update() error = nil, want error about uncommitted changes")
		}
	})

	t.Run("Fetch error", func(t *testing.T) {
		repo := newRepo()
		repo.SetGitCommandExecutor(stdMock("main", errors.New("fetch failed"), "", refLine("main", "*", "origin/main", "", "", ""), nil))
		_, err := repo.Update(context.Background(), true, false)
		if err == nil {
			t.Error("Update() error = nil, want fetch error")
		}
	})

	t.Run("Pull error", func(t *testing.T) {
		repo := newRepo()
		// main is current and behind — pull is invoked and fails
		repo.SetGitCommandExecutor(stdMock("main", nil, "",
			refLine("main", "*", "origin/main", "[behind 1]", "", ""), errors.New("pull failed")))
		result, err := repo.Update(context.Background(), false, false)
		if err != nil {
			t.Errorf("Update() unexpected top-level error = %v", err)
		}
		if result == nil {
			t.Error("Update() result = nil, want non-nil")
		} else if !result.HasErrors {
			t.Error("Update() result.HasErrors = false, want true (pull failed)")
		}
	})

	t.Run("Detached HEAD restores the commit SHA", func(t *testing.T) {
		repo := newRepo()
		const detachedSHA = "abc123def456"
		var lastCheckout string
		repo.SetGitCommandExecutor(&MockGitCommandExecutor{
			ExecuteFunc: func(ctx context.Context, repoPath string, stdout bool, args ...string) ([]byte, error) {
				switch args[0] {
				case "rev-parse":
					// GetCurrentBranch uses --abbrev-ref and reports HEAD when detached;
					// Update then resolves the bare SHA.
					if len(args) > 1 && args[1] == "--abbrev-ref" {
						return []byte("HEAD\n"), nil
					}
					return []byte(detachedSHA + "\n"), nil
				case "fetch":
					return []byte(""), nil
				case "status":
					return []byte(""), nil
				case "for-each-ref":
					// A branch that is behind, forcing a checkout+pull cycle.
					return []byte(refLine("feature", "", "origin/feature", "[behind 1]", "", "")), nil
				case "stash":
					return []byte(""), nil
				case "pull":
					return []byte(""), nil
				case "checkout":
					lastCheckout = args[len(args)-1]
					return []byte(""), nil
				}
				return nil, fmt.Errorf("unexpected command: %v", args)
			},
		})

		_, err := repo.Update(context.Background(), false, false)
		if err != nil {
			t.Fatalf("Update() error = %v, want nil", err)
		}
		if lastCheckout != detachedSHA {
			t.Errorf("final checkout arg = %q, want the detached SHA %q", lastCheckout, detachedSHA)
		}
	})
}

func TestPruneBranches(t *testing.T) {
	// Create a repository for testing
	repo := NewTestRepository()
	repo.Path = "/tmp" // must be a real path so os.Stat passes in Repository.Status()

	// Test prune branches with gone remotes
	t.Run("Prune branches with gone remotes", func(t *testing.T) {
		mockExecutor := &MockGitCommandExecutor{
			ExecuteFunc: func(ctx context.Context, repoPath string, stdout bool, args ...string) ([]byte, error) {
				if len(args) > 0 && args[0] == "status" {
					return []byte(""), nil
				}
				if len(args) > 0 && args[0] == "for-each-ref" {
					return []byte(refLine("main", "*", "origin/main", "", "", "") + "\n" +
						refLine("feature", "", "origin/feature", "", "", "") + "\n" +
						refLine("old-feature", "", "origin/old-feature", "[gone]", "", "")), nil
				}
				if len(args) > 0 && args[0] == "branch" {
					if len(args) > 1 && args[1] == "-D" {
						if args[2] != "old-feature" {
							t.Errorf("Expected to delete old-feature, got: %v", args[2])
						}
						return []byte("Deleted branch " + args[2]), nil
					}
				}
				if len(args) > 1 && args[0] == "stash" && args[1] == "list" {
					return []byte(""), nil
				}
				if len(args) > 0 && args[0] == "show-ref" {
					if args[len(args)-1] == "refs/heads/main" {
						return nil, nil // main branch exists
					}
					return nil, errors.New("ref not found")
				}
				if len(args) > 0 && args[0] == "rev-parse" {
					return []byte("main"), nil
				}
				return nil, errors.New("unexpected command")
			},
		}

		repo.Repository.SetGitCommandExecutor(mockExecutor)
		branches, err := repo.Repository.PruneBranches(context.Background(), true, false, false, false)
		if err != nil {
			t.Errorf("PruneBranches() error = %v, want nil", err)
		}
		if len(branches) != 1 || branches[0] != "old-feature" {
			t.Errorf("PruneBranches() branches = %v, want [old-feature]", branches)
		}
	})

	// Test prune merged branches
	t.Run("Prune merged branches", func(t *testing.T) {
		mockExecutor := &MockGitCommandExecutor{
			ExecuteFunc: func(ctx context.Context, repoPath string, stdout bool, args ...string) ([]byte, error) {
				if len(args) > 0 && args[0] == "status" {
					return []byte(""), nil
				}
				if len(args) > 0 && args[0] == "for-each-ref" {
					return []byte(refLine("main", "*", "origin/main", "", "", "") + "\n" +
						refLine("feature", "", "origin/feature", "", "", "") + "\n" +
						refLine("merged-feature", "", "origin/merged-feature", "", "", "")), nil
				}
				if len(args) > 0 && args[0] == "branch" {
					if len(args) > 1 && args[1] == "--merged" {
						return []byte("  feature\n  merged-feature"), nil
					}
					if len(args) > 1 && args[1] == "-D" {
						if args[2] != "merged-feature" && args[2] != "feature" {
							t.Errorf("Expected to delete feature or merged-feature, got: %v", args[2])
						}
						return []byte("Deleted branch " + args[2]), nil
					}
				}
				if len(args) > 1 && args[0] == "stash" && args[1] == "list" {
					return []byte(""), nil
				}
				if len(args) > 0 && args[0] == "show-ref" {
					if args[len(args)-1] == "refs/heads/main" {
						return nil, nil // main branch exists
					}
					return nil, errors.New("ref not found")
				}
				if len(args) > 0 && args[0] == "rev-parse" {
					return []byte("main"), nil
				}
				return nil, errors.New("unexpected command")
			},
		}

		repo.Repository.SetGitCommandExecutor(mockExecutor)
		branches, err := repo.Repository.PruneBranches(context.Background(), false, true, false, false)
		if err != nil {
			t.Errorf("PruneBranches() error = %v, want nil", err)
		}
		if len(branches) != 2 {
			t.Errorf("PruneBranches() branches = %v, want [feature merged-feature]", branches)
		}
	})

	// Test dry run
	t.Run("Dry run", func(t *testing.T) {
		mockExecutor := &MockGitCommandExecutor{
			ExecuteFunc: func(ctx context.Context, repoPath string, stdout bool, args ...string) ([]byte, error) {
				if len(args) > 0 && args[0] == "status" {
					return []byte(""), nil
				}
				if len(args) > 0 && args[0] == "for-each-ref" {
					return []byte(refLine("main", "*", "origin/main", "", "", "") + "\n" +
						refLine("feature", "", "origin/feature", "", "", "") + "\n" +
						refLine("old-feature", "", "origin/old-feature", "[gone]", "", "")), nil
				}
				if len(args) > 0 && args[0] == "branch" {
					if len(args) > 1 && args[1] == "-D" {
						t.Error("Branch deletion should not be called in dry run mode")
						return nil, errors.New("should not be called")
					}
				}
				if len(args) > 1 && args[0] == "stash" && args[1] == "list" {
					return []byte(""), nil
				}
				if len(args) > 0 && args[0] == "show-ref" {
					if args[len(args)-1] == "refs/heads/main" {
						return nil, nil // main branch exists
					}
					return nil, errors.New("ref not found")
				}
				if len(args) > 0 && args[0] == "rev-parse" {
					return []byte("main"), nil
				}
				return nil, errors.New("unexpected command")
			},
		}

		repo.Repository.SetGitCommandExecutor(mockExecutor)
		branches, err := repo.Repository.PruneBranches(context.Background(), true, false, true, false)
		if err != nil {
			t.Errorf("PruneBranches() error = %v, want nil", err)
		}
		if len(branches) != 1 || branches[0] != "old-feature" {
			t.Errorf("PruneBranches() branches = %v, want [old-feature]", branches)
		}
	})

	// Test error getting repository status
	t.Run("Error getting repository status", func(t *testing.T) {
		mockExecutor := &MockGitCommandExecutor{
			ExecuteFunc: func(ctx context.Context, repoPath string, stdout bool, args ...string) ([]byte, error) {
				if len(args) > 0 && args[0] == "status" {
					return nil, errors.New("status failed")
				}
				return nil, errors.New("unexpected command")
			},
		}

		repo.Repository.SetGitCommandExecutor(mockExecutor)
		_, err := repo.Repository.PruneBranches(context.Background(), true, false, false, false)
		if err == nil {
			t.Error("PruneBranches() error = nil, want error")
		}
	})

	// Test error deleting branch
	t.Run("Error deleting branch", func(t *testing.T) {
		mockExecutor := &MockGitCommandExecutor{
			ExecuteFunc: func(ctx context.Context, repoPath string, stdout bool, args ...string) ([]byte, error) {
				if len(args) > 0 && args[0] == "status" {
					return []byte(""), nil
				}
				if len(args) > 0 && args[0] == "for-each-ref" {
					return []byte(refLine("main", "*", "origin/main", "", "", "") + "\n" +
						refLine("feature", "", "origin/feature", "", "", "") + "\n" +
						refLine("old-feature", "", "origin/old-feature", "[gone]", "", "")), nil
				}
				if len(args) > 0 && args[0] == "branch" {
					if len(args) > 1 && args[1] == "-D" {
						return nil, errors.New("failed to delete branch")
					}
				}
				if len(args) > 1 && args[0] == "stash" && args[1] == "list" {
					return []byte(""), nil
				}
				if len(args) > 0 && args[0] == "show-ref" {
					if args[len(args)-1] == "refs/heads/main" {
						return nil, nil // main branch exists
					}
					return nil, errors.New("ref not found")
				}
				if len(args) > 0 && args[0] == "rev-parse" {
					return []byte("main"), nil
				}
				return nil, errors.New("unexpected command")
			},
		}

		repo.Repository.SetGitCommandExecutor(mockExecutor)
		_, err := repo.Repository.PruneBranches(context.Background(), true, false, false, false)
		if err == nil {
			t.Error("PruneBranches() error = nil, want error")
		}
	})
}

// exitError produces an *exec.ExitError with the given exit code.
func exitError(code int) error {
	cmd := exec.Command("sh", "-c", fmt.Sprintf("exit %d", code))
	err := cmd.Run()
	return err
}

func TestFilterRepositories(t *testing.T) {
	repos := []*Repository{
		{Host: "github.com", Organization: "acme", Name: "api"},
		{Host: "github.com", Organization: "acme", Name: "frontend"},
		{Host: "gitlab.com", Organization: "acme", Name: "infra"},
		{Host: "github.com", Organization: "other", Name: "tool"},
	}

	tests := []struct {
		name  string
		host  string
		org   string
		repo  string
		count int
	}{
		{"no filters returns all", "", "", "", 4},
		{"filter by host", "github.com", "", "", 3},
		{"filter by org", "", "acme", "", 3},
		{"filter by repo name", "", "", "api", 1},
		{"host + org", "github.com", "acme", "", 2},
		{"all three filters", "github.com", "acme", "api", 1},
		{"no match", "bitbucket.org", "", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterRepositories(repos, tt.host, tt.org, tt.repo)
			if len(got) != tt.count {
				t.Errorf("FilterRepositories() len = %d, want %d", len(got), tt.count)
			}
		})
	}
}

func TestGetCurrentBranch(t *testing.T) {
	repo := NewTestRepository()
	repo.Path = "/mock/path"

	t.Run("success", func(t *testing.T) {
		repo.SetGitCommandExecutor(&MockGitCommandExecutor{
			ExecuteFunc: func(_ context.Context, _ string, _ bool, args ...string) ([]byte, error) {
				return []byte("feature\n"), nil
			},
		})
		branch, err := repo.Repository.GetCurrentBranch(context.Background())
		if err != nil {
			t.Fatalf("GetCurrentBranch() error = %v, want nil", err)
		}
		if branch != "feature" {
			t.Errorf("GetCurrentBranch() = %q, want %q", branch, "feature")
		}
	})

	t.Run("error", func(t *testing.T) {
		repo.SetGitCommandExecutor(&MockGitCommandExecutor{
			ExecuteFunc: func(_ context.Context, _ string, _ bool, args ...string) ([]byte, error) {
				return nil, errors.New("not a git repo")
			},
		})
		_, err := repo.Repository.GetCurrentBranch(context.Background())
		if err == nil {
			t.Error("GetCurrentBranch() error = nil, want error")
		}
	})
}

func TestCheckout(t *testing.T) {
	repo := NewTestRepository()
	repo.Path = "/mock/path"

	t.Run("success", func(t *testing.T) {
		var gotArgs []string
		repo.SetGitCommandExecutor(&MockGitCommandExecutor{
			ExecuteFunc: func(_ context.Context, _ string, _ bool, args ...string) ([]byte, error) {
				gotArgs = args
				return nil, nil
			},
		})
		if err := repo.Repository.Checkout(context.Background(), "main"); err != nil {
			t.Fatalf("Checkout() error = %v, want nil", err)
		}
		if len(gotArgs) < 2 || gotArgs[0] != "checkout" || gotArgs[1] != "main" {
			t.Errorf("Checkout() args = %v, want [checkout main]", gotArgs)
		}
	})

	t.Run("error", func(t *testing.T) {
		repo.SetGitCommandExecutor(&MockGitCommandExecutor{
			ExecuteFunc: func(_ context.Context, _ string, _ bool, args ...string) ([]byte, error) {
				return nil, errors.New("branch not found")
			},
		})
		if err := repo.Repository.Checkout(context.Background(), "nonexistent"); err == nil {
			t.Error("Checkout() error = nil, want error")
		}
	})
}

func TestGetDefaultBranch(t *testing.T) {
	repo := NewTestRepository()
	repo.Path = "/mock/path"

	t.Run("main exists", func(t *testing.T) {
		repo.SetGitCommandExecutor(&MockGitCommandExecutor{
			ExecuteFunc: func(_ context.Context, _ string, _ bool, args ...string) ([]byte, error) {
				if args[0] == "show-ref" && args[len(args)-1] == "refs/heads/main" {
					return nil, nil // main exists
				}
				return nil, errors.New("unexpected")
			},
		})
		branch, err := repo.Repository.GetDefaultBranch(context.Background())
		if err != nil {
			t.Fatalf("GetDefaultBranch() error = %v, want nil", err)
		}
		if branch != "main" {
			t.Errorf("GetDefaultBranch() = %q, want %q", branch, "main")
		}
	})

	t.Run("master fallback", func(t *testing.T) {
		repo.SetGitCommandExecutor(&MockGitCommandExecutor{
			ExecuteFunc: func(_ context.Context, _ string, _ bool, args ...string) ([]byte, error) {
				if args[0] == "show-ref" {
					if args[len(args)-1] == "refs/heads/main" {
						return nil, exitError(1) // main absent
					}
					if args[len(args)-1] == "refs/heads/master" {
						return nil, nil // master exists
					}
				}
				return nil, errors.New("unexpected")
			},
		})
		branch, err := repo.Repository.GetDefaultBranch(context.Background())
		if err != nil {
			t.Fatalf("GetDefaultBranch() error = %v, want nil", err)
		}
		if branch != "master" {
			t.Errorf("GetDefaultBranch() = %q, want %q", branch, "master")
		}
	})

	t.Run("fallback to current branch", func(t *testing.T) {
		repo.SetGitCommandExecutor(&MockGitCommandExecutor{
			ExecuteFunc: func(_ context.Context, _ string, _ bool, args ...string) ([]byte, error) {
				if args[0] == "show-ref" {
					return nil, exitError(1) // neither main nor master
				}
				if args[0] == "rev-parse" {
					return []byte("develop\n"), nil
				}
				return nil, errors.New("unexpected")
			},
		})
		branch, err := repo.Repository.GetDefaultBranch(context.Background())
		if err != nil {
			t.Fatalf("GetDefaultBranch() error = %v, want nil", err)
		}
		if branch != "develop" {
			t.Errorf("GetDefaultBranch() = %q, want %q", branch, "develop")
		}
	})

	t.Run("real error on main check", func(t *testing.T) {
		repo.SetGitCommandExecutor(&MockGitCommandExecutor{
			ExecuteFunc: func(_ context.Context, _ string, _ bool, args ...string) ([]byte, error) {
				return nil, errors.New("network error")
			},
		})
		_, err := repo.Repository.GetDefaultBranch(context.Background())
		if err == nil {
			t.Error("GetDefaultBranch() error = nil, want error")
		}
	})

	t.Run("real error on master check", func(t *testing.T) {
		repo.SetGitCommandExecutor(&MockGitCommandExecutor{
			ExecuteFunc: func(_ context.Context, _ string, _ bool, args ...string) ([]byte, error) {
				if args[0] == "show-ref" && args[len(args)-1] == "refs/heads/main" {
					return nil, exitError(1)
				}
				return nil, errors.New("disk error")
			},
		})
		_, err := repo.Repository.GetDefaultBranch(context.Background())
		if err == nil {
			t.Error("GetDefaultBranch() error = nil, want error")
		}
	})
}

func TestMarkStaleBranches(t *testing.T) {
	repo := NewTestRepository()
	repo.Path = "/mock/path"

	now := time.Now()
	old := now.Add(-60 * 24 * time.Hour)
	recent := now.Add(-1 * 24 * time.Hour)
	threshold := 30 * 24 * time.Hour

	// MarkStaleBranches relies on LastCommitDate already being populated on the
	// branches (ListBranches supplies it via Status); it only calls
	// GetDefaultBranch (show-ref) and rev-list for the behind count.
	t.Run("marks old branch stale, skips default branch", func(t *testing.T) {
		repo.SetGitCommandExecutor(&MockGitCommandExecutor{
			ExecuteFunc: func(_ context.Context, _ string, _ bool, args ...string) ([]byte, error) {
				if args[0] == "show-ref" && args[len(args)-1] == "refs/heads/main" {
					return nil, nil
				}
				if args[0] == "rev-list" {
					return []byte("5\n"), nil
				}
				return nil, errors.New("unexpected: " + args[0])
			},
		})

		status := &RepositoryStatus{
			Branches: []BranchInfo{
				{Name: "main", LastCommitDate: old},      // default branch, old but skipped
				{Name: "feature", LastCommitDate: old},   // stale
				{Name: "active", LastCommitDate: recent}, // recent, not stale
			},
		}

		if err := repo.Repository.MarkStaleBranches(context.Background(), status, threshold); err != nil {
			t.Fatalf("MarkStaleBranches() error = %v", err)
		}

		if !status.HasStaleBranches {
			t.Error("HasStaleBranches = false, want true")
		}
		for _, b := range status.Branches {
			switch b.Name {
			case "main":
				if b.Stale {
					t.Error("main is the default branch and must not be marked stale")
				}
			case "feature":
				if !b.Stale {
					t.Error("feature is old and should be marked stale")
				}
				if b.CommitsBehindDefault != 5 {
					t.Errorf("feature CommitsBehindDefault = %d, want 5", b.CommitsBehindDefault)
				}
			case "active":
				if b.Stale {
					t.Error("active is recent and must not be marked stale")
				}
			}
		}
	})

	t.Run("GetDefaultBranch error propagates", func(t *testing.T) {
		repo.SetGitCommandExecutor(&MockGitCommandExecutor{
			ExecuteFunc: func(_ context.Context, _ string, _ bool, args ...string) ([]byte, error) {
				return nil, errors.New("git error")
			},
		})
		status := &RepositoryStatus{Branches: []BranchInfo{{Name: "feature", LastCommitDate: old}}}
		if err := repo.Repository.MarkStaleBranches(context.Background(), status, threshold); err == nil {
			t.Error("MarkStaleBranches() error = nil, want error")
		}
	})
}

// refLine builds a NUL-separated for-each-ref line matching branchRefFormat:
// refname, HEAD, upstream:short, upstream:track, committerdate, worktreepath.
func refLine(name, head, upstream, track, date, worktree string) string {
	return strings.Join([]string{name, head, upstream, track, date, worktree}, "\x00")
}

func TestParseBranchRefLine(t *testing.T) {
	tests := []struct {
		name string
		line string
		want *BranchInfo
	}{
		{
			name: "current branch with tracking",
			line: refLine("main", "*", "origin/main", "", "", ""),
			want: &BranchInfo{Name: "main", Current: true, RemoteTracking: "origin/main"},
		},
		{
			name: "non-current branch",
			line: refLine("feature", "", "origin/feature", "", "", ""),
			want: &BranchInfo{Name: "feature", Current: false, RemoteTracking: "origin/feature"},
		},
		{
			name: "branch behind remote",
			line: refLine("develop", "", "origin/develop", "[behind 3]", "", ""),
			want: &BranchInfo{Name: "develop", RemoteTracking: "origin/develop", Behind: 3},
		},
		{
			name: "branch ahead of remote",
			line: refLine("feature", "", "origin/feature", "[ahead 2]", "", ""),
			want: &BranchInfo{Name: "feature", RemoteTracking: "origin/feature", Ahead: 2},
		},
		{
			name: "branch ahead and behind",
			line: refLine("topic", "", "origin/topic", "[ahead 1, behind 2]", "", ""),
			want: &BranchInfo{Name: "topic", RemoteTracking: "origin/topic", Ahead: 1, Behind: 2},
		},
		{
			name: "branch with remote gone",
			line: refLine("old-feature", "", "origin/old-feature", "[gone]", "", ""),
			want: &BranchInfo{Name: "old-feature", RemoteTracking: "origin/old-feature", RemoteGone: true},
		},
		{
			name: "current branch with remote gone",
			line: refLine("old-main", "*", "origin/old-main", "[gone]", "", ""),
			want: &BranchInfo{Name: "old-main", Current: true, RemoteTracking: "origin/old-main", RemoteGone: true},
		},
		{
			name: "branch without remote tracking",
			line: refLine("local-only", "", "", "", "", ""),
			want: &BranchInfo{Name: "local-only", NoRemoteTracking: true},
		},
		{
			name: "current branch without remote tracking",
			line: refLine("local-main", "*", "", "", "", ""),
			want: &BranchInfo{Name: "local-main", Current: true, NoRemoteTracking: true},
		},
		{
			name: "branch literally named with brackets and gone in subject is safe",
			line: refLine("feature/[gone]", "", "", "", "", ""),
			want: &BranchInfo{Name: "feature/[gone]", NoRemoteTracking: true},
		},
		{
			name: "branch checked out in a worktree",
			line: refLine("wt", "", "origin/wt", "", "", "/home/user/wt"),
			want: &BranchInfo{Name: "wt", RemoteTracking: "origin/wt", WorktreePath: "/home/user/wt"},
		},
		{
			name: "empty line returns nil",
			line: "",
			want: nil,
		},
		{
			name: "whitespace only returns nil",
			line: "   ",
			want: nil,
		},
		{
			name: "malformed line (too few fields) returns nil",
			line: "just-a-name\x00*",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseBranchRefLine(tt.line)
			if tt.want == nil {
				if got != nil {
					t.Errorf("parseBranchRefLine(%q) = %+v, want nil", tt.line, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("parseBranchRefLine(%q) = nil, want %+v", tt.line, tt.want)
			}
			if got.Name != tt.want.Name {
				t.Errorf("Name = %q, want %q", got.Name, tt.want.Name)
			}
			if got.Current != tt.want.Current {
				t.Errorf("Current = %v, want %v", got.Current, tt.want.Current)
			}
			if got.RemoteTracking != tt.want.RemoteTracking {
				t.Errorf("RemoteTracking = %q, want %q", got.RemoteTracking, tt.want.RemoteTracking)
			}
			if got.RemoteGone != tt.want.RemoteGone {
				t.Errorf("RemoteGone = %v, want %v", got.RemoteGone, tt.want.RemoteGone)
			}
			if got.NoRemoteTracking != tt.want.NoRemoteTracking {
				t.Errorf("NoRemoteTracking = %v, want %v", got.NoRemoteTracking, tt.want.NoRemoteTracking)
			}
			if got.Ahead != tt.want.Ahead {
				t.Errorf("Ahead = %d, want %d", got.Ahead, tt.want.Ahead)
			}
			if got.Behind != tt.want.Behind {
				t.Errorf("Behind = %d, want %d", got.Behind, tt.want.Behind)
			}
			if got.WorktreePath != tt.want.WorktreePath {
				t.Errorf("WorktreePath = %q, want %q", got.WorktreePath, tt.want.WorktreePath)
			}
		})
	}
}
