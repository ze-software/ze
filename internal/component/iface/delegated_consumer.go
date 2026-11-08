// Design: docs/guide/pppoe.md -- numbering a LAN unit from a delegated prefix
// Related: delegated_lease.go -- the delegation table this worker fills
// Related: delegated_prefix.go -- deriveDelegatedAddress, the /64 each unit takes
// Related: register.go -- subscribes the lease events and starts this worker
//
// RFC: rfc/short/rfc9915.md, rfc/short/rfc7084.md

package iface

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"
)

// errUnitNotConfigured is the answer for an entry whose unit left the config
// or was disabled between the event and the install.
var errUnitNotConfigured = errors.New("the unit is not configured, or is disabled")

// delegationKey names one delegated prefix of one source.
type delegationKey struct {
	source string
	prefix netip.Prefix
}

// delegationQueue carries DHCPv6 delegated-prefix lease events from the event
// bus to the delegation worker. A bus handler records the latest event for its
// source and prefix and signals; the worker does every netlink call. Pending
// is bounded by the delegations the DHCPv6 clients hold, never by the event
// rate, because a later event for one prefix replaces the earlier one.
// Safe for concurrent use.
type delegationQueue struct {
	mu      sync.Mutex
	pending map[delegationKey]DHCPPayload
	notify  chan struct{} // depth 1: one wake-up stands for any number of events
}

func newDelegationQueue() *delegationQueue {
	return &delegationQueue{
		pending: make(map[delegationKey]DHCPPayload),
		notify:  make(chan struct{}, 1),
	}
}

// pushLease records one lease event and wakes the worker. It does no I/O, so
// it is safe from an event bus handler. An event that is not a delegated
// prefix is not this queue's.
func (q *delegationQueue) pushLease(data string, log *slog.Logger) {
	var p DHCPPayload
	if err := json.Unmarshal([]byte(data), &p); err != nil {
		log.Warn("interface: unreadable DHCP lease event, delegated prefixes not updated", "error", err)
		return
	}
	if !p.Delegated {
		return
	}
	addr, err := netip.ParseAddr(p.Address)
	if err != nil {
		log.Warn("interface: DHCPv6 delegated prefix event with no valid prefix, ignored", "source", p.Name, "address", p.Address, "error", err)
		return
	}
	prefix := netip.PrefixFrom(addr, p.PrefixLength)
	if !prefix.IsValid() {
		log.Warn("interface: DHCPv6 delegated prefix event with an invalid length, ignored", "source", p.Name, "address", p.Address, "prefix-length", p.PrefixLength)
		return
	}
	q.mu.Lock()
	q.pending[delegationKey{source: p.Name, prefix: prefix.Masked()}] = p
	q.mu.Unlock()
	q.wake()
}

// wake asks the worker for a pass without a lease event: a config apply may
// add a unit that numbers from a delegation already held.
func (q *delegationQueue) wake() {
	select {
	case q.notify <- struct{}{}:
	default:
	}
}

// take returns and clears every pending event.
func (q *delegationQueue) take() map[delegationKey]DHCPPayload {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := q.pending
	q.pending = make(map[delegationKey]DHCPPayload)
	return out
}

// delegatedUnitKey names one delegated-prefix entry of one unit.
type delegatedUnitKey struct {
	ifaceName string
	unit      string
	source    string
}

// delegatedInstall is the address one entry put on its unit's device.
type delegatedInstall struct {
	device string
	advert DelegatedAdvert
}

// delegatedState is what the worker installed and what it already reported,
// owned by the worker goroutine alone.
type delegatedState struct {
	installed map[delegatedUnitKey]delegatedInstall
	// refused holds the delegation each too-small entry was last reported
	// for, so the error is logged once per delegation, not on every pass.
	refused map[delegatedUnitKey]netip.Prefix
}

func newDelegatedState() *delegatedState {
	return &delegatedState{
		installed: make(map[delegatedUnitKey]delegatedInstall),
		refused:   make(map[delegatedUnitKey]netip.Prefix),
	}
}

