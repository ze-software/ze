package rfc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// namingStems is the summary stem set the naming tests judge against: two RFCs
// whose prefixes nest textually (rfc79 and rfc792), a non-RFC stem, and a draft.
func namingStems() map[string]bool {
	return map[string]bool{"rfc79": true, "rfc792": true, "rfc7606": true, "rfc7311": true,
		"sflow-v5": true, "draft-ietf-sidrops-8210bis": true}
}

// namingRequirements maps one requirement id to each fixture stem, the lookup a
// tag is resolved through.
func namingRequirements() []Requirement {
	return []Requirement{
		{RFC: "rfc7606", RID: "RFC7606-3-1"},
		{RFC: "rfc7311", RID: "RFC7311-4-1"},
		{RFC: "rfc792", RID: "RFC792-2-1"},
		{RFC: "sflow-v5", RID: "SFLOW-V5-1-1"},
	}
}

// namingCarriers is the one unit-test carrier row the production table holds,
// plus an interop row, so a test can show an interop carrier is not judged.
func namingCarriers() []Carrier {
	return []Carrier{
		{Name: "interop-x", Kind: kindInterop, Prefix: "internal/le/interoplab/", Suffix: ".go", Reader: "go"},
		{Name: kindUnit, Kind: kindUnit, Suffix: "_test.go", Reader: "go"},
		{Name: "ci", Kind: kindFunctional, Prefix: "test/", Suffix: ".ci", Reader: "ci"},
	}
}

// namingTree lays files under a temporary root and answers it.
func namingTree(t *testing.T, files map[string]string) string {
	t.Helper()

	root := t.TempDir()
	for rel, body := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("fixture directory: %v", err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("fixture file: %v", err)
		}
	}
	return root
}

// judgeNames runs checkTestFileNames over files with tags placed by path.
func judgeNames(t *testing.T, files map[string]string, tags []Tag) []string {
	t.Helper()

	findings, err := checkTestFileNames(namingTree(t, files), namingCarriers(), tags,
		namingRequirements(), namingStems())
	if err != nil {
		t.Fatalf("checkTestFileNames: %v", err)
	}
	return findings
}

const namingBody = "package sample\n"

// VALIDATES: AC-16 -- an RFC stem's prefix is the stem and one underscore, with
// no zero padding.
func TestStemPrefixOfRFCHasNoPadding(t *testing.T) {
	if got := stemPrefix("rfc792"); got != "rfc792_" {
		t.Errorf("stemPrefix(rfc792) = %q, want rfc792_", got)
	}
}

// VALIDATES: AC-16 -- a non-RFC stem's hyphens become underscores.
func TestStemPrefixOfDraftTurnsHyphensToUnderscores(t *testing.T) {
	for stem, want := range map[string]string{
		"sflow-v5":                   "sflow_v5_",
		"draft-ietf-sidrops-8210bis": "draft_ietf_sidrops_8210bis_",
	} {
		if got := stemPrefix(stem); got != want {
			t.Errorf("stemPrefix(%s) = %q, want %q", stem, got, want)
		}
	}
}

// VALIDATES: AC-16 and R-6 -- a file name maps back to the LONGEST known prefix,
// so rfc792_x is not read as rfc79, and an rfcN prefix with no summary is still
// read as that stem.
func TestStemOfFileNameTakesTheLongestKnownPrefix(t *testing.T) {
	for base, want := range map[string]string{
		"rfc792_echo_test.go":                    "rfc792",
		"rfc79_echo_test.go":                     "rfc79",
		"sflow_v5_sample_test.go":                "sflow-v5",
		"draft_ietf_sidrops_8210bis_pdu_test.go": "draft-ietf-sidrops-8210bis",
		"rfc4444_unsummarised_test.go":           "rfc4444",
		"echo_rfc792_test.go":                    "",
		"rfc792x_test.go":                        "",
	} {
		if got := stemOfFileName(base, namingStems()); got != want {
			t.Errorf("stemOfFileName(%s) = %q, want %q", base, got, want)
		}
	}
}

