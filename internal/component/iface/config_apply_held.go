// Design: docs/features/interfaces.md -- Interface reconciliation and application
// Related: config_apply.go -- applyConfigPass, whose steps write only what differs
// Related: config_bridge_members.go -- enslaveBridgeMembers, which skips a port already enslaved

package iface

import (
	"bytes"
	"errors"
	"maps"
	"net"
	"net/netip"

	"github.com/ze-software/ze/internal/core/report"
)

// The apply writes a link setting only when the device holds another value.
// A setting the device already holds owes no write, and each write needs
// CAP_NET_ADMIN: an unprivileged ze started over a host bridge (docker0) whose
// state already matches the config failed for a change that changed nothing.
// Every helper here answers "write" for a state it could not read: an unread
// state is never one that matches. The compare lives in the apply path rather
// than in each backend, as applyVLANMTU's does, so every backend benefits, and
// a backend that does not report a setting (the Unreported values) keeps
// writing it.

// linkFacts reads device's link facts for a step that writes only what
// differs. A read that fails answers nil, which every caller takes as "not
// known to match" and writes. A read that answers clears the warning an
// earlier failed read of device raised: the state is known again.
func linkFacts(b Backend, device string) *InterfaceInfo {
	held, err := b.GetLink(device)
	if err != nil {
		logUnreadState(device, err)
		return nil
	}
	report.ClearWarning(reportSourceIface, codeDoctorIfaceStateUnread, device)
	return held
}

// logUnreadState records a failed read of device's link state; see
// logUnreadStateAs.
func logUnreadState(device string, err error) {
	logUnreadStateAs(device, device, err)
}

// logUnreadStateAs records a state read that failed before the configured
// value is written over it. Two answers are logged at debug, because the write
// that follows reports them itself: the backend not being ready yet (the
// designed vpp startup path), and a device the read did not find (the vpp
// backend names a loopback loopN in its dataplane, so a read by the configured
// name finds nothing while the write by that name reaches it). Any other
// failure is logged and raised as a warning on the report bus under subject,
// so show doctor reports it until a later read of that state answers. subject
// is device for its link state; a state read by another call has a subject of
// its own (wireguardStateSubject), because a link read that answers clears the
// device's subject in the same apply.
func logUnreadStateAs(device, subject string, err error) {
	log := loggerPtr.Load()
	if errors.Is(err, ErrBackendNotReady) {
		log.Debug("iface config: current state not read, backend not ready", "iface", device, "err", err)
		return
	}
	if errors.Is(err, ErrInterfaceNotFound) {
		log.Debug("iface config: current state not read, device not found by this name", "iface", device, "err", err)
		return
	}
	fix := stateUnreadFix(device)
	log.Warn("iface config: could not read the current state of the interface, so its configured values are written without knowing whether they were already in place",
		"iface", device, "subject", subject, "err", err, "code", codeDoctorIfaceStateUnread, "fix", fix)
	report.RaiseWarningFix(reportSourceIface, codeDoctorIfaceStateUnread, subject,
		report.FitMessage(stateUnreadMessage(device, err)), fix)
}

// stateUnreadMessage is the report-bus text of a failed state read: what ze
// did without the state, that the apply reports a failed write itself, and
// the backend's answer as the cause.
func stateUnreadMessage(device string, err error) string {
	return "ze could not read the current state of interface " + device +
		" before applying its configuration, so it wrote the configured values without knowing" +
		" whether they were already in place; the apply reports any write that fails;" +
		" the backend answered: " + err.Error()
}

// stateUnreadFix is the command that reads device's state again.
func stateUnreadFix(device string) string {
	return "ze show interface name " + device + " detail"
}

// adminUpHeld reports whether device was read to be administratively up. An
// unread state answers false, so the caller brings the device up.
func adminUpHeld(b Backend, device string) bool {
	held := linkFacts(b, device)
	if held == nil {
		return false
	}
	return held.AdminState == AdminStateUp
}

// sameMAC reports whether held and want name the same hardware address,
// whatever their spelling. An address that does not parse, the "" of an
// unread one included, matches nothing.
func sameMAC(held, want string) bool {
	heldHW, err := net.ParseMAC(held)
	if err != nil {
		return false
	}
	wantHW, err := net.ParseMAC(want)
	if err != nil {
		return false
	}
	return bytes.Equal(heldHW, wantHW)
}

