package git

import (
	"fmt"
	"os/exec"
	"strings"
)

type RepoStatus int

const (
	RepoSynced   RepoStatus = iota
	RepoConflict RepoStatus = iota
)

// CheckStatus inspects the working tree for merge conflicts. A clean tree
// (or only staged/unstaged changes) returns RepoSynced.
func CheckStatus(repoPath string) RepoStatus {
	out, err := exec.Command("git", "-C", repoPath, "status", "--porcelain").Output()
	if err != nil {
		return RepoConflict
	}
	for _, line := range strings.Split(string(out), "\n") {
		if len(line) < 2 {
			continue
		}
		switch line[:2] {
		case "UU", "AA", "DD", "AU", "UA", "DU", "UD":
			return RepoConflict
		}
	}
	return RepoSynced
}

func run(repoPath string, args ...string) error {
	cmd := exec.Command("git", append([]string{"-C", repoPath}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %v: %w\n%s", args, err, out)
	}
	return nil
}

func CommitAndPush(repoPath, message string) error {
	if err := run(repoPath, "add", "notes/", "identity.age"); err != nil {
		return err
	}
	// commit may be a no-op if nothing changed
	cmd := exec.Command("git", "-C", repoPath, "commit", "-m", message)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if isNothingToCommit(out) {
			return nil
		}
		return fmt.Errorf("git commit: %w\n%s", err, out)
	}
	return run(repoPath, "push")
}

func isNothingToCommit(out []byte) bool {
	return contains(out, "nothing to commit") || contains(out, "nothing added to commit")
}

func contains(b []byte, s string) bool {
	return len(b) > 0 && indexOf(b, []byte(s)) >= 0
}

func indexOf(haystack, needle []byte) int {
	for i := range haystack {
		if i+len(needle) <= len(haystack) {
			match := true
			for j := range needle {
				if haystack[i+j] != needle[j] {
					match = false
					break
				}
			}
			if match {
				return i
			}
		}
	}
	return -1
}
