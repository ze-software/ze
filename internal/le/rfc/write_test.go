package rfc

// VALIDATES: `./le rfc index-update` writes rfc/graph.tsv from the RFC texts (AC-1 wiring).
// PREVENTS: a graph the check parses but the generated file never shows.

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestIndexUpdateWritesGraph runs the production writer over the check fixture
// plus two texts that relate to its RFC, and reads rfc/graph.tsv back: the
// header line, the update edge and the builds-on edge are rows, the RFC 2119
// citation is not, the report names the file, and a re-run writes the same
// bytes (the file is a pure function of the texts).
func TestIndexUpdateWritesGraph(t *testing.T) {
	root := checkFixtureTree(t, map[string]string{
		"rfc/full/rfc10000.txt": "Network Working Group\nUpdates: 9999                                    A. Author\n\n" +
			"1.  Introduction\n\n   Widgets.\n\n2.  Normative References\n\n   [RFC2119] RFC 2119.\n   [RFC9999] RFC 9999.\n",
		"rfc/full/rfc2119.txt": "Key words\n\n1.  Introduction\n\n   Words.\n\nReferences\n\n   none\n",
	})
	report, err := IndexUpdate(root)
	if err != nil {
		t.Fatalf("IndexUpdate: %v", err)
	}
	if !slices.Contains(report.Files, graphRel) {
		t.Errorf("report.Files = %v, want %s among them", report.Files, graphRel)
	}
	first, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(graphRel)))
	if err != nil {
		t.Fatalf("read %s: %v", graphRel, err)
	}
	for _, row := range []string{"from\tkind\tto\n", "rfc10000\tbuilds-on\trfc9999\n", "rfc10000\tupdates\trfc9999\n"} {
		if !strings.Contains(string(first), row) {
			t.Errorf("%s lacks %q:\n%s", graphRel, row, first)
		}
	}
	if strings.Contains(string(first), "rfc2119") {
		t.Errorf("%s carries an RFC 2119 edge:\n%s", graphRel, first)
	}
	if _, err := IndexUpdate(root); err != nil {
		t.Fatalf("IndexUpdate again: %v", err)
	}
	second, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(graphRel)))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Errorf("a second run changed %s", graphRel)
	}
}