// vlanQoSNeedsWrite reports whether an existing VLAN must be sent u's QoS
// maps. A VLAN whose maps were read is written only when they program another
// translation. Where they were not read, the previous config is the only
// record: the VLAN is written unless that config asked for the same maps.
func vlanQoSNeedsWrite(held *InterfaceInfo, previous unitQoS, known bool, u *unitEntry) bool {
	if heldQoS, read := heldVLANQoS(held); read {
		if !qosMapsAgree(heldQoS.ingress, u.IngressQoSMap) {
			return true
		}
		return !qosMapsAgree(heldQoS.egress, u.EgressQoSMap)
	}
	if !known {
		return true
	}
	return !previous.equal(u)
}

// heldVLANQoS returns the maps a VLAN was read to hold. read is false when
// held is nil or the backend did not read the maps, and then the maps mean
// nothing.
func heldVLANQoS(held *InterfaceInfo) (qos unitQoS, read bool) {
	if held == nil {
		return unitQoS{}, false
	}
	if !held.QoSMapsRead {
		return unitQoS{}, false
	}
	return unitQoS{ingress: held.IngressQoSMap, egress: held.EgressQoSMap}, true
}

// qosMapsAgree reports whether two 802.1p maps program the same translation.
// An entry at 0 is the kernel's default, which the kernel keeps without
// dumping it, so it agrees with an absent key (qosReplaceMaps in the netlink
// backend relies on the same equivalence).
func qosMapsAgree(a, b map[uint32]uint32) bool {
	return qosMapCovered(a, b) && qosMapCovered(b, a)
}

// qosMapCovered reports whether every non-default entry of from is in to.
func qosMapCovered(from, to map[uint32]uint32) bool {
	for key, value := range from {
		if value == 0 {
			continue
		}
		if to[key] != value {
			return false
		}
	}
	return true
}

// portInBridge reports whether port is already a member of bridge, read from
// both links. A read that fails, or a bridge reported with no index, answers
// false and the port is enslaved: an unread membership is never taken for a
// present one.
func portInBridge(b Backend, bridge, port string) bool {
	bridgeHeld := linkFacts(b, bridge)
	if bridgeHeld == nil {
		return false
	}
	if bridgeHeld.Index <= 0 {
		return false
	}
	portHeld := linkFacts(b, port)
	if portHeld == nil {
		return false
	}
	return portHeld.MasterIndex == bridgeHeld.Index
}

// wireguardStateSubject is the report-bus subject of a failed read of
// device's WireGuard keys and peers.
func wireguardStateSubject(device string) string {
	return device + " wireguard"
}

// wireguardNeedsWrite reports whether ConfigureWireguardDevice must be sent
// want, read from the device it names. A read that fails answers true: an
// unread device is never one that holds the config. The read and the compare
// handle key material; neither logs nor reports it, and the backend's error
// carries none.
func wireguardNeedsWrite(b Backend, want *WireguardSpec) bool {
	subject := wireguardStateSubject(want.Name)
	held, err := b.GetWireguardDevice(want.Name)
	// refuse-by-default: guards applyConfigPass -- true sends the configured keys and peers, so a device whose state could not be read is written, never taken as matching; the failure is warned and raised above
	if err != nil {
		logUnreadStateAs(want.Name, subject, err)
		return true
	}
	report.ClearWarning(reportSourceIface, codeDoctorIfaceStateUnread, subject)
	return !wireguardHolds(&held, want)
}

// wireguardHolds reports whether held, a device's WireGuard state, is what
// ConfigureWireguardDevice(want) would leave, so the write would change
// nothing. It follows the netlink write field by field. The private key and
// the firewall mark are always written (an absent fwmark writes 0). The
// listen port is written only when the config sets it, so a port the kernel
// picked matches an absent leaf. The peer set is replaced
// (WGDEVICE_F_REPLACE_PEERS removes every current peer first), so the device
// must hold exactly the enabled peers, each as a new peer holds it: no
// preshared key, endpoint or keepalive unless configured, and exactly the
// configured allowed IPs. A peer the device learned an endpoint for since
// (roaming) therefore differs, because the write resets it. A backend that does
// not report a field the config sets (the vpp backend reports no fwmark and
// no preshared key) reads as differing, and keeps writing.
func wireguardHolds(held, want *WireguardSpec) bool {
	if held.PrivateKey != want.PrivateKey {
		return false
	}
	if held.FirewallMark != want.FirewallMark {
		return false
	}
	if want.ListenPortSet {
		if held.ListenPort != want.ListenPort {
			return false
		}
	}
	return wireguardPeersHeld(held.Peers, want.Peers)
}

