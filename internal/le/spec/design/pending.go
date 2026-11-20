// Design: docs/contributing/spec-workflow.md -- the ready specs whose design waits for Thomas
// Related: approval.go -- the one approval lookup each spec is asked of
// Related: away.go -- the away state the list ends with
//
// pending.go is `le spec design pending`: every ready spec with no approval of
// its design as it stands, by Priority, then category, then Updated, each with
// what it lacks; then every spec started under an away admission Thomas has
// not approved since; then the away state
// (plan/release/validation/spec-design-needs-owner-approval.md, AC-30, C-2).
package specdesign

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	specpath "github.com/ze-software/ze/internal/le/spec/path"
	specstatus "github.com/ze-software/ze/internal/le/spec/status"
)

// statusReady is the status of a spec that waits for an approval.
const statusReady = "ready"

// startedStatuses are the statuses of a spec whose implementation started.
var startedStatuses = []string{"in-progress", "verification", "blocked"}

// What a pending spec can lack, in the order the line names them.
const (
	lacksDocuments    = documentsHeading
	lacksPresentation = presentationHeading
	lacksJudge        = "judge"
	lacksApproval     = "approval"
	lacksSpecName     = "a spec file name the records can key on"
)

// pendingSpec is one ready spec that waits for Thomas.
type pendingSpec struct {
	Path     string   `json:"path"`
	Priority string   `json:"priority"`
	Category string   `json:"category"`
	Updated  string   `json:"updated"`
	Lacks    []string `json:"lacks"`
	// order is the category's place in specpath.Categories.
	order int
}

// awayStart is a spec started under an away admission Thomas has not
// approved since.
type awayStart struct {
	Path   string `json:"path"`
	Status string `json:"status"`
	From   string `json:"from"`
	Until  string `json:"until"`
}

// pendingAnswer is the answer of `le spec design pending`.
type pendingAnswer struct {
	Ready      []pendingSpec `json:"ready"`
	AwayStarts []awayStart   `json:"away-starts"`
	// Away is the active absence, or nil when Thomas is not away.
	Away *Away `json:"away"`
}

// Text renders the list for Thomas: one line per spec, then the away state.
func (a pendingAnswer) Text() string {
	var tb textbuf.Buffer
	for _, spec := range a.Ready {
		updated := spec.Updated
		if updated == "" {
			updated = "an unrecorded date"
		}
		tb.Str(spec.Priority).Byte(' ').Str(spec.Category).Str("  ").Str(filepath.Base(spec.Path)).
			Str("  ready since ").Str(updated).Str("  lacks: ").Str(strings.Join(spec.Lacks, ", ")).Byte('\n')
	}
	for _, start := range a.AwayStarts {
		tb.Str("started while Thomas was away (").Str(start.From).Str(" to ").Str(start.Until).Str("): ").
			Str(filepath.Base(start.Path)).Str(", ").Str(start.Status).Str(", not approved since\n")
	}
	tb.Str(strconv.Itoa(len(a.Ready))).Str(" ready specs wait for an approval. ")
	if a.Away == nil {
		return tb.Str("Thomas is not away.\n").String()
	}
	return tb.Str("Thomas is away ").Str(a.Away.From).Str(" to ").Str(a.Away.Until).Str(".\n").String()
}

// answerPending is `le spec design pending`.
func answerPending() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		return nil, reportError(err)
	}
	answer, err := pending(root, time.Now())
	if err != nil {
		return nil, reportError(err)
	}
	return answer, 0
}

// pending reads every current spec once and answers the list. An error is a
// spec, a record or the history that could not be read, never an empty list.
func pending(root string, now time.Time) (pendingAnswer, error) {
	paths, err := specpath.All(root)
	if err != nil {
		return pendingAnswer{}, err
	}
	categories := specpath.Categories()
	lookup := approvalLookup{root: root}
	var answer pendingAnswer
	for _, relative := range paths {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative))) //nolint:gosec // a spec path specpath.All listed
		if err != nil {
			return pendingAnswer{}, fmt.Errorf("read %s: %w", relative, err)
		}
		meta, err := specstatus.Parse(data, relative, func(string) {})
		if err != nil {
			return pendingAnswer{}, err
		}
		if meta.Status == statusReady {
			waiting, waits, err := readySpecWaits(root, &lookup, meta, categories)
			if err != nil {
				return pendingAnswer{}, err
			}
			if waits {
				answer.Ready = append(answer.Ready, waiting)
			}
			continue
		}
		if !slices.Contains(startedStatuses, meta.Status) {
			continue
		}
		start, admitted, err := startedWhileAway(root, &lookup, meta)
		if err != nil {
			return pendingAnswer{}, err
		}
		if admitted {
			answer.AwayStarts = append(answer.AwayStarts, start)
		}
	}
	slices.SortStableFunc(answer.Ready, comparePending)
	away, found, err := ActiveAway(root, now)
	if err != nil {
		return pendingAnswer{}, err
	}
	if found {
		answer.Away = &away
	}
	return answer, nil
}