// VALIDATES: AC-16 -- rfcN_test.go with no topic is named for rfcN.
func TestStemOfFileNameAcceptsBareStemName(t *testing.T) {
	if got := stemOfFileName("rfc792_test.go", namingStems()); got != "rfc792" {
		t.Errorf("stemOfFileName(rfc792_test.go) = %q, want rfc792", got)
	}
	if got := stemOfFileName("sflow_v5_test.go", namingStems()); got != "sflow-v5" {
		t.Errorf("stemOfFileName(sflow_v5_test.go) = %q, want sflow-v5", got)
	}
}

// VALIDATES: A-5 over this checkout's rfc/short/ -- no stem holds an underscore,
// every prefix is distinct, and every prefix maps back to its own stem.
func TestStemPrefixesAreInjectiveOverTheCorpus(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve checkout: %v", err)
	}
	stems, err := summaryStems(root)
	if err != nil {
		t.Fatalf("summary stems: %v", err)
	}
	if len(stems) == 0 {
		t.Fatal("the checkout holds no summary, so the test proves nothing")
	}
	seen := map[string]string{}
	for stem := range stems {
		if strings.Contains(stem, "_") {
			t.Errorf("stem %s holds an underscore, so its prefix is ambiguous", stem)
		}
		prefix := stemPrefix(stem)
		if other, taken := seen[prefix]; taken {
			t.Errorf("stems %s and %s share the prefix %s", stem, other, prefix)
		}
		seen[prefix] = stem
		if got := stemOfFileName(prefix+"topic_test.go", stems); got != stem {
			t.Errorf("%stopic_test.go maps to %q, want %s", prefix, got, stem)
		}
	}
}

// VALIDATES: the marker is not a tag, so writing it adds no evidence.
func TestRFCNamingMarkerIsNotATag(t *testing.T) {
	line := namingMarkerText + " a fixture reason"
	if goTagRE.MatchString(line) {
		t.Errorf("the marker %q matches goTagRE", line)
	}
}

// VALIDATES: AC-11 -- a file named for S with no tag for S and no marker is
// reported, naming the file, S, the marker text and a rename command.
func TestCheckTestFileNamesRefusesUntaggedStemNamedFile(t *testing.T) {
	rel := "internal/sample/rfc7606_split_test.go"
	findings := judgeNames(t, map[string]string{rel: namingBody}, nil)
	if len(findings) != 1 {
		t.Fatalf("want one finding, got %d: %v", len(findings), findings)
	}
	for _, want := range []string{rel, "rfc7606", namingMarkerText,
		"./le rfc rename from " + rel + " to internal/sample/split_test.go"} {
		if !strings.Contains(findings[0], want) {
			t.Errorf("the finding omits %q: %s", want, findings[0])
		}
	}
}

// VALIDATES: AC-12 -- the marker with a reason clears the finding.
func TestCheckTestFileNamesAcceptsMarkerWithReason(t *testing.T) {
	body := namingBody + namingMarkerText + " exercises the split helper RFC 7606 needs\n"
	if findings := judgeNames(t, map[string]string{"internal/sample/rfc7606_split_test.go": body},
		nil); len(findings) != 0 {
		t.Errorf("a marker with a reason was refused: %v", findings)
	}
}

