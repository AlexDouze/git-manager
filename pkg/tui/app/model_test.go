package app

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	"github.com/alexDouze/gitm/pkg/config"
	"github.com/alexDouze/gitm/pkg/git"
)

// keyPress builds a KeyPressMsg for a keystroke string, mirroring how Bubble Tea
// v2 reports keys (see Key.String / Keystroke). Special keys use their Code;
// printable single characters carry Text so String() returns the character.
func keyPress(s string) tea.KeyPressMsg {
	switch s {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "space":
		return tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
	case "ctrl+c":
		// A control combo carries no printable Text, so String() falls through
		// to Keystroke(), yielding "ctrl+c".
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	default:
		r := []rune(s)[0]
		return tea.KeyPressMsg{Code: r, Text: s}
	}
}

// mockExecutor is a local GitCommandExecutor stub. The git package's own mock
// lives in a _test.go file and isn't importable here, so the app tests define
// their own against the exported GitCommandExecutor interface.
type mockExecutor struct {
	fn func(ctx context.Context, repoPath string, stdout bool, args ...string) ([]byte, error)
}

func (m mockExecutor) Execute(ctx context.Context, repoPath string, stdout bool, args ...string) ([]byte, error) {
	return m.fn(ctx, repoPath, stdout, args...)
}

// newRepo builds a Repository with a synthetic host/org/name/path.
func newRepo(name string) *git.Repository {
	r := git.NewRepository()
	r.Host = "github.com"
	r.Organization = "org"
	r.Name = name
	r.Path = "/root/github.com/org/" + name
	return r
}

// seededModel returns a Model that has finished its initial load with the named
// repositories, sized to a real window so the list has a viewport.
func seededModel(t *testing.T, names ...string) Model {
	t.Helper()
	cfg := &config.Config{RootDirectory: "/root"}
	m := New(context.Background(), cfg, Filter{})

	tm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = tm.(Model)

	repos := make([]*git.Repository, 0, len(names))
	for _, n := range names {
		repos = append(repos, newRepo(n))
	}
	tm, _ = m.Update(reposLoadedMsg{repos: repos})
	return tm.(Model)
}

func TestInitialLoadPopulatesRepos(t *testing.T) {
	m := seededModel(t, "alpha", "beta")

	if m.loading {
		t.Error("loading should be false after reposLoadedMsg")
	}
	if got := len(m.repos.Items()); got != 2 {
		t.Fatalf("repo item count = %d, want 2", got)
	}
	if _, ok := m.byPath["/root/github.com/org/alpha"]; !ok {
		t.Error("byPath missing alpha")
	}
	if !m.statusBusy {
		t.Error("statusBusy should be true while the status load is dispatched")
	}
}

func TestReposLoadedError(t *testing.T) {
	cfg := &config.Config{RootDirectory: "/root"}
	m := New(context.Background(), cfg, Filter{})

	tm, _ := m.Update(reposLoadedMsg{err: errors.New("walk failed")})
	m = tm.(Model)

	if m.loading {
		t.Error("loading should be false after an error")
	}
	if m.err == nil {
		t.Error("err should be set")
	}
}

func TestDrillInAndBack(t *testing.T) {
	m := seededModel(t, "alpha", "beta")

	tm, _ := m.Update(keyPress("enter"))
	m = tm.(Model)
	if m.screen != screenBranches {
		t.Fatalf("screen = %d, want screenBranches", m.screen)
	}
	if m.activeRepo == nil || m.activeRepo.Name != "alpha" {
		t.Fatalf("activeRepo = %v, want alpha", m.activeRepo)
	}
	if !m.branchBusy {
		t.Error("branchBusy should be true after drill-in")
	}

	tm, _ = m.Update(keyPress("esc"))
	m = tm.(Model)
	if m.screen != screenRepos {
		t.Errorf("screen = %d, want screenRepos", m.screen)
	}
	if m.activeRepo != nil {
		t.Error("activeRepo should be cleared after back")
	}
}

