// Design: docs/architecture/core-design.md -- tests for read-only firewall packet decision trace
// Related: explain.go -- EvaluatePacket implementation
// Related: cmd_trace.go -- show firewall trace RPC handler

package firewall

import (
	"encoding/json"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
)

func mustAddr(t *testing.T, raw string) netip.Addr {
	t.Helper()
	addr, err := netip.ParseAddr(raw)
	if err != nil {
		t.Fatalf("parse addr %q: %v", raw, err)
	}
	return addr
}

func mustPrefix(t *testing.T, raw string) netip.Prefix {
	t.Helper()
	prefix, err := netip.ParsePrefix(raw)
	if err != nil {
		t.Fatalf("parse prefix %q: %v", raw, err)
	}
	return prefix
}

// VALIDATES: EvaluatePacket terminal verdicts (accept, drop, reject, snat, dnat, masquerade, redirect).
// PREVENTS: Terminal firewall actions producing wrong verdict, matched chain/term, or failing to stop evaluation.
func TestEvaluatePacketTerminalVerdicts(t *testing.T) {
	tests := []struct {
		name           string
		action         Action
		wantVerdict    string
		wantActionPart string
	}{
		{
			name:           "accept",
			action:         Accept{},
			wantVerdict:    "accept",
			wantActionPart: "accept",
		},
		{
			name:           "drop",
			action:         Drop{},
			wantVerdict:    "drop",
			wantActionPart: "drop",
		},
		{
			name:           "reject with tcp-reset",
			action:         Reject{Type: "tcp-reset"},
			wantVerdict:    "reject",
			wantActionPart: "reject with tcp-reset",
		},
		{
			name:           "snat",
			action:         SNAT{Address: mustAddr(t, "198.51.100.1"), Port: 10000, PortEnd: 20000},
			wantVerdict:    "snat",
			wantActionPart: "snat to 198.51.100.1:10000-20000",
		},
		{
			name:           "dnat",
			action:         DNAT{Address: mustAddr(t, "10.0.0.50"), Port: 8080},
			wantVerdict:    "dnat",
			wantActionPart: "dnat to 10.0.0.50:8080",
		},
		{
			name:           "masquerade",
			action:         Masquerade{},
			wantVerdict:    "masquerade",
			wantActionPart: "masquerade",
		},
		{
			name:           "redirect",
			action:         Redirect{Port: 3128},
			wantVerdict:    "redirect",
			wantActionPart: "redirect to 3128",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tables := []Table{
				{
					Name:   "ze_filter",
					Family: FamilyInet,
					Chains: []Chain{
						{
							Name:     "input",
							IsBase:   true,
							Type:     ChainFilter,
							Hook:     HookInput,
							Priority: 0,
							Policy:   PolicyDrop,
							Terms: []Term{
								{
									Name:    "match-it",
									Matches: []Match{MatchProtocol{Protocol: "tcp"}},
									Actions: []Action{Counter{Name: "hits"}, tc.action},
								},
								{
									Name:    "unreachable",
									Actions: []Action{Drop{}},
								},
							},
						},
					},
				},
			}

			res, opErr := EvaluatePacket(tables, TraceQuery{
				Table:    "filter",
				Protocol: "TCP",
			})
			if opErr != nil {
				t.Fatalf("EvaluatePacket unexpected error: %v", opErr)
			}
			if res.Verdict != tc.wantVerdict {
				t.Errorf("Verdict = %q, want %q", res.Verdict, tc.wantVerdict)
			}
			if res.MatchedChain != "input" {
				t.Errorf("MatchedChain = %q, want %q", res.MatchedChain, "input")
			}
			if res.MatchedTerm != "match-it" {
				t.Errorf("MatchedTerm = %q, want %q", res.MatchedTerm, "match-it")
			}
			if res.DefaultPolicy {
				t.Errorf("DefaultPolicy = true, want false")
			}
			if len(res.Steps) != 1 {
				t.Fatalf("len(Steps) = %d, want 1 (must stop after terminal verdict)", len(res.Steps))
			}
			if !res.Steps[0].Matched {
				t.Errorf("Steps[0].Matched = false, want true")
			}
			joined := strings.Join(res.Steps[0].Actions, ",")
			if !strings.Contains(joined, tc.wantActionPart) {
				t.Errorf("Steps[0].Actions = %v, want substring %q", res.Steps[0].Actions, tc.wantActionPart)
			}
		})
	}
}

