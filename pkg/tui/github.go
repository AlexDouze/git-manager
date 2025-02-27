package tui

import (
	"fmt"

	"github.com/alexDouze/gitm/pkg/git"
)

func SelectGithubReposRender(repos []git.Repository) ([]git.Repository, error) {
	for i, repo := range repos {
		fmt.Print(i+1, ". ", repo.Organization, "/", repo.Name, "\n")
	}
	return nil, nil
}