// VALIDATES: AC-13, three cases -- a marker with no reason, a marker in a file
// that carries a tag for its name stem, and a marker in a file named for no stem.
func TestCheckTestFileNamesRefusesBadMarker(t *testing.T) {
	cases := []struct {
		name, rel, body, want string
		tags                  []Tag
	}{
		{"empty reason", "internal/a/rfc7606_x_test.go", namingBody + namingMarkerText + "  \n",
			"no reason", nil},
		{"tagged for its stem", "internal/b/rfc7606_x_test.go",
			namingBody + namingMarkerText + " stale reason\n", "carries a tag for rfc7606",
			[]Tag{{RID: "RFC7606-3-1", Polarity: PolarityPositive, File: "internal/b/rfc7606_x_test.go"}}},
		{"named for no stem", "internal/c/split_test.go",
			namingBody + namingMarkerText + " nothing to say\n", "not named for any RFC", nil},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			findings := judgeNames(t, map[string]string{one.rel: one.body}, one.tags)
			if len(findings) != 1 {
				t.Fatalf("want one finding, got %d: %v", len(findings), findings)
			}
			if !strings.Contains(findings[0], one.want) || !strings.Contains(findings[0], one.rel) {
				t.Errorf("the finding does not say %q about %s: %s", one.want, one.rel, findings[0])
			}
		})
	}
}

// VALIDATES: AC-14 -- a file whose tags all cite one stem and that is not named
// for it is reported with the exact rename command.
func TestCheckTestFileNamesRefusesSingleStemFileNamedOtherwise(t *testing.T) {
	cases := map[string]string{
		"internal/sample/aigp_test.go":         "internal/sample/rfc7311_aigp_test.go",
		"internal/sample/rfc7606_aigp_test.go": "internal/sample/rfc7311_aigp_test.go",
		"internal/sample/rfc7606_test.go":      "internal/sample/rfc7311_test.go",
		"internal/sample/aigp_linux_test.go":   "internal/sample/rfc7311_aigp_linux_test.go",
	}
	for rel, target := range cases {
		tags := []Tag{{RID: "RFC7311-4-1", Polarity: PolarityPositive, File: rel}}
		findings := judgeNames(t, map[string]string{rel: namingBody}, tags)
		if len(findings) != 1 {
			t.Fatalf("%s: want one finding, got %d: %v", rel, len(findings), findings)
		}
		want := "./le rfc rename from " + rel + " to " + target
		if !strings.Contains(findings[0], want) {
			t.Errorf("%s: the finding omits %q: %s", rel, want, findings[0])
		}
	}
}

// VALIDATES: AC-15 -- a multi-stem file has no part (b) finding whatever its
// name, and a part (a) finding only when its name stem is not among its tags.
func TestCheckTestFileNamesJudgesMultiStemFilesByPartAOnly(t *testing.T) {
	tagsFor := func(rel string) []Tag {
		return []Tag{{RID: "RFC7311-4-1", Polarity: PolarityPositive, File: rel},
			{RID: "RFC7606-3-1", Polarity: PolarityNegative, File: rel}}
	}
	for _, rel := range []string{"internal/a/anything_test.go", "internal/b/rfc7606_mixed_test.go",
		"internal/c/rfc7311_mixed_test.go"} {
		if findings := judgeNames(t, map[string]string{rel: namingBody}, tagsFor(rel)); len(findings) != 0 {
			t.Errorf("%s: a multi-stem file was refused: %v", rel, findings)
		}
	}
	rel := "internal/d/rfc792_mixed_test.go"
	findings := judgeNames(t, map[string]string{rel: namingBody}, tagsFor(rel))
	if len(findings) != 1 || !strings.Contains(findings[0], "rfc792") {
		t.Errorf("a multi-stem file named for a stem it does not tag answered %v", findings)
	}
}

// VALIDATES: A-9 -- a gap tag cites its stem for naming just as a proof tag does.
func TestCheckTestFileNamesCountsGapTags(t *testing.T) {
	rel := "internal/sample/rfc7606_gap_test.go"
	tags := []Tag{{RID: "RFC7606-3-1", Gap: true, File: rel}}
	if findings := judgeNames(t, map[string]string{rel: namingBody}, tags); len(findings) != 0 {
		t.Errorf("a gap-tagged file named for its stem was refused: %v", findings)
	}
	misnamed := "internal/sample/gap_test.go"
	tags = []Tag{{RID: "RFC7606-3-1", Gap: true, File: misnamed}}
	findings := judgeNames(t, map[string]string{misnamed: namingBody}, tags)
	if len(findings) != 1 || !strings.Contains(findings[0], "internal/sample/rfc7606_gap_test.go") {
		t.Errorf("a gap-only file not named for its stem answered %v", findings)
	}
}

