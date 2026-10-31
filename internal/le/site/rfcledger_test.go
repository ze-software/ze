// Design: website/AI.md -- the RFC data files the site publishes
package site

import (
	"encoding/json"
	"strings"
	"testing"
)

// retiredLedgerKeys is every JSON key the site published from the retired
// `Support status`, `Support coverage` and `Support remaining` Meta cells.
var retiredLedgerKeys = []string{
	`"public-status"`, `"public-coverage"`, `"public-remaining"`,
	`"status-counts"`, `"supported-with-remaining"`, `"status"`,
}

// VALIDATES: spec-rfc-support-derived-proof AC-7 -- the RFC data files carry
// derived proof only: no key from the retired Support cells, and each stem's
// figures carry the evidence-kind counts and the unmet ids.
//
// Method: build data/rfc-requirements.json through collectRequirementLedger,
// the path the site build takes, over a one-RFC tree whose only MUST row is
// `{partial}`, so its id must be listed as unmet. Then marshal an
// rfc-compliance.json value carrying a gap cluster, the shape that once held
// `top-rfcs[].status`, and check it holds no retired key either.
func TestRFCLedgerPublishesDerivedProofOnly(t *testing.T) {
	root, _, _ := partialPublicationTree(t)
	ledger, err := collectRequirementLedger(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(ledger.Stems) != 1 {
		t.Fatalf("want one stem, got %d", len(ledger.Stems))
	}
	entry := &ledger.Stems[0]
	if entry.PublicArea != "Widgets" {
		t.Errorf("public-area is %q, want the authored Support area %q", entry.PublicArea, "Widgets")
	}
	if entry.Figures == nil {
		t.Fatal("the stem publishes no figures, so its derived proof is absent")
	}
	data, err := json.Marshal(ledger)
	if err != nil {
		t.Fatal(err)
	}
	published := string(data)
	for _, key := range retiredLedgerKeys {
		if strings.Contains(published, key) {
			t.Errorf("rfc-requirements.json carries the retired key %s", key)
		}
	}
	for _, key := range []string{`"mandatory-by-kind"`, `"unmet":["RFC9999-2-1"]`} {
		if !strings.Contains(published, key) {
			t.Errorf("rfc-requirements.json lacks %s in the stem's derived figures", key)
		}
	}

	compliance := rfcCompliance{Gaps: rfcGaps{TopRFCs: []rfcGapCluster{{RFC: "rfc9999", Count: 1}}}}
	data, err = json.Marshal(compliance)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"top-rfcs":[{"rfc":"rfc9999"`) {
		t.Fatalf("the compliance value did not marshal its gap cluster: %s", data)
	}
	for _, key := range retiredLedgerKeys {
		if strings.Contains(string(data), key) {
			t.Errorf("rfc-compliance.json carries the retired key %s", key)
		}
	}
}