// VALIDATES: CIDR, IPv6, port range, wildcard interface, and connection state match predicates.
// PREVENTS: False negatives or false positives on IPv4/IPv6 prefixes, port ranges, and wildcard interfaces.
func TestEvaluatePacketCIDRIPv6PortsWildcardAndConnState(t *testing.T) {
	tables := []Table{
		{
			Name:   "ze_edge",
			Family: FamilyInet,
			Chains: []Chain{
				{
					Name:     "forward",
					IsBase:   true,
					Type:     ChainFilter,
					Hook:     HookForward,
					Priority: 0,
					Policy:   PolicyDrop,
					Terms: []Term{
						{
							Name: "allow-v4-range",
							Matches: []Match{
								MatchSourceAddress{Prefix: mustPrefix(t, "10.20.0.0/16")},
								MatchDestinationAddress{Prefix: mustPrefix(t, "192.0.2.0/24")},
								MatchProtocol{Protocol: "tcp"},
								MatchSourcePort{Ranges: []PortRange{{Lo: 1024, Hi: 65535}}},
								MatchDestinationPort{Ranges: []PortRange{{Lo: 80, Hi: 80}, {Lo: 443, Hi: 445}}},
								MatchInputInterface{Name: "l2tp", Wildcard: true},
								MatchOutputInterface{Name: "eth0", Wildcard: false},
								MatchConnState{States: ConnStateNew | ConnStateEstablished},
							},
							Actions: []Action{Accept{}},
						},
						{
							Name: "allow-v6",
							Matches: []Match{
								MatchSourceAddress{Prefix: mustPrefix(t, "2001:db8:1::/48")},
								MatchDestinationAddress{Prefix: mustPrefix(t, "2001:db8:2::/48")},
								MatchProtocol{Protocol: "udp"},
								MatchDestinationPort{Ranges: []PortRange{{Lo: 53, Hi: 53}}},
							},
							Actions: []Action{Accept{}},
						},
					},
				},
			},
		},
	}

	v4Res, opErr := EvaluatePacket(tables, TraceQuery{
		Table:           "edge",
		Protocol:        "tcp",
		SourceIP:        mustAddr(t, "10.20.5.9"),
		DestinationIP:   mustAddr(t, "192.0.2.42"),
		SourcePort:      54321,
		DestinationPort: 444,
		InInterface:     "l2tp12",
		OutInterface:    "eth0",
		ConnState:       ConnStateEstablished,
	})
	if opErr != nil {
		t.Fatalf("v4 trace error: %v", opErr)
	}
	if v4Res.Verdict != verdictAccept || v4Res.MatchedTerm != "allow-v4-range" {
		t.Errorf("v4 trace got verdict=%q term=%q, want accept / allow-v4-range", v4Res.Verdict, v4Res.MatchedTerm)
	}

	v6Res, opErr := EvaluatePacket(tables, TraceQuery{
		Table:           "edge",
		Protocol:        "udp",
		SourceIP:        mustAddr(t, "2001:db8:1::10"),
		DestinationIP:   mustAddr(t, "2001:db8:2::53"),
		DestinationPort: 53,
	})
	if opErr != nil {
		t.Fatalf("v6 trace error: %v", opErr)
	}
	if v6Res.Verdict != verdictAccept || v6Res.MatchedTerm != "allow-v6" {
		t.Errorf("v6 trace got verdict=%q term=%q, want accept / allow-v6", v6Res.Verdict, v6Res.MatchedTerm)
	}
	if len(v6Res.Steps) != 2 || v6Res.Steps[0].Matched || !v6Res.Steps[1].Matched {
		t.Errorf("v6 steps unexpected: %+v", v6Res.Steps)
	}

	// Missing source address reports explicit reason and falls through to default policy.
	missRes, opErr := EvaluatePacket(tables, TraceQuery{
		Table:    "edge",
		Protocol: "tcp",
	})
	if opErr != nil {
		t.Fatalf("miss trace error: %v", opErr)
	}
	if missRes.Verdict != verdictDrop || !missRes.DefaultPolicy {
		t.Errorf("miss trace got verdict=%q defaultPolicy=%v, want drop / true", missRes.Verdict, missRes.DefaultPolicy)
	}
	if missRes.Steps[0].Reason != reasonSourceAddrMissing {
		t.Errorf("Steps[0].Reason = %q, want %q", missRes.Steps[0].Reason, reasonSourceAddrMissing)
	}
}

