// Design: docs/architecture/site-facts.md -- the committed-facts pattern, its second instance
// Related: sections.go -- the section each counted path falls in
// Related: days.go -- the recorded days measured here
// Related: update.go -- the update that merges what this measures
//
// The measurement, steps 3 to 7 of the Transformation Path of
// plan/post-release/engineering/spec-activity-codebase-growth.md: list each
// recorded day's committed tree, count each blob once per run, classify each
// path, and sum the day's files, lines, Go split, module count, and the lines
// each section gained and lost against the previous recorded day. Only git
// objects are read, never the working tree or the index.

package sitehistory

import (
	"bytes"
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/le/repo/objects"
	reporewrite "github.com/ze-software/ze/internal/le/repo/rewrite"
)

// binaryProbeBytes is how much of a blob the binary test reads: a NUL byte at
// offsets 0 to 7,999 makes the blob binary. It is git's own probe
// (buffer_is_binary reads the first 8,000 bytes), so a blob git shows as text
// counts lines here too.
const binaryProbeBytes = 8000

// modulesPath is the file whose module count a recorded day carries, named
// from the repository root as a git tree listing names it.
const modulesPath = "vendor/modules.txt"

// blobCount is what one blob counts, cached by blob ID for a run. A facet is
// counted only when a path that reaches the blob needs it, and its flag says
// whether it was.
type blobCount struct {
	// lines is the line rule's count: zero for a binary blob, else one per
	// newline plus one for an unterminated last line.
	lines uint64
	// goLines is the Go split of lines, valid when goCounted is set.
	goLines GoLines
	// goCounted says goLines was counted. A binary blob counts a zero split.
	goCounted bool
	// modules is the module count of a vendor/modules.txt, valid when
	// modulesCounted is set.
	modules uint64
	// modulesCounted says modules was counted.
	modulesCounted bool
}

// covers reports whether the count holds every facet need asks for.
func (c blobCount) covers(need blobNeed) bool {
	if need.goSplit && !c.goCounted {
		return false
	}
	return c.modulesCounted || !need.modules
}

// merged answers c joined with a fresh count of the same blob: the line count
// is the same by construction, and each facet comes from whichever counted it.
func (c blobCount) merged(fresh blobCount) blobCount {
	result := fresh
	if c.goCounted {
		result.goLines, result.goCounted = c.goLines, true
	}
	if c.modulesCounted {
		result.modules, result.modulesCounted = c.modules, true
	}
	return result
}

// blobNeed says which facets the paths of one day that reach a blob need.
type blobNeed struct {
	// goSplit says a Go path of a Go-split section reaches the blob.
	goSplit bool
	// modules says the blob is the tree's vendor/modules.txt.
	modules bool
}

// with answers the facets n and other need together.
func (n blobNeed) with(other blobNeed) blobNeed {
	return blobNeed{goSplit: n.goSplit || other.goSplit, modules: n.modules || other.modules}
}

// fileCount is one regular file of a recorded day: its section and what it
// counts.
type fileCount struct {
	// section is the ID of the section the path falls in.
	section string
	// lines is the file's line count.
	lines uint64
	// goLines is the file's Go split, zero outside a Go-split section.
	goLines GoLines
}

// treeFiles is one recorded day's regular files, keyed by repository path. A
// run holds at most two: the previous day's and the current one.
type treeFiles map[string]fileCount

// regularFile is one regular entry of a listing, classified once.
type regularFile struct {
	// path is the repository-relative slash path.
	path string
	// id is the blob ID.
	id string
	// section is the section the path falls in.
	section Section
	// goSplit says the blob is counted with the Go split: a path ending .go in
	// a Go-split section.
	goSplit bool
}

// measurer measures the recorded days of one run. Its cache holds one count
// per unique blob the run met, never a blob body, so it is bounded by the
// unique blobs of the history (about 159,000 for the whole history). Not safe
// for concurrent use.
type measurer struct {
	// root is the repository root.
	root string
	// command is the `./le site history` line that asked, which a refusal
	// names as the line to run again.
	command string
	// counts caches each counted blob by ID.
	counts map[string]blobCount
}

