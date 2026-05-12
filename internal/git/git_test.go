package git

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestInspect_NotARepo(t *testing.T) {
	dir := t.TempDir()
	info, err := Inspect(dir)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if info.IsRepo {
		t.Errorf("IsRepo = true, want false for non-repo dir")
	}
	if info.Display() != "" {
		t.Errorf("Display = %q, want empty", info.Display())
	}
}

func TestInspect_StandardRepoOnBranch(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".git", "HEAD"), "ref: refs/heads/main\n")

	info, err := Inspect(dir)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if !info.IsRepo {
		t.Errorf("IsRepo = false, want true")
	}
	if info.Branch != "main" {
		t.Errorf("Branch = %q, want %q", info.Branch, "main")
	}
	if info.ShortSHA != "" {
		t.Errorf("ShortSHA = %q, want empty for branch checkout", info.ShortSHA)
	}
	if info.Display() != "main" {
		t.Errorf("Display = %q, want %q", info.Display(), "main")
	}
}

func TestInspect_BranchWithSlash(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".git", "HEAD"), "ref: refs/heads/feature/cross-repo-tasks\n")

	info, err := Inspect(dir)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if info.Branch != "feature/cross-repo-tasks" {
		t.Errorf("Branch = %q, want %q", info.Branch, "feature/cross-repo-tasks")
	}
}

func TestInspect_DetachedHEAD(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".git", "HEAD"), "a1b2c3d4e5f60718293a4b5c6d7e8f9012345678\n")

	info, err := Inspect(dir)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if !info.IsRepo {
		t.Errorf("IsRepo = false, want true")
	}
	if info.Branch != "" {
		t.Errorf("Branch = %q, want empty for detached HEAD", info.Branch)
	}
	if info.ShortSHA != "a1b2c3d" {
		t.Errorf("ShortSHA = %q, want %q", info.ShortSHA, "a1b2c3d")
	}
	if info.Display() != "@a1b2c3d" {
		t.Errorf("Display = %q, want %q", info.Display(), "@a1b2c3d")
	}
}

func TestInspect_Worktree(t *testing.T) {
	// Linked worktree: .git is a file containing "gitdir: <path>".
	dir := t.TempDir()
	realGitDir := filepath.Join(t.TempDir(), "real-git-dir")
	writeFile(t, filepath.Join(realGitDir, "HEAD"), "ref: refs/heads/worktree-branch\n")
	writeFile(t, filepath.Join(dir, ".git"), "gitdir: "+realGitDir+"\n")

	info, err := Inspect(dir)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if info.Branch != "worktree-branch" {
		t.Errorf("Branch = %q, want %q", info.Branch, "worktree-branch")
	}
}

func TestInspect_WorktreeRelativePath(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "real-git-dir", "HEAD"), "ref: refs/heads/rel-branch\n")
	writeFile(t, filepath.Join(dir, ".git"), "gitdir: real-git-dir\n")

	info, err := Inspect(dir)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if info.Branch != "rel-branch" {
		t.Errorf("Branch = %q, want %q", info.Branch, "rel-branch")
	}
}

func TestRecentCommits_NotARepo(t *testing.T) {
	commits, err := RecentCommits(t.TempDir(), 5)
	if err != nil {
		t.Fatalf("RecentCommits: %v", err)
	}
	if commits != nil {
		t.Errorf("expected nil, got %#v", commits)
	}
}

func TestRecentCommits_NoReflog(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	commits, err := RecentCommits(dir, 5)
	if err != nil {
		t.Fatalf("RecentCommits: %v", err)
	}
	if commits != nil {
		t.Errorf("expected nil for missing reflog, got %#v", commits)
	}
}

