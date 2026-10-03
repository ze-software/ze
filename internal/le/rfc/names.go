// Design: docs/architecture/core-design.md -- the rfc area, as one command
// Related: rename.go -- the rename refuses a target this predicate refuses
// Related: carriers.go -- CarrierFor decides which files are judged
//
// names.go holds the one naming rule for a unit test file and its RFC tags. A
// file named rfcNNNN_<topic>_test.go carries a tag for RFC NNNN or a marker
// saying why it carries none; a file whose tags cite exactly one stem is named
// for that stem. `./le rfc check` and `./le rfc rename` both judge through it.
package rfc

import (
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// namingMarkerText opens the comment that says a stem-named file carries no tag
// on purpose. The reason follows it on the same line and MUST NOT be empty.
// It never holds tagMarker's phrase, so goTagRE cannot read it as a tag.
const namingMarkerText = "// RFC naming: untagged --"

// namingMarkerLead is the part of the marker that says a line is MEANT as one,
// so a misspelled marker is refused rather than ignored.
const namingMarkerLead = "// RFC naming:"

// testFileSuffix is the only file shape the naming rule judges.
const testFileSuffix = "_test.go"

// stemPrefix answers the file-name prefix of a summary stem: the stem with its
// hyphens turned into underscores, then one underscore. rfc792 gives rfc792_
// with no padding; sflow-v5 gives sflow_v5_.
func stemPrefix(stem string) string {
	return strings.ReplaceAll(stem, "-", "_") + "_"
}

// stemOfFileName answers the stem a base file name is named for: the LONGEST
// known stem whose prefix opens it, so rfc792_x is never read as rfc79. A name
// opening rfc<digits>_ whose stem has no summary is still named for that stem.
// Empty when the name opens with no stem.
func stemOfFileName(base string, stems map[string]bool) string {
	best := ""
	for stem := range stems {
		if !strings.HasPrefix(base, stemPrefix(stem)) {
			continue
		}
		if len(stem) > len(best) {
			best = stem
		}
	}
	if best != "" {
		return best
	}
	return rfcNumberStem(base)
}

// rfcNumberStem answers rfc<digits> when base opens with rfc<digits>_, and empty
// otherwise.
func rfcNumberStem(base string) string {
	if !strings.HasPrefix(base, "rfc") {
		return ""
	}
	digits := 0
	for digits < len(base)-3 && base[3+digits] >= '0' && base[3+digits] <= '9' {
		digits++
	}
	if digits == 0 {
		return ""
	}
	if 3+digits >= len(base) || base[3+digits] != '_' {
		return ""
	}
	return base[:3+digits]
}

// namingMarker is what one file says about its own missing tag.
type namingMarker struct {
	Present bool
	Reason  string
}

// readNamingMarker answers the first marker line in src. A line opening with
// the marker lead but not the full marker text reads as present with no
// reason, which the rule refuses.
func readNamingMarker(src string) namingMarker {
	for line := range strings.SplitSeq(src, "\n") {
		text := strings.TrimSpace(line)
		if !strings.HasPrefix(text, namingMarkerLead) {
			continue
		}
		reason, full := strings.CutPrefix(text, namingMarkerText)
		if !full {
			return namingMarker{Present: true}
		}
		return namingMarker{Present: true, Reason: strings.TrimSpace(reason)}
	}
	return namingMarker{}
}

// namedTestFile is everything the naming rule reads about one file.
type namedTestFile struct {
	Rel      string
	TagStems map[string]bool
	Marker   namingMarker
}

// nameVerdict is one file the rule refuses. Target is the exact rename that
// repairs it when one exists, and empty when the file needs a marker or a
// hand-chosen topic instead.
type nameVerdict struct {
	Rel     string
	Problem string
	Target  string
}

// judgeTestFileName applies the naming rule to one file, and false when the
// file passes. Part (a) asks that a stem-named file tag that stem or carry a
// true marker; part (b) asks that a file whose tags cite exactly one stem be
// named for it. Part (b) is judged first, because its repair is one rename.
func judgeTestFileName(file namedTestFile, stems map[string]bool) (nameVerdict, bool) {
	dir, base := path.Split(file.Rel)
	nameStem := stemOfFileName(base, stems)
	topic := base
	if nameStem != "" {
		topic = strings.TrimPrefix(base, stemPrefix(nameStem))
	}
	if single, ok := singleStem(file.TagStems); ok && single != nameStem {
		var tb textbuf.Buffer
		target := dir + stemPrefix(single) + topic
		tb.Str("every tag in it cites ").Str(single).Str(", so it is named for that RFC")
		return nameVerdict{Rel: file.Rel, Problem: tb.String(), Target: target}, true
	}
	if file.Marker.Present {
		return judgeNamingMarker(file, nameStem)
	}
	if nameStem == "" || file.TagStems[nameStem] {
		return nameVerdict{}, false
	}
	var tb textbuf.Buffer
	suggested := topic
	if suggested == "test.go" {
		suggested = "<topic>_test.go"
	}
	tb.Str("it is named for ").Str(nameStem).Str(" and carries no tag for it: tag what it ").
		Str("covers, add `").Str(namingMarkerText).Str(" <reason>`, or name it after what ").
		Str("it covers (./le rfc rename from ").Str(file.Rel).Str(" to ").Str(dir).Str(suggested).Str(")")
	return nameVerdict{Rel: file.Rel, Problem: tb.String()}, true
}

// judgeNamingMarker refuses a marker that is empty, that contradicts a tag for
// the name stem, or that sits in a file named for no stem.
func judgeNamingMarker(file namedTestFile, nameStem string) (nameVerdict, bool) {
	var tb textbuf.Buffer
	switch {
	case file.Marker.Reason == "":
		tb.Str("its naming marker gives no reason: write `").Str(namingMarkerText).Str(" <reason>`")
	case nameStem == "":
		tb.Str("it carries a naming marker but is not named for any RFC: delete the marker")
	case file.TagStems[nameStem]:
		tb.Str("it carries a naming marker but also carries a tag for ").Str(nameStem).
			Str(": delete the marker")
	default:
		return nameVerdict{}, false
	}
	return nameVerdict{Rel: file.Rel, Problem: tb.String()}, true
}

// singleStem answers the one stem a tag set cites, and false for none or many.
func singleStem(tagStems map[string]bool) (string, bool) {
	if len(tagStems) != 1 {
		return "", false
	}
	for stem := range tagStems {
		return stem, true
	}
	return "", false
}

// tagStemsByFile answers the stems each file's proof and gap tags cite. A tag
// resolves through its requirement's stem; an id no requirement declares falls
// back to the longest summary stem it is prefixed by, and an id that resolves to
// no stem is left out, because another check already refuses an unknown id.
func tagStemsByFile(tags []Tag, requirements []Requirement, stems map[string]bool) map[string]map[string]bool {
	stemByRID := make(map[string]string, len(requirements))
	for position := range requirements {
		stemByRID[requirements[position].RID] = requirements[position].RFC
	}
	out := map[string]map[string]bool{}
	for position := range tags {
		stem := tagStem(tags[position].RID, stemByRID, stems)
		if stem == "" {
			continue
		}
		file := tags[position].File
		if out[file] == nil {
			out[file] = map[string]bool{}
		}
		out[file][stem] = true
	}
	return out
}

func tagStem(rid string, stemByRID map[string]string, stems map[string]bool) string {
	if stem, known := stemByRID[rid]; known {
		return stem
	}
	best := ""
	for stem := range stems {
		if hasRIDStem(rid, stem) && len(stem) > len(best) {
			best = stem
		}
	}
	return best
}

// testFileNameVerdicts walks the test roots and answers every unit test file
// the naming rule refuses, sorted by path. Only `_test.go` files the unit
// carrier holds are judged: interop carriers, test/draft/, internal/le/,
// testdata/ and vendor/ are not (CarrierFor, skipDirs).
//
// tags MUST hold both the proof and the gap tags (Collected.Tags and GapTags):
// a gap tag cites its stem for naming just as a proof tag does.
func testFileNameVerdicts(tree string, carriers []Carrier, tags []Tag, requirements []Requirement,
	stems map[string]bool) ([]nameVerdict, error) {
	stemsByFile := tagStemsByFile(tags, requirements, stems)
	var out []nameVerdict
	for _, sub := range testRoots {
		root := treePath(tree, sub)
		info, statErr := os.Stat(root)
		if statErr != nil || !info.IsDir() {
			continue
		}
		walkErr := filepath.WalkDir(root, func(full string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if skipDirs[entry.Name()] {
					return filepath.SkipDir
				}
				return nil
			}
			rel := relTo(tree, full)
			if !strings.HasSuffix(rel, testFileSuffix) {
				return nil
			}
			carrier, held := CarrierFor(rel, carriers)
			if !held || carrier.Kind != kindUnit {
				return nil
			}
			src, readErr := readFile(full, rel)
			if readErr != nil {
				return readErr
			}
			file := namedTestFile{Rel: rel, TagStems: stemsByFile[rel], Marker: readNamingMarker(src)}
			if verdict, refused := judgeTestFileName(file, stems); refused {
				out = append(out, verdict)
			}
			return nil
		})
		if walkErr != nil {
			return nil, walkErr
		}
	}
	slices.SortFunc(out, func(a, b nameVerdict) int { return strings.Compare(a.Rel, b.Rel) })
	return out, nil
}

// checkTestFileNames answers one finding per unit test file whose name and RFC
// tags disagree. A finding with a repair names the exact `./le rfc rename`
// command that makes it.
func checkTestFileNames(tree string, carriers []Carrier, tags []Tag, requirements []Requirement,
	stems map[string]bool) ([]string, error) {
	verdicts, err := testFileNameVerdicts(tree, carriers, tags, requirements, stems)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(verdicts))
	for _, verdict := range verdicts {
		var tb textbuf.Buffer
		tb.Str(verdict.Rel).Str(": test file name: ").Str(verdict.Problem)
		if verdict.Target != "" {
			tb.Str(": ./le rfc rename from ").Str(verdict.Rel).Str(" to ").Str(verdict.Target)
		}
		out = append(out, tb.String())
	}
	return out, nil
}
