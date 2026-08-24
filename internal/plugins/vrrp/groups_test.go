// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- VRRP config extraction + verification tests

package vrrp

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/test/sim"
)

// mkSection wraps an interface subtree into the shared `interface` config
// section the plugin receives (umbrella A-2).
func mkSection(t *testing.T, ifaceTree map[string]any) configSection {
	t.Helper()
	data, err := json.Marshal(map[string]any{"interface": ifaceTree})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return configSection{Root: configRoot, Data: string(data)}
}

// oneGroup builds an interface tree with a single vrrp group under
// ethernet/eth0/unit 0/<family>, plus optional real addresses on that unit.
func oneGroup(family, vrid string, group map[string]any, realAddrs ...string) map[string]any {
	// The list key is the operator's NAME; vrid is a leaf. Tests still pass the
	// vrid string, so derive a stable name from it.
	if _, ok := group["vrid"]; !ok {
		group["vrid"] = vrid
	}
	fam := map[string]any{"vrrp": map[string]any{"group": map[string]any{"g" + vrid: group}}}
	if len(realAddrs) > 0 {
		addrs := make([]any, len(realAddrs))
		for i, a := range realAddrs {
			addrs[i] = a
		}
		fam["address"] = addrs
	}
	return map[string]any{
		"ethernet": map[string]any{
			"eth0": map[string]any{
				"unit": map[string]any{"0": map[string]any{family: fam}},
			},
		},
	}
}

func vips(addrs ...string) []any {
	out := make([]any, len(addrs))
	for i, a := range addrs {
		out[i] = a
	}
	return out
}

// TestExtractGroupSpecsManyVIPs locks the headline capability: a group may carry
// up to 16 virtual addresses (RFC 9568 Section 5.2.9; the leaf-list is
// max-elements 16) and the engine MUST preserve their configuration order (the
// first address is the advert source identity). Every other positive test used a
// single virtual-address, which is exactly how the config-parser bracket
// leaf-list regression (fix 0b3259949) hid: a leaf-list that collapsed to one
// joined mirror still "worked" for one member, so only a multi-member group with
// an order assertion catches a leaf-list that lost its members.
//
// VALIDATES: spec-vrrp-5 AC-1 -- multiple virtual-address members, in order.
// PREVENTS: a leaf-list that silently collapses to one element, drops members,
// or reorders them.
func TestExtractGroupSpecsManyVIPs(t *testing.T) {
	const n = 16 // the RFC / YANG maximum
	want := make([]string, n)
	addrs := make([]string, n)
	for i := range n {
		a := netip.AddrFrom4([4]byte{192, 0, 2, byte(i + 1)}).String()
		want[i], addrs[i] = a, a
	}
	tree := oneGroup("ipv4", "10", map[string]any{
		"vrid":            float64(10),
		"virtual-address": vips(addrs...),
	})
	specs, err := extractGroupSpecs([]configSection{mkSection(t, tree)})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("want 1 spec, got %d", len(specs))
	}
	got := specs[0].VIPs
	if len(got) != n {
		t.Fatalf("want %d VIPs, got %d: %v", n, len(got), got)
	}
	for i := range n {
		if got[i].String() != want[i] {
			t.Errorf("VIP[%d] = %s, want %s (configuration order must be preserved)", i, got[i], want[i])
		}
	}
}

// TestExtractGroupSpecsRejectsJoinedVIP is the VRRP-side guard against the
// bracket leaf-list parser regression (fix 0b3259949): if virtual-address ever
// again arrives as ONE joined scalar ("192.0.2.1 192.0.2.2") instead of a slice,
// VRRP must reject it loudly, never silently accept a malformed single address.
// asSlice wraps a bare scalar into a one-element slice, so the joined string
// reaches ParseAddr and fails -- which is the "not an IP address" failure that
// originally surfaced the parser bug.
//
// VALIDATES: spec-vrrp-5 AC-4 -- a malformed virtual-address is rejected.
// PREVENTS: a joined-string leaf-list being accepted as a single VIP, which is
// how a collapsed multi-address group would slip through undetected.
func TestExtractGroupSpecsRejectsJoinedVIP(t *testing.T) {
	tree := oneGroup("ipv4", "10", map[string]any{
		"vrid":            float64(10),
		"virtual-address": "192.0.2.1 192.0.2.2", // bare scalar, not a slice: the bug's shape
	})
	_, err := extractGroupSpecs([]configSection{mkSection(t, tree)})
	if err == nil {
		t.Fatal("a joined-string virtual-address must be rejected, not accepted as one VIP")
	}
	if !strings.Contains(err.Error(), "not an IP address") {
		t.Errorf("want a 'not an IP address' rejection, got: %v", err)
	}
}

func TestExtractGroupSpecs(t *testing.T) {
	// All four interface types x two families, plus unrelated keys that MUST be
	// ignored (extract-only walk, umbrella R-6).
	tree := map[string]any{
		"backend":   "netlink",
		"dhcp-auto": false,
		"ethernet": map[string]any{
			"eth0": map[string]any{
				"description": "wan",
				"unit": map[string]any{
					"0": map[string]any{
						"vlan-id": float64(10),
						"ipv4": map[string]any{
							"address": vips("192.0.2.251/24"),
							"vrrp": map[string]any{"group": map[string]any{
								"uplink": map[string]any{
									"vrid":                            float64(10),
									"virtual-address":                 vips("192.0.2.1", "192.0.2.2"),
									"priority":                        float64(200),
									"preempt":                         true,
									"preempt-delay-seconds":           float64(5),
									"advertise-interval-milliseconds": float64(2000),
								},
							}},
						},
						"ipv6": map[string]any{
							"vrrp": map[string]any{"group": map[string]any{
								// Same VRID as the IPv4 group above: independent
								// namespaces per family (RFC 9568 Section 1.2).
								"uplink6": map[string]any{
									"vrid":            float64(10),
									"virtual-address": vips("fe80::1", "2001:db8::1"),
								},
							}},
						},
					},
				},
			},
		},
		"veth":   oneTypeGroup("veth", "veth0"),
		"bridge": oneTypeGroup("bridge", "br0"),
		"dummy":  oneTypeGroup("dummy", "dummy0"),
	}
	specs, err := extractGroupSpecs([]configSection{mkSection(t, tree)})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	// eth0(v4+v6) + veth0(v4) + br0(v4) + dummy0(v4) = 5 specs.
	if len(specs) != 5 {
		t.Fatalf("want 5 specs, got %d: %+v", len(specs), specs)
	}

	var v4, v6 *GroupSpec
	for i := range specs {
		if specs[i].Interface == "eth0" && specs[i].Family == familyIPv4 {
			v4 = &specs[i]
		}
		if specs[i].Interface == "eth0" && specs[i].Family == familyIPv6 {
			v6 = &specs[i]
		}
	}
	if v4 == nil || v6 == nil {
		t.Fatalf("missing eth0 v4/v6 specs: %+v", specs)
	}
	if v4.IfType != "ethernet" || v4.Unit != "0" || v4.VRID != 10 {
		t.Errorf("v4 key wrong: %+v", v4)
	}
	if len(v4.VIPs) != 2 || v4.VIPs[0].String() != "192.0.2.1" || v4.VIPs[1].String() != "192.0.2.2" {
		t.Errorf("v4 VIPs wrong (order matters): %v", v4.VIPs)
	}
	if v4.Priority != 200 || v4.PreemptDelaySeconds != 5 || v4.AdvertIntervalMs != 2000 || v4.Version != 3 {
		t.Errorf("v4 leaves wrong: %+v", v4)
	}
	if v6.Version != 3 {
		t.Errorf("v6 must default version 3, got %d", v6.Version)
	}
	// v6 defaults: priority 100, preempt true, interval 1000.
	if v6.Priority != 100 || !v6.Preempt || v6.AdvertIntervalMs != 1000 {
		t.Errorf("v6 defaults wrong: %+v", v6)
	}
}

func oneTypeGroup(_ /*ifType*/, name string) map[string]any {
	return map[string]any{
		name: map[string]any{
			"unit": map[string]any{
				"0": map[string]any{
					"ipv4": map[string]any{
						"vrrp": map[string]any{"group": map[string]any{
							"lan": map[string]any{"vrid": float64(1), "virtual-address": vips("10.0.0.1")},
						}},
					},
				},
			},
		},
	}
}

