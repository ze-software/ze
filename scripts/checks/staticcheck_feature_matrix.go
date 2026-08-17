// Design: docs/architecture/testing/tracked-build-gate.md -- feature-tag type-check boundary
//
// Usage: CGO_ENABLED=0 go run scripts/checks/staticcheck_feature_matrix.go [--print-matrix] [--deadline=D]
// Called by: make ze-staticcheck-feature-matrix-check, and both verification
// modes through stagesForMode in scripts/status/verify_run.go.
//
//go:build ignore

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	pathpkg "path"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"
)

const featureManifestPath = "feature-gates.txt"

const defaultStaticcheckDeadline = 25 * time.Minute

var (
	featureTagPattern  = regexp.MustCompile(`^ze_[A-Za-z0-9_]+$`)
	matrixNamePattern  = regexp.MustCompile(`^[A-Za-z0-9_]+$`)
	packagePathPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._~/-]*$`)
)

type featureMatrixRow struct {
	name string
	tags []string
}

func main() {
	os.Exit(runStaticcheckFeatureMatrix(os.Args[1:], os.Stdout, os.Stderr))
}

func runStaticcheckFeatureMatrix(args []string, stdout, stderr io.Writer) int {
	printMatrix := false
	deadline := defaultStaticcheckDeadline
	deadlineSet := false
	for _, arg := range args {
		switch {
		case arg == "--print-matrix" && !printMatrix:
			printMatrix = true
		case strings.HasPrefix(arg, "--deadline=") && !deadlineSet:
			parsed, err := time.ParseDuration(strings.TrimPrefix(arg, "--deadline="))
			if err != nil || parsed <= 0 {
				fmt.Fprintf(
					stderr,
					"staticcheck feature matrix: --deadline needs a positive duration, got %q; matrix could not be judged\n",
					arg,
				)
				return 2
			}
			deadline = parsed
			deadlineSet = true
		default:
			fmt.Fprintf(
				stderr,
				"staticcheck feature matrix: malformed invocation %q; usage: staticcheck_feature_matrix [--print-matrix] [--deadline=D]; matrix could not be judged\n",
				arg,
			)
			return 2
		}
	}

	matrix, err := deriveFeatureMatrix(featureManifestPath)
	if err != nil {
		io.WriteString(stderr, "staticcheck feature matrix: ")
		io.WriteString(stderr, err.Error())
		io.WriteString(stderr, "\n")
		return 2
	}
	if printMatrix {
		if _, err := stdout.Write(matrix); err != nil {
			io.WriteString(stderr, "staticcheck feature matrix: write matrix: ")
			io.WriteString(stderr, err.Error())
			io.WriteString(stderr, "\n")
			return 2
		}
		return 0
	}

	return judgeStaticcheckFeatureMatrix(matrix, deadline, stdout, stderr)
}

func judgeStaticcheckFeatureMatrix(matrix []byte, deadline time.Duration, stdout, stderr io.Writer) int {
	ctx, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()

	cmd := exec.CommandContext(ctx, "staticcheck", "-checks=-all", "-matrix", "./...")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.Stdin = bytes.NewReader(matrix)
	var toolStdout, toolStderr bytes.Buffer
	cmd.Stdout = &toolStdout
	cmd.Stderr = &toolStderr
	if err := cmd.Start(); err != nil {
		switch {
		case errors.Is(ctx.Err(), context.DeadlineExceeded):
			fmt.Fprintf(
				stderr,
				"staticcheck feature matrix: Staticcheck could not start before the %s deadline; matrix could not be judged\n",
				deadline,
			)
		case errors.Is(err, exec.ErrNotFound):
			io.WriteString(
				stderr,
				"staticcheck feature matrix: staticcheck was not found on PATH; install standalone Staticcheck, ensure it is on PATH, and retry; matrix could not be judged\n",
			)
		default:
			fmt.Fprintf(
				stderr,
				"staticcheck feature matrix: could not start staticcheck: %v; verify the executable and PATH, then retry; matrix could not be judged\n",
				err,
			)
		}
		return 2
	}

	runErr := cmd.Wait()
	if _, err := stdout.Write(toolStdout.Bytes()); err != nil {
		fmt.Fprintf(stderr, "staticcheck feature matrix: preserve Staticcheck stdout: %v; matrix could not be judged\n", err)
		return 2
	}
	if _, err := stderr.Write(toolStderr.Bytes()); err != nil {
		fmt.Fprintf(stderr, "staticcheck feature matrix: preserve Staticcheck stderr: %v; matrix could not be judged\n", err)
		return 2
	}
	if bytes.Contains(toolStdout.Bytes(), []byte("matched no packages")) ||
		bytes.Contains(toolStderr.Bytes(), []byte("matched no packages")) {
		io.WriteString(
			stderr,
			"staticcheck feature matrix: Staticcheck matched no packages; matrix could not be judged; run from a Go module containing the selected packages\n",
		)
		return 2
	}
	if runErr == nil {
		fmt.Fprintf(stdout, "staticcheck feature matrix: checked %d rows\n", bytes.Count(matrix, []byte{'\n'}))
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) && exitErr.ExitCode() == 1 {
		return 1
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		fmt.Fprintf(
			stderr,
			"staticcheck feature matrix: Staticcheck exceeded the %s deadline; matrix could not be judged, so retry after resolving the timeout\n",
			deadline,
		)
		return 2
	}
	if errors.As(runErr, &exitErr) {
		fmt.Fprintf(
			stderr,
			"staticcheck feature matrix: Staticcheck exited %d without a type-check verdict; verify that standalone Staticcheck supports -checks=-all and -matrix; matrix could not be judged\n",
			exitErr.ExitCode(),
		)
		return 2
	}
	fmt.Fprintf(
		stderr,
		"staticcheck feature matrix: Staticcheck did not complete: %v; matrix could not be judged\n",
		runErr,
	)
	return 2
}

func deriveFeatureMatrix(manifestPath string) ([]byte, error) {
	tags, err := readFeatureTags(manifestPath)
	if err != nil {
		return nil, err
	}
	rows, err := buildFeatureMatrix(tags)
	if err != nil {
		return nil, err
	}
	return renderFeatureMatrix(rows)
}

func readFeatureTags(manifestPath string) ([]string, error) {
	raw, err := os.ReadFile(manifestPath) //nolint:gosec // caller supplies the explicit manifest path
	if err != nil {
		return nil, fmt.Errorf("read feature manifest %s: %w", manifestPath, err)
	}

	seen := make(map[string]bool)
	var tags []string
	for index, rawLine := range strings.Split(string(raw), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, fmt.Errorf(
				"%s:%d: expected <tag> <package>, got %d fields",
				manifestPath,
				index+1,
				len(fields),
			)
		}
		tag, packagePath := fields[0], fields[1]
		if !featureTagPattern.MatchString(tag) {
			return nil, fmt.Errorf(
				"%s:%d: invalid feature tag %q; want ze_ followed by letters, numbers, or underscores",
				manifestPath,
				index+1,
				tag,
			)
		}
		if tag == "ze_core" || tag == "ze_distro" {
			return nil, fmt.Errorf(
				"%s:%d: reserved feature tag %q is supplied by every matrix row and must not be in the manifest",
				manifestPath,
				index+1,
				tag,
			)
		}
		if !validPackagePath(packagePath) {
			return nil, fmt.Errorf(
				"%s:%d: invalid package path %q; want a clean relative Go import path",
				manifestPath,
				index+1,
				packagePath,
			)
		}
		if !seen[tag] {
			seen[tag] = true
			tags = append(tags, tag)
		}
	}
	if len(tags) == 0 {
		return nil, fmt.Errorf("%s: no feature tags found", manifestPath)
	}
	sort.Strings(tags)
	return tags, nil
}

func validPackagePath(packagePath string) bool {
	return packagePathPattern.MatchString(packagePath) &&
		pathpkg.Clean(packagePath) == packagePath &&
		!strings.Contains(packagePath, "//")
}

func buildFeatureMatrix(tags []string) ([]featureMatrixRow, error) {
	sorted, err := validateAndSortFeatureTags(tags)
	if err != nil {
		return nil, err
	}
	rows := matrixRowsForTags(sorted)
	if err := validateFeatureMatrix(sorted, rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func validateAndSortFeatureTags(tags []string) ([]string, error) {
	if len(tags) == 0 {
		return nil, fmt.Errorf("matrix requires at least one feature tag")
	}
	sorted := append([]string(nil), tags...)
	sort.Strings(sorted)
	for index, tag := range sorted {
		if !featureTagPattern.MatchString(tag) {
			return nil, fmt.Errorf("matrix feature tag %q is invalid", tag)
		}
		if tag == "ze_core" || tag == "ze_distro" {
			return nil, fmt.Errorf("matrix feature tag %q is reserved", tag)
		}
		if index > 0 && tag == sorted[index-1] {
			return nil, fmt.Errorf("matrix feature tag %q is duplicated", tag)
		}
	}
	return sorted, nil
}

func matrixRowsForTags(tags []string) []featureMatrixRow {
	allTags := make([]string, 0, len(tags)+2)
	allTags = append(allTags, "ze_core", "ze_distro")
	allTags = append(allTags, tags...)
	rows := make([]featureMatrixRow, 0, len(tags)+2)
	rows = append(rows,
		featureMatrixRow{name: "all_features", tags: allTags},
		featureMatrixRow{name: "core_only", tags: []string{"ze_core"}},
	)
	for omitted, tag := range tags {
		rowTags := make([]string, 0, len(tags)+1)
		rowTags = append(rowTags, "ze_core", "ze_distro")
		rowTags = append(rowTags, tags[:omitted]...)
		rowTags = append(rowTags, tags[omitted+1:]...)
		rows = append(rows, featureMatrixRow{name: "without_" + tag, tags: rowTags})
	}
	return rows
}

func validateFeatureMatrix(tags []string, rows []featureMatrixRow) error {
	if len(tags) == 0 {
		return fmt.Errorf("matrix requires at least one feature tag")
	}
	wantCount := len(tags) + 2
	if len(rows) != wantCount {
		return fmt.Errorf("matrix row count is %d, want %d for %d unique feature tags", len(rows), wantCount, len(tags))
	}
	if err := validateMatrixRows(rows); err != nil {
		return err
	}

	want := matrixRowsForTags(tags)
	for index := range want {
		if rows[index].name != want[index].name {
			return fmt.Errorf(
				"matrix row %d has name %q, want %q",
				index+1,
				rows[index].name,
				want[index].name,
			)
		}
		if !equalStrings(rows[index].tags, want[index].tags) {
			return fmt.Errorf(
				"matrix row %q has tags %q, want %q",
				rows[index].name,
				strings.Join(rows[index].tags, ","),
				strings.Join(want[index].tags, ","),
			)
		}
	}
	return nil
}

func validateMatrixRows(rows []featureMatrixRow) error {
	if len(rows) == 0 {
		return fmt.Errorf("matrix has zero rows")
	}
	names := make(map[string]bool, len(rows))
	for _, row := range rows {
		if !matrixNamePattern.MatchString(row.name) {
			return fmt.Errorf("matrix has invalid row name %q; want letters, numbers, or underscores", row.name)
		}
		if names[row.name] {
			return fmt.Errorf("matrix has duplicate row name %q", row.name)
		}
		names[row.name] = true
		if len(row.tags) == 0 {
			return fmt.Errorf("matrix row %q has no build tags", row.name)
		}
		rowTags := make(map[string]bool, len(row.tags))
		for _, tag := range row.tags {
			if !featureTagPattern.MatchString(tag) {
				return fmt.Errorf("matrix row %q has invalid build tag %q", row.name, tag)
			}
			if rowTags[tag] {
				return fmt.Errorf("matrix row %q has duplicate build tag %q", row.name, tag)
			}
			rowTags[tag] = true
		}
	}
	return nil
}

func renderFeatureMatrix(rows []featureMatrixRow) ([]byte, error) {
	if err := validateMatrixRows(rows); err != nil {
		return nil, err
	}
	var rendered strings.Builder
	for _, row := range rows {
		rendered.WriteString(row.name)
		rendered.WriteString(": -tags=")
		rendered.WriteString(strings.Join(row.tags, ","))
		rendered.WriteByte('\n')
	}
	out := []byte(rendered.String())
	if err := validateRenderedMatrix(rows, out); err != nil {
		return nil, err
	}
	return out, nil
}

func validateRenderedMatrix(rows []featureMatrixRow, rendered []byte) error {
	if len(rendered) == 0 {
		return fmt.Errorf("rendered matrix is empty")
	}
	if rendered[len(rendered)-1] != '\n' {
		return fmt.Errorf("rendered matrix is missing its final newline")
	}
	if lines := bytes.Count(rendered, []byte{'\n'}); lines != len(rows) {
		return fmt.Errorf("rendered matrix has %d lines, want %d rows", lines, len(rows))
	}
	return nil
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
