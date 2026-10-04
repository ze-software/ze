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
	if 3+digits >= len(base) {
		return ""
	}
	if base[3+digits] != '_' {
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
	// Stem is the stem Target is named for, set whenever Target is.
	Stem string
	// Offer is the rename part (a) offers an untagged stem-named file, one of
	// its three repairs, and empty when its topic held nothing to keep. It is
	// never a Target, because a tag or a marker repairs the file as well, so
	// `./le rfc rename propose` writes no pair for it.
	Offer string
	// Source is the file judged. pairRefusals judges a rename target with its
	// tags and the marker it reads from the file, because the rename moves both.
	Source namedTestFile
}

// judgeTestFileName applies the naming rule to one file, and false when the
// file passes. Part (a) asks that a stem-named file tag that stem or carry a
// true marker; part (b) asks that a file whose tags cite exactly one stem be
// named for it. Part (b) is judged first, because its repair is one rename.
//
// The repair's topic drops every spelling of the target stem the old name
// carried (topicForStem), so gtsm_rfc5082_linux_test.go becomes
// rfc5082_gtsm_linux_test.go rather than naming RFC 5082 twice. A topic that
// held nothing else names the bare rfcN_test.go whether or not that file
// exists: a taken target is left for `./le rfc rename propose` to report as a
// collision, so a hand-chosen topic replaces it rather than the doubled stem.
func judgeTestFileName(file namedTestFile, stems map[string]bool) (nameVerdict, bool) {
	dir, base := path.Split(file.Rel)
	nameStem := stemOfFileName(base, stems)
	topic := base
	if nameStem != "" {
		topic = strings.TrimPrefix(base, stemPrefix(nameStem))
	}
	if single, ok := singleStem(file.TagStems); ok && single != nameStem {
		var tb textbuf.Buffer
		target := dir + stemPrefix(single) + topicForStem(topic, single)
		tb.Str("every tag in it cites ").Str(single).Str(", so it is named for that RFC")
		return nameVerdict{Rel: file.Rel, Problem: tb.String(), Target: target, Stem: single}, true
	}
	if file.Marker.Present {
		return judgeNamingMarker(file, nameStem)
	}
	if nameStem == "" {
		return nameVerdict{}, false
	}
	if file.TagStems[nameStem] {
		return nameVerdict{}, false
	}
	var tb textbuf.Buffer
	tb.Str("it is named for ").Str(nameStem).Str(" and carries no tag for it: tag what it ").
		Str("covers, add `").Str(namingMarkerText).Str(" <reason>`, or name it after what ").
		Str("it covers")
	if topic == bareTopic {
		tb.Str(" (./le rfc rename from ").Str(file.Rel).Str(" to ").Str(dir).Str("<topic>_test.go)")
		return nameVerdict{Rel: file.Rel, Problem: tb.String()}, true
	}
	return nameVerdict{Rel: file.Rel, Problem: tb.String(), Offer: dir + topic}, true
}

// bareTopic is the topic of a file named for its stem alone, rfcN_test.go.
const bareTopic = "test.go"

// legacyRFCToken opens the legacy spellings rfc_<stem>, rfc_draft_<word> and
// rfc_<word>, the element before the stem words in a pre-convention name.
const legacyRFCToken = "rfc"

// topicForStem answers topic with every `_`-delimited spelling of stem removed.
// Four spellings count: the stem itself (rfc5082, sflow_v5), the legacy
// rfc_<stem> (rfc_sflow_v5), and for a draft the legacy rfc_draft_<word> and
// rfc_<word>, where word is one of the draft name's own words after "draft"
// (rfc_draft_abraitis for draft-abraitis-bgp-version-capability, rfc_mup for
// draft-ietf-bess-mup-safi). A lone draft word with no rfc_ before it is kept,
// so rfc_mup_safi gives the topic safi. A topic that holds nothing else
// answers bareTopic.
//
// The GOOS/GOARCH suffix is never touched: the trailing elements go/build
// reads as a constraint (buildSuffixToken) are kept even where they spell part
// of the stem, so the rename never changes which platforms compile the file.
// Every other element is kept as it was, so no underscore is added or lost.
func topicForStem(topic, stem string) string {
	body, isTest := strings.CutSuffix(topic, testFileSuffix)
	if !isTest {
		return topic
	}
	if body == "" {
		return topic
	}
	tokens := strings.Split(body, "_")
	free := tokens[:len(tokens)-buildSuffixLength(tokens)]
	words := strings.Split(strings.TrimSuffix(stemPrefix(stem), "_"), "_")
	kept := make([]string, 0, len(tokens))
	for index := 0; index < len(tokens); {
		width := stemSpellingAt(free, index, words)
		if width == 0 {
			kept = append(kept, tokens[index])
			index++
			continue
		}
		index += width
	}
	if len(kept) == 0 {
		return bareTopic
	}
	return strings.Join(kept, "_") + testFileSuffix
}

