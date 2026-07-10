// VALIDATES: the XFRM backend's read path against the real kernel — ListSAs
// issues XfrmStateList and returns well-typed SAInfo entries. Auto-enrolled in
// the QEMU integration run via the derived `integration && linux` package list.
// PREVENTS: a regression in the netlink XFRM state enumeration surfacing only on
// a live appliance running IKE.

//go:build integration && linux

package dataplane

import (
	"errors"
	"testing"

	"golang.org/x/sys/unix"
)

func TestXFRMListSAs(t *testing.T) {
	b := &xfrmBackend{}
	sas, err := b.ListSAs(0)
	if err != nil {
		if errors.Is(err, unix.EPERM) || errors.Is(err, unix.EACCES) {
			t.Skipf("XFRM state list needs CAP_NET_ADMIN: %v", err)
		}
		t.Fatalf("ListSAs: %v", err)
	}
	// The list may legitimately be empty; assert only that entries are well formed.
	for i, sa := range sas {
		if sa.Src == nil || sa.Dst == nil {
			t.Errorf("SA %d has a nil endpoint: %+v", i, sa)
		}
	}
}