// wireguardPeersHeld reports whether the device's peers are exactly want's
// enabled peers, each holding its configuration. A disabled peer is written
// as a removal, so the device must not hold it. A public key that appears
// twice on either side matches nothing.
func wireguardPeersHeld(held, want []WireguardPeerSpec) bool {
	enabled := make(map[WireguardKey]*WireguardPeerSpec, len(want))
	listed := make(map[WireguardKey]bool, len(want))
	for i := range want {
		if listed[want[i].PublicKey] {
			return false
		}
		listed[want[i].PublicKey] = true
		if want[i].Disable {
			continue
		}
		enabled[want[i].PublicKey] = &want[i]
	}
	if len(held) != len(enabled) {
		return false
	}
	for i := range held {
		peer, found := enabled[held[i].PublicKey]
		if !found {
			return false
		}
		delete(enabled, held[i].PublicKey)
		if !wireguardPeerHeld(&held[i], peer) {
			return false
		}
	}
	return true
}

// wireguardPeerHeld reports whether held is the peer the write makes of want.
// The peer's name is a config label the kernel never sees.
func wireguardPeerHeld(held, want *WireguardPeerSpec) bool {
	if wireguardPresharedKey(held) != wireguardPresharedKey(want) {
		return false
	}
	if held.PersistentKeepalive != want.PersistentKeepalive {
		return false
	}
	if !wireguardEndpointHeld(held, want) {
		return false
	}
	return wireguardAllowedIPsHeld(held.AllowedIPs, want.AllowedIPs)
}

// wireguardPresharedKey answers the preshared key a peer holds or is given:
// the all-zero key when it has none, which is how the kernel holds "no
// preshared key".
func wireguardPresharedKey(peer *WireguardPeerSpec) WireguardKey {
	if peer.HasPresharedKey {
		return peer.PresharedKey
	}
	return WireguardKey{}
}

// wireguardEndpointHeld reports whether held has the endpoint the write gives
// want. A peer with no configured endpoint is written with none, so it must
// hold none. Addresses compare as addresses: the write sends an IPv4-mapped
// IPv6 address as IPv4, so both sides are unmapped. An address that does not
// parse matches nothing.
func wireguardEndpointHeld(held, want *WireguardPeerSpec) bool {
	if want.EndpointIP == "" {
		return held.EndpointIP == ""
	}
	wantAddr, err := netip.ParseAddr(want.EndpointIP)
	if err != nil {
		return false
	}
	heldAddr, err := netip.ParseAddr(held.EndpointIP)
	if err != nil {
		return false
	}
	if heldAddr.Unmap() != wantAddr.Unmap() {
		return false
	}
	return held.EndpointPort == want.EndpointPort
}

// wireguardAllowedIPsHeld reports whether held and want are the same set of
// prefixes. The kernel keeps a set and stores each prefix masked to its
// length, so order, duplicates and host bits do not count. A prefix that does
// not parse matches nothing.
func wireguardAllowedIPsHeld(held, want []string) bool {
	heldSet, ok := wireguardPrefixSet(held)
	if !ok {
		return false
	}
	wantSet, ok := wireguardPrefixSet(want)
	if !ok {
		return false
	}
	return maps.Equal(heldSet, wantSet)
}

// wireguardPrefixSet parses cidrs as the netlink write does (net.ParseCIDR,
// which masks the address) into a set. ok is false when one does not parse.
// An IPv4 prefix stays IPv4 and an IPv6 one IPv6, so an IPv4-mapped IPv6
// prefix never matches its IPv4 spelling.
func wireguardPrefixSet(cidrs []string) (set map[netip.Prefix]struct{}, ok bool) {
	set = make(map[netip.Prefix]struct{}, len(cidrs))
	for _, cidr := range cidrs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, false
		}
		addr, isAddr := netip.AddrFromSlice(network.IP)
		if !isAddr {
			return nil, false
		}
		ones, bits := network.Mask.Size()
		if bits != addr.BitLen() {
			return nil, false
		}
		set[netip.PrefixFrom(addr, ones)] = struct{}{}
	}
	return set, true
}
