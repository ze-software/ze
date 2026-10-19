// Design: docs/features/interfaces.md -- Unnumbered IPv4 units
// Related: unnumbered.go -- the donor address is resolved at verify
// Related: register.go -- runEngine starts the worker and feeds it events
//
// An unnumbered unit's /32 is a copy of the donor's address, so it is only
// true while the donor holds that address on a device that is up. The apply
// path installs the /32 from config; this file withdraws it when the donor
// goes down or loses the address, and re-installs it when the donor recovers.
// A protocol that runs on the unit (OSPF) sees the address go and takes the
// link out of service.

package iface

import (
	"errors"
	"net/netip"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/events"
	ifaceevents "github.com/ze-software/ze/internal/core/iface/events"
	"github.com/ze-software/ze/pkg/ze"
)

// errUnnumberedAddressesUnread refuses a donor judgement on a backend that did
// not read device addresses: an empty list would read as "the donor lost it".
var errUnnumberedAddressesUnread = errors.New("unnumbered: the backend did not report device addresses, so the donor state is unknown; the borrowed /32 is left as it is")

// reconcileUnnumbered installs each unnumbered unit's /32 while its donor
// address sits on an up device other than the borrowers, and removes it
// otherwise. It is level-triggered: every call compares the kernel state with
// that rule, so a missed or coalesced event costs nothing.
func reconcileUnnumbered(cfg *ifaceConfig, b Backend) []error {
	borrowers := cfg.unnumberedDevices()
	if len(borrowers) == 0 {
		return nil
	}
	infos, err := b.ListInterfaces()
	if err != nil {
		return []error{err}
	}
	var errs []error
	for device, addr := range borrowers {
		info := findInterface(infos, device)
		if info == nil {
			continue // The borrowing device does not exist yet; apply creates it.
		}
		if !info.AddressesRead {
			errs = append(errs, errUnnumberedAddressesUnread)
			continue
		}
		cidr := netip.PrefixFrom(addr, 32).String()
		live := donorLive(infos, addr, borrowers)
		held := holdsAddress(info, addr, 32)
		if live && !held {
			if err := b.AddAddress(device, cidr); err != nil {
				errs = append(errs, err)
			}
			continue
		}
		if !live && held {
			if err := b.RemoveAddress(device, cidr); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errs
}

// donorLive reports whether addr sits on an up device that is not one of the
// borrowers, whose /32 copies prove nothing about the donor.
func donorLive(infos []InterfaceInfo, addr netip.Addr, borrowers map[string]netip.Addr) bool {
	for i := range infos {
		info := &infos[i]
		if _, borrower := borrowers[info.Name]; borrower {
			continue
		}
		if info.State != LinkStateUp {
			continue
		}
		if holdsAddress(info, addr, -1) {
			return true
		}
	}
	return false
}

// holdsAddress reports whether info carries addr. A prefixLength of -1 accepts
// any length.
func holdsAddress(info *InterfaceInfo, addr netip.Addr, prefixLength int) bool {
	for _, a := range info.Addresses {
		if prefixLength >= 0 && a.PrefixLength != prefixLength {
			continue
		}
		parsed, err := netip.ParseAddr(a.Address)
		if err == nil && parsed == addr {
			return true
		}
	}
	return false
}

func findInterface(infos []InterfaceInfo, name string) *InterfaceInfo {
	for i := range infos {
		if infos[i].Name == name {
			return &infos[i]
		}
	}
	return nil
}

// runUnnumberedWorker reconciles the unnumbered /32s each time ch signals,
// until stop closes; it closes done on return. One long-lived worker: the
// event handlers only signal ch, so no netlink call runs on the event bus.
func runUnnumberedWorker(ch, stop <-chan struct{}, done chan<- struct{}, activeCfg *atomic.Pointer[ifaceConfig]) {
	defer close(done)
	for {
		select {
		case <-ch:
			reconcileUnnumberedActive(activeCfg)
		case <-stop:
			return
		}
	}
}

func reconcileUnnumberedActive(activeCfg *atomic.Pointer[ifaceConfig]) {
	cfg := activeCfg.Load()
	if cfg == nil {
		return
	}
	b := GetBackend()
	if b == nil {
		return
	}
	for _, err := range reconcileUnnumbered(cfg, b) {
		loggerPtr.Load().Warn("unnumbered: borrowed /32 not reconciled with its donor", "err", err)
	}
}

// subscribeUnnumberedEvents signals ch on every link and address event: any of
// them can change whether a donor holds its address on an up device.
func subscribeUnnumberedEvents(eb ze.EventBus, ch chan<- struct{}) []func() {
	signal := events.AsString(func(string) { nonBlockingNotify(ch) })
	return []func(){
		eb.Subscribe(ifaceevents.Namespace, ifaceevents.EventUp, signal),
		eb.Subscribe(ifaceevents.Namespace, ifaceevents.EventDown, signal),
		eb.Subscribe(ifaceevents.Namespace, ifaceevents.EventAddrAdded, signal),
		eb.Subscribe(ifaceevents.Namespace, ifaceevents.EventAddrRemoved, signal),
	}
}