func TestDrillInNoSelectionIsNoop(t *testing.T) {
	m := seededModel(t) // no repos

	tm, _ := m.Update(keyPress("enter"))
	m = tm.(Model)
	if m.screen != screenRepos {
		t.Errorf("screen = %d, want screenRepos when list is empty", m.screen)
	}
}

func TestQuitKeys(t *testing.T) {
	for _, k := range []string{"q", "ctrl+c"} {
		m := seededModel(t, "alpha")
		_, cmd := m.Update(keyPress(k))
		if cmd == nil {
			t.Fatalf("%q returned no command", k)
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Errorf("%q did not produce a QuitMsg", k)
		}
	}
}

func TestPruneOpensConfirm(t *testing.T) {
	m := seededModel(t, "alpha")

	tm, _ := m.Update(keyPress("p"))
	m = tm.(Model)
	if m.confirm == nil {
		t.Fatal("prune should open a confirm overlay")
	}
	if !strings.Contains(m.confirm.prompt, "alpha") {
		t.Errorf("confirm prompt = %q, want it to mention alpha", m.confirm.prompt)
	}
}

func TestConfirmCancelAndAccept(t *testing.T) {
	// Cancel with n.
	m := seededModel(t, "alpha")
	tm, _ := m.Update(keyPress("p"))
	m = tm.(Model)
	tm, _ = m.Update(keyPress("n"))
	m = tm.(Model)
	if m.confirm != nil {
		t.Error("n should dismiss the confirm overlay")
	}

	// Cancel with esc.
	tm, _ = m.Update(keyPress("p"))
	m = tm.(Model)
	tm, _ = m.Update(keyPress("esc"))
	m = tm.(Model)
	if m.confirm != nil {
		t.Error("esc should dismiss the confirm overlay")
	}

	// Accept with y runs the pending command and clears the overlay.
	tm, _ = m.Update(keyPress("p"))
	m = tm.(Model)
	tm, cmd := m.Update(keyPress("y"))
	m = tm.(Model)
	if m.confirm != nil {
		t.Error("y should clear the confirm overlay")
	}
	if cmd == nil {
		t.Error("y should return the pending command")
	}
}

func TestUpdateAllMarksRowsBusyAndBulkBusy(t *testing.T) {
	m := seededModel(t, "alpha", "beta")

	tm, cmd := m.Update(keyPress("U"))
	m = tm.(Model)
	if !m.bulkBusy {
		t.Error("U should set bulkBusy")
	}
	if cmd == nil {
		t.Error("U should return a command")
	}
	for _, li := range m.repos.Items() {
		it := li.(repoItem)
		if !it.busy || it.busyLabel != "updating…" {
			t.Errorf("repo %s: busy=%v label=%q, want busy with \"updating…\"", it.repo.Name, it.busy, it.busyLabel)
		}
	}
	if !strings.Contains(m.footer, "2") {
		t.Errorf("footer = %q, want it to mention the repo count", m.footer)
	}
}

func TestPruneAllOpensConfirmWithCount(t *testing.T) {
	m := seededModel(t, "alpha", "beta", "gamma")

	tm, _ := m.Update(keyPress("P"))
	m = tm.(Model)
	if m.confirm == nil {
		t.Fatal("P should open a confirm overlay")
	}
	if !strings.Contains(m.confirm.prompt, "3") {
		t.Errorf("confirm prompt = %q, want it to mention the repo count", m.confirm.prompt)
	}
	// No row should be marked busy yet -- only onAccept (on y) does that.
	for _, li := range m.repos.Items() {
		if li.(repoItem).busy {
			t.Error("rows should not be busy before the prune-all confirm is accepted")
		}
	}
}

func TestPruneAllAcceptMarksRowsBusy(t *testing.T) {
	m := seededModel(t, "alpha", "beta")

	tm, _ := m.Update(keyPress("P"))
	m = tm.(Model)
	tm, cmd := m.Update(keyPress("y"))
	m = tm.(Model)

	if !m.bulkBusy {
		t.Error("accepting prune-all should set bulkBusy")
	}
	if cmd == nil {
		t.Error("accepting prune-all should return the pending command")
	}
	for _, li := range m.repos.Items() {
		it := li.(repoItem)
		if !it.busy || it.busyLabel != "pruning…" {
			t.Errorf("repo %s: busy=%v label=%q, want busy with \"pruning…\"", it.repo.Name, it.busy, it.busyLabel)
		}
	}
}

