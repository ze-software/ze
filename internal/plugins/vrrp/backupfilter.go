// Design: docs/architecture/vrrp/vrrp-macvlan-vmac-dataplane.md -- what the virtual-MAC macvlan receives in Backup
// RFC: rfc/short/rfc9568.md (VRRPv3) -- Section 6.4.2 Backup
// Related: ownerfilter.go -- the same firewall-registry route for the owner's answers
// Related: instance.go -- run, doInstallVIPs and doRemoveVIPs own this filter's lifetime
//
// The virtual-MAC macvlan exists for the whole life of a group, in every state:
// it is created at config apply so its MAC, and the IPv6 link-local address
// derived from it, stay stable across failovers. Only the virtual addresses wait
// for promotion. A frame addressed to the Virtual Router MAC that reaches a
// Backup, for example by unknown-unicast flooding after a failover, is therefore
// received by the macvlan, and with forwarding enabled the kernel forwards it.
//
// RFC 3768, RFC 5798 and RFC 9568 Section 6.4.2 forbid that. While the instance
// is not Active this file drops, at the inet prerouting hook, every packet the
// macvlan receives. A private macvlan receives a unicast frame only when its
// destination MAC is the macvlan's own, the Virtual Router MAC. Broadcast and
// multicast frames reach the parent as well, so the parent still handles them,
// and the advertisements this router listens for arrive on the parent socket
// (transport/backend_linux.go openV4, openV6). ARP is not carried by the inet
// family, and a Backup holds no virtual address to answer for.
//
// The rules reach the kernel through the firewall component's table registry,
// under an owner separate from the other two filters', because each set changes
// at its own moments and one owner registration replaces the other.
package vrrp

import (
	"slices"
	"sync"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	// backupFilterTableName carries the "ze_" ownership prefix RegisterTables
	// requires.
	backupFilterTableName = "ze_vrrp_backup"
	// backupFilterChainName is the base chain at the prerouting hook, which
	// runs before the routing decision, so a dropped packet is neither
	// forwarded nor delivered locally.
	backupFilterChainName = "prerouting"
	// backupFilterOwner is this filter's identity in the firewall table
	// registry, distinct from acceptFilterOwner and ownerFilterOwner.
	backupFilterOwner = "vrrp-backup"
)

// backupFilterState holds, for each VRRP instance that is not Active, the
// virtual-MAC device whose received packets are dropped.
//
// Process-wide, because one kernel table carries every group's rules. Bounded
// by the configured group count, which is operator configuration.
var (
	backupFilterMu    sync.Mutex
	backupFilterState = map[string]string{}
)

// setBackupFilter records that device, one instance's virtual-MAC macvlan, MUST
// NOT pass on what it receives, and reconciles the kernel. The caller MUST call
// clearBackupFilter when the instance becomes Active, or it drops the traffic
// an Active router has to forward, and when the instance stops.
func setBackupFilter(instanceOwner, device string) error {
	backupFilterMu.Lock()
	defer backupFilterMu.Unlock()

	if current, ok := backupFilterState[instanceOwner]; ok && current == device {
		return nil
	}
	backupFilterState[instanceOwner] = device
	return reconcileBackupFilterLocked()
}

// clearBackupFilter withdraws one instance's entry. It MUST be called after
// setBackupFilter once the instance is Active or stopped. It reconciles only if
// the entry was there to remove.
func clearBackupFilter(instanceOwner string) error {
	backupFilterMu.Lock()
	defer backupFilterMu.Unlock()

	if _, ok := backupFilterState[instanceOwner]; !ok {
		return nil
	}
	delete(backupFilterState, instanceOwner)
	return reconcileBackupFilterLocked()
}

// reconcileBackupFilterLocked publishes the current entries and reconciles the
// kernel. Synchronous under backupFilterMu for the reason
// reconcileAcceptFilterLocked gives: two instances applying snapshots out of
// order would leave the kernel holding the older one.
func reconcileBackupFilterLocked() error {
	devices := make([]string, 0, len(backupFilterState))
	for _, device := range backupFilterState {
		devices = append(devices, device)
	}
	return backupFilterPublish(backupFilterTables(devices))
}

// backupFilterPublish hands the desired tables to the firewall component. It is
// a var so a test can watch what this package publishes without a kernel.
var backupFilterPublish = func(tables []firewall.Table) error {
	if err := firewall.RegisterTables(backupFilterOwner, tables); err != nil {
		return err
	}
	return firewall.ApplyAll()
}

// backupFilterTables builds the inet table that drops what each device
// receives. No device gives no table, so the last Backup to leave withdraws it
// instead of leaving an empty table behind.
//
// The devices are sorted and deduplicated: map iteration order is not stable,
// and the kernel rules would otherwise be rewritten on every apply.
func backupFilterTables(devices []string) []firewall.Table {
	slices.Sort(devices)
	devices = slices.Compact(devices)
	if len(devices) == 0 {
		return nil
	}

	terms := make([]firewall.Term, 0, len(devices))
	for _, device := range devices {
		terms = append(terms, backupDiscardTerm(device))
	}
	return []firewall.Table{{
		Name:   backupFilterTableName,
		Family: firewall.FamilyInet,
		Chains: []firewall.Chain{{
			Name:     backupFilterChainName,
			IsBase:   true,
			Type:     firewall.ChainFilter,
			Hook:     firewall.HookPrerouting,
			Priority: 0,
			Policy:   firewall.PolicyAccept,
			Terms:    terms,
		}},
	}}
}

// backupDiscardTerm drops every packet received by one virtual-MAC device.
func backupDiscardTerm(device string) firewall.Term {
	// RFC 9568 Section 6.4.2: "It MUST discard packets with a destination
	// link-layer MAC address equal to the Virtual Router MAC address."
	var tb textbuf.Buffer
	return firewall.Term{
		Name:    tb.Str("discard-").Str(device).String(),
		Matches: []firewall.Match{firewall.MatchInputInterface{Name: device}},
		Actions: []firewall.Action{firewall.Drop{}},
	}
}