// VALIDATES: Named set evaluation for IPv4 CIDR, IPv6, RawIntervals (including top-of-space), inet_service, ifname, and unsupported set types.
// PREVENTS: Set-based firewall rules failing to match CIDR prefixes, pre-expanded CoPP intervals, ports, or interface sets.
func TestEvaluatePacketNamedSets(t *testing.T) {
	tables := []Table{
		{
			Name:   "ze_sets",
			Family: FamilyInet,
			Sets: []Set{
				{
					Name:  "v4_cidrs",
					Type:  SetTypeIPv4,
					Flags: SetFlagInterval,
					Elements: []SetElement{
						{Value: "10.0.0.0/24"},
						{Value: "198.51.100.7"},
					},
				},
				{
					Name:  "v6_cidrs",
					Type:  SetTypeIPv6,
					Flags: SetFlagInterval,
					Elements: []SetElement{
						{Value: "2001:db8:abcd::/48"},
					},
				},
				{
					Name:         "copp_raw",
					Type:         SetTypeIPv4,
					Flags:        SetFlagInterval,
					RawIntervals: true,
					Elements: []SetElement{
						{Value: "172.16.0.0"},
						{Value: "172.16.1.0", IntervalEnd: true},
						{Value: "255.255.255.0"},
					},
				},
				{
					Name: "web_ports",
					Type: SetTypeInetService,
					Elements: []SetElement{
						{Value: "80"},
						{Value: "8000-8080"},
					},
				},
				{
					Name: "trusted_ifaces",
					Type: SetTypeIfname,
					Elements: []SetElement{
						{Value: "eth0"},
						{Value: "wg*"},
					},
				},
				{
					Name: "mac_set",
					Type: SetTypeEther,
					Elements: []SetElement{
						{Value: "00:11:22:33:44:55"},
					},
				},
			},
			Chains: []Chain{
				{
					Name:     "input",
					IsBase:   true,
					Type:     ChainFilter,
					Hook:     HookInput,
					Priority: 0,
					Policy:   PolicyDrop,
					Terms: []Term{
						{
							Name: "match-v4-set",
							Matches: []Match{
								MatchInSet{SetName: "v4_cidrs", MatchField: SetFieldSourceAddr},
								MatchInSet{SetName: "web_ports", MatchField: SetFieldDestPort},
							},
							Actions: []Action{Accept{}},
						},
						{
							Name: "match-v6-set",
							Matches: []Match{
								MatchInSet{SetName: "v6_cidrs", MatchField: SetFieldSourceAddr},
							},
							Actions: []Action{Accept{}},
						},
						{
							Name: "match-raw-intervals",
							Matches: []Match{
								MatchInSet{SetName: "copp_raw", MatchField: SetFieldSourceAddr},
							},
							Actions: []Action{Accept{}},
						},
						{
							Name: "match-ifname-set",
							Matches: []Match{
								MatchInSet{SetName: "trusted_ifaces", MatchField: SetFieldSourceAddr},
							},
							Actions: []Action{Accept{}},
						},
						{
							Name: "match-ether-set",
							Matches: []Match{
								MatchInSet{SetName: "mac_set", MatchField: SetFieldSourceAddr},
							},
							Actions: []Action{Accept{}},
						},
					},
				},
			},
		},
	}

	// IPv4 CIDR + port range set match.
	res, opErr := EvaluatePacket(tables, TraceQuery{
		Table:           "sets",
		SourceIP:        mustAddr(t, "10.0.0.99"),
		DestinationPort: 8042,
	})
	if opErr != nil {
		t.Fatalf("v4 set trace error: %v", opErr)
	}
	if res.Verdict != verdictAccept || res.MatchedTerm != "match-v4-set" {
		t.Errorf("v4 set got verdict=%q term=%q", res.Verdict, res.MatchedTerm)
	}

	// IPv6 set match.
	resV6, opErr := EvaluatePacket(tables, TraceQuery{
		Table:    "sets",
		SourceIP: mustAddr(t, "2001:db8:abcd::42"),
	})
	if opErr != nil {
		t.Fatalf("v6 set trace error: %v", opErr)
	}
	if resV6.Verdict != verdictAccept || resV6.MatchedTerm != "match-v6-set" {
		t.Errorf("v6 set got verdict=%q term=%q", resV6.Verdict, resV6.MatchedTerm)
	}

	// RawIntervals half-open interval interior + exclusive upper bound + top-of-space.
	resRawHit, opErr := EvaluatePacket(tables, TraceQuery{
		Table:    "sets",
		SourceIP: mustAddr(t, "172.16.0.255"),
	})
	if opErr != nil {
		t.Fatalf("raw interval hit error: %v", opErr)
	}
	if resRawHit.Verdict != verdictAccept || resRawHit.MatchedTerm != "match-raw-intervals" {
		t.Errorf("raw interval hit got verdict=%q term=%q", resRawHit.Verdict, resRawHit.MatchedTerm)
	}

	resRawTop, opErr := EvaluatePacket(tables, TraceQuery{
		Table:    "sets",
		SourceIP: mustAddr(t, "255.255.255.255"),
	})
	if opErr != nil {
		t.Fatalf("raw interval top-of-space error: %v", opErr)
	}
	if resRawTop.Verdict != verdictAccept || resRawTop.MatchedTerm != "match-raw-intervals" {
		t.Errorf("raw interval top-of-space got verdict=%q term=%q", resRawTop.Verdict, resRawTop.MatchedTerm)
	}

	// Exclusive upper bound 172.16.1.0 must NOT match copp_raw, and reaching mac_set marks Incomplete.
	resRawMiss, opErr := EvaluatePacket(tables, TraceQuery{
		Table:    "sets",
		SourceIP: mustAddr(t, "172.16.1.0"),
	})
	if opErr != nil {
		t.Fatalf("raw interval exclusive end error: %v", opErr)
	}
	if resRawMiss.Verdict != verdictDrop || !resRawMiss.DefaultPolicy {
		t.Errorf("exclusive upper bound got verdict=%q default=%v, want drop / true", resRawMiss.Verdict, resRawMiss.DefaultPolicy)
	}
	if !resRawMiss.Incomplete {
		t.Errorf("expected Incomplete=true after hitting unsupported SetTypeEther")
	}
	if !resRawMiss.Steps[4].Unevaluated {
		t.Errorf("expected Steps[4].Unevaluated=true for mac_set, got %+v", resRawMiss.Steps[4])
	}

	// Ifname set match with wildcard.
	resIf, opErr := EvaluatePacket(tables, TraceQuery{
		Table:       "sets",
		InInterface: "wg0",
	})
	if opErr != nil {
		t.Fatalf("ifname set trace error: %v", opErr)
	}
	if resIf.Verdict != verdictAccept || resIf.MatchedTerm != "match-ifname-set" {
		t.Errorf("ifname set got verdict=%q term=%q", resIf.Verdict, resIf.MatchedTerm)
	}
}

