// Design: docs/features/interfaces.md -- plugin-owned devices (bridges and links)
// Related: device_owner.go -- the device name grant and the macvlan registry beside these
// Related: config_apply.go -- reconcileOwnedDevices applies every owned kind
// Related: owned_namespace_linux.go -- the namespace a link's peer end is created in
// Related: cmd/owned_devices.go -- the engine operations that call this file
//
// A plugin that granted itself device names (MustGrantOwnedDeviceNames) asks
// for bridges and links through engine operations, so the owner is the
// authenticated calling process and the request works internal and forked.
// The bridges are a complete set per owner, reconciled like the owned
// macvlans. A link is made on request, once: it dies with the namespace that
// holds its peer end, so a reconcile never re-makes one, and it forgets a link
// record whose host end is gone.

package iface

import (
	"errors"
	"fmt"
	"maps"
	"net"
	"net/netip"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// Bounds on what one owner may ask for. A request over a bound is refused
// before any state changes.
const (
	ownedBridgesMax   = 64   // bridges in one owner's complete set
	ownedAddressesMax = 16   // addresses on one bridge or one peer end
	ownedLinksMax     = 1024 // links one owner holds at once
	ownedLinkKeyMax   = 64   // bytes of a link record key
	ownedMTUMin       = 68   // the IPv4 minimum (RFC 791)
	ownedMTUMax       = 16000
)

// OwnedBridgeSpec is one bridge an owner holds. Addresses are the bridge's
// complete address set, apart from the IPv6 link-local the kernel derives.
// Alias is the ownership mark "ze:owned:<owner>", set by the reconcile pass,
// never by the requester.
type OwnedBridgeSpec struct {
	Name      string
	MTU       int
	Addresses []netip.Prefix
	Alias     string
}

// OwnedLinkSpec is one link the backend creates: a veth pair whose host end
// HostEnd is marked with Alias, carries MTU, is a member of Bridge and is up,
// and whose peer end is created inside Namespace under PeerName, then
// addressed, given one default route per gateway, and set up. PeerMAC nil lets
// the kernel choose. IPv6 addresses are installed with duplicate address
// detection off. The backend never closes Namespace.
type OwnedLinkSpec struct {
	HostEnd       string
	Bridge        string
	MTU           int
	Alias         string
	Namespace     *os.File
	PeerName      string
	PeerMAC       net.HardwareAddr
	PeerAddresses []netip.Prefix
	PeerGateways  []netip.Addr
}

// ownedLink is one recorded link: its host end and the bridge it joins.
type ownedLink struct {
	hostEnd string
	bridge  string
}

// ownedBridgeOwners maps owner -> bridge name -> desired spec (Alias empty).
// ownedLinkOwners maps owner -> record key -> links. Both are guarded by
// deviceOwnerMu, and both change only under reconcileMu, so a reconcile pass
// reads them consistently and never sees a link between its create and its
// record.
var (
	ownedBridgeOwners = map[string]map[string]OwnedBridgeSpec{}
	ownedLinkOwners   = map[string]map[string][]ownedLink{}
)

// ReplaceOwnedBridges makes input the complete set of bridges owner holds and
// reconciles the kernel to it before it answers. A bridge left out is deleted,
// and so is every link of owner that joined it. A refused request changes
// nothing: the previous set is restored and the kernel steps of the pass are
// undone. owner is the authenticated name of the calling process. Safe for
// concurrent use.
func ReplaceOwnedBridges(owner string, input *rpc.OwnedBridgesReplaceInput) (*rpc.OwnedBridgesReplaceOutput, error) {
	if err := RefuseUngrantedDeviceOwner(owner); err != nil {
		return nil, err
	}
	specs, err := ownedBridgeSpecs(owner, input)
	if err != nil {
		return nil, err
	}

	reconcileMu.Lock()
	defer reconcileMu.Unlock()
	b := GetBackend()
	if b == nil {
		return nil, ownedBackendAbsent(owner)
	}

	deviceOwnerMu.Lock()
	previousBridges := ownedBridgeOwners[owner]
	previousLinks := ownedLinkOwners[owner]
	setOwnedBridgesLocked(owner, specs)
	deviceOwnerMu.Unlock()

	journal := sdk.NewJournal()
	if err := reconcileOwnedLocked(b, journal); err != nil {
		failures := append([]error{err}, journal.Rollback()...)
		deviceOwnerMu.Lock()
		restoreOwnedLocked(owner, previousBridges, previousLinks)
		deviceOwnerMu.Unlock()
		updateOwnedDeviceGauge()
		// The journal undoes what the pass created and changed. A device the
		// orphan scan deleted has no undo, so the previous set is reconciled
		// once more, which makes it again.
		if reassertErr := reconcileOwnedLocked(b, nil); reassertErr != nil {
			raiseOwnedReconcileFailure(owner, reassertErr)
			failures = append(failures, reassertErr)
		}
		return nil, ownedApplyRefusal(owner, "bridges", errors.Join(failures...))
	}
	journal.Discard()
	updateOwnedDeviceGauge()
	return &rpc.OwnedBridgesReplaceOutput{Bridges: slices.Sorted(maps.Keys(specs))}, nil
}

// setOwnedBridgesLocked records specs as owner's bridges and drops every link
// record of owner whose bridge left the set. The caller MUST hold
// deviceOwnerMu and reconcileMu.
func setOwnedBridgesLocked(owner string, specs map[string]OwnedBridgeSpec) {
	if len(specs) == 0 {
		delete(ownedBridgeOwners, owner)
	} else {
		ownedBridgeOwners[owner] = specs
	}
	kept := map[string][]ownedLink{}
	for key, links := range ownedLinkOwners[owner] {
		var still []ownedLink
		for _, link := range links {
			if _, ok := specs[link.bridge]; ok {
				still = append(still, link)
			}
		}
		if len(still) > 0 {
			kept[key] = still
		}
	}
	if len(kept) == 0 {
		delete(ownedLinkOwners, owner)
		return
	}
	ownedLinkOwners[owner] = kept
}

// restoreOwnedLocked puts back owner's bridge and link records as they were.
// The caller MUST hold deviceOwnerMu and reconcileMu.
func restoreOwnedLocked(owner string, bridges map[string]OwnedBridgeSpec, links map[string][]ownedLink) {
	if bridges == nil {
		delete(ownedBridgeOwners, owner)
	} else {
		ownedBridgeOwners[owner] = bridges
	}
	if links == nil {
		delete(ownedLinkOwners, owner)
		return
	}
	ownedLinkOwners[owner] = links
}

// ownedBridgeSpecs validates the requested set and returns it by name. Each
// name lies inside a prefix owner granted itself, so it never collides with a
// configured interface (refuseGrantedDeviceName) or another owner's device.
func ownedBridgeSpecs(owner string, input *rpc.OwnedBridgesReplaceInput) (map[string]OwnedBridgeSpec, error) {
	if input == nil {
		return nil, ownedRequestRefusal(owner, "the request carries no bridge set")
	}
	if len(input.Bridges) > ownedBridgesMax {
		return nil, ownedRequestRefusal(owner, "the set holds "+strconv.Itoa(len(input.Bridges))+" bridges, more than "+strconv.Itoa(ownedBridgesMax))
	}
	specs := make(map[string]OwnedBridgeSpec, len(input.Bridges))
	for i := range input.Bridges {
		bridge := &input.Bridges[i]
		if err := refuseOwnedDeviceName(owner, bridge.Name); err != nil {
			return nil, err
		}
		if _, dup := specs[bridge.Name]; dup {
			return nil, ownedRequestRefusal(owner, "bridge "+bridge.Name+" appears twice in the set")
		}
		mtu, err := ownedMTU(owner, bridge.Name, bridge.MTU)
		if err != nil {
			return nil, err
		}
		if err := refuseOwnedAddresses(owner, bridge.Name, bridge.Addresses); err != nil {
			return nil, err
		}
		specs[bridge.Name] = OwnedBridgeSpec{Name: bridge.Name, MTU: mtu, Addresses: slices.Clone(bridge.Addresses)}
	}
	return specs, nil
}

// CreateOwnedLink creates one link for owner: the host end, named from owner's
// first granted prefix, the bridge's interface index and the slot, joins an
// owned bridge of owner; the peer end is created inside the network namespace
// of input.PID, which MUST carry the inode number input.NamespaceInode. The
// namespace is opened once, and that one descriptor is both checked and used,
// so a reused process id cannot redirect the link. The link is recorded under
// input.Key and the host end is answered. A refused request records nothing.
// Safe for concurrent use.
func CreateOwnedLink(owner string, input *rpc.OwnedLinkCreateInput) (*rpc.OwnedLinkCreateOutput, error) {
	if err := RefuseUngrantedDeviceOwner(owner); err != nil {
		return nil, err
	}
	peerMAC, err := refuseOwnedLinkInput(owner, input)
	if err != nil {
		return nil, err
	}
	// The namespace is checked before any other state is read, so a request
	// that names the wrong namespace is refused whatever else holds.
	namespace, err := openOwnedLinkNamespace(input.PID, input.NamespaceInode)
	if err != nil {
		return nil, err
	}
	defer namespace.Close() //nolint:errcheck // a read-only namespace descriptor; the link is already made or refused

	reconcileMu.Lock()
	defer reconcileMu.Unlock()
	b := GetBackend()
	if b == nil {
		return nil, ownedBackendAbsent(owner)
	}
	if err := refuseLinkRecordLimit(owner, input.Bridge); err != nil {
		return nil, err
	}
	bridge, err := b.GetLink(input.Bridge)
	if err != nil {
		return nil, ownedRequestRefusal(owner, "bridge "+input.Bridge+" is not in the kernel: "+err.Error())
	}
	alias := ownedDeviceAliasPrefix + owner
	if bridge.Alias != alias {
		return nil, ownedRequestRefusal(owner, "bridge "+input.Bridge+" does not carry the mark "+alias)
	}
	hostEnd, err := ComposeOwnedDeviceName(ownedDevicePrefixes(owner)[0], bridge.Index, int(input.Slot))
	if err != nil {
		return nil, ownedRequestRefusal(owner, err.Error())
	}
	if holder := ownedLinkHolder(hostEnd); holder != "" {
		return nil, ownedRequestRefusal(owner, "host end "+hostEnd+" is already recorded under key "+holder)
	}

	spec := OwnedLinkSpec{
		HostEnd:       hostEnd,
		Bridge:        input.Bridge,
		MTU:           int(input.MTU),
		Alias:         alias,
		Namespace:     namespace,
		PeerName:      input.Name,
		PeerMAC:       peerMAC,
		PeerAddresses: slices.Clone(input.Addresses),
		PeerGateways:  slices.Clone(input.Gateways),
	}
	if err := b.CreateOwnedLink(spec); err != nil {
		return nil, ownedApplyRefusal(owner, "link "+hostEnd, err)
	}

	deviceOwnerMu.Lock()
	if ownedLinkOwners[owner] == nil {
		ownedLinkOwners[owner] = map[string][]ownedLink{}
	}
	ownedLinkOwners[owner][input.Key] = append(ownedLinkOwners[owner][input.Key], ownedLink{hostEnd: hostEnd, bridge: input.Bridge})
	deviceOwnerMu.Unlock()
	updateOwnedDeviceGauge()
	return &rpc.OwnedLinkCreateOutput{HostEnd: hostEnd}, nil
}

// refuseOwnedLinkInput validates a link request before any state is read, and
// answers the peer end's hardware address, nil when the request names none.
func refuseOwnedLinkInput(owner string, input *rpc.OwnedLinkCreateInput) (net.HardwareAddr, error) {
	if input == nil {
		return nil, ownedRequestRefusal(owner, "the request carries no link")
	}
	if err := refuseOwnedLinkKey(owner, input.Key); err != nil {
		return nil, err
	}
	if err := refuseOwnedDeviceName(owner, input.Bridge); err != nil {
		return nil, err
	}
	if input.PID <= 0 {
		return nil, ownedRequestRefusal(owner, "process id "+strconv.Itoa(input.PID)+" names no process")
	}
	if input.NamespaceInode == 0 {
		return nil, ownedRequestRefusal(owner, "the request names no namespace inode number")
	}
	if err := ValidateIfaceName(input.Name); err != nil {
		return nil, ownedRequestRefusal(owner, "peer end name: "+err.Error())
	}
	if _, err := ownedMTU(owner, input.Name, input.MTU); err != nil {
		return nil, err
	}
	if err := refuseOwnedAddresses(owner, input.Name, input.Addresses); err != nil {
		return nil, err
	}
	if err := refuseOwnedGateways(owner, input.Addresses, input.Gateways); err != nil {
		return nil, err
	}
	if input.HardwareAddress == "" {
		return nil, nil
	}
	mac, err := parseUnicastMAC(input.HardwareAddress)
	if err != nil {
		return nil, ownedRequestRefusal(owner, "peer end hardware address: "+err.Error())
	}
	return mac, nil
}

// refuseLinkRecordLimit refuses a link on a bridge owner does not hold, and a
// link over the per-owner bound. The caller MUST hold reconcileMu.
func refuseLinkRecordLimit(owner, bridge string) error {
	deviceOwnerMu.Lock()
	defer deviceOwnerMu.Unlock()
	if _, held := ownedBridgeOwners[owner][bridge]; !held {
		return ownedRequestRefusal(owner, "bridge "+bridge+" is not in the plugin's bridge set; declare it with "+rpc.MethodOwnedBridgesReplace+" first")
	}
	count := 0
	for _, links := range ownedLinkOwners[owner] {
		count += len(links)
	}
	if count >= ownedLinksMax {
		return ownedRequestRefusal(owner, "the plugin already holds "+strconv.Itoa(count)+" links, the most one owner may hold")
	}
	return nil
}

// ownedLinkHolder answers the record key of the link whose host end is
// hostEnd, of any owner, and "" when no record names it.
func ownedLinkHolder(hostEnd string) string {
	deviceOwnerMu.Lock()
	defer deviceOwnerMu.Unlock()
	for _, keys := range ownedLinkOwners {
		for key, links := range keys {
			for _, link := range links {
				if link.hostEnd == hostEnd {
					return key
				}
			}
		}
	}
	return ""
}

// RemoveOwnedLink deletes every link owner recorded under input.Key and
// forgets the record. A host end that is already gone is forgotten without a
// call, because the kernel removes a pair with its namespace. It answers the
// host ends whose record it dropped. A host end the kernel refused to delete
// keeps its record and fails the request. Safe for concurrent use.
func RemoveOwnedLink(owner string, input *rpc.OwnedLinkRemoveInput) (*rpc.OwnedLinkRemoveOutput, error) {
	if err := RefuseUngrantedDeviceOwner(owner); err != nil {
		return nil, err
	}
	if input == nil {
		return nil, ownedRequestRefusal(owner, "the request carries no key")
	}
	if err := refuseOwnedLinkKey(owner, input.Key); err != nil {
		return nil, err
	}

	reconcileMu.Lock()
	defer reconcileMu.Unlock()
	b := GetBackend()
	if b == nil {
		return nil, ownedBackendAbsent(owner)
	}
	deviceOwnerMu.Lock()
	links := slices.Clone(ownedLinkOwners[owner][input.Key])
	deviceOwnerMu.Unlock()

	removed := []string{}
	var kept []ownedLink
	var failures []error
	for _, link := range links {
		if err := deleteOwnedHostEnd(b, link.hostEnd); err != nil {
			kept = append(kept, link)
			failures = append(failures, err)
			continue
		}
		removed = append(removed, link.hostEnd)
	}

	deviceOwnerMu.Lock()
	if len(kept) == 0 {
		delete(ownedLinkOwners[owner], input.Key)
		if len(ownedLinkOwners[owner]) == 0 {
			delete(ownedLinkOwners, owner)
		}
	} else {
		ownedLinkOwners[owner][input.Key] = kept
	}
	deviceOwnerMu.Unlock()
	updateOwnedDeviceGauge()

	if len(failures) > 0 {
		return nil, ownedApplyRefusal(owner, "links under key "+input.Key, errors.Join(failures...))
	}
	return &rpc.OwnedLinkRemoveOutput{Removed: removed}, nil
}

// deleteOwnedHostEnd deletes hostEnd unless the kernel has no such device. An
// answer that is not absence is a failure, never read as "already gone".
func deleteOwnedHostEnd(b Backend, hostEnd string) error {
	if _, err := b.GetLink(hostEnd); err != nil {
		if errors.Is(err, ErrInterfaceNotFound) {
			return nil
		}
		return err
	}
	return b.DeleteInterface(hostEnd)
}

// WithdrawOwnedDevices forgets every bridge and link owner holds and
// reconciles, which deletes their marked devices. It is the cleanup for a
// plugin process that is gone. Owned macvlans are not touched: they are
// registered in process, not through the engine operations. A failed deletion
// is raised for the iface-owned-devices doctor check (doctor_owned.go) before
// the error is returned. Safe for concurrent use.
func WithdrawOwnedDevices(owner string) error {
	reconcileMu.Lock()
	defer reconcileMu.Unlock()
	deviceOwnerMu.Lock()
	_, hadBridges := ownedBridgeOwners[owner]
	_, hadLinks := ownedLinkOwners[owner]
	delete(ownedBridgeOwners, owner)
	delete(ownedLinkOwners, owner)
	deviceOwnerMu.Unlock()
	if !hadBridges && !hadLinks {
		return nil
	}
	updateOwnedDeviceGauge()
	b := GetBackend()
	if b == nil {
		return nil
	}
	if err := reconcileOwnedLocked(b, nil); err != nil {
		raiseOwnedCleanupFailure(owner, err)
		return err
	}
	return nil
}

// reconcileOwnedLocked runs the owned-device pass alone, against a fresh
// listing, so a request answers with the kernel's outcome. The caller MUST
// hold reconcileMu.
func reconcileOwnedLocked(b Backend, journal *sdk.Journal) error {
	currentInfos, err := b.ListInterfaces()
	if err != nil {
		return fmt.Errorf("list interfaces for the owned devices: %w", err)
	}
	var errs []error
	record := func(msg string, err error) {
		loggerPtr.Load().Warn(msg, "err", err, "code", CodeDoctorIfaceOwnedDeviceReconcile)
		errs = append(errs, fmt.Errorf("%s: %w", msg, err))
	}
	reconcileOwnedDevices(b, journal, currentInfos, record)
	return errors.Join(errs...)
}

// ownedBridgeSnapshot answers every owner's bridges by name, with the
// ownership alias set. Device names are unique across owners because each
// lies inside its owner's granted prefix. The map is a copy.
func ownedBridgeSnapshot() map[string]OwnedBridgeSpec {
	deviceOwnerMu.Lock()
	defer deviceOwnerMu.Unlock()
	result := map[string]OwnedBridgeSpec{}
	for owner, bridges := range ownedBridgeOwners {
		for name, spec := range bridges {
			spec.Alias = ownedDeviceAliasPrefix + owner
			result[name] = spec
		}
	}
	return result
}

// forgetGoneOwnedLinks drops every link record whose host end is not in the
// listing, because the kernel removed the pair with its namespace, and
// answers the host ends still recorded. The caller MUST hold reconcileMu.
func forgetGoneOwnedLinks(currentByName map[string]InterfaceInfo) map[string]bool {
	deviceOwnerMu.Lock()
	defer deviceOwnerMu.Unlock()
	live := map[string]bool{}
	for owner, keys := range ownedLinkOwners {
		for key, links := range keys {
			var still []ownedLink
			for _, link := range links {
				if _, present := currentByName[link.hostEnd]; present {
					still = append(still, link)
					live[link.hostEnd] = true
				}
			}
			if len(still) == 0 {
				delete(keys, key)
				continue
			}
			keys[key] = still
		}
		if len(keys) == 0 {
			delete(ownedLinkOwners, owner)
		}
	}
	return live
}

// ownedKindRegistered answers whether a device of an owned kind that carries
// the ownership mark is still wanted, and whether its kind is one the owned
// registries create at all. A marked device of any other kind is never
// deleted: the orphan scan needs both facts, the kind and the mark.
func ownedKindRegistered(info *InterfaceInfo, macvlans map[string]MacvlanSpec, bridges map[string]OwnedBridgeSpec, hostEnds map[string]bool) (registered, ownedKind bool) {
	switch info.Type {
	case zeTypeMacvlan:
		_, registered = macvlans[info.Name]
		return registered, true
	case TypeBridge:
		_, registered = bridges[info.Name]
		return registered, true
	case TypeVeth:
		return hostEnds[info.Name], true
	}
	return false, false
}

// reconcileOwnedBridges creates, re-asserts and addresses every registered
// bridge. A name held by a device of another kind, or by a bridge without this
// owner's mark, fails closed: the device is never touched. A marked bridge is
// adopted after a restart: its MTU and addresses are re-asserted in place,
// because deleting it would drop the links that joined it. It returns false
// on the first failure, which record has logged.
func reconcileOwnedBridges(b Backend, journal *sdk.Journal, currentByName map[string]InterfaceInfo, record func(string, error)) bool {
	bridges := ownedBridgeSnapshot()
	for _, name := range slices.Sorted(maps.Keys(bridges)) {
		desired := bridges[name]
		cur, exists := currentByName[name]
		if !exists {
			if err := applyBackendStep(journal, func() error {
				return b.CreateOwnedBridge(desired)
			}, func() error {
				return b.DeleteInterface(name)
			}); err != nil {
				record("create owned bridge", fmt.Errorf("%s: %w", name, err))
				return false
			}
		} else if !reassertOwnedBridge(b, journal, cur, desired, record) {
			return false
		}
		if !reconcileOwnedBridgeAddresses(b, journal, cur, desired, record) {
			return false
		}
	}
	return true
}

// reassertOwnedBridge checks that the device holding desired.Name is this
// owner's bridge and puts its MTU back when it drifted.
func reassertOwnedBridge(b Backend, journal *sdk.Journal, cur InterfaceInfo, desired OwnedBridgeSpec, record func(string, error)) bool {
	name := desired.Name
	if cur.Type != TypeBridge {
		record("owned bridge name conflict", fmt.Errorf("%s: name occupied by a non-owned %s device", name, cur.Type))
		return false
	}
	if cur.Alias != desired.Alias {
		record("owned bridge name conflict", fmt.Errorf("%s: name occupied by a bridge without the mark %s", name, desired.Alias))
		return false
	}
	if cur.MTU == desired.MTU {
		return true
	}
	oldMTU := cur.MTU
	if err := applyBackendStep(journal, func() error {
		return b.SetMTU(name, desired.MTU)
	}, func() error {
		return b.SetMTU(name, oldMTU)
	}); err != nil {
		record("re-assert owned bridge mtu", fmt.Errorf("%s: %w", name, err))
		return false
	}
	return true
}

// reconcileOwnedBridgeAddresses makes the bridge's addresses the spec's. The
// kernel's IPv6 link-local is left in place: no owner asks for it and the
// kernel puts it back. cur is the listing entry, empty for a bridge this pass
// created.
func reconcileOwnedBridgeAddresses(b Backend, journal *sdk.Journal, cur InterfaceInfo, desired OwnedBridgeSpec, record func(string, error)) bool {
	name := desired.Name
	current := map[string]bool{}
	for _, held := range cur.Addresses {
		current[held.Address+"/"+strconv.Itoa(held.PrefixLength)] = true
	}
	wanted := map[string]bool{}
	for _, prefix := range desired.Addresses {
		cidr := prefix.String()
		wanted[cidr] = true
		if current[cidr] {
			continue
		}
		if err := applyBackendStep(journal, func() error {
			return b.AddAddress(name, cidr)
		}, func() error {
			return b.RemoveAddress(name, cidr)
		}); err != nil {
			record("add owned bridge address", fmt.Errorf("%s %s: %w", name, cidr, err))
			return false
		}
	}
	for _, cidr := range slices.Sorted(maps.Keys(current)) {
		if wanted[cidr] {
			continue
		}
		if prefix, err := netip.ParsePrefix(cidr); err == nil && prefix.Addr().Is6() && prefix.Addr().IsLinkLocalUnicast() {
			continue
		}
		if err := applyBackendStep(journal, func() error {
			return b.RemoveAddress(name, cidr)
		}, func() error {
			return b.AddAddress(name, cidr)
		}); err != nil {
			record("remove owned bridge address", fmt.Errorf("%s %s: %w", name, cidr, err))
			return false
		}
	}
	return true
}

// deleteOwnedDevicesOnStop deletes every device that carries the ownership
// mark and is of an owned kind, and forgets the link records, whose devices
// are then gone. The bridge and macvlan registrations stay: they are their
// owners' declarations, and an engine that starts again re-asserts them. It
// is the ordered stop's actor, because plugins stop together with no order
// and an owner's own release would race the backend close. Every device is
// attempted, and the failures are returned together. The caller MUST hold
// reconcileMu.
func deleteOwnedDevicesOnStop(b Backend) error {
	infos, err := b.ListInterfaces()
	if err != nil {
		return fmt.Errorf("list interfaces for the owned devices: %w", err)
	}
	var errs []error
	for i := range infos {
		info := &infos[i]
		if !strings.HasPrefix(info.Alias, ownedDeviceAliasPrefix) {
			continue
		}
		if _, ownedKind := ownedKindRegistered(info, nil, nil, nil); !ownedKind {
			continue
		}
		if err := b.DeleteInterface(info.Name); err != nil {
			errs = append(errs, fmt.Errorf("delete owned %s %s: %w", info.Type, info.Name, err))
		}
	}
	deviceOwnerMu.Lock()
	ownedLinkOwners = map[string]map[string][]ownedLink{}
	deviceOwnerMu.Unlock()
	updateOwnedDeviceGauge()
	return errors.Join(errs...)
}

// closeBackendAfterOwnedDevices is the end of the interface engine's ordered
// stop: it deletes the owned devices, then closes the backend, both under
// reconcileMu, so no owned-device request lands between the two.
func closeBackendAfterOwnedDevices() (ownedErr, closeErr error) {
	reconcileMu.Lock()
	defer reconcileMu.Unlock()
	if b := GetBackend(); b != nil {
		ownedErr = deleteOwnedDevicesOnStop(b)
	}
	return ownedErr, CloseBackend()
}

// refuseOwnedDeviceName refuses a device name that is not a valid interface
// name or that lies outside every prefix owner granted itself.
func refuseOwnedDeviceName(owner, name string) error {
	if err := ValidateIfaceName(name); err != nil {
		return ownedRequestRefusal(owner, err.Error())
	}
	for _, prefix := range ownedDevicePrefixes(owner) {
		if strings.HasPrefix(name, prefix) && len(name) > len(prefix) {
			return nil
		}
	}
	return ownedRequestRefusal(owner, "device "+name+" lies outside the device name prefixes "+strings.Join(ownedDevicePrefixes(owner), ", ")+" the plugin granted itself")
}

// refuseOwnedLinkKey refuses an empty or over-long record key.
func refuseOwnedLinkKey(owner, key string) error {
	if key == "" {
		return ownedRequestRefusal(owner, "the link record key is empty")
	}
	if len(key) > ownedLinkKeyMax {
		return ownedRequestRefusal(owner, "the link record key is longer than "+strconv.Itoa(ownedLinkKeyMax)+" bytes")
	}
	return nil
}

// ownedMTU answers mtu as an int, or refuses it outside the bounds every
// owned kind accepts. A uint32 always fits the int of the 64-bit targets ze
// builds for.
func ownedMTU(owner, device string, mtu uint32) (int, error) {
	if mtu < ownedMTUMin {
		return 0, ownedRequestRefusal(owner, device+" mtu "+strconv.Itoa(int(mtu))+" is below "+strconv.Itoa(ownedMTUMin))
	}
	if mtu > ownedMTUMax {
		return 0, ownedRequestRefusal(owner, device+" mtu "+strconv.Itoa(int(mtu))+" is above "+strconv.Itoa(ownedMTUMax))
	}
	return int(mtu), nil
}

// refuseOwnedAddresses refuses more addresses than the bound, an invalid or
// unspecified address, and one address twice.
func refuseOwnedAddresses(owner, device string, addresses []netip.Prefix) error {
	if len(addresses) > ownedAddressesMax {
		return ownedRequestRefusal(owner, device+" carries "+strconv.Itoa(len(addresses))+" addresses, more than "+strconv.Itoa(ownedAddressesMax))
	}
	seen := make(map[netip.Prefix]bool, len(addresses))
	for _, prefix := range addresses {
		if !prefix.IsValid() {
			return ownedRequestRefusal(owner, device+" carries an invalid address")
		}
		if prefix.Addr().IsUnspecified() {
			return ownedRequestRefusal(owner, device+" carries the unspecified address "+prefix.String())
		}
		if seen[prefix] {
			return ownedRequestRefusal(owner, device+" carries "+prefix.String()+" twice")
		}
		seen[prefix] = true
	}
	return nil
}

// refuseOwnedGateways refuses a gateway that no address of its family on the
// peer end reaches, and two gateways of one family: each becomes the one
// default route of its family.
func refuseOwnedGateways(owner string, addresses []netip.Prefix, gateways []netip.Addr) error {
	families := map[bool]bool{}
	for _, gateway := range gateways {
		if !gateway.IsValid() {
			return ownedRequestRefusal(owner, "a gateway is not a valid address")
		}
		if families[gateway.Is4()] {
			return ownedRequestRefusal(owner, "two gateways of one family; a link has one default route per family")
		}
		families[gateway.Is4()] = true
		if !slices.ContainsFunc(addresses, func(prefix netip.Prefix) bool { return prefix.Masked().Contains(gateway) }) {
			return ownedRequestRefusal(owner, "gateway "+gateway.String()+" is inside no address of the peer end, so no route can reach it")
		}
	}
	return nil
}

// ownedRequestRefusal is the operator error for a request the interface
// component refuses before it changes anything.
func ownedRequestRefusal(owner, detail string) error {
	return opererr.New(
		"Plugin "+owner+" asked for an owned interface device the interface component refuses: "+detail,
		"The request was refused before any device changed, so the plugin's devices are as they were",
		"The plugin built a request the interface component does not accept; run ze show log and report the plugin "+owner+" with this message",
	)
}

// ownedApplyRefusal is the operator error for a request the kernel refused.
func ownedApplyRefusal(owner, subject string, cause error) error {
	return opererr.Wrap(cause,
		"The kernel refused the owned "+subject+" of plugin "+owner,
		"The steps already made were undone, so the plugin's devices are as they were before the request",
		"Run ze show interface to see the devices, and ze doctor to find what the kernel lacks; set interface backend netlink when another backend is active",
	)
}

// ownedBackendAbsent is the operator error for a request that arrives before
// the interface backend is loaded or after it closed.
func ownedBackendAbsent(owner string) error {
	return opererr.New(
		"Plugin "+owner+" asked for an owned interface device while no interface backend is loaded",
		"The interface component loads its backend when it is configured and closes it when it stops, so nothing was changed",
		"Make the request after the interface component is configured: declare interface in the plugin's dependencies",
	)
}
