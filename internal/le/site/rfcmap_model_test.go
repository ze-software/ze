// Design: docs/contributing/gh-pages.md -- RFC map projection over recorded inputs
// Related: rfcmap_model.go validates graph identities, scope and association provenance.
package site

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// TestRFCMapGraphPreservesIdentityAndScope loads shared parents, a citation
// cycle and a text-only document, then checks that scope remains authored.
func TestRFCMapGraphPreservesIdentityAndScope(t *testing.T) {
	data, err := loadRFCMapData(rfcMapModelFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	var stems []string
	for _, node := range data.Nodes {
		stems = append(stems, node.Stem)
	}
	if want := []string{"rfc4271", "rfc5492", "rfc9999", "rfc10000"}; !slices.Equal(stems, want) {
		t.Fatalf("graph identities = %v, want %v", stems, want)
	}
	if !slices.Equal(data.Nodes[0].Component, []string{"rfc4271", "rfc5492"}) {
		t.Errorf("citation cycle lost its members: %v", data.Nodes[0].Component)
	}
	if data.Nodes[3].Summary {
		t.Error("text-only graph node acquired a summary")
	}
	if data.Nodes[3].Walked {
		t.Error("text-only graph node acquired proof review")
	}
	wantEdges := []rfcLedgerEdge{
		{From: "rfc4271", Kind: "builds-on", To: "rfc5492"},
		{From: "rfc5492", Kind: "updates", To: "rfc4271"},
		{From: "rfc9999", Kind: "obsoletes", To: "rfc5492"},
	}
	if !slices.Equal(data.Edges, wantEdges) {
		t.Errorf("typed shared/cyclic edges = %v, want %v", data.Edges, wantEdges)
	}
	if !slices.Equal(data.Layers[0].Stems, []string{"rfc4271"}) {
		t.Errorf("proposed layer expanded through citations: %v", data.Layers[0].Stems)
	}
	wantProfile := rfcMapProfile{ID: "base", Title: "Base", Kind: "use-case", Release: true,
		Required: []string{"rfc4271"}, Optional: []string{"rfc5492"}}
	if !reflect.DeepEqual(data.Profiles, []rfcMapProfile{wantProfile}) {
		t.Errorf("profile tiers changed or candidates became owed: %+v", data.Profiles)
	}
}

// TestRFCMapRelationsPreserveProvenance projects mixed header and annotation
// associations, retaining exact levels and uncertainty without inventing owners.
func TestRFCMapRelationsPreserveProvenance(t *testing.T) {
	data, err := loadRFCMapData(rfcMapModelFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Relations) != 3 {
		t.Fatalf("relations = %+v, want one exact header, one broad header and one citation", data.Relations)
	}
	want := []rfcMapRelation{
		{Spec: "spec-exact", Stem: "rfc4271", RID: "RFC4271-1-1", Kind: "declared-requirement", Source: "plan/spec-exact.md"},
		{Spec: "spec-broad", Stem: "rfc4271", Kind: "declared-rfc", Source: "plan/spec-broad.md"},
		{Spec: "spec-exact", Stem: "rfc4271", RID: "RFC4271-1-2", Kind: "summary-gap-citation", Source: "rfc/short/rfc4271.md",
			Note: "Named as plan/spec-exact.md; this citation does not establish sole implementation ownership."},
	}
	if !reflect.DeepEqual(data.Relations, want) {
		t.Errorf("association provenance = %+v, want %+v", data.Relations, want)
	}
	var levels []string
	var gated []bool
	for _, requirement := range data.Requirements["rfc4271"] {
		levels = append(levels, requirement.Level)
		gated = append(gated, requirement.Gated)
	}
	if !slices.Equal(levels, []string{"MUST", "SHOULD", "MAY"}) {
		t.Errorf("exact requirement levels changed: %v", levels)
	}
	if !slices.Equal(gated, []bool{true, false, false}) {
		t.Errorf("mandatory scope changed: %v", gated)
	}
	if got := data.Requirements["rfc4271"][1].Annotation; got == nil {
		t.Error("gap annotation disappeared")
	} else if got.Reason != "See plan/spec-exact.md and spec-missing.md" {
		t.Errorf("gap qualification changed: %+v", got)
	}
	if len(data.Diagnostics) != 3 {
		t.Fatalf("diagnostics = %+v, want malformed metadata, unknown RID and unresolved citation", data.Diagnostics)
	}
	if data.Diagnostics[0].Spec != "spec-unmapped" {
		t.Errorf("unmapped spec lost its recorded diagnostic: %+v", data.Diagnostics[0])
	}
	if data.Diagnostics[1].Reference != "RFC4271-99-99" {
		t.Errorf("unknown RID became a confirmed association: %+v", data.Diagnostics[1])
	}
	if data.Diagnostics[2].Reference != "spec-missing.md" {
		t.Errorf("historical citation was guessed or dropped: %+v", data.Diagnostics[2])
	}
}

// TestRFCMapAmbiguousCitationRemainsUnassigned supplies two current files with
// the same historical basename and requires a diagnostic instead of an owner.
func TestRFCMapAmbiguousCitationRemainsUnassigned(t *testing.T) {
	paths := rfcMapModelFixture(t)
	writeFixtureFile(t, filepath.Join(paths.Output, roadmapDataFile), `{
		"schema-version":2,"revision":"recorded-revision","items":[
			{"name":"first","path":"plan/one/spec-exact.md"},
			{"name":"second","path":"plan/two/spec-exact.md"}
		]}`)
	data, err := loadRFCMapData(paths)
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Relations) != 0 {
		t.Errorf("ambiguous or missing historical paths acquired owners: %+v", data.Relations)
	}
	if len(data.Diagnostics) != 2 {
		t.Fatalf("citation diagnostics = %+v, want ambiguous and missing", data.Diagnostics)
	}
	if !strings.Contains(data.Diagnostics[0].Message, "2 recorded specs") {
		t.Errorf("ambiguous citation did not disclose both candidates: %+v", data.Diagnostics[0])
	}
}

