// Design: docs/architecture/flowexport/flow-export-2-flow-records.md -- kernel sampling filter installation.

//go:build integration && linux

package sampling

import (
	"errors"
	"runtime"
	"testing"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
)

// TestSampleFilterKernelLifecycle checks the filter consumed by Linux, not just
// the builder's returned fields. Replacement on one link must leave the other
// installed sampler intact; removal must leave its sibling intact as well.
// This is not a statistical conformance test of Linux's sampling algorithm.
func TestSampleFilterKernelLifecycle(t *testing.T) {
	runtime.LockOSThread()
	original, err := netns.Get()
	if err != nil {
		runtime.UnlockOSThread()
		t.Fatal(err)
	}
	private, err := netns.New()
	if err != nil {
		_ = original.Close()
		runtime.UnlockOSThread()
		if errors.Is(err, unix.EPERM) || errors.Is(err, unix.EACCES) {
			t.Skipf("requires a private network namespace: %v", err)
		}
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := netns.Set(original); err != nil {
			// Do not unlock a thread left in the private namespace.
			t.Fatal(err)
		}
		if err := private.Close(); err != nil {
			t.Error(err)
		}
		if err := original.Close(); err != nil {
			t.Error(err)
		}
		runtime.UnlockOSThread()
	})
	h, err := netlink.NewHandle(unix.NETLINK_ROUTE)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Close)
	links := []*netlink.Dummy{
		{Name: "zsf0"},
		{Name: "zsf1"},
	}
	for _, link := range links {
		if err := h.LinkAdd(link); err != nil {
			t.Fatal(err)
		}
		if err := h.QdiscAdd(&netlink.Clsact{
			LinkIndex: link.Index, Handle: netlink.MakeHandle(0xffff, 0), Parent: netlink.HANDLE_CLSACT,
		}); err != nil {
			t.Fatal(err)
		}
	}
	first := buildSampleFilter(links[0].Index, 64, 7, 128)
	second := buildSampleFilter(links[1].Index, 1024, 9, 256)
	if err := h.FilterAdd(first); err != nil {
		t.Fatalf("install first sample filter (requires cls_matchall and act_sample): %v", err)
	}
	if err := h.FilterAdd(second); err != nil {
		t.Fatal(err)
	}
	assertKernelSampleFilter(t, h, links[0], 64, 7, 128)
	assertKernelSampleFilter(t, h, links[1], 1024, 9, 256)

	if err := h.FilterDel(first); err != nil {
		t.Fatal(err)
	}
	first = buildSampleFilter(links[0].Index, 1, 11, 64)
	if err := h.FilterAdd(first); err != nil {
		t.Fatal(err)
	}
	assertKernelSampleFilter(t, h, links[0], 1, 11, 64)
	assertKernelSampleFilter(t, h, links[1], 1024, 9, 256)
	if err := h.FilterDel(first); err != nil {
		t.Fatal(err)
	}
	remaining, err := h.FilterList(links[0], netlink.HANDLE_MIN_INGRESS)
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 0 {
		t.Fatalf("removed sampler still has %d ingress filters", len(remaining))
	}
	assertKernelSampleFilter(t, h, links[1], 1024, 9, 256)
}

func assertKernelSampleFilter(t *testing.T, h *netlink.Handle, link netlink.Link, rate, group, truncation uint32) {
	t.Helper()
	filters, err := h.FilterList(link, netlink.HANDLE_MIN_INGRESS)
	if err != nil {
		t.Fatal(err)
	}
	if len(filters) != 1 {
		t.Fatalf("%s: got %d ingress filters, want one", link.Attrs().Name, len(filters))
	}
	filter, ok := filters[0].(*netlink.MatchAll)
	if !ok {
		t.Fatalf("%s: kernel classifier is %T, want MatchAll", link.Attrs().Name, filters[0])
	}
	if filter.Protocol != unix.ETH_P_ALL || filter.Priority != SampleFilterPriority || len(filter.Actions) != 1 {
		t.Fatalf("%s: incorrect kernel match-all classifier: %+v", link.Attrs().Name, filter)
	}
	action, ok := filter.Actions[0].(*netlink.SampleAction)
	if !ok {
		t.Fatalf("%s: kernel action is %T, want sample", link.Attrs().Name, filter.Actions[0])
	}
	if action.Rate != rate || action.Group != group || action.TruncSize != truncation {
		t.Fatalf("%s: kernel sample rate/group/trunc = %d/%d/%d, want %d/%d/%d", link.Attrs().Name, action.Rate, action.Group, action.TruncSize, rate, group, truncation)
	}
	egress, err := h.FilterList(link, netlink.HANDLE_MIN_EGRESS)
	if err != nil {
		t.Fatal(err)
	}
	if len(egress) != 0 {
		t.Fatalf("%s: sampler installed %d egress filters", link.Attrs().Name, len(egress))
	}
}
