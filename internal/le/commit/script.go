// Design: docs/features/ai-first.md -- atomic add/remove/commit script generation
//
// This file is the only native source that spells the raw staging and commit
// verbs. They are emitted into the generated script, never executed here.
package commit

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	scriptMarker = "# ze-commit-script:"
	blockMarker  = "# ze-commit-block:"
	pushMarker   = "# ze-commit-push:"
)

type commitBlock struct {
	Tag         string
	Subject     string
	Paths       []string
	Removed     []string
	MessagePath string
	ReviewCheck string
}

func renderBlock(block commitBlock, scriptPath string) string {
	all := append(append([]string{}, block.Paths...), block.Removed...)
	lines := []string{
		commentLine("Commit " + block.Tag + ": " + block.Subject),
		blockMarker + " tag=" + block.Tag + " paths=" + quotePaths(all),
	}
	if block.ReviewCheck != "" {
		lines = append(lines, "# critical-review gate re-check", block.ReviewCheck)
	}
	// The guard runs BEFORE anything stages. A script that adds first and
	// refuses second leaves the index dirtier than it found it: the abort
	// reports the other session's paths while this session's are now staged
	// too, so both scripts refuse each other and neither can proceed. Clearing
	// that needs `git restore --staged`, which no agent may run, so the
	// deadlock reaches the owner.
	//
	// The guard reads the same index either way. Its exclusion list names this
	// commit's own paths, which are simply not staged yet at this point, so
	// moving it up costs it nothing and it still sees every foreign path.
	if guard := renderStagingGuard(all, scriptPath); guard != "" {
		lines = append(lines, guard)
	}
	if len(block.Paths) != 0 {
		lines = append(lines, renderAdd(block.Paths))
	}
	if len(block.Removed) != 0 {
		lines = append(lines, "git rm -- "+quotePaths(block.Removed))
	}
	lines = append(lines, "git commit -F "+shellQuote(block.MessagePath))
	return strings.Join(lines, "\n") + "\n"
}

func renderAdd(paths []string) string {
	lines := make([]string, 0, 1+len(paths))
	// -f is what lets a TRACKED file under an ignored directory be staged. Git
	// reports the whole pathspec as ignored and exits 1 without it, even though
	// it staged every path, which stops the script between the add and the
	// commit and leaves the index full. Forcing costs nothing here: every path
	// in this script passed validateAddPath, which already refuses a path git
	// ignores with the index consulted.
	lines = append(lines, "git add -f -- \\")
	for index, path := range paths {
		suffix := " \\"
		if index+1 == len(paths) {
			suffix = ""
		}
		lines = append(lines, "  "+shellQuote(path)+suffix)
	}
	return strings.Join(lines, "\n")
}

