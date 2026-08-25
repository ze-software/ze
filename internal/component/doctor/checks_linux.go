// Design: docs/features/ai-first.md — Linux-specific readiness checks

//go:build linux

package doctor

import (
	"context"
	"errors"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/host"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/smart"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/zefs"
)

const (
	defaultVPPSocket     = "/run/vpp/api.sock"
	backendVPP           = "vpp"
	doctorModulesEnv     = "ze.test.doctor.modules-file"
	doctorProcRootEnv    = "ze.test.doctor.procfs-root"
	doctorNetlinkFailEnv = "ze.test.doctor.netlink-fail"
	doctorMachineIDEnv   = "ze.test.doctor.machine-id-path"
	doctorRandomSeedEnv  = "ze.test.doctor.random-seed-path"

	capSysTime       = 25
	dpdkSysfsDevDir  = "/sys/bus/pci/devices"
	sysClassBlockDir = "/sys/class/block"
	machineIDPath    = "/etc/machine-id"

	gokrazyRandomSeedPath = "/perm/random.seed"
	systemdRandomSeedPath = "/var/lib/systemd/random-seed"
)

var _ = env.MustRegister(env.EnvEntry{
	Key:         doctorModulesEnv,
	Type:        "string",
	Description: "Override /proc/modules path for doctor functional tests",
	Private:     true,
})

var _ = env.MustRegister(env.EnvEntry{
	Key:         doctorProcRootEnv,
	Type:        "string",
	Description: "Override /proc root path for doctor functional tests",
	Private:     true,
})

var _ = env.MustRegister(env.EnvEntry{
	Key:         doctorNetlinkFailEnv,
	Type:        "bool",
	Description: "Force route netlink doctor probe failure (test infrastructure)",
	Private:     true,
})

var _ = env.MustRegister(env.EnvEntry{
	Key:         doctorMachineIDEnv,
	Type:        "string",
	Description: "Override /etc/machine-id path for doctor functional tests",
	Private:     true,
})

var _ = env.MustRegister(env.EnvEntry{
	Key:         doctorRandomSeedEnv,
	Type:        "string",
	Description: "Override random-seed path for doctor functional tests",
	Private:     true,
})

type routeNetlinkHandle interface {
	Close()
}

var loadedKernelModules = readLoadedModules
var statPath = os.Stat
var readFilePath = os.ReadFile
var accessPath = unix.Access
var currentUID = os.Getuid
var newRouteNetlinkHandle = func() (routeNetlinkHandle, error) {
	return netlink.NewHandle(unix.NETLINK_ROUTE)
}

// xfrmNetlinkProbe reports why the kernel holds no XFRM dataplane, or nil when it
// holds one. A var so a unit test drives both answers without a kernel.
var xfrmNetlinkProbe = openXFRMNetlink

// openXFRMNetlink opens the XFRM netlink socket and closes it again.
//
// Opening the socket is the honest test of whether the kernel carries XFRM, and it
// is the one /proc/modules cannot make: a built-in CONFIG_XFRM_USER=y appears in no
// module list, which is the appliance case. The socket needs no CAP_NET_ADMIN, so
// an unprivileged ze doctor still gets the right answer, and on a modular kernel
// the open loads the module the same way any XFRM user would.
func openXFRMNetlink() error {
	h, err := netlink.NewHandle(unix.NETLINK_XFRM)
	if err != nil {
		return err
	}
	h.Close()
	return nil
}

