// Design: docs/guide/developer-setup.md -- kernel features the tests need
//
// A few kernel tests need an option that Ze's own kernel builds in and a
// distribution kernel builds as a module, or not at all: the nftables
// flowtable, and the tc police and gact actions. Without the option the kernel
// answers ENOENT or EOPNOTSUPP, which names nothing. This package says which
// option is missing and how to get it, in the failure message of a test that
// needs one. Such a test runs on Ze's kernel, which carries every option here.

package kernelfeature

import (
	"bufio"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// Feature is one kernel option a test needs, and the module that provides it
// where a kernel builds the option as a module.
type Feature struct {
	// Symbol is the Kconfig name, CONFIG_ prefix included.
	Symbol string
	// Module is the module name modprobe and /proc/modules use.
	Module string
	// Tests names the tests that need the option, for a reader.
	Tests string
}

// flowtableTests names the tests the three flowtable options serve.
const flowtableTests = "the nftables flowtable test"

// Features is every option the tests need from a host kernel. Each is =y in
// Ze's kernel fragments (gokrazy/kernel), which TestZesKernelBuildsEveryFeatureIn
// checks. The order is load order: nf_flow_table before the two modules that
// depend on it.
var Features = []Feature{
	{Symbol: "CONFIG_NF_FLOW_TABLE", Module: "nf_flow_table", Tests: flowtableTests},
	{Symbol: "CONFIG_NF_FLOW_TABLE_INET", Module: "nf_flow_table_inet", Tests: flowtableTests},
	{Symbol: "CONFIG_NFT_FLOW_OFFLOAD", Module: "nft_flow_offload", Tests: flowtableTests},
	{Symbol: "CONFIG_NET_ACT_POLICE", Module: "act_police", Tests: "the tc ingress policer tests"},
	{Symbol: "CONFIG_NET_ACT_GACT", Module: "act_gact", Tests: "the tc ingress policer coexistence test"},
}

// MustLookup answers the feature for symbol. A symbol the table does not hold
// is a defect in the caller, which names its features as constants.
func MustLookup(symbol string) Feature {
	for _, feature := range Features {
		if feature.Symbol == symbol {
			return feature
		}
	}
	panic("BUG: kernelfeature: no feature " + symbol)
}

// State is what a kernel offers of one feature.
type State uint8

const (
	// StateUnspecified is the zero value, never an answer.
	StateUnspecified State = iota
	// BuiltIn means the option is =y.
	BuiltIn
	// Loaded means the option is =m and the module is loaded.
	Loaded
	// NotLoaded means the option is =m and the module is not loaded.
	NotLoaded
	// Absent means the kernel does not build the option at all.
	Absent
)

// String answers the state for a reader.
func (s State) String() string {
	switch s {
	case StateUnspecified:
		return "unspecified"
	case BuiltIn:
		return "built-in"
	case Loaded:
		return "loaded"
	case NotLoaded:
		return "not-loaded"
	case Absent:
		return "absent"
	}
	return "invalid"
}

// Kernel is one kernel's config and its loaded modules.
type Kernel struct {
	release string
	config  map[string]string
	loaded  map[string]bool
}

// New answers a kernel from its release, its config symbols and the names of
// its loaded modules.
func New(release string, config map[string]string, loaded []string) Kernel {
	set := make(map[string]bool, len(loaded))
	for _, module := range loaded {
		set[module] = true
	}
	return Kernel{release: release, config: config, loaded: set}
}

// Release answers the kernel release, as uname -r prints it.
func (k Kernel) Release() string { return k.release }

// State answers what this kernel offers of feature.
func (k Kernel) State(feature Feature) State {
	value, set := k.config[feature.Symbol]
	if !set {
		return Absent
	}
	if value == "y" {
		return BuiltIn
	}
	if value != "m" {
		// A tristate holds y or m when set; any other value builds nothing.
		return Absent
	}
	if k.loaded[feature.Module] {
		return Loaded
	}
	return NotLoaded
}

// Read answers the running kernel. The config comes from /proc/config.gz, else
// from /boot/config-<release>, which Debian and Ubuntu install; the loaded
// modules from /proc/modules.
func Read() (Kernel, error) {
	var name unix.Utsname
	if err := unix.Uname(&name); err != nil {
		var tb textbuf.Buffer
		return Kernel{}, errors.New(tb.Str("uname: ").Err(err).String())
	}
	release := unix.ByteSliceToString(name.Release[:])

	config, err := readRunningConfig(release)
	if err != nil {
		return Kernel{}, err
	}
	loaded, err := readLoadedModules()
	if err != nil {
		return Kernel{}, err
	}
	return New(release, config, loaded), nil
}

// readRunningConfig reads the running kernel's config from the first place
// that holds it.
func readRunningConfig(release string) (map[string]string, error) {
	file, err := os.Open("/proc/config.gz")
	if err == nil {
		defer file.Close() //nolint:errcheck // read-only close
		unzipped, err := gzip.NewReader(file)
		if err != nil {
			var tb textbuf.Buffer
			return nil, errors.New(tb.Str("read /proc/config.gz: ").Err(err).String())
		}
		return ParseConfig(unzipped)
	}
	if !errors.Is(err, os.ErrNotExist) {
		var tb textbuf.Buffer
		return nil, errors.New(tb.Str("read /proc/config.gz: ").Err(err).String())
	}

	bootConfig := "/boot/config-" + release
	file, err = os.Open(bootConfig) //nolint:gosec // the path is /boot/config- and the release uname answered
	if err != nil {
		var tb textbuf.Buffer
		return nil, errors.New(tb.Str("the kernel config is in neither /proc/config.gz nor ").
			Str(bootConfig).Str(": ").Err(err).String())
	}
	defer file.Close() //nolint:errcheck // read-only close
	return ParseConfig(file)
}

// readLoadedModules answers the first field of each /proc/modules line.
func readLoadedModules() ([]string, error) {
	body, err := os.ReadFile("/proc/modules")
	if errors.Is(err, os.ErrNotExist) {
		// A kernel built with CONFIG_MODULES=n has no /proc/modules: no
		// module is loaded, because none can be.
		return nil, nil
	}
	if err != nil {
		var tb textbuf.Buffer
		return nil, errors.New(tb.Str("read /proc/modules: ").Err(err).String())
	}
	var loaded []string
	for line := range strings.Lines(string(body)) {
		module, _, _ := strings.Cut(line, " ")
		loaded = append(loaded, strings.TrimSpace(module))
	}
	return loaded, nil
}

// ParseConfig reads a kernel config: each CONFIG_<name>=<value> line becomes
// one entry. A "# CONFIG_<name> is not set" line adds nothing, so an unset
// symbol and one the config never names read the same.
func ParseConfig(r io.Reader) (map[string]string, error) {
	config := make(map[string]string)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "CONFIG_") {
			continue
		}
		symbol, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		config[symbol] = value
	}
	if err := scanner.Err(); err != nil {
		var tb textbuf.Buffer
		return nil, errors.New(tb.Str("read kernel config: ").Err(err).String())
	}
	return config, nil
}

