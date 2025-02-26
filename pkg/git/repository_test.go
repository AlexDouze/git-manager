package git

import (
	"errors"
	"fmt"
	"testing"
)

// MockGitCommandExecutor is a mock implementation of GitCommandExecutor for testing
type MockGitCommandExecutor struct {
	// ExecuteFunc allows tests to define custom behavior for the Execute method
	ExecuteFunc func(repoPath string, stdout bool, args ...string) ([]byte, error)
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
func (r *TestRepository) Status() (*RepositoryStatus, error) {
	status := &RepositoryStatus{
		Repository: r.Repository,
	}

	// Skip the filesystem check that would normally happen in Repository.Status()
	// and only perform it if we explicitly want to test that case
	if !r.pathExists {
		return nil, fmt.Errorf("repository path does not exist: %s", r.Path)
	}

	// Get uncommitted changes
	if err := r.getUncommittedChanges(status); err != nil {
		return nil, fmt.Errorf("failed to get uncommitted changes: %w", err)
	}

	// Get branch information
	if err := r.getBranchInformation(status); err != nil {
		return nil, fmt.Errorf("failed to get branch information: %w", err)
	}

	// Check for stashes
	if err := r.getStashInformation(status); err != nil {
		return nil, fmt.Errorf("failed to get stash information: %w", err)
	}

	return status, nil
}

// Execute calls the mock's ExecuteFunc if defined, or returns a default response
func (m *MockGitCommandExecutor) Execute(repoPath string, stdout bool, args ...string) ([]byte, error) {
	if m.ExecuteFunc != nil {
		return m.ExecuteFunc(repoPath, stdout, args...)
	}
	return []byte("mock response"), nil
}

func TestRepositoryWithMockExecutor(t *testing.T) {
	// Create a new test repository
	repo := NewTestRepository()

	// Create a mock executor with custom behavior
	mockExecutor := &MockGitCommandExecutor{
		ExecuteFunc: func(repoPath string, stdout bool, args ...string) ([]byte, error) {
			// Check if the command is "status"
			if len(args) > 0 && args[0] == "status" {
				return []byte("M  README.md"), nil
			}

			// Check if the command is "branch"
			if len(args) > 0 && args[0] == "branch" {
				return []byte("* main\n  feature"), nil
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
	status, err := repo.Status()
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
		ExecuteFunc: func(repoPath string, stdout bool, args ...string) ([]byte, error) {
			return nil, errors.New("mock error")
		},
	}

	// Set the mock executor on the repository
	repo.SetGitCommandExecutor(mockExecutor)

	// Test repository with mock
	repo.Path = "/mock/path"

	// Test Status method with mock
	_, err := repo.Status()
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
	_, err := repo.Status()
	if err == nil {
		t.Fatal("Expected an error for non-existent path, got nil")
	}

	if err.Error() != "repository path does not exist: /non/existent/path" {
		t.Errorf("Expected path not exist error, got: %v", err)
	}
}