func TestPruneAllCancelLeavesRowsIdle(t *testing.T) {
	m := seededModel(t, "alpha", "beta")

	tm, _ := m.Update(keyPress("P"))
	m = tm.(Model)
	tm, _ = m.Update(keyPress("n"))
	m = tm.(Model)

	if m.bulkBusy {
		t.Error("cancelling prune-all should not set bulkBusy")
	}
	for _, li := range m.repos.Items() {
		if li.(repoItem).busy {
			t.Error("cancelling prune-all should leave rows idle")
		}
	}
}

func TestBulkOpDoneClearsBusyAndSetsFooter(t *testing.T) {
	m := seededModel(t, "alpha", "beta")
	tm, _ := m.Update(keyPress("U"))
	m = tm.(Model)

	paths := make([]string, 0, len(m.repos.Items()))
	for _, li := range m.repos.Items() {
		paths = append(paths, li.(repoItem).repo.Path)
	}

	tm, cmd := m.Update(bulkOpDoneMsg{
		kind:    opUpdate,
		results: []bulkResult{{path: paths[0]}, {path: paths[1]}},
		summary: "updated 2/2 repositories",
	})
	m = tm.(Model)

	if m.bulkBusy {
		t.Error("bulkOpDoneMsg should clear bulkBusy")
	}
	if m.footer != "updated 2/2 repositories" {
		t.Errorf("footer = %q, want the bulk summary", m.footer)
	}
	if m.footerErr {
		t.Error("a bulk result with no errors should not set footerErr")
	}
	for _, li := range m.repos.Items() {
		if li.(repoItem).busy {
			t.Error("bulkOpDoneMsg should clear every row's busy flag")
		}
	}
	if cmd == nil {
		t.Error("bulkOpDoneMsg should trigger a status reload command")
	}
}

func TestBulkOpDoneWithErrorSetsFooterErr(t *testing.T) {
	m := seededModel(t, "alpha")
	tm, _ := m.Update(keyPress("U"))
	m = tm.(Model)
	path := m.repos.Items()[0].(repoItem).repo.Path

	tm, _ = m.Update(bulkOpDoneMsg{
		kind:    opUpdate,
		results: []bulkResult{{path: path, err: errors.New("boom")}},
		summary: "updated 0/1 repositories (1 failed)",
	})
	m = tm.(Model)

	if !m.footerErr {
		t.Error("a bulk result with a failure should set footerErr")
	}
}

func TestBulkShortcutsNoopWhileBulkBusy(t *testing.T) {
	m := seededModel(t, "alpha", "beta")
	tm, _ := m.Update(keyPress("U"))
	m = tm.(Model)

	// A second U while the first pass is in flight must not stack another
	// worker-pool pass or reset the footer.
	footerBefore := m.footer
	tm, cmd := m.Update(keyPress("U"))
	m = tm.(Model)
	if cmd != nil {
		t.Error("U while bulkBusy should be a no-op (no command)")
	}
	if m.footer != footerBefore {
		t.Error("U while bulkBusy should not change the footer")
	}

	// Single-repo u/p and refresh r should also no-op while a bulk pass runs.
	for _, k := range []string{"u", "p", "r"} {
		tm, cmd = m.Update(keyPress(k))
		m = tm.(Model)
		if cmd != nil {
			t.Errorf("%q while bulkBusy should be a no-op (no command)", k)
		}
		if m.confirm != nil {
			t.Errorf("%q while bulkBusy should not open a confirm overlay", k)
		}
	}
}

