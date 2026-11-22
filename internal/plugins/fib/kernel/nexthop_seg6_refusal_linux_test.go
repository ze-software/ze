//go:build linux

// Design: docs/architecture/fib/fib-depth-4-srv6.md -- Kernel backend
// Related: nexthop_linux.go -- addRichRoute, replaceRichRoute, wrapSEG6InstallError
//
// Unit tests proving that an EOPNOTSUPP refusal from netlink on an SRv6
// Service SID route becomes an operator error naming CONFIG_IPV6_SEG6_LWTUNNEL,
// the prefix, that the route was not installed, and ze doctor, while non-SEG6
// routes (such as MPLS) and non-EOPNOTSUPP errnos pass through unchanged.

package fibkernel

import (
	"errors"
	"net/netip"
	"strings"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
)

const (
	seg6RefusalFeature      = "CONFIG_IPV6_SEG6_LWTUNNEL"
	seg6RefusalNotInstalled = "was not installed"
	seg6RefusalFix          = "ze doctor"
)

type fakeSEG6NetlinkHandle struct {
	netlinkHandle
	addErr       error
	replaceErr   error
	listed       []netlink.Route
	addCalls     int
	replaceCalls int
}

func (f *fakeSEG6NetlinkHandle) RouteAdd(_ *netlink.Route) error {
	f.addCalls++
	return f.addErr
}

func (f *fakeSEG6NetlinkHandle) RouteReplace(_ *netlink.Route) error {
	f.replaceCalls++
	return f.replaceErr
}

func (f *fakeSEG6NetlinkHandle) RouteListFiltered(_ int, _ *netlink.Route, _ uint64) ([]netlink.Route, error) {
	return f.listed, nil
}