func checkVPPSocket(sockPath string) []diagnostic.Diagnostic {
	if sockPath == "" {
		sockPath = defaultVPPSocket
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var d net.Dialer
	var tb textbuf.Buffer
	conn, err := d.DialContext(ctx, "unix", sockPath)
	if err != nil {
		return []diagnostic.Diagnostic{{
			Code:     "doctor-vpp-unreachable",
			Severity: diagnostic.SeverityError,
			Message:  tb.Str("VPP API socket unreachable: ").Str(sockPath).Str(": ").Err(err).String(),
			Path:     sockPath,
		}}
	}
	if closeErr := conn.Close(); closeErr != nil {
		return []diagnostic.Diagnostic{{
			Code:     "doctor-vpp-unreachable",
			Severity: diagnostic.SeverityWarning,
			Message:  tb.Reset().Str("VPP API socket close: ").Str(sockPath).Str(": ").Err(closeErr).String(),
			Path:     sockPath,
		}}
	}
	return nil
}

func checkKernelModules(tree *config.Tree) []diagnostic.Diagnostic {
	var required []string
	hasIPsec := false
	l2tpRequired := false
	pppoeRequired := false

	if tree != nil {
		ifaceBlock := tree.GetContainer("interface")
		if ifaceBlock != nil {
			backend, _ := ifaceBlock.Get("backend")
			if backend == backendVPP {
				required = append(required, "vhost_net")
			}
		}

		if l2tp := tree.GetContainer("l2tp"); configEnabled(l2tp, true) {
			l2tpRequired = true
		}

		if pppoe := tree.GetContainer("pppoe"); configEnabled(pppoe, true) {
			pppoeRequired = true
		}

		if getContainerPath(tree, "vpn", "ipsec") != nil {
			hasIPsec = true
		}
	}

	if len(required) == 0 && !l2tpRequired && !pppoeRequired && !hasIPsec {
		return nil
	}

	loaded := loadedKernelModules()
	var diags []diagnostic.Diagnostic
	var tb textbuf.Buffer
	for _, mod := range required {
		if !loaded[mod] {
			diags = append(diags, diagnostic.Diagnostic{
				Code:     "doctor-module-missing",
				Severity: diagnostic.SeverityError,
				Message:  tb.Reset().Str("kernel module not loaded: ").Str(mod).String(),
			})
		}
	}

	if l2tpRequired && !loaded["l2tp_ppp"] && !loaded["pppol2tp"] {
		diags = append(diags, diagnostic.Diagnostic{
			Code:     "doctor-l2tp-module",
			Severity: diagnostic.SeverityError,
			Message:  "L2TP kernel module not loaded: l2tp_ppp or pppol2tp",
		})
	}

	if pppoeRequired && !loaded["pppoe"] {
		diags = append(diags, diagnostic.Diagnostic{
			Code:     "doctor-pppoe-module",
			Severity: diagnostic.SeverityError,
			Message:  "PPPoE kernel module not loaded: pppoe",
		})
	}

	// XFRM is asked about through netlink, not through /proc/modules.
	//
	// readLoadedModules parses /proc/modules, which lists LOADED MODULES ONLY. An
	// appliance kernel builds XFRM in (CONFIG_XFRM_USER=y in
	// gokrazy/kernel/kernel.config), so xfrm_user and xfrm_algo appear nowhere in
	// that file and the module rows reported two missing modules for a dataplane
	// that was present and working. ze doctor exited 1 on a healthy appliance.
	//
	// The check is not weakened: a host whose XFRM netlink socket cannot be opened
	// holds no IPsec dataplane at all, and that is still an error. Only the
	// evidence changed, from how the kernel was PACKAGED to whether the capability
	// EXISTS.
	if hasIPsec {
		if err := xfrmNetlinkProbe(); err != nil {
			diags = append(diags, diagnostic.Diagnostic{
				Code:     "doctor-module-missing",
				Severity: diagnostic.SeverityError,
				Message: tb.Reset().Str("IPsec: the kernel holds no XFRM dataplane (xfrm_user, xfrm_algo): ").
					Err(err).String(),
			})
		}
	}

	if hasIPsec && !loaded["ip_tables"] && !loaded["nf_tables"] {
		diags = append(diags, diagnostic.Diagnostic{
			Code:     "doctor-module-missing",
			Severity: diagnostic.SeverityWarning,
			Message:  "IPsec: neither ip_tables nor nf_tables loaded (firewall marking may not work)",
		})
	}

	return diags
}

// sysClassNetDir is the directory the interface presence and state checks read.
// A variable so a test can point them at a fixture tree, matching
// loadedModulesPath below.
var sysClassNetDir = "/sys/class/net"

// checkInterfaces reports on every configured ethernet interface: absent,
// administratively down, or carrying a hardware selector that names no device or
// more than one.
//
// The entry NAME is not the device when the entry carries a selector. `os-name`
// aliases a kernel device and `mac/match` binds to the device carrying a
// hardware address, and the config apply path keys its work by whichever one
// answers (bindDevices, internal/component/iface/config_apply.go). A check that
// looked the entry name up in sysfs therefore called a perfectly good mac/match
// config a missing interface, and said nothing at all about a selector that
// resolves to nothing.
//
// The two selector verdicts differ in severity because the daemon treats them
// differently. A selector no device answers to is a DEFERRED binding, which the
// YANG promises and the apply accepts, so it is a warning. A selector several
// devices answer to is refused by the apply, so it is an error.
func checkInterfaces(tree *config.Tree) []diagnostic.Diagnostic {
	if tree == nil {
		return nil
	}
	ifaceBlock := tree.GetContainer("interface")
	if ifaceBlock == nil {
		return nil
	}

	backend, _ := ifaceBlock.Get("backend")
	if backend == backendVPP {
		return nil
	}

	ethList := ifaceBlock.GetList("ethernet")
	if len(ethList) == 0 {
		return nil
	}

	var diags []diagnostic.Diagnostic
	var tb textbuf.Buffer
	for name, entry := range ethList {
		device, diag := selectedNetDevice(name, entry)
		if diag != nil {
			diags = append(diags, *diag)
			continue
		}
		if strings.Contains(device, "..") || strings.ContainsAny(device, "/\x00") {
			continue
		}
		statePath := tb.Reset().Str(sysClassNetDir).Byte('/').Str(device).String()
		info, err := os.Stat(statePath)
		if err != nil || !info.IsDir() {
			diags = append(diags, diagnostic.Diagnostic{
				Code:     "doctor-iface-missing",
				Severity: diagnostic.SeverityError,
				Message:  tb.Reset().Str("ethernet interface not found: ").Str(device).String(),
			})
			continue
		}
		operstate, err := os.ReadFile(tb.Reset().Str(statePath).Str("/operstate").String()) //nolint:gosec // path traversal guarded above
		if err != nil {
			continue
		}
		state := strings.TrimSpace(string(operstate))
		if state != "up" && state != "unknown" {
			diags = append(diags, diagnostic.Diagnostic{
				Code:     "doctor-iface-down",
				Severity: diagnostic.SeverityWarning,
				Message:  tb.Reset().Str("ethernet interface ").Str(device).Str(" operstate: ").Str(state).String(),
			})
		}
	}
	return diags
}

// selectedNetDevice returns the kernel device an ethernet entry configures, or a
// diagnostic when its hardware selector names no device or more than one.
// mac/match wins over os-name, as the YANG leaf states, and an entry with
// neither selector IS its own kernel device.
func selectedNetDevice(name string, entry *config.Tree) (string, *diagnostic.Diagnostic) {
	if entry == nil {
		return name, nil
	}
	var tb textbuf.Buffer
	if mac := entry.GetContainer("mac"); mac != nil {
		if match, ok := mac.Get("match"); ok && match != "" {
			devices := netDevicesWithAddress(match)
			switch len(devices) {
			case 1:
				return devices[0], nil
			case 0:
				return "", &diagnostic.Diagnostic{
					Code:     "doctor-iface-selector-unmatched",
					Severity: diagnostic.SeverityWarning,
					Message: tb.Reset().Str("ethernet ").Str(name).Str(": no device carries MAC ").Str(match).
						Str("; the binding stays deferred until one appears").String(),
				}
			default:
				return "", &diagnostic.Diagnostic{
					Code:     "doctor-iface-selector-ambiguous",
					Severity: diagnostic.SeverityError,
					Message: tb.Reset().Str("ethernet ").Str(name).Str(": MAC ").Str(match).Str(" is carried by ").
						Str(strings.Join(devices, ", ")).Str("; a hardware MAC selects at most one device").String(),
				}
			}
		}
	}
	if osName, ok := entry.Get("os-name"); ok && osName != "" {
		return osName, nil
	}
	return name, nil
}

// netDevicesWithAddress returns the names of the kernel devices that carry mac
// as their OWN hardware address, read from sysfs and sorted for a reproducible
// message.
//
// A device standing on another one is not a candidate, and that exclusion is
// load-bearing rather than tidy. A vlan and a macvlan hang off a parent, and a
// bridge or a bond wears the address of a port it holds, so a second device
// reports the selector's address the moment ze builds anything on the selected
// port -- which ze does, on the operator's own config
// (test/plugin/iface-bridge-mac-match-apply.ci). The config apply path skips the
// same two kinds before it matches (devicesWithMAC,
// internal/component/iface/config_apply.go), so a doctor that counted them
// called that config ambiguous, at SeverityError, while the daemon bound to the
// right port.
//
// sysfs exposes the CURRENT address; the daemon's resolver prefers the permanent
// (factory) one and falls back to the current address only for the virtual kinds
// that report none (deviceMatchMAC, internal/component/iface/resolve.go). The two
// agree unless something outside ze overrode a NIC's operational address, and on
// such a box this check reports "no device carries MAC" where the daemon binds.
// That is why the unmatched verdict is a warning naming the address it compared,
// rather than an error.
func netDevicesWithAddress(mac string) []string {
	target := strings.ToLower(strings.TrimSpace(mac))
	entries, err := os.ReadDir(sysClassNetDir)
	if err != nil {
		return nil
	}
	var tb textbuf.Buffer
	var found []string
	for _, e := range entries {
		device := e.Name()
		if strings.Contains(device, "..") || strings.ContainsAny(device, "/\x00") {
			continue
		}
		deviceDir := tb.Reset().Str(sysClassNetDir).Byte('/').Str(device).String()
		addr, readErr := os.ReadFile(tb.Reset().Str(deviceDir).Str("/address").String()) //nolint:gosec // path traversal guarded above
		if readErr != nil {
			continue
		}
		if strings.ToLower(strings.TrimSpace(string(addr))) != target {
			continue
		}
		if hasLowerDevice(deviceDir) {
			continue
		}
		found = append(found, device)
	}
	sort.Strings(found)
	return found
}

// hasLowerDevice reports whether the kernel lists a device standing under the
// one in deviceDir, which is what makes the address that device reports somebody
// else's. sysfs writes the relation on the UPPER device, as a lower_<name>
// symlink, and it writes it for both kinds a hardware selector must skip: a
// stacked device gets one for its parent, and an aggregator gets one for each
// member it holds.
//
// Measured against a live kernel on 2026-08-19, in a network namespace holding
// one dummy port: a vlan and a macvlan on that port each carry lower_<port>, a
// bridge carries lower_<port> once the port is enslaved and carries nothing
// before that, and a veth carries none at all -- its peer is IFLA_LINK, which
// sysfs reports as iflink rather than as a link. The port itself carries master
// and upper_<bridge>, never a lower_ link, so it stays a candidate here exactly
// as it stays one in the apply path.
//
// A directory that cannot be read reports no lower device. The caller has just
// read this device's address file, so the only reader that arrives here on an
// unreadable directory is one racing a device that is being removed, and the
// device it would judge no longer exists.
func hasLowerDevice(deviceDir string) bool {
	entries, err := os.ReadDir(deviceDir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "lower_") {
			return true
		}
	}
	return false
}

