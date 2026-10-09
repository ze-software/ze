//go:build linux

// Design: docs/functional-tests.md -- independent FRR recipient assertions.
// Related: plugin_fixture_rfc2545_joint_subnet_frr_linux.go -- process owner.
// JSON contract: FRR bgpd/bgp_route.c route_vty_out_detail emits paths[].nexthops
// with ip/afi/scope, adding link-local only for the received 32-octet field.
// https://github.com/FRRouting/frr/blob/frr-10.2.1/bgpd/bgp_route.c
// Neighbor contract: bgp_vty.c emits bgpState and messageStats.updatesRecv.
// Unsupported installed schemas fail explicitly; raw JSON/version remain evidence.
package fixture

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/vishvananda/netns"
)

const jointSubnetFRRConfig = `frr defaults traditional
hostname joint-subnet-recipient
log stdout
router bgp 65003
 bgp router-id 10.254.5.3
 no bgp default ipv4-unicast
 no bgp ebgp-requires-policy
 no bgp enforce-first-as
 neighbor 2001:db8:a::1 remote-as 65001
 neighbor 2001:db8:a::1 passive
 address-family ipv6 unicast
  neighbor 2001:db8:a::1 activate
 exit-address-family
!
`

const jointSubnetFRRZeConfig = `plugin {
 internal rs { use bgp-rs; }
}
bgp {
 peer inject {
  connection { remote { ip GLOBAL; port PORT; } local { ip SOURCE_LOCAL; accept false; } }
  session {
   asn { local 65001; remote 65004; }
   router-id 10.254.5.1;
   next-hop unchanged;
   family { ipv6/unicast { prefix { maximum 10000; } } }
  }
  behavior { rs-fast-path enable; }
  timer { connect-retry 2; }
 }
 peer receive {
  connection { remote { ip 2001:db8:a::2; port PORT; } local { ip 2001:db8:a::1; accept false; } }
  session {
   asn { local 65001; remote 65003; }
   router-id 10.254.5.1;
   next-hop unchanged;
   family { ipv6/unicast { prefix { maximum 10000; } } }
  }
  behavior { rs-fast-path enable; }
  timer { connect-retry 2; }
 }
}
`

