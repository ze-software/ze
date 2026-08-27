// Design: docs/architecture/core-design.md -- bounded repository populations for document checks
// Overview: links.go -- the gate that consumes these populations.

package doccheck

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const gitTimeout = 10 * time.Minute

func gitOutput(root string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	argv := append([]string{"-C", root}, args...)
	cmd := exec.CommandContext(ctx, "git", argv...) //nolint:gosec // fixed repository query with caller-owned arguments
	out, err := cmd.CombinedOutput()
	if err == nil {
		return out, nil
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return nil, ctx.Err()
	}
	return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
}

func readRepositoryFile(root, relative string) ([]byte, error) {
	repository, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	body, readErr := repository.ReadFile(filepath.FromSlash(relative))
	return body, errors.Join(readErr, repository.Close())
}

func trackedNames(root string) ([]string, error) {
	out, err := gitOutput(root, "ls-files", "-z")
	if err != nil {
		return nil, fmt.Errorf("listing tracked citation population: %w", err)
	}
	names := make([]string, 0, bytes.Count(out, []byte{0}))
	for token := range bytes.SplitSeq(out, []byte{0}) {
		if len(token) > 0 {
			names = append(names, filepath.ToSlash(string(token)))
		}
	}
	sort.Strings(names)
	return names, nil
}

func trackedFiles(root string) ([]string, error) {
	names, err := trackedNames(root)
	if err != nil {
		return nil, err
	}
	files := make([]string, 0, len(names))
	for _, rel := range names {
		_, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
		if statErr == nil {
			files = append(files, rel)
			continue
		}
		if !os.IsNotExist(statErr) {
			return nil, fmt.Errorf("inspecting tracked path %s: %w", rel, statErr)
		}
	}
	return files, nil
}

func checkIgnored(root string, paths []string) (map[string]bool, error) {
	ignored := make(map[string]bool)
	if len(paths) == 0 {
		return ignored, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", root, "check-ignore", "-z", "--stdin") //nolint:gosec // fixed Git query
	var input textbuf.Buffer
	input.Join(paths, "\x00").Byte(0)
	cmd.Stdin = strings.NewReader(input.Slice())
	out, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			return nil, fmt.Errorf("checking ignored citation targets: %w", err)
		}
		if exit.ExitCode() != 1 {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return nil, fmt.Errorf("checking ignored citation targets: %w", ctx.Err())
			}
			return nil, fmt.Errorf("checking ignored citation targets: %w", err)
		}
	}
	for token := range bytes.SplitSeq(out, []byte{0}) {
		if len(token) > 0 {
			ignored[filepath.ToSlash(string(token))] = true
		}
	}
	return ignored, nil
}
