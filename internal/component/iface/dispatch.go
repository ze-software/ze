// Design: docs/features/interfaces.md -- Backend dispatch functions
// Overview: iface.go -- shared types and topic constants
// Related: backend.go -- Backend interface and registry

package iface

import (
	"errors"
	"net/netip"

	"github.com/ze-software/ze/internal/core/rtproto"
)

var errIfaceNoBackendLoaded = errors.New("iface: no backend loaded")

// backendOrErr returns the active backend or an error if none is loaded.
func backendOrErr() (Backend, error) {
	b := GetBackend()
	if b == nil {
		return nil, errIfaceNoBackendLoaded
	}
	return b, nil
}

// resolveOS translates a logical interface name to its kernel device name via
// the shared resolver, so the by-name dispatch ops below honor the os-name /
// mac-match selectors instead of assuming name == kernel device. It is
// best-effort: when no backend is loaded or the device is absent (resolution
// fails), it returns the name unchanged, so the backend call produces exactly
// the error it would have produced without translation -- behavior is identical
// to before whenever resolution is unavailable, and only redirects when a
// binding is known. The name "" (ResetCounters uses it to mean "every
// interface") never resolves and passes through untouched.
//
// GetInterface / ListInterfaces are deliberately NOT routed through here: the
// resolver is built on them (resolve.go osDeviceFor), so translating them would
// recurse. The Create* ops are also raw -- a created device's name IS its
// kernel name.
func resolveOS(name string) string {
	if name == "" {
		return ""
	}
	if b, err := Resolve(name); err == nil && b.OsName != "" {
		return b.OsName
	}
	return name
}

// Package-level functions that delegate to the active backend. By-name
// mutation/query ops translate the logical name to its kernel device via
// resolveOS first; Create* / GetInterface / ListInterfaces stay raw (see
// resolveOS).

func CreateDummy(name string) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	return b.CreateDummy(name)
}
func CreateVeth(name, peer string) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	return b.CreateVeth(name, peer)
}
func CreateBridge(name string) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	return b.CreateBridge(name)
}
func CreateVLAN(parent string, vid int) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	return b.CreateVLAN(VLANSpec{Parent: resolveOS(parent), VLANID: vid})
}
func DeleteInterface(name string) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	return b.DeleteInterface(resolveOS(name))
}
func AddAddress(iface, cidr string) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	return b.AddAddress(resolveOS(iface), cidr)
}
func RemoveAddress(iface, cidr string) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	return b.RemoveAddress(resolveOS(iface), cidr)
}

func AddRoute(ifaceName, destCIDR, gateway string, metric int, proto rtproto.Proto) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	return b.AddRoute(resolveOS(ifaceName), destCIDR, gateway, metric, proto)
}

func RemoveRoute(ifaceName, destCIDR, gateway string, metric int, proto rtproto.Proto) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	return b.RemoveRoute(resolveOS(ifaceName), destCIDR, gateway, metric, proto)
}

func ListRoutes(ifaceName, destCIDR string) ([]RouteInfo, error) {
	b, err := backendOrErr()
	if err != nil {
		return nil, err
	}
	return b.ListRoutes(resolveOS(ifaceName), destCIDR)
}

// ListNeighbors returns the kernel neighbor table via the active backend.
// family is one of NeighborFamilyAny / NeighborFamilyIPv4 / NeighborFamilyIPv6.
func ListNeighbors(family int) ([]NeighborInfo, error) {
	b, err := backendOrErr()
	if err != nil {
		return nil, err
	}
	return b.ListNeighbors(family)
}

// RouteLookup performs a longest-prefix-match lookup for the given
// destination IP via the active backend.
func RouteLookup(dest netip.Addr) (map[string]any, error) {
	b, err := backendOrErr()
	if err != nil {
		return nil, err
	}
	return b.RouteLookup(dest)
}

// AddressIsLocal reports whether dest is an address this box terminates (owned by
// one of its interfaces) rather than one it forwards, via the active backend. Used
// to classify a DDoS victim as local (control-plane, INPUT hook) vs remote (transit,
// FORWARD hook). Callers treat an error as "not local" (remote is the fail-safe).
func AddressIsLocal(dest netip.Addr) (bool, error) {
	b, err := backendOrErr()
	if err != nil {
		return false, err
	}
	return b.AddressIsLocal(dest)
}

// ListKernelRoutes returns up to `limit` entries from the kernel's
// routing table via the active backend. filterPrefix (non-empty)
// narrows the dump to a single CIDR. limit == 0 means unbounded.
func ListKernelRoutes(filterPrefix string, limit int) ([]KernelRoute, error) {
	b, err := backendOrErr()
	if err != nil {
		return nil, err
	}
	return b.ListKernelRoutes(filterPrefix, limit)
}