// VALIDATES: AC-17 -- files under internal/le/, test/draft/, testdata/ and
// vendor/, and any carrier that is not a unit _test.go, are not judged.
func TestCheckTestFileNamesSkipsOutOfScopeFiles(t *testing.T) {
	files := map[string]string{
		"internal/le/sample/rfc7606_x_test.go":         namingBody,
		"internal/le/interoplab/rfc7606_x_test.go":     namingBody,
		"test/draft/rfc7606_x_test.go":                 namingBody,
		"internal/sample/testdata/rfc7606_x_test.go":   namingBody,
		"vendor/example.com/x/rfc7606_x_test.go":       namingBody,
		"test/plugin/rfc7606_widget.ci":                namingBody,
		"internal/sample/rfc7606_helper.go":            namingBody,
		"internal/le/interoplab/scenario/aigp_test.go": namingBody,
	}
	tags := []Tag{{RID: "RFC7311-4-1", Polarity: PolarityPositive,
		File: "internal/le/interoplab/scenario/aigp_test.go"}}
	if findings := judgeNames(t, files, tags); len(findings) != 0 {
		t.Errorf("out-of-scope files were judged: %v", findings)
	}
}

// innerStemStems is the stem set of the inner-stem cases: the stems Phase 2's
// proposal named twice, plus a draft whose words include a GOOS name.
func innerStemStems() map[string]bool {
	return map[string]bool{"rfc5082": true, "rfc7311": true, "rfc7999": true, "sflow-v5": true,
		"draft-abraitis-bgp-version-capability": true, "draft-ietf-idr-linklocal-capability": true,
		"draft-ze-linux-thing": true, "draft-ietf-bess-mup-safi": true, "rfc4760": true}
}