// checkVPPVersion runs `vppctl show version` and warns if the major version
// is not in the expected range. Only runs when VPP backend is configured.
func checkVPPVersion(tree *config.Tree) []diagnostic.Diagnostic {
	if tree == nil {
		return nil
	}
	ifaceBlock := tree.GetContainer("interface")
	if ifaceBlock == nil {
		return nil
	}
	backend, _ := ifaceBlock.Get("backend")
	if backend != backendVPP {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "vppctl", "show", "version").Output() //nolint:gosec // fixed command
	var tb textbuf.Buffer
	if err != nil {
		return []diagnostic.Diagnostic{{
			Code:     "doctor-vpp-version",
			Severity: diagnostic.SeverityWarning,
			Message:  tb.Str("cannot determine VPP version: ").Err(err).String(),
		}}
	}

	version := strings.TrimSpace(string(out))
	if !strings.Contains(version, "vpp v") {
		return []diagnostic.Diagnostic{{
			Code:     "doctor-vpp-version",
			Severity: diagnostic.SeverityWarning,
			Message:  tb.Reset().Str("unexpected VPP version output: ").Str(version).String(),
		}}
	}
	return nil
}

// loadedModulesPath is the file the module list is read from: the test stub when
// ze.test.doctor.modules-file names one, else procfs. Its only reader is
// readLoadedModules, which feeds checkKernelModules. No diagnostic message names
// this path: checkMPLSSupport used to, and it now probes the AF_MPLS sysctl
// instead, because a module list cannot see a built-in capability.
func loadedModulesPath() string {
	if path := env.Get(doctorModulesEnv); path != "" {
		return path
	}
	return procPath("modules")
}

