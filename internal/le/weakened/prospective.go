// Design: docs/architecture/testing/test-health.md -- prospective isolated-index weakening audit
// Related: weakened.go -- comparison and ledger pairing over the exact commit population.
package weakened

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// Prospective describes the exact add/remove population as Git would record it.
type Prospective struct {
	Paths       []string     `json:"paths"`
	Removed     []string     `json:"removed"`
	RenamePairs []RenamePair `json:"rename-pairs,omitempty"`
}

// ProspectiveCommit builds an isolated Git index and derives safe test rename
// pairs without touching the shared staging area.
func ProspectiveCommit(root string, paths, removed []string) (Prospective, []string) {
	result := Prospective{Paths: slices.Clone(paths), Removed: slices.Clone(removed)}
	raw, problem := prospectiveDiff(root, paths, removed,
		"--name-status", "-z", "--find-renames=1%", "-l0", "--diff-filter=R")
	if problem != "" {
		return result, []string{cannotRunPrefix + problem + ", so no rename was compared"}
	}
	pairs, problem := parseRenamePairs(raw)
	if problem != "" {
		return result, []string{cannotRunPrefix + problem}
	}
	accepted, problem := acceptedRenamePairs(paths, removed, pairs)
	if problem != "" {
		return result, []string{cannotRunPrefix + problem}
	}
	for _, pair := range accepted {
		if isTestPath(pair.OldPath) && isTestPath(pair.NewPath) {
			result.RenamePairs = append(result.RenamePairs, pair)
		}
	}
	return result, nil
}

func prospectiveDiff(root string, paths, removed []string, diffArgs ...string) ([]byte, string) {
	tmp := filepath.Join(root, "tmp")
	if err := os.MkdirAll(tmp, 0o750); err != nil {
		return nil, fmt.Sprintf("create prospective-index directory: %v", err)
	}
	file, err := os.CreateTemp(tmp, "ze-commit-index-")
	if err != nil {
		return nil, fmt.Sprintf("reserve prospective index: %v", err)
	}
	index := file.Name()
	if err := file.Close(); err != nil {
		return nil, fmt.Sprintf("close prospective index reservation: %v", err)
	}
	if err := os.Remove(index); err != nil {
		return nil, fmt.Sprintf("release prospective index reservation: %v", err)
	}
	defer os.Remove(index)           //nolint:errcheck // isolated scratch index
	defer os.Remove(index + ".lock") //nolint:errcheck // interrupted Git scratch lock

	environment := append(os.Environ(), "GIT_INDEX_FILE="+index)
	run := func(args ...string) ([]byte, int, error) {
		command := exec.CommandContext(context.Background(), "git", args...) // #nosec G204 -- Git and every subcommand are closed; paths remain argv.
		command.Dir = root
		command.Env = environment
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		command.Stdout = &stdout
		command.Stderr = &stderr
		runErr := command.Run()
		if runErr == nil {
			return stdout.Bytes(), 0, nil
		}
		if exit, ok := errors.AsType[*exec.ExitError](runErr); ok {
			return stderr.Bytes(), exit.ExitCode(), nil
		}
		return stderr.Bytes(), -1, runErr
	}

	_, code, runErr := run("rev-parse", "--verify", "-q", "HEAD")
	if runErr != nil {
		return nil, fmt.Sprintf("git rev-parse HEAD: %v", runErr)
	}
	if code == 0 {
		output, readCode, readErr := run("read-tree", "HEAD")
		if readErr != nil {
			return nil, fmt.Sprintf("git read-tree HEAD: %v", readErr)
		}
		if readCode != 0 {
			return nil, "git read-tree HEAD failed for the prospective commit: " + strings.TrimSpace(string(output))
		}
	}
	if len(paths) != 0 {
		args := append([]string{"update-index", "--add", "--remove", "--"}, paths...)
		output, updateCode, updateErr := run(args...)
		if updateErr != nil {
			return nil, fmt.Sprintf("git update-index: %v", updateErr)
		}
		if updateCode != 0 {
			return nil, "git update-index failed for the prospective commit: " + strings.TrimSpace(string(output))
		}
	}
	if len(removed) != 0 {
		args := append([]string{"update-index", "--force-remove", "--"}, removed...)
		output, removeCode, removeErr := run(args...)
		if removeErr != nil {
			return nil, fmt.Sprintf("git update-index --force-remove: %v", removeErr)
		}
		if removeCode != 0 {
			return nil, "git update-index --force-remove failed for the prospective commit: " + strings.TrimSpace(string(output))
		}
	}
	args := append([]string{"diff", "--cached"}, diffArgs...)
	output, diffCode, diffErr := run(args...)
	if diffErr != nil {
		return nil, fmt.Sprintf("git diff --cached: %v", diffErr)
	}
	if diffCode != 0 {
		return nil, "git diff --cached failed for the prospective commit: " + strings.TrimSpace(string(output))
	}
	return output, ""
}

func parseRenamePairs(raw []byte) ([]RenamePair, string) {
	fields := bytes.Split(raw, []byte{0})
	if len(fields) != 0 && len(fields[len(fields)-1]) == 0 {
		fields = fields[:len(fields)-1]
	}
	pairs := make([]RenamePair, 0, len(fields)/3)
	for index := 0; index < len(fields); index += 3 {
		if index+2 >= len(fields) {
			return nil, "Git returned malformed or ambiguous rename status, so no rename was compared"
		}
		status := string(fields[index])
		if !strings.HasPrefix(status, "R") {
			return nil, "Git returned malformed or ambiguous rename status, so no rename was compared"
		}
		score, err := strconv.Atoi(strings.TrimPrefix(status, "R"))
		if err != nil {
			return nil, "Git returned malformed or ambiguous rename status, so no rename was compared"
		}
		pairs = append(pairs, RenamePair{
			OldPath: string(fields[index+1]), NewPath: string(fields[index+2]), Score: score,
		})
	}
	return pairs, ""
}