func TestUpdateSelectedRepoMarksRowBusy(t *testing.T) {
	m := seededModel(t, "alpha", "beta")

	tm, cmd := m.Update(keyPress("u"))
	m = tm.(Model)
	if cmd == nil {
		t.Fatal("u should return a command")
	}

	items := m.repos.Items()
	sel := items[m.repos.Index()].(repoItem)
	if !sel.busy || sel.busyLabel != "updating…" {
		t.Errorf("selected repo: busy=%v label=%q, want busy with \"updating…\"", sel.busy, sel.busyLabel)
	}
	// The non-selected row must be untouched.
	for i, li := range items {
		if i == m.repos.Index() {
			continue
		}
		if li.(repoItem).busy {
			t.Error("only the selected repo should be marked busy by single-repo update")
		}
	}

	// The matching opDoneMsg clears the busy flag again.
	tm, _ = m.Update(opDoneMsg{kind: opUpdate, path: sel.repo.Path, summary: "updated " + sel.repo.Name})
	m = tm.(Model)
	updated := m.repos.Items()[m.repos.Index()].(repoItem)
	if updated.busy {
		t.Error("opDoneMsg should clear the row's busy flag")
	}
}

func TestOpenEditorReturnsCommand(t *testing.T) {
	m := seededModel(t, "alpha", "beta")

	tm, cmd := m.Update(keyPress("o"))
	m = tm.(Model)
	if cmd == nil {
		t.Fatal("o should return a command")
	}
	// Pressing o must not itself mutate any row state (unlike u/p) -- the
	// editor runs out-of-band via tea.ExecProcess.
	for _, li := range m.repos.Items() {
		if li.(repoItem).busy {
			t.Error("o should not mark any row busy")
		}
	}
}

func TestOpenEditorNoSelectionIsNoop(t *testing.T) {
	m := seededModel(t) // no repos

	tm, cmd := m.Update(keyPress("o"))
	m = tm.(Model)
	if cmd != nil {
		t.Error("o with no repos selected should be a no-op")
	}
}

func TestOpenEditorOpDoneSetsFooter(t *testing.T) {
	m := seededModel(t, "alpha")
	path := m.repos.Items()[0].(repoItem).repo.Path

	tm, _ := m.Update(opDoneMsg{kind: opOpenEditor, path: path, summary: "closed editor for alpha"})
	m = tm.(Model)
	if m.footer != "closed editor for alpha" {
		t.Errorf("footer = %q, want the editor summary", m.footer)
	}
	if m.footerErr {
		t.Error("a successful editor session should not set footerErr")
	}
}

func TestFilterSuppressesShortcuts(t *testing.T) {
	m := seededModel(t, "alpha", "beta")

	// "/" enters the list's filter mode.
	tm, _ := m.Update(keyPress("/"))
	m = tm.(Model)
	if m.repos.FilterState() != list.Filtering {
		t.Fatalf("filter state = %v, want Filtering after '/'", m.repos.FilterState())
	}

	// While filtering, "p" is typed into the filter box, not treated as prune.
	tm, _ = m.Update(keyPress("p"))
	m = tm.(Model)
	if m.confirm != nil {
		t.Error("shortcut 'p' should be suppressed while filtering")
	}
}

func TestCloneOpensGHBrowse(t *testing.T) {
	m := seededModel(t, "alpha")

	tm, _ := m.Update(keyPress("c"))
	m = tm.(Model)
	if m.screen != screenGHBrowse {
		t.Fatalf("screen = %d, want screenGHBrowse", m.screen)
	}
	if m.gh == nil {
		t.Error("gh sub-model should be created")
	}
}

func TestGHExitReturnsToRepos(t *testing.T) {
	m := seededModel(t, "alpha")
	tm, _ := m.Update(keyPress("c"))
	m = tm.(Model)

	tm, cmd := m.Update(ghExitMsg{})
	m = tm.(Model)
	if m.screen != screenRepos {
		t.Errorf("screen = %d, want screenRepos after gh exit", m.screen)
	}
	if m.gh != nil {
		t.Error("gh sub-model should be cleared on exit")
	}
	if !m.loading {
		t.Error("closing the browser should reload repos (loading=true)")
	}
	if cmd == nil {
		t.Error("closing the browser should return a reload command")
	}
}