func readLoadedModules() map[string]bool {
	path := loadedModulesPath()
	data, err := readFilePath(path)
	if err != nil {
		return nil
	}
	set := make(map[string]bool)
	for line := range strings.SplitSeq(string(data), "\n") {
		if sp := strings.IndexByte(line, ' '); sp > 0 {
			set[line[:sp]] = true
		}
	}
	return set
}

func checkKernelNexthop() []diagnostic.Diagnostic {
	path := procPath("net", "nexthop")
	_, err := statPath(path)
	if err != nil {
		var tb textbuf.Buffer
		return []diagnostic.Diagnostic{{
			Code:     "doctor-kernel-nexthop",
			Severity: diagnostic.SeverityWarning,
			Message:  tb.Str("kernel nexthop objects unavailable (").Str(path).Str(" not found); ECMP uses legacy multipath").String(),
		}}
	}
	return nil
}

func checkMPLSSupport(tree *config.Tree) []diagnostic.Diagnostic {
	if tree == nil {
		return nil
	}
	// MPLS modules only matter for the kernel FIB backend.
	fibBlock := tree.GetContainer("fib")
	if fibBlock == nil || fibBlock.GetContainer("kernel") == nil {
		return nil
	}
	// Only warn when MPLS forwarding is actually configured (F15): a labeled BGP
	// family, LDP, RSVP-TE, or a per-interface MPLS enable. A plain BGP config
	// over the kernel FIB imposes no labels and needs no MPLS modules.
	if !mplsInUse(tree) {
		return nil
	}

	// Ask whether the CAPABILITY exists, never how the kernel was PACKAGED.
	//
	// readLoadedModules parses /proc/modules, which lists LOADED MODULES ONLY. A
	// kernel with CONFIG_MPLS_ROUTING=y has no mpls_router.ko to list, so the
	// module rows reported MPLS absent on the very kernel that forwards it.
	// ze's own runtime kernel became such a kernel when CONFIG_MPLS_ROUTING and
	// CONFIG_MPLS_IPTUNNEL were built in (gokrazy/kernel/runtime.config), so
	// every appliance with MPLS configured would have warned about a kernel that
	// supports it. Same defect, and the same fix, as the XFRM rows above.
	//
	// af_mpls creates net.mpls.platform_labels when MPLS routing is available,
	// built in or loaded as a module, so one probe answers for both packagings.
	//
	// The VALUE is read, never only the path. The sysctl is the size of the label
	// space and it defaults to 0, which disables MPLS entirely
	// (docs/guide/mpls.md). Building MPLS into ze's runtime kernel therefore
	// creates this file on every appliance, at 0, so a check that stopped at the
	// stat would go silent on exactly the machines the built-in kernel was meant
	// to serve: the table exists, the label space does not, and ze programs
	// labels the kernel refuses. Present-but-zero is its own answer with its own
	// remedy, so it gets its own code rather than being folded into either
	// neighbor.
	path := mplsPlatformLabelsPath()
	raw, err := readFilePath(path)
	if err == nil {
		if strings.TrimSpace(string(raw)) != "0" {
			return nil
		}
		var tb textbuf.Buffer
		return []diagnostic.Diagnostic{{
			Code:     "doctor-mpls-disabled",
			Severity: diagnostic.SeverityWarning,
			Message: tb.Str("MPLS forwarding disabled: ").Str(path).
				Str(" is 0, so the kernel holds no label space and labeled routes cannot be installed; set net.mpls.platform_labels in the sysctl {} block").String(),
		}}
	}
	// The probe could not be READ, which is not the same as "MPLS is absent". A
	// guard that cannot reach its evidence says so rather than reporting the
	// answer it did not get (ai/rules/evidence.md).
	if !errors.Is(err, fs.ErrNotExist) {
		var tb textbuf.Buffer
		return []diagnostic.Diagnostic{{
			Code:     "doctor-mpls-unknown",
			Severity: diagnostic.SeverityWarning,
			Message: tb.Str("cannot determine MPLS kernel support, the capability probe is unreadable: ").
				Err(err).String(),
		}}
	}

	var tb textbuf.Buffer
	return []diagnostic.Diagnostic{{
		Code:     "doctor-mpls-unavailable",
		Severity: diagnostic.SeverityWarning,
		Message: tb.Str("MPLS forwarding unavailable: ").Str(path).
			Str(" does not exist, so the kernel holds no AF_MPLS table and labeled routes cannot be installed").String(),
	}}
}

