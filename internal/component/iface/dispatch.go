// Design: docs/features/interfaces.md -- Backend dispatch functions
// Overview: iface.go -- shared types and topic constants
// Related: backend.go -- Backend interface and registry

package iface

import (
	"errors"
	"fmt"
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
// mac-match selectors instead of assuming name == kernel device. The name ""
// (ResetCounters uses it to mean "every interface") never resolves and passes
// through untouched.
//
// A failed resolution is answered two ways, and which one it gets is the whole
// point of this function. A name with NO selector configured IS its own kernel
// device, so the name passes through and the backend produces exactly the error
// it would have produced without translation. A name WITH a selector was bound
// to other hardware, so a failure is refused: falling back to the logical name
// there is how an address, an MTU or an admin-down reaches whatever else happens
// to carry that name, which is the same wrong-port landing the config-apply path
// refuses through bindDevices.
//
// GetInterface / ListInterfaces are deliberately NOT routed through here: the
// resolver is built on them (resolve.go osDeviceFor), so translating them would
// recurse. The Create* ops are also raw -- a created device's name IS its
// kernel name.
func resolveOS(name string) (string, error) {
	if name == "" {
		return "", nil
	}
	b, err := Resolve(name)
	if err == nil && b.OsName != "" {
		return b.OsName, nil
	}
	if globalResolver.hasSelector(name) {
		if err == nil {
			err = errIfaceSelectorUnresolved
		}
		return "", fmt.Errorf("iface: interface %q is bound to hardware by a selector that resolves to no device: %w", name, err)
	}
	return name, nil
}

// errIfaceSelectorUnresolved names the one case Resolve reports as success and
// resolveOS still refuses: a binding whose device carries no name.
var errIfaceSelectorUnresolved = errors.New("resolved device has no name")

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
	osParent, err := resolveOS(parent)
	if err != nil {
		return err
	}
	return b.CreateVLAN(VLANSpec{Parent: osParent, VLANID: vid})
}
func DeleteInterface(name string) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	osName, err := resolveOS(name)
	if err != nil {
		return err
	}
	return b.DeleteInterface(osName)
}
func AddAddress(iface, cidr string) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	osName, err := resolveOS(iface)
	if err != nil {
		return err
	}
	return b.AddAddress(osName, cidr)
}
func RemoveAddress(iface, cidr string) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	osName, err := resolveOS(iface)
	if err != nil {
		return err
	}
	return b.RemoveAddress(osName, cidr)
}

func AddRoute(ifaceName, destCIDR, gateway string, metric int, proto rtproto.Proto) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	osName, err := resolveOS(ifaceName)
	if err != nil {
		return err
	}
	return b.AddRoute(osName, destCIDR, gateway, metric, proto)
}

func RemoveRoute(ifaceName, destCIDR, gateway string, metric int, proto rtproto.Proto) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	osName, err := resolveOS(ifaceName)
	if err != nil {
		return err
	}
	return b.RemoveRoute(osName, destCIDR, gateway, metric, proto)
}

func ListRoutes(ifaceName, destCIDR string) ([]RouteInfo, error) {
	b, err := backendOrErr()
	if err != nil {
		return nil, err
	}
	osName, err := resolveOS(ifaceName)
	if err != nil {
		return nil, err
	}
	return b.ListRoutes(osName, destCIDR)
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
	osName, err := resolveOS(name)
	if err != nil {
		return err
	}
	return resetCountersViaBackend(b, osName)
}

func ReplaceAddressWithLifetime(ifaceName, cidr string, validLft, preferredLft int) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	osName, err := resolveOS(ifaceName)
	if err != nil {
		return err
	}
	return b.ReplaceAddressWithLifetime(osName, cidr, validLft, preferredLft)
}

func SetAdminUp(iface string) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	osName, err := resolveOS(iface)
	if err != nil {
		return err
	}
	return b.SetAdminUp(osName)
}
func SetAdminDown(iface string) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	osName, err := resolveOS(iface)
	if err != nil {
		return err
	}
	return b.SetAdminDown(osName)
}
func SetMTU(iface string, mtu int) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	osName, err := resolveOS(iface)
	if err != nil {
		return err
	}
	return b.SetMTU(osName, mtu)
}
func SetMACAddress(iface, mac string) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	osName, err := resolveOS(iface)
	if err != nil {
		return err
	}
	return b.SetMACAddress(osName, mac)
}

func GetMACAddress(iface string) (string, error) {
	b, err := backendOrErr()
	if err != nil {
		return "", err
	}
	osName, err := resolveOS(iface)
	if err != nil {
		return "", err
	}
	return b.GetMACAddress(osName)
}

func GetStats(iface string) (*InterfaceStats, error) {
	b, err := backendOrErr()
	if err != nil {
		return nil, err
	}
	// Resolve once and key the baseline on the kernel device name, so a clear
	// (ResetCounters) and a subsequent read agree on the key regardless of the
	// selector (both resolve the logical name to the same os device).
	osName, err := resolveOS(iface)
	if err != nil {
		return nil, err
	}
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
	osName, err := resolveOS(name)
	if err != nil {
		return 0, ""
	}
	return b.LinkSpeedDuplex(osName)
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
	osBridge, err := resolveOS(bridge)
	if err != nil {
		return err
	}
	osPort, err := resolveOS(port)
	if err != nil {
		return err
	}
	return b.BridgeAddPort(osBridge, osPort)
}
func BridgeDelPort(port string) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	osPort, err := resolveOS(port)
	if err != nil {
		return err
	}
	return b.BridgeDelPort(osPort)
}
func BridgeSetSTP(bridge string, on bool) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	osBridge, err := resolveOS(bridge)
	if err != nil {
		return err
	}
	return b.BridgeSetSTP(osBridge, on)
}

func SetupMirror(src, dst string, ingress, egress bool) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	osSrc, err := resolveOS(src)
	if err != nil {
		return err
	}
	osDst, err := resolveOS(dst)
	if err != nil {
		return err
	}
	return b.SetupMirror(osSrc, osDst, ingress, egress)
}

func RemoveMirror(src string) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	osSrc, err := resolveOS(src)
	if err != nil {
		return err
	}
	return b.RemoveMirror(osSrc)
}

func GetXFRMInfo(name string) (XFRMInfo, error) {
	b, err := backendOrErr()
	if err != nil {
		return XFRMInfo{}, err
	}
	osName, err := resolveOS(name)
	if err != nil {
		return XFRMInfo{}, err
	}
	return b.GetXFRMInfo(osName)
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
