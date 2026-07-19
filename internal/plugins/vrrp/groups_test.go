// Design: plan/learned/1124-vrrp-first-hop-redundancy.md -- VRRP config extraction + verification tests

package vrrp

import (
	"encoding/json"
	"net/netip"
	"strconv"
	"strings"
	"testing"
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

// TestUnitDeviceResolution proves a group's kernel-facing device is the UNIT's
// device, not the logical interface name.
//
// A unit with a vlan-id lives on a sub-interface (iface names it
// "<parent>.<vlan-id>", config_apply.go:35-39). Binding sockets and the macvlan
// to the bare parent instead would advertise into the wrong broadcast domain --
// the group would simply never see its peers, while looking configured. It
// would also collapse two units of one interface onto a single transport
// InstanceKey and a single metric series.
//
// VALIDATES: ParentDevice per unit.
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
	devices := map[string]string{}
	for _, s := range specs {
		devices[s.Name] = s.ParentDevice
	}
	if devices["base"] != "eth0" {
		t.Errorf("plain unit device = %q, want eth0", devices["base"])
	}
	if devices["tagged"] != "eth0.100" {
		t.Errorf("vlan unit device = %q, want eth0.100", devices["tagged"])
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