// VALIDATES: Jump with return, Jump with terminal verdict, and Goto without return.
// PREVENTS: Control-flow actions losing caller resumption on Jump or wrongly resuming caller after Goto.
func TestEvaluatePacketJumpAndGoto(t *testing.T) {
	tables := []Table{
		{
			Name:   "ze_flow",
			Family: FamilyInet,
			Chains: []Chain{
				{
					Name:     "main",
					IsBase:   true,
					Type:     ChainFilter,
					Hook:     HookInput,
					Priority: 0,
					Policy:   PolicyDrop,
					Terms: []Term{
						{
							Name:    "call-sub",
							Matches: []Match{MatchProtocol{Protocol: "tcp"}},
							Actions: []Action{Jump{Target: "sub"}},
						},
						{
							Name:    "goto-branch",
							Matches: []Match{MatchProtocol{Protocol: "udp"}},
							Actions: []Action{Goto{Target: "udp_sub"}},
						},
						{
							Name:    "fallback-accept",
							Actions: []Action{Accept{}},
						},
					},
				},
				{
					Name: "sub",
					Terms: []Term{
						{
							Name:    "drop-ssh",
							Matches: []Match{MatchDestinationPort{Ranges: []PortRange{{Lo: 22, Hi: 22}}}},
							Actions: []Action{Drop{}},
						},
						{
							Name:    "return-rest",
							Actions: []Action{Return{}},
						},
					},
				},
				{
					Name: "udp_sub",
					Terms: []Term{
						{
							Name:    "allow-dns",
							Matches: []Match{MatchDestinationPort{Ranges: []PortRange{{Lo: 53, Hi: 53}}}},
							Actions: []Action{Accept{}},
						},
					},
				},
			},
		},
	}

	// Jump that terminates in callee.
	resTerm, opErr := EvaluatePacket(tables, TraceQuery{
		Table:           "flow",
		Protocol:        "tcp",
		DestinationPort: 22,
	})
	if opErr != nil {
		t.Fatalf("jump terminal error: %v", opErr)
	}
	if resTerm.Verdict != verdictDrop || resTerm.MatchedChain != "sub" || resTerm.MatchedTerm != "drop-ssh" {
		t.Errorf("jump terminal got verdict=%q chain=%q term=%q", resTerm.Verdict, resTerm.MatchedChain, resTerm.MatchedTerm)
	}

	// Jump that returns to caller and matches fallback-accept.
	resRet, opErr := EvaluatePacket(tables, TraceQuery{
		Table:           "flow",
		Protocol:        "tcp",
		DestinationPort: 443,
	})
	if opErr != nil {
		t.Fatalf("jump return error: %v", opErr)
	}
	if resRet.Verdict != verdictAccept || resRet.MatchedChain != "main" || resRet.MatchedTerm != "fallback-accept" {
		t.Errorf("jump return got verdict=%q chain=%q term=%q", resRet.Verdict, resRet.MatchedChain, resRet.MatchedTerm)
	}

	// Goto that falls off udp_sub must NOT resume main's fallback-accept; hits base policy drop.
	resGoto, opErr := EvaluatePacket(tables, TraceQuery{
		Table:           "flow",
		Protocol:        "udp",
		DestinationPort: 123,
	})
	if opErr != nil {
		t.Fatalf("goto fallthrough error: %v", opErr)
	}
	if resGoto.Verdict != verdictDrop || !resGoto.DefaultPolicy {
		t.Errorf("goto fallthrough got verdict=%q defaultPolicy=%v, want drop / true", resGoto.Verdict, resGoto.DefaultPolicy)
	}
}

