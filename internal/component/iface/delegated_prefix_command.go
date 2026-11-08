// Design: docs/guide/pppoe.md -- numbering a LAN unit from a delegated prefix
// Related: delegated_consumer.go -- the worker that publishes the rows answered here
// Related: delegated_lease.go -- the delegation table and the deadlines each row reads
// Related: rate_command.go -- commandDecls and executeCommand, which route here
// Related: cmd/show_delegated_prefix.go -- the daemon handler that forwards here
//
// The delegation worker runs in the iface plugin's own process, like the PPPoE
// clients whose leases it reads, so the plugin answers what each LAN unit took
// from a delegated prefix as a command of its own, and the daemon handler
// forwards to it through the dispatcher.
//
// RFC: rfc/short/rfc9915.md

package iface

import (
	"errors"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// DelegatedPrefixCommand is the plugin command the iface plugin answers with
// one row per delegated-prefix entry of every enabled unit. The name is the
// YANG path of `show interface delegated-prefix`.
const DelegatedPrefixCommand = "show interface delegated-prefix"

// errDelegationWorkerNotRunning refuses a call that reaches a process where
// runEngine never started the delegation worker.
var errDelegationWorkerNotRunning = errors.New("delegated prefix status unavailable: the interface plugin has not started its delegated-prefix worker")

// delegatedUnitState is where one delegated-prefix entry of a unit stands.
// The zero value is not a state, so a row nobody classified cannot pass for
// one.
type delegatedUnitState uint8

const (
	delegatedUnitUnspecified delegatedUnitState = iota
	// delegatedUnitInstalled: the unit holds its address in its /64.
	delegatedUnitInstalled
	// delegatedUnitWaiting: the source holds no live delegated prefix.
	delegatedUnitWaiting
	// delegatedUnitTooSmall: the delegation cannot hold the subnet id.
	delegatedUnitTooSmall
	// delegatedUnitNotInstalled: a delegation is held and its address is not
	// on the unit, because the install failed.
	delegatedUnitNotInstalled
)

// String returns the state as the `state` key spells it.
func (s delegatedUnitState) String() string {
	switch s {
	case delegatedUnitInstalled:
		return "installed"
	case delegatedUnitWaiting:
		return "waiting"
	case delegatedUnitTooSmall:
		return "too-small"
	case delegatedUnitNotInstalled:
		return "not-installed"
	case delegatedUnitUnspecified:
		panic("BUG: delegated-prefix row published with no state")
	}
	panic("BUG: unknown delegatedUnitState " + strconv.Itoa(int(s)))
}

// delegatedUnitRow is one delegated-prefix entry as the worker last left it.
// The worker publishes rows whole and never changes one after publishing.
type delegatedUnitRow struct {
	ifaceName string
	unit      string
	source    string
	subnetID  uint16
	state     delegatedUnitState
	// device and advert are set for an installed row only.
	device string
	advert DelegatedAdvert
	// delegated is the source's chosen delegation for every state but
	// waiting, where the source holds none.
	delegated netip.Prefix
}

// globalDelegatedUnits is the rows the delegation worker last published, nil
// while no worker runs in this process.
var globalDelegatedUnits atomic.Pointer[[]delegatedUnitRow]

// DelegatedUnitStatus is one delegated-prefix entry of a unit, the answer of
// DelegatedPrefixCommand. Address, prefix and the lifetimes are present only
// when the unit holds its address; the lifetimes are the whole seconds that
// remain at the moment of the answer, and 4294967295 is infinite (RFC 9915
// Section 7.7).
type DelegatedUnitStatus struct {
	Interface string `json:"interface"`
	Unit      string `json:"unit"`
	Source    string `json:"source"`
	SubnetID  uint16 `json:"subnet-id"`
	State     string `json:"state"`
	Device    string `json:"device,omitempty"`
	// Address is the unit's address with its /64 length.
	Address netip.Prefix `json:"address,omitzero"`
	// Prefix is the /64 the unit derived and advertises.
	Prefix netip.Prefix `json:"prefix,omitzero"`
	// Delegated is the whole delegated prefix the /64 comes from.
	Delegated         netip.Prefix `json:"delegated,omitzero"`
	ValidLifetime     *uint32      `json:"valid-lifetime,omitempty"`
	PreferredLifetime *uint32      `json:"preferred-lifetime,omitempty"`
	// Reason says why a unit holds no address, and what to do about it.
	Reason string `json:"reason,omitempty"`
}

// delegatedPrefixAnswer is the answer of DelegatedPrefixCommand over rows at
// now. It is never nil while a worker runs, so a config with no
// delegated-prefix entry answers an empty list.
func delegatedPrefixAnswer(rows *[]delegatedUnitRow, now time.Time) ([]DelegatedUnitStatus, error) {
	if rows == nil {
		return nil, errDelegationWorkerNotRunning
	}
	out := make([]DelegatedUnitStatus, 0, len(*rows))
	for i := range *rows {
		out = append(out, delegatedUnitStatusOf(&(*rows)[i], now))
	}
	return out, nil
}

// delegatedUnitStatusOf turns one published row into its answer at now.
func delegatedUnitStatusOf(r *delegatedUnitRow, now time.Time) DelegatedUnitStatus {
	s := DelegatedUnitStatus{
		Interface: r.ifaceName,
		Unit:      r.unit,
		Source:    r.source,
		SubnetID:  r.subnetID,
		State:     r.state.String(),
		Delegated: r.delegated,
	}
	switch r.state {
	case delegatedUnitInstalled:
		// RFC 9915 Section 6.3
		valid, preferred := r.advert.Lifetimes(now)
		s.Device = r.device
		s.Address = r.advert.Address
		s.Prefix = r.advert.Address.Masked()
		s.ValidLifetime = &valid
		s.PreferredLifetime = &preferred
	case delegatedUnitWaiting:
		s.Reason = "pppoe-client " + r.source + " holds no delegated prefix: show interface pppoe-client lists what its DHCPv6 lease holds"
	case delegatedUnitTooSmall:
		s.Reason = "the delegated prefix cannot hold subnet-id " + strconv.Itoa(int(r.subnetID)) +
			": set interface " + r.ifaceName + " unit " + r.unit + " ipv6 delegated-prefix " + r.source +
			" subnet-id to an id the delegated prefix holds, or ask for a shorter prefix with interface pppoe-client " +
			r.source + " ipv6 dhcpv6 pd length <48-64>"
	case delegatedUnitNotInstalled:
		s.Reason = "the address install on the unit failed: the interface log names the cause"
	case delegatedUnitUnspecified:
		panic("BUG: delegated-prefix row published with no state")
	}
	return s
}

// delegatedUnitRows classifies every delegated-prefix entry of every enabled
// unit at now, from what the worker installed and what it refused, sorted by
// interface, unit and source. It runs on the worker goroutine, the only
// reader of state.
func delegatedUnitRows(cfg *ifaceConfig, state *delegatedState, now time.Time) []delegatedUnitRow {
	rows := []delegatedUnitRow{}
	forEachEnabledUnit(cfg, func(ifaceName string, u *unitEntry) {
		if u.IPv6 == nil {
			return
		}
		for _, dp := range u.IPv6.DelegatedPrefixes {
			key := delegatedUnitKey{ifaceName: ifaceName, unit: u.Label, source: dp.Source}
			rows = append(rows, delegatedUnitRowOf(key, dp.SubnetID, state, now))
		}
	})
	slices.SortFunc(rows, func(a, b delegatedUnitRow) int {
		if c := strings.Compare(a.ifaceName, b.ifaceName); c != 0 {
			return c
		}
		if c := strings.Compare(a.unit, b.unit); c != 0 {
			return c
		}
		return strings.Compare(a.source, b.source)
	})
	return rows
}

// delegatedUnitRowOf classifies one entry. An installed address wins; without
// one, the source's live delegation decides between waiting, too small for
// the subnet id the worker refused, and a failed install.
func delegatedUnitRowOf(key delegatedUnitKey, subnetID uint16, state *delegatedState, now time.Time) delegatedUnitRow {
	row := delegatedUnitRow{ifaceName: key.ifaceName, unit: key.unit, source: key.source, subnetID: subnetID}
	if inst, ok := state.installed[key]; ok {
		row.state = delegatedUnitInstalled
		row.device = inst.device
		row.advert = inst.advert
		row.delegated = inst.advert.Delegated
		return row
	}
	d, ok := heldDelegations.chosen(key.source, now)
	if !ok {
		row.state = delegatedUnitWaiting
		return row
	}
	row.delegated = d.prefix
	if refused, ok := state.refused[key]; ok && refused == d.prefix {
		row.state = delegatedUnitTooSmall
		return row
	}
	row.state = delegatedUnitNotInstalled
	return row
}
