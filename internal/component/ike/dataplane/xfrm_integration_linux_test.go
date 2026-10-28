// VALIDATES: the XFRM backend's read path against the real kernel — ListSAs
// issues XfrmStateList and returns well-typed SAInfo entries. Auto-enrolled in
// the QEMU integration run via the derived `integration && linux` package list.
// PREVENTS: a regression in the netlink XFRM state enumeration surfacing only on
// a live appliance running IKE.
// METHOD: userns.Enter runs it as root over a user and network namespace of its
// own, so it needs no host privilege and a refused list is a failure.

//go:build linux

package dataplane

import (
	"testing"

	"github.com/ze-software/ze/internal/test/userns"
)

func TestXFRMListSAs(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	b := &xfrmBackend{}
	sas, err := b.ListSAs(0)
	if err != nil {
		t.Fatalf("ListSAs: %v", err)
	}
	// The list may legitimately be empty; assert only that entries are well formed.
	for i, sa := range sas {
		if sa.Src == nil || sa.Dst == nil {
			t.Errorf("SA %d has a nil endpoint: %+v", i, sa)
		}
	}
}
