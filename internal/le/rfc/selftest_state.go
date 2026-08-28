// Design: docs/architecture/core-design.md -- the RFC engine proved against fixtures
// Overview: selftest.go -- fixture-suite orchestration and action answer
// Related: check_extraction.go -- the pure extraction-ratchet comparison this suite drives
//
// selftest_state.go exercises the authored audit and extraction records, their
// freshness and ratchets, and the generated-page writer and ownership rules.
package rfc

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/leroot"
)

const selftestRFCSource = `Test RFC 9999

1.  Introduction

    This document describes widgets.

2.  Widgets

    A speaker MUST send the widget. A receiver MUST NOT drop the widget.
`

func runAuditSelftest() ([]leroot.SelftestResult, error) {
	const testPath = "internal/sample/widget_test.go"
	const rid = "RFC9999-2-2"
	root, err := newSelftestTree("rfc-selftest-audit-", map[string]string{
		".github/workflows/nightly.yml": selftestWorkflow,
		"rfc/enrolled.txt":              "rfc9999\n",
		"rfc/short/rfc9999.md":          selftestSummary,
		testPath: "package sample\nfunc TestWidget() {\n" +
			"\t// RFC requirement: RFC9999-2-2 positive\n" +
			"\t// RFC requirement: RFC9999-2-2 negative\n}\n",
	})
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(root) //nolint:errcheck // temporary fixture checkout

	collected, err := Collect(root)
	if err != nil {
		return nil, err
	}
	var requirement Requirement
	for _, candidate := range collected.Requirements {
		if candidate.RID == rid {
			requirement = candidate
			break
		}
	}
	tags := tagsByRID(collected.Tags)[rid]
	reader := newSourceReader(root)
	index := newScopeIndex()
	keys := tagKeys(tags, reader, index)
	tests := taggedUnitSHAs(tags, reader, index)
	units, err := unitSHAs(keys, reader, index, "rfc selftest")
	if err != nil {
		return nil, err
	}
	auditDocument := map[string]any{
		"rfc":     "rfc9999",
		"audited": "2026-08-26",
		"requirements": map[string]any{
			rid: map[string]any{
				"verdict":         verdictEnforced,
				"note":            "TestWidget exercises accepted and rejected widget input.",
				"requirement_sha": RequirementSHA(requirement.Text),
				"tests":           tests,
				"units":           units,
				"code":            map[string]string{},
			},
		},
	}
	body, err := marshalSelftestJSON(auditDocument)
	if err != nil {
		return nil, err
	}
	if err := writeSelftestFiles(root, map[string]string{"rfc/audit/rfc9999.json": body}); err != nil {
		return nil, err
	}

	audits, err := loadAudits(root, collected.Enrolled)
	if err != nil {
		return nil, err
	}
	schemaErrors := checkAuditSchema(collected.Requirements, collected.Tags, audits)
	fresh := AuditFreshness(AuditFreshnessInput{
		Tree: root, Requirements: collected.Requirements, Tags: collected.Tags,
		Enrolled: collected.Enrolled, Audits: audits,
	})

	path := filepath.Join(root, filepath.FromSlash(testPath))
	original, err := os.ReadFile(path) // #nosec G304 -- path is constructed inside the temporary selftest fixture
	if err != nil {
		return nil, err
	}
	var shiftedFile textbuf.Buffer
	shiftedFile.Str("// File header inserted by the fixture.\n").Str(string(original))
	if err := os.WriteFile(path, []byte(shiftedFile.String()), 0o600); err != nil {
		return nil, err
	}
	shiftedTags, err := ScanTree(root)
	if err != nil {
		return nil, err
	}
	shifted := AuditFreshness(AuditFreshnessInput{
		Tree: root, Requirements: collected.Requirements, Tags: shiftedTags,
		Enrolled: collected.Enrolled, Audits: audits,
	})
	resealReport, err := resealTree(root)
	if err != nil {
		return nil, err
	}
	resealedAudits, err := loadAudits(root, collected.Enrolled)
	if err != nil {
		return nil, err
	}
	resealed := AuditFreshness(AuditFreshnessInput{
		Tree: root, Requirements: collected.Requirements, Tags: shiftedTags,
		Enrolled: collected.Enrolled, Audits: resealedAudits,
	})

	return []leroot.SelftestResult{
		selftestResult("audit/schema", len(schemaErrors) == 0 && len(audits["rfc9999"].Verdicts) == 1,
			"the complete enforced verdict did not satisfy the audit schema"),
		selftestResult("audit/freshness", fresh[rid].State == FreshState,
			"a verdict over unchanged requirement and test units was not fresh"),
		selftestResult("audit/file-shift", shifted[rid].State == ShiftedState,
			"a file-only movement was not distinguished from a changed test unit"),
		selftestResult("audit/reseal", len(resealReport.Resealed) == 1 && len(resealReport.Refused) == 0 && resealed[rid].State == FreshState,
			"the reseal writer did not re-stamp only the shifted verdict"),
	}, nil
}