func renderStagingGuard(paths []string, scriptPath string) string {
	if len(paths) == 0 {
		return ""
	}
	expectedPaths := append([]string{}, paths...)
	sort.Strings(expectedPaths)
	expected := make([]string, 0, len(expectedPaths)*2)
	for _, path := range expectedPaths {
		expected = append(expected, "-e", shellQuote(path))
	}
	lines := []string{
		"# Concurrency guard: refuse a concurrent session's staged files.",
		"_ze_foreign=$(git -c core.quotePath=false diff --cached --name-only | grep -vxF " + strings.Join(expected, " ") + " || true)",
		`if [ -n "$_ze_foreign" ]; then`,
		`  echo "ABORT: index has staged files not in this commit (concurrent session?):" >&2`,
		"  echo " + shellQuote("  this script: "+scriptPath) + " >&2",
		`  echo "$_ze_foreign" >&2`,
		"  exit 1",
		"fi",
	}
	return strings.Join(lines, "\n")
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func quotePaths(paths []string) string {
	quoted := make([]string, len(paths))
	for index, path := range paths {
		quoted[index] = shellQuote(path)
	}
	return strings.Join(quoted, " ")
}

func commentSafe(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func markerLine(marker, payload string) string {
	return marker + " " + commentSafe(payload)
}

func commentLine(value string) string {
	return markerLine("#", value)
}

func renderPush(authorisation string) string {
	return markerLine(pushMarker, authorisation) + "\n" +
		"# Push authorized by the owner. Reached only after every commit succeeded.\n" +
		"git push\n"
}

func splitPush(text string) (string, string, error) {
	lines := strings.SplitAfter(text, "\n")
	found := -1
	for index, line := range lines {
		if strings.HasPrefix(line, pushMarker) {
			if found >= 0 {
				return "", "", errors.New("refusing a script with more than one push marker")
			}
			found = index
		}
	}
	if found < 0 {
		return text, "", nil
	}
	authorisation := commentSafe(strings.TrimPrefix(strings.TrimSpace(lines[found]), pushMarker))
	tail := strings.Join(lines[found:], "")
	if strings.TrimSpace(tail) != strings.TrimSpace(renderPush(authorisation)) {
		return "", "", errors.New("refusing a script whose push marker is not its final section")
	}
	body := strings.TrimRight(strings.Join(lines[:found], ""), "\n")
	if body != "" {
		body += "\n"
	}
	return body, authorisation, nil
}

func pushAuthorisation(reason string) (string, error) {
	if reason == "" {
		return "", nil
	}
	reason = commentSafe(reason)
	if len(reason) < 12 {
		return "", fmt.Errorf("push authorisation is too short: %q is %d characters, 12 is the minimum", reason, len(reason))
	}
	return reason, nil
}

// validateTag reports whether a caller-supplied tag can name a message file.
// Create MUST call it before it records verification debt: nextTag runs after
// that record is written, so a tag refused there leaves rows naming a commit
// that was never made (plan/journal/record-written-before-the-operation-succeeds.md).
func validateTag(requested string) error {
	if requested == "" {
		return nil
	}
	// The length is reported on its own. tagPattern carries a {0,31} bound as
	// well as a character class, and a message naming only the class sends the
	// author looking for a bad character in a tag that has none.
	if len(requested) > tagMaxLength {
		return fmt.Errorf("tag is %d characters, %d over the %d limit: %s",
			len(requested), len(requested)-tagMaxLength, tagMaxLength, requested)
	}
	if !tagPattern.MatchString(requested) {
		return errors.New("tag must start with an alphanumeric character and contain only alnum, dot, underscore, or dash")
	}
	return nil
}

func nextTag(root, session, requested string) (string, string, error) {
	if requested != "" {
		if err := validateTag(requested); err != nil {
			return "", "", err
		}
		return requested, filepath.ToSlash(filepath.Join("tmp", "commit-msg-"+session+"-"+requested+".txt")), nil
	}
	for code := byte('a'); code <= byte('z'); code++ {
		tag := string(code)
		relative := filepath.ToSlash(filepath.Join("tmp", "commit-msg-"+session+"-"+tag+".txt"))
		file, err := os.OpenFile(filepath.Join(root, filepath.FromSlash(relative)), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600) //nolint:gosec // the path is this session's commit artifact or a tracked file under the checkout root
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", "", err
		}
		if err := file.Close(); err != nil {
			return "", "", err
		}
		return tag, relative, nil
	}
	return "", "", errors.New("no free message tag; clear old tmp/commit-msg-* files")
}

func allocateScript(root, session, tag string) (string, error) {
	for range 64 {
		var nonce [3]byte
		if _, err := rand.Read(nonce[:]); err != nil {
			return "", err
		}
		relative := filepath.ToSlash(filepath.Join("tmp", "commit-"+session+"-"+tag+"-"+hex.EncodeToString(nonce[:])+".sh"))
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); errors.Is(err, os.ErrNotExist) {
			return relative, nil
		}
	}
	return "", errors.New("cannot allocate an unused commit script path")
}

func declaredPaths(text string) map[string]bool {
	paths := make(map[string]bool)
	for line := range strings.SplitSeq(text, "\n") {
		if !strings.HasPrefix(line, blockMarker) {
			continue
		}
		_, raw, found := strings.Cut(line, "paths=")
		if !found {
			continue
		}
		for _, path := range parseShellWords(raw) {
			paths[path] = true
		}
	}
	return paths
}

func parseShellWords(raw string) []string {
	words := make([]string, 0)
	for raw != "" {
		raw = strings.TrimLeft(raw, " \t")
		if raw == "" {
			break
		}
		if raw[0] != '\'' {
			word, rest, _ := strings.Cut(raw, " ")
			words = append(words, word)
			raw = rest
			continue
		}
		raw = raw[1:]
		var word strings.Builder
		for raw != "" {
			at := strings.IndexByte(raw, '\'')
			if at < 0 {
				word.WriteString(raw)
				raw = ""
				break
			}
			word.WriteString(raw[:at])
			raw = raw[at+1:]
			if strings.HasPrefix(raw, `"'"'`) {
				word.WriteByte('\'')
				raw = raw[4:]
				continue
			}
			break
		}
		words = append(words, word.String())
	}
	return words
}