// stemSpellingAt answers how many tokens from index spell the stem whose words
// are given, and zero when none of topicForStem's four spellings opens there.
func stemSpellingAt(tokens []string, index int, words []string) int {
	if index >= len(tokens) {
		return 0
	}
	rest := tokens[index:]
	if len(rest) > len(words) && rest[0] == legacyRFCToken && slices.Equal(rest[1:1+len(words)], words) {
		return 1 + len(words)
	}
	if len(rest) >= len(words) && slices.Equal(rest[:len(words)], words) {
		return len(words)
	}
	if words[0] != "draft" {
		return 0
	}
	if len(rest) >= 3 && rest[0] == legacyRFCToken && rest[1] == "draft" && slices.Contains(words[1:], rest[2]) {
		return 3
	}
	if len(rest) >= 2 && rest[0] == legacyRFCToken && slices.Contains(words[1:], rest[1]) {
		return 2
	}
	return 0
}

// buildSuffixLength answers how many trailing tokens go/build reads as a
// GOOS/GOARCH constraint: zero, one (_linux, _amd64) or two (_linux_amd64).
func buildSuffixLength(tokens []string) int {
	length := 0
	for length < 2 && length < len(tokens) && buildSuffixToken(tokens[len(tokens)-1-length]) {
		length++
	}
	return length
}

// constraintFreePlatform names no GOOS and no GOARCH, so go/build compiles a
// file for it exactly when the file name carries no GOOS/GOARCH constraint.
var constraintFreePlatform = []goPlatform{{OS: "ze_no_os", Arch: "ze_no_arch"}}