// Explain answers, for a test that failed, why the running kernel may be the
// cause: one line for each named feature it lacks or has not loaded, each with
// its fix. It answers "" when every feature is usable, and says so when the
// running kernel cannot be read. It never changes whether the test fails.
func Explain(symbols ...string) string {
	kernel, err := Read()
	if err != nil {
		var tb textbuf.Buffer
		return tb.Str("\nwhether this kernel has ").Join(symbols, ", ").
			Str(" is unknown: ").Err(err).String()
	}
	return kernel.Explain(symbols...)
}

// Explain answers Explain's lines for this kernel.
func (k Kernel) Explain(symbols ...string) string {
	var tb textbuf.Buffer
	for _, symbol := range symbols {
		feature := MustLookup(symbol)
		switch k.State(feature) {
		case BuiltIn, Loaded:
			continue
		case NotLoaded:
			tb.Str("\nthis kernel (").Str(k.release).Str(") builds ").Str(symbol).
				Str(" as the module ").Str(feature.Module).Str(", which is not loaded; ").
				Str(feature.Tests).Str(" cannot run without it. Boot Ze's kernel, which builds it in, or load it now with: sudo modprobe ").
				Str(feature.Module)
		case Absent:
			tb.Str("\nthis kernel (").Str(k.release).Str(") lacks ").Str(symbol).
				Str(", needed by ").Str(feature.Tests).
				Str(". Boot Ze's kernel, which builds it in (gokrazy/kernel): on a Linux host, ./le setup docker kernel install, then reboot")
		case StateUnspecified:
			panic("BUG: kernelfeature: State answered no state")
		}
	}
	return tb.String()
}
