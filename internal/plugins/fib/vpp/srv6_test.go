package fibvpp

import (
	"net/netip"
	"testing"

	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
	sysribevents "codeberg.org/thomas-mangin/ze/internal/component/sysrib/events"
)

type mockSRv6Backend struct {
	steers    []srv6SteerOp
	delSteers []netip.Prefix
	err       error
}

type srv6SteerOp struct {
	prefix  netip.Prefix
	sid     netip.Addr
	tableID uint32
}

func (m *mockSRv6Backend) addSRv6Steer(prefix netip.Prefix, sid netip.Addr, tableID uint32) error {
	if m.err != nil {
		return m.err
	}
	m.steers = append(m.steers, srv6SteerOp{prefix, sid, tableID})
	return nil
}

func (m *mockSRv6Backend) delSRv6Steer(prefix netip.Prefix, _ uint32) error {
	if m.err != nil {
		return m.err
	}
	m.delSteers = append(m.delSteers, prefix)
	return nil
}

func TestSRv6SteerAdd(t *testing.T) {
	mock := &mockSRv6Backend{}
	f := newFibVPP(&mockBackend{})
	f.srv6Backend = mock

	sid := netip.MustParseAddr("2001:db8::1")
	prefix := netip.MustParsePrefix("10.0.0.0/24")

	f.processEvent(&sysribevents.BestChangeBatch{
		Changes: []sysribevents.BestChangeEntry{{
			Action:  bgptypes.RouteActionAdd,
			Prefix:  prefix,
			NextHop: netip.MustParseAddr("192.0.2.1"),
			SRv6SID: sid,
		}},
	})

	if len(mock.steers) != 1 {
		t.Fatalf("expected 1 steer, got %d", len(mock.steers))
	}
	if mock.steers[0].sid != sid {
		t.Errorf("sid = %v, want %v", mock.steers[0].sid, sid)
	}
	if mock.steers[0].prefix != prefix {
		t.Errorf("prefix = %v, want %v", mock.steers[0].prefix, prefix)
	}
	if !f.srv6Installed[prefix.String()] {
		t.Error("prefix not tracked in srv6Installed")
	}
}

func TestSRv6SteerWithdraw(t *testing.T) {
	mock := &mockSRv6Backend{}
	f := newFibVPP(&mockBackend{})
	f.srv6Backend = mock

	prefix := netip.MustParsePrefix("10.0.0.0/24")
	f.srv6Installed[prefix.String()] = true

	f.processEvent(&sysribevents.BestChangeBatch{
		Changes: []sysribevents.BestChangeEntry{{
			Action: bgptypes.RouteActionWithdraw,
			Prefix: prefix,
		}},
	})

	if len(mock.delSteers) != 1 {
		t.Fatalf("expected 1 del, got %d", len(mock.delSteers))
	}
	if mock.delSteers[0] != prefix {
		t.Errorf("del prefix = %v, want %v", mock.delSteers[0], prefix)
	}
	if f.srv6Installed[prefix.String()] {
		t.Error("prefix still tracked after withdraw")
	}
}

func TestSRv6ZeroSIDSkipped(t *testing.T) {
	mock := &mockSRv6Backend{}
	backend := &mockBackend{}
	f := newFibVPP(backend)
	f.srv6Backend = mock

	prefix := netip.MustParsePrefix("10.0.0.0/24")
	nh := netip.MustParseAddr("192.0.2.1")

	f.processEvent(&sysribevents.BestChangeBatch{
		Changes: []sysribevents.BestChangeEntry{{
			Action:  bgptypes.RouteActionAdd,
			Prefix:  prefix,
			NextHop: nh,
		}},
	})

	if len(mock.steers) != 0 {
		t.Error("zero SID should not trigger SRv6 steer")
	}
	if len(backend.adds) != 1 {
		t.Error("expected plain route add")
	}
}
