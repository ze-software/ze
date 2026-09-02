// Design: docs/architecture/core-design.md -- attributing a verify red to its files
// Overview: docwiring.go -- the gate that declares these groups
//
// groups.go is how a failure of this gate says which files it is about.
//
// The producer knows which files caused a failure. The verify runner can only
// guess from prose. Thus, each failure declares a group whose `related` field
// names its repository paths. The field is empty for a check over a POPULATION.
// The commit gate attributes path-bearing groups to their files. It charges
// groups without paths to the committing session because a check-name guess can
// leave a real failure uncharged.

package docwiring

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	// failureGroupPrefix opens a declared group's line.
	failureGroupPrefix = "VERIFY FAILURE GROUP:"
	// failureGroupsCompletePrefix opens the line stating how many groups a run
	// declared.
	failureGroupsCompletePrefix = "VERIFY FAILURE GROUPS COMPLETE:"

	// pathBearingKind marks a group that names files. The commit gate attributes
	// such a failure to those files.
	pathBearingKind = "files"
	// unattributableKind marks a group with no files. The commit gate charges
	// such a failure to the committing session.
	unattributableKind = "subcheck"

	// RelatedPerGroup bounds how many paths one group line carries.
	//
	// The verify log scanner has a 4 MiB token limit. An oversized line stops
	// the scan, so it and all later lines become unclassified. A tree-wide check
	// can name hundreds of files. Each group therefore has a bounded path count,
	// and remaining paths use sibling groups. Every line stays far below the
	// limit, and no hidden truncated path can conceal which session caused the
	// failure.
	RelatedPerGroup = 50
)

// Group declares one failure: what failed, its files, and how to reproduce it.
type Group struct {
	GroupID string   `json:"group-id"`
	Kind    string   `json:"kind"`
	Related []string `json:"related"`
	Summary string   `json:"summary"`
	Rerun   string   `json:"rerun"`
}

// declareFailureGroup records the group for the failure being reported, right
// here.
//
// It MUST be called at the failure point, not in an end-of-run dump. The run
// stops after a wiring failure or failed delegated target. A final dump would
// include only failures that survived until the end.
func (g *checker) declareFailureGroup(check string, related []string, summary, rerun string) {
	kind := unattributableKind
	if len(related) > 0 {
		kind = pathBearingKind
	}

	var tb textbuf.Buffer
	for number, chunk := range chunkPaths(related) {
		id := tb.Reset().Str(kind).Byte(':').Str(check).String()
		if number > 0 {
			id = tb.Reset().Str(kind).Byte(':').Str(check).Byte('#').Int(int64(number + 1)).String()
		}
		g.report.Groups = append(g.report.Groups, Group{
			GroupID: id,
			Kind:    kind,
			Related: chunk,
			Summary: summary,
			Rerun:   rerun,
		})
	}
}

// chunkPaths splits a related list into group-sized pieces. A list with no
// paths still yields ONE piece, because a failure that names no file still owes
// a group.
func chunkPaths(related []string) [][]string {
	if len(related) == 0 {
		return [][]string{{}}
	}
	var chunks [][]string
	for start := 0; start < len(related); start += RelatedPerGroup {
		end := min(start+RelatedPerGroup, len(related))
		chunks = append(chunks, related[start:end])
	}
	return chunks
}

// findingPrefixRe reads the `<path>:<line>: ` prefix from doc-link and wiring
// findings. The path identifies the file with the bad reference. Its owner
// therefore owns the failure.
var findingPrefixRe = regexp.MustCompile(`^([^\s:]+):\d+: `)

// findingPaths answers the repository paths a run of findings names, sorted and
// deduplicated.
//
// A token remains only when it is a relative path in the checkout. The commit
// gate later applies the same test. A finding with no valid prefix contributes
// no path. A check without paths declares an unattributable group.
func findingPaths(root string, findings []string) []string {
	found := make(map[string]bool)
	for _, line := range findings {
		m := findingPrefixRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		candidate := m[1]
		if strings.HasPrefix(candidate, "/") || escapes(candidate) {
			continue // absolute or escaping: not a path in this checkout
		}
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(candidate))); err == nil {
			found[candidate] = true
		}
	}

	out := make([]string, 0, len(found))
	for path := range found {
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}

// escapes reports a relative path that walks out of the checkout.
func escapes(candidate string) bool {
	return slices.Contains(strings.Split(filepath.ToSlash(candidate), "/"), "..")
}

// Line renders one group the way the verify runner reads it back.
//
// JSON escapes a quote, newline, or prefix inside a path. The path cannot forge
// another group. HTML escaping is off because a JSON parser reads the output,
// not a browser. A path with an angle bracket reads as itself.
func (gr Group) Line() string {
	var out textbuf.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(gr); err != nil {
		// Every field is a string or a string slice, so the encoder has no
		// value it can refuse. Answering the prefix alone keeps the count and
		// the line population in step rather than dropping a line the reader is
		// about to count.
		return failureGroupPrefix
	}
	var tb textbuf.Buffer
	return tb.Str(failureGroupPrefix).Byte(' ').Str(strings.TrimRight(out.String(), "\n")).String()
}