// mplsPlatformLabelsPath names the sysctl af_mpls creates when the kernel holds
// an AF_MPLS forwarding table. Shared with the doctor-mpls-unavailable message
// so it names the path the check actually tried; the doctor-mpls-unknown message
// gets that path from os.Stat's own *PathError.
func mplsPlatformLabelsPath() string {
	return procPath("sys", "net", "mpls", "platform_labels")
}

// mplsInUse reports whether the config actually uses MPLS forwarding: a BGP
// labeled-unicast/VPN family on a peer (directly or via a group), LDP, RSVP-TE,
// or a per-interface MPLS enable. checkMPLSSupport gates the MPLS-module warning
// on this so a plain BGP-over-kernel config does not warn about modules it does
// not need (F15).
func mplsInUse(tree *config.Tree) bool {
	if tree.GetContainer("ldp") != nil || tree.GetContainer("rsvp-te") != nil {
		return true
	}
	for _, i := range tree.GetListOrdered("interface") {
		if i.Value.GetContainer("mpls") != nil {
			return true
		}
	}
	bgp := tree.GetContainer("bgp")
	if bgp == nil {
		return false
	}
	if containerPeersLabeled(bgp) {
		return true
	}
	for _, g := range bgp.GetListOrdered("group") {
		// A group carries the full peer-fields grouping
		// (internal/component/bgp/yang/ze-bgp-conf.yang `list group { uses
		// peer-fields; }`), so the family may be declared ONCE on the group and
		// on none of its peers; ResolveBGPTree deep-merges it into every member
		// (internal/component/bgp/config/resolve.go). Checking only the group's
		// peers missed that shape entirely -- and it is the idiomatic one, used
		// by 26 configs in this repo (test/encode/group-inheritance.ci is the
		// canonical example).
		if sessionLabeled(g.Value) || containerPeersLabeled(g.Value) {
			return true
		}
	}
	return false
}

