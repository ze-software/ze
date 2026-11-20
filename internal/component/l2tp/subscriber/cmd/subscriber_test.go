// VALIDATES: AC-6, AC-7, AC-8 (wiring: handlers call show.Enrich/EnrichBrief)
// PREVENTS: enrichment regression if handler is refactored

package cmd

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/show"
)

func TestSubscriberDetailCallsEnrich(t *testing.T) {
	t.Cleanup(show.ResetForTest)

	subscriber.DefaultRegistry.Add(&subscriber.Session{
		ID:         "test-1",
		AccessType: subscriber.AccessPPPoE,
		State:      subscriber.StateActive,
	})
	t.Cleanup(func() { subscriber.DefaultRegistry.Remove("test-1") })

	enriched := false
	show.MustRegister("show subscriber detail", "test", show.Enricher{
		Detail: func(base map[string]any) {
			enriched = true
			base["test-enriched"] = true
		},
	})

	ctx := &pluginserver.CommandContext{
		Selectors: map[string]string{"id": "test-1"},
	}
	resp, err := handleDetail(ctx, commandtest.Args())
	if err != nil {
		t.Fatalf("handleDetail error: %v", err)
	}
	if resp == nil {
		t.Fatal("handleDetail returned nil response")
	}
	if !enriched {
		t.Fatal("show.Enrich was not called by handleDetail")
	}
}

func TestSubscriberSummaryCallsEnrichBrief(t *testing.T) {
	t.Cleanup(show.ResetForTest)

	subscriber.DefaultRegistry.Add(&subscriber.Session{
		ID:         "test-2",
		AccessType: subscriber.AccessL2TP,
		State:      subscriber.StateActive,
	})
	t.Cleanup(func() { subscriber.DefaultRegistry.Remove("test-2") })

	briefCalled := 0
	show.MustRegister("show subscriber", "test", show.Enricher{
		Brief: func(base map[string]any) {
			briefCalled++
			base["test-brief"] = true
		},
	})

	resp, err := handleSummary(nil, commandtest.Args())
	if err != nil {
		t.Fatalf("handleSummary error: %v", err)
	}
	if resp == nil {
		t.Fatal("handleSummary returned nil response")
	}
	if briefCalled != 1 {
		t.Fatalf("expected EnrichBrief called 1 time, got %d", briefCalled)
	}
}

// TestSessionFullPrintsIPv6InterfaceID proves `show subscriber id <id> detail`
// reports the registry's IPv6CP interface identifier. Method: sessionFull, the
// producer of the detail answer, is given a set identifier and then a zero one.
// A set identifier is 16 uppercase hex digits with no 0x (the CLI raw-hex
// rule); a zero identifier omits the key, as an unset IPv4 address omits ipv4,
// so an IPv6 family release shows as the key leaving while ipv4 stays.
func TestSessionFullPrintsIPv6InterfaceID(t *testing.T) {
	sess := &subscriber.Session{
		ID:              "test-ipv6-id",
		AccessType:      subscriber.AccessL2TP,
		State:           subscriber.StateActive,
		IPv4Addr:        netip.MustParseAddr("192.0.2.9"),
		IPv6InterfaceID: [8]byte{0x02, 0x1a, 0x2b, 0xff, 0xfe, 0x3c, 0x4d, 0x5e},
	}
	full := sessionFull(sess)
	if got := full["ipv6-interface-id"]; got != "021A2BFFFE3C4D5E" {
		t.Fatalf("ipv6-interface-id = %v, want 021A2BFFFE3C4D5E", got)
	}
	if got := full["ipv4"]; got != "192.0.2.9" {
		t.Fatalf("ipv4 = %v, want 192.0.2.9 beside the interface identifier", got)
	}

	sess.IPv6InterfaceID = [8]byte{}
	full = sessionFull(sess)
	if got, present := full["ipv6-interface-id"]; present {
		t.Fatalf("ipv6-interface-id present for a zero identifier: %v", got)
	}
	if got := full["ipv4"]; got != "192.0.2.9" {
		t.Fatalf("ipv4 = %v after the identifier cleared, want 192.0.2.9", got)
	}
}