// VALIDATES: SetMark and SetDSCP update effective trace metadata for subsequent terms without mutating caller pointers.
// PREVENTS: Packet mark/DSCP pipeline rules failing to see modifications made by earlier terms.
func TestEvaluatePacketSetMarkAndSetDSCPPropagation(t *testing.T) {
	initialMark := uint32(0x100)
	initialDSCP := uint8(0)
	tables := []Table{
		{
			Name:   "ze_mangle",
			Family: FamilyIP,
			Chains: []Chain{
				{
					Name:     "prerouting",
					IsBase:   true,
					Type:     ChainFilter,
					Hook:     HookPrerouting,
					Priority: -150,
					Policy:   PolicyDrop,
					Terms: []Term{
						{
							Name: "tag-packet",
							Actions: []Action{
								SetMark{Value: 0x05, Mask: 0x0F},
								SetDSCP{Value: 46},
							},
						},
						{
							Name: "match-tagged",
							Matches: []Match{
								MatchMark{Value: 0x105, Mask: 0xFFF},
								MatchDSCP{Value: 46},
							},
							Actions: []Action{Accept{}},
						},
					},
				},
			},
		},
	}

	query := TraceQuery{
		Table: "mangle",
		Mark:  &initialMark,
		DSCP:  &initialDSCP,
	}
	res, opErr := EvaluatePacket(tables, query)
	if opErr != nil {
		t.Fatalf("mangle trace error: %v", opErr)
	}
	if res.Verdict != verdictAccept || res.MatchedTerm != "match-tagged" {
		t.Errorf("got verdict=%q term=%q, want accept / match-tagged (steps=%+v)", res.Verdict, res.MatchedTerm, res.Steps)
	}
	if initialMark != 0x100 || initialDSCP != 0 {
		t.Errorf("EvaluatePacket mutated caller pointers: mark=0x%x dscp=%d", initialMark, initialDSCP)
	}
}

