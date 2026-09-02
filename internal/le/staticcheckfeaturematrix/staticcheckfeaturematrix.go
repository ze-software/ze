// Design: docs/architecture/testing/tracked-build-gate.md -- feature-tag type-check boundary
//
// Package staticcheckfeaturematrix type-checks the working tree once per feature-tag
// combination Ze can be built in. A package compiled out by one tag set and in
// by another is judged in the rows that carry it, so a break confined to
// `ze_web && !ze_ssh` source is caught by the row that drops ze_ssh and by no
// other.
//
// The matrix is DERIVED from feature-gates.txt, never listed: all_features,
// core_only, and one without_<tag> row per declared feature. A tag added to the
// manifest gains its row for free, where a list of row names would silently
// stop covering it.
//
// A run can be SCOPED. The verify runner writes the feature tags this run's
// change set reaches, and a row that differs from all_features only in the
// packages of a tag nothing touched is subtracted. Every doubt widens: no
// answer, an answer that cannot be read, and an answer naming a tag the
// manifest does not declare each judge the whole matrix, because a guard that
// cannot read its input must not return a valid-looking narrow answer.
//
// A run can also be CUT. `check part <index> of <count>` judges the rows dealt
// to one piece, and the pieces together judge every row the scope left. CI runs
// one piece per shard, because one Staticcheck run type-checks the whole module
// once per row and the 38 rows took 23m36s in one job. The cut is a DEAL over
// the derived rows (Part), never an assignment written by hand, so a tag added
// to the manifest is judged by one piece for free.

package staticcheckfeaturematrix