// VALIDATES: AC-14 -- the repair drops every spelling of the target stem the
// old name carried, in the four spellings the tree uses (rfcN, rfc_<stem>,
// rfc_draft_<word>, rfc_<word>), keeps the GOOS/GOARCH suffix, and names the
// bare stem file whether or not it exists (propose reports a taken one). The
// cases are the Phase 2 proposals that named their RFC twice.
// PREVENTS: gtsm_rfc5082_linux_test.go being renamed rfc5082_gtsm_rfc5082_linux_test.go.
func TestJudgeTestFileNameDropsTheInnerStem(t *testing.T) {
	cases := []struct {
		rel, stem, target string
	}{
		{"c/gtsm/gtsm_rfc5082_linux_test.go", "rfc5082", "c/gtsm/rfc5082_gtsm_linux_test.go"},
		{"c/attr/aigp_rfc7311_test.go", "rfc7311", "c/attr/rfc7311_aigp_test.go"},
		{"c/attr/aigp_test.go", "rfc7311", "c/attr/rfc7311_aigp_test.go"},
		{"c/fc/blackhole_rfc7999_test.go", "rfc7999", "c/fc/rfc7999_blackhole_test.go"},
		{"c/fc/blackhole_test.go", "rfc7999", "c/fc/rfc7999_blackhole_test.go"},
		{"c/rib/rib_blackhole_rfc7999_cover_test.go", "rfc7999", "c/rib/rfc7999_rib_blackhole_cover_test.go"},
		{"c/msg/rfc_draft_abraitis_softver_test.go", "draft-abraitis-bgp-version-capability",
			"c/msg/draft_abraitis_bgp_version_capability_softver_test.go"},
		{"c/sv/rfc_draft_abraitis_test.go", "draft-abraitis-bgp-version-capability",
			"c/sv/draft_abraitis_bgp_version_capability_test.go"},
		{"c/re/rfc_draft_linklocal_send_test.go", "draft-ietf-idr-linklocal-capability",
			"c/re/draft_ietf_idr_linklocal_capability_send_test.go"},
		{"c/sf/config_rfc_sflow_v5_test.go", "sflow-v5", "c/sf/sflow_v5_config_test.go"},
		{"c/sf/counters_sflow_v5_test.go", "sflow-v5", "c/sf/sflow_v5_counters_test.go"},
		{"c/x/foo_rfc5082_linux_amd64_test.go", "rfc5082", "c/x/rfc5082_foo_linux_amd64_test.go"},
		// The draft's word "linux" is also the GOOS suffix, so it stays.
		{"c/x/foo_rfc_draft_linux_test.go", "draft-ze-linux-thing",
			"c/x/draft_ze_linux_thing_foo_rfc_draft_linux_test.go"},
		{"c/x/plain_test.go", "rfc5082", "c/x/rfc5082_plain_test.go"},
		// The stem word "linux" sits at n-2, before the arch token, so both
		// stay: the suffix go/build reads is two elements long.
		{"c/x/foo_rfc_draft_linux_amd64_test.go", "draft-ze-linux-thing",
			"c/x/draft_ze_linux_thing_foo_rfc_draft_linux_amd64_test.go"},
		// The legacy draft abbreviation rfc_<word>: rfc_mup is a spelling of
		// draft-ietf-bess-mup-safi, and the lone word safi after it is kept.
		{"c/mup/rfc_mup_ingress_test.go", "draft-ietf-bess-mup-safi",
			"c/mup/draft_ietf_bess_mup_safi_ingress_test.go"},
		{"c/mup/rfc_mup_safi_test.go", "draft-ietf-bess-mup-safi",
			"c/mup/draft_ietf_bess_mup_safi_safi_test.go"},
		// mup is no word of rfc4760, so rfc_mup is topic here and the file
		// takes a hand-chosen topic in Phase 2.
		{"c/mup/rfc_mup_session_test.go", "rfc4760", "c/mup/rfc4760_rfc_mup_session_test.go"},
		// rfc_<word> is a spelling of a draft stem only: v5 is a word of
		// sflow-v5, which is no draft, so rfc_v5 stays as topic.
		{"c/sf/foo_rfc_v5_test.go", "sflow-v5", "c/sf/sflow_v5_foo_rfc_v5_test.go"},
		// A trailing rfc with no word after it is no spelling of a draft stem.
		{"c/x/foo_rfc_test.go", "draft-ze-linux-thing", "c/x/draft_ze_linux_thing_foo_rfc_test.go"},
	}
	for _, tc := range cases {
		file := namedTestFile{Rel: tc.rel, TagStems: map[string]bool{tc.stem: true}}
		verdict, refused := judgeTestFileName(file, innerStemStems())
		if !refused {
			t.Errorf("%s: not refused", tc.rel)
			continue
		}
		if verdict.Target != tc.target {
			t.Errorf("%s: target %s, want %s", tc.rel, verdict.Target, tc.target)
		}
	}
}

// VALIDATES: the GOOS/GOARCH reading comes from go/build: an OS, an arch and
// an implied OS are constraints, and a topic word is not.
func TestBuildSuffixTokenFollowsGoBuild(t *testing.T) {
	for token, want := range map[string]bool{"linux": true, "amd64": true, "illumos": true,
		"gtsm": false, "rfc5082": false, "unix": false} {
		if got := buildSuffixToken(token); got != want {
			t.Errorf("buildSuffixToken(%q) = %v, want %v", token, got, want)
		}
	}
}