// TestRFCMapInputProvenance binds every projected input to its recorded bytes;
// only the roadmap's own revision may become a recorded revision.
func TestRFCMapInputProvenance(t *testing.T) {
	paths := rfcMapModelFixture(t)
	data, err := loadRFCMapData(paths)
	if err != nil {
		t.Fatal(err)
	}
	if data.RoadmapRevision != "recorded-revision" {
		t.Errorf("roadmap revision = %q", data.RoadmapRevision)
	}
	if len(data.Inputs) != 3 {
		t.Fatalf("input provenance = %+v, want three input identities", data.Inputs)
	}
	for index, name := range []string{rfcLedgerFile, roadmapDataFile, rfcMapLayersFile} {
		root := paths.Output
		published := name
		if index == 2 {
			root = paths.Source
			published = "website/" + name
		}
		content, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		input := data.Inputs[index]
		if input.Path != published {
			t.Errorf("input %d path = %q, want %q", index, input.Path, published)
		}
		if want := fmt.Sprintf("%x", sha256.Sum256(content)); input.SHA256 != want {
			t.Errorf("input %s digest = %q, want %q", name, input.SHA256, want)
		}
		wantRevision := ""
		if index == 1 {
			wantRevision = "recorded-revision"
		}
		if input.Revision != wantRevision {
			t.Errorf("input %s revision = %q, want %q; absent evidence identity must remain unknown", name, input.Revision, wantRevision)
		}
	}
}

