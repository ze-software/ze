// Design: docs/contributing/rfc-conformance-gates.md -- one-off RFC accounting.
package rfc

import (
	"strings"
	"testing"
)

// TestNotEnrolledWriterReaderAgreement follows every declared disposition through
// summary parsing, collection, the remainder writer and public-page publication.
// A document implemented elsewhere must publish that disposition, not `enrolled`.
func TestNotEnrolledWriterReaderAgreement(t *testing.T) {
	cases := []struct{ stem, kind, implementation string }{
		{"rfc9001", dispositionBacklog, implementationZe},
		{"rfc9002", dispositionBlocked, implementationZe},
		{"rfc9003", dispositionNonNormative, implementationZe},
		{"iso-iec-10589", dispositionSourceRestricted, implementationZe},
		{"rfc9005", dispositionOutOfScope, implementationZe},
		{"rfc9006", enrolmentEnrolled, implementationThirdParty},
		{"rfc9007", enrolmentEnrolled, implementationFoundation},
	}
	files := map[string]string{
		FixtureAggregateRel: FixtureAggregateProfile,
		selftestWorkflowRel: selftestWorkflow,
	}
	for _, tc := range cases {
		body := strings.Replace(selftestMeta, "| Enrolment | enrolled |", "| Enrolment | "+tc.kind+" |", 1)
		body = strings.Replace(body, "| Implementation | ze |", "| Implementation | "+tc.implementation+" |", 1)
		body = strings.Replace(body, "the fixture RFC, gated so the selftest has a population", "ISO copyright; extraction deferred by owner decision 2026-11-03; Informational document", 1)
		if tc.implementation == implementationThirdParty {
			body = strings.Replace(body, "the fixture's own Go answers it", "Linux XFRM builds every fixture ESP packet", 1)
		}
		if tc.implementation == implementationFoundation {
			body = strings.Replace(body, "the fixture's own Go answers it", "The fixture defines a registry and obliges no implementer", 1)
		}
		if tc.stem == "iso-iec-10589" {
			body += "| Support name | ISO/IEC 10589 |\n"
		}
		files[summaryRel+"/"+tc.stem+".md"] = "# Disposition fixture\n\n" + body
	}
	root := t.TempDir()
	writeFixtureFiles(t, root, files)
	collected, err := Collect(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(collected.MetaProblems) != 0 {
		t.Fatalf("metadata: %v", collected.MetaProblems)
	}
	in, err := NewRenderInput(root, collected, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	published, err := ledgerFilesFrom(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(collected.Enrolled) != 0 {
		t.Fatalf("not-enrolled documents entered counted set: %v", collected.Enrolled)
	}
	seen := map[string]bool{}
	for line := range strings.SplitSeq(published[notEnrolledRel], "\n") {
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		cells := strings.Split(line, "\t")
		if len(cells) != 3 {
			t.Fatalf("malformed remainder row %q", line)
		}
		want, known := in.Dispositions[cells[0]]
		if !known {
			t.Fatalf("writer emitted an unknown document: %q", line)
		}
		if cells[1] != want.Kind {
			t.Errorf("writer %q disagrees with reader %+v", line, want)
		}
		if cells[2] != want.Reason {
			t.Errorf("writer %q disagrees with reader %+v", line, want)
		}
		seen[cells[0]] = true
	}
	for _, tc := range cases {
		if !seen[tc.stem] {
			t.Errorf("writer omitted %s", tc.stem)
		}
	}
	page, err := StatusPage(root)
	if err != nil {
		t.Fatal(err)
	}
	if page != published[statusRel] {
		t.Fatal("publication differs from ledger's public page")
	}
	if !strings.Contains(page, "ISO/IEC 10589") {
		t.Fatal("publication omitted the source-restricted ISO document")
	}
}