// VALIDATES: F-4 and R-4 -- a finding whose repair `./le rfc rename` would
// refuse says a topic must be chosen by hand, and names no command that fails:
// a target that exists, a target two files' repairs share, and a target whose
// build-constraint suffix compiles the file on other platforms. Both parts of
// the rule are judged: the repair of a file whose tags cite one other stem
// (part b), and the rename an untagged stem-named file is offered (part a),
// where rfc5881_linux_test.go would become linux_test.go, which every platform
// builds.
// PREVENTS: the armed check suggesting a rename the action then refuses, which
// sends the reader round a loop with no way out.
func TestCheckTestFileNamesNeverSuggestsARefusedTarget(t *testing.T) {
	tagged := func(rels ...string) []Tag {
		var tags []Tag
		for _, rel := range rels {
			tags = append(tags, Tag{RID: "RFC7311-4-1", Polarity: PolarityPositive, File: rel})
		}
		return tags
	}
	for _, one := range []struct {
		name   string
		files  []string
		judged string
		target string
		reason string
		hand   string
		// untagged leaves every file without a tag, so part (a) judges it.
		untagged bool
	}{
		{"taken", []string{"internal/sample/aigp_test.go", "internal/sample/rfc7311_aigp_test.go"},
			"internal/sample/aigp_test.go", "internal/sample/rfc7311_aigp_test.go",
			"the name its repair takes, internal/sample/rfc7311_aigp_test.go, is taken",
			"internal/sample/rfc7311_<topic>_test.go", false},
		{"shared", []string{"internal/sample/aigp_test.go", "internal/sample/rfc7606_aigp_test.go"},
			"internal/sample/aigp_test.go", "internal/sample/rfc7311_aigp_test.go",
			"another file's repair takes the same name, internal/sample/rfc7311_aigp_test.go",
			"internal/sample/rfc7311_<topic>_test.go", false},
		{"build suffix", []string{"internal/sample/linux_test.go"},
			"internal/sample/linux_test.go", "internal/sample/rfc7311_linux_test.go",
			"the name its repair takes, internal/sample/rfc7311_linux_test.go, changes which platforms build it",
			"internal/sample/rfc7311_<topic>_test.go", false},
		{"untagged taken", []string{"internal/sample/rfc5881_echo_test.go", "internal/sample/echo_test.go"},
			"internal/sample/rfc5881_echo_test.go", "internal/sample/echo_test.go",
			"the name its repair takes, internal/sample/echo_test.go, is taken",
			"internal/sample/<topic>_test.go", true},
		{"untagged shared", []string{"internal/sample/rfc5881_echo_test.go", "internal/sample/rfc5882_echo_test.go"},
			"internal/sample/rfc5881_echo_test.go", "internal/sample/echo_test.go",
			"another file's repair takes the same name, internal/sample/echo_test.go",
			"internal/sample/<topic>_test.go", true},
		{"untagged build suffix", []string{"internal/sample/rfc5881_linux_test.go"},
			"internal/sample/rfc5881_linux_test.go", "internal/sample/linux_test.go",
			"the name its repair takes, internal/sample/linux_test.go, changes which platforms build it",
			"internal/sample/<topic>_test.go", true},
	} {
		t.Run(one.name, func(t *testing.T) {
			files := map[string]string{}
			for _, rel := range one.files {
				files[rel] = namingBody
			}
			tags := tagged(one.files...)
			if one.untagged {
				tags = nil
			}
			var finding string
			for _, line := range judgeNames(t, files, tags) {
				if strings.HasPrefix(line, one.judged+":") {
					finding = line
				}
			}
			if finding == "" {
				t.Fatalf("no finding for %s", one.judged)
			}
			if !strings.Contains(finding, one.reason) {
				t.Errorf("the finding does not say %q: %s", one.reason, finding)
			}
			hand := "choose its topic by hand: ./le rfc rename from " + one.judged + " to " + one.hand
			if !strings.Contains(finding, hand) {
				t.Errorf("the finding does not ask for a hand-chosen topic (%q): %s", hand, finding)
			}
			if strings.Contains(finding, "rename from "+one.judged+" to "+one.target) {
				t.Errorf("the finding suggests the target the rename refuses: %s", finding)
			}
		})
	}
}
