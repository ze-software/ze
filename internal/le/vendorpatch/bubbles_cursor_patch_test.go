package vendorpatch

import "testing"

const (
	bubblesCursorPatchPath = "internal/le/vendorpatch/patches/bubbles-textinput-cursor.patch"
	bubblesCursorRecovery  = "git apply internal/le/vendorpatch/patches/bubbles-textinput-cursor.patch"
)

// TestBubblesCursorPatchApplied keeps go mod vendor from removing the fix for
// charmbracelet/bubbles#1001, which is not upstream yet.
//
// textinput.Model.Cursor placed the HARDWARE cursor with m.Position(), the
// absolute caret index, while View draws with the scroll-adjusted
// max(0, m.pos-m.offset). Once the text is longer than the input width the two
// disagree and the cursor lands past the viewport, so the column an operator
// sees is not the column the program believes it wrote.
//
// It is guarded because it is invisible when lost. A dependency bump runs
// go mod vendor, the vendored file returns to upstream, nothing fails to
// compile, and no test outside this one reads that line. The same shape already
// cost this repository the netlink XFRM fixes, which is why
// TestNetlinkXFRMPatchApplied exists and why this is its twin.
func TestBubblesCursorPatchApplied(t *testing.T) {
	assertVendorPatchApplied(t, bubblesCursorPatchPath, bubblesCursorRecovery)
}
