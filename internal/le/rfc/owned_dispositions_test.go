// Design: docs/contributing/rfc-conformance-gates.md -- document scope and retirement accounting.
package rfc

import (
	"slices"
	"testing"

	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// TestOwnedDocumentDisposition asks the native corpus readers about each of the
// eight independently owed documents. Retirement provenance must remain usable
// without enrolling a document outside the approved profiles. This tests
// accounting, not implementation conformance of the retired obligations.
func TestOwnedDocumentDisposition(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	collected, err := Collect(root)
	if err != nil {
		t.Fatal(err)
	}
	retired, err := retiredIDs(root)
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string]Requirement{}
	for _, row := range collected.Requirements {
		rows[row.RID] = row
	}
	for _, tc := range []struct {
		stem string
		ids  []string
	}{
		{"draft-ietf-sidrops-aspa-profile", []string{"DRAFT-IETF-SIDROPS-8210BIS-5.12-2", "DRAFT-IETF-SIDROPS-8210BIS-5.12-7", "RFC9582-5.12-2", "RFC9582-5.12-7"}},
		{"rfc4506", []string{"SFLOW-V5-x-8"}},
		{"rfc5080", []string{"RFC2865-2.5-2"}},
		{"rfc7519", []string{"RFC8707-5-1", "RFC8707-5-2"}},
		{"rfc1008", []string{"RFC905-x-5"}},
	} {
		t.Run(tc.stem, func(t *testing.T) {
			if collected.Enrolled[tc.stem] {
				t.Fatalf("unlisted document %s entered the counted population", tc.stem)
			}
			for _, id := range tc.ids {
				if !retired[id] {
					t.Errorf("native retirement reader lost correction provenance for %s", id)
				}
				if _, held := rows[id]; held {
					t.Errorf("retired obligation %s returned under its wrong document", id)
				}
				for _, tag := range collected.Tags {
					if tag.RID == id {
						t.Errorf("test still attributes obligation to retired id %s", id)
					}
				}
			}
		})
	}
	t.Run("rfc9117", func(t *testing.T) {
		if !collected.Enrolled["rfc9117"] {
			t.Fatal("existing RFC9117 repair left the counted population")
		}
		if !retired["RFC8955-6-2"] {
			t.Fatal("replaced RFC8955 rule lost retirement provenance")
		}
		row, held := rows["RFC9117-4.2-1"]
		if !held {
			t.Fatal("replacement RFC9117 left-most AS requirement is absent or ungated")
		}
		if !row.Gated() {
			t.Fatal("replacement RFC9117 left-most AS requirement is absent or ungated")
		}
		polarities := map[string]bool{}
		for _, tag := range collected.Tags {
			if tag.RID == row.RID {
				polarities[tag.Polarity] = true
			}
		}
		if !polarities[PolarityPositive] {
			t.Fatal("replacement RFC9117 requirement lost its polarity pair")
		}
		if !polarities[PolarityNegative] {
			t.Fatal("replacement RFC9117 requirement lost its polarity pair")
		}
	})
	t.Run("rfc7383", func(t *testing.T) {
		meta, held := collected.Metas["rfc7383"]
		if !held {
			t.Fatal("optional fragmentation debt was omitted or enrolled")
		}
		if collected.Enrolled["rfc7383"] {
			t.Fatal("optional fragmentation debt was omitted or enrolled")
		}
		if !meta.OutOfScope() {
			t.Fatal("optional fragmentation debt was omitted or enrolled")
		}
		if len(meta.UseCases) != 0 {
			t.Fatalf("fragmentation profile scope changed: required %v, optional %v", meta.UseCases, meta.OptionalUseCases)
		}
		if !slices.Equal(meta.OptionalUseCases, []string{"vpn-concentrator", "vpn-endpoint"}) {
			t.Fatalf("fragmentation profile scope changed: required %v, optional %v", meta.UseCases, meta.OptionalUseCases)
		}
		if _, held := rows["RFC7383-2.4-2"]; !held {
			t.Fatal("retired RFC7296 fragmentation recommendation was not retained in RFC7383")
		}
	})
	t.Run("rfc8247", func(t *testing.T) {
		row, held := rows["RFC8247-3.1-4"]
		if !held {
			t.Fatal("Digital Signature method 14 must remain an optional SHOULD row")
		}
		if row.Level != "SHOULD" {
			t.Fatal("Digital Signature method 14 must remain an optional SHOULD row")
		}
		if row.Gated() {
			t.Fatal("Digital Signature method 14 must remain an optional SHOULD row")
		}
		if !retired["RFC7296-3.8-1"] {
			t.Fatal("method14's original wrong-document attribution lost provenance")
		}
	})
}