// VALIDATES: AC-10. When netlink answers EOPNOTSUPP on addRichRoute or
// replaceRichRoute for a route carrying SEG6 encapsulation, the backend wraps
// the error as an opererr.Error naming CONFIG_IPV6_SEG6_LWTUNNEL, the prefix,
// that the route was not installed, and ze doctor (Fix() == "ze doctor"), while
// preserving errors.Is(..., unix.EOPNOTSUPP), and fibKernel.processEvent raises
// a fib-sync-failure report carrying that explanation. An EOPNOTSUPP on an
// MPLS route, a discard route, or a non-EOPNOTSUPP error on an SRv6 route is
// never rewritten.
// PREVENTS: an operator seeing a bare "operation not supported" on a kernel
// without CONFIG_IPV6_SEG6_LWTUNNEL, or an unrelated MPLS/discard/netlink error
// being misattributed to missing SRv6 support.
func TestSEG6InstallRefusalNamesKernelFeature(t *testing.T) {
	prefix := netip.MustParsePrefix("192.0.2.0/24")
	sid := netip.MustParseAddr("2001:db8:1::2")
	nextHop := netip.MustParseAddr("198.51.100.1")

	srv6Route := RichRoute{
		Prefix:  prefix,
		NextHop: nextHop,
		SRv6SID: sid,
	}
	mplsRoute := RichRoute{
		Prefix:  prefix,
		NextHop: nextHop,
		Labels:  []uint32{300},
	}
	existingZeRoute := []netlink.Route{{Protocol: rtprotZE, Priority: 0}}

	t.Run("SEG6 addRichRoute EOPNOTSUPP wraps with operator error", func(t *testing.T) {
		fake := &fakeSEG6NetlinkHandle{addErr: unix.EOPNOTSUPP}
		backend := &netlinkBackend{handle: fake}

		err := backend.addRichRoute(srv6Route)
		if err == nil {
			t.Fatal("addRichRoute succeeded despite EOPNOTSUPP")
		}
		if fake.addCalls != 1 {
			t.Errorf("RouteAdd calls = %d, want 1", fake.addCalls)
		}
		assertSEG6RefusalError(t, err, prefix)
	})

	t.Run("SEG6 replaceRichRoute EOPNOTSUPP wraps on RouteReplace", func(t *testing.T) {
		fake := &fakeSEG6NetlinkHandle{
			replaceErr: unix.EOPNOTSUPP,
			listed:     existingZeRoute,
		}
		backend := &netlinkBackend{handle: fake}

		err := backend.replaceRichRoute(srv6Route)
		if err == nil {
			t.Fatal("replaceRichRoute succeeded despite EOPNOTSUPP")
		}
		if fake.replaceCalls != 1 {
			t.Errorf("RouteReplace calls = %d, want 1", fake.replaceCalls)
		}
		assertSEG6RefusalError(t, err, prefix)
	})

	t.Run("SEG6 replaceRichRoute EOPNOTSUPP wraps on empty-slot RouteAdd", func(t *testing.T) {
		fake := &fakeSEG6NetlinkHandle{addErr: unix.EOPNOTSUPP}
		backend := &netlinkBackend{handle: fake}

		err := backend.replaceRichRoute(srv6Route)
		if err == nil {
			t.Fatal("replaceRichRoute succeeded despite EOPNOTSUPP")
		}
		if fake.addCalls != 1 {
			t.Errorf("RouteAdd calls = %d, want 1", fake.addCalls)
		}
		assertSEG6RefusalError(t, err, prefix)
	})

	t.Run("MPLS EOPNOTSUPP is not rewritten on addRichRoute or replaceRichRoute", func(t *testing.T) {
		addFake := &fakeSEG6NetlinkHandle{addErr: unix.EOPNOTSUPP}
		addBackend := &netlinkBackend{handle: addFake}
		addErr := addBackend.addRichRoute(mplsRoute)
		if !errors.Is(addErr, unix.EOPNOTSUPP) {
			t.Fatalf("MPLS addRichRoute err = %v, want EOPNOTSUPP", addErr)
		}
		assertNotSEG6Refusal(t, addErr)

		repFake := &fakeSEG6NetlinkHandle{
			replaceErr: unix.EOPNOTSUPP,
			listed:     existingZeRoute,
		}
		repBackend := &netlinkBackend{handle: repFake}
		repErr := repBackend.replaceRichRoute(mplsRoute)
		if !errors.Is(repErr, unix.EOPNOTSUPP) {
			t.Fatalf("MPLS replaceRichRoute err = %v, want EOPNOTSUPP", repErr)
		}
		assertNotSEG6Refusal(t, repErr)
	})

	t.Run("Discard route with SRv6SID does not misclassify EOPNOTSUPP as SEG6", func(t *testing.T) {
		fake := &fakeSEG6NetlinkHandle{addErr: unix.EOPNOTSUPP}
		backend := &netlinkBackend{handle: fake}

		discardRoute := RichRoute{
			Prefix:    prefix,
			NextHop:   nextHop,
			SRv6SID:   sid,
			RouteType: events.RouteTypeBlackhole,
		}
		err := backend.addRichRoute(discardRoute)
		if !errors.Is(err, unix.EOPNOTSUPP) {
			t.Fatalf("discard addRichRoute err = %v, want EOPNOTSUPP", err)
		}
		assertNotSEG6Refusal(t, err)
	})

	t.Run("SEG6 non-EOPNOTSUPP error passes through unchanged", func(t *testing.T) {
		fake := &fakeSEG6NetlinkHandle{addErr: unix.EEXIST}
		backend := &netlinkBackend{handle: fake}

		err := backend.addRichRoute(srv6Route)
		if !errors.Is(err, unix.EEXIST) {
			t.Fatalf("SEG6 EEXIST err = %v, want EEXIST", err)
		}
		assertNotSEG6Refusal(t, err)
	})

	t.Run("processEvent raises fib-sync-failure report naming kernel feature and fix", func(t *testing.T) {
		report.ResetForTest()
		defer report.ResetForTest()

		fake := &fakeSEG6NetlinkHandle{addErr: unix.EOPNOTSUPP}
		f := newFIBKernel(&netlinkBackend{handle: fake})
		f.processEvent(makeSysribPayload([]incomingChange{{
			Action:  routeaction.Add,
			Prefix:  prefix,
			NextHop: nextHop,
			SRv6SID: sid,
		}}))

		var matched report.Issue
		found := false
		errs := report.Errors(10)
		for _, issue := range errs {
			if issue.Source != reportSourceFIB {
				continue
			}
			if issue.Code != reportCodeFIBSyncFailure {
				continue
			}
			if issue.Subject != prefix.String() {
				continue
			}
			matched = issue
			found = true
			break
		}
		if !found {
			t.Fatalf("no %s report error for %s in %+v", reportCodeFIBSyncFailure, prefix, errs)
		}
		assertSEG6RefusalMessage(t, matched.Message, prefix)
	})
}

func assertSEG6RefusalError(t *testing.T, err error, prefix netip.Prefix) {
	t.Helper()
	if !errors.Is(err, unix.EOPNOTSUPP) {
		t.Errorf("error %v does not wrap EOPNOTSUPP", err)
	}
	opErr, ok := errors.AsType[*opererr.Error](err)
	if !ok {
		t.Fatalf("error %T is not an *opererr.Error", err)
	}
	if opErr.Fix() != seg6RefusalFix {
		t.Errorf("opererr Fix() = %q, want %q", opErr.Fix(), seg6RefusalFix)
	}
	for _, want := range []string{seg6RefusalFeature, "Ze's kernel"} {
		if !strings.Contains(opErr.Why(), want) {
			t.Errorf("opererr Why() %q does not mention %q", opErr.Why(), want)
		}
	}
	assertSEG6RefusalMessage(t, err.Error(), prefix)
}

func assertSEG6RefusalMessage(t *testing.T, msg string, prefix netip.Prefix) {
	t.Helper()
	for _, want := range []string{
		prefix.String(),
		seg6RefusalFeature,
		seg6RefusalNotInstalled,
		seg6RefusalFix,
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("refusal %q does not mention %q", msg, want)
		}
	}
}

func assertNotSEG6Refusal(t *testing.T, err error) {
	t.Helper()
	if _, ok := errors.AsType[*opererr.Error](err); ok {
		t.Errorf("non-SEG6 or non-EOPNOTSUPP error was wrapped as opererr.Error: %v", err)
	}
	if strings.Contains(err.Error(), seg6RefusalFeature) {
		t.Errorf("error %q wrongly mentions %s", err, seg6RefusalFeature)
	}
}
