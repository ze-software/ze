// Design: docs/contributing/rfc-conformance-gates.md -- the extraction sign-off register
//
// PREVENTS: an id the extraction sanctions as unsourced being billed against
// the keyword budget that decides the register. The same `./le rfc check`
// accepts the id through a section's `unsourced-ids`, so counting it there too
// flips a stem at the boundary to `prose` and refuses its rfc2119 sign-off
// (plan/journal/exemption-still-billed-against-its-budget.md).
package rfc

import (
	"os"
	"path/filepath"
	"testing"
)

// unsourcedBoundarySummary declares three gated rows over a source holding two
// keyword sites (extractionCreateSource). The third row has no capitalised
// source sentence, so the extraction lists it in `unsourced-ids`.
const unsourcedBoundarySummary = `# RFC 9999

## Compliance Checklist

- [ ] [RFC9999-2-1] [MUST] A speaker MUST do the first thing (§2)
- [ ] [RFC9999-2-2] [MUST NOT] A speaker MUST NOT do the second thing (§2)
- [ ] [RFC9999-2-3] [MUST] A speaker MUST do the implied third thing (§2)
`

func unsourcedBoundaryArtifact() Extraction {
	return Extraction{
		Stem: "rfc9999", Register: registerRFC2119, SourcePath: "rfc/full/rfc9999.txt",
		Sections: []ExtractionSection{{
			ID: "2", Sites: 2, Disposition: "walked",
			Reason: "Requirements.", UnsourcedIDs: []string{"RFC9999-2-3"},
		}},
	}
}

// TestAnUnsourcedIDIsNotBilledAgainstTheKeywordRegister drives the register
// `./le rfc check` derives for a signed stem at the boundary: two keyword
// sites, three gated rows, one of them sanctioned as unsourced.
func TestAnUnsourcedIDIsNotBilledAgainstTheKeywordRegister(t *testing.T) {
	tree := fixtureTree(t, map[string]string{"rfc/full/rfc9999.txt": extractionCreateSource})
	requirements, err := parseSummaryText(unsourcedBoundarySummary, "rfc9999", "rfc/short/rfc9999.md")
	if err != nil {
		t.Fatalf("parse summary: %v", err)
	}
	signed := map[string]Extraction{"rfc9999": unsourcedBoundaryArtifact()}

	registers, err := derivedRegisters(NewDeriver(tree), signed, requirements)
	if err != nil {
		t.Fatalf("derivedRegisters: %v", err)
	}
	if registers["rfc9999"] != registerRFC2119 {
		t.Errorf("2 keyword sites over 3 gated rows, 1 of them unsourced: register = %q, want %q",
			registers["rfc9999"], registerRFC2119)
	}

	// The sanction subtracts only the ids it names: with a second unsourced
	// row billed, three gated rows exceed two keyword sites.
	unnamed := unsourcedBoundaryArtifact()
	unnamed.Sections[0].UnsourcedIDs = []string{"RFC9999-9-9"}
	registers, err = derivedRegisters(NewDeriver(tree), map[string]Extraction{"rfc9999": unnamed}, requirements)
	if err != nil {
		t.Fatalf("derivedRegisters: %v", err)
	}
	if registers["rfc9999"] == registerRFC2119 {
		t.Errorf("an unsourced id naming no gated row was subtracted: register = %q", registers["rfc9999"])
	}
}

// TestTheExtractWriterDoesNotBillAnUnsourcedID drives `./le rfc extract`'s
// refresh of an artifact whose section already sanctions the unsourced row.
func TestTheExtractWriterDoesNotBillAnUnsourcedID(t *testing.T) {
	tree := fixtureTree(t, map[string]string{
		"feature-gates.txt":    "",
		"go.mod":               "module fixture\n",
		"rfc/full/rfc9999.txt": extractionCreateSource,
		"rfc/short/rfc9999.md": unsourcedBoundarySummary,
	})
	inventory, err := NewDeriver(tree).Inventory("rfc9999", 2)
	if err != nil {
		t.Fatalf("Inventory: %v", err)
	}
	if inventory == nil {
		t.Fatal("the source is present and the inventory is nil")
	}
	previous := unsourcedBoundaryArtifact()
	body, err := marshalExtractionDocument(newExtractionDocument(inventory, &previous))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	path := filepath.Join(tree, "rfc", "extraction", "rfc9999.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}

	document, err := deriveExtractionDocument(tree, "rfc9999")
	if err != nil {
		t.Fatalf("deriveExtractionDocument: %v", err)
	}
	if document.Register != registerRFC2119 {
		t.Errorf("the refreshed skeleton derived register %q, want %q: the unsourced id was billed",
			document.Register, registerRFC2119)
	}
}

// TestAWeakerSignOffIsJudgedAgainstItsOwnRegistersSites derives one source two
// ways. Once the sourced count reaches the keyword sites the source supports
// rfc2119, and a sign-off under prose stays legal: it must be compared with the
// prose site set it walked, never the keyword set it did not.
func TestAWeakerSignOffIsJudgedAgainstItsOwnRegistersSites(t *testing.T) {
	// The lowercase sentence is a prose site and no keyword site, so the two
	// sets differ and a sign-off judged against the wrong one shows.
	tree := fixtureTree(t, map[string]string{"rfc/full/rfc9999.txt": derivedFixture +
		"\n    The widget must be blue.\n"})
	deriver := NewDeriver(tree)

	proseOnly, err := deriver.Inventory("rfc9999", 99)
	if err != nil {
		t.Fatalf("Inventory: %v", err)
	}
	if proseOnly == nil || proseOnly.Register != registerProse {
		t.Fatalf("1 keyword site over 99 sourced rows should derive prose: %+v", proseOnly)
	}
	weaker, err := deriver.InventoryUnder("rfc9999", 1, registerProse)
	if err != nil {
		t.Fatalf("InventoryUnder: %v", err)
	}
	if weaker == nil {
		t.Fatal("the source is present and the inventory is nil")
	}
	if weaker.Register != registerRFC2119 {
		t.Errorf("Register must answer what the source supports: %q, want %q", weaker.Register, registerRFC2119)
	}
	if len(proseOnly.Sites) != 2 {
		t.Fatalf("the fixture should hold 2 prose sites: %+v", proseOnly.Sites)
	}
	if len(weaker.Sites) != len(proseOnly.Sites) {
		t.Fatalf("a prose sign-off got %d site(s), the prose set holds %d: %+v",
			len(weaker.Sites), len(proseOnly.Sites), weaker.Sites)
	}
	for i := range weaker.Sites {
		if weaker.Sites[i] != proseOnly.Sites[i] {
			t.Errorf("site %d = %+v, want the prose site %+v", i, weaker.Sites[i], proseOnly.Sites[i])
		}
	}

	stronger, err := deriver.InventoryUnder("rfc9999", 1, registerRFC2119)
	if err != nil {
		t.Fatalf("InventoryUnder: %v", err)
	}
	if len(stronger.Sites) != 1 || stronger.Sites[0].ID != "2:1" {
		t.Errorf("an rfc2119 sign-off must read the keyword set: %+v", stronger.Sites)
	}
}
