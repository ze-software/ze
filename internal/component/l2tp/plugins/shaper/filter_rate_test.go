// Design: docs/architecture/l2tp/bng-1-radius-attributes.md -- Filter-Id rate wiring tests

package l2tpshaper

import (
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp"
	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/traffic"
)

// TestFilterIDRateReachesBothDirections drives every accepted Filter-Id
// spelling through the shaper's own entry point, rather than through the
// parser alone.
//
// The parser is tested in internal/component/traffic. What this file tests is
// the chain: a RADIUS Access-Accept stores the Filter-Id, session-up reads it,
// and BOTH halves reach an interface. The download half always did. The upload
// half was parsed, logged, stored on the session and applied to nothing, which
// is the defect
// plan/immediate/spec-l2tp-shaper-upload-rate-is-not-enforced.md closes.
func TestFilterIDRateReachesBothDirections(t *testing.T) {
	cases := []struct {
		filterID string
		download uint64
		upload   uint64
	}{
		{"10mbit", 10_000_000, 10_000_000},
		{"20mbit/5mbit", 20_000_000, 5_000_000},
		{"rate:100mbit/50mbit", 100_000_000, 50_000_000},
		{"rate:10mbit", 10_000_000, 10_000_000},
		{"1gbit", 1_000_000_000, 1_000_000_000},
	}
	mb := setupMockBackend(t)
	s := &shaperPlugin{}
	s.cfgPtr.Store(&shaperConfig{QdiscType: traffic.QdiscTBF, DefaultRate: 1_000_000})

	for i, c := range cases {
		tunnel, session := uint16(100+i), uint16(900+i)
		iface := "pppfid" + string(rune('a'+i))
		l2tp.StoreSessionMetadata(tunnel, session, &l2tp.AuthMetadata{FilterID: c.filterID})
		t.Cleanup(func() { l2tp.ClearSessionMetadata(tunnel, session) })

		s.onSessionUp(&l2tpevents.SessionUpPayload{TunnelID: tunnel, SessionID: session, Interface: iface})

		qos, ok := mb.getApplied(iface)
		if !ok {
			t.Fatalf("Filter-Id %q: no TC applied on %s", c.filterID, iface)
		}
		if qos.Qdisc.Classes[0].Rate != c.download {
			t.Errorf("Filter-Id %q: download = %d, want %d", c.filterID, qos.Qdisc.Classes[0].Rate, c.download)
		}
		if qos.Ingress.RateBps != c.upload {
			t.Errorf("Filter-Id %q: upload = %d, want %d", c.filterID, qos.Ingress.RateBps, c.upload)
		}
	}
}

// TestNonRateFilterIDLeavesConfiguredRates checks the other polarity. A
// Filter-Id that names something other than a rate must leave the configured
// rates in place, in both directions, rather than shaping at zero.
func TestNonRateFilterIDLeavesConfiguredRates(t *testing.T) {
	mb := setupMockBackend(t)
	s := &shaperPlugin{}
	s.cfgPtr.Store(&shaperConfig{
		QdiscType:   traffic.QdiscTBF,
		DefaultRate: 8_000_000,
		UploadRate:  2_000_000,
	})

	for i, filterID := range []string{"", "rate:", "not-a-rate", "10", "abc/def"} {
		tunnel, session := uint16(200+i), uint16(800+i)
		iface := "pppnr" + string(rune('a'+i))
		l2tp.StoreSessionMetadata(tunnel, session, &l2tp.AuthMetadata{FilterID: filterID})
		t.Cleanup(func() { l2tp.ClearSessionMetadata(tunnel, session) })

		s.onSessionUp(&l2tpevents.SessionUpPayload{TunnelID: tunnel, SessionID: session, Interface: iface})

		qos, ok := mb.getApplied(iface)
		if !ok {
			t.Fatalf("Filter-Id %q: no TC applied on %s", filterID, iface)
		}
		if qos.Qdisc.Classes[0].Rate != 8_000_000 {
			t.Errorf("Filter-Id %q: download = %d, want the configured 8000000", filterID, qos.Qdisc.Classes[0].Rate)
		}
		if qos.Ingress.RateBps != 2_000_000 {
			t.Errorf("Filter-Id %q: upload = %d, want the configured 2000000", filterID, qos.Ingress.RateBps)
		}
	}
}
