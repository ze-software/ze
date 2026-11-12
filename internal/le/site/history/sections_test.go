package sitehistory

import (
	"testing"
)

// rulesIdentifierPin is the digest RulesIdentifier answers for the section
// table, the third-party trees, CountingRulesVersion and
// reporewrite.GoLineRulesVersion as declared today: the rules every day of
// website/data/repo-history.json is measured under. It was copied by hand from
// the digest TestRulesIdentifierPinsTheSectionTable printed, and it changes
// only on purpose, together with a whole-history run.
const rulesIdentifierPin = "9d19dbe4e9734923f2422f17f18208d0968c7277327a619ef7fa2542a06de890"

// TestSectionsClassifyEveryPathExactlyOnce asks which section each path falls
// in. Method: one path per rule of the table, then the paths two rules could
// both claim, each with the section the spec's order gives it, written by hand.
func TestSectionsClassifyEveryPathExactlyOnce(t *testing.T) {
	for _, row := range []struct {
		path string
		want string
	}{
		{"vendor/github.com/example/lib/lib.go", "third-party-go"},
		{"vendor/x/y_test.go", "third-party-go"},
		{"third_party/lib/a.go", "third-party-go"},
		{"website/assets/vendor/lib/a.go", "third-party-go"},
		{"vendor/modules.txt", "third-party-other"},
		{"website/assets/vendor/htmx/htmx.min.js", "third-party-other"},
		{"third_party/lib/README.md", "third-party-other"},
		{"internal/core/family/family_test.go", "go-tests"},
		{"test/x_test.go", "go-tests"},
		{"internal/le/web/vendor/a_test.go", "go-tests"},
		{"internal/core/family/family.go", "go-production"},
		{"test/x.go", "go-production"},
		{"internal/tool/vendor/a.go", "go-production"},
		{"vendorx/a.go", "go-production"},
		{"ai/tool.go", "go-production"},
		{"test/plugin/reload.ci", "functional-tests"},
		{"rfc/full/rfc4271.txt", "rfc-corpus"},
		{"reference/iso10589.txt", "rfc-corpus"},
		{"plan/x.md", "plans"},
		{"plan/x.yang", "plans"},
		{"ai/rules/CORE.md", "agent-rules"},
		{".claude/settings.json", "agent-rules"},
		{"AGENTS.md", "agent-rules"},
		{"CLAUDE.md", "agent-rules"},
		{"GEMINI.md", "agent-rules"},
		{"docs/guide/bgp.md", "documentation"},
		{"docs/guide/example.yang", "documentation"},
		{"features/rfc-9774.md", "documentation"},
		{"README.md", "documentation"},
		{"website/index.html", "website"},
		{"demos/cli.cast", "website"},
		{"internal/component/bgp/schema/ze-bgp-conf.yang", "schemas"},
		{"Makefile", "other"},
		{"internal/core/family/README.md", "other"},
		{"internal/tool/vendor/a.js", "other"},
		{"testdata/x.txt", "other"},
		{"planning.txt", "other"},
		{"website", "other"},
	} {
		if got := SectionOf(row.path).ID; got != row.want {
			t.Errorf("SectionOf(%q) = %s, want %s", row.path, got, row.want)
		}
	}
}

// TestSectionListFollowsTheSpecOrder asks what the section list holds. Method:
// the twelve sections of the spec's "Sections" table, in its order, with the
// Go split on the three Go sections only, written by hand.
func TestSectionListFollowsTheSpecOrder(t *testing.T) {
	want := []Section{
		{ID: "third-party-go", Label: "Third-party Go", GoSplit: true},
		{ID: "third-party-other", Label: "Third-party other"},
		{ID: "go-tests", Label: "Go tests", GoSplit: true},
		{ID: "go-production", Label: "Go production", GoSplit: true},
		{ID: "functional-tests", Label: "Functional tests"},
		{ID: "rfc-corpus", Label: "RFC corpus"},
		{ID: "plans", Label: "Plans"},
		{ID: "agent-rules", Label: "Agent rules"},
		{ID: "documentation", Label: "Documentation"},
		{ID: "website", Label: "Website"},
		{ID: "schemas", Label: "Schemas"},
		{ID: "other", Label: "Other"},
	}
	got := Sections()
	if len(got) != len(want) {
		t.Fatalf("Sections() holds %d sections, want %d: %v", len(got), len(want), got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Errorf("section %d = %+v, want %+v", index+1, got[index], want[index])
		}
	}
}

// TestRulesIdentifierPinsTheSectionTable asks whether the rules identifier is
// still the one recorded history was measured under. Method: compare it with
// the pin, so an edit to the table or a raised version fails here first.
func TestRulesIdentifierPinsTheSectionTable(t *testing.T) {
	if got := RulesIdentifier(); got != rulesIdentifierPin {
		t.Fatalf("RulesIdentifier() = %s, pinned %s.\n"+
			"The section table, CountingRulesVersion or reporewrite.GoLineRulesVersion changed. "+
			"Change the pin on purpose, and because every recorded day was measured under the old rules, "+
			"rewrite the history with ./le site history update all", got, rulesIdentifierPin)
	}
}

// TestRulesIdentifierCoversRuleOrderAndConstraints asks whether the
// identifier moves when the table does. Method: swap two rules, then move one
// constraint between lists, and compare each identifier with the declared one.
func TestRulesIdentifierCoversRuleOrderAndConstraints(t *testing.T) {
	declared := RulesIdentifier()
	original := sections.rules
	t.Cleanup(func() { sections.rules = original })

	swapped := append([]rule(nil), original...)
	swapped[2], swapped[3] = swapped[3], swapped[2]
	sections.rules = swapped
	if RulesIdentifier() == declared {
		t.Errorf("swapping the Go tests and Go production rules left the identifier %s", declared)
	}

	moved := append([]rule(nil), original...)
	moved[8].rootSuffixes, moved[8].rootNames = nil, []string{".md"}
	sections.rules = moved
	if RulesIdentifier() == declared {
		t.Errorf("moving .md from root-suffixes to root-names left the identifier %s", declared)
	}
}
