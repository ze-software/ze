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
	"github.com/ze-software/ze/internal/le/lepath"
)

const (
	templOrphanVerb = "templ-orphans"
	templScope      = "internal/"
	templVendor     = "vendor/"
	templTimeout    = 30 * time.Minute
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

// templOrphanReport is the exact orphan finding set.
// The two lists stay separate because a stray source and an orphan output have
// different repairs.
type templOrphanReport struct {
	StraySources  []string `json:"stray-sources"`
	OrphanOutputs []string `json:"orphan-outputs"`
}

type templPopulationListError struct {
	cause  error
	detail string
}

func (e *templPopulationListError) Error() string {
	if e.detail == "" {
		return fmt.Sprintf("listing templ population: %v", e.cause)
	}
	return fmt.Sprintf("listing templ population: %v: %s", e.cause, e.detail)
}

func (e *templPopulationListError) Unwrap() error {
	return e.cause
}

func (r templOrphanReport) Text() string {
	var lines []string
	var tb textbuf.Buffer
	switch {
	case len(r.StraySources) > 0:
		lines = append(lines, "error: templ generate reads internal/ only, and these .templ files sit outside it:")
		for _, rel := range r.StraySources {
			lines = append(lines, tb.Reset().Str("  ").Str(rel).String())
		}
	case len(r.OrphanOutputs) > 0:
		lines = append(lines, "error: generated file with no .templ source:")
		for _, rel := range r.OrphanOutputs {
			lines = append(lines, tb.Reset().Str("  ").Str(rel).String())
		}
		lines = append(lines, "delete it, or restore the .templ source")
	default:
		return ""
	}
	return tb.Reset().Join(lines, "\n").Byte('\n').String()
}

func (r templOrphanReport) failed() bool {
	if len(r.StraySources) > 0 {
		return true
	}
	return len(r.OrphanOutputs) > 0
}

func answerTemplOrphansHere() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		reportError(err)
		return nil, 2
	}
	return answerTemplOrphans(root)
}

func answerTemplOrphans(root string) (any, int) {
	paths, err := templRepositoryPaths(root)
	if err != nil {
		var listErr *templPopulationListError
		if errors.As(err, &listErr) {
			detail := listErr.detail
			if detail == "" {
				detail = listErr.cause.Error()
			}
			reportError(fmt.Errorf("git ls-files failed in %s: %s", root, detail))
			return nil, 2
		}
		reportError(err)
		return nil, 2
	}
	report, err := templOrphans(root, paths, templSourceExists)
	if err != nil {
		reportError(err)
		return nil, 2
	}
	if report.failed() {
		return report, 1
	}
	return report, 0
}

func templOrphans(
	root string,
	paths []string,
	sourceExists func(string) (bool, error),
) (templOrphanReport, error) {
	report := templOrphanReport{
		StraySources:  make([]string, 0),
		OrphanOutputs: make([]string, 0),
	}
	for _, rel := range paths {
		if !strings.HasSuffix(rel, ".templ") {
			continue
		}
		if strings.HasPrefix(rel, templScope) {
			continue
		}
		if strings.HasPrefix(rel, templVendor) {
			continue
		}
		report.StraySources = append(report.StraySources, rel)
	}
	sort.Strings(report.StraySources)
	if len(report.StraySources) > 0 {
		return report, nil
	}

	var tb textbuf.Buffer
	for _, rel := range paths {
		if !strings.HasPrefix(rel, templScope) {
			continue
		}
		if !strings.HasSuffix(rel, "_templ.go") {
			continue
		}
		source := tb.Reset().Str(strings.TrimSuffix(rel, "_templ.go")).Str(".templ").String()
		exists, err := sourceExists(filepath.Join(root, filepath.FromSlash(source)))
		if err != nil {
			return report, fmt.Errorf("inspecting templ source %s: %w", source, err)
		}
		if !exists {
			report.OrphanOutputs = append(report.OrphanOutputs, rel)
		}
	}
	sort.Strings(report.OrphanOutputs)
	return report, nil
}

func templSourceExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
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

func answerTemplOutput(root string) (any, int) {
	paths, err := templRepositoryPaths(root)
	if err != nil {
		return errorPage(err), 2
	}
	orphans, err := templOrphans(root, paths, regularTemplPath)
	if err != nil {
		return errorPage(err), 2
	}
	if orphans.failed() {
		return orphans, 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), templTimeout)
	defer cancel()
	var stdout, stderr bytes.Buffer
	err = generatecmd.Run(ctx, &stdout, &stderr, []string{
		"-check", "-keep-orphaned-files", "-path", filepath.Join(root, "internal"),
	})
	lines := outputLines(stdout.String(), stderr.String())
	var tb textbuf.Buffer
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			lines = append(lines, tb.Str("error: templ output check: ").
				Err(ctx.Err()).String())
		} else {
			lines = append(lines, tb.Str("error: templ output check: ").Err(err).String())
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
			return nil, &templPopulationListError{cause: ctx.Err()}
		}
		return nil, &templPopulationListError{
			cause:  err,
			detail: strings.TrimSpace(string(out)),
		}
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