// readySpecWaits answers the pending row of the ready spec meta describes;
// waits is false only when Thomas approved its design as it stands. An away
// admission of that text still waits, lacking the approval.
func readySpecWaits(root string, lookup *approvalLookup, meta specstatus.Spec, categories []specpath.Category) (waiting pendingSpec, waits bool, err error) {
	waiting = pendingSpec{Path: meta.Path, Priority: meta.Priority, Updated: meta.Updated, order: len(categories)}
	if waiting.Priority == "" {
		waiting.Priority = "-"
	}
	waiting.Category = meta.Bucket
	for position, category := range categories {
		if category.Name == meta.Bucket {
			waiting.order = position
			waiting.Category = strings.TrimPrefix(category.Dir, specpath.Root+"/")
		}
	}
	spec, err := loadSpec(root, meta.Path)
	if err != nil {
		// A file name the records cannot key on can never be approved: it is
		// listed with that lack rather than hiding the rest of the list.
		waiting.Lacks = []string{lacksSpecName, lacksApproval}
		return waiting, true, nil //nolint:nilerr // the refusal is the listed lack
	}
	approval, found, err := lookup.approved(spec.stem, spec.text)
	if err != nil {
		return pendingSpec{}, false, err
	}
	// An away admission is not Thomas's approval: the spec waits for him to
	// review it on return, so only his own approval leaves it off the list.
	if found && !approval.Away {
		return pendingSpec{}, false, nil
	}
	if _, found := spec.section(documentsHeading); !found {
		waiting.Lacks = append(waiting.Lacks, lacksDocuments)
	}
	if _, found := spec.section(presentationHeading); !found {
		waiting.Lacks = append(waiting.Lacks, lacksPresentation)
	}
	problem, err := judgeApprovalProblem(root, spec)
	if err != nil {
		return pendingSpec{}, false, err
	}
	if problem != "" {
		waiting.Lacks = append(waiting.Lacks, lacksJudge)
	}
	waiting.Lacks = append(waiting.Lacks, lacksApproval)
	return waiting, true, nil
}

// startedWhileAway answers the started spec meta describes; admitted is true
// only when the approval of its text is an away admission.
func startedWhileAway(root string, lookup *approvalLookup, meta specstatus.Spec) (start awayStart, admitted bool, err error) {
	spec, err := loadSpec(root, meta.Path)
	if err != nil {
		// No admission can name a file the records cannot key on.
		return awayStart{}, false, nil //nolint:nilerr // such a spec holds no approval of any kind
	}
	approval, found, err := lookup.approved(spec.stem, spec.text)
	if err != nil {
		return awayStart{}, false, err
	}
	if !found {
		return awayStart{}, false, nil
	}
	if !approval.Away {
		return awayStart{}, false, nil
	}
	return awayStart{Path: meta.Path, Status: meta.Status, From: approval.From, Until: approval.Until}, true, nil
}

// comparePending orders by Priority (P0 first, an unparsed one last), then by
// category in specpath.Categories order, then by Updated, oldest first and an
// unrecorded date last, then by path.
func comparePending(a, b pendingSpec) int {
	if order := priorityRank(a.Priority) - priorityRank(b.Priority); order != 0 {
		return order
	}
	if order := a.order - b.order; order != 0 {
		return order
	}
	if order := compareUpdated(a.Updated, b.Updated); order != 0 {
		return order
	}
	return strings.Compare(a.Path, b.Path)
}

// priorityUnranked is the rank of a Priority cell that is not P<digits>.
const priorityUnranked = 1 << 20

// priorityRank is the number of a P<n> priority, or priorityUnranked.
func priorityRank(priority string) int {
	digits, found := strings.CutPrefix(priority, "P")
	if !found {
		return priorityUnranked
	}
	rank, err := strconv.Atoi(digits)
	if err != nil {
		return priorityUnranked
	}
	if rank < 0 {
		return priorityUnranked
	}
	return rank
}

// compareUpdated orders two Updated cells, oldest first, an empty one last.
func compareUpdated(a, b string) int {
	if a == b {
		return 0
	}
	if a == "" {
		return 1
	}
	if b == "" {
		return -1
	}
	return strings.Compare(a, b)
}