// VALIDATES: Unevaluated kernel-only predicates mark Unevaluated and Incomplete and continue evaluation; base chain priority selection and non-base return.
// PREVENTS: Offline packet trace fabricating a verdict over unevaluated kernel predicates or picking the wrong default entry chain.
func TestEvaluatePacketUnevaluatedAndChainSelection(t *testing.T) {
	tables := []Table{
		{
			Name:   "ze_mixed",
			Family: FamilyIP,
			Chains: []Chain{
				{
					Name:     "later",
					IsBase:   true,
					Type:     ChainFilter,
					Hook:     HookInput,
					Priority: 10,
					Policy:   PolicyAccept,
				},
				{
					Name:     "earlier",
					IsBase:   true,
					Type:     ChainFilter,
					Hook:     HookInput,
					Priority: -10,
					Policy:   PolicyDrop,
					Terms: []Term{
						{
							Name: "check-local",
							Matches: []Match{
								MatchDestinationLocal{},
							},
							Actions: []Action{Accept{}},
						},
						{
							Name: "check-tcp-flags",
							Matches: []Match{
								MatchProtocol{Protocol: "tcp"},
								MatchTCPFlags{Flags: TCPFlagSYN, Mask: TCPFlagSYN | TCPFlagACK},
							},
							Actions: []Action{Accept{}},
						},
					},
				},
				{
					Name: "helper",
					Terms: []Term{
						{
							Name:    "only-udp",
							Matches: []Match{MatchProtocol{Protocol: "udp"}},
							Actions: []Action{Accept{}},
						},
					},
				},
			},
		},
	}

	// Default entry chain picks lowest priority ("earlier" at -10).
	res, opErr := EvaluatePacket(tables, TraceQuery{
		Table:    "mixed",
		Protocol: "tcp",
	})
	if opErr != nil {
		t.Fatalf("unexpected error: %v", opErr)
	}
	if res.Chain != "earlier" {
		t.Errorf("Chain = %q, want %q", res.Chain, "earlier")
	}
	if !res.Incomplete {
		t.Errorf("Incomplete = false, want true")
	}
	if !res.Steps[0].Unevaluated || res.Steps[0].Matched {
		t.Errorf("Steps[0] = %+v, want Unevaluated=true Matched=false", res.Steps[0])
	}
	if !res.Steps[1].Unevaluated || !strings.Contains(res.Steps[1].Reason, "tcp-flags") {
		t.Errorf("Steps[1] = %+v, want Unevaluated=true naming tcp-flags", res.Steps[1])
	}
	if res.Verdict != verdictDrop || !res.DefaultPolicy {
		t.Errorf("Verdict = %q DefaultPolicy = %v, want drop / true", res.Verdict, res.DefaultPolicy)
	}

	// Non-base chain fallthrough yields Verdict="return", DefaultPolicy=false.
	resHelper, opErr := EvaluatePacket(tables, TraceQuery{
		Table:    "mixed",
		Chain:    "helper",
		Protocol: "tcp",
	})
	if opErr != nil {
		t.Fatalf("helper trace error: %v", opErr)
	}
	if resHelper.Verdict != verdictReturn || resHelper.DefaultPolicy {
		t.Errorf("helper got verdict=%q defaultPolicy=%v, want return / false", resHelper.Verdict, resHelper.DefaultPolicy)
	}
}