import (
	"fmt"
	"os"
	pathpkg "path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// featureManifest is the single source of truth for the compile-out-able
// features, relative to the tree.
const featureManifest = "feature-gates.txt"

// ScopeTagsKey is the dot-notation spelling of ZE_VERIFY_SCOPE_TAGS: the file
// holding this run's feature-tag answer, written once by the verify runner and
// read by every stage that run starts. Unset means every row is judged, which
// is what a standalone gate invocation gets.
const ScopeTagsKey = "ze.verify.scope.tags"

var scopeTagsEntry = env.MustRegister(env.EnvEntry{
	Key:         ScopeTagsKey,
	Type:        "string",
	Default:     "",
	Description: "the file naming the feature tags this verify run's change set reaches; unset judges every matrix row",
	// Private keeps the key out of `ze env list`. It is a build-host path the
	// verify runner owns, and an operator has nothing to do with it.
	Private: true,
})

// The two tags every matrix row supplies. They are never declared in the
// manifest, because every build carries them.
const (
	coreTag   = "ze_core"
	distroTag = "ze_distro"
)

// minMatrixRows is the floor no scoped matrix goes under. all_features and
// core_only judge the two combinations Ze ships, so every run judges them
// however narrow its change set is.
const minMatrixRows = 2

var (
	featureTagPattern  = regexp.MustCompile(`^ze_[A-Za-z0-9_]+$`)
	matrixNamePattern  = regexp.MustCompile(`^[A-Za-z0-9_]+$`)
	packagePathPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._~/-]*$`)
)

// Row is one feature-tag combination the tree is type-checked in, and it is one
// ROW of the matrix answer.
type Row struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
	// Omits names the one feature tag this row removes from all_features. It is
	// empty for all_features and core_only, the two rows every run judges, and
	// that emptiness is what Scope reads as "never subtract me".
	Omits string `json:"omits,omitempty"`
}

// Matrix is the whole set of rows one run judges.
type Matrix []Row

// changeScope is the part of the matrix a change set can move: the feature tags
// the change reaches, or every feature when the answer cannot be narrowed.
type changeScope struct {
	tags  map[string]bool
	every bool
}

// Notice is what a run says about ITSELF before any verdict: whether the matrix
// was narrowed to the rows this change set can move, and why it was not.
//
// It is data rather than a printed line because the scope is a fact about the
// run that a caller of `| json` has as much reason to read as a person does.
type Notice struct {
	// Widened is the reason the scope could not be narrowed, and is empty when
	// nothing went wrong. A reason is not a failure: every doubt judges the
	// whole matrix.
	Widened string `json:"widened,omitempty"`
	// Scoped says the run judges fewer rows than the manifest implies.
	Scoped bool `json:"scoped"`
	// Reached is the number of feature tags the change set reaches.
	Reached int `json:"reached"`
	// Judged is the number of rows this run judges.
	Judged int `json:"judged"`
	// Total is the number of rows the whole matrix holds.
	Total int `json:"total"`
}

// Text renders the two lines the script wrote to stderr before judging: the
// widening reason when there is one, and the scoped-run count when the matrix
// was narrowed. It is empty for an unscoped run that had no doubt.
func (n Notice) Text() string {
	var tb textbuf.Buffer
	if n.Widened != "" {
		tb.Str("staticcheck feature matrix: ").Str(n.Widened).Str(", so every row is judged\n")
	}
	if n.Scoped {
		tb.Str("staticcheck feature matrix: the change set reaches ").Int(int64(n.Reached)).
			Str(" feature tag(s), so ").Int(int64(n.Judged)).
			Str(" of ").Int(int64(n.Total)).Str(" rows are judged\n")
	}
	return tb.String()
}

// Derive answers the rows this run judges over tree -- every row the manifest
// implies, less the rows the change set cannot move -- and what the run says
// about its own scope.
//
// The feature-tag answer comes from the environment, which is how the verify
// runner hands one run's scope to every stage it starts.
func Derive(tree string) (Matrix, Notice, error) {
	return DeriveScoped(tree, env.Get(scopeTagsEntry.Key))
}

// DeriveScoped is Derive with the feature-tag answer named explicitly.
//
// It exists because env.Get caches the whole environment on its first call, so
// a caller that sets the variable afterwards is reading a value that is no
// longer there. A test and a parity comparison therefore name the answer, and
// the environment is read at the command boundary alone.
func DeriveScoped(tree, answerPath string) (Matrix, Notice, error) {
	tags, err := readFeatureTags(filepath.Join(tree, featureManifest))
	if err != nil {
		return nil, Notice{}, err
	}

	scope, widen := readChangeScope(answerPath, tags)
	notice := Notice{Total: len(tags) + minMatrixRows, Reached: len(tags)}
	if widen != nil {
		notice.Widened = widen.Error()
	}

	rows, err := buildMatrix(tags)
	if err != nil {
		return nil, notice, err
	}
	scoped, err := scopeMatrix(rows, scope)
	if err != nil {
		return nil, notice, err
	}

	notice.Judged = len(scoped)
	if !scope.every {
		notice.Scoped = true
		notice.Reached = len(scope.tags)
	}
	return scoped, notice, nil
}

// readChangeScope reads the run's feature-tag answer and answers the scope the
// matrix judges. The second result is the reason the scope was widened.
//
// Every doubt widens: no answer named, an answer that cannot be read, and an
// answer naming a tag the manifest in hand does not declare all judge the whole
// matrix. A narrower scope is taken only from an answer that parses against
// that manifest, because a guard that cannot read its input must not return a
// valid-looking narrow answer.
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
	for rawLine := range strings.SplitSeq(string(raw), "\n") {
		tag := strings.TrimSpace(rawLine)
		if tag == "" {
			continue
		}
		if !declared[tag] {
			return changeScope{every: true}, fmt.Errorf(
				"feature-tag answer %s names %q, which %s does not declare", answerPath, tag, featureManifest)
		}
		reached[tag] = true
	}
	if len(reached) == len(declared) {
		return changeScope{every: true}, nil
	}
	return changeScope{tags: reached}, nil
}

// scopeMatrix subtracts the rows the change set cannot move.
//
// It SUBTRACTS from the derived rows and never names the rows to keep: a tag
// added to feature-gates.txt gains its row in rowsForTags and is judged here
// for free, where a list of row names would silently stop covering it.
//
// A row that omits tag T differs from all_features only in the packages T
// gates. A change confined to packages gated by the tags in scope therefore
// moves such a row only when T is one of them: nothing always-on imports a
// gated package, and the dependency audit refuses a gated package importing one
// gated by a different tag, so removing any other tag leaves both the changed
// package and its whole import closure standing.
//
// That import argument is about what a row DROPS, and it is only half of what
// makes the subtraction sound. A file constrained !ze_T is compiled by the rows
// that dropped T and by no other, so a change to `ze_web && !ze_ssh` source is
// visible in without_ze_ssh alone. The producer of the answer readChangeScope
// reads is what closes that half: it unions the tags a changed file NEGATES
// into the scope, so the row survives the subtraction here.
func scopeMatrix(rows Matrix, scope changeScope) (Matrix, error) {
	if scope.every {
		return rows, nil
	}
	scoped := make(Matrix, 0, len(scope.tags)+minMatrixRows)
	for _, row := range rows {
		if row.Omits == "" || scope.tags[row.Omits] {
			scoped = append(scoped, row)
		}
	}
	if err := validateScoped(scoped); err != nil {
		return nil, err
	}
	return scoped, nil
}

// validateScoped refuses a scoped matrix that lost its floor. The two rows
// omitting no tag are all_features and core_only, and they judge what Ze ships:
// a filter that drops either one leaves a shipped combination unjudged.
func validateScoped(rows Matrix) error {
	if len(rows) < minMatrixRows {
		return fmt.Errorf(
			"scoped matrix has %d rows, want at least %d: all_features and core_only judge the shipped combinations",
			len(rows), minMatrixRows)
	}
	floor := 0
	for _, row := range rows {
		if row.Omits == "" {
			floor++
		}
	}
	if floor != minMatrixRows {
		return fmt.Errorf(
			"scoped matrix keeps %d of the %d rows that omit no feature tag: all_features and core_only are never subtracted",
			floor, minMatrixRows)
	}
	return nil
}

// Part answers the rows of one piece of a run that was cut into count pieces.
// Both numbers are counted from one, so an undivided run is part 1 of 1 and
// gets every row back.
//
// The rows are DEALT round-robin rather than cut into contiguous blocks. Two
// facts make that the right shape. all_features is the widest row and core_only
// the narrowest, and they are always the first two, so a contiguous first block
// carries both the most and the least expensive work. And a scoped run keeps
// those two plus one row per reached tag, so a contiguous cut would put a small
// scoped matrix entirely in the first piece and leave the rest with nothing.
//
// Dealing also keeps the partition derived from the matrix: a tag added to
// feature-gates.txt gains its row in rowsForTags and lands in one piece here,
// with nothing to assign by hand.
func (m Matrix) Part(index, count int) (Matrix, error) {
	if count < 1 {
		return nil, fmt.Errorf("the matrix is cut into %d pieces, want at least 1", count)
	}
	if index < 1 || index > count {
		return nil, fmt.Errorf("part %d is outside the 1 to %d the run was cut into", index, count)
	}
	part := make(Matrix, 0, len(m)/count+1)
	for position, row := range m {
		if position%count == index-1 {
			part = append(part, row)
		}
	}
	return part, nil
}

// readFeatureTags answers the feature tags the manifest declares, sorted and
// deduplicated. Every malformed line is an error: a manifest this reader could
// not understand would otherwise shrink the matrix silently.
func readFeatureTags(manifestPath string) ([]string, error) {
	raw, err := os.ReadFile(manifestPath) //nolint:gosec // caller supplies the explicit manifest path
	if err != nil {
		return nil, fmt.Errorf("read feature manifest %s: %w", featureManifest, err)
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
			return nil, fmt.Errorf("%s:%d: expected <tag> <package>, got %d fields", featureManifest, index+1, len(fields))
		}
		tag, packagePath := fields[0], fields[1]
		if !featureTagPattern.MatchString(tag) {
			return nil, fmt.Errorf(
				"%s:%d: invalid feature tag %q; want ze_ followed by letters, numbers, or underscores",
				featureManifest, index+1, tag)
		}
		if tag == coreTag || tag == distroTag {
			return nil, fmt.Errorf(
				"%s:%d: reserved feature tag %q is supplied by every matrix row and must not be in the manifest",
				featureManifest, index+1, tag)
		}
		if !validPackagePath(packagePath) {
			return nil, fmt.Errorf(
				"%s:%d: invalid package path %q; want a clean relative Go import path",
				featureManifest, index+1, packagePath)
		}
		if seen[tag] {
			continue
		}
		seen[tag] = true
		tags = append(tags, tag)
	}
	if len(tags) == 0 {
		return nil, fmt.Errorf("%s: no feature tags found", featureManifest)
	}
	sort.Strings(tags)
	return tags, nil
}

// validPackagePath reports whether a manifest's second field is a clean
// relative Go import path.
func validPackagePath(packagePath string) bool {
	return packagePathPattern.MatchString(packagePath) &&
		pathpkg.Clean(packagePath) == packagePath &&
		!strings.Contains(packagePath, "//")
}

// buildMatrix derives the rows for a tag set and checks the derivation against
// itself before handing it back.
func buildMatrix(tags []string) (Matrix, error) {
	sorted, err := validateAndSortTags(tags)
	if err != nil {
		return nil, err
	}
	rows := rowsForTags(sorted)
	if err := validateMatrix(sorted, rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// validateAndSortTags refuses a tag set the matrix cannot be built from.
func validateAndSortTags(tags []string) ([]string, error) {
	if len(tags) == 0 {
		return nil, fmt.Errorf("matrix requires at least one feature tag")
	}
	sorted := append([]string(nil), tags...)
	sort.Strings(sorted)
	for index, tag := range sorted {
		if !featureTagPattern.MatchString(tag) {
			return nil, fmt.Errorf("matrix feature tag %q is invalid", tag)
		}
		if tag == coreTag || tag == distroTag {
			return nil, fmt.Errorf("matrix feature tag %q is reserved", tag)
		}
		if index > 0 && tag == sorted[index-1] {
			return nil, fmt.Errorf("matrix feature tag %q is duplicated", tag)
		}
	}
	return sorted, nil
}

// rowsForTags answers all_features, core_only, and one without_<tag> row per
// declared feature, in that order.
func rowsForTags(tags []string) Matrix {
	allTags := make([]string, 0, len(tags)+2)
	allTags = append(allTags, coreTag, distroTag)
	allTags = append(allTags, tags...)

	rows := make(Matrix, 0, len(tags)+2)
	rows = append(rows,
		Row{Name: "all_features", Tags: allTags},
		Row{Name: "core_only", Tags: []string{coreTag}},
	)
	for omitted, tag := range tags {
		rowTags := make([]string, 0, len(tags)+1)
		rowTags = append(rowTags, coreTag, distroTag)
		rowTags = append(rowTags, tags[:omitted]...)
		rowTags = append(rowTags, tags[omitted+1:]...)
		var tb textbuf.Buffer
		rows = append(rows, Row{Name: tb.Str("without_").Str(tag).String(), Tags: rowTags, Omits: tag})
	}
	return rows
}

// validateMatrix checks a derived matrix against a fresh derivation of the same
// tags, so a row that lost its name, its tags or its omission is refused rather
// than handed to Staticcheck.
func validateMatrix(tags []string, rows Matrix) error {
	if len(tags) == 0 {
		return fmt.Errorf("matrix requires at least one feature tag")
	}
	if want := len(tags) + 2; len(rows) != want {
		return fmt.Errorf("matrix row count is %d, want %d for %d unique feature tags", len(rows), want, len(tags))
	}
	if err := validateRows(rows); err != nil {
		return err
	}

	want := rowsForTags(tags)
	for index := range want {
		if rows[index].Name != want[index].Name {
			return fmt.Errorf("matrix row %d has name %q, want %q", index+1, rows[index].Name, want[index].Name)
		}
		if !equalStrings(rows[index].Tags, want[index].Tags) {
			return fmt.Errorf("matrix row %q has tags %q, want %q",
				rows[index].Name, strings.Join(rows[index].Tags, ","), strings.Join(want[index].Tags, ","))
		}
		if rows[index].Omits != want[index].Omits {
			return fmt.Errorf("matrix row %q omits %q, want %q", rows[index].Name, rows[index].Omits, want[index].Omits)
		}
	}
	return nil
}

// validateRows refuses a row Staticcheck could not be asked about: no rows, a
// name it cannot spell, a duplicate name, no build tag, or a duplicate tag.
func validateRows(rows Matrix) error {
	if len(rows) == 0 {
		return fmt.Errorf("matrix has zero rows")
	}
	names := make(map[string]bool, len(rows))
	for _, row := range rows {
		if !matrixNamePattern.MatchString(row.Name) {
			return fmt.Errorf("matrix has invalid row name %q; want letters, numbers, or underscores", row.Name)
		}
		if names[row.Name] {
			return fmt.Errorf("matrix has duplicate row name %q", row.Name)
		}
		names[row.Name] = true
		if len(row.Tags) == 0 {
			return fmt.Errorf("matrix row %q has no build tags", row.Name)
		}
		rowTags := make(map[string]bool, len(row.Tags))
		for _, tag := range row.Tags {
			if !featureTagPattern.MatchString(tag) {
				return fmt.Errorf("matrix row %q has invalid build tag %q", row.Name, tag)
			}
			if rowTags[tag] {
				return fmt.Errorf("matrix row %q has duplicate build tag %q", row.Name, tag)
			}
			rowTags[tag] = true
		}
	}
	return nil
}

// equalStrings reports whether two tag lists hold the same tags in the same
// order.
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
