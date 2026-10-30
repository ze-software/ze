package rfc

import (
	"strings"
	"testing"
)

// backlogFile is an evidence backlog holding one count per kind. Every case
// varies only the {not-applicable} count, the kind its fixture row carries;
// the other two kinds are held at zero.
func backlogFile(notApplicable string) string {
	return "# fixture\nfeature-declined 0\nlower-layer 0\nnot-applicable " + notApplicable + "\n"
}

// VALIDATES: the exclusion evidence ratchet -- an untouched row lacking
// evidence is carried by the committed count, a row the commit added or
// changed is refused by id, a count that fell must be lowered in the file, a
// count that rose is refused, a kind at zero is refused row by row, and a
// missing or partial count file is refused.
// METHOD: one legacy {not-applicable} row of the fixture RFC, judged against
// HEAD^ fingerprints and count files varied per case.
func TestExclusionEvidenceRatchet(t *testing.T) {
	legacy := Requirement{RFC: selftestStem, RID: "RFC9999-2-1", Level: levelMust, Section: "2",
		Source: selftestSummaryRel, Line: 7, Annotation: &Annotation{Kind: AnnotationNotApplicable, Reason: "ze sends no widget"}}
	second := legacy
	second.RID = "RFC9999-2-2"
	unchanged := map[string]string{legacy.RID: evidenceFingerprint(&legacy), second.RID: evidenceFingerprint(&second)}
	changed := map[string]string{legacy.RID: "2\x00not-applicable\x00an older reason", second.RID: evidenceFingerprint(&second)}
	cases := []struct {
		name     string
		backlog  string
		rows     []Requirement
		baseline map[string]string
		known    bool
		want     []string
	}{
		{"an untouched row the count carries", backlogFile("1"), []Requirement{legacy}, unchanged, true, nil},
		{"a new row", backlogFile("1"), []Requirement{legacy}, map[string]string{}, true,
			[]string{"RFC9999-2-1 is annotated {not-applicable} and lacks the evidence"}},
		{"a changed row", backlogFile("1"), []Requirement{legacy}, changed, true,
			[]string{"RFC9999-2-1 is annotated {not-applicable} and lacks the evidence"}},
		{"no HEAD^ to compare", backlogFile("1"), []Requirement{legacy}, nil, false, nil},
		{"a count that fell", backlogFile("2"), []Requirement{legacy}, unchanged, true,
			[]string{"set the line to `not-applicable 1`"}},
		{"a count that rose", backlogFile("1"), []Requirement{legacy, second}, unchanged, true,
			[]string{"2 {not-applicable} rows lack their evidence, and the committed count is 1. A row whose line this commit left alone lost its evidence"}},
		{"a kind at zero", backlogFile("0"), []Requirement{legacy}, unchanged, true,
			[]string{"RFC9999-2-1 is annotated {not-applicable} and lacks the evidence"}},
		{"no count file", "", []Requirement{legacy}, unchanged, true, []string{"cannot read the committed evidence counts"}},
		{"a kind with no line", "feature-declined 0\nlower-layer 0\n", []Requirement{legacy}, unchanged, true,
			[]string{"no line counts {not-applicable}"}},
		{"an unknown kind", backlogFile("1") + "gap 3\n", []Requirement{legacy}, unchanged, true,
			[]string{"'gap' is not a kind this ratchet counts"}},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			tree := t.TempDir()
			files := map[string]string{"rfc/full/rfc9999.txt": quoteFixtureSource}
			if one.backlog != "" {
				files[evidenceBacklogRel] = one.backlog
			}
			writeFixtureFiles(t, tree, files)
			errs := checkExclusionEvidence(tree, one.rows, one.baseline, nil, one.known)
			if len(errs) != len(one.want) {
				t.Fatalf("got %d refusals, want %d: %q", len(errs), len(one.want), errs)
			}
			for i, want := range one.want {
				if !strings.Contains(errs[i], want) {
					t.Errorf("refusal %q does not say %q", errs[i], want)
				}
			}
		})
	}
}

// VALIDATES: a {lower-layer} layer names a component path a reader can open:
// a vendored module this checkout carries, a path under a Linux source tree's
// top level, or a standard library package.
// METHOD: the parser reads the path out of the layer, and layerPathGap judges
// each form against a fixture checkout holding one vendored module.
func TestLowerLayerNamesItsLayerPath(t *testing.T) {
	parsed, err := parseAnnotation("lower-layer: Linux IPv6 neighbor discovery, linux:net/ipv6/ndisc.c; "+
		"internal/x/nd.go::install installs the address, and the kernel answers", "fixture")
	if err != nil {
		t.Fatalf("a lower-layer payload naming its layer path did not parse: %v", err)
	}
	if parsed.LayerPath != "linux:net/ipv6/ndisc.c" {
		t.Errorf("LayerPath is %q, want linux:net/ipv6/ndisc.c", parsed.LayerPath)
	}
	tree := t.TempDir()
	writeFixtureFiles(t, tree, map[string]string{"vendor/github.com/insomniacslk/dhcp/dhcpv6/doc.go": "package dhcpv6\n"})
	cases := []struct{ path, refused string }{
		{"linux:net/ipv6/ndisc.c", ""},
		{"go:crypto/x509", ""},
		{"vendor/github.com/insomniacslk/dhcp/dhcpv6", ""},
		{"", "names no component path"},
		{"vendor/github.com/gone/module", "cannot be opened in this checkout"},
		{"linux:ipv6/ndisc.c", "opens under no top-level directory"},
		{"linux:net/../../etc", "climbs out of its source tree"},
		{"go:github.com/x/y", "is not a standard library package"},
	}
	for _, one := range cases {
		missing, lacks := layerPathGap(tree, one.path)
		if one.refused == "" {
			if lacks {
				t.Errorf("%q was refused: %s", one.path, missing)
			}
			continue
		}
		if !lacks || !strings.Contains(missing, one.refused) {
			t.Errorf("%q answered (%q, %v), want a refusal saying %q", one.path, missing, lacks, one.refused)
		}
	}
}