func runExtractionSelftest() ([]leroot.SelftestResult, error) {
	root, err := newSelftestTree("rfc-selftest-extraction-", map[string]string{
		"rfc/enrolled.txt":     "rfc9999\n",
		"rfc/short/rfc9999.md": selftestSummary,
		"rfc/full/rfc9999.txt": selftestRFCSource,
	})
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(root) //nolint:errcheck // temporary fixture checkout

	requirements, err := parseSummaryText(selftestSummary, "rfc9999", "rfc/short/rfc9999.md")
	if err != nil {
		return nil, err
	}
	inventory, err := NewDeriver(root).Inventory("rfc9999", gatedCounts(requirements)["rfc9999"])
	if err != nil {
		return nil, err
	}
	if inventory == nil {
		return nil, &ParseError{msg: "rfc selftest: extraction inventory is absent"}
	}
	artifact := extractionSelftestArtifact(inventory)
	body, err := marshalSelftestJSON(artifact)
	if err != nil {
		return nil, err
	}
	if err := writeSelftestFiles(root, map[string]string{"rfc/extraction/rfc9999.json": body}); err != nil {
		return nil, err
	}
	parsed, err := ParseExtractionArtifact(root, filepath.Join(root, "rfc", "extraction", "rfc9999.json"))
	if err != nil {
		return nil, err
	}
	signed, violations, err := evaluateExtractions(NewDeriver(root), requirements)
	if err != nil {
		return nil, err
	}
	ratchet := checkExtractionRatchetAgainst(
		map[string]Extraction{},
		map[string]baselineExtraction{"rfc9999": {excluded: 0, signedOff: "2026-08-26"}},
		true,
	)

	return []leroot.SelftestResult{
		selftestResult("extraction/inventory", inventory.Register == registerRFC2119 && inventory.KeywordSites == 2 && len(inventory.Sites) == 2,
			"the source walk did not derive two RFC 2119 sites"),
		selftestResult("extraction/artifact", parsed.Mapped() == 2 && parsed.SourceSHA == inventory.SourceSHA,
			"the authored artifact did not retain both mapped sites and its source fingerprint"),
		selftestResult("extraction/evaluation", len(violations) == 0 && len(signed) == 1,
			"a complete extraction artifact did not earn sign-off"),
		selftestResult("extraction/ratchet", len(ratchet) == 1 && strings.Contains(ratchet[0], "had an extraction sign-off at HEAD"),
			"removing a baseline extraction sign-off did not fail the ratchet"),
	}, nil
}

