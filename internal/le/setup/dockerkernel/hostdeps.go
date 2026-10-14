// Design: docs/architecture/appliance/kernel-profiles.md -- what a Docker host relies on the Ze kernel for
// Related: dockerkernel.go -- runDockerKernelInstall, which refuses through zeKernelServesHost before any root step
//
// hostdeps.go answers whether the Ze kernel an install is about to place
// carries what this host's own setup relies on its running kernel for: the
// names its network config gives its interfaces, and the iptables matches and
// targets its firewall loads. A kernel that lacks one boots a host with no
// network or no firewall (owner's Ubuntu host on 7.2.9-ze, 2026-10-14), so the
// install refuses before any root step instead.

package dockerkernel

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// netfilterModuleOptions maps each iptables table, match and target module a
// host's firewall loads to the Ze kernel option that provides it. It is the
// one table both detectors read: the modules loaded now, and the ones UFW's
// stock rule set loads. A loaded module of these families that is missing here
// is refused by name, never passed unchecked.
var netfilterModuleOptions = map[string]string{
	"iptable_filter":  "CONFIG_IP_NF_FILTER",
	"iptable_mangle":  "CONFIG_IP_NF_MANGLE",
	"iptable_nat":     "CONFIG_IP_NF_NAT",
	"iptable_raw":     "CONFIG_IP_NF_RAW",
	"ip6table_filter": "CONFIG_IP6_NF_FILTER",
	"ip6table_mangle": "CONFIG_IP6_NF_MANGLE",
	"ip6table_nat":    "CONFIG_IP6_NF_NAT",
	"ip6table_raw":    "CONFIG_IP6_NF_RAW",
	"ipt_REJECT":      "CONFIG_IP_NF_TARGET_REJECT",
	"ip6t_REJECT":     "CONFIG_IP6_NF_TARGET_REJECT",
	"ip6t_ipv6header": "CONFIG_IP6_NF_MATCH_IPV6HEADER",
	"ip6t_rt":         "CONFIG_IP6_NF_MATCH_RT",
	"xt_LOG":          "CONFIG_NETFILTER_XT_TARGET_LOG",
	"xt_MASQUERADE":   "CONFIG_NETFILTER_XT_TARGET_MASQUERADE",
	"xt_addrtype":     "CONFIG_NETFILTER_XT_MATCH_ADDRTYPE",
	"xt_comment":      "CONFIG_NETFILTER_XT_MATCH_COMMENT",
	"xt_conntrack":    "CONFIG_NETFILTER_XT_MATCH_CONNTRACK",
	"xt_hl":           "CONFIG_NETFILTER_XT_MATCH_HL",
	"xt_ipvs":         "CONFIG_NETFILTER_XT_MATCH_IPVS",
	"xt_limit":        "CONFIG_NETFILTER_XT_MATCH_LIMIT",
	"xt_mark":         "CONFIG_NETFILTER_XT_MARK",
	"xt_multiport":    "CONFIG_NETFILTER_XT_MATCH_MULTIPORT",
	"xt_nat":          "CONFIG_NETFILTER_XT_NAT",
	"xt_recent":       "CONFIG_NETFILTER_XT_MATCH_RECENT",
	"xt_state":        "CONFIG_NETFILTER_XT_MATCH_STATE",
	"xt_tcpudp":       "CONFIG_NETFILTER_XTABLES",
}

// netfilterModulePrefixes are the module families netfilterModuleOptions
// answers for. A loaded module with one of these prefixes is a firewall
// dependency; any other module is not this check's question.
var netfilterModulePrefixes = []string{"iptable_", "ip6table_", "ipt_", "ip6t_", "xt_"}

// ufwModules are what Ubuntu's stock UFW rule set (/usr/share/ufw/iptables
// and the chains ufw writes) loads: the conntrack, addrtype, multiport,
// comment, limit, recent, hl and rt matches, and the LOG and REJECT targets.
var ufwModules = []string{
	"iptable_filter", "ip6table_filter", "xt_tcpudp", "xt_conntrack", "xt_addrtype", "xt_multiport",
	"xt_comment", "xt_limit", "xt_recent", "xt_hl", "ip6t_rt", "xt_LOG", "ipt_REJECT", "ip6t_REJECT",
}

// slotNameOptions are what keeps an interface named by its PCI slot: the
// hotplug drivers that publish /sys/bus/pci/slots, from which udev builds the
// slot name (acpiphp for ACPI slots, pciehp for PCIe root ports).
var slotNameOptions = []string{"CONFIG_HOTPLUG_PCI", "CONFIG_HOTPLUG_PCI_ACPI", "CONFIG_HOTPLUG_PCI_PCIE"}

// slotNamePrefixes are systemd's slot-name prefixes (systemd.net-naming-scheme):
// Ethernet, WLAN and WWAN, each followed by `s` and the slot number.
var slotNamePrefixes = []string{"ens", "wls", "wws"}

// netplanNetworkPrefix and netplanNetworkSuffix frame the interface id in the
// networkd file netplan generates. The file itself is root-only on Ubuntu, so
// its name is what tells this check which interface the config names.
const (
	netplanNetworkPrefix = "10-netplan-"
	netplanNetworkSuffix = ".network"
)

// hostFacts is what the check reads from the host: every interface name the
// host has or its network config names, the kernel modules loaded now, and
// whether UFW is installed.
type hostFacts struct {
	interfaces   []string
	modules      []string
	ufwInstalled bool
}

// hostDependency is one thing the host relies on its kernel for, what a kernel
// without it does to the host, and the Ze kernel options that provide it.
type hostDependency struct {
	relies  string
	breaks  string
	options []string
}