func writeJointSubnetFRRFiles(plan *jointSubnetFRRPlan) error {
	var global, sourceLocal string
	switch plan.scenario {
	case jointSubnetFRRSameLink:
		global, sourceLocal = "2001:db8:a::9", "2001:db8:a::1"
	case jointSubnetFRRSplitLink:
		global, sourceLocal = "2001:db8:b::9", "2001:db8:b::1"
	case jointSubnetFRRUnspecified:
		return fmt.Errorf("joint-subnet-frr: uninitialized scenario")
	default:
		// Named scalars admit arbitrary conversions; refuse invalid plans.
		return fmt.Errorf("joint-subnet-frr: unsupported scenario %d", plan.scenario)
	}
	ze := strings.NewReplacer("GLOBAL", global, "SOURCE_LOCAL", sourceLocal, "PORT", strconv.Itoa(int(plan.port))).Replace(jointSubnetFRRZeConfig)
	// RFC 2545 Section 3: independent literal wire input, not Ze's encoder.
	pair := jointSubnetFRRUpdate(global, true)
	control := jointSubnetFRRUpdate(global, false)
	inject := "option=asn:value=65004\noption=bind:value=" + global + "\noption=linger:value=true\n" +
		"expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304\n" +
		"action=send:conn=1:seq=2:hex=" + hex.EncodeToString(pair) + "\n" +
		"action=send:conn=1:seq=3:hex=" + hex.EncodeToString(control) + "\n"
	for _, file := range []struct{ name, body string }{
		{"frr.conf", jointSubnetFRRConfig}, {"ze.conf", ze}, {"inject.peer", inject},
	} {
		if err := os.WriteFile(filepath.Join(plan.output, file.name), []byte(file.body), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// jointSubnetFRRUpdate builds two fixed RFC 4271 Section 4.3 messages. Header
// offsets: marker [0:16], length [16:18], type [18]; body withdrawn length [19:21],
// attribute length [21:23], attributes [23:]. MP_REACH value is AFI(2), SAFI(1),
// NH-length(1), global(16), optional LL(16), reserved(1), /48 NLRI(7).
// RFC 2545 Section 3: "The value of the Length of Next Hop Network Address field
// on a MP_REACH_NLRI attribute shall be set to 16, when only a global address is
// present, or 32 if a link-local address is also included in the Next Hop field."
func jointSubnetFRRUpdate(global string, subject bool) []byte {
	attributes := []byte{0x40, 1, 1, 0, 0x40, 2, 6, 2, 1, 0, 0, 0xfd, 0xec}
	nextHop := netip.MustParseAddr(global).As16()
	field := append([]byte{}, nextHop[:]...)
	nlri := []byte{48, 0x20, 1, 0x0d, 0xb8, 0x57, 2}
	if subject {
		linkLocal := netip.MustParseAddr("fe80::9").As16()
		field = append(field, linkLocal[:]...)
		nlri[6] = 1
	} else {
		// RFC 1997, "COMMUNITIES attribute": "This document creates the
		// COMMUNITIES path attribute is an optional transitive attribute of
		// variable length." "The COMMUNITIES attribute has Type Code 8."
		// Received MED cannot serve as this fence (RFC 4271 Section 5.1.4).
		attributes = append(attributes, 0xc0, 8, 4, 0xfd, 0xe9, 0, 7) // 65001:7
	}
	attributes = append(attributes, 0x80, 14, byte(5+len(field)+len(nlri)), 0, 2, 1, byte(len(field)))
	attributes = append(attributes, field...)
	attributes = append(attributes, 0)
	attributes = append(attributes, nlri...)
	length := 23 + len(attributes)
	message := []byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, byte(length >> 8), byte(length), 2, 0, 0, byte(len(attributes) >> 8), byte(len(attributes))}
	return append(message, attributes...)
}

// These JSON DTOs deliberately preserve missing/null values. They are not
// validated domain values: callers MUST use parseJointSubnetFRRRoute before the
// route assertions and MUST NOT mutate its borrowed pointer/slice fields.
type jointSubnetFRRNeighbor struct {
	State    *string `json:"bgpState"`
	Messages struct {
		Updates *uint64 `json:"updatesRecv"`
	} `json:"messageStats"`
}

type jointSubnetFRRRoute struct {
	Prefix *string `json:"prefix"`
	Paths  []struct {
		Community *jointSubnetFRRCommunity `json:"community"`
		Peer      struct {
			ID *string `json:"peerId"`
		} `json:"peer"`
		NextHops []struct {
			IP    *netip.Addr `json:"ip"`
			AFI   *string     `json:"afi"`
			Scope *string     `json:"scope"`
		} `json:"nexthops"`
	} `json:"paths"`
}

// FRR 10.2.1 route_vty_out_detail attaches community JSON produced by
// bgp_community.c set_community_string: list contains AS:VAL strings.
// Source: https://github.com/FRRouting/frr/blob/frr-10.2.1/bgpd/bgp_community.c
type jointSubnetFRRCommunity struct {
	List []*string `json:"list"`
}

// jointSubnetFRRReady requires receipt of Ze's initial UPDATE as well as the
// recipient's Established state. No injection has begun, so that UPDATE is EOR.
func jointSubnetFRRReady(ctx context.Context, plan *jointSubnetFRRPlan, namespace *testNetns, orig netns.NsHandle) (bool, error) {
	exists, err := peerFileExists(filepath.Join(plan.runtimeDir, "bgpd.vty"))
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}
	data, err := jointSubnetFRRJSON(ctx, plan, namespace, orig, "show bgp neighbors 2001:db8:a::1 json", "neighbor.json")
	if err != nil {
		return false, err
	}
	return jointSubnetFRRNeighborReady(data)
}

// jointSubnetFRRNeighborReady follows FRR 10.2.1 bgp_show_neighbor_vty:
// {} means no default BGP instance, not an incompatible peer schema.
// frr_config_fork queues config reading; a VTY socket is not a config fence.
// Only this known absence may wait under the existing readiness deadline.
// Source: https://github.com/FRRouting/frr/blob/frr-10.2.1/bgpd/bgp_vty.c
func jointSubnetFRRNeighborReady(data []byte) (bool, error) {
	var neighbors map[string]json.RawMessage
	if err := json.Unmarshal(data, &neighbors); err != nil {
		return false, fmt.Errorf("unsupported FRR neighbor JSON: %w", err)
	}
	if neighbors == nil {
		return false, fmt.Errorf("unsupported FRR neighbor JSON: null response")
	}
	if len(neighbors) == 0 {
		return false, nil
	}
	if absent, found := neighbors["bgpNoSuchNeighbor"]; found {
		var missing *bool
		if err := json.Unmarshal(absent, &missing); err != nil {
			return false, fmt.Errorf("unsupported FRR neighbor absence JSON: %w", err)
		}
		if missing == nil {
			return false, fmt.Errorf("unsupported FRR neighbor absence JSON: null flag")
		}
		if !*missing {
			return false, fmt.Errorf("unsupported FRR neighbor absence JSON: false flag")
		}
		return false, fmt.Errorf("FRR recipient setup: configured neighbor absent from default BGP instance")
	}
	raw, found := neighbors["2001:db8:a::1"]
	if !found {
		return false, fmt.Errorf("unsupported FRR neighbor JSON: unexpected nonempty response")
	}
	var neighbor jointSubnetFRRNeighbor
	if err := json.Unmarshal(raw, &neighbor); err != nil {
		return false, fmt.Errorf("unsupported FRR neighbor JSON: %w", err)
	}
	if neighbor.State == nil {
		return false, fmt.Errorf("unsupported FRR neighbor JSON: bgpState absent")
	}
	if neighbor.Messages.Updates == nil {
		return false, fmt.Errorf("unsupported FRR neighbor JSON: updatesRecv absent")
	}
	return *neighbor.State == "Established" && *neighbor.Messages.Updates > 0, nil
}

// jointSubnetFRRReceived evaluates only after FRR has retained the control route.
// The exact global/link-local list is independent implementation evidence, not
// next-hop reachability or FIB installation. PCAP preserves the complete history
// for the execution owner's separate exact-wire/no-withdrawal check.
func jointSubnetFRRReceived(ctx context.Context, plan *jointSubnetFRRPlan, namespace *testNetns, orig netns.NsHandle) (bool, error) {
	global, err := plan.scenario.nextHop()
	if err != nil {
		return false, err
	}
	control, err := readJointSubnetFRRRoute(ctx, plan, namespace, orig, "2001:db8:5702::/48", "control.json")
	if err != nil {
		return false, err
	}
	if control.Prefix == nil {
		return false, nil
	}
	subject, err := readJointSubnetFRRRoute(ctx, plan, namespace, orig, "2001:db8:5701::/48", "subject.json")
	if err != nil {
		return false, err
	}
	if err := assertJointSubnetFRRControl(control, global); err != nil {
		return false, err
	}
	// RFC 2545 Section 3: exact retained pair versus global-only, not a floor.
	if err := assertJointSubnetFRRRoute(subject, "2001:db8:5701::/48", global, plan.scenario == jointSubnetFRRSameLink); err != nil {
		return false, err
	}
	return true, nil
}

// assertJointSubnetFRRControl requires the explicit nondefault transitive
// marker, not mere prefix presence or FRR's unrelated nexthop IGP metric.
func assertJointSubnetFRRControl(control jointSubnetFRRRoute, global netip.Addr) error {
	if err := assertJointSubnetFRRRoute(control, "2001:db8:5702::/48", global, false); err != nil {
		return err
	}
	community := control.Paths[0].Community
	if community == nil {
		return fmt.Errorf("unsupported FRR completion control JSON: community absent or null")
	}
	if community.List == nil {
		return fmt.Errorf("unsupported FRR completion control JSON: community.list absent or null")
	}
	for _, value := range community.List {
		if value == nil {
			return fmt.Errorf("unsupported FRR completion control JSON: null community.list element")
		}
	}
	if len(community.List) != 1 {
		return fmt.Errorf("FRR completion control community count=%d, want exactly one", len(community.List))
	}
	if *community.List[0] != "65001:7" {
		return fmt.Errorf("FRR completion control community=%q, want 65001:7", *community.List[0])
	}
	return nil
}

func readJointSubnetFRRRoute(ctx context.Context, plan *jointSubnetFRRPlan, namespace *testNetns, orig netns.NsHandle, prefix, artifact string) (jointSubnetFRRRoute, error) {
	var route jointSubnetFRRRoute
	data, err := jointSubnetFRRJSON(ctx, plan, namespace, orig, "show bgp ipv6 unicast "+prefix+" json", artifact)
	if err != nil {
		return route, err
	}
	return parseJointSubnetFRRRoute(data)
}

// parseJointSubnetFRRRoute separates the one supported absence representation
// ({}) from incompatible schemas before any received-field judgment. Non-nil
// pointers and slices distinguish missing/null fields from present wrong data.
func parseJointSubnetFRRRoute(data []byte) (jointSubnetFRRRoute, error) {
	var route jointSubnetFRRRoute
	if err := json.Unmarshal(data, &route); err != nil {
		return route, fmt.Errorf("unsupported FRR route JSON: %w", err)
	}
	if route.Prefix == nil {
		var object map[string]json.RawMessage
		if err := json.Unmarshal(data, &object); err != nil {
			return route, fmt.Errorf("unsupported FRR empty route JSON: %w", err)
		}
		if object == nil {
			return route, fmt.Errorf("unsupported FRR null route JSON")
		}
		if len(object) != 0 {
			return route, fmt.Errorf("unsupported FRR route JSON: prefix absent or null")
		}
		return route, nil
	}
	if route.Paths == nil {
		return route, fmt.Errorf("unsupported FRR route JSON: paths absent or null")
	}
	for i, path := range route.Paths {
		if path.Peer.ID == nil {
			return route, fmt.Errorf("unsupported FRR route JSON: paths[%d].peer.peerId absent or null", i)
		}
		if path.NextHops == nil {
			return route, fmt.Errorf("unsupported FRR route JSON: paths[%d].nexthops absent or null", i)
		}
		for j, hop := range path.NextHops {
			if hop.IP == nil {
				return route, fmt.Errorf("unsupported FRR route JSON: paths[%d].nexthops[%d].ip absent or null", i, j)
			}
			if hop.AFI == nil {
				return route, fmt.Errorf("unsupported FRR route JSON: paths[%d].nexthops[%d].afi absent or null", i, j)
			}
			if hop.Scope == nil {
				return route, fmt.Errorf("unsupported FRR route JSON: paths[%d].nexthops[%d].scope absent or null", i, j)
			}
		}
	}
	return route, nil
}

func assertJointSubnetFRRRoute(route jointSubnetFRRRoute, prefix string, global netip.Addr, pair bool) error {
	if route.Prefix == nil {
		return fmt.Errorf("FRR semantic mismatch: %s absent after control receipt", prefix)
	}
	if *route.Prefix != prefix {
		return fmt.Errorf("FRR semantic mismatch: prefix %q, want %s after control receipt", *route.Prefix, prefix)
	}
	if len(route.Paths) != 1 {
		return fmt.Errorf("FRR semantic mismatch: %s has %d paths, want one", prefix, len(route.Paths))
	}
	path := &route.Paths[0]
	if *path.Peer.ID != "2001:db8:a::1" {
		return fmt.Errorf("FRR semantic mismatch: path peer %q, want speaker S", *path.Peer.ID)
	}
	want := 1
	if pair {
		want = 2
	}
	if len(path.NextHops) != want {
		return fmt.Errorf("FRR semantic mismatch: %s next-hop count=%d, want %d", prefix, len(path.NextHops), want)
	}
	if *path.NextHops[0].IP != global {
		return fmt.Errorf("FRR semantic mismatch: %s global=%s, want %s", prefix, *path.NextHops[0].IP, global)
	}
	if *path.NextHops[0].AFI != "ipv6" {
		return fmt.Errorf("FRR semantic mismatch: global next-hop address-family")
	}
	if *path.NextHops[0].Scope != "global" {
		return fmt.Errorf("FRR semantic mismatch: global next-hop scope")
	}
	if pair {
		if *path.NextHops[1].IP != netip.MustParseAddr("fe80::9") {
			return fmt.Errorf("FRR semantic mismatch: link-local=%s, want fe80::9", *path.NextHops[1].IP)
		}
		if *path.NextHops[1].AFI != "ipv6" {
			return fmt.Errorf("FRR semantic mismatch: link-local next-hop address-family")
		}
		if *path.NextHops[1].Scope != "link-local" {
			return fmt.Errorf("FRR semantic mismatch: link-local next-hop scope")
		}
	}
	return nil
}

func jointSubnetFRRJSON(ctx context.Context, plan *jointSubnetFRRPlan, namespace *testNetns, orig netns.NsHandle, command, artifact string) ([]byte, error) {
	data, err := jointSubnetFRRCommand(ctx, []string{plan.vtysh, "--vty_socket", plan.runtimeDir, "-d", "bgpd", "-c", command}, namespace, orig)
	if writeErr := os.WriteFile(filepath.Join(plan.output, artifact), data, 0o644); writeErr != nil {
		return nil, writeErr
	}
	if err != nil {
		return nil, err
	}
	if !json.Valid(data) {
		return nil, fmt.Errorf("FRR command %q did not return JSON; retained %s", command, artifact)
	}
	return data, nil
}

// jointSubnetFRRBuffer bounds one command's complete output at 64 KiB. Commands
// query one peer or one prefix; a larger result fails setup instead of growing.
type jointSubnetFRRBuffer struct {
	data [65536]byte
	used int
}

func (b *jointSubnetFRRBuffer) Write(data []byte) (int, error) {
	if len(data) > len(b.data)-b.used {
		return 0, fmt.Errorf("FRR command output exceeded 64 KiB")
	}
	copy(b.data[b.used:], data)
	b.used += len(data)
	return len(data), nil
}

func jointSubnetFRRCommand(parent context.Context, argv []string, namespace *testNetns, orig netns.NsHandle) ([]byte, error) {
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()
	if err := netns.Set(namespace.ns); err != nil {
		return nil, err
	}
	var output jointSubnetFRRBuffer
	command := exec.CommandContext(ctx, argv[0], argv[1:]...)
	command.Stdout, command.Stderr = &output, &output
	runErr := command.Run()
	if err := netns.Set(orig); err != nil {
		return nil, err
	}
	if runErr != nil {
		return output.data[:output.used], fmt.Errorf("FRR command %q: %w", argv, runErr)
	}
	return output.data[:output.used], nil
}