// ResetCounters zeros RX/TX counters for the named interface (or every
// managed interface when name == "") via the active backend. Backends
// that cannot physically clear counters in the kernel (Linux netlink)
// trigger a baseline-delta fallback: the current values become a
// per-interface baseline and GetStats/ListInterfaces/GetInterface
// subtract that baseline before returning. Wrap detection (raw < baseline)
// automatically rebases the baseline to zero so a subsequent kernel-level
// reset does not poison the delta view. See counters.go.
func ResetCounters(name string) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	return resetCountersViaBackend(b, resolveOS(name))
}

func ReplaceAddressWithLifetime(ifaceName, cidr string, validLft, preferredLft int) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	return b.ReplaceAddressWithLifetime(resolveOS(ifaceName), cidr, validLft, preferredLft)
}

func SetAdminUp(iface string) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	return b.SetAdminUp(resolveOS(iface))
}
func SetAdminDown(iface string) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	return b.SetAdminDown(resolveOS(iface))
}
func SetMTU(iface string, mtu int) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	return b.SetMTU(resolveOS(iface), mtu)
}
func SetMACAddress(iface, mac string) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	return b.SetMACAddress(resolveOS(iface), mac)
}

func GetMACAddress(iface string) (string, error) {
	b, err := backendOrErr()
	if err != nil {
		return "", err
	}
	return b.GetMACAddress(resolveOS(iface))
}

func GetStats(iface string) (*InterfaceStats, error) {
	b, err := backendOrErr()
	if err != nil {
		return nil, err
	}
	// Resolve once and key the baseline on the kernel device name, so a clear
	// (ResetCounters) and a subsequent read agree on the key regardless of the
	// selector (both resolve the logical name to the same os device).
	osName := resolveOS(iface)
	s, err := b.GetStats(osName)
	if err != nil {
		return nil, err
	}
	baselines.applyBaseline(osName, s)
	return s, nil
}

// LinkSpeedDuplex returns the link speed (Mbit/s) and duplex for the named
// interface, or (0, "") when unknown or no backend is loaded. Best-effort
// enrichment for the flow-export sFlow if_counters; never errors so a missing
// backend simply leaves ifSpeed/ifDirection unset.
func LinkSpeedDuplex(name string) (int, string) {
	b := GetBackend()
	if b == nil {
		return 0, ""
	}
	return b.LinkSpeedDuplex(resolveOS(name))
}

func ListInterfaces() ([]InterfaceInfo, error) {
	b, err := backendOrErr()
	if err != nil {
		return nil, err
	}
	ifs, err := b.ListInterfaces()
	if err != nil {
		return nil, err
	}
	for i := range ifs {
		baselines.applyBaseline(ifs[i].Name, ifs[i].Stats)
	}
	return ifs, nil
}

func GetInterface(name string) (*InterfaceInfo, error) {
	b, err := backendOrErr()
	if err != nil {
		return nil, err
	}
	info, err := b.GetInterface(name)
	if err != nil {
		return nil, err
	}
	if info != nil {
		baselines.applyBaseline(info.Name, info.Stats)
	}
	return info, nil
}

func BridgeAddPort(bridge, port string) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	return b.BridgeAddPort(resolveOS(bridge), resolveOS(port))
}
func BridgeDelPort(port string) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	return b.BridgeDelPort(resolveOS(port))
}
func BridgeSetSTP(bridge string, on bool) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	return b.BridgeSetSTP(resolveOS(bridge), on)
}

func SetupMirror(src, dst string, ingress, egress bool) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	return b.SetupMirror(resolveOS(src), resolveOS(dst), ingress, egress)
}

func RemoveMirror(src string) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	return b.RemoveMirror(resolveOS(src))
}

func GetXFRMInfo(name string) (XFRMInfo, error) {
	b, err := backendOrErr()
	if err != nil {
		return XFRMInfo{}, err
	}
	return b.GetXFRMInfo(resolveOS(name))
}

// ListRates returns the current rate data for all interfaces.
// Returns nil if the rate tracker is not running.
func ListRates() map[string]InterfaceRate {
	t := globalTracker.Load()
	if t == nil {
		return nil
	}
	return t.snapshot()
}

// GetRate returns the current rate data for a single interface.
// Returns false if the rate tracker is not running or the name is unknown.
func GetRate(name string) (InterfaceRate, bool) {
	t := globalTracker.Load()
	if t == nil {
		return InterfaceRate{}, false
	}
	return t.get(name)
}
