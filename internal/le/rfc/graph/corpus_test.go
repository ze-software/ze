package graph

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// noReferences is every corpus text that has no references section the
// parser can find, each with the reason, so a text that stops yielding one
// is a red rather than a silently shrunk profile (A-1).
var noReferences = map[string]string{
	"rfc5301": "its only references section is Informative References",
	"rfc5396": "its only references section is Informative References",
	"rfc7457": "its only references section is Informative References",
}

// VALIDATES: every corpus text yields a references section or is named here.
// PREVENTS: a text the parser cannot read silently dropping out of profiles.
// TestEveryCorpusTextYieldsReferences holds A-1 and A-5 over the real corpus:
// every text under rfc/full/ and rfc/drafts/ yields a references section or is
// on the named noReferences list, and the list names no text that does.
func TestEveryCorpusTextYieldsReferences(t *testing.T) {
	tree, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	g, err := Load(tree)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	var missing, listedButFound []string
	for _, stem := range g.Nodes() {
		_, listed := noReferences[stem]
		has := g.texts[stem].HasReferences
		if !has && !listed {
			missing = append(missing, stem)
		}
		if has && listed {
			listedButFound = append(listedButFound, stem)
		}
	}
	if len(missing) > 0 {
		t.Errorf("texts with no references section and not on noReferences: %s", strings.Join(missing, ", "))
	}
	if len(listedButFound) > 0 {
		t.Errorf("noReferences names texts that do have one: %s", strings.Join(listedButFound, ", "))
	}
	// The research anchors: RFC 4271 obsoletes 1771 and builds on 4760's
	// predecessor lineage; RFC 7606 updates 4271.
	if got := g.Updaters("rfc4271"); !slices.Contains(got, "rfc7606") {
		t.Errorf("Updaters(rfc4271) = %v, want rfc7606 among them", got)
	}
	if got, _ := g.HeaderObsoletes("rfc4271"); !slices.Equal(got, []string{"rfc1771"}) {
		t.Errorf("HeaderObsoletes(rfc4271) = %v, want [rfc1771]", got)
	}
}
