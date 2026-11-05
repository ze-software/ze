// Design: docs/contributing/rfc-conformance-gates.md -- explicit-kernel placement.
package rfc

import (
	"testing"

	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// TestRecordExplicitKernelSelectsCompatibleGuest exercises the recording entry's
// runner construction without starting a compiler or VM. A plain Go unit matches
// both contexts: a kernel must select the guest; no kernel must retain the host.
func TestRecordExplicitKernelSelectsCompatibleGuest(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	const rel = "internal/le/rfc/discriminate_guest_selection_test.go"
	tag := Tag{File: rel}
	record := DiscriminationRecord{Unit: tag.File + "::TestSelection"}
	for _, kernel := range []string{"", "tmp/kernel/build/vmlinuz"} {
		t.Run(kernel, func(t *testing.T) {
			runner, err := newObservationRunner(root, newSourceReader(root), newScopeIndex(),
				Carrier{Kind: kindUnit}, tag, &record, kernel)
			if err != nil {
				t.Fatal(err)
			}
			if runner.guest != (kernel != "") {
				t.Errorf("guest=%v for kernel %q", runner.guest, kernel)
			}
		})
	}
}
