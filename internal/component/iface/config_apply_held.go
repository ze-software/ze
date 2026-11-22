// Design: docs/features/interfaces.md -- Interface reconciliation and application
// Related: config_apply.go -- applyConfigPass, whose steps write only what differs
// Related: config_bridge_members.go -- enslaveBridgeMembers, which skips a port already enslaved

package iface

import (
	"bytes"
	"errors"
	"net"

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

// logUnreadState records a state read that failed before the configured value
// is written over it. Two answers are logged at debug, because the write that
// follows reports them itself: the backend not being ready yet (the designed
// vpp startup path), and a device the read did not find (the vpp backend
// names a loopback loopN in its dataplane, so a read by the configured name
// finds nothing while the write by that name reaches it). Any other failure
// is logged and raised as a warning on the report bus under the device's name,
// so show doctor reports it until a later read of that device answers.
func logUnreadState(device string, err error) {
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
		"iface", device, "err", err, "code", codeDoctorIfaceStateUnread, "fix", fix)
	report.RaiseWarningFix(reportSourceIface, codeDoctorIfaceStateUnread, device,
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