// measureDays measures days, which MUST be ascending recorded days of one
// resolved commit's history, and answers one record per day in their order.
// baseline, when not nil, is the recorded day immediately before days[0], at
// the commit the history file records for it: the first day's gained and lost
// lines are counted against its tree, and it is never answered as a record.
// With no baseline the first day gains its whole tree against an empty one.
// command is the `./le site history` line that asked, which a refusal names as
// the line to run again once its objects are fetched.
//
// At most two days' path maps are held at once. The records are not judged
// here: the update passes them to NewHistory, whose arithmetic check refuses a
// defect of this code rather than writing it, and the check compares them with
// the file.
func measureDays(ctx context.Context, root, command string, baseline *Day, days []Day) ([]DayRecord, error) {
	run := measurer{root: root, command: command, counts: make(map[string]blobCount)}
	previous := treeFiles{}
	if baseline != nil {
		files, _, err := run.measureTree(ctx, *baseline)
		if err != nil {
			return nil, err
		}
		previous = files
	}

	records := make([]DayRecord, 0, len(days))
	for _, day := range days {
		current, modules, err := run.measureTree(ctx, day)
		if err != nil {
			return nil, err
		}
		records = append(records, dayRecord(day, modules, previous, current))
		previous = current
	}
	return records, nil
}

// measureTree answers the regular files of day's tree and the module count of
// its vendor/modules.txt, zero when the tree holds none. Symbolic links and
// submodules are not regular files and are not counted.
func (m *measurer) measureTree(ctx context.Context, day Day) (treeFiles, uint64, error) {
	entries, err := objects.List(ctx, m.root, day.Tree)
	if err != nil {
		return nil, 0, unreadableDay(err, day, m.command)
	}

	regular := make([]regularFile, 0, len(entries))
	needs := make(map[string]blobNeed)
	for _, entry := range entries {
		if !entry.Regular() {
			continue
		}
		section := SectionOf(entry.Path)
		file := regularFile{path: entry.Path, id: entry.ID, section: section, goSplit: goSplitPath(section, entry.Path)}
		regular = append(regular, file)
		need := blobNeed{goSplit: file.goSplit, modules: entry.Path == modulesPath}
		if m.counted(entry.ID, need) {
			continue
		}
		needs[entry.ID] = needs[entry.ID].with(need)
	}
	if err := m.count(ctx, needs); err != nil {
		return nil, 0, unreadableDay(err, day, m.command)
	}

	files := make(treeFiles, len(regular))
	var modules uint64
	for _, file := range regular {
		count, found := m.counts[file.id]
		if !found {
			return nil, 0, fmt.Errorf("blob %s of %s on day %s was never counted; this is a defect of internal/le/site/history/measure.go", file.id, file.path, day.Date)
		}
		counted := fileCount{section: file.section.ID, lines: count.lines}
		if file.goSplit {
			counted.goLines = count.goLines
		}
		files[file.path] = counted
		if file.path == modulesPath {
			modules = count.modules
		}
	}
	return files, modules, nil
}

// goSplitPath reports whether the blob at path is counted with the Go split:
// a path ending .go in a section that carries the split. Every Go-split
// section's rule matches only .go paths today; the suffix test keeps the
// classifier off any other file a future rule might add there, whose split
// then stays zero and fails NewHistory's sum rather than being invented.
func goSplitPath(section Section, path string) bool {
	return section.GoSplit && strings.HasSuffix(path, ".go")
}

// counted reports whether the run already holds every facet need asks of the
// blob id.
func (m *measurer) counted(id string, need blobNeed) bool {
	count, found := m.counts[id]
	return found && count.covers(need)
}

// count streams every blob needs names and caches its count. Stream sees
// trailing output and a failed git exit only after every visit, so the counts
// one call makes are kept only when it succeeds: on an error nothing it counted
// reaches the cache.
func (m *measurer) count(ctx context.Context, needs map[string]blobNeed) error {
	ids := slices.Sorted(maps.Keys(needs))
	fresh := make(map[string]blobCount, len(ids))
	err := objects.Stream(ctx, m.root, ids, func(id string, blob []byte) error {
		need := needs[id]
		counted, err := countBlob(blob, need.goSplit)
		if err != nil {
			return fmt.Errorf("blob %s: %w", id, err)
		}
		if need.modules {
			counted.modules = countOf(reporewrite.CountVendorModules(blob))
			counted.modulesCounted = true
		}
		fresh[id] = counted
		return nil
	})
	if err != nil {
		return err
	}
	for id, counted := range fresh {
		m.counts[id] = m.counts[id].merged(counted)
	}
	return nil
}