// readHostFacts reads the host under root ("/" outside tests). A source that
// cannot be read is an error, never read as "the host relies on nothing";
// only netplan's directory and UFW's config may be absent.
func readHostFacts(root string) (hostFacts, error) {
	var facts hostFacts
	links, err := os.ReadDir(filepath.Join(root, "sys", "class", "net"))
	if err != nil {
		return hostFacts{}, err
	}
	for _, link := range links {
		facts.interfaces = append(facts.interfaces, link.Name())
	}
	generated, err := os.ReadDir(filepath.Join(root, "run", "systemd", "network"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return hostFacts{}, err
	}
	for _, file := range generated {
		id, ok := strings.CutPrefix(file.Name(), netplanNetworkPrefix)
		if !ok {
			continue
		}
		if id, ok = strings.CutSuffix(id, netplanNetworkSuffix); ok {
			facts.interfaces = append(facts.interfaces, id)
		}
	}
	modules, err := os.ReadFile(filepath.Join(root, "proc", "modules")) //nolint:gosec // the kernel's module list under the host root
	if err != nil {
		return hostFacts{}, err
	}
	for line := range strings.SplitSeq(string(modules), "\n") {
		if name, _, ok := strings.Cut(line, " "); ok {
			facts.modules = append(facts.modules, name)
		}
	}
	_, err = os.Stat(filepath.Join(root, "etc", "ufw", "ufw.conf"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return hostFacts{}, err
	}
	facts.ufwInstalled = err == nil
	return facts, nil
}

// hostDependencies answers what the host relies on its kernel for, one entry
// per interface named by its PCI slot, one for the netfilter modules loaded
// now, and one for UFW when it is installed. It refuses a loaded netfilter
// module netfilterModuleOptions does not map, because the check cannot answer
// for it.
func hostDependencies(facts hostFacts) ([]hostDependency, error) {
	var deps []hostDependency
	seen := map[string]bool{}
	for _, name := range facts.interfaces {
		if seen[name] {
			continue
		}
		seen[name] = true
		if !slotNamed(name) {
			continue
		}
		deps = append(deps, hostDependency{
			relies:  "interface " + name + " is named by its PCI slot",
			breaks:  "udev would name it by its path instead, and the network config written for " + name + " would match nothing",
			options: slotNameOptions,
		})
	}
	var loaded []string
	for _, module := range facts.modules {
		if !netfilterModule(module) {
			continue
		}
		if _, known := netfilterModuleOptions[module]; !known {
			return nil, errors.New("the netfilter module " + module + " is loaded, and this check does not know which kernel" +
				" option provides it; add it to netfilterModuleOptions in internal/le/setup/dockerkernel/hostdeps.go")
		}
		loaded = append(loaded, module)
	}
	if len(loaded) > 0 {
		deps = append(deps, hostDependency{
			relies:  "the firewall rules loaded now use " + strings.Join(loaded, ", "),
			breaks:  "they would fail to load",
			options: moduleOptions(loaded),
		})
	}
	if facts.ufwInstalled {
		deps = append(deps, hostDependency{
			relies:  "UFW is installed (/etc/ufw/ufw.conf)",
			breaks:  "ufw.service would fail to load its rules (\"Extension limit revision 0 not supported\")",
			options: moduleOptions(ufwModules),
		})
	}
	return deps, nil
}

// zeKernelServesHost refuses a Ze kernel config that leaves any dependency an
// option it neither builds in nor builds as a module, naming each dependency,
// what it would break, and the options it lacks.
func zeKernelServesHost(release, config string, deps []hostDependency) error {
	built := kernelOptionsSet(config)
	var refusal textbuf.Buffer
	unmet := 0
	for _, dep := range deps {
		var missing []string
		for _, option := range dep.options {
			if !built[option] {
				missing = append(missing, option)
			}
		}
		if len(missing) == 0 {
			continue
		}
		unmet++
		refusal.Str("\n  - ").Str(dep.relies).Str(": without ").Str(strings.Join(missing, ", ")).
			Str(" ").Str(dep.breaks)
	}
	if unmet == 0 {
		return nil
	}
	var tb textbuf.Buffer
	return errors.New(tb.Str("the Ze kernel ").Str(release).Str(" lacks what this host relies on its kernel for,").
		Str(" and no step ran:").Str(refusal.String()).
		Str("\nEnable the options in gokrazy/kernel/runtime.config and runtime.require, then rebuild:\n").
		Str(runtimeKernelRebuild).String())
}

// kernelOptionsSet answers the options a kernel config builds in or as a
// module.
func kernelOptionsSet(config string) map[string]bool {
	set := map[string]bool{}
	for line := range strings.SplitSeq(config, "\n") {
		option, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		if value == "y" {
			set[option] = true
		}
		if value == "m" {
			set[option] = true
		}
	}
	return set
}

// moduleOptions answers the distinct options netfilterModuleOptions maps the
// modules to, sorted.
func moduleOptions(modules []string) []string {
	var options []string
	for _, module := range modules {
		option := netfilterModuleOptions[module]
		if !slices.Contains(options, option) {
			options = append(options, option)
		}
	}
	slices.Sort(options)
	return options
}

// netfilterModule reports whether a module belongs to a family
// netfilterModuleOptions answers for.
func netfilterModule(module string) bool {
	for _, prefix := range netfilterModulePrefixes {
		if strings.HasPrefix(module, prefix) {
			return true
		}
	}
	return false
}

// slotNamed reports whether an interface name is a systemd slot name: a slot
// prefix followed by the slot number.
func slotNamed(name string) bool {
	for _, prefix := range slotNamePrefixes {
		rest, ok := strings.CutPrefix(name, prefix)
		if ok && rest != "" && rest[0] >= '0' && rest[0] <= '9' {
			return true
		}
	}
	return false
}