// runDelegationWorker numbers each unit from its delegated prefix. Create the
// queue, start this worker once, push events from the bus, then close stop
// and wait on done. After each pass it publishes each entry's rows for
// DelegatedPrefixCommand and calls refreshRA, so a unit's advertisement
// follows its address. The loop runs until stop closes, and the rows are
// withdrawn then, so no answer outlives the worker that produced it.
func runDelegationWorker(q *delegationQueue, stop <-chan struct{}, done chan<- struct{}, activeCfg *atomic.Pointer[ifaceConfig], refreshRA func(), log *slog.Logger) {
	defer close(done)
	defer globalDelegatedUnits.Store(nil)
	state := newDelegatedState()
	empty := []delegatedUnitRow{}
	globalDelegatedUnits.Store(&empty)
	for {
		select {
		case <-stop:
			return
		case <-q.notify:
		}
		recordDelegationEvents(q.take(), log)
		cfg := activeCfg.Load()
		now := time.Now()
		applyDelegatedAddresses(cfg, state, now, log)
		rows := delegatedUnitRows(cfg, state, now)
		globalDelegatedUnits.Store(&rows)
		refreshRA()
	}
}

// recordDelegationEvents stores each pending lease event in the table.
func recordDelegationEvents(pending map[delegationKey]DHCPPayload, log *slog.Logger) {
	for key := range pending {
		p := pending[key]
		if p.Received == 0 {
			// Without the Reply's time nothing derived can be bounded by what
			// remains (RFC 9915 Section 6.3), so the event is refused.
			log.Warn("interface: DHCPv6 delegated prefix event carries no receive time, ignored", "source", key.source, "prefix", key.prefix)
			continue
		}
		heldDelegations.record(key.source, key.prefix, time.Unix(p.Received, 0), p.LeaseTime, p.PreferredLifetime)
	}
}

// applyDelegatedAddresses makes each unit hold the address of its /64 from
// every live delegation its config names, and nothing else from one.
func applyDelegatedAddresses(cfg *ifaceConfig, state *delegatedState, now time.Time, log *slog.Logger) {
	desired := make(map[delegatedUnitKey]DelegatedAdvert)
	forEachEnabledUnit(cfg, func(ifaceName string, u *unitEntry) {
		adverts, refusals := claimDelegatedAdverts(u, now)
		for _, a := range adverts {
			key := delegatedUnitKey{ifaceName: ifaceName, unit: u.Label, source: a.Source}
			desired[key] = a
			delete(state.refused, key)
		}
		for _, r := range refusals {
			key := delegatedUnitKey{ifaceName: ifaceName, unit: u.Label, source: r.source}
			reportDelegationTooSmall(key, r, state, log)
		}
	})

	for key := range state.installed {
		inst := state.installed[key]
		want, ok := desired[key]
		if ok && want.Address == inst.advert.Address {
			continue
		}
		removeDelegatedAddress(key, inst, log)
		delete(state.installed, key)
	}

	for key, advert := range desired {
		inst, ok := state.installed[key]
		if ok && inst.advert.Equal(advert) {
			continue
		}
		installed, ok := installDelegatedAddress(cfg, key, advert, now, log)
		if !ok {
			continue
		}
		state.installed[key] = installed
	}
}

// reportDelegationTooSmall logs, once per delegation, an entry whose subnet id
// the delegation the server chose cannot hold. RFC 9915 lets a server ignore
// the length hint, so this is a run-time outcome config verify cannot refuse.
func reportDelegationTooSmall(key delegatedUnitKey, r delegatedRefusal, state *delegatedState, log *slog.Logger) {
	if state.refused[key] == r.delegated {
		return
	}
	state.refused[key] = r.delegated
	log.Error("interface: the delegated prefix is too small for this unit's subnet-id, so the unit gets no /64 from it",
		"iface", key.ifaceName, "unit", key.unit, "source", key.source, "subnet-id", r.subnetID,
		"delegated", r.delegated, "error", r.err,
		"fix", "set interface "+key.ifaceName+" unit "+key.unit+" ipv6 delegated-prefix "+key.source+
			" subnet-id to an id the delegated prefix holds, or ask for a shorter prefix with interface pppoe-client "+
			key.source+" ipv6 dhcpv6 pd length <48-64>")
}

