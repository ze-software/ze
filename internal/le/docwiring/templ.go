// Design: docs/architecture/core-design.md -- generated templ output freshness
// Overview: delegate.go -- the selected target callback table
//
// templ.go keeps the repository-specific orphan and scope checks in Go, then
// calls templ's Go command package directly for the compiler-owned comparison.

package docwiring

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

	"github.com/a-h/templ/cmd/templ/generatecmd"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	templScope   = "internal/"
	templVendor  = "vendor/"
	templTimeout = 30 * time.Minute
)

type templOutputPage struct {
	lines []string
}

func (p templOutputPage) Text() string {
	if len(p.lines) == 0 {
		return ""
	}
	var tb textbuf.Buffer
	return tb.Join(p.lines, "\n").Byte('\n').String()
}

func answerTemplOutput(root string) (any, int) {
	paths, err := templRepositoryPaths(root)
	if err != nil {
		return errorPage(err), 2
	}

	stray := make([]string, 0)
	orphans := make([]string, 0)
	var tb textbuf.Buffer
	for _, rel := range paths {
		switch {
		case strings.HasSuffix(rel, ".templ") && !strings.HasPrefix(rel, templScope) && !strings.HasPrefix(rel, templVendor):
			stray = append(stray, rel)
		case strings.HasPrefix(rel, templScope) && strings.HasSuffix(rel, "_templ.go"):
			source := tb.Reset().Str(strings.TrimSuffix(rel, "_templ.go")).Str(".templ").String()
			exists, err := regularTemplPath(filepath.Join(root, filepath.FromSlash(source)))
			if err != nil {
				return errorPage(fmt.Errorf("inspecting templ source %s: %w", source, err)), 2
			}
			if !exists {
				orphans = append(orphans, rel)
			}
		}
	}
	sort.Strings(stray)
	sort.Strings(orphans)
	if len(stray) > 0 {
		lines := []string{"error: templ generate reads internal/ only, and these .templ files sit outside it:"}
		for _, rel := range stray {
			lines = append(lines, tb.Reset().Str("  ").Str(rel).String())
		}
		return templOutputPage{lines: lines}, 1
	}
	if len(orphans) > 0 {
		lines := []string{"error: generated file with no .templ source:"}
		for _, rel := range orphans {
			lines = append(lines, tb.Reset().Str("  ").Str(rel).String())
		}
		lines = append(lines,
			"delete it, or restore the .templ it came from",
			"`make generate` deletes it for you, because no source produces it")
		return templOutputPage{lines: lines}, 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), templTimeout)
	defer cancel()
	var stdout, stderr bytes.Buffer
	err = generatecmd.Run(ctx, &stdout, &stderr, []string{
		"-check", "-keep-orphaned-files", "-path", filepath.Join(root, "internal"),
	})
	lines := outputLines(stdout.String(), stderr.String())
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			lines = append(lines, tb.Reset().Str("error: templ output check: ").
				Err(ctx.Err()).String())
		} else {
			lines = append(lines, tb.Reset().Str("error: templ output check: ").Err(err).String())
		}
		return templOutputPage{lines: lines}, 1
	}
	return templOutputPage{lines: lines}, 0
}

func templRepositoryPaths(root string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	// #nosec G204 -- Git and its repository-inventory query are fixed; root only selects the checkout.
	cmd := exec.CommandContext(ctx, "git", "-C", root, "ls-files", "-z",
		"--cached", "--others", "--exclude-standard")
	out, err := cmd.CombinedOutput()
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf("listing templ population: %w", ctx.Err())
		}
		return nil, fmt.Errorf("listing templ population: %w: %s", err, strings.TrimSpace(string(out)))
	}

	paths := make([]string, 0, bytes.Count(out, []byte{0}))
	for token := range bytes.SplitSeq(out, []byte{0}) {
		if len(token) == 0 {
			continue
		}
		rel := filepath.ToSlash(string(token))
		_, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
		if statErr == nil {
			paths = append(paths, rel)
			continue
		}
		if !os.IsNotExist(statErr) {
			return nil, fmt.Errorf("inspecting templ population path %s: %w", rel, statErr)
		}
	}
	return paths, nil
}

func regularTemplPath(path string) (bool, error) {
	info, err := os.Stat(path)
	if err == nil {
		return info.Mode().IsRegular(), nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func outputLines(outputs ...string) []string {
	var lines []string
	for _, output := range outputs {
		for line := range strings.SplitSeq(strings.TrimSuffix(output, "\n"), "\n") {
			if line != "" {
				lines = append(lines, line)
			}
		}
	}
	return lines
}
