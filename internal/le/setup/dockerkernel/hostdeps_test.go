package dockerkernel

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// zeKernel72Config is the part of /boot/config-7.2.9-ze, as installed on the
// owner's Ubuntu host on 2026-10-14, that the host's dependencies name.
const zeKernel72Config = `CONFIG_HOTPLUG_PCI=y
# CONFIG_HOTPLUG_PCI_ACPI is not set
# CONFIG_HOTPLUG_PCI_PCIE is not set
CONFIG_NETFILTER_XTABLES=y
CONFIG_IP_NF_FILTER=y
CONFIG_IP6_NF_FILTER=y
CONFIG_NETFILTER_XT_MATCH_CONNTRACK=y
CONFIG_NETFILTER_XT_MATCH_ADDRTYPE=y
CONFIG_NETFILTER_XT_MATCH_MULTIPORT=y
CONFIG_NETFILTER_XT_MATCH_COMMENT=y
CONFIG_NETFILTER_XT_TARGET_LOG=m
# CONFIG_NETFILTER_XT_MATCH_LIMIT is not set
# CONFIG_NETFILTER_XT_MATCH_RECENT is not set
# CONFIG_NETFILTER_XT_MATCH_HL is not set
# CONFIG_IP_NF_TARGET_REJECT is not set
# CONFIG_IP6_NF_TARGET_REJECT is not set
# CONFIG_IP6_NF_MATCH_RT is not set
`

// fixedZeKernelConfig is zeKernel72Config with every option the owner's host
// relies on set.
var fixedZeKernelConfig = zeKernel72Config + `CONFIG_HOTPLUG_PCI_ACPI=y
CONFIG_HOTPLUG_PCI_PCIE=y
CONFIG_NETFILTER_XT_MATCH_LIMIT=y
CONFIG_NETFILTER_XT_MATCH_RECENT=y
CONFIG_NETFILTER_XT_MATCH_HL=y
CONFIG_IP_NF_TARGET_REJECT=y
CONFIG_IP6_NF_TARGET_REJECT=y
CONFIG_IP6_NF_MATCH_RT=y
`

// VALIDATES: the install refuses the kernel that broke the owner's host (a
// slot-named NIC and UFW), naming the interface, UFW and every missing option,
// and accepts the same host once the options are set.
// PREVENTS: an install that boots a host with no network or no firewall.
func TestZeKernelServesHostRefusesWhatBrokeTheOwnersHost(t *testing.T) {
	deps, err := hostDependencies(hostFacts{interfaces: []string{"lo", "enp0s18", "ens18"}, ufwInstalled: true})
	if err != nil {
		t.Fatalf("dependencies: %v", err)
	}
	err = zeKernelServesHost("7.2.9-ze", zeKernel72Config, deps)
	if err == nil {
		t.Fatal("a kernel without PCI hotplug and UFW's matches was accepted")
	}
	for _, want := range []string{"interface ens18 is named by its PCI slot", "CONFIG_HOTPLUG_PCI_ACPI", "CONFIG_HOTPLUG_PCI_PCIE",
		"UFW is installed", "CONFIG_NETFILTER_XT_MATCH_LIMIT", "CONFIG_IP_NF_TARGET_REJECT", "CONFIG_IP6_NF_TARGET_REJECT",
		"CONFIG_NETFILTER_XT_MATCH_RECENT", "CONFIG_NETFILTER_XT_MATCH_HL", "CONFIG_IP6_NF_MATCH_RT", "no step ran",
		"gokrazy/kernel/runtime.config"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal lacks %q:\n%v", want, err)
		}
	}
	if strings.Contains(err.Error(), "enp0s18") {
		t.Errorf("a path-named interface was reported as relying on slot naming:\n%v", err)
	}
	if strings.Contains(err.Error(), "CONFIG_NETFILTER_XT_TARGET_LOG") {
		t.Errorf("a module (=m) option was reported missing:\n%v", err)
	}
	if err := zeKernelServesHost("7.2.9-ze", fixedZeKernelConfig, deps); err != nil {
		t.Fatalf("the fixed kernel was refused: %v", err)
	}
}

// VALIDATES: a host whose interfaces are path-named, with no UFW and no
// firewall module loaded, relies on none of these options.
// PREVENTS: the check refusing a host for options it never uses.
func TestHostDependenciesNoneForAPlainHost(t *testing.T) {
	deps, err := hostDependencies(hostFacts{interfaces: []string{"lo", "eth0", "enp3s0", "docker0"}, modules: []string{"ext4", "veth"}})
	if err != nil {
		t.Fatalf("dependencies: %v", err)
	}
	if len(deps) != 0 {
		t.Fatalf("dependencies = %+v, want none", deps)
	}
	if err := zeKernelServesHost("7.2.9-ze", "", deps); err != nil {
		t.Fatalf("a host relying on nothing was refused: %v", err)
	}
}