// buildSuffixToken reports whether go/build reads token, as the last element
// of a file name, as a GOOS or GOARCH constraint. go/build's own known-name
// list answers through platformsBuilding, so no list is copied here. A judging
// error answers true, the direction that keeps the token where it stands.
func buildSuffixToken(token string) bool {
	built, err := platformsBuilding("a_"+token+testFileSuffix, constraintFreePlatform)
	if err != nil {
		return true
	}
	return !built[0]
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
		if statErr != nil {
			continue
		}
		if !info.IsDir() {
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
			if !held {
				return nil
			}
			if carrier.Kind != kindUnit {
				return nil
			}
			src, readErr := readFile(full, rel)
			if readErr != nil {
				return readErr
			}
			file := namedTestFile{Rel: rel, TagStems: stemsByFile[rel], Marker: readNamingMarker(src)}
			if verdict, refused := judgeTestFileName(file, stems); refused {
				verdict.Source = file
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
// tags disagree. A finding with a repair, or with the rename part (a) offers,
// names the exact `./le rfc rename` command that makes it, unless the rename
// would refuse that target, in which case the finding says a topic must be
// chosen by hand (repairBlocked).
func checkTestFileNames(tree string, carriers []Carrier, tags []Tag, requirements []Requirement,
	stems map[string]bool) ([]string, error) {
	verdicts, err := testFileNameVerdicts(tree, carriers, tags, requirements, stems)
	if err != nil {
		return nil, err
	}
	shared := map[string]int{}
	for _, verdict := range verdicts {
		if rename := verdictRename(verdict); rename != "" {
			shared[rename]++
		}
	}
	// Read once, and only when a finding names a target, because listing the
	// platforms runs the toolchain. TagStems stays unset: a verdict carries its
	// own source's tags.
	judged := renameJudgement{Stems: stems, Carriers: carriers}
	out := make([]string, 0, len(verdicts))
	for _, verdict := range verdicts {
		var tb textbuf.Buffer
		tb.Str(verdict.Rel).Str(": test file name: ").Str(verdict.Problem)
		rename := verdictRename(verdict)
		if rename == "" {
			out = append(out, tb.String())
			continue
		}
		if judged.Platforms == nil {
			judged.Platforms, err = goPlatforms()
			if err != nil {
				return nil, err
			}
		}
		blocked, err := repairBlocked(tree, verdict, rename, shared[rename], judged)
		if err != nil {
			return nil, err
		}
		if blocked != "" {
			// A part (a) file is renamed off its stem, so its hand-chosen name
			// carries no prefix; Stem is empty for it.
			dir, _ := path.Split(verdict.Rel)
			prefix := ""
			if verdict.Stem != "" {
				prefix = stemPrefix(verdict.Stem)
			}
			tb.Str(": ").Str(blocked).Str(", so choose its topic by hand: ./le rfc rename from ").
				Str(verdict.Rel).Str(" to ").Str(dir).Str(prefix).Str("<topic>_test.go")
			out = append(out, tb.String())
			continue
		}
		tb.Str(": ./le rfc rename from ").Str(verdict.Rel).Str(" to ").Str(rename)
		out = append(out, tb.String())
	}
	return out, nil
}

// verdictRename answers the rename a finding names: the repair when the file
// has one, else the rename part (a) offers, and empty for neither.
func verdictRename(verdict nameVerdict) string {
	if verdict.Target != "" {
		return verdict.Target
	}
	return verdict.Offer
}

// repairBlocked answers why `./le rfc rename` would refuse the rename a finding
// names, and empty when it would take it. The refusals come from pairRefusals,
// the predicate the rename applies to a pair; a target two files would share is
// the one refusal a single pair cannot see, so claimants is how many verdicts
// name the target.
//
// A finding judges names, never the working tree, so the source's state there
// (pairRefusal.SourceState) is set aside: `./le rfc check` reports a name and
// tag disagreement that holds whether or not another session is editing the
// file, and the rename refuses an edited source with its own reason. A command
// a finding names therefore draws none of the refusals one pair draws from its
// paths, its source's state and its target once its source matches HEAD.
// proposeRenames writes pairs to run now, so it keeps the source-state refusal.
// A refusal judged on an evidence record (an escaped path spelling, evidence
// that is not one JSON value) is judged by the batch's rewrite alone, so neither
// a finding nor proposeRenames sees it, and the rename refuses the batch whole.
func repairBlocked(tree string, verdict nameVerdict, target string, claimants int,
	judged renameJudgement) (string, error) {
	pair := renamePair{Source: verdict.Rel, Target: target}
	refusal, err := pairRefusals(tree, pair, verdict.Source.TagStems, judged)
	if err != nil {
		return "", err
	}
	refusal.SourceState = ""
	return pairBlocked(refusal, target, claimants), nil
}

// pairBlocked answers the first reason a pair's refusal, and the count of
// verdicts naming its target, give for leaving the rename out, and empty when
// neither refuses it.
func pairBlocked(refusal pairRefusal, target string, claimants int) string {
	if refusal.Unclean != "" {
		return refusal.Unclean + " is not a clean path inside the checkout"
	}
	if len(refusal.NotTestFile) > 0 {
		return refusal.NotTestFile[0] + " is not a _test.go file"
	}
	if refusal.OtherDirectory {
		return "the name its repair takes, " + target + ", leaves its source's directory"
	}
	if refusal.SourceState != "" {
		return "its source " + refusal.SourceState
	}
	if refusal.Unreadable {
		return "its source cannot be read"
	}
	if refusal.Target.Taken {
		return "the name its repair takes, " + target + ", is taken"
	}
	if claimants > 1 {
		return "another file's repair takes the same name, " + target
	}
	if refusal.Target.MovesPlatforms {
		return "the name its repair takes, " + target + ", changes which platforms build it"
	}
	if refusal.Target.Misnamed {
		return "the name its repair takes, " + target + ", fails the test file naming rule"
	}
	return ""
}
