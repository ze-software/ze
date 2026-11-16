// Design: docs/architecture/api/process-protocol.md -- owned interface devices
// Related: firewall_tables.go -- the firewall table contribution request

package rpc

import "net/netip"

// Engine operations a plugin calls to have the interface component create and
// remove devices it owns. The interface component registers the handlers from
// its own package; the owner of every device is the authenticated calling
// process, never a name in the request body.
const (
	// MethodOwnedBridgesReplace replaces the caller's complete set of owned
	// bridges with the set in the request.
	MethodOwnedBridgesReplace = "ze-iface:owned-bridges-replace"
	// MethodOwnedLinkCreate creates one link whose host end is a member of an
	// owned bridge and whose peer end is inside a network namespace.
	MethodOwnedLinkCreate = "ze-iface:owned-link-create"
	// MethodOwnedLinkRemove removes every link recorded under one key.
	MethodOwnedLinkRemove = "ze-iface:owned-link-remove"
)

// OwnedBridge is one bridge in a caller's complete set.
type OwnedBridge struct {
	Name      string         `json:"name"`
	MTU       uint32         `json:"mtu"`
	Addresses []netip.Prefix `json:"addresses,omitempty"`
}

// OwnedBridgesReplaceInput carries the caller's complete set of owned bridges.
// An empty set withdraws every bridge the caller owns.
type OwnedBridgesReplaceInput struct {
	Bridges []OwnedBridge `json:"bridges"`
}

// OwnedBridgesReplaceOutput names the bridges the caller owns after the
// replacement.
type OwnedBridgesReplaceOutput struct {
	Bridges []string `json:"bridges"`
}

// OwnedLinkCreateInput asks for one link. Key is the caller's own record name,
// under which the link is recorded and later removed. PID and NamespaceInode
// name the target network namespace together: the handler refuses the request
// when the namespace of PID does not carry that inode number, so a reused
// process id cannot redirect the link.
type OwnedLinkCreateInput struct {
	Key             string         `json:"key"`
	Bridge          string         `json:"bridge"`
	Slot            uint16         `json:"slot"`
	PID             int            `json:"pid"`
	NamespaceInode  uint64         `json:"namespace-inode"`
	Name            string         `json:"name"`
	HardwareAddress string         `json:"hardware-address,omitempty"`
	MTU             uint32         `json:"mtu"`
	Addresses       []netip.Prefix `json:"addresses,omitempty"`
	Gateways        []netip.Addr   `json:"gateways,omitempty"`
}

// OwnedLinkCreateOutput names the host end of the created link.
type OwnedLinkCreateOutput struct {
	HostEnd string `json:"host-end"`
}

// OwnedLinkRemoveInput names the record whose links are removed.
type OwnedLinkRemoveInput struct {
	Key string `json:"key"`
}

// OwnedLinkRemoveOutput names the host ends that were removed.
type OwnedLinkRemoveOutput struct {
	Removed []string `json:"removed"`
}