// sessionLabeled reports whether c's OWN session negotiates a labeled family.
// Used for a group, whose session is inherited by each member peer.
func sessionLabeled(c *config.Tree) bool {
	session := c.GetContainer("session")
	if session == nil {
		return false
	}
	for _, fam := range session.GetListOrdered("family") {
		if slices.Contains(labeledFamilies, fam.Key) {
			return true
		}
	}
	return false
}

// labeledFamilies are the family names that mean MPLS forwarding, and therefore
// that the kernel needs mpls_router / mpls_iptunnel. Package-level so the test
// that pins them against a parsed config reads the same list the check does.
var labeledFamilies = []string{"ipv4/mpls-label", "ipv6/mpls-label", "ipv4/mpls-vpn", "ipv6/mpls-vpn"}

// containerPeersLabeled reports whether any peer directly under c negotiates a
// labeled-unicast or MPLS-VPN family.
//
// `family` is a LIST keyed by the family name, not a container -- `family
// ipv4/mpls-label { ... }` parses to a list ENTRY whose key is the family. This
// read used session.GetContainer("family"), which is nil for a list, so the loop
// below was unreachable and checkMPLSSupport could never fire on a real config;
// it was the only reader in the tree doing so (web/page_bgp_peers.go,
// page_bgp_families.go, page_bgp_groups.go and exabgp/migration all use
// GetListOrdered). test/plugin/mpls-doctor.ci existed to catch this and could
// not: it is skip-os everywhere but Linux, so it had never run until CI, and it
// ALSO named a family (`ipv4/mpls-unicast`) that does not exist -- two
// independent faults, either of which alone produced the same silent pass.
func containerPeersLabeled(c *config.Tree) bool {
	for _, p := range c.GetListOrdered("peer") {
		if sessionLabeled(p.Value) {
			return true
		}
	}
	return false
}

func checkFirewallBackend(tree *config.Tree) []diagnostic.Diagnostic {
	if tree == nil {
		return nil
	}
	firewall := tree.GetContainer("firewall")
	if firewall == nil {
		return nil
	}
	backend, _ := firewall.Get("backend")
	if backend != "" && backend != "nft" {
		return nil
	}
	if loadedKernelModules()["nf_tables"] {
		return nil
	}
	return []diagnostic.Diagnostic{{
		Code:     "doctor-firewall-nftables",
		Severity: diagnostic.SeverityWarning,
		Message:  "firewall: nf_tables kernel module not loaded",
	}}
}

func checkTelemetryProcfs(tree *config.Tree) []diagnostic.Diagnostic {
	prom := getContainerPath(tree, "telemetry", "prometheus")
	if !configEnabled(prom, false) {
		return nil
	}
	path := procPath("stat")
	if _, err := readFilePath(path); err != nil {
		var tb textbuf.Buffer
		return []diagnostic.Diagnostic{{
			Code:     "doctor-telemetry-procfs",
			Severity: diagnostic.SeverityWarning,
			Message:  tb.Str("telemetry: cannot read ").Str(path).Str(": ").Err(err).String(),
			Path:     path,
		}}
	}
	return nil
}

func checkSysctlProcfs(tree *config.Tree) []diagnostic.Diagnostic {
	if tree == nil || tree.GetContainer("sysctl") == nil {
		return nil
	}
	path := procPath("sys")
	if err := accessPath(path, unix.W_OK); err != nil {
		var tb textbuf.Buffer
		return []diagnostic.Diagnostic{{
			Code:     "doctor-sysctl-procfs",
			Severity: diagnostic.SeverityWarning,
			Message:  tb.Str("sysctl: ").Str(path).Str(" is not writable: ").Err(err).String(),
			Path:     path,
		}}
	}
	return nil
}