func TestExtractGroupSpecsEmpty(t *testing.T) {
	tree := map[string]any{
		"ethernet": map[string]any{
			"eth0": map[string]any{
				"unit": map[string]any{
					"0": map[string]any{"ipv4": map[string]any{"address": vips("10.0.0.2/24")}},
				},
			},
		},
	}
	specs, err := extractGroupSpecs([]configSection{mkSection(t, tree)})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(specs) != 0 {
		t.Fatalf("want 0 specs for vrrp-free config, got %d", len(specs))
	}
}

func TestValidateIntervalVersion(t *testing.T) {
	cases := []struct {
		name    string
		family  string
		group   map[string]any
		wantErr bool
	}{
		{"v3 min 10", familyIPv4, map[string]any{"virtual-address": vips("10.0.0.1"), "advertise-interval-milliseconds": float64(10)}, false},
		{"v3 max 40950", familyIPv4, map[string]any{"virtual-address": vips("10.0.0.1"), "advertise-interval-milliseconds": float64(40950)}, false},
		{"v3 not multiple of 10 (1005)", familyIPv4, map[string]any{"virtual-address": vips("10.0.0.1"), "advertise-interval-milliseconds": float64(1005)}, true},
		{"v3 over range 50000", familyIPv4, map[string]any{"virtual-address": vips("10.0.0.1"), "advertise-interval-milliseconds": float64(50000)}, true},
		{"v2 whole second 2000", familyIPv4, map[string]any{"version": "2", "virtual-address": vips("10.0.0.1"), "advertise-interval-milliseconds": float64(2000)}, false},
		{"v2 max 255000", familyIPv4, map[string]any{"version": "2", "virtual-address": vips("10.0.0.1"), "advertise-interval-milliseconds": float64(255000)}, false},
		{"v2 non-whole 1500", familyIPv4, map[string]any{"version": "2", "virtual-address": vips("10.0.0.1"), "advertise-interval-milliseconds": float64(1500)}, true},
		{"v2 below 500", familyIPv4, map[string]any{"version": "2", "virtual-address": vips("10.0.0.1"), "advertise-interval-milliseconds": float64(500)}, true},
		{"v2 over 256000", familyIPv4, map[string]any{"version": "2", "virtual-address": vips("10.0.0.1"), "advertise-interval-milliseconds": float64(256000)}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			specs, err := extractGroupSpecs([]configSection{mkSection(t, oneGroup(tc.family, "5", tc.group))})
			if err != nil {
				t.Fatalf("extract: %v", err)
			}
			err = validateGroups(specs, "netlink")
			if tc.wantErr && err == nil {
				t.Fatalf("want rejection, got none")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("want accept, got %v", err)
			}
		})
	}
}

// TestBoundaryVRID covers the VRID range (spec-vrrp-5 Boundary Tests). 0 and
// 256 are rejected by extraction; 1 and 255 are the last valid values.
func TestBoundaryVRID(t *testing.T) {
	cases := []struct {
		vrid    string
		wantErr bool
	}{
		{"0", true},    // invalid below
		{"1", false},   // first valid
		{"255", false}, // last valid
		{"256", true},  // invalid above
	}
	for _, tc := range cases {
		t.Run(tc.vrid, func(t *testing.T) {
			tree := oneGroup(familyIPv4, tc.vrid, map[string]any{"virtual-address": vips("10.0.0.1")})
			specs, err := extractGroupSpecs([]configSection{mkSection(t, tree)})
			if tc.wantErr {
				if err == nil {
					t.Fatalf("vrid %s must be rejected", tc.vrid)
				}
				return
			}
			if err != nil {
				t.Fatalf("vrid %s must be accepted: %v", tc.vrid, err)
			}
			if err := validateGroups(specs, backendNetlink); err != nil {
				t.Fatalf("vrid %s must validate: %v", tc.vrid, err)
			}
		})
	}
}

// TestBoundaryPriority covers the priority range: 1..254 configurable, 0 and
// 255 rejected (255 is the owner's, assigned by ze -- RFC 9568 Section 5.2.4).
func TestBoundaryPriority(t *testing.T) {
	// RFC requirement: RFC3768-5.3.4-2 positive -- a Backup priority in 1..254 is accepted (validateGroup groups.go:503).
	// RFC requirement: RFC3768-5.3.4-2 negative -- priority 0 (resignation) and 255 (owner-reserved) are rejected, so a Backup can never be configured outside 1..254 (groups.go:503).
	// RFC requirement: RFC9568-5.2.4-2 positive -- a VRRP Router backing up a Virtual Router is configured with a priority in 1..254 (validateGroup groups.go:503)
	// RFC requirement: RFC9568-5.2.4-2 negative -- priority 0 (the relinquishing value) and 255 (the address owner's) are rejected, so a Backup can never run outside 1..254 (groups.go:503).
	cases := []struct {
		priority float64
		wantErr  bool
	}{
		{0, true},    // invalid below
		{1, false},   // first valid
		{254, false}, // last valid
		{255, true},  // reserved for the owner
	}
	for _, tc := range cases {
		t.Run(textbufUint(tc.priority), func(t *testing.T) {
			group := map[string]any{"virtual-address": vips("10.0.0.1"), "priority": tc.priority}
			specs, err := extractGroupSpecs([]configSection{mkSection(t, oneGroup(familyIPv4, "5", group))})
			if err != nil {
				t.Fatalf("extract: %v", err)
			}
			err = validateGroups(specs, backendNetlink)
			if tc.wantErr && err == nil {
				t.Fatalf("priority %v must be rejected", tc.priority)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("priority %v must be accepted: %v", tc.priority, err)
			}
		})
	}
}

// TestBoundaryVIPCount covers the 1..16 virtual-address range.
func TestBoundaryVIPCount(t *testing.T) {
	addrs := make([]string, 0, 17)
	for i := range 17 {
		addrs = append(addrs, netip.AddrFrom4([4]byte{192, 0, 2, byte(i + 1)}).String())
	}
	cases := []struct {
		name    string
		count   int
		wantErr bool
	}{
		{"zero", 0, true},       // invalid below
		{"one", 1, false},       // first valid
		{"sixteen", 16, false},  // last valid
		{"seventeen", 17, true}, // invalid above
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			group := map[string]any{"virtual-address": vips(addrs[:tc.count]...)}
			specs, err := extractGroupSpecs([]configSection{mkSection(t, oneGroup(familyIPv4, "5", group))})
			if err != nil {
				t.Fatalf("extract: %v", err)
			}
			err = validateGroups(specs, backendNetlink)
			if tc.wantErr && err == nil {
				t.Fatalf("%d VIPs must be rejected", tc.count)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("%d VIPs must be accepted: %v", tc.count, err)
			}
		})
	}
}

// TestBoundaryPreemptDelay covers the preempt-delay-seconds range 0..3600.
func TestBoundaryPreemptDelay(t *testing.T) {
	cases := []struct {
		delay   float64
		wantErr bool
	}{
		{0, false},    // first valid (default)
		{3600, false}, // last valid
		{3601, true},  // invalid above
	}
	for _, tc := range cases {
		t.Run(textbufUint(tc.delay), func(t *testing.T) {
			group := map[string]any{"virtual-address": vips("10.0.0.1"), "preempt-delay-seconds": tc.delay}
			_, err := extractGroupSpecs([]configSection{mkSection(t, oneGroup(familyIPv4, "5", group))})
			if tc.wantErr && err == nil {
				t.Fatalf("preempt-delay-seconds %v must be rejected", tc.delay)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("preempt-delay-seconds %v must be accepted: %v", tc.delay, err)
			}
		})
	}
}

