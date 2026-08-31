// Design: docs/features/ai-first.md -- explicit commit path and message contracts
package commit

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

const messageWidth = 72

// tagMaxLength bounds a commit tag, which becomes one path component of the
// message file and of the commit script. The pattern below is BUILT from it so
// the bound is declared once: a second copy inside the regexp literal is a
// disagreement with nothing to arbitrate it, and validateTag reports the two
// halves separately.
const tagMaxLength = 32

var tagPattern = regexp.MustCompile(
	`^[A-Za-z0-9][A-Za-z0-9._-]{0,` + strconv.Itoa(tagMaxLength-1) + `}$`)

var forbiddenGeneratedPaths = map[string]bool{
	"AGENTS.md": true,
	"CLAUDE.md": true,
}

// normalizePath validates one explicit repository path without changing its identity.
func normalizePath(root, raw string) (string, error) {
	if raw == "" {
		return "", errors.New("empty path is not allowed")
	}
	if strings.ContainsAny(raw, "\x00\t\r\n") || strings.Count(raw, "\n") != 0 {
		return "", fmt.Errorf("path contains a line break or control character and cannot be committed: %q", raw)
	}
	path := raw
	if filepath.IsAbs(path) {
		relative, err := filepath.Rel(root, filepath.Clean(path))
		if err != nil {
			return "", fmt.Errorf("path is outside repository: %s", raw)
		}
		path = relative
	}
	path = filepath.Clean(path)
	if path == "." {
		return "", errors.New("repository root is not an explicit file path")
	}
	if path == ".." || strings.HasPrefix(path, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path is outside repository: %s", raw)
	}
	path = filepath.ToSlash(path)
	if path == ".git" || strings.HasPrefix(path, ".git/") {
		return "", fmt.Errorf("git internals cannot be committed: %s", path)
	}
	return path, nil
}

func validateAddPath(root, path string) error {
	if forbiddenGeneratedPaths[path] {
		return fmt.Errorf("generated agent file must not be committed: %s", path)
	}
	ignored, err := gitExit(root, "check-ignore", "--no-index", "-q", "--", path)
	if err != nil {
		return fmt.Errorf("check ignored path %s: %w", path, err)
	}
	if ignored == 0 {
		return fmt.Errorf("ignored path must not be committed: %s", path)
	}
	info, err := os.Stat(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("path does not exist, use remove for tracked deletions: %s", path)
		}
		return fmt.Errorf("inspect path %s: %w", path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("commit scripts require explicit files, not directories: %s", path)
	}
	return nil
}

func validateRemovePath(root, path string) error {
	code, err := gitExit(root, "ls-files", "--error-unmatch", "--", path)
	if err != nil {
		return fmt.Errorf("check tracked removal %s: %w", path, err)
	}
	if code != 0 {
		return fmt.Errorf("remove path is not tracked: %s", path)
	}
	return nil
}

func gitExit(root string, args ...string) (int, error) {
	command := exec.CommandContext(context.Background(), "git", args...) // #nosec G204 -- executable and verbs are closed; paths are argv.
	command.Dir = root
	var complaint bytes.Buffer
	command.Stdout = &complaint
	command.Stderr = &complaint
	err := command.Run()
	if err == nil {
		return 0, nil
	}
	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		return exit.ExitCode(), nil
	}
	return -1, err
}

func unique(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

// Message returns a commit message with a one-line 72-column subject and a
// body wrapped without breaking words.
func Message(subject string, body []string) (string, error) {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return "", errors.New("subject is required")
	}
	if strings.ContainsAny(subject, "\r\n") {
		return "", errors.New("subject must be a single line")
	}
	if utf8.RuneCountInString(subject) > messageWidth {
		length := utf8.RuneCountInString(subject)
		over := length - messageWidth
		return "", fmt.Errorf("subject is %d characters, %d over the %d limit. Cut %d characters from: %s", length, over, messageWidth, over, subject)
	}
	wrapped, err := wrapBody(body)
	if err != nil {
		return "", err
	}
	if wrapped == "" {
		return subject + "\n", nil
	}
	return subject + "\n\n" + wrapped + "\n", nil
}

func wrapBody(chunks []string) (string, error) {
	lines := make([]string, 0)
	for _, chunk := range chunks {
		if chunk == "" {
			lines = append(lines, "")
			continue
		}
		for raw := range strings.SplitSeq(chunk, "\n") {
			line := strings.TrimRight(raw, " \t")
			if strings.TrimSpace(line) == "" {
				lines = append(lines, "")
				continue
			}
			indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			words := strings.Fields(strings.TrimSpace(line))
			for _, word := range words {
				if utf8.RuneCountInString(indent+word) > messageWidth {
					return "", fmt.Errorf("body contains an unwrappable line longer than %d characters", messageWidth)
				}
			}
			current := indent
			for _, word := range words {
				separator := ""
				if current != indent {
					separator = " "
				}
				if utf8.RuneCountInString(current+separator+word) <= messageWidth {
					current += separator + word
					continue
				}
				lines = append(lines, current)
				current = indent + word
			}
			lines = append(lines, current)
		}
	}
	for len(lines) != 0 && lines[0] == "" {
		lines = lines[1:]
	}
	for len(lines) != 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n"), nil
}
