// Design: docs/architecture/diagnostics/crash-capture.md -- the system crash-dump config subtree
// Overview: system.go -- SystemConfig, which carries this subtree
// Related: internal/core/crashlog/kernel.go -- the harvest and the readiness answer this feeds
//
// The `system crash-dump` subtree is the operator's INTENT. It is not the state
// of the machine: a reservation is a kernel boot argument, so a commit records
// what the operator wants and the next boot of an image built with
// image.crash-dump is what arms it. Readiness reports the two separately, and
// crashlog computes the armed half from the running kernel rather than from
// anything here.

package system

import (
	"fmt"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/configvalue"
	"github.com/ze-software/ze/internal/core/crashlog"
	"github.com/ze-software/ze/internal/core/resolve"
)

// The defaults of the `system crash-dump` leaves, kept beside the extraction so
// a value the operator did not set reads the same here as in the YANG schema.
const (
	crashDumpReserveDefault     uint16 = 16
	crashDumpMemoryImageDefault uint16 = 256
)

// CrashDumpConfig holds kernel crash capture settings from
// system { crash-dump {} }.
type CrashDumpConfig struct {
	Enabled              bool
	ReserveMegabytes     uint16
	MemoryImage          bool
	MemoryImageMegabytes uint16
}

// Intent lowers the subtree into the value crashlog answers readiness from.
// crashlog is a core package and reads no config, so this is the one place the
// four leaves cross from the config tree into the crash subsystem.
func (c CrashDumpConfig) Intent() crashlog.Intent {
	return crashlog.Intent{
		Enabled:              c.Enabled,
		ReserveMegabytes:     c.ReserveMegabytes,
		MemoryImage:          c.MemoryImage,
		MemoryImageMegabytes: c.MemoryImageMegabytes,
	}
}

// LoadCrashDumpIntent reads the crash-capture intent out of the active config.
//
// Every surface that reports readiness reads it through this one call, so the
// answer cannot drift between the daemon RPC, the offline fallback that runs
// when the daemon has died, and the support bundle. configPath selects a file;
// empty means the instance's own config.
//
// An error is returned rather than a zero intent, because a config that could
// not be read is not a config that says crash capture is off.
func LoadCrashDumpIntent(configPath string) (crashlog.Intent, error) {
	name, data, err := readCrashDumpConfig(configPath)
	if err != nil {
		return crashlog.Intent{}, err
	}

	result, parseErr := config.LoadConfig(string(data), name, nil)
	if parseErr != nil {
		return crashlog.Intent{}, fmt.Errorf("crash-dump intent: parse %s: %w", name, parseErr)
	}
	sys := result.Tree.GetContainer("system")
	if sys == nil {
		return CrashDumpConfig{
			ReserveMegabytes:     crashDumpReserveDefault,
			MemoryImageMegabytes: crashDumpMemoryImageDefault,
		}.Intent(), nil
	}
	return extractCrashDump(sys).Intent(), nil
}

// readCrashDumpConfig reads the config text the intent is extracted from, and
// names the file it read. A named path is read as a file; the empty path resolves
// the instance's own config out of the config store.
//
// The store is opened only for the empty path, so a caller that names a file
// needs no storage at all: that is the path `ze support --config` takes, and the
// path a test takes.
func readCrashDumpConfig(configPath string) (name string, data []byte, err error) {
	if configPath != "" {
		data, err = cliio.ReadFile(configPath) // "-" reads stdin
		if err != nil {
			return "", nil, fmt.Errorf("crash-dump intent: read %s: %w", configPath, err)
		}
		return configPath, data, nil
	}

	store, err := resolve.Storage()
	if err != nil {
		return "", nil, fmt.Errorf("crash-dump intent: storage: %w", err)
	}
	defer store.Close() //nolint:errcheck // read-only close

	name = resolve.DefaultConfig(store)
	data, err = store.ReadFile(name)
	if err != nil {
		return "", nil, fmt.Errorf("crash-dump intent: read %s: %w", name, err)
	}
	return name, data, nil
}

// extractCrashDump reads system { crash-dump {} }. A subtree the operator never
// wrote reads as the schema defaults, so a caller gets the same values whether
// the container is absent or present and empty.
func extractCrashDump(sys *config.Tree) CrashDumpConfig {
	cd := CrashDumpConfig{
		ReserveMegabytes:     crashDumpReserveDefault,
		MemoryImageMegabytes: crashDumpMemoryImageDefault,
	}

	crash := sys.GetContainer("crash-dump")
	if crash == nil {
		return cd
	}

	if v, ok := crash.Get("enabled"); ok {
		if enabled, read := configvalue.Bool(v); read {
			cd.Enabled = enabled
		}
	}
	if v, ok := crash.Get("reserve"); ok {
		cd.ReserveMegabytes = boundedMegabytes(v, crashlog.ReserveMegabytesMin, crashlog.ReserveMegabytesMax, cd.ReserveMegabytes)
	}

	image := crash.GetContainer("memory-image")
	if image == nil {
		return cd
	}
	if v, ok := image.Get("enabled"); ok {
		if enabled, read := configvalue.Bool(v); read {
			cd.MemoryImage = enabled
		}
	}
	if v, ok := image.Get("reserve"); ok {
		cd.MemoryImageMegabytes = boundedMegabytes(v, crashlog.MemoryImageMegabytesMin, crashlog.MemoryImageMegabytesMax, cd.MemoryImageMegabytes)
	}
	return cd
}

// boundedMegabytes coerces a reserve leaf and keeps the caller's default when
// the value is outside the range the YANG schema publishes. The schema refuses
// an out-of-range value at commit, so a value that reaches here outside the
// range came from a tree nobody validated, and the default is the safe answer.
func boundedMegabytes(value string, low, high, fallback uint16) uint16 {
	n, read := configvalue.Int(value)
	if !read {
		return fallback
	}
	if n < int64(low) || n > int64(high) {
		return fallback
	}
	return uint16(n) //nolint:gosec // bounded by the range check above
}