func textbufUint(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// TestLeafShapesFromTextConfig pins the SHAPES a leaf can arrive in.
//
// The config tree delivers a leaf as whatever the producer encoded: the text
// parser stores scalars as strings, while a JSON producer sends real bools and
// numbers. A type assertion that accepts only one shape drops the other
// SILENTLY -- which is exactly how `version 2; accept-mode true;` slipped past
// the verifier in a live `ze config validate` run: accept-mode arrived as the
// string "true", the .(bool) assertion failed, AcceptMode stayed false, and the
// v2+accept-mode rejection never fired.
//
// VALIDATES: every leaf accepts both the string and the native shape.
// PREVENTS: a silently-ignored leaf, i.e. config an operator wrote having no
// effect and no error.
func TestLeafShapesFromTextConfig(t *testing.T) {
	cases := []struct {
		name  string
		group map[string]any
		check func(t *testing.T, g GroupSpec)
	}{
		{
			name:  "accept-mode as string",
			group: map[string]any{"virtual-address": vips("10.0.0.1"), "accept-mode": "true"},
			check: func(t *testing.T, g GroupSpec) {
				if !g.AcceptMode {
					t.Error("accept-mode \"true\" (string) was dropped")
				}
			},
		},
		{
			name:  "accept-mode as bool",
			group: map[string]any{"virtual-address": vips("10.0.0.1"), "accept-mode": true},
			check: func(t *testing.T, g GroupSpec) {
				if !g.AcceptMode {
					t.Error("accept-mode true (bool) was dropped")
				}
			},
		},
		{
			name:  "preempt false as string",
			group: map[string]any{"virtual-address": vips("10.0.0.1"), "preempt": "false"},
			check: func(t *testing.T, g GroupSpec) {
				if g.Preempt {
					t.Error("preempt \"false\" (string) was dropped; it must override the default true")
				}
			},
		},
		{
			name:  "preempt false as bool",
			group: map[string]any{"virtual-address": vips("10.0.0.1"), "preempt": false},
			check: func(t *testing.T, g GroupSpec) {
				if g.Preempt {
					t.Error("preempt false (bool) was dropped")
				}
			},
		},
		{
			name:  "priority as string",
			group: map[string]any{"virtual-address": vips("10.0.0.1"), "priority": "200"},
			check: func(t *testing.T, g GroupSpec) {
				if g.Priority != 200 {
					t.Errorf("priority = %d, want 200 from the string form", g.Priority)
				}
			},
		},
		{
			name:  "version as number",
			group: map[string]any{"virtual-address": vips("10.0.0.1"), "version": float64(2)},
			check: func(t *testing.T, g GroupSpec) {
				if g.Version != versionV2 {
					t.Errorf("version = %d, want 2 from the numeric form", g.Version)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			specs, err := extractGroupSpecs([]configSection{mkSection(t, oneGroup(familyIPv4, "5", tc.group))})
			if err != nil {
				t.Fatalf("extract: %v", err)
			}
			if len(specs) != 1 {
				t.Fatalf("specs = %d, want 1", len(specs))
			}
			tc.check(t, specs[0])
		})
	}
}

// TestValidateRejectsUnparsableLeaf proves a leaf whose value cannot be
// interpreted is an ERROR, never a silent default.
func TestValidateRejectsUnparsableLeaf(t *testing.T) {
	group := map[string]any{"virtual-address": vips("10.0.0.1"), "accept-mode": "yes-please"}
	if _, err := extractGroupSpecs([]configSection{mkSection(t, oneGroup(familyIPv4, "5", group))}); err == nil {
		t.Fatal("an uninterpretable accept-mode must be rejected, not defaulted")
	}
}

func TestValidateAcceptModeVersion(t *testing.T) {
	group := map[string]any{"version": "2", "virtual-address": vips("10.0.0.1"), "advertise-interval-milliseconds": float64(1000), "accept-mode": true}
	specs, err := extractGroupSpecs([]configSection{mkSection(t, oneGroup(familyIPv4, "5", group))})
	if err != nil {
		t.Fatal(err)
	}
	if err := validateGroups(specs, "netlink"); err == nil {
		t.Fatal("accept-mode true with version 2 must be rejected")
	}
}

// TestValidateIPv6LinkLocal covers the transmit side of the link-local rule: a
// group whose advertisements would not lead with the Virtual Router's IPv6
// link-local address is refused at config time, so no such advert is ever sent.
//
// RFC requirement: RFC9568-5.2.9-1 positive -- an IPv6 group whose FIRST virtual-address is the link-local one validates, so its advertisements lead with that address (validateGroup groups.go:524)
// RFC requirement: RFC9568-5.2.9-1 negative -- an IPv6 group whose first virtual-address is global is rejected, so ze cannot transmit an advertisement whose first address is not the link-local (groups.go:524).
func TestValidateIPv6LinkLocal(t *testing.T) {
	// First VIP not link-local -> reject.
	bad := oneGroup(familyIPv6, "5", map[string]any{"virtual-address": vips("2001:db8::1", "fe80::1")})
	specs, _ := extractGroupSpecs([]configSection{mkSection(t, bad)})
	if err := validateGroups(specs, "netlink"); err == nil {
		t.Fatal("ipv6 group whose FIRST vip is not link-local must be rejected")
	}
	// First VIP link-local -> accept.
	good := oneGroup(familyIPv6, "5", map[string]any{"virtual-address": vips("fe80::1", "2001:db8::1")})
	specs, _ = extractGroupSpecs([]configSection{mkSection(t, good)})
	if err := validateGroups(specs, "netlink"); err != nil {
		t.Fatalf("link-local-first ipv6 group must validate: %v", err)
	}
}

// TestValidateVIPFamilyMatchesGroupFamily proves a group only ever advertises
// addresses of its own family, which is the family of the IPvX header its
// advertisements are carried in: an IPv4 group lives under the unit's ipv4
// container and sends over IPv4, an IPv6 group under ipv6 and over IPv6.
//
// RFC requirement: RFC9568-5.2.9-2 positive -- a group whose virtual addresses match its family (and therefore the VRRP packet's IPvX header) validates and runs (validateGroup groups.go:513-518)
// RFC requirement: RFC9568-5.2.9-2 negative -- an IPv6 address configured on an IPv4 group (and an IPv4 address on an IPv6 group) is rejected, so an advertisement can never carry an address of the other family (groups.go:513-518).
func TestValidateVIPFamilyMatchesGroupFamily(t *testing.T) {
	cases := []struct {
		name    string
		family  string
		vips    []any
		wantErr bool
	}{
		{"ipv4-group-ipv4-vip", familyIPv4, vips("192.0.2.1"), false},
		{"ipv4-group-ipv6-vip", familyIPv4, vips("2001:db8::1"), true},
		{"ipv6-group-ipv6-vip", familyIPv6, vips("fe80::1"), false},
		{"ipv6-group-ipv4-vip", familyIPv6, vips("192.0.2.1"), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := oneGroup(tc.family, "5", map[string]any{"virtual-address": tc.vips})
			specs, err := extractGroupSpecs([]configSection{mkSection(t, tree)})
			if err != nil {
				t.Fatalf("extract: %v", err)
			}
			err = validateGroups(specs, backendNetlink)
			if tc.wantErr && err == nil {
				t.Fatalf("%s: a virtual-address of the other family must be rejected", tc.name)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("%s: a same-family virtual-address must be accepted: %v", tc.name, err)
			}
		})
	}
}

func TestValidateDuplicateVIP(t *testing.T) {
	// Two groups on the same unit+family sharing a VIP -> reject.
	tree := map[string]any{
		"ethernet": map[string]any{
			"eth0": map[string]any{
				"unit": map[string]any{
					"0": map[string]any{
						"ipv4": map[string]any{"vrrp": map[string]any{"group": map[string]any{
							"primary":   map[string]any{"vrid": float64(10), "virtual-address": vips("192.0.2.1")},
							"secondary": map[string]any{"vrid": float64(20), "virtual-address": vips("192.0.2.1")},
						}}},
					},
				},
			},
		},
	}
	specs, _ := extractGroupSpecs([]configSection{mkSection(t, tree)})
	if err := validateGroups(specs, "netlink"); err == nil {
		t.Fatal("duplicate VIP across two groups on one unit+family must be rejected")
	}

	// Same VIP on different units -> accepted.
	tree2 := map[string]any{
		"ethernet": map[string]any{
			"eth0": map[string]any{
				"unit": map[string]any{
					"0": map[string]any{"ipv4": map[string]any{"vrrp": map[string]any{"group": map[string]any{
						"a": map[string]any{"vrid": float64(10), "virtual-address": vips("192.0.2.1")},
					}}}},
					"1": map[string]any{"ipv4": map[string]any{"vrrp": map[string]any{"group": map[string]any{
						"b": map[string]any{"vrid": float64(20), "virtual-address": vips("192.0.2.1")},
					}}}},
				},
			},
		},
	}
	specs2, err := extractGroupSpecs([]configSection{mkSection(t, tree2)})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if err := validateGroups(specs2, "netlink"); err != nil {
		t.Fatalf("same VIP on different units must be allowed: %v", err)
	}
}

// TestValidateDuplicateVRID proves two named groups cannot claim one VRID on the
// same unit and family.
//
// This rule only became possible (and necessary) when the group key became a
// NAME: with the VRID as the key it was unrepresentable. Two groups sharing a
// VRID would be one virtual router to every peer on the link, while ze ran two
// state machines fighting over one identity, one virtual MAC, and one macvlan
// name (RFC 9568 Section 1.2).
func TestValidateDuplicateVRID(t *testing.T) {
	tree := map[string]any{
		"ethernet": map[string]any{
			"eth0": map[string]any{
				"unit": map[string]any{
					"0": map[string]any{"ipv4": map[string]any{"vrrp": map[string]any{"group": map[string]any{
						"first":  map[string]any{"vrid": float64(10), "virtual-address": vips("192.0.2.1")},
						"second": map[string]any{"vrid": float64(10), "virtual-address": vips("192.0.2.2")},
					}}}},
				},
			},
		},
	}
	specs, err := extractGroupSpecs([]configSection{mkSection(t, tree)})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	err = validateGroups(specs, backendNetlink)
	if err == nil {
		t.Fatal("two groups with the same vrid on one unit+family must be rejected")
	}
	if !strings.Contains(err.Error(), "vrid 10") {
		t.Errorf("error must name the conflicting vrid, got %q", err)
	}

	// The SAME vrid across families is legal: independent virtual routers.
	ok := map[string]any{
		"ethernet": map[string]any{
			"eth0": map[string]any{
				"unit": map[string]any{
					"0": map[string]any{
						"ipv4": map[string]any{"vrrp": map[string]any{"group": map[string]any{
							"v4": map[string]any{"vrid": float64(10), "virtual-address": vips("192.0.2.1")},
						}}},
						"ipv6": map[string]any{"vrrp": map[string]any{"group": map[string]any{
							"v6": map[string]any{"vrid": float64(10), "virtual-address": vips("fe80::1")},
						}}},
					},
				},
			},
		},
	}
	specs2, err := extractGroupSpecs([]configSection{mkSection(t, ok)})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if err := validateGroups(specs2, backendNetlink); err != nil {
		t.Fatalf("the same vrid in different families must be allowed: %v", err)
	}
}

// TestUnitDeviceResolution proves extraction records each unit's 802.1Q tag, so
// the device the group ends up on is the UNIT's device rather than the bare
// parent.
//
// A unit with a vlan-id lives on a sub-interface (iface names it
// "<parent>.<vlan-id>", config_apply.go unitOSName). Binding sockets and the
// macvlan to the bare parent instead would advertise into the wrong broadcast
// domain -- the group would simply never see its peers, while looking
// configured. It would also collapse two units of one interface onto a single
// transport InstanceKey and a single metric series.
//
// The tag is carried, never pre-composed into a device name: the device it
// hangs off is not known until the interface's hardware selector is answered,
// which engine.apply does (TestVRRPParentComposesVLANOnTheResolvedDevice).
//
// VALIDATES: VLANID per unit.
// PREVENTS: VRRP on a VLAN unit silently running on the wrong link.
func TestUnitDeviceResolution(t *testing.T) {
	tree := map[string]any{
		"ethernet": map[string]any{
			"eth0": map[string]any{
				"unit": map[string]any{
					// Plain unit: the device IS the interface.
					"0": map[string]any{"ipv4": map[string]any{"vrrp": map[string]any{"group": map[string]any{
						"base": map[string]any{"vrid": float64(10), "virtual-address": vips("192.0.2.1")},
					}}}},
					// VLAN unit: the device is the sub-interface.
					"1": map[string]any{
						"vlan-id": float64(100),
						"ipv4": map[string]any{"vrrp": map[string]any{"group": map[string]any{
							"tagged": map[string]any{"vrid": float64(10), "virtual-address": vips("198.51.100.1")},
						}}},
					},
				},
			},
		},
	}
	specs, err := extractGroupSpecs([]configSection{mkSection(t, tree)})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(specs) != 2 {
		t.Fatalf("specs = %d, want 2", len(specs))
	}
	tags := map[string]uint16{}
	for _, s := range specs {
		tags[s.Name] = s.VLANID
		if s.ParentDevice != "" {
			t.Errorf("group %q left extraction with ParentDevice %q; extraction is pure and binds no device",
				s.Name, s.ParentDevice)
		}
	}
	if tags["base"] != 0 {
		t.Errorf("plain unit VLANID = %d, want 0", tags["base"])
	}
	if tags["tagged"] != 100 {
		t.Errorf("vlan unit VLANID = %d, want 100", tags["tagged"])
	}
	// The same vrid on two DIFFERENT units of one interface is legal: they are
	// different links, so different virtual routers.
	if err := validateGroups(specs, backendNetlink); err != nil {
		t.Fatalf("same vrid on different units must be allowed: %v", err)
	}
}

// TestExtractRequiresVRID proves a group without a vrid is refused rather than
// defaulted: the name is a local label, so nothing else can supply the virtual
// router's identity.
func TestExtractRequiresVRID(t *testing.T) {
	tree := map[string]any{
		"ethernet": map[string]any{
			"eth0": map[string]any{
				"unit": map[string]any{
					"0": map[string]any{"ipv4": map[string]any{"vrrp": map[string]any{"group": map[string]any{
						"nameless": map[string]any{"virtual-address": vips("192.0.2.1")},
					}}}},
				},
			},
		},
	}
	if _, err := extractGroupSpecs([]configSection{mkSection(t, tree)}); err == nil {
		t.Fatal("a group without a vrid must be rejected")
	}
}

func TestOwnerAutoDetection(t *testing.T) {
	// RFC requirement: RFC3768-5.3.4-1 positive -- the router that owns the virtual address runs with priority 255 (EffectivePriority groups.go:141).
	// RFC requirement: RFC3768-5.3.4-1 negative -- a non-owner is NOT forced to 255; it keeps its configured Backup priority, so 255 marks only the address owner (groups.go:141).
	// RFC requirement: RFC9568-5.2.4-1 positive -- the VRRP Router that owns the Virtual Router's IPvX address runs with priority 255, whatever priority was configured (EffectivePriority groups.go:141)
	// RFC requirement: RFC9568-5.2.4-1 negative -- a non-owner is NOT raised to 255; it keeps its configured Backup priority, so 255 marks the address owner alone (groups.go:141).
	// VIP equals a real address on the same unit+family -> owner.
	owner := oneGroup(familyIPv4, "5", map[string]any{"virtual-address": vips("192.0.2.10"), "priority": float64(120)}, "192.0.2.10/24")
	specs, err := extractGroupSpecs([]configSection{mkSection(t, owner)})
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 1 || !specs[0].IsOwner {
		t.Fatalf("VIP equal to real unit address must mark owner: %+v", specs)
	}
	if specs[0].EffectivePriority() != 255 {
		t.Errorf("owner effective priority must be 255, got %d", specs[0].EffectivePriority())
	}
	if !specs[0].EffectiveAcceptMode() {
		t.Errorf("owner accept-mode must be forced true")
	}

	// Near-miss: same address but a different unit/family -> not owner.
	nonOwner := oneGroup(familyIPv4, "5", map[string]any{"virtual-address": vips("192.0.2.99")}, "192.0.2.10/24")
	specs2, _ := extractGroupSpecs([]configSection{mkSection(t, nonOwner)})
	if specs2[0].IsOwner {
		t.Fatalf("non-matching VIP must not be owner: %+v", specs2)
	}
	if specs2[0].EffectivePriority() != 100 {
		t.Errorf("non-owner effective priority must equal configured, got %d", specs2[0].EffectivePriority())
	}
}

func TestVerifyRejectsVPPBackend(t *testing.T) {
	group := oneGroup(familyIPv4, "5", map[string]any{"virtual-address": vips("10.0.0.1")})
	specs, _ := extractGroupSpecs([]configSection{mkSection(t, group)})
	err := validateGroups(specs, backendVPP)
	if err == nil || !strings.Contains(err.Error(), "backend") {
		t.Fatalf("vpp backend with a group must be rejected naming the backend: %v", err)
	}
	// vpp + zero groups -> ok (idle).
	if err := validateGroups(nil, backendVPP); err != nil {
		t.Fatalf("vpp backend with no groups must be accepted: %v", err)
	}
}

func TestIfaceBackend(t *testing.T) {
	s := mkSection(t, map[string]any{"backend": "vpp"})
	if got := ifaceBackend([]configSection{s}); got != "vpp" {
		t.Fatalf("want vpp, got %q", got)
	}
	// Default when absent.
	s2 := mkSection(t, map[string]any{})
	if got := ifaceBackend([]configSection{s2}); got != "netlink" {
		t.Fatalf("want netlink default, got %q", got)
	}
}

// --- Parent-device binding (spec-fixit-vrrp-parent-ignores-the-selector) -----
//
// An operator pins an interface to its hardware with `mac/match` or aliases it
// with `os-name`, and the virtual router must live on the device that selector
// answered. These tests drive engine.apply, not the helper, because apply is
// where ParentDevice is bound and where the kernel-facing values (macvlan
// parent, per-device sysctls, transport parent) are handed out. Asserting on
// the helper alone would leave a sink free to take the logical name.

// applyTree extracts a config tree, binds it through engine.apply under the
// given selector answer, and returns the engine plus the recorded platform
// calls. A nil resolve means "no selector configured": every name is its own
// kernel device.
func applyTree(t *testing.T, tree map[string]any, resolve deviceResolver) (*engine, *fakePlatform) {
	t.Helper()
	specs, err := extractGroupSpecs([]configSection{mkSection(t, tree)})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	p := newFakePlatform()
	if resolve != nil {
		p.resolveParent = resolve
	}
	f := &fakeDeps{}
	eng := newEngine(sim.NewFakeClock(time.Unix(0, 0).UTC()), p.platform(), f.deps())
	t.Cleanup(eng.stopAll)
	eng.apply(specs)
	return eng, p
}

// boundDevice returns the ParentDevice of the running instance for group name,
// or "" when no instance was created for it.
func boundDevice(eng *engine, name string) string {
	eng.mu.Lock()
	defer eng.mu.Unlock()
	for _, in := range eng.instances {
		if in.spec.Name == name {
			return in.spec.ParentDevice
		}
	}
	return ""
}

// identityDevice is the resolver for a configuration with no hardware selector:
// every logical name is its own kernel device. It is the fake platform's
// default, so a test that configures no selector reads back the name it wrote.
func identityDevice(name string) (string, error) { return name, nil }

// selectedIface is the logical name every selector test configures. It is
// deliberately not a kernel device name: the whole point is that the operator's
// label and the NIC's name are two different strings.
const selectedIface = "wan"

// selects answers selectedIface with device and refuses every other name the
// way iface.ResolveDevice refuses an unbound selector.
func selects(device string) deviceResolver {
	return func(n string) (string, error) {
		if n == selectedIface {
			return device, nil
		}
		return "", fmt.Errorf("iface: interface %q is bound to hardware by a selector that resolves to no device", n)
	}
}

// vrrpOn builds an interface tree with one vrrp group on
// ethernet/selectedIface/unit <unit>/ipv4, optionally VLAN-tagged.
func vrrpOn(unit string, vlanID int, group string, vrid int) map[string]any {
	unitCfg := map[string]any{
		"ipv4": map[string]any{"vrrp": map[string]any{"group": map[string]any{
			group: map[string]any{"vrid": float64(vrid), "virtual-address": vips("192.0.2.1")},
		}}},
	}
	if vlanID > 0 {
		unitCfg["vlan-id"] = float64(vlanID)
	}
	return map[string]any{"ethernet": map[string]any{selectedIface: map[string]any{
		"unit": map[string]any{unit: unitCfg},
	}}}
}

// TestVRRPParentTakesTheResolvedDevice proves the virtual router is built on
// the device the interface's hardware selector answered, not on the logical
// name the operator chose for it.
//
// RFC 9568 Section 7.3 puts the virtual MAC on the interface the virtual router
// protects. An operator who pinned "wan" to a NIC by permanent MAC asked for
// the virtual router on THAT NIC, so a macvlan built on whatever else wears the
// name "wan" is not a degraded VRRP -- it is VRRP for the wrong link, and the
// protected address fails over to a router that cannot carry it.
//
// VALIDATES: AC-1 (mac/match), AC-2 (os-name -- one resolver answers both, the
// selector form is iface's business and never reaches vrrp).
// PREVENTS: the virtual MAC landing on a device that merely shares the
// configured interface's name.
func TestVRRPParentTakesTheResolvedDevice(t *testing.T) {
	eng, p := applyTree(t, vrrpOn("0", 0, "lab", 10), selects("eth3"))

	if got := boundDevice(eng, "lab"); got != "eth3" {
		t.Fatalf("ParentDevice = %q, want eth3 (the device the selector answered)", got)
	}
	if len(p.parents) != 1 {
		t.Fatalf("created %d macvlans, want 1", len(p.parents))
	}
	for dev, parent := range p.parents {
		if parent != "eth3" {
			t.Errorf("macvlan %s created on parent %q, want eth3", dev, parent)
		}
	}
	if len(p.applied) != 1 || !strings.HasPrefix(p.applied[0], "eth3/") {
		t.Errorf("dataplane sysctls applied to %v, want the eth3 device", p.applied)
	}
	if len(p.opened) != 1 || p.opened[0].Parent != "eth3" {
		t.Errorf("transport opened on %v, want parent eth3", p.opened)
	}
}

// TestVRRPParentComposesVLANOnTheResolvedDevice proves the VLAN suffix hangs
// off the resolved device, not the logical name.
//
// Both interface backends compose a VLAN netdev name from the parent they are
// handed (iface config_apply.go unitOSName), so a VLAN on selected hardware IS
// named after that hardware. "wan.100" names a device the kernel does not have.
//
// VALIDATES: AC-3.
// PREVENTS: a VLAN-tagged group failing to find its device, or finding a
// same-named one on other hardware.
func TestVRRPParentComposesVLANOnTheResolvedDevice(t *testing.T) {
	eng, p := applyTree(t, vrrpOn("1", 100, "tagged", 11), selects("eth3"))

	if got := boundDevice(eng, "tagged"); got != "eth3.100" {
		t.Fatalf("ParentDevice = %q, want eth3.100 (the tag on the RESOLVED device)", got)
	}
	if len(p.opened) != 1 || p.opened[0].Parent != "eth3.100" {
		t.Errorf("transport opened on %v, want parent eth3.100", p.opened)
	}
}

// TestVRRPParentUnselectedInterfaceIsUnchanged pins the common configuration:
// an interface with no hardware selector IS its own kernel device, tagged or
// not, exactly as before this binding existed.
//
// This is the regression guard on the fix. A resolution that made every name
// depend on the hardware being present would refuse a virtual router on a
// device Ze itself creates, and would break every deployment that never
// configured a selector.
//
// VALIDATES: AC-4.
// PREVENTS: the fix changing behavior for interfaces that carry no selector.
func TestVRRPParentUnselectedInterfaceIsUnchanged(t *testing.T) {
	tree := map[string]any{"ethernet": map[string]any{"eth0": map[string]any{"unit": map[string]any{
		"0": map[string]any{"ipv4": map[string]any{"vrrp": map[string]any{"group": map[string]any{
			"base": map[string]any{"vrid": float64(10), "virtual-address": vips("192.0.2.1")},
		}}}},
		"1": map[string]any{
			"vlan-id": float64(100),
			"ipv4": map[string]any{"vrrp": map[string]any{"group": map[string]any{
				"tagged": map[string]any{"vrid": float64(11), "virtual-address": vips("198.51.100.1")},
			}}},
		},
	}}}}
	eng, _ := applyTree(t, tree, identityDevice)

	if got := boundDevice(eng, "base"); got != "eth0" {
		t.Errorf("plain unit ParentDevice = %q, want eth0", got)
	}
	if got := boundDevice(eng, "tagged"); got != "eth0.100" {
		t.Errorf("vlan unit ParentDevice = %q, want eth0.100", got)
	}
}

// TestVRRPGroupRefusesAnUnansweredSelector proves a group whose selector
// answers no device, or more than one, runs on NO device at all.
//
// Failing closed is the whole point (ai/rules/evidence.md): the tempting
// fallback is the logical name, and that is precisely how a virtual MAC reaches
// hardware the operator did not name. Nothing distinguishes two devices
// carrying one MAC either, so picking one is a guess about which physical port
// carries the protected address.
//
// VALIDATES: AC-5, both the zero-device and the several-device case.
// PREVENTS: an unbound selector degrading into "use the name".
func TestVRRPGroupRefusesAnUnansweredSelector(t *testing.T) {
	refusals := map[string]deviceResolver{
		"no device answers": func(string) (string, error) {
			return "", fmt.Errorf("iface: no device with MAC 02:00:00:00:be:99 for logical interface %q", selectedIface)
		},
		"several devices answer": func(string) (string, error) {
			return "", fmt.Errorf("iface: MAC 02:00:00:00:be:99 for logical interface %q is carried by 2 devices (eth3, eth4)", selectedIface)
		},
	}
	for name, resolve := range refusals {
		t.Run(name, func(t *testing.T) {
			eng, p := applyTree(t, vrrpOn("0", 0, "lab", 10), resolve)

			eng.mu.Lock()
			running := len(eng.instances)
			eng.mu.Unlock()
			if running != 0 {
				t.Errorf("instances = %d, want 0: an unbound selector must run no virtual router", running)
			}
			if len(p.devices) != 0 {
				t.Errorf("macvlans created on %v, want none on any device", p.devices)
			}
			if len(p.applied) != 0 {
				t.Errorf("dataplane sysctls applied to %v, want none", p.applied)
			}
			if len(p.opened) != 0 {
				t.Errorf("transports opened on %v, want none", p.opened)
			}
		})
	}
}

// TestVRRPGroupSurvivesAnUnresolvedSelector proves a RUNNING virtual router is
// left where it is when its selector cannot be answered, rather than torn down.
//
// This is the availability half of the binding, and it is worth more than the
// defect the binding fixes. "Could not resolve" is not "the device is gone":
// iface.ResolveDevice refuses on ANY Resolve failure once the name carries a
// selector, one failed ListInterfaces inside matchByMAC included, and the
// resolver cache cannot soften it because setMapping drops the cache on every
// iface apply. A teardown here resigns the FSM with a Priority-0 advertisement
// and removes the VIP, and apply runs only on a config event -- so one transient
// netlink read during an UNRELATED commit would fail a live master over, and it
// would stay down until somebody committed again.
//
// The device actually disappearing is handled elsewhere and handled better:
// parentReady stops the virtual router and watchParent starts it again when the
// device returns, with no commit and no macvlan churn.
//
// VALIDATES: AC-5 does not extend to destroying a running virtual router.
// PREVENTS: a transient resolver failure failing a master over, permanently.
func TestVRRPGroupSurvivesAnUnresolvedSelector(t *testing.T) {
	tree := vrrpOn("0", 0, "lab", 10)
	specs, err := extractGroupSpecs([]configSection{mkSection(t, tree)})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	p := newFakePlatform()
	p.resolveParent = selects("eth3")
	f := &fakeDeps{}
	eng := newEngine(sim.NewFakeClock(time.Unix(0, 0).UTC()), p.platform(), f.deps())
	t.Cleanup(eng.stopAll)

	eng.apply(specs)
	if got := boundDevice(eng, "lab"); got != "eth3" {
		t.Fatalf("ParentDevice = %q, want eth3 before the selector fails", got)
	}

	// Every way ResolveDevice can refuse, including the two that say nothing
	// about the hardware: the backend is not loaded, and the listing failed.
	refusals := []string{
		"iface: no backend loaded",
		"iface: interface \"wan\" is bound to hardware by a selector that resolves to no device: netlink receive: interrupted system call",
		"iface: no device with MAC 02:00:00:00:be:99 for logical interface \"wan\"",
		"iface: MAC 02:00:00:00:be:99 for logical interface \"wan\" is carried by 2 devices (eth3, eth4)",
	}
	for _, reason := range refusals {
		p.resolveParent = func(string) (string, error) { return "", errors.New(reason) }
		eng.apply(specs)

		if got := boundDevice(eng, "lab"); got != "eth3" {
			t.Fatalf("after %q: ParentDevice = %q, want the virtual router still on eth3", reason, got)
		}
	}

	// No teardown side effect fired: no Priority-0 resignation path, no macvlan
	// removed, no dataplane reverted, no transport closed.
	if len(p.deleted) != 0 {
		t.Errorf("macvlan %v was deleted; an unresolved selector must not tear a running group down", p.deleted)
	}
	if len(p.reverted) != 0 {
		t.Errorf("dataplane reverted %v; the running group was torn down", p.reverted)
	}
	if len(p.closed) != 0 {
		t.Errorf("transport closed %v; the running group was torn down", p.closed)
	}
	if len(p.devices) != 1 {
		t.Errorf("macvlans = %v, want exactly the one this group owns", p.devices)
	}

	// And it heals on its own: the next pass that CAN resolve finds it in place
	// and reconfigures rather than rebuilding.
	p.resolveParent = selects("eth3")
	eng.apply(specs)
	if got := boundDevice(eng, "lab"); got != "eth3" {
		t.Fatalf("ParentDevice = %q after the selector recovered, want eth3", got)
	}
	if len(p.deleted) != 0 {
		t.Errorf("recovery rebuilt the group (deleted %v); it was never gone, so it must reconfigure", p.deleted)
	}
}

// TestVRRPGroupEditsLandWhileItsSelectorIsUnresolved proves the rest of an
// operator's edit still reaches a running group whose selector could not be
// answered this pass. Keeping the group must not mean freezing it.
//
// VALIDATES: an unresolved selector costs the binding, not the whole apply.
// PREVENTS: a priority or VIP change silently not taking effect because an
// unrelated netlink read failed.
func TestVRRPGroupEditsLandWhileItsSelectorIsUnresolved(t *testing.T) {
	specs, err := extractGroupSpecs([]configSection{mkSection(t, vrrpOn("0", 0, "lab", 10))})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	p := newFakePlatform()
	p.resolveParent = selects("eth3")
	f := &fakeDeps{}
	eng := newEngine(sim.NewFakeClock(time.Unix(0, 0).UTC()), p.platform(), f.deps())
	t.Cleanup(eng.stopAll)
	eng.apply(specs)

	edited := make([]GroupSpec, len(specs))
	copy(edited, specs)
	edited[0].Priority = 150
	p.resolveParent = func(string) (string, error) { return "", errors.New("iface: no backend loaded") }
	eng.apply(edited)

	eng.mu.Lock()
	defer eng.mu.Unlock()
	// Count FIRST. Ranging over the instances would pass vacuously on an empty
	// map, which is exactly the state the defect this test guards produces.
	if len(eng.instances) != 1 {
		t.Fatalf("instances = %d, want 1: the group was dropped rather than kept", len(eng.instances))
	}
	for _, in := range eng.instances {
		if in.spec.Priority != 150 {
			t.Errorf("priority = %d, want 150: the edit was dropped with the binding", in.spec.Priority)
		}
		if in.spec.ParentDevice != "eth3" {
			t.Errorf("ParentDevice = %q, want eth3: the edit must not move the device", in.spec.ParentDevice)
		}
	}
}

// TestVRRPGroupMovesWithItsSelector proves a virtual router whose selector
// starts answering a DIFFERENT device is rebuilt on that device.
//
// Reconfigure-in-place is the right answer for a priority or interval edit, and
// the wrong one here: the macvlan, the sockets and the per-device sysctls all
// hang off the old device and reconfigure touches none of them, so the virtual
// MAC would stay on hardware the operator stopped naming.
//
// VALIDATES: AC-1 across an apply that moves the binding.
// PREVENTS: a NIC swap leaving the virtual MAC on the old port.
func TestVRRPGroupMovesWithItsSelector(t *testing.T) {
	tree := vrrpOn("0", 0, "lab", 10)
	specs, err := extractGroupSpecs([]configSection{mkSection(t, tree)})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	p := newFakePlatform()
	p.resolveParent = selects("eth3")
	f := &fakeDeps{}
	eng := newEngine(sim.NewFakeClock(time.Unix(0, 0).UTC()), p.platform(), f.deps())
	t.Cleanup(eng.stopAll)

	eng.apply(specs)
	p.resolveParent = selects("eth7")
	eng.apply(specs)

	if got := boundDevice(eng, "lab"); got != "eth7" {
		t.Fatalf("ParentDevice = %q after the selector moved, want eth7", got)
	}
	last := p.parents[p.opened[len(p.opened)-1].MacvlanDevice]
	if last != "eth7" {
		t.Errorf("the rebuilt macvlan hangs off %q, want eth7", last)
	}
	if len(p.reverted) == 0 {
		t.Error("the old device's dataplane sysctls were never reverted")
	}
}

// TestVRRPGroupMoveKeepsTheOldRouterWhenTheRebuildFails proves a MOVE whose
// replacement cannot be built leaves the running virtual router exactly where
// it is, rather than destroying it.
//
// The rebuild is the one path where a binding outcome reaches teardown, and its
// create half can fail for a reason this group does not control:
// reconcileOwnedDevices (iface/config_apply.go) fails fast on the FIRST
// owned-device error in a pass, so one unrelated device times out this group's
// waitDevicePresent. Releasing before building would turn that into a destroyed
// virtual router -- and a permanent one, because apply runs only on a config
// event. That is the same outage the binding rule exists to forbid, arriving
// through the one door still open.
//
// VALIDATES: "a binding outcome may create and may move, never destroy" holds on
// the MOVE path too, not just on the unresolved one.
// PREVENTS: an unrelated owned-device failure taking a working VIP off the wire.
func TestVRRPGroupMoveKeepsTheOldRouterWhenTheRebuildFails(t *testing.T) {
	specs, err := extractGroupSpecs([]configSection{mkSection(t, vrrpOn("0", 0, "lab", 10))})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	p := newFakePlatform()
	p.resolveParent = selects("eth3")
	f := &fakeDeps{}
	eng := newEngine(sim.NewFakeClock(time.Unix(0, 0).UTC()), p.platform(), f.deps())
	t.Cleanup(eng.stopAll)
	eng.apply(specs)

	before := p.devices["zv4-2-10"]
	if before == "" {
		t.Fatalf("the group never came up on eth3: devices = %v", p.devices)
	}

	// The selector now answers a different NIC, and the replacement cannot be
	// built. Nothing about the running router has changed.
	p.resolveParent = selects("eth7")
	p.macvlanErr = errors.New("iface: create owned macvlan: timed out waiting for zv4-3-10")
	eng.apply(specs)

	if got := boundDevice(eng, "lab"); got != "eth3" {
		t.Fatalf("ParentDevice = %q after a failed move, want the router still on eth3", got)
	}
	if len(p.deleted) != 0 {
		t.Errorf("macvlan %v was deleted; a failed rebuild must not destroy the running router", p.deleted)
	}
	if len(p.reverted) != 0 {
		t.Errorf("dataplane reverted %v; the running router was torn down by a failed rebuild", p.reverted)
	}
	if len(p.closed) != 0 {
		t.Errorf("transport closed %v; the running router was torn down by a failed rebuild", p.closed)
	}
	if p.devices["zv4-2-10"] != before {
		t.Errorf("the router's macvlan on eth3 is gone: devices = %v", p.devices)
	}

	// And the move still happens once the rebuild can succeed.
	p.macvlanErr = nil
	eng.apply(specs)
	if got := boundDevice(eng, "lab"); got != "eth7" {
		t.Fatalf("ParentDevice = %q once the rebuild could succeed, want eth7", got)
	}
	if len(p.deleted) != 1 {
		t.Errorf("deleted = %v, want exactly the old device released by the successful move", p.deleted)
	}
}

// TestVRRPGroupRenameIsNotAMove proves a kernel RENAME of the parent leaves the
// virtual router and its VIP exactly where they are.
//
// A rename preserves the ifindex, so the old and the new name compose the SAME
// macvlan (deviceName -> ComposeOwnedDeviceName). Treating it as a move is
// destructive rather than merely wasteful: build re-registers that one device
// and teardown then unregisters what build just wrote, after which the next
// reconcile orphan-deletes the device and its addresses
// (UnregisterOwnedMacvlan) while the FSM still reports master. The VIP goes off
// the wire and every ze surface still says the group is up.
//
// Nothing refuses the collision. RegisterOwnedMacvlan's conflict loop skips its
// OWN owner, and VRRP derives the owner FROM the device name (ownerString), so a
// same-name registration is always a same-owner overwrite, never a refusal.
//
// VALIDATES: the move branch keys on the netdev, not on the name it wears.
// PREVENTS: a NIC rename silently taking a live VIP off the wire.
func TestVRRPGroupRenameIsNotAMove(t *testing.T) {
	specs, err := extractGroupSpecs([]configSection{mkSection(t, vrrpOn("0", 0, "lab", 10))})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	p := newFakePlatform()
	p.resolveParent = selects("eth3")
	f := &fakeDeps{}
	eng := newEngine(sim.NewFakeClock(time.Unix(0, 0).UTC()), p.platform(), f.deps())
	t.Cleanup(eng.stopAll)
	eng.apply(specs)

	dev := p.soleDevice(t)
	opened := len(p.opened)

	// The kernel renames the netdev: same hardware, same ifindex, new name.
	p.renameParent("eth3", "eth5")
	p.resolveParent = selects("eth5")
	eng.apply(specs)

	if len(p.deleted) != 0 {
		t.Errorf("macvlan %v deleted by a rename; the netdev underneath never changed", p.deleted)
	}
	if len(p.reverted) != 0 {
		t.Errorf("dataplane reverted %v; a rename tore the running router down", p.reverted)
	}
	if len(p.closed) != 0 {
		t.Errorf("transport closed %v; a rename tore the running router down", p.closed)
	}
	if len(p.opened) != opened {
		t.Errorf("transports opened = %d, was %d: a rename must not re-open the sockets", len(p.opened), opened)
	}
	// soleDevice fails when the macvlan is gone, which is the state the defect
	// produces: registered by build, then unregistered by teardown.
	if got := p.soleDevice(t); got != dev {
		t.Errorf("macvlan = %q, want the original %q kept in place", got, dev)
	}

	// The spec still follows the new name, so the per-device sysctls the
	// reassert loop writes land on the path the device now has.
	if got := boundDevice(eng, "lab"); got != "eth5" {
		t.Errorf("ParentDevice = %q, want eth5: the rename must reach the spec", got)
	}
}

// TestVRRPGroupNeverAdoptsAParentItsMacvlanIsNotOn proves an instance keeps the
// parent its macvlan is actually built on when the ifindex cannot be read.
//
// The binding loop and this check are two INDEPENDENT resolutions under
// different cache keys: ResolveDevice on the logical name, then parentIfindex on
// the device it answered. The first can succeed and the second fail, and then
// spec.ParentDevice names a device this instance's macvlan is not on. reconfigure
// assigns the spec wholesale, so adopting it desyncs the instance from its own
// device -- and evaluateReadiness then calls parentReady on that name on every
// link event. The name is absent, which is WHY the ifindex read failed, so
// readiness goes false and the master resigns and drops the VIP. That is the
// original blocker's outcome reached by the original blocker's own transient
// class, through a branch added to fix something else.
//
// VALIDATES: a failure to ask never moves the spec, only a successful answer does.
// PREVENTS: a transient ifindex failure resigning a master by desynchronising
// the spec from the macvlan.
func TestVRRPGroupNeverAdoptsAParentItsMacvlanIsNotOn(t *testing.T) {
	specs, err := extractGroupSpecs([]configSection{mkSection(t, vrrpOn("0", 0, "lab", 10))})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	p := newFakePlatform()
	p.resolveParent = selects("eth3")
	f := &fakeDeps{}
	eng := newEngine(sim.NewFakeClock(time.Unix(0, 0).UTC()), p.platform(), f.deps())
	t.Cleanup(eng.stopAll)
	eng.apply(specs)

	dev := p.soleDevice(t)
	p.reasserted = nil

	// The selector now answers a different NIC, so the binding loop succeeds and
	// hands the spec eth7. The ifindex read fails -- eth7 is not there, which is
	// the same absence -- so nothing can say whether this is a move.
	edited := make([]GroupSpec, len(specs))
	copy(edited, specs)
	edited[0].Priority = 150
	p.resolveParent = selects("eth7")
	p.ifindexErr = errors.New("iface: interface \"eth7\" not found")
	eng.apply(edited)

	if got := boundDevice(eng, "lab"); got != "eth3" {
		t.Errorf("ParentDevice = %q, want eth3: the instance adopted a parent its macvlan %q is not on", got, dev)
	}
	for _, r := range p.reasserted {
		if strings.HasPrefix(r, "eth7/") {
			t.Errorf("dataplane reasserted on %q; the sysctls belong to the device the macvlan is on", r)
		}
	}
	if len(p.deleted) != 0 || len(p.reverted) != 0 || len(p.closed) != 0 {
		t.Errorf("the running router was torn down: deleted=%v reverted=%v closed=%v", p.deleted, p.reverted, p.closed)
	}
	if got := p.soleDevice(t); got != dev {
		t.Errorf("macvlan = %q, want the original %q", got, dev)
	}

	// The rest of the edit still lands: a failure to ask costs the binding, not
	// the whole apply.
	eng.mu.Lock()
	defer eng.mu.Unlock()
	if len(eng.instances) != 1 {
		t.Fatalf("instances = %d, want 1", len(eng.instances))
	}
	for _, in := range eng.instances {
		if in.spec.Priority != 150 {
			t.Errorf("priority = %d, want 150: the edit was dropped with the binding", in.spec.Priority)
		}
	}
}

// TestVRRPGroupMoveUnderOneParentNameKeepsALiveTransport proves a move whose
// parent keeps its NAME leaves the group with sockets that are actually open.
//
// A transport instance is keyed {parent name, vrid, family}
// (transport.go InstanceSpec.key), and the macvlan is named from the parent's
// IFINDEX, so a netdev replaced under one name -- a card that re-enumerates, a
// driver that reloads, an iface apply that recreates a VLAN device -- moves the
// macvlan and leaves the transport key exactly where it was. Building the
// replacement before releasing the predecessor then opens it over the running
// key: OpenInstance overwrites that map entry without shutting the old sockets
// down, and teardown's CloseInstance closes the REPLACEMENT. The engine holds a
// virtual router whose sockets are shut and every ze surface reports it running.
//
// VALIDATES: build-before-release is applied only where the two can coexist.
// PREVENTS: a re-enumerated NIC leaving a group advertising on a closed socket.
func TestVRRPGroupMoveUnderOneParentNameKeepsALiveTransport(t *testing.T) {
	specs, err := extractGroupSpecs([]configSection{mkSection(t, vrrpOn("0", 0, "lab", 10))})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	p := newFakePlatform()
	p.resolveParent = selects("eth3")
	f := &fakeDeps{}
	eng := newEngine(sim.NewFakeClock(time.Unix(0, 0).UTC()), p.platform(), f.deps())
	t.Cleanup(eng.stopAll)
	eng.apply(specs)

	first := p.soleDevice(t)

	// Same name, new netdev. The selector still answers eth3, so the binding
	// succeeds and only the macvlan name moves.
	p.replaceNetdev("eth3")
	eng.apply(specs)

	if len(p.overwrote) != 0 {
		t.Errorf("transport opened over the live key %v; the displaced sockets are never shut down", p.overwrote)
	}
	dev := p.soleDevice(t)
	if dev == first {
		t.Fatalf("macvlan = %q, want a new one: the netdev under eth3 was replaced", dev)
	}
	// Count first: an empty map would satisfy every assertion the loop makes,
	// and "no transport at all" is one of the states the defect produces.
	if len(p.live) != 1 {
		t.Fatalf("open transports = %v, want exactly the replacement's", p.live)
	}
	for key, served := range p.live {
		if key.Interface != "eth3" {
			t.Errorf("open transport on parent %q, want eth3", key.Interface)
		}
		if served != dev {
			t.Errorf("the open transport serves %q, want the macvlan %q the group now owns", served, dev)
		}
	}
	if got := boundDevice(eng, "lab"); got != "eth3" {
		t.Errorf("ParentDevice = %q, want eth3", got)
	}
}

// TestVRRPGroupMoveBackToTheNameItsSocketsUseKeepsALiveTransport proves the
// collision test reads the transport KEY rather than the spec's device name.
//
// A rename adopts the new name into the spec and re-opens nothing, so the
// running instance's key keeps naming the parent its sockets were opened under.
// Comparing the two spec fields would then miss the one case where the names
// diverge: a parent renamed away and back, with a new netdev behind it the
// second time. That is a collision the spec comparison calls a clean move.
//
// VALIDATES: the release-first branch keys on in.key.Interface.
// PREVENTS: the collision returning through the branch that renames the spec.
func TestVRRPGroupMoveBackToTheNameItsSocketsUseKeepsALiveTransport(t *testing.T) {
	specs, err := extractGroupSpecs([]configSection{mkSection(t, vrrpOn("0", 0, "lab", 10))})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	p := newFakePlatform()
	p.resolveParent = selects("eth3")
	f := &fakeDeps{}
	eng := newEngine(sim.NewFakeClock(time.Unix(0, 0).UTC()), p.platform(), f.deps())
	t.Cleanup(eng.stopAll)
	eng.apply(specs)

	// A rename: same netdev, new name. The spec follows it and the transport
	// stays open under eth3.
	p.renameParent("eth3", "eth5")
	p.resolveParent = selects("eth5")
	eng.apply(specs)

	// The name comes back, on a netdev that is not the one that left.
	p.replaceNetdev("eth3")
	p.resolveParent = selects("eth3")
	eng.apply(specs)

	if len(p.overwrote) != 0 {
		t.Errorf("transport opened over the live key %v; the displaced sockets are never shut down", p.overwrote)
	}
	dev := p.soleDevice(t)
	if len(p.live) != 1 {
		t.Fatalf("open transports = %v, want exactly the replacement's", p.live)
	}
	for _, served := range p.live {
		if served != dev {
			t.Errorf("the open transport serves %q, want the macvlan %q the group now owns", served, dev)
		}
	}
}

// TestNoRegistryResolvesAConfiguredNameAgainstTheKernel proves no kernel-facing
// value VRRP hands out is a configured interface name.
//
// Each sink resolves its argument against the kernel itself: the owned-macvlan
// registry does netlink.LinkByName(spec.Parent), the dataplane writes
// /proc/sys/net/ipv4/conf/<parent>/, and the transport binds sockets to the
// device it is given. One resolution feeds all of them, so this asserts the
// property over every sink at once rather than over the one that was fixed.
//
// VALIDATES: AC-6.
// PREVENTS: a new sink taking the logical name because only the macvlan path
// was ever checked.
func TestNoRegistryResolvesAConfiguredNameAgainstTheKernel(t *testing.T) {
	eng, p := applyTree(t, vrrpOn("1", 100, "tagged", 11), selects("eth3"))

	sinks := map[string][]string{
		"macvlan parent":     mapValues(p.parents),
		"dataplane sysctls":  p.applied,
		"reasserted sysctls": p.reasserted,
		"instance spec":      {boundDevice(eng, "tagged")},
	}
	for _, s := range p.opened {
		sinks["transport parent"] = append(sinks["transport parent"], s.Parent)
	}
	for sink, values := range sinks {
		if len(values) == 0 {
			t.Fatalf("%s recorded nothing; the test proves nothing about it", sink)
		}
		for _, v := range values {
			if strings.Contains(v, selectedIface) {
				t.Errorf("%s carries the configured name in %q; it must carry the resolved device", sink, v)
			}
			if !strings.Contains(v, "eth3") {
				t.Errorf("%s = %q, want the resolved device eth3", sink, v)
			}
		}
	}
}

// mapValues returns a map's values, for asserting over a recording keyed by
// device name.
func mapValues(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}