// VALIDATES: Cyclic jump and goto chains stop cleanly at maxTraceDepth (16).
// PREVENTS: Infinite recursion or stack overflow on cyclic chain references.
func TestEvaluatePacketCyclicJumpDepthBound(t *testing.T) {
	tables := []Table{
		{
			Name:   "ze_loop",
			Family: FamilyInet,
			Chains: []Chain{
				{
					Name:     "a",
					IsBase:   true,
					Type:     ChainFilter,
					Hook:     HookInput,
					Priority: 0,
					Policy:   PolicyAccept,
					Terms:    []Term{{Name: "to-b", Actions: []Action{Jump{Target: "b"}}}},
				},
				{
					Name:  "b",
					Terms: []Term{{Name: "to-a", Actions: []Action{Jump{Target: "a"}}}},
				},
			},
		},
	}

	res, opErr := EvaluatePacket(tables, TraceQuery{Table: "loop"})
	if opErr != nil {
		t.Fatalf("unexpected error on cyclic jump: %v", opErr)
	}
	if res.Verdict != verdictDepthLimitExceeded {
		t.Errorf("Verdict = %q, want %q", res.Verdict, verdictDepthLimitExceeded)
	}
	if !res.Incomplete {
		t.Errorf("Incomplete = false, want true")
	}
	if len(res.Steps) != maxTraceDepth {
		t.Errorf("len(Steps) = %d, want %d", len(res.Steps), maxTraceDepth)
	}
}

// VALIDATES: Unknown table, unknown chain, and table with no base chains return structured operator errors.
// PREVENTS: Silent zero verdicts when operator names a missing table or chain.
func TestEvaluatePacketErrors(t *testing.T) {
	tables := []Table{
		{
			Name:   "ze_zeta",
			Family: FamilyInet,
			Chains: []Chain{{Name: "sub_only", IsBase: false}},
		},
		{
			Name:   "ze_alpha",
			Family: FamilyInet,
			Chains: []Chain{{Name: "input", IsBase: true, Type: ChainFilter, Hook: HookInput, Policy: PolicyAccept}},
		},
	}

	_, opErr := EvaluatePacket(tables, TraceQuery{Table: "missing"})
	if opErr == nil {
		t.Fatalf("expected error for unknown table")
	}
	if !strings.Contains(opErr.Problem(), "alpha, zeta") {
		t.Errorf("unknown table error %q should list sorted applied tables", opErr.Problem())
	}

	_, opErr = EvaluatePacket(tables, TraceQuery{Table: "alpha", Chain: "nope"})
	if opErr == nil {
		t.Fatalf("expected error for unknown chain")
	}
	if !strings.Contains(opErr.Problem(), "input") {
		t.Errorf("unknown chain error %q should list available chains", opErr.Problem())
	}

	_, opErr = EvaluatePacket(tables, TraceQuery{Table: "zeta"})
	if opErr == nil {
		t.Fatalf("expected error for table with no base chains")
	}
	if !strings.Contains(opErr.Problem(), "sub_only") {
		t.Errorf("no-base-chain error %q should list available chains", opErr.Problem())
	}
}

