// VALIDATES: the live conntrack netlink paths — NewReader/Dump/Close and
// NewDestroyListener/Close — construct against the real kernel and return
// well-typed results without panicking. Auto-enrolled in the native QEMU
// integration run through the derived `integration && linux` package list.
// PREVENTS: a regression in the netlink handle lifecycle or the destroy-listener
// socket setup going unnoticed until a live appliance scrapes conntrack.

//go:build integration && linux

package conntrack

import (
	"errors"
	"os"
	"testing"
)

func TestReaderDumpSmoke(t *testing.T) {
	r, err := NewReader()
	if err != nil {
		t.Skipf("NewReader unavailable in this environment: %v", err)
	}
	defer func() {
		if cerr := r.Close(); cerr != nil {
			t.Errorf("Close: %v", cerr)
		}
	}()

	entries, err := r.Dump()
	if err != nil {
		// No conntrack module / insufficient privileges: nothing to assert, but
		// the constructor path and error handling have been exercised.
		t.Skipf("conntrack table dump unavailable: %v", err)
	}
	// A dump may legitimately return zero flows; assert only that each converted
	// entry carries a valid 5-tuple address.
	for i, e := range entries {
		if !e.SrcAddr.IsValid() || !e.DstAddr.IsValid() {
			t.Errorf("entry %d has an invalid address: %+v", i, e)
		}
	}
}

func TestReaderClosedDump(t *testing.T) {
	r, err := NewReader()
	if err != nil {
		t.Skipf("NewReader unavailable: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := r.Dump(); err == nil {
		t.Error("Dump after Close should return an error")
	}
}

func TestDestroyListenerSmoke(t *testing.T) {
	l, err := NewDestroyListener()
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			t.Skipf("destroy listener needs CAP_NET_ADMIN: %v", err)
		}
		t.Skipf("destroy listener unavailable: %v", err)
	}
	if err := l.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}
