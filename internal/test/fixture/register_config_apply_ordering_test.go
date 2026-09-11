package fixture

import (
	"strings"
	"testing"
)

// swapMonitorText is what `ip -4 monitor address` prints for the swap the
// requirement asks for: both destroys, then both creates. The indented
// lifetime lines and the local and broadcast route notifications are kept,
// because the parser has to ignore exactly those.
const swapMonitorText = `Deleted 3: zdual0    inet 10.90.0.1/24 scope global zdual0
Deleted 4: zdual1    inet 10.91.0.1/24 scope global zdual1
3: zdual0    inet 10.91.0.1/24 scope global zdual0
       valid_lft forever preferred_lft forever
4: zdual1    inet 10.90.0.1/24 scope global zdual1
       valid_lft forever preferred_lft forever
`

// makeBeforeBreakText is the same swap applied the way the deleted relaxation
// applied it: both creates first, so each interface holds two addresses for
// the duration of the window.
const makeBeforeBreakText = `3: zdual0    inet 10.91.0.1/24 scope global zdual0
       valid_lft forever preferred_lft forever
4: zdual1    inet 10.90.0.1/24 scope global zdual1
       valid_lft forever preferred_lft forever
Deleted 3: zdual0    inet 10.90.0.1/24 scope global zdual0
Deleted 4: zdual1    inet 10.91.0.1/24 scope global zdual1
`

// mixedMonitorText is what `ip -4 monitor address route` prints for the
// mixed-root reload, with the connected and local routes the kernel adds
// alongside the address. The old address goes first, which is phase 3, and the
// route the coarse root installs comes last.
const mixedMonitorText = `Deleted 172.31.0.0/24 via 10.92.0.2 dev zmix0 proto static
Deleted 2: zmix0    inet 10.92.0.1/24 scope global zmix0
2: zmix0    inet 10.93.0.1/24 scope global zmix0
       valid_lft forever preferred_lft forever
local 10.93.0.1 dev zmix0 table local proto kernel scope host src 10.93.0.1
10.93.0.0/24 dev zmix0 proto kernel scope link src 10.93.0.1
172.30.0.0/24 via 10.93.0.2 dev zmix0 proto static
`

func TestParseKernelEventsReadsAddressAndRouteNotifications(t *testing.T) {
	events := parseKernelEvents(mixedMonitorText)
	want := []string{
		"del route 172.31.0.0/24 dev zmix0",
		"del address 10.92.0.1/24 dev zmix0",
		"add address 10.93.0.1/24 dev zmix0",
		"add route 10.93.0.0/24 dev zmix0",
		"add route 172.30.0.0/24 dev zmix0",
	}
	got := make([]string, 0, len(events))
	for _, event := range events {
		got = append(got, event.String())
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("parsed %v, want %v", got, want)
	}
}

func TestAssertSwapOrderAcceptsBothDestroysBeforeBothCreates(t *testing.T) {
	if err := assertSwapOrder(parseKernelEvents(swapMonitorText)); err != nil {
		t.Fatalf("the requirement's own order was refused: %v", err)
	}
}

func TestAssertSwapOrderRefusesMakeBeforeBreak(t *testing.T) {
	err := assertSwapOrder(parseKernelEvents(makeBeforeBreakText))
	if err == nil {
		t.Fatal("a make-before-break order was accepted, so the driver would pass on the policy it exists to catch")
	}
	if !strings.Contains(err.Error(), "held both addresses") {
		t.Fatalf("the refusal named the wrong failure: %v", err)
	}
}

func TestAssertMixedRootOrderAcceptsRemovalThenAddressThenRoute(t *testing.T) {
	if err := assertMixedRootOrder(parseKernelEvents(mixedMonitorText)); err != nil {
		t.Fatalf("the ordered sequence was refused: %v", err)
	}
}

func TestAssertMixedRootOrderRefusesARouteInstalledFirst(t *testing.T) {
	unordered := `Deleted 2: zmix0    inet 10.92.0.1/24 scope global zmix0
172.30.0.0/24 via 10.93.0.2 dev zmix0 proto static
2: zmix0    inet 10.93.0.1/24 scope global zmix0
`
	err := assertMixedRootOrder(parseKernelEvents(unordered))
	if err == nil {
		t.Fatal("a route installed before its address was accepted")
	}
	if !strings.Contains(err.Error(), "before its address") {
		t.Fatalf("the refusal named the wrong failure: %v", err)
	}
}

func TestAssertMixedRootOrderRefusesTheNewAddressArrivingFirst(t *testing.T) {
	unordered := `2: zmix0    inet 10.93.0.1/24 scope global zmix0
Deleted 2: zmix0    inet 10.92.0.1/24 scope global zmix0
172.30.0.0/24 via 10.93.0.2 dev zmix0 proto static
`
	err := assertMixedRootOrder(parseKernelEvents(unordered))
	if err == nil {
		t.Fatal("the new address arriving before the old one left was accepted, which is make-before-break")
	}
	if !strings.Contains(err.Error(), "before the old one") {
		t.Fatalf("the refusal named the wrong failure: %v", err)
	}
}
