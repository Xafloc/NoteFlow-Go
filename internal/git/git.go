// Package git exposes the small slice of git state NoteFlow shows in the UI.
//
// It deliberately reads .git/HEAD directly rather than shelling out to `git`,
// so NoteFlow has no runtime dependency on a git binary. This supports the
// "boring stack" goal — see docs/TODO.md → "Long-term Direction" goal 3.
package git

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// RepoInfo is the subset of git state surfaced to the web UI.
type RepoInfo struct {
	IsRepo   bool   // true when basePath sits inside a git repo
	Branch   string // current branch name; empty when detached or not a repo
	ShortSHA string // 7-char commit SHA when in detached HEAD; empty otherwise
}

// Inspect reads basePath/.git/HEAD and returns the current branch (or short SHA
// for detached HEAD). When basePath is not a git repo, returns a zero-value
// RepoInfo with no error — "no repo here" is not an error condition.
func Inspect(basePath string) (RepoInfo, error) {
	gitPath := filepath.Join(basePath, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		if os.IsNotExist(err) {
			return RepoInfo{}, nil
		}
		return RepoInfo{}, err
	}

	headPath, err := resolveHeadPath(basePath, gitPath, info)
	if err != nil {
		return RepoInfo{}, err
	}
	if headPath == "" {
		return RepoInfo{}, nil
	}

	data, err := os.ReadFile(headPath)
	if err != nil {
		if os.IsNotExist(err) {
			// .git exists but HEAD is missing — treat as broken repo, surface as not-a-repo.
			return RepoInfo{IsRepo: true}, nil
		}
		return RepoInfo{IsRepo: true}, err
	}

	content := strings.TrimSpace(string(data))
	if rest, ok := strings.CutPrefix(content, "ref:"); ok {
		ref := strings.TrimSpace(rest)
		branch := strings.TrimPrefix(ref, "refs/heads/")
		return RepoInfo{IsRepo: true, Branch: branch}, nil
	}

	// Detached HEAD: HEAD contains a raw SHA.
	sha := content
	if len(sha) > 7 {
		sha = sha[:7]
	}
	return RepoInfo{IsRepo: true, ShortSHA: sha}, nil
}

// resolveHeadPath handles both standard repos (.git is a directory) and linked
// worktrees (.git is a file containing "gitdir: <path>"). Returns the absolute
// path to the HEAD file. An empty return means we couldn't resolve a HEAD;
// callers should treat that as "not a repo we recognize."
func resolveHeadPath(basePath, gitPath string, info os.FileInfo) (string, error) {
	if info.IsDir() {
		return filepath.Join(gitPath, "HEAD"), nil
	}

	data, err := os.ReadFile(gitPath)
	if err != nil {
		return "", err
	}
	line := strings.TrimSpace(string(data))
	rest, ok := strings.CutPrefix(line, "gitdir:")
	if !ok {
		return "", nil
	}
	gitDir := strings.TrimSpace(rest)
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(basePath, gitDir)
	}
	return filepath.Join(gitDir, "HEAD"), nil
}

// Display returns a short human label for the UI: "branch-name", "@sha1234",
// or "" when there is nothing to show.
func (r RepoInfo) Display() string {
	if r.Branch != "" {
		return r.Branch
	}
	if r.ShortSHA != "" {
		return "@" + r.ShortSHA
	}
	return ""
}

// CommitInfo is a single commit entry extracted from the reflog.
type CommitInfo struct {
	ShortSHA string    // 7-char abbreviated commit SHA
	Subject  string    // commit message, with the "commit: " / "commit (initial): " prefix stripped
	Time     time.Time // commit time
}

// RecentCommits returns up to `limit` most-recent commit entries from
// basePath/.git/logs/HEAD, newest first. Only entries whose reflog message
// begins with "commit" (commit, commit (initial), commit (merge),
// commit (amend), etc.) are returned — checkouts, resets, and other HEAD
// movements are filtered out.
//
// Returns nil with no error when basePath is not a git repo or the reflog
// is missing — "no commits to show" is not an error condition.
//
// Note: this reads HEAD's reflog, which records every commit the user has
// made while HEAD pointed anywhere in this clone — not strictly "commits
// reachable from the current branch tip." For the UI's "recent activity"
// surface this is the right shape; reachability would require walking the
// object graph, which we explicitly avoid.
func RecentCommits(basePath string, limit int) ([]CommitInfo, error) {
	if limit <= 0 {
		return nil, nil
	}

	gitPath := filepath.Join(basePath, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var logPath string
	if info.IsDir() {
		logPath = filepath.Join(gitPath, "logs", "HEAD")
	} else {
		// Linked worktree: follow the gitdir pointer.
		data, err := os.ReadFile(gitPath)
		if err != nil {
			return nil, err
		}
		line := strings.TrimSpace(string(data))
		rest, ok := strings.CutPrefix(line, "gitdir:")
		if !ok {
			return nil, nil
		}
		gitDir := strings.TrimSpace(rest)
		if !filepath.IsAbs(gitDir) {
			gitDir = filepath.Join(basePath, gitDir)
		}
		logPath = filepath.Join(gitDir, "logs", "HEAD")
	}

	f, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	// Reflogs are typically small (KB), so reading line-by-line is fine.
	// Bump the scanner buffer so unusually long messages don't truncate.
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var all []CommitInfo
	for scanner.Scan() {
		if c, ok := parseReflogLine(scanner.Text()); ok {
			all = append(all, c)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Reflog is oldest-first; we want newest-first.
	if len(all) > limit {
		all = all[len(all)-limit:]
	}
	out := make([]CommitInfo, len(all))
	for i := range all {
		out[i] = all[len(all)-1-i]
	}
	return out, nil
}

// parseReflogLine extracts a CommitInfo from one reflog line, or returns
// ok=false for lines that aren't commits.
//
// Format:
//
//	<old-sha> <new-sha> <committer-name> <committer-email> <unix-ts> <tz>\t<message>
//
// We split on the literal tab between the metadata and the message, then
// peel fields off the right side of the metadata so the committer name
// (which may contain spaces) stays intact.
func parseReflogLine(line string) (CommitInfo, bool) {
	tabIdx := strings.IndexByte(line, '\t')
	if tabIdx == -1 {
		return CommitInfo{}, false
	}
	meta := line[:tabIdx]
	message := line[tabIdx+1:]

	if !strings.HasPrefix(message, "commit") {
		return CommitInfo{}, false
	}
	// Strip the "commit: " or "commit (foo): " prefix.
	colonIdx := strings.IndexByte(message, ':')
	if colonIdx == -1 || colonIdx+2 > len(message) {
		return CommitInfo{}, false
	}
	subject := strings.TrimSpace(message[colonIdx+1:])

	// Peel timezone, unix-ts off the right of the metadata.
	fields := strings.Fields(meta)
	if len(fields) < 4 {
		return CommitInfo{}, false
	}
	tsStr := fields[len(fields)-2]
	unixSec, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return CommitInfo{}, false
	}
	newSHA := fields[1]
	if len(newSHA) < 7 {
		return CommitInfo{}, false
	}

	return CommitInfo{
		ShortSHA: newSHA[:7],
		Subject:  subject,
		Time:     time.Unix(unixSec, 0),
	}, true
}
