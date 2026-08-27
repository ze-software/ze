// Design: docs/architecture/core-design.md -- durable design references in Go headers
// Overview: checks.go -- the unconditional checks this gate runs
//
// designrefs.go ports the `--design-only` half of check_doc_links.py. Git owns
// the tracked-file population; all reference parsing and judging stays in Go.

package docwiring

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	designHeadBytes  = 4096
	designRefTimeout = 10 * time.Minute
)

var designLineRe = regexp.MustCompile(`^// Design:\s*(.+)$`)

type brokenDesignRef struct {
	where  string
	target string
}

// designReferenceFindings answers the broken durable-document references in
// producer order. An unreadable tracked file or an unusable Git population is
// an error because either would make the scan incomplete.
func designReferenceFindings(root string) ([]string, error) {
	files, err := trackedGoFiles(root)
	if err != nil {
		return nil, err
	}

	broken := make([]brokenDesignRef, 0)
	var tb textbuf.Buffer
	for _, rel := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		head, err := readDesignHead(path)
		if err != nil {
			if os.IsNotExist(err) {
				_, stateErr := os.Lstat(path) //nolint:gosec // tracked path under the named checkout
				if os.IsNotExist(stateErr) {
					continue
				}
				if stateErr != nil {
					return nil, fmt.Errorf("inspecting %s after a read failure: %w", rel, stateErr)
				}
			}
			return nil, fmt.Errorf("reading %s: %w", rel, err)
		}
		first, _, _ := strings.Cut(head, "\n")
		if strings.Contains(first, "Code generated") {
			continue
		}
		for lineIndex, line := range strings.Split(head, "\n") {
			match := designLineRe.FindStringSubmatch(line)
			if match == nil {
				continue
			}
			target := strings.TrimSpace(match[1])
			if strings.HasPrefix(target, "(") {
				break
			}
			fields := strings.Fields(target)
			if len(fields) == 0 {
				return nil, fmt.Errorf("%s:%d: empty Design reference", rel, lineIndex+1)
			}
			target = strings.TrimRight(fields[0], ".,;:")
			resolved := target
			if !strings.Contains(target, "/") {
				exists, err := designPathResolves(root, target)
				if err != nil {
					return nil, fmt.Errorf("resolving %s from %s: %w", target, rel, err)
				}
				if !exists {
					resolved = filepath.ToSlash(filepath.Join(filepath.Dir(rel), target))
				}
			}

			where := tb.Reset().Str(rel).Byte(':').Int(int64(lineIndex + 1)).String()
			if strings.HasSuffix(rel, "_test.go") && strings.HasPrefix(resolved, "plan/spec-") {
				broken = append(broken, brokenDesignRef{
					where: tb.Reset().Str(where).
						Str(": a test must cite a durable document, never a spec deleted at closure ").
						Str("(ai/rules/planning.md)").String(),
					target: resolved,
				})
				break
			}
			exists, err := designPathResolves(root, resolved)
			if err != nil {
				return nil, fmt.Errorf("resolving %s from %s: %w", resolved, rel, err)
			}
			if !exists {
				broken = append(broken, brokenDesignRef{
					where:  tb.Reset().Str(where).Str(": broken Design reference").String(),
					target: resolved,
				})
			}
			break
		}
	}

	ignored, err := ignoredDesignTargets(root, broken)
	if err != nil {
		return nil, err
	}
	findings := make([]string, 0, len(broken))
	for _, one := range broken {
		if ignored[strings.TrimSuffix(one.target, "/")] || ignored[one.target] {
			continue
		}
		findings = append(findings, tb.Reset().Str(one.where).Str(": ").Str(one.target).String())
	}
	return findings, nil
}

// trackedGoFiles asks Git for the same source population as the producer. Git
// is the operation's subject here, not a repository-owned implementation.
func trackedGoFiles(root string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), designRefTimeout)
	defer cancel()

	// #nosec G204 -- Git and its tracked-Go-files query are fixed; root only selects the checkout.
	cmd := exec.CommandContext(ctx, "git", "-C", root, "ls-files", "-z", "--",
		"internal/", "cmd/", "pkg/", "scripts/", "*.go")
	out, err := cmd.CombinedOutput()
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf("listing tracked Go files: %w", ctx.Err())
		}
		return nil, fmt.Errorf("listing tracked Go files: %w: %s", err, strings.TrimSpace(string(out)))
	}

	files := make([]string, 0, bytes.Count(out, []byte{0}))
	for token := range bytes.SplitSeq(out, []byte{0}) {
		if len(token) == 0 || !bytes.HasSuffix(token, []byte(".go")) {
			continue
		}
		files = append(files, filepath.ToSlash(string(token)))
	}
	sort.Strings(files)
	return files, nil
}

func readDesignHead(path string) (string, error) {
	file, err := os.Open(path) //nolint:gosec // tracked path under the named checkout
	if err != nil {
		return "", err
	}
	body, readErr := io.ReadAll(io.LimitReader(file, designHeadBytes))
	closeErr := file.Close()
	if readErr != nil {
		return "", readErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	return string(body), nil
}

func designPathResolves(root, rel string) (bool, error) {
	path := filepath.Join(root, filepath.FromSlash(strings.TrimSuffix(rel, "/")))
	if strings.ContainsAny(rel, "*?[") {
		matches, err := filepath.Glob(path)
		if err != nil {
			return false, err
		}
		return len(matches) > 0, nil
	}
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func ignoredDesignTargets(root string, broken []brokenDesignRef) (map[string]bool, error) {
	ignored := make(map[string]bool)
	if len(broken) == 0 {
		return ignored, nil
	}

	var input textbuf.Buffer
	for _, one := range broken {
		input.Str(one.target).Byte('\n')
	}
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", root, "check-ignore", "--stdin") //nolint:gosec // fixed Git query over judged paths
	cmd.Stdin = strings.NewReader(input.Slice())
	out, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 1 {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return nil, fmt.Errorf("checking ignored design targets: %w", ctx.Err())
			}
			return nil, fmt.Errorf("checking ignored design targets: %w", err)
		}
	}
	for line := range strings.SplitSeq(string(out), "\n") {
		if line != "" {
			ignored[line] = true
		}
	}
	return ignored, nil
}