func checkConntrackProcfs(tree *config.Tree) []diagnostic.Diagnostic {
	if getContainerPath(tree, "system", "conntrack") == nil {
		return nil
	}
	var tb textbuf.Buffer
	dir := procPath("sys", "net", "netfilter")
	if _, err := statPath(dir); err != nil {
		return []diagnostic.Diagnostic{{
			Code:     "doctor-conntrack-procfs",
			Severity: diagnostic.SeverityWarning,
			Message:  tb.Str("conntrack: ").Str(dir).Str(" unavailable: ").Err(err).String(),
			Path:     dir,
		}}
	}
	key := procPath("sys", "net", "netfilter", "nf_conntrack_max")
	if err := accessPath(key, unix.W_OK); err != nil {
		return []diagnostic.Diagnostic{{
			Code:     "doctor-conntrack-procfs",
			Severity: diagnostic.SeverityWarning,
			Message:  tb.Reset().Str("conntrack: ").Str(key).Str(" is not writable: ").Err(err).String(),
			Path:     key,
		}}
	}
	return nil
}

func checkPolicyRouteNetlink(tree *config.Tree) []diagnostic.Diagnostic {
	policy := tree.GetContainer("policy")
	if policy == nil || len(policy.GetList("route")) == 0 {
		return nil
	}
	if env.IsEnabled(doctorNetlinkFailEnv) {
		return []diagnostic.Diagnostic{{
			Code:     "doctor-policyroute-netlink",
			Severity: diagnostic.SeverityWarning,
			Message:  "policy route: route netlink unavailable: forced failure",
		}}
	}
	h, err := newRouteNetlinkHandle()
	if err != nil {
		var tb textbuf.Buffer
		return []diagnostic.Diagnostic{{
			Code:     "doctor-policyroute-netlink",
			Severity: diagnostic.SeverityWarning,
			Message:  tb.Str("policy route: route netlink unavailable: ").Err(err).String(),
		}}
	}
	if h != nil {
		h.Close()
	}
	return nil
}

func procPath(parts ...string) string {
	root := env.Get(doctorProcRootEnv)
	if root == "" {
		root = "/proc"
	}
	all := make([]string, 0, len(parts)+1)
	all = append(all, root)
	all = append(all, parts...)
	return filepath.Join(all...)
}

func checkNTPClockPrivilege(tree *config.Tree) []diagnostic.Diagnostic {
	ntp := getContainerPath(tree, "environment", "ntp")
	if !configEnabled(ntp, false) {
		return nil
	}

	if currentUID() == 0 {
		return nil
	}

	data, err := readFilePath(procPath("self", "status"))
	if err != nil {
		return nil
	}

	for line := range strings.SplitSeq(string(data), "\n") {
		hex, ok := strings.CutPrefix(line, "CapEff:\t")
		if !ok {
			continue
		}
		caps, parseErr := strconv.ParseUint(strings.TrimSpace(hex), 16, 64)
		if parseErr != nil {
			return nil
		}
		if caps&(1<<capSysTime) == 0 {
			return []diagnostic.Diagnostic{{
				Code:     "doctor-ntp-clock-privilege",
				Severity: diagnostic.SeverityWarning,
				Message:  "NTP: CAP_SYS_TIME not granted; clock adjustment will fail",
			}}
		}
		return nil
	}
	return nil
}

func checkMachineID(platform *host.PlatformInfo, store storage.Storage) []diagnostic.Diagnostic {
	if platform == nil || (platform.Type != host.PlatformGokrazy && platform.Type != host.PlatformSystemd) {
		return nil
	}

	path := doctorMachineIDPath()
	data, err := readFilePath(path)
	if err == nil && strings.TrimSpace(string(data)) != "" {
		return nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil
	}

	if store != nil {
		if zefsData, zefsErr := store.ReadFile(zefs.KeyMachineID.Pattern); zefsErr == nil {
			if strings.TrimSpace(string(zefsData)) != "" {
				return nil
			}
		}
	}

	var tb textbuf.Buffer
	return []diagnostic.Diagnostic{{
		Code:     "doctor-machine-id-missing",
		Severity: diagnostic.SeverityWarning,
		Message:  tb.Str("machine-id is missing or empty on ").Str(platform.Type.String()).String(),
		Path:     path,
		Expected: tb.Reset().Str("non-empty ").Str(path).Str(" or zefs meta/instance/machine-id").String(),
		Actual:   "missing or empty",
	}}
}

func doctorMachineIDPath() string {
	path := strings.TrimSpace(env.Get(doctorMachineIDEnv))
	if path != "" {
		return path
	}
	return machineIDPath
}

var dpdkVFIOModules = []string{"vfio", "vfio_pci", "vfio_iommu_type1"}