// branchModel drills into the first repo and seeds its branch list.
func branchModel(t *testing.T, branches ...git.BranchInfo) Model {
	t.Helper()
	m := seededModel(t, "alpha")
	tm, _ := m.Update(keyPress("enter"))
	m = tm.(Model)
	tm, _ = m.Update(branchesLoadedMsg{path: m.activeRepo.Path, branches: branches})
	return tm.(Model)
}

func TestBranchesLoadedPopulatesList(t *testing.T) {
	m := branchModel(t,
		git.BranchInfo{Name: "main", Current: true},
		git.BranchInfo{Name: "feature"},
	)
	if m.branchBusy {
		t.Error("branchBusy should be false after branchesLoadedMsg")
	}
	if got := len(m.branches.Items()); got != 2 {
		t.Fatalf("branch item count = %d, want 2", got)
	}
}

func TestStaleBranchesLoadedIgnoredAfterBack(t *testing.T) {
	m := seededModel(t, "alpha")
	tm, _ := m.Update(keyPress("enter"))
	m = tm.(Model)
	activePath := m.activeRepo.Path

	// Navigate back before the branch load returns.
	tm, _ = m.Update(keyPress("esc"))
	m = tm.(Model)

	// A late branchesLoadedMsg for the abandoned repo must not populate anything.
	tm, _ = m.Update(branchesLoadedMsg{path: activePath, branches: []git.BranchInfo{{Name: "main"}}})
	m = tm.(Model)
	if got := len(m.branches.Items()); got != 0 {
		t.Errorf("stale branch load populated %d items, want 0", got)
	}
}

func TestDeleteWorktreeBranchSkipped(t *testing.T) {
	m := branchModel(t, git.BranchInfo{Name: "wt", WorktreePath: "/some/worktree"})

	tm, cmd := m.Update(keyPress("d"))
	m = tm.(Model)
	if !m.footerErr {
		t.Error("deleting a worktree-checked-out branch should set footerErr")
	}
	if !strings.Contains(m.footer, "worktree") {
		t.Errorf("footer = %q, want it to mention worktree", m.footer)
	}
	if cmd != nil {
		t.Error("no delete command should run for a worktree-checked-out branch")
	}
}

func TestOpDoneNotFullyMergedOpensForceConfirm(t *testing.T) {
	m := branchModel(t, git.BranchInfo{Name: "feature"})

	tm, _ := m.Update(opDoneMsg{
		kind:           opDeleteBranch,
		path:           m.activeRepo.Path,
		branch:         "feature",
		notFullyMerged: true,
	})
	m = tm.(Model)
	if m.confirm == nil {
		t.Fatal("a not-fully-merged delete should open a force-delete confirm")
	}
	if !strings.Contains(m.confirm.prompt, "Force delete") {
		t.Errorf("confirm prompt = %q, want a force-delete prompt", m.confirm.prompt)
	}
}

func TestOpDoneErrorSetsFooter(t *testing.T) {
	m := branchModel(t, git.BranchInfo{Name: "feature"})

	tm, _ := m.Update(opDoneMsg{
		kind:    opCheckout,
		path:    m.activeRepo.Path,
		summary: "checkout failed: boom",
		err:     errors.New("boom"),
	})
	m = tm.(Model)
	if !m.footerErr {
		t.Error("an op error should set footerErr")
	}
	if m.footer != "checkout failed: boom" {
		t.Errorf("footer = %q, want the op summary", m.footer)
	}
}

func TestCheckoutCmd(t *testing.T) {
	tests := []struct {
		name       string
		execErr    error
		wantErr    bool
		wantSubstr string
	}{
		{name: "success", execErr: nil, wantErr: false, wantSubstr: "checked out feature"},
		{name: "failure", execErr: errors.New("boom"), wantErr: true, wantSubstr: "checkout failed"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := newRepo("alpha")
			r.SetGitCommandExecutor(mockExecutor{
				fn: func(_ context.Context, _ string, _ bool, args ...string) ([]byte, error) {
					if len(args) == 0 || args[0] != "checkout" {
						t.Errorf("expected checkout, got %v", args)
					}
					return nil, tc.execErr
				},
			})

			msg := checkoutCmd(context.Background(), r, "feature")().(opDoneMsg)
			if msg.kind != opCheckout {
				t.Errorf("kind = %v, want opCheckout", msg.kind)
			}
			if (msg.err != nil) != tc.wantErr {
				t.Errorf("err = %v, wantErr = %v", msg.err, tc.wantErr)
			}
			if !strings.Contains(msg.summary, tc.wantSubstr) {
				t.Errorf("summary = %q, want it to contain %q", msg.summary, tc.wantSubstr)
			}
		})
	}
}