func extractionSelftestArtifact(inventory *Inventory) map[string]any {
	sections := make([]map[string]any, 0, len(inventory.Sections))
	for _, section := range inventory.Sections {
		sections = append(sections, map[string]any{
			"id": section.ID, "sites": section.Sites, "disposition": "walked",
		})
	}
	mappedIDs := map[string]string{
		"2:1": "RFC9999-2-1",
		"2:2": "RFC9999-2-2",
	}
	sites := make([]map[string]any, 0, len(inventory.Sites))
	for _, site := range inventory.Sites {
		entry := map[string]any{
			"id": site.ID, "quote": site.Quote,
			"disposition":   dispositionExcluded,
			"excluded-kind": "not-a-requirement",
			"reason":        "the fixture did not declare this site",
		}
		if mappedTo := mappedIDs[site.ID]; mappedTo != "" {
			entry = map[string]any{
				"id": site.ID, "quote": site.Quote,
				"disposition": dispositionMapped, "mapped-to": mappedTo,
			}
		}
		sites = append(sites, entry)
	}
	return map[string]any{
		"schema-version": extractionSchemaVersion,
		"stem":           inventory.Stem,
		"register":       inventory.Register,
		"source-path":    inventory.SourcePath,
		"source-sha":     inventory.SourceSHA,
		"signed-off":     "2026-08-26",
		"reviewer":       "RFC selftest",
		"sections":       sections,
		"sites":          sites,
	}
}

func runRenderSelftest() ([]leroot.SelftestResult, error) {
	root, err := newSelftestTree("rfc-selftest-render-", map[string]string{
		".github/workflows/nightly.yml": selftestWorkflow,
		"rfc/enrolled.txt":              "rfc9999\n",
		"rfc/short/rfc9999.md":          selftestSummary,
		"rfc/full/rfc9999.txt":          selftestRFCSource,
		"docs/features/rfc-status.md":   "| RFC 9999 | Widgets | Partial | selftest | one MUST gap |\n",
	})
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(root) //nolint:errcheck // temporary fixture checkout

	first, err := IndexUpdate(root)
	if err != nil {
		return nil, err
	}
	indexPath := filepath.Join(root, filepath.FromSlash(ledgerRel))
	shardPath := filepath.Join(root, filepath.FromSlash(shardRel("rfc9999")))
	indexBody, err := os.ReadFile(indexPath) // #nosec G304 -- path is constructed inside the temporary selftest fixture
	if err != nil {
		return nil, err
	}
	shardBody, err := os.ReadFile(shardPath) // #nosec G304 -- path is constructed inside the temporary selftest fixture
	if err != nil {
		return nil, err
	}

	if err := writeSelftestFiles(root, map[string]string{"rfc/requirements/orphan.md": "generated orphan\n"}); err != nil {
		return nil, err
	}
	second, err := IndexUpdate(root)
	if err != nil {
		return nil, err
	}
	_, prunedErr := os.Stat(filepath.Join(root, "rfc", "requirements", "orphan.md"))

	if err := writeSelftestFiles(root, map[string]string{
		"rfc/requirements/kept-on-refusal.md": "must survive\n",
		"rfc/short/rfc-broken.md":             "## Compliance Checklist\n- [ ] [MUST] missing id (§2)\n",
	}); err != nil {
		return nil, err
	}
	before, err := os.ReadFile(indexPath) // #nosec G304 -- path is constructed inside the temporary selftest fixture
	if err != nil {
		return nil, err
	}
	_, refusal := IndexUpdate(root)
	after, readErr := os.ReadFile(indexPath) // #nosec G304 -- path is constructed inside the temporary selftest fixture
	if readErr != nil {
		return nil, readErr
	}
	_, keptErr := os.Stat(filepath.Join(root, "rfc", "requirements", "kept-on-refusal.md"))

	written := first.Ledger == ledgerRel && first.Shards == 1
	written = written && strings.Contains(string(indexBody), "# RFC Requirement Ledger")
	written = written && strings.Contains(string(shardBody), "RFC9999-2-1")
	pruned := len(second.Deleted) == 1 && second.Deleted[0] == "orphan" && os.IsNotExist(prunedErr)
	refused := refusal != nil && bytes.Equal(before, after) && keptErr == nil

	return []leroot.SelftestResult{
		selftestResult("render/pages", written,
			"the index and per-RFC shard were not produced from the shared render input"),
		selftestResult("render/generated-ownership", pruned,
			"the writer did not remove exactly the generated orphan shard"),
		selftestResult("render/write-refusal", refused,
			"a malformed summary changed a generated page or pruned an owned file"),
	}, nil
}

func marshalSelftestJSON(document any) (string, error) {
	body, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return "", err
	}
	var out textbuf.Buffer
	return out.Str(string(body)).Byte('\n').String(), nil
}
