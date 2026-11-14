package rfc

import "testing"

// TestQualifiedApprovalContentCellRoundTrips proves the Content cell of a row
// qualified with its file splits back into the digest pair and the file, an
// unqualified cell keeps no file, and a qualifier that leaves the checkout or
// cannot sit in one table cell is refused.
// VALIDATES: QualifiedApprovalContentCell, SplitApprovalCarrier and
// ValidApprovalCarrierPath agree on one grammar.
// PREVENTS: a qualified row whose file the loader reads back wrong, so its
// staleness check resolves another carrier than the judge read.
func TestQualifiedApprovalContentCellRoundTrips(t *testing.T) {
	content := ApprovalUnitContent{Base: approvalAbsent, Current: approvalAbsent}
	digests, file := SplitApprovalCarrier(QualifiedApprovalContentCell(content, "pkg/split_test.go"))
	if digests != "none..none" || file != "pkg/split_test.go" {
		t.Fatalf("qualified cell split = %q, %q", digests, file)
	}
	if digests, file := SplitApprovalCarrier(QualifiedApprovalContentCell(content, "")); digests != "none..none" || file != "" {
		t.Fatalf("unqualified cell split = %q, %q", digests, file)
	}
	for _, path := range []string{"pkg/split_test.go", "a/b/c_test.go"} {
		if !ValidApprovalCarrierPath(path) {
			t.Errorf("ValidApprovalCarrierPath(%q) = false, want true", path)
		}
	}
	for _, path := range []string{"", "../x_test.go", "..", "/abs_test.go", "pkg/../x_test.go", "pkg/a|b_test.go", "pkg/a\nb_test.go", " pkg/x_test.go", `pkg\x_test.go`} {
		if ValidApprovalCarrierPath(path) {
			t.Errorf("ValidApprovalCarrierPath(%q) = true, want false", path)
		}
	}
}