func TestOpenEditorCmd(t *testing.T) {
	tests := []struct {
		name       string
		editor     string
		wantErr    bool
		wantSubstr string
	}{
		{name: "success", editor: "true", wantErr: false, wantSubstr: "closed editor for alpha"},
		{name: "failure", editor: "false", wantErr: true, wantSubstr: "editor failed"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("EDITOR", tc.editor)
			r := newRepo("alpha")

			cmd := openEditorCmd(r)
			if cmd == nil {
				t.Fatal("openEditorCmd returned a nil command")
			}
			// openEditorCmd wraps tea.ExecProcess, whose Msg is only produced by
			// running the ExecCommand and invoking the callback -- exercise that
			// directly rather than through a live tea.Program.
			execCmd := exec.Command(tc.editor, r.Path)
			err := execCmd.Run()
			if (err != nil) != tc.wantErr {
				t.Fatalf("running %q on %q: err = %v, wantErr = %v", tc.editor, r.Path, err, tc.wantErr)
			}

			msg := openEditorMsg(r, err).(opDoneMsg)
			if msg.kind != opOpenEditor {
				t.Errorf("kind = %v, want opOpenEditor", msg.kind)
			}
			if (msg.err != nil) != tc.wantErr {
				t.Errorf("err = %v, wantErr = %v", msg.err, tc.wantErr)
			}
			if !strings.Contains(msg.summary, tc.wantSubstr) {
				t.Errorf("summary = %q, want it to contain %q", msg.summary, tc.wantSubstr)
			}
		})
	}
}

func TestDeleteBranchCmd(t *testing.T) {
	tests := []struct {
		name             string
		force            bool
		execErr          error
		wantErr          bool
		wantNotMerged    bool
		wantSummarySubst string
		wantFlag         string
	}{
		{
			name:             "safe delete succeeds",
			force:            false,
			execErr:          nil,
			wantSummarySubst: "deleted feature",
			wantFlag:         "-d",
		},
		{
			name:             "not fully merged",
			force:            false,
			execErr:          errors.New("error: the branch 'feature' is not fully merged"),
			wantNotMerged:    true,
			wantSummarySubst: "not fully merged",
			wantFlag:         "-d",
		},
		{
			name:             "force delete succeeds",
			force:            true,
			execErr:          nil,
			wantSummarySubst: "deleted feature",
			wantFlag:         "-D",
		},
		{
			name:             "other error surfaces",
			force:            false,
			execErr:          errors.New("permission denied"),
			wantErr:          true,
			wantSummarySubst: "delete failed",
			wantFlag:         "-d",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gotFlag string
			r := newRepo("alpha")
			r.SetGitCommandExecutor(mockExecutor{
				fn: func(_ context.Context, _ string, _ bool, args ...string) ([]byte, error) {
					// args: ["branch", "-d"|"-D", "feature"]
					if len(args) >= 2 && args[0] == "branch" {
						gotFlag = args[1]
					}
					return nil, tc.execErr
				},
			})

			msg := deleteBranchCmd(context.Background(), r, "feature", tc.force)().(opDoneMsg)

			if gotFlag != tc.wantFlag {
				t.Errorf("delete flag = %q, want %q", gotFlag, tc.wantFlag)
			}
			if msg.notFullyMerged != tc.wantNotMerged {
				t.Errorf("notFullyMerged = %v, want %v", msg.notFullyMerged, tc.wantNotMerged)
			}
			if (msg.err != nil) != tc.wantErr {
				t.Errorf("err = %v, wantErr = %v", msg.err, tc.wantErr)
			}
			if !strings.Contains(msg.summary, tc.wantSummarySubst) {
				t.Errorf("summary = %q, want it to contain %q", msg.summary, tc.wantSummarySubst)
			}
		})
	}
}

