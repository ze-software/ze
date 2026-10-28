package rfc

import (
	"os"
	"strings"
	"testing"
)

// VALIDATES: every group the gate files findings under is declared once, with
// the words the public page prints for it.
//
// Method: walk CheckGroups and refuse a duplicate key, an empty key, or a group
// with no reader text, then confirm every declared constant is in the list.
func TestEveryCheckGroupIsDeclaredOnceWithReaderText(t *testing.T) {
	seen := map[CheckGroupKey]bool{}
	for _, group := range CheckGroups {
		if group.Key == "" {
			t.Error("a check group carries the empty key, which means no group")
		}
		if seen[group.Key] {
			t.Errorf("check group %q is declared twice", group.Key)
		}
		seen[group.Key] = true
		if strings.TrimSpace(group.Name) == "" {
			t.Errorf("check group %q has no name for the reader", group.Key)
		}
		if strings.TrimSpace(group.Verifies) == "" {
			t.Errorf("check group %q does not say what it verifies", group.Key)
		}
		if strings.TrimSpace(group.Failure) == "" {
			t.Errorf("check group %q does not say what a failure means", group.Key)
		}
	}
	for _, key := range []CheckGroupKey{CheckRequirementLists, CheckTestEvidence,
		CheckAnnotations, CheckPublicStatus, CheckAudits, CheckProofRecords, CheckTextReviews} {
		if !seen[key] {
			t.Errorf("check group %q has a constant and no entry in CheckGroups", key)
		}
	}
}

// VALIDATES: inGroup files every finding of a check under the group it names.
func TestInGroupFilesEveryFinding(t *testing.T) {
	findings := inGroup(CheckAudits, []Finding{{Message: "a"}, {Message: "b"}})
	for _, finding := range findings {
		if finding.Check != CheckAudits {
			t.Errorf("finding %q is filed under %q, want %q", finding.Message, finding.Check, CheckAudits)
		}
	}
}

// VALIDATES: every check whose findings check() collects goes through inGroup,
// so no finding reaches the public page without the group that raised it.
//
// Method: read check.go and refuse any append to the findings list that does
// not wrap its argument in inGroup. A new check added without a group goes red
// here rather than landing on the page as an unattributed finding.
func TestEveryCollectedCheckNamesItsGroup(t *testing.T) {
	source, err := os.ReadFile("check.go")
	if err != nil {
		t.Fatal(err)
	}
	appends := 0
	for number, line := range strings.Split(string(source), "\n") {
		if !strings.Contains(line, "findings = append(findings, ") {
			continue
		}
		appends++
		if !strings.Contains(line, "findings = append(findings, inGroup(") {
			t.Errorf("check.go:%d appends findings with no group: %s", number+1, strings.TrimSpace(line))
		}
	}
	if appends == 0 {
		t.Error("check.go carries no append to the findings list, so this test checks nothing")
	}
}