func acceptedRenamePairs(paths, removed []string, pairs []RenamePair) ([]RenamePair, string) {
	accepted := make([]RenamePair, 0, len(pairs))
	usedOld := make(map[string]bool)
	usedNew := make(map[string]bool)
	lowByName := make(map[string][]RenamePair)
	for _, pair := range pairs {
		if pair.Score >= 50 {
			accepted = append(accepted, pair)
			usedOld[pair.OldPath] = true
			usedNew[pair.NewPath] = true
			continue
		}
		name := filepath.Base(pair.OldPath)
		if name != filepath.Base(pair.NewPath) {
			continue
		}
		if commonSuffixComponents(pair.OldPath, pair.NewPath) < 2 {
			continue
		}
		lowByName[name] = append(lowByName[name], pair)
	}

	names := make([]string, 0, len(lowByName))
	for name := range lowByName {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		oldCandidates := filterBasename(removed, name, usedOld)
		newCandidates := filterBasename(paths, name, usedNew)
		optimum, unique := uniqueSuffixPairing(oldCandidates, newCandidates)
		if !unique {
			return nil, "low-similarity rename pairing is ambiguous for basename " + strconv.Quote(name) + ": the maximum common-suffix pairing is not unique"
		}
		scored := make(map[[2]string]RenamePair, len(lowByName[name]))
		for _, pair := range lowByName[name] {
			scored[[2]string{pair.OldPath, pair.NewPath}] = pair
		}
		if len(scored) != len(optimum) {
			return nil, "Git's low-similarity rename pairs conflict with the unique common-suffix pairing for basename " + strconv.Quote(name)
		}
		for _, paths := range optimum {
			pair, ok := scored[paths]
			if !ok {
				return nil, "Git's low-similarity rename pairs conflict with the unique common-suffix pairing for basename " + strconv.Quote(name)
			}
			accepted = append(accepted, pair)
		}
	}
	sort.Slice(accepted, func(left, right int) bool {
		if accepted[left].OldPath == accepted[right].OldPath {
			return accepted[left].NewPath < accepted[right].NewPath
		}
		return accepted[left].OldPath < accepted[right].OldPath
	})
	return accepted, ""
}

func filterBasename(paths []string, name string, used map[string]bool) []string {
	filtered := make([]string, 0)
	for _, path := range paths {
		if used[path] {
			continue
		}
		if filepath.Base(path) == name {
			filtered = append(filtered, path)
		}
	}
	return filtered
}

func commonSuffixComponents(oldPath, newPath string) int {
	oldParts := strings.Split(filepath.ToSlash(oldPath), "/")
	newParts := strings.Split(filepath.ToSlash(newPath), "/")
	count := 0
	for count < len(oldParts) && count < len(newParts) {
		if oldParts[len(oldParts)-count-1] != newParts[len(newParts)-count-1] {
			break
		}
		count++
	}
	return count
}

func uniqueSuffixPairing(oldPaths, newPaths []string) ([][2]string, bool) {
	pairs, _, _, unique := pairAtDepth(slices.Clone(oldPaths), slices.Clone(newPaths), 0)
	sort.Slice(pairs, func(left, right int) bool {
		if pairs[left][0] == pairs[right][0] {
			return pairs[left][1] < pairs[right][1]
		}
		return pairs[left][0] < pairs[right][0]
	})
	return pairs, unique
}

func pairAtDepth(oldPaths, newPaths []string, depth int) ([][2]string, []string, []string, bool) {
	oldGroups := groupAtDepth(oldPaths, depth)
	newGroups := groupAtDepth(newPaths, depth)
	keys := make([]string, 0)
	for key := range oldGroups {
		if key != "" && len(newGroups[key]) != 0 {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	shared := make(map[string]bool, len(keys))
	pairs := make([][2]string, 0)
	oldLeft := make([]string, 0)
	newLeft := make([]string, 0)
	unique := true
	for _, key := range keys {
		shared[key] = true
		childPairs, childOld, childNew, childUnique := pairAtDepth(oldGroups[key], newGroups[key], depth+1)
		pairs = append(pairs, childPairs...)
		oldLeft = append(oldLeft, childOld...)
		newLeft = append(newLeft, childNew...)
		if !childUnique {
			unique = false
		}
	}
	for key, group := range oldGroups {
		if !shared[key] {
			oldLeft = append(oldLeft, group...)
		}
	}
	for key, group := range newGroups {
		if !shared[key] {
			newLeft = append(newLeft, group...)
		}
	}
	sort.Strings(oldLeft)
	sort.Strings(newLeft)
	matched := 0
	if depth >= 2 {
		matched = min(len(oldLeft), len(newLeft))
	}
	if matched != 0 && (len(oldLeft) != 1 || len(newLeft) != 1) {
		unique = false
	}
	for index := range matched {
		pairs = append(pairs, [2]string{oldLeft[index], newLeft[index]})
	}
	return pairs, oldLeft[matched:], newLeft[matched:], unique
}

func groupAtDepth(paths []string, depth int) map[string][]string {
	groups := make(map[string][]string)
	for _, path := range paths {
		parts := strings.Split(filepath.ToSlash(path), "/")
		component := ""
		if depth < len(parts) {
			component = parts[len(parts)-depth-1]
		}
		groups[component] = append(groups[component], path)
	}
	return groups
}