func checkVPPDPDK(tree *config.Tree) []diagnostic.Diagnostic {
	ifaceBlock := tree.GetContainer("interface")
	if ifaceBlock == nil {
		return nil
	}
	backend, _ := ifaceBlock.Get("backend")
	if backend != backendVPP {
		return nil
	}

	vppBlock := tree.GetContainer("vpp")
	if vppBlock == nil {
		return nil
	}
	dpdk := vppBlock.GetContainer("dpdk")
	if dpdk == nil {
		return nil
	}

	interfaces := dpdk.GetListOrdered("interface")
	if len(interfaces) == 0 {
		return nil
	}

	var diags []diagnostic.Diagnostic

	var tb textbuf.Buffer
	loaded := loadedKernelModules()
	for _, mod := range dpdkVFIOModules {
		if !loaded[mod] {
			diags = append(diags, diagnostic.Diagnostic{
				Code:     "doctor-vpp-dpdk",
				Severity: diagnostic.SeverityError,
				Message:  tb.Reset().Str("VPP DPDK: VFIO kernel module not loaded: ").Str(mod).String(),
			})
		}
	}

	for _, iface := range interfaces {
		pci := iface.Key
		sysfsPath := filepath.Join(dpdkSysfsDevDir, pci)
		if _, err := statPath(sysfsPath); err != nil {
			diags = append(diags, diagnostic.Diagnostic{
				Code:     "doctor-vpp-dpdk",
				Severity: diagnostic.SeverityError,
				Message:  tb.Reset().Str("VPP DPDK: PCI device not found: ").Str(pci).String(),
				Path:     sysfsPath,
			})
		}
	}

	return diags
}

func checkRandomSeed(platform *host.PlatformInfo) []diagnostic.Diagnostic {
	if platform == nil {
		return nil
	}

	switch platform.Type {
	case host.PlatformGokrazy:
		path := randomSeedPath(gokrazyRandomSeedPath)
		if _, err := statPath(path); err == nil {
			return nil
		}
		return []diagnostic.Diagnostic{{
			Code:     "doctor-random-seed",
			Severity: diagnostic.SeverityWarning,
			Message:  "gokrazy random seed not found at " + path + "; verify randomd is included in the gokrazy image",
			Path:     path,
			Expected: "randomd seed file",
			Actual:   "missing",
		}}

	case host.PlatformSystemd:
		path := randomSeedPath(systemdRandomSeedPath)
		if _, err := statPath(path); err == nil {
			return nil
		}
		return []diagnostic.Diagnostic{{
			Code:     "doctor-random-seed",
			Severity: diagnostic.SeverityWarning,
			Message:  "systemd random seed not found at " + path + "; systemd-random-seed.service may not be enabled",
			Path:     path,
			Expected: "systemd-random-seed.service seed file",
			Actual:   "missing",
		}}

	case host.PlatformPlainLinux:
		return []diagnostic.Diagnostic{{
			Code:     "doctor-random-seed",
			Severity: diagnostic.SeverityWarning,
			Message:  "non-systemd Linux without a known random-seed service; early-boot entropy may be insufficient for cryptographic operations",
		}}

	default:
		return nil
	}
}

func randomSeedPath(defaultPath string) string {
	if override := strings.TrimSpace(env.Get(doctorRandomSeedEnv)); override != "" {
		return override
	}
	return defaultPath
}

func checkSmartEnabled(tree *config.Tree) []diagnostic.Diagnostic {
	storageCfg := tree.GetContainer("storage")
	if storageCfg == nil {
		return nil
	}
	smartCfg := storageCfg.GetContainer("smart")
	if smartCfg == nil {
		return nil
	}
	enabled, ok := smartCfg.Get("enabled")
	if !ok || enabled != "true" {
		return nil
	}

	entries, err := os.ReadDir("/sys/class/block")
	if err != nil {
		return []diagnostic.Diagnostic{{
			Code:     "doctor-smart-sysfs",
			Severity: diagnostic.SeverityWarning,
			Message:  "cannot enumerate block devices: " + err.Error(),
		}}
	}

	var diags []diagnostic.Diagnostic
	checked := 0
	accessible := 0
	for _, e := range entries {
		name := e.Name()
		if _, statErr := os.Stat(filepath.Join(sysClassBlockDir, name, "partition")); statErr == nil {
			continue
		}
		checked++
		info := smart.Detect(name, "")
		if info == nil {
			continue
		}
		if !info.Unavailable {
			accessible++
		}
	}

	if checked > 0 && accessible == 0 {
		diags = append(diags, diagnostic.Diagnostic{
			Code:     "doctor-smart-access",
			Severity: diagnostic.SeverityWarning,
			Message:  "SMART enabled in config but no devices are accessible (check privileges)",
		})
	}

	return diags
}