// VALIDATES: handleShowFirewallTrace parses keyed and token arguments, validates IP/port boundaries (0, 1, 65535, 65536), and returns structured JSON.
// PREVENTS: Invalid ports or IP addresses slipping past the RPC handler or response payloads using non-kebab keys.
func TestHandleShowFirewallTrace(t *testing.T) {
	prev := LastApplied()
	t.Cleanup(func() {
		StoreLastApplied(prev)
	})

	StoreLastApplied([]Table{
		{
			Name:   "ze_filter",
			Family: FamilyInet,
			Chains: []Chain{
				{
					Name:     "input",
					IsBase:   true,
					Type:     ChainFilter,
					Hook:     HookInput,
					Priority: 0,
					Policy:   PolicyDrop,
					Terms: []Term{
						{
							Name: "allow-edge-ports",
							Matches: []Match{
								MatchProtocol{Protocol: "tcp"},
								MatchSourceAddress{Prefix: mustPrefix(t, "10.0.0.0/8")},
								MatchSourcePort{Ranges: []PortRange{{Lo: 1, Hi: 1}}},
								MatchDestinationPort{Ranges: []PortRange{{Lo: 65535, Hi: 65535}}},
								MatchConnState{States: ConnStateNew},
							},
							Actions: []Action{Accept{}},
						},
					},
				},
			},
		},
	})

	defs := []command.ArgDef{
		commandtest.Must(command.NewStringArg("table", []command.UintRange{{Min: 1, Max: 64}}, nil, command.ArgOptions{Mandatory: true})),
		commandtest.Must(command.NewStringArg("chain", []command.UintRange{{Min: 1, Max: 64}}, nil, command.ArgOptions{})),
		commandtest.Must(command.NewEnumArg("protocol", []string{"tcp", "udp", "icmp", "icmpv6"}, command.ArgOptions{})),
		commandtest.Must(command.NewStringArg("source-address", nil, nil, command.ArgOptions{})),
		commandtest.Must(command.NewStringArg("destination-address", nil, nil, command.ArgOptions{})),
		commandtest.Must(command.NewUintArg("source-port", 16, []command.UintRange{{Min: 1, Max: 65535}}, command.ArgOptions{})),
		commandtest.Must(command.NewUintArg("destination-port", 16, []command.UintRange{{Min: 1, Max: 65535}}, command.ArgOptions{})),
		commandtest.Must(command.NewStringArg("input-interface", nil, nil, command.ArgOptions{})),
		commandtest.Must(command.NewStringArg("output-interface", nil, nil, command.ArgOptions{})),
		commandtest.Must(command.NewEnumArg("connection-state", []string{"new", "established", "related", "invalid"}, command.ArgOptions{})),
	}

	// Valid boundary ports 1 and 65535 via real YANG-keyed ValidateArgs.
	validated, err := command.ValidateArgs([]string{
		"table", "filter",
		"protocol", "tcp",
		"source-address", "10.1.2.3",
		"destination-address", "192.0.2.1",
		"source-port", "1",
		"destination-port", "65535",
		"connection-state", "new",
	}, defs, nil)
	if err != nil {
		t.Fatalf("ValidateArgs failed: %v", err)
	}

	resp, err := handleShowFirewallTrace(nil, validated)
	if err != nil {
		t.Fatalf("handleShowFirewallTrace returned err: %v", err)
	}
	if resp.Status != plugin.StatusDone {
		t.Fatalf("Status = %q (error=%q), want %q", resp.Status, resp.Error, plugin.StatusDone)
	}
	rawJSON, err := json.Marshal(resp.Data)
	if err != nil {
		t.Fatalf("marshal response data: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(rawJSON, &decoded); err != nil {
		t.Fatalf("unmarshal response data: %v", err)
	}
	if decoded["verdict"] != verdictAccept {
		t.Errorf("verdict = %v, want accept", decoded["verdict"])
	}
	if decoded["matched-term"] != "allow-edge-ports" {
		t.Errorf("matched-term = %v, want allow-edge-ports", decoded["matched-term"])
	}

	// Negative boundary cases through direct handler invocation.
	badCases := []struct {
		name   string
		tokens []string
		want   string
	}{
		{
			name:   "missing table",
			tokens: []string{"protocol", "tcp"},
			want:   "missing required table",
		},
		{
			name:   "source-port 0",
			tokens: []string{"table", "filter", "source-port", "0"},
			want:   "out of range",
		},
		{
			name:   "destination-port 65536",
			tokens: []string{"table", "filter", "destination-port", "65536"},
			want:   "out of range",
		},
		{
			name:   "non-numeric port",
			tokens: []string{"table", "filter", "source-port", "http"},
			want:   "invalid source-port",
		},
		{
			name:   "invalid source IP",
			tokens: []string{"table", "filter", "source-address", "not-an-ip"},
			want:   "invalid source-address",
		},
		{
			name:   "invalid destination IP",
			tokens: []string{"table", "filter", "destination-address", "999.999.999.999"},
			want:   "invalid destination-address",
		},
		{
			name:   "invalid protocol",
			tokens: []string{"table", "filter", "protocol", "gre"},
			want:   "invalid protocol",
		},
		{
			name:   "invalid connection-state",
			tokens: []string{"table", "filter", "connection-state", "bogus"},
			want:   "invalid connection-state",
		},
	}

	for _, tc := range badCases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := handleShowFirewallTrace(nil, commandtest.Args(tc.tokens...))
			if err != nil {
				t.Fatalf("handleShowFirewallTrace returned err: %v", err)
			}
			if resp.Status != plugin.StatusError {
				t.Fatalf("Status = %q, want %q", resp.Status, plugin.StatusError)
			}
			if !strings.Contains(resp.Error, tc.want) {
				t.Errorf("Error = %q, want substring %q", resp.Error, tc.want)
			}
		})
	}
}
