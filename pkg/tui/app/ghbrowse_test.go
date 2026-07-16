package app

import (
	"context"
	"testing"

	"github.com/alexDouze/gitm/pkg/config"
	"github.com/alexDouze/gitm/pkg/git"
)

// listedGHScreen returns a browser advanced to the list phase with the given
// repositories, sized to a real window.
func listedGHScreen(t *testing.T, repos ...git.Repository) ghScreen {
	t.Helper()
	cfg := &config.Config{RootDirectory: "/root"}
	gh := newGHScreen(context.Background(), cfg, newStyles(), "owner", "", 100)
	gh.setSize(80, 24)
	gh, _ = gh.update(ghReposLoadedMsg{repos: repos})
	return gh
}

func TestGHToggleSelectsWithSpace(t *testing.T) {
	gh := listedGHScreen(t, git.Repository{Host: "github.com", Organization: "org", Name: "repo1"})
	if gh.phase != ghPhaseList {
		t.Fatalf("phase = %d, want ghPhaseList", gh.phase)
	}

	// Pressing space must toggle the highlighted repo's selection. This guards
	// the binding: v2 reports the space key as "space", not " ".
	gh, _ = gh.update(keyPress("space"))

	items := gh.list.Items()
	if len(items) != 1 {
		t.Fatalf("item count = %d, want 1", len(items))
	}
	it := items[0].(ghItem)
	if !it.selected {
		t.Error("space did not select the repo (Toggle binding regressed)")
	}

	// Space again deselects.
	gh, _ = gh.update(keyPress("space"))
	it = gh.list.Items()[0].(ghItem)
	if it.selected {
		t.Error("second space did not deselect the repo")
	}
}

func TestGHBackExits(t *testing.T) {
	gh := listedGHScreen(t, git.Repository{Host: "github.com", Organization: "org", Name: "repo1"})

	_, cmd := gh.update(keyPress("esc"))
	if cmd == nil {
		t.Fatal("esc should return a command")
	}
	if _, ok := cmd().(ghExitMsg); !ok {
		t.Error("esc should produce a ghExitMsg")
	}
}

func TestGHCloneNothingSelected(t *testing.T) {
	gh := listedGHScreen(t, git.Repository{Host: "github.com", Organization: "org", Name: "repo1"})

	gh, cmd := gh.update(keyPress("enter"))
	if !gh.footerErr {
		t.Error("cloning with nothing selected should set footerErr")
	}
	if cmd != nil {
		t.Error("cloning with nothing selected should not start a clone batch")
	}
}

func TestGHOwnerPromptSubmit(t *testing.T) {
	cfg := &config.Config{RootDirectory: "/root"}
	gh := newGHScreen(context.Background(), cfg, newStyles(), "", "", 100)
	gh.setSize(80, 24)
	if gh.phase != ghPhaseOwner {
		t.Fatalf("phase = %d, want ghPhaseOwner when no owner is supplied", gh.phase)
	}
	// init() focuses the input; without focus the textinput ignores keystrokes.
	gh.init()

	// Type an owner and submit.
	gh, _ = gh.update(keyPress("a"))
	gh, cmd := gh.update(keyPress("enter"))
	if gh.phase != ghPhaseLoading {
		t.Errorf("phase = %d, want ghPhaseLoading after submitting owner", gh.phase)
	}
	if gh.owner != "a" {
		t.Errorf("owner = %q, want %q", gh.owner, "a")
	}
	if cmd == nil {
		t.Error("submitting an owner should kick off the repo listing")
	}
}
