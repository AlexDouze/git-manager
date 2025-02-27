package git

import (
	"encoding/json"
	"reflect"
	"testing"
)

// MockGithubCommandExecutor is a mock implementation of GithubCommandExecutor for testing
type MockGithubCommandExecutor struct {
	MockOutput []byte
	MockError  error
	CalledWith []string
}

// Execute records the arguments and returns the mock output and error
func (m *MockGithubCommandExecutor) Execute(args ...string) ([]byte, error) {
	m.CalledWith = args
	return m.MockOutput, m.MockError
}

func TestListGitHubRepositoriesWithExecutor(t *testing.T) {
	// Create mock data
	mockRepos := []githubRepository{
		{
			Name: "repo1",
			Owner: struct {
				Login string `json:"login"`
			}{
				Login: "user1",
			},
			URL: "https://github.com/user1/repo1",
		},
		{
			Name: "repo2",
			Owner: struct {
				Login string `json:"login"`
			}{
				Login: "user1",
			},
			URL: "https://github.com/user1/repo2",
		},
	}

	// Convert mock data to JSON
	mockOutput, err := json.Marshal(mockRepos)
	if err != nil {
		t.Fatalf("Failed to marshal mock data: %v", err)
	}

	// Create mock executor
	mockExecutor := &MockGithubCommandExecutor{
		MockOutput: mockOutput,
		MockError:  nil,
	}

	// Call the function with the mock executor
	repos, err := ListGitHubRepositoriesWithExecutor("user1", mockExecutor)

	// Verify there was no error
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify the correct arguments were passed to the executor
	expectedArgs := []string{"repo", "list", "user1", "--json", "name,owner,url", "--limit", "1000"}
	if !reflect.DeepEqual(mockExecutor.CalledWith, expectedArgs) {
		t.Errorf("Expected args %v, got %v", expectedArgs, mockExecutor.CalledWith)
	}

	// Verify the correct repositories were returned
	expectedRepos := []Repository{
		{
			Host:         "github.com",
			Organization: "user1",
			Name:         "repo1",
		},
		{
			Host:         "github.com",
			Organization: "user1",
			Name:         "repo2",
		},
	}

	if !reflect.DeepEqual(repos, expectedRepos) {
		t.Errorf("Expected repos %+v, got %+v", expectedRepos, repos)
	}
}

func TestListGitHubRepositoriesWithExecutorNoOwner(t *testing.T) {
	// Create mock executor
	mockExecutor := &MockGithubCommandExecutor{
		MockOutput: []byte("[]"), // Empty array
		MockError:  nil,
	}

	// Call the function with no owner
	_, err := ListGitHubRepositoriesWithExecutor("", mockExecutor)

	// Verify there was no error
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify the correct arguments were passed to the executor (no owner)
	expectedArgs := []string{"repo", "list", "--json", "name,owner,url", "--limit", "1000"}
	if !reflect.DeepEqual(mockExecutor.CalledWith, expectedArgs) {
		t.Errorf("Expected args %v, got %v", expectedArgs, mockExecutor.CalledWith)
	}
}