// countBlob applies the line rule to one blob, in this order. A blob with a
// NUL byte at offsets 0 to 7,999 is binary: it counts zero lines, and a Go
// blob a zero split. Any other blob counts one line per newline, plus one for
// an unterminated last line. When goSplit is set, a text blob is also split
// into code, blank and comment lines by reporewrite.CountGoLines, whose Total
// counts lines the same way; a Total that differs is a defect and is refused.
func countBlob(blob []byte, goSplit bool) (blobCount, error) {
	probe := blob[:min(len(blob), binaryProbeBytes)]
	if bytes.IndexByte(probe, 0) >= 0 {
		return blobCount{goCounted: goSplit}, nil
	}

	lines := countOf(bytes.Count(blob, []byte{'\n'}))
	if len(blob) > 0 && blob[len(blob)-1] != '\n' {
		lines++
	}
	counted := blobCount{lines: lines}
	if !goSplit {
		return counted, nil
	}

	split := reporewrite.CountGoLines(blob)
	if countOf(split.Total) != lines {
		return blobCount{}, fmt.Errorf("the Go classifier counts %d lines where the line rule counts %d; "+
			"the two rules must agree, so this is a defect of internal/le/repo/rewrite/activity.go CountGoLines", split.Total, lines)
	}
	counted.goLines = GoLines{Code: countOf(split.Code), Blank: countOf(split.Blank), Comment: countOf(split.Comment)}
	counted.goCounted = true
	return counted, nil
}

// countOf converts a count the shared counters answer as an int. A count is
// never negative, so the conversion cannot wrap.
func countOf(count int) uint64 {
	return uint64(max(count, 0)) //nolint:gosec // max keeps the value non-negative
}

// dayRecord sums one recorded day: each section's files, lines and Go split
// over current, and against previous, per path, the lines each section gained
// and lost. A new path gains all its lines and a removed path loses them; a
// path whose section changed loses its old lines in the old section and gains
// its new lines in the new one; a path that stayed gains or loses the
// difference. A section that holds nothing and changed nothing is left out, as
// the file contract reads an absent section.
func dayRecord(day Day, modules uint64, previous, current treeFiles) DayRecord {
	values := make(map[string]SectionValues)
	for path, file := range current {
		section := values[file.section]
		section.Files++
		section.Lines += file.lines
		section.Go.Code += file.goLines.Code
		section.Go.Blank += file.goLines.Blank
		section.Go.Comment += file.goLines.Comment
		values[file.section] = section

		before, existed := previous[path]
		if !existed {
			addGained(values, file.section, file.lines)
			continue
		}
		if before.section != file.section {
			addLost(values, before.section, before.lines)
			addGained(values, file.section, file.lines)
			continue
		}
		if file.lines >= before.lines {
			addGained(values, file.section, file.lines-before.lines)
			continue
		}
		addLost(values, file.section, before.lines-file.lines)
	}
	for path, before := range previous {
		if _, kept := current[path]; kept {
			continue
		}
		addLost(values, before.section, before.lines)
	}
	for id, section := range values {
		if section == (SectionValues{}) {
			delete(values, id)
		}
	}
	return DayRecord{Date: day.Date, Commit: day.Commit, Modules: modules, Sections: values}
}

// addGained adds lines to what section gained.
func addGained(values map[string]SectionValues, section string, lines uint64) {
	entry := values[section]
	entry.Gained += lines
	values[section] = entry
}

// addLost adds lines to what section lost.
func addLost(values map[string]SectionValues, section string, lines uint64) {
	entry := values[section]
	entry.Lost += lines
	values[section] = entry
}

// unreadableDay is the refusal of a day whose tree or blobs could not all be
// read and counted. A missing object is never read as an empty file, so the
// day is neither recorded nor judged, and nothing is written.
func unreadableDay(err error, day Day, command string) error {
	return opererr.Wrap(err, "cannot measure day "+day.Date.String()+" at commit "+day.Commit,
		"every recorded day is counted from its commit's committed objects, so a day whose objects cannot all be read is neither recorded nor judged, and "+HistoryFile+" is left as it is",
		"git fetch --refetch, then run the same "+command+" again")
}