// TestRFCMapRejectsInvalidRecordedInputs corrupts one valid fixture input at a
// time and checks that the refusal names the broken identity or scope.
func TestRFCMapRejectsInvalidRecordedInputs(t *testing.T) {
	for _, test := range []struct {
		name, file, old, replacement, problem string
	}{
		{"roadmap schema", roadmapDataFile, `"schema-version":2`, `"schema-version":1`, "roadmap schema 1"},
		{"missing revision", roadmapDataFile, `"revision":"recorded-revision"`, `"revision":""`, "no recorded revision"},
		{"missing inventory", roadmapDataFile, `"items":[`, `"discarded":[`, "no spec inventory"},
		{"unknown graph endpoint", rfcLedgerFile, `"to":"rfc5492"`, `"to":"rfc404"`, "edge ends at unknown rfc404"},
		{"unknown cycle member", rfcLedgerFile, `"component":["rfc4271","rfc5492"]`, `"component":["rfc4271","rfc404"]`, "cycle of rfc4271"},
		{"duplicate graph node", rfcLedgerFile, `"stem":"rfc10000"`, `"stem":"rfc9999"`, "repeats graph node rfc9999"},
		{"duplicate requirement", rfcLedgerFile, `"rid":"RFC4271-1-3"`, `"rid":"RFC4271-1-1"`, "repeats RID RFC4271-1-1"},
		{"overlapping profile tiers", rfcLedgerFile, `"optional":{"rows":[{"stem":"rfc5492"}]}`, `"optional":{"rows":[{"stem":"rfc4271"}]}`, "profile base repeats rfc4271"},
		{"unknown layer member", rfcMapLayersFile, `"stems":["rfc4271"]`, `"stems":["rfc404"]`, "layer base names unknown"},
		{"unqualified proposal", rfcMapLayersFile, `"proposed":true`, `"proposed":false`, "must declare proposed true"},
		{"unknown default", rfcMapLayersFile, `"default-layer":"base"`, `"default-layer":"absent"`, "names no group"},
		{"unknown layer field", rfcMapLayersFile, `"proposed":true`, `"proposed":true,"support-guaranteed":true`, "unknown field"},
	} {
		t.Run(test.name, func(t *testing.T) {
			paths := rfcMapModelFixture(t)
			root := paths.Output
			if test.file == rfcMapLayersFile {
				root = paths.Source
			}
			name := filepath.Join(root, test.file)
			content, err := os.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(content), test.old) {
				t.Fatalf("fixture lacks corruption target %q", test.old)
			}
			writeFixtureFile(t, name, strings.ReplaceAll(string(content), test.old, test.replacement))
			_, err = loadRFCMapData(paths)
			if err == nil {
				t.Fatalf("invalid recorded input accepted: %s", test.name)
			}
			if !strings.Contains(err.Error(), test.problem) {
				t.Errorf("refusal = %v, want %q", err, test.problem)
			}
		})
	}
}

// rfcMapModelFixture writes only the recorded JSON inputs the model consumes;
// it has no git tree, test scanner, proof collector or production data dependency.
func rfcMapModelFixture(t *testing.T) Paths {
	t.Helper()
	root := t.TempDir()
	paths := Paths{Repository: root, Source: filepath.Join(root, "website"), Output: filepath.Join(root, "output")}
	writeFixtureFile(t, filepath.Join(paths.Output, rfcLedgerFile), `{
		"graph":{"nodes":[
			{"stem":"rfc4271","title":"Base BGP","summary":true,"component":["rfc4271","rfc5492"]},
			{"stem":"rfc5492","title":"Capabilities","summary":false,"component":["rfc4271","rfc5492"]},
			{"stem":"rfc9999","title":"Context","summary":false},
			{"stem":"rfc10000","title":"Text only","summary":false}
		],"edges":[
			{"from":"rfc4271","kind":"builds-on","to":"rfc5492"},
			{"from":"rfc5492","kind":"updates","to":"rfc4271"},
			{"from":"rfc9999","kind":"obsoletes","to":"rfc5492"}
		]},
		"profiles":[{"profile":"base","title":"Base","kind":"use-case","release":true,
			"rows":[{"stem":"rfc4271"}],"optional":{"rows":[{"stem":"rfc5492"}]},
			"candidates":[{"stem":"rfc9999"}]}],
		"stems":[{"stem":"rfc4271","summary-path":"rfc/short/rfc4271.md","requirements":[
			{"rid":"RFC4271-1-1","level":"MUST","gated":true,"text":"Base obligation","section":"1"},
			{"rid":"RFC4271-1-2","level":"SHOULD","gated":false,"text":"Advisory behavior","section":"1",
				"annotation":{"kind":"gap","reason":"See plan/spec-exact.md and spec-missing.md"}},
			{"rid":"RFC4271-1-3","level":"MAY","gated":false,"text":"Optional behavior","section":"1"}
		]}]}`)
	writeFixtureFile(t, filepath.Join(paths.Output, roadmapDataFile), `{
		"schema-version":2,"revision":"recorded-revision","items":[
			{"name":"spec-exact","path":"plan/spec-exact.md","rfc-gaps":["RFC4271-1-1","RFC4271-1-1"]},
			{"name":"spec-broad","path":"plan/spec-broad.md","rfc-gaps":["rfc4271"]},
			{"name":"spec-unmapped","path":"plan/spec-unmapped.md","diagnostics":["missing RFC gaps field"]},
			{"name":"spec-no-gap","path":"plan/spec-no-gap.md","rfc-gaps":[]},
			{"name":"spec-stale","path":"plan/spec-stale.md","rfc-gaps":["RFC4271-99-99"]}
		]}`)
	writeFixtureFile(t, filepath.Join(paths.Source, rfcMapLayersFile), `{
		"schema-version":1,"proposed":true,"default-layer":"base",
		"layers":[{"id":"base","title":"Base BGP","stems":["rfc4271"]}]}`)
	return paths
}