// delegatedOwner is the address-owner name of one entry, unique per unit and
// source so two units never replace each other's registration.
func delegatedOwner(key delegatedUnitKey) string {
	return "delegated-prefix " + key.source + " " + key.ifaceName + " unit " + key.unit
}

// installDelegatedAddress puts the entry's address on its unit's device with
// the lifetimes that remain of the delegation.
//
// RFC 9915 Section 6.3: "If the client uses a delegated prefix to configure
// addresses on interfaces on itself or other nodes behind it, the preferred
// and valid lifetimes of those addresses MUST be no longer than the remaining
// preferred and valid lifetimes, respectively, for the delegated prefix at any
// time." The kernel counts the installed lifetimes down from the install, so
// a value bounded at install stays bounded at any later time.
func installDelegatedAddress(cfg *ifaceConfig, key delegatedUnitKey, advert DelegatedAdvert, now time.Time, log *slog.Logger) (delegatedInstall, bool) {
	device, err := delegatedUnitDevice(cfg, key)
	if err != nil {
		log.Warn("interface: no device for a unit numbered from a delegated prefix, address not installed",
			"iface", key.ifaceName, "unit", key.unit, "source", key.source, "error", err)
		return delegatedInstall{}, false
	}
	// RFC 9915 Section 6.3
	valid, preferred := advert.Lifetimes(now)
	if valid == 0 {
		return delegatedInstall{}, false
	}
	cidr := advert.Address.String()
	// The owner registers BEFORE the install (RegisterLeasedAddresses), so
	// no address reconcile sees the address unregistered and removes it.
	if err := RegisterLeasedAddresses(device, delegatedOwner(key), []string{cidr}); err != nil {
		log.Warn("interface: delegated address not installed, another owner holds it",
			"device", device, "address", cidr, "error", err)
		return delegatedInstall{}, false
	}
	if err := ReplaceAddressWithLifetime(device, cidr, int(valid), int(preferred)); err != nil {
		log.Warn("interface: delegated address install failed", "device", device, "address", cidr, "error", err)
		if rerr := RegisterLeasedAddresses(device, delegatedOwner(key), nil); rerr != nil {
			log.Warn("interface: delegated address registration not cleared", "device", device, "error", rerr)
		}
		return delegatedInstall{}, false
	}
	log.Info("interface: unit numbered from a delegated prefix",
		"iface", key.ifaceName, "unit", key.unit, "address", cidr, "delegated", advert.Delegated,
		"valid", valid, "preferred", preferred)
	return delegatedInstall{device: device, advert: advert}, true
}

// removeDelegatedAddress takes an entry's address off its device: the lease
// ended, or the config no longer names it. The address goes BEFORE its
// registration, so no reconcile sees it unregistered.
func removeDelegatedAddress(key delegatedUnitKey, inst delegatedInstall, log *slog.Logger) {
	cidr := inst.advert.Address.String()
	if err := RemoveAddress(inst.device, cidr); err != nil {
		// The kernel removes an address whose valid lifetime ran out on its
		// own, so an expiry often finds it gone already.
		log.Warn("interface: delegated address removal failed", "device", inst.device, "address", cidr, "error", err)
	}
	if err := RegisterLeasedAddresses(inst.device, delegatedOwner(key), nil); err != nil {
		log.Warn("interface: delegated address registration not cleared", "device", inst.device, "error", err)
	}
	log.Info("interface: delegated address removed", "iface", key.ifaceName, "unit", key.unit, "address", cidr)
}

// delegatedUnitDevice returns the kernel device of the unit an entry names.
func delegatedUnitDevice(cfg *ifaceConfig, key delegatedUnitKey) (string, error) {
	osName, err := ResolveDevice(key.ifaceName)
	if err != nil {
		return "", err
	}
	var device string
	forEachEnabledUnit(cfg, func(ifaceName string, u *unitEntry) {
		if ifaceName == key.ifaceName && u.Label == key.unit {
			device = unitOSName(osName, u)
		}
	})
	if device == "" {
		return "", errUnitNotConfigured
	}
	return device, nil
}