// branchRefLine builds a single for-each-ref line in the NUL-separated format
// the real git package's ListBranches expects (see branchRefFormat in
// pkg/git/repository.go): refname, HEAD marker, upstream, track, date, worktree.
func branchRefLine(name, head, upstream string) string {
	return strings.Join([]string{name, head, upstream, "", "", ""}, "\x00")
}

func TestUpdateAllCmd(t *testing.T) {
	branchLine := branchRefLine("main", "*", "origin/main")

	makeRepo := func(name string, fetchErr error) *git.Repository {
		r := newRepo(name)
		r.Path = "/tmp"
		r.SetGitCommandExecutor(mockExecutor{
			fn: func(_ context.Context, _ string, _ bool, args ...string) ([]byte, error) {
				if len(args) == 0 {
					return nil, errors.New("unexpected empty command")
				}
				switch args[0] {
				case "rev-parse":
					return []byte("main"), nil
				case "fetch":
					return nil, fetchErr
				case "status":
					return []byte(""), nil
				case "for-each-ref":
					return []byte(branchLine), nil
				case "stash":
					return []byte(""), nil
				case "checkout":
					return []byte(""), nil
				}
				return nil, errors.New("unexpected command")
			},
		})
		return r
	}

	repos := []*git.Repository{
		makeRepo("alpha", nil),
		makeRepo("beta", errors.New("boom")),
	}

	msg := updateAllCmd(context.Background(), repos)().(bulkOpDoneMsg)

	if msg.kind != opUpdate {
		t.Errorf("kind = %v, want opUpdate", msg.kind)
	}
	if len(msg.results) != 2 {
		t.Fatalf("results len = %d, want 2", len(msg.results))
	}
	failed := 0
	for _, r := range msg.results {
		if r.err != nil {
			failed++
		}
	}
	if failed != 1 {
		t.Errorf("failed count = %d, want 1", failed)
	}
	if !strings.Contains(msg.summary, "1/2") || !strings.Contains(msg.summary, "1 failed") {
		t.Errorf("summary = %q, want it to mention 1/2 and 1 failed", msg.summary)
	}
}

func TestPruneAllCmd(t *testing.T) {
	branchLine := branchRefLine("main", "*", "origin/main")

	makeRepo := func(name string, statusErr error) *git.Repository {
		r := newRepo(name)
		r.Path = "/tmp"
		r.SetGitCommandExecutor(mockExecutor{
			fn: func(_ context.Context, _ string, _ bool, args ...string) ([]byte, error) {
				if len(args) == 0 {
					return nil, errors.New("unexpected empty command")
				}
				switch args[0] {
				case "status":
					return []byte(""), statusErr
				case "for-each-ref":
					return []byte(branchLine), nil
				case "stash":
					return []byte(""), nil
				case "symbolic-ref":
					return []byte("origin/main"), nil
				}
				return nil, errors.New("unexpected command")
			},
		})
		return r
	}

	repos := []*git.Repository{
		makeRepo("alpha", nil),
		makeRepo("beta", errors.New("boom")),
	}

	msg := pruneAllCmd(context.Background(), repos)().(bulkOpDoneMsg)

	if msg.kind != opDeleteBranch {
		t.Errorf("kind = %v, want opDeleteBranch", msg.kind)
	}
	if len(msg.results) != 2 {
		t.Fatalf("results len = %d, want 2", len(msg.results))
	}
	failed := 0
	for _, r := range msg.results {
		if r.err != nil {
			failed++
		}
	}
	if failed != 1 {
		t.Errorf("failed count = %d, want 1", failed)
	}
	if !strings.Contains(msg.summary, "1/2") {
		t.Errorf("summary = %q, want it to mention 1/2", msg.summary)
	}
}