// TestRFCMapRequirementStatusesMatchRecordedEvidence checks that every
// per-requirement status, gap demonstration and unmet audit note projects from
// the recorded requirement ledger without duplicating proof counts onto nodes.
func TestRFCMapRequirementStatusesMatchRecordedEvidence(t *testing.T) {
	root := t.TempDir()
	paths := Paths{Repository: root, Source: filepath.Join(root, "website"), Output: filepath.Join(root, "output")}
	writeFixtureFile(t, filepath.Join(paths.Output, rfcLedgerFile), `{
		"graph":{"nodes":[
			{"stem":"rfc4271","title":"Base BGP","summary":true,"walked":true},
			{"stem":"rfc9000","title":"Successor","summary":true,"walked":true}
		],"edges":[]},
		"profiles":[{"profile":"base","title":"Base","kind":"use-case","release":true,
			"rows":[{"stem":"rfc4271"}]}],
		"stems":[
			{"stem":"rfc4271","summary-path":"rfc/short/rfc4271.md",
			 "figures":{"stem":"rfc4271","walked":true,"mandatory-proven":3,"mandatory-total":10,
				"unmet":["RFC4271-1-3","RFC4271-1-4","RFC4271-1-5"],
				"sections":{"must":{"total":10,"kept":9,"ze-proven":2,"os-verified":1,"os-gap":1,
					"proof-pending":1,"exclusion-unchecked":1,"not-assessed":1,"not-built":3,
					"not-owed":1,"not-owed-by-kind":{"not-applicable":1}}}},
			 "requirements":[
				{"rid":"RFC4271-1-1","level":"MUST","gated":true,"text":"Proven row","section":"1",
					"covers":[{"polarity":"positive","unit":"p::T","proof":{"route":"mutant","state":"verified","verified":true,"proves":true}}]},
				{"rid":"RFC4271-1-2","level":"MUST","gated":true,"text":"OS proven row","section":"1",
					"annotation":{"kind":"lower-layer","producer":"net/x.go::Set","reason":"kernel"},
					"covers":[{"polarity":"positive","unit":"p::TOS","proof":{"route":"mutant","state":"verified","producer":"net/x.go::Set","verified":true,"proves":true}}]},
				{"rid":"RFC4271-1-3","level":"MUST","gated":true,"text":"Gap row","section":"1",
					"annotation":{"kind":"gap","reason":"Not implemented yet"},
					"demonstrated-by":"pkg/x_test.go::TestGap"},
				{"rid":"RFC4271-1-4","level":"MUST","gated":true,"text":"Partial row","section":"1",
					"annotation":{"kind":"partial","tested":"half","gap":"other half","reason":"partial scope"}},
				{"rid":"RFC4271-1-5","level":"MUST","gated":true,"text":"Audit unmet row","section":"1",
					"audit":{"verdict":"unimplemented","meaning":"no code path","note":"missing handler","freshness":"fresh"}},
				{"rid":"RFC4271-1-6","level":"MUST","gated":true,"text":"OS gap row","section":"1",
					"annotation":{"kind":"lower-layer","producer":"net/x.go::Other","reason":"kernel"}},
				{"rid":"RFC4271-1-7","level":"MUST","gated":true,"text":"Pending row","section":"1",
					"covers":[{"polarity":"positive","unit":"p::TPending"}]},
				{"rid":"RFC4271-1-8","level":"MUST","gated":true,"text":"Unassessed row","section":"1"},
				{"rid":"RFC4271-1-9","level":"MUST","gated":true,"text":"Not owed row","section":"1",
					"annotation":{"kind":"not-applicable","reason":"operator choice"}},
				{"rid":"RFC4271-1-10","level":"MUST","gated":true,"text":"Unchecked exclusion row","section":"1",
					"annotation":{"kind":"feature-declined","reason":"declined feature"}},
				{"rid":"RFC4271-1-11","level":"MUST","gated":true,"text":"Superseded row","section":"1",
					"superseded":{"disposition":"restated","target":"RFC9000-1-1","reason":"moved"}},
				{"rid":"RFC4271-1-12","level":"MUST","gated":true,"text":"Rollup row","section":"1",
					"annotation":{"kind":"rollup","targets":["RFC4271-1-1"],"derived":"met","reason":"summary"}}
			]},
			{"stem":"rfc9000","summary-path":"rfc/short/rfc9000.md",
			 "figures":{"stem":"rfc9000","walked":true,"mandatory-proven":1,"mandatory-total":1,"unmet":[]},
			 "requirements":[
				{"rid":"RFC9000-1-1","level":"MUST","gated":true,"text":"Successor proven","section":"1",
					"covers":[{"polarity":"positive","unit":"p::TS","proof":{"route":"mutant","state":"verified","verified":true,"proves":true}}]}
			]}
		]}`)
	writeFixtureFile(t, filepath.Join(paths.Output, roadmapDataFile), `{
		"schema-version":2,"revision":"recorded-revision","items":[]}`)
	writeFixtureFile(t, filepath.Join(paths.Source, rfcMapLayersFile), `{
		"schema-version":1,"proposed":true,"default-layer":"base",
		"layers":[{"id":"base","title":"Base BGP","stems":["rfc4271"]}]}`)

	data, err := loadRFCMapData(paths)
	if err != nil {
		t.Fatal(err)
	}
	wantStatus := map[string]string{
		"RFC4271-1-1":  rfcSegZeProven,
		"RFC4271-1-2":  rfcSegOSVerified,
		"RFC4271-1-3":  rfcSegNotBuilt,
		"RFC4271-1-4":  rfcSegNotBuilt,
		"RFC4271-1-5":  rfcSegNotBuilt,
		"RFC4271-1-6":  rfcSegOSGap,
		"RFC4271-1-7":  rfcSegProofPending,
		"RFC4271-1-8":  rfcSegNotAssessed,
		"RFC4271-1-9":  rfcSegNotOwed,
		"RFC4271-1-10": rfcSegExclusionUnchecked,
		"RFC4271-1-11": rfcSegZeProven,
		"RFC4271-1-12": rfcMapStatusRollup,
	}
	gotByRID := map[string]rfcMapRequirement{}
	for _, row := range data.Requirements["rfc4271"] {
		gotByRID[row.RID] = row
		if want := wantStatus[row.RID]; row.Status != want {
			t.Errorf("%s status = %q, want %q", row.RID, row.Status, want)
		}
	}
	if len(gotByRID) != len(wantStatus) {
		t.Fatalf("projected %d requirements, want %d", len(gotByRID), len(wantStatus))
	}
	if got := gotByRID["RFC4271-1-3"].DemonstratedBy; got != "pkg/x_test.go::TestGap" {
		t.Errorf("gap demonstrated-by = %q, want pkg/x_test.go::TestGap", got)
	}
	if got := gotByRID["RFC4271-1-5"]; got.AuditVerdict != "unimplemented" || got.AuditNote != "missing handler" {
		t.Errorf("unmet audit disclosure = (%q, %q), want (unimplemented, missing handler)", got.AuditVerdict, got.AuditNote)
	}
}
