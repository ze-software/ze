// Design: docs/architecture/api/process-protocol.md -- routing-view field ownership.

package rpc

import (
	"encoding/json"
	"math"
	"net/netip"
	"reflect"
	"strings"
	"testing"
)

func TestRouteViewResultBudgetIncludesReplyFraming(t *testing.T) {
	for _, extra := range []int{0, 1} {
		result := json.RawMessage(strings.Repeat(" ", RouteViewResultBudget()+extra))
		line := AppendResult(nil, math.MaxUint64, result)
		if got, want := len(line), MaxMessageSize+extra; got != want {
			t.Fatalf("result frame width %d, want %d", got, want)
		}
	}
}

// A selected record must retain zero-valued present metadata, IPv6 adjacency,
// equal-cost members, and repair data rather than becoming a gateway-only route.
func TestRouteViewPathRoundTrip(t *testing.T) {
	want := RouteViewEntry{
		AFI: 1, SAFI: 1, Prefix: netip.MustParsePrefix("192.0.2.0/24"), Action: RouteViewActionUpdate,
		Path: RouteViewPath{
			Protocol: "bgp", Instance: 42, Distance: 0, Metric: 9,
			NextHop: netip.MustParseAddr("fe80::1%eth0"), Interface: "eth0", OnLink: true, Weight: 3,
			Labels: []uint32{16000, 16001}, RouteType: 1, SRv6SID: netip.MustParseAddr("2001:db8::1"),
			IsEBGP: true, IsBGP: true, AIGP: 0, AIGPPresent: true, MetricRecursive: true,
			Tag: 0, HasTag: true, Level: 2, OSPFRouteType: 3,
			ECMP:          []RouteViewNextHop{{NextHop: netip.MustParseAddr("fe80::2%eth1"), Interface: "eth1", OnLink: true, Weight: 5, Labels: []uint32{17000}, SRv6SID: netip.MustParseAddr("2001:db8::2")}},
			BackupNextHop: netip.MustParseAddr("fe80::3%eth2"), BackupInterface: "eth2", BackupOnLink: true,
			BackupRepairLabels: []uint32{18000, 18001},
		},
	}
	raw, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	var got RouteViewEntry
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("forwarding metadata changed across JSON: got %+v, want %+v", got, want)
	}
	if !got.Path.HasTag || got.Path.Tag != 0 || !got.Path.AIGPPresent || got.Path.AIGP != 0 {
		t.Fatal("present zero tag or AIGP became absent")
	}
}

// A tombstone and explicit reset must remain different from an empty live page.
func TestRouteViewResetAndWithdrawHaveExplicitWireForms(t *testing.T) {
	withdraw := RouteViewEntry{AFI: 2, SAFI: 1, Prefix: netip.MustParsePrefix("2001:db8::/48"), Action: RouteViewActionWithdraw}
	raw, err := json.Marshal(withdraw)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	if _, present := fields["path"]; present {
		t.Fatalf("withdraw carries an invented path: %s", raw)
	}
	if string(fields["action"]) != `"withdraw"` {
		t.Fatalf("withdraw action changed: %s", raw)
	}
	for _, phase := range []RouteViewPhase{RouteViewPhaseSnapshot, RouteViewPhaseDelta, RouteViewPhaseReset} {
		raw, err := json.Marshal(phase)
		if err != nil {
			t.Fatal(err)
		}
		var got RouteViewPhase
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatal(err)
		}
		if got != phase {
			t.Fatalf("phase %d decoded as %d", phase, got)
		}
	}
	for _, raw := range []string{`"unknown"`, `0`, `null`} {
		phase := RouteViewPhaseDelta
		if err := json.Unmarshal([]byte(raw), &phase); err == nil {
			t.Fatalf("invalid phase %s was accepted", raw)
		}
		if phase != RouteViewPhaseDelta {
			t.Fatalf("invalid phase %s replaced existing state", raw)
		}
	}
}

func FuzzRouteViewPhase(f *testing.F) {
	for _, input := range []string{`"snapshot"`, `"delta"`, `"reset"`, `"unknown"`, `null`, `0`, `{}`, `""`} {
		f.Add([]byte(input))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > MaxMessageSize {
			return
		}
		phase := RouteViewPhaseDelta
		if err := json.Unmarshal(data, &phase); err != nil {
			if phase != RouteViewPhaseDelta {
				t.Fatal("rejected input changed the current phase")
			}
			return
		}
		encoded, err := json.Marshal(phase)
		if err != nil {
			t.Fatalf("accepted phase cannot be encoded: %v", err)
		}
		var restored RouteViewPhase
		if err := json.Unmarshal(encoded, &restored); err != nil || restored != phase {
			t.Fatalf("accepted phase failed round trip: phase %d, restored %d, error %v", phase, restored, err)
		}
	})
}