// VALIDATES: a loaded firewall module becomes a dependency on its option, and
// a loaded module of those families the table does not map is refused by
// name rather than passed unchecked.
// PREVENTS: an iptables rule set the check cannot see breaking on reboot.
func TestHostDependenciesLoadedModules(t *testing.T) {
	deps, err := hostDependencies(hostFacts{modules: []string{"xt_limit", "ext4", "ipt_REJECT"}})
	if err != nil {
		t.Fatalf("dependencies: %v", err)
	}
	if len(deps) != 1 {
		t.Fatalf("dependencies = %+v, want one", deps)
	}
	want := []string{"CONFIG_IP_NF_TARGET_REJECT", "CONFIG_NETFILTER_XT_MATCH_LIMIT"}
	if !slices.Equal(deps[0].options, want) {
		t.Fatalf("options = %v, want %v", deps[0].options, want)
	}
	_, err = hostDependencies(hostFacts{modules: []string{"xt_owner"}})
	if err == nil {
		t.Fatal("an unmapped netfilter module was accepted")
	}
	if !strings.Contains(err.Error(), "xt_owner") {
		t.Fatalf("refusal does not name the module: %v", err)
	}
}

// VALIDATES: the facts come from /sys/class/net, the netplan-generated
// networkd file names, /proc/modules and UFW's config, under the given root.
// PREVENTS: a renamed interface whose only trace is netplan's config going
// unseen, and an unreadable source read as "relies on nothing".
func TestReadHostFacts(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"sys/class/net/enp0s18/type":                   "1\n",
		"sys/class/net/lo/type":                        "772\n",
		"run/systemd/network/10-netplan-ens18.network": "",
		"proc/modules":                                 "xt_limit 12288 1 - Live 0x0\next4 1003520 2 - Live 0x0\n",
		"etc/ufw/ufw.conf":                             "ENABLED=yes\n",
	}
	for name, content := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("make %s: %v", name, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	facts, err := readHostFacts(root)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !slices.Equal(facts.interfaces, []string{"enp0s18", "lo", "ens18"}) {
		t.Errorf("interfaces = %v", facts.interfaces)
	}
	if !slices.Equal(facts.modules, []string{"xt_limit", "ext4"}) {
		t.Errorf("modules = %v", facts.modules)
	}
	if !facts.ufwInstalled {
		t.Error("UFW's config was not seen")
	}
	if err := os.Remove(filepath.Join(root, "proc", "modules")); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := readHostFacts(root); err == nil {
		t.Fatal("a host whose module list cannot be read was read as relying on nothing")
	}
}

// VALIDATES: every option the check can require is built in by the runtime
// profile, so the kernel Ze builds serves every host the check knows.
// PREVENTS: the check and the profile drifting apart.
func TestHostDependencyOptionsAreInTheRuntimeProfile(t *testing.T) {
	config, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "gokrazy", "kernel", "runtime.config"))
	if err != nil {
		t.Fatalf("read runtime.config: %v", err)
	}
	built := kernelOptionsSet(string(config))
	options := slices.Clone(slotNameOptions)
	for _, option := range netfilterModuleOptions {
		options = append(options, option)
	}
	for _, option := range options {
		if !built[option] {
			t.Errorf("%s is not =y in gokrazy/kernel/runtime.config", option)
		}
	}
}

// VALIDATES: grub.cfg is read as the user, and through `sudo cat` only when
// that read is refused for permission; any other error is returned as is.
// PREVENTS: the install failing with "permission denied" on a host whose
// grub.cfg is root-only, after every other root step has run.
func TestReadGrubConfigFallsBackToSudo(t *testing.T) {
	var asRoot []installStep
	readAsRoot := func(step installStep) ([]byte, error) {
		asRoot = append(asRoot, step)
		return []byte("menuentry 'Ubuntu'"), nil
	}
	denied := func(string) ([]byte, error) {
		return nil, &fs.PathError{Op: "open", Path: grubConfigPath, Err: fs.ErrPermission}
	}
	got, err := readGrubConfig(denied, readAsRoot)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != "menuentry 'Ubuntu'" {
		t.Fatalf("read %q", got)
	}
	if len(asRoot) != 1 || !slices.Equal(asRoot[0].Argv, []string{"sudo", "cat", grubConfigPath}) {
		t.Fatalf("root reads = %+v, want one `sudo cat %s`", asRoot, grubConfigPath)
	}

	asRoot = nil
	readable := func(string) ([]byte, error) { return []byte("plain"), nil }
	if got, err := readGrubConfig(readable, readAsRoot); err != nil || string(got) != "plain" || len(asRoot) != 0 {
		t.Fatalf("readable grub.cfg: %q, %v, root reads %d", got, err, len(asRoot))
	}
	missing := func(string) ([]byte, error) { return nil, fs.ErrNotExist }
	if _, err := readGrubConfig(missing, readAsRoot); !errors.Is(err, fs.ErrNotExist) || len(asRoot) != 0 {
		t.Fatalf("missing grub.cfg: %v, root reads %d", err, len(asRoot))
	}
}
