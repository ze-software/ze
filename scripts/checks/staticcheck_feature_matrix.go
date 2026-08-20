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
	"strconv"
	"strings"
	"syscall"
	"time"
)

const featureManifestPath = "feature-gates.txt"

// scopeTagsEnv names the file holding this run's feature-tag answer, written
// once by scripts/status/verify_run.go and read by every stage that run starts.
// It is unset for a standalone `make ze-staticcheck-feature-matrix-check`, and
// an unset variable judges every row.
const scopeTagsEnv = "ZE_VERIFY_SCOPE_TAGS"

// minMatrixRows is the floor no scoped matrix goes under. all_features and
// core_only judge the two combinations Ze ships, so every run judges them
// however narrow its change set is.
const minMatrixRows = 2

const defaultStaticcheckDeadline = 25 * time.Minute

var (
	featureTagPattern  = regexp.MustCompile(`^ze_[A-Za-z0-9_]+$`)
	matrixNamePattern  = regexp.MustCompile(`^[A-Za-z0-9_]+$`)
	packagePathPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._~/-]*$`)
)

type featureMatrixRow struct {
	name string
	tags []string
	// omits names the one feature tag this row removes from all_features. It is
	// empty for all_features and core_only, the two rows every run judges, and
	// that emptiness is what scopeFeatureMatrix reads as "never subtract me".
	omits string
}

// changeScope is the part of the matrix a change set can move: the feature tags
// the change reaches, or every feature when the answer cannot be narrowed.
type changeScope struct {
	tags  map[string]bool
	every bool
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

	tags, err := readFeatureTags(featureManifestPath)
	if err != nil {
		return reportMatrixFailure(stderr, err)
	}
	scope, widen := readChangeScope(os.Getenv(scopeTagsEnv), tags)
	if widen != nil {
		// Written piece by piece rather than with a format verb or a `+`: this
		// file is compiled STANDALONE by TestStaticcheckFeatureMatrixRejectsVacuousInput,
		// so it can import nothing from the module, textbuf included.
		io.WriteString(stderr, "staticcheck feature matrix: ")
		io.WriteString(stderr, widen.Error())
		io.WriteString(stderr, ", so every row is judged\n")
	}
	matrix, err := deriveFeatureMatrix(tags, scope)
	if err != nil {
		return reportMatrixFailure(stderr, err)
	}
	if !scope.every {
		io.WriteString(stderr, "staticcheck feature matrix: the change set reaches ")
		io.WriteString(stderr, strconv.Itoa(len(scope.tags)))
		io.WriteString(stderr, " feature tag(s), so ")
		io.WriteString(stderr, strconv.Itoa(bytes.Count(matrix, []byte{'\n'})))
		io.WriteString(stderr, " of ")
		io.WriteString(stderr, strconv.Itoa(len(tags)+minMatrixRows))
		io.WriteString(stderr, " rows are judged\n")
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

// reportMatrixFailure states a matrix that could not be built, in the one form
// every caller of this check reads.
func reportMatrixFailure(stderr io.Writer, err error) int {
	io.WriteString(stderr, "staticcheck feature matrix: ")
	io.WriteString(stderr, err.Error())
	io.WriteString(stderr, "\n")
	return 2
}

// deriveFeatureMatrix renders the rows this run judges: every row the manifest
// implies, less the rows the change set cannot move.
func deriveFeatureMatrix(tags []string, scope changeScope) ([]byte, error) {
	rows, err := buildFeatureMatrix(tags)
	if err != nil {
		return nil, err
	}
	scoped, err := scopeFeatureMatrix(rows, scope)
	if err != nil {
		return nil, err
	}
	return renderFeatureMatrix(scoped)
}

// readChangeScope reads the run's feature-tag answer and returns the scope the
// matrix judges. The second result is the reason the scope was widened, and it
// is a reason rather than a failure: the caller states it and judges every row,
// exactly as emit does in scripts/checks/verify_scope_selector.go.
//
// Every doubt widens: no answer named, an answer that cannot be read, and an
// answer naming a tag the manifest in hand does not declare all judge the whole
// matrix. A narrower scope is taken only from an answer that parses against
// that manifest, because a guard that cannot read its input must not return a
// valid-looking narrow answer (ai/rules/evidence.md).
func readChangeScope(answerPath string, manifestTags []string) (changeScope, error) {
	if strings.TrimSpace(answerPath) == "" {
		return changeScope{every: true}, nil
	}
	raw, err := os.ReadFile(answerPath) //nolint:gosec // the path comes from the verify runner that started this stage
	if err != nil {
		return changeScope{every: true}, fmt.Errorf("feature-tag answer %s could not be read (%w)", answerPath, err)
	}
	declared := make(map[string]bool, len(manifestTags))
	for _, tag := range manifestTags {
		declared[tag] = true
	}
	reached := make(map[string]bool, len(manifestTags))
	for _, rawLine := range strings.Split(string(raw), "\n") {
		tag := strings.TrimSpace(rawLine)
		if tag == "" {
			continue
		}
		if !declared[tag] {
			return changeScope{every: true}, fmt.Errorf(
				"feature-tag answer %s names %q, which %s does not declare",
				answerPath,
				tag,
				featureManifestPath,
			)
		}
		reached[tag] = true
	}
	if len(reached) == len(declared) {
		return changeScope{every: true}, nil
	}
	return changeScope{tags: reached}, nil
}

// scopeFeatureMatrix subtracts the rows the change set cannot move.
//
// It SUBTRACTS from the derived rows and never names the rows to keep: a tag
// added to feature-gates.txt gains its row in matrixRowsForTags and is judged
// here for free, where a list of row names would silently stop covering it.
//
// A row that omits tag T differs from all_features only in the packages T
// gates. A change confined to packages gated by the tags in scope therefore
// moves such a row only when T is one of them: nothing always-on imports a
// gated package, and dep_audit.py refuses a gated package importing one gated
// by a different tag, so removing any other tag leaves both the changed package
// and its whole import closure standing (reachedTags in
// scripts/checks/verify_scope_selector.go relies on the same two facts).
//
// That import argument is about what a row DROPS, and it is only half of what
// makes the subtraction sound. A file constrained !ze_T is compiled by the rows
// that dropped T and by no other, so a change to `ze_web && !ze_ssh` source is
// visible in without_ze_ssh alone. The producer of the answer this function
// reads is what closes that half: reachedTags unions the tags a changed file
// NEGATES into the scope, so the row survives the subtraction here.
func scopeFeatureMatrix(rows []featureMatrixRow, scope changeScope) ([]featureMatrixRow, error) {
	if scope.every {
		return rows, nil
	}
	scoped := make([]featureMatrixRow, 0, len(scope.tags)+minMatrixRows)
	for _, row := range rows {
		if row.omits == "" || scope.tags[row.omits] {
			scoped = append(scoped, row)
		}
	}
	if err := validateScopedMatrix(scoped); err != nil {
		return nil, err
	}
	return scoped, nil
}

// validateScopedMatrix refuses a scoped matrix that lost its floor. The two
// rows omitting no tag are all_features and core_only, and they judge what Ze
// ships: a filter that drops either one leaves a shipped combination unjudged.
func validateScopedMatrix(rows []featureMatrixRow) error {
	if len(rows) < minMatrixRows {
		return fmt.Errorf(
			"scoped matrix has %d rows, want at least %d: all_features and core_only judge the shipped combinations",
			len(rows),
			minMatrixRows,
		)
	}
	floor := 0
	for _, row := range rows {
		if row.omits == "" {
			floor++
		}
	}
	if floor != minMatrixRows {
		return fmt.Errorf(
			"scoped matrix keeps %d of the %d rows that omit no feature tag: all_features and core_only are never subtracted",
			floor,
			minMatrixRows,
		)
	}
	return nil
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
	var name strings.Builder
	for omitted, tag := range tags {
		rowTags := make([]string, 0, len(tags)+1)
		rowTags = append(rowTags, "ze_core", "ze_distro")
		rowTags = append(rowTags, tags[:omitted]...)
		rowTags = append(rowTags, tags[omitted+1:]...)
		// Built rather than concatenated with `+`, which allocates a backing
		// array and copies both sides. textbuf is not reachable here: this file
		// is compiled standalone by its own vacuity test.
		name.Reset()
		name.WriteString("without_")
		name.WriteString(tag)
		rows = append(rows, featureMatrixRow{name: name.String(), tags: rowTags, omits: tag})
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
		if rows[index].omits != want[index].omits {
			return fmt.Errorf(
				"matrix row %q omits %q, want %q",
				rows[index].name,
				rows[index].omits,
				want[index].omits,
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