func TestRecentCommits_ParsesAndOrders(t *testing.T) {
	dir := t.TempDir()
	// Reflog format: <old> <new> <name> <email> <unix-ts> <tz>\t<message>
	// Oldest line first, newest last (as git writes them).
	reflog := strings.Join([]string{
		"0000000000000000000000000000000000000000 aaaaaaa1111111111111111111111111111111111 A B <a@b> 1736276400 -0500\tcommit (initial): first commit",
		"aaaaaaa1111111111111111111111111111111111 bbbbbbb2222222222222222222222222222222222 A B <a@b> 1736276500 -0500\tcheckout: moving from master to main",
		"bbbbbbb2222222222222222222222222222222222 ccccccc3333333333333333333333333333333333 A B <a@b> 1736276600 -0500\tcommit: feat: add the thing",
		"ccccccc3333333333333333333333333333333333 ddddddd4444444444444444444444444444444444 A B <a@b> 1736276700 -0500\tcommit (amend): feat: amended the thing",
	}, "\n")
	writeFile(t, filepath.Join(dir, ".git", "logs", "HEAD"), reflog+"\n")

	commits, err := RecentCommits(dir, 5)
	if err != nil {
		t.Fatalf("RecentCommits: %v", err)
	}
	// 3 commit entries (initial + commit + amend); 1 checkout filtered out.
	if len(commits) != 3 {
		t.Fatalf("got %d commits, want 3 (checkout should be filtered): %#v", len(commits), commits)
	}
	// Newest first.
	if commits[0].Subject != "feat: amended the thing" {
		t.Errorf("commits[0].Subject = %q, want %q", commits[0].Subject, "feat: amended the thing")
	}
	if commits[0].ShortSHA != "ddddddd" {
		t.Errorf("commits[0].ShortSHA = %q, want %q", commits[0].ShortSHA, "ddddddd")
	}
	if commits[2].Subject != "first commit" {
		t.Errorf("commits[2].Subject = %q, want %q (initial commit)", commits[2].Subject, "first commit")
	}
	// Times decreasing.
	if !commits[0].Time.After(commits[1].Time) {
		t.Errorf("expected commits[0].Time after commits[1].Time; got %v, %v", commits[0].Time, commits[1].Time)
	}
}

func TestRecentCommits_LimitApplied(t *testing.T) {
	dir := t.TempDir()
	var lines []string
	for i := 1; i <= 10; i++ {
		old := "0000000000000000000000000000000000000000"
		if i > 1 {
			old = strings.Repeat(string(rune('a'+i-1)), 40)
		}
		new := strings.Repeat(string(rune('a'+i)), 40)
		lines = append(lines, old+" "+new+" A B <a@b> 1736276400 -0500\tcommit: msg "+strconv.Itoa(i))
	}
	writeFile(t, filepath.Join(dir, ".git", "logs", "HEAD"), strings.Join(lines, "\n")+"\n")

	commits, err := RecentCommits(dir, 3)
	if err != nil {
		t.Fatalf("RecentCommits: %v", err)
	}
	if len(commits) != 3 {
		t.Fatalf("got %d commits, want 3", len(commits))
	}
	// Newest three messages.
	if commits[0].Subject != "msg 10" || commits[1].Subject != "msg 9" || commits[2].Subject != "msg 8" {
		t.Errorf("got subjects [%q, %q, %q], want [msg 10, msg 9, msg 8]",
			commits[0].Subject, commits[1].Subject, commits[2].Subject)
	}
}

func TestRecentCommits_ZeroLimit(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".git", "logs", "HEAD"),
		"0000000000000000000000000000000000000000 aaaaaaa1111111111111111111111111111111111 A B <a@b> 1736276400 -0500\tcommit: x\n")
	commits, err := RecentCommits(dir, 0)
	if err != nil {
		t.Fatalf("RecentCommits: %v", err)
	}
	if commits != nil {
		t.Errorf("expected nil for zero limit, got %#v", commits)
	}
}

func TestInspect_MissingHEAD(t *testing.T) {
	// .git directory exists but HEAD is missing — broken state, surface as IsRepo only.
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	info, err := Inspect(dir)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if !info.IsRepo {
		t.Errorf("IsRepo = false, want true")
	}
	if info.Display() != "" {
		t.Errorf("Display = %q, want empty", info.Display())
	}
}
