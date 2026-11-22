// Design: docs/features/ai-first.md — system readiness checks for agent tooling
// Related: doctor.go — readiness check runner and output contract
// Related: checks_platform.go — platformMismatch helper used by checkNTPPersistPath

// Storage checks: blob store integrity, free disk space, persistent storage
// smaller than its disk on an appliance, and writability of every
// config-declared file destination (NTP persist-path, BFD persist-dir,
// resolv.conf parent, file:// archives, self-update binary directory).

package doctor

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/host"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/diskspace"
	"github.com/ze-software/ze/internal/core/paths"
	"github.com/ze-software/ze/internal/core/resolve"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/zefs"
)

// defaultNTPPersistPath is the gokrazy default for environment/ntp persist-path.
const defaultNTPPersistPath = "/perm/ze/timefile"

func checkStoreIntegrity(configDir string) []diagnostic.Diagnostic {
	if configDir == "" {
		configDir = resolve.StoreDir("")
	}
	storePath := filepath.Join(configDir, "database")
	var tb textbuf.Buffer
	if _, err := os.Stat(storePath); err != nil {
		// Absence is not corruption: the host has not run ze init yet.
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		// Any other error means the check could not run. An empty result is the
		// HEALTHY verdict, so returning it here would report a store that was
		// never opened (ai/rules/evidence.md).
		return []diagnostic.Diagnostic{{
			Code:     diagnostic.CodeDoctorStoreIntegrity,
			Severity: diagnostic.SeverityError,
			Message:  tb.Str("store unreadable at ").Str(storePath).Str(": ").Err(err).String(),
			Path:     storePath,
		}}
	}

	report, err := zefs.CheckPath(storePath)
	if err != nil {
		return []diagnostic.Diagnostic{{
			Code:     diagnostic.CodeDoctorStoreIntegrity,
			Severity: diagnostic.SeverityError,
			Message:  tb.Str("store integrity check failed: ").Err(err).String(),
		}}
	}

	if report.ContainerError != "" {
		return []diagnostic.Diagnostic{{
			Code:     diagnostic.CodeDoctorStoreIntegrity,
			Severity: diagnostic.SeverityError,
			Message:  tb.Reset().Str("store corrupt: ").Str(report.ContainerError).String(),
		}}
	}

	var diags []diagnostic.Diagnostic
	for _, entry := range report.Entries {
		if entry.Status == "ok" {
			continue
		}
		diags = append(diags, diagnostic.Diagnostic{
			Code:     diagnostic.CodeDoctorStoreIntegrity,
			Severity: diagnostic.SeverityError,
			Message:  tb.Reset().Str("store key ").Str(entry.Key).Str(": ").Str(entry.Error).String(),
			Path:     filepath.Join(storePath, filepath.FromSlash(entry.Key)),
		})
	}
	return append(diags, checkStoreHistory(storePath)...)
}

// checkStoreHistory raises the findings `ze data check` prints for
// content-addressed history: a dangling or malformed entry, a wrong-hash
// object and a pointer naming an unresolvable stamp as errors, an orphan
// object as a warning, and a history key removed while the check ran as a
// warning: the key was listed and gone at its read, which a live store does,
// so it is reported as not read and never as a failed check. A store that
// cannot be opened is an error, never the healthy empty answer.
func checkStoreHistory(storePath string) []diagnostic.Diagnostic {
	var tb textbuf.Buffer
	store, err := storage.OpenTree(storePath, false)
	if err != nil {
		return []diagnostic.Diagnostic{{
			Code:     diagnostic.CodeDoctorStoreIntegrity,
			Severity: diagnostic.SeverityError,
			Message:  tb.Str("store history unreadable at ").Str(storePath).Str(": ").Err(err).String(),
			Path:     storePath,
		}}
	}
	defer store.Close() //nolint:errcheck // read-only doctor handle.
	findings, err := storage.CheckHistory(store)
	if err != nil {
		return []diagnostic.Diagnostic{{
			Code:     diagnostic.CodeDoctorStoreIntegrity,
			Severity: diagnostic.SeverityError,
			Message:  tb.Str("store history check failed: ").Err(err).String(),
			Path:     storePath,
		}}
	}
	var diags []diagnostic.Diagnostic
	for _, finding := range findings {
		severity := diagnostic.SeverityError
		if finding.Severity == storage.HistorySeverityWarning {
			severity = diagnostic.SeverityWarning
		}
		diags = append(diags, diagnostic.Diagnostic{
			Code:     diagnostic.CodeDoctorStoreIntegrity,
			Severity: severity,
			Message:  tb.Reset().Str(finding.Kind).Str(": ").Str(finding.Key).Str(": ").Str(finding.Detail).String(),
			Path:     filepath.Join(storePath, filepath.FromSlash(finding.Key)),
		})
	}
	return diags
}

func checkDiskSpace() []diagnostic.Diagnostic {
	configDir := paths.DefaultConfigDir()
	if configDir == "" {
		return nil
	}
	blocks, err := diskspace.Stat(configDir)
	if err != nil {
		// Absence is not a fault: the host has not run ze init yet. Every OTHER
		// error means the free space was never measured, and an empty result is
		// the HEALTHY verdict (ai/rules/evidence.md).
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		var tb textbuf.Buffer
		return []diagnostic.Diagnostic{{
			Code:     codeDoctorDiskSpace,
			Fix:      "df -h " + configDir,
			Severity: diagnostic.SeverityError,
			Message:  tb.Str("config partition unreadable at ").Str(configDir).Str(": ").Err(err).String(),
			Path:     configDir,
		}}
	}
	return checkDiskSpaceBlocks(configDir, blocks.Total, blocks.Available)
}

func checkDiskSpaceBlocks(configDir string, total, available uint64) []diagnostic.Diagnostic {
	// A filesystem reporting zero total blocks cannot answer the question. Report
	// it rather than divide by zero OR pass it off as healthy.
	if total == 0 {
		var tb textbuf.Buffer
		return []diagnostic.Diagnostic{{
			Code:     codeDoctorDiskSpace,
			Fix:      "df -h " + configDir,
			Severity: diagnostic.SeverityError,
			Message:  tb.Str("config partition reports zero total blocks at ").Str(configDir).String(),
			Path:     configDir,
		}}
	}
	pctFree := diskspace.Percent(available, total)
	if pctFree < 5 {
		pctStr := textbuf.UintStr(pctFree, "%")
		return []diagnostic.Diagnostic{{
			Code:     codeDoctorDiskSpace,
			Fix:      "df -h " + configDir,
			Severity: diagnostic.SeverityWarning,
			Message:  textbuf.StrUintStr("config partition has ", pctFree, "% free space"),
			Path:     configDir,
			Expected: ">= 5%",
			Actual:   pctStr,
		}}
	}
	return nil
}

// The commands a persistent-storage finding names as its fix: the one that
// grows the storage, and the one that reads its sizes.
const (
	fixStorageGrow = "request system storage grow"
	fixStorageShow = "show host storage"
)

// readPersistentStorage is the seam the storage tests replace. The read is the
// host component's, which owns that state. The check is registered here and
// not there because internal/core/diagnostic imports the host component for
// the platform type, so the host component cannot import the check registry.
var readPersistentStorage = host.DetectPersistentStorage

// checkStorageGrown reports the persistent storage of an appliance when it is
// smaller than its disk. The installer grows the partition and its filesystem
// to the disk; a system written to its disk another way, or whose growth
// failed at install, keeps the size of the image until an operator grows it.
//
// A state that cannot be read is a warning, never the healthy empty answer:
// storage that was not measured may be smaller than its disk.
func checkStorageGrown() []diagnostic.Diagnostic {
	storage, err := readPersistentStorage()
	if errors.Is(err, host.ErrNoPersistentStorage) {
		// An answer about the system, not a failed read: nothing is mounted
		// on the persistent mount point, so there is no storage to grow.
		return nil
	}
	var tb textbuf.Buffer
	if err != nil {
		return []diagnostic.Diagnostic{{
			Code:     codeDoctorStorageNotGrown,
			Severity: diagnostic.SeverityWarning,
			Message:  tb.Str("cannot read the size of the persistent storage, so it may be smaller than its disk: ").Err(err).String(),
			Fix:      fixStorageShow,
		}}
	}
	if storage.Grown {
		return nil
	}
	return []diagnostic.Diagnostic{{
		Code:     codeDoctorStorageNotGrown,
		Severity: diagnostic.SeverityWarning,
		Message: tb.Str("persistent storage ").Str(storage.Device).
			Str(" uses ").Str(binarySize(storage.FilesystemBytes)).
			Str(" of a ").Str(binarySize(storage.DiskBytes)).
			Str(" disk, because it was not grown to the disk when this system was installed").String(),
		Path: storage.Device,
		Fix:  fixStorageGrow,
	}}
}

// binarySizeUnits are the units binarySize writes, largest first.
var binarySizeUnits = [...]struct {
	name  string
	bytes uint64
}{
	{"TiB", 1 << 40},
	{"GiB", 1 << 30},
	{"MiB", 1 << 20},
	{"KiB", 1 << 10},
}

// binarySize writes a byte count in the largest binary unit that holds at
// least one of it: 989855744 reads "944 MiB". Below ten units it keeps one
// decimal, rounded down, so 1.9 GiB does not read as 1 GiB.
func binarySize(byteCount uint64) string {
	for _, unit := range binarySizeUnits {
		if byteCount < unit.bytes {
			continue
		}
		whole := byteCount / unit.bytes
		// The remainder is below 1 TiB, so ten times it cannot overflow.
		tenths := (byteCount % unit.bytes) * 10 / unit.bytes
		var tb textbuf.Buffer
		tb.Uint(whole)
		if whole < 10 && tenths > 0 {
			tb.Byte('.').Uint(tenths)
		}
		return tb.Byte(' ').Str(unit.name).String()
	}
	return textbuf.UintStr(byteCount, " bytes")
}

// probeWritable is the seam the storage tests replace. The probe itself lives
// in internal/core/diagnostic so a check an owner package registers runs the
// same one (diagnostic.DoctorProbeWritableDir).
var probeWritable = diagnostic.DoctorProbeWritableDir

func checkWritableDestinations(tree *config.Tree, platform *host.PlatformInfo) []diagnostic.Diagnostic {
	var tb textbuf.Buffer
	var diags []diagnostic.Diagnostic

	if ntp := getContainerPath(tree, "environment", "ntp"); configEnabled(ntp, false) {
		persistPath := valueOrDefault(ntp, "persist-path", defaultNTPPersistPath)
		if persistPath != "" {
			diags = append(diags, checkNTPPersistPath(persistPath, platform)...)
			dir := filepath.Dir(persistPath)
			if err := probeWritable(dir); err != nil {
				diags = append(diags, diagnostic.Diagnostic{
					Code:     codeDoctorWriteDestination,
					Fix:      "chmod u+w " + dir,
					Severity: diagnostic.SeverityWarning,
					Message:  tb.Reset().Str("NTP persist-path parent not writable: ").Str(dir).String(),
					Path:     persistPath,
				})
			}
		}
	}

	if bfd := tree.GetContainer("bfd"); bfd != nil {
		if persistDir, ok := bfd.Get("persist-dir"); ok && persistDir != "" {
			if err := probeWritable(persistDir); err != nil {
				diags = append(diags, diagnostic.Diagnostic{
					Code:     codeDoctorWriteDestination,
					Fix:      "chmod u+w " + persistDir,
					Severity: diagnostic.SeverityWarning,
					Message:  tb.Reset().Str("BFD persist-dir not writable: ").Str(persistDir).String(),
					Path:     persistDir,
				})
			}
		}
	}

	if dns := getContainerPath(tree, "system", "dns"); dns != nil {
		if rcPath, ok := dns.Get("resolv-conf-path"); ok && rcPath != "" {
			dir := filepath.Dir(rcPath)
			if err := probeWritable(dir); err != nil {
				diags = append(diags, diagnostic.Diagnostic{
					Code:     codeDoctorWriteDestination,
					Fix:      "chmod u+w " + dir,
					Severity: diagnostic.SeverityWarning,
					Message:  tb.Reset().Str("DNS resolv-conf-path parent not writable: ").Str(dir).String(),
					Path:     rcPath,
				})
			}
		}
	}

	if system := tree.GetContainer("system"); system != nil {
		for _, a := range system.GetListOrdered("archive") {
			loc, ok := a.Value.Get("location")
			if !ok || loc == "" {
				continue
			}
			if !strings.HasPrefix(loc, "file://") {
				continue
			}
			path := strings.TrimPrefix(loc, "file://")
			if path == "" {
				continue
			}
			if err := probeWritable(path); err != nil {
				diags = append(diags, diagnostic.Diagnostic{
					Code:     codeDoctorWriteDestination,
					Fix:      "chmod u+w " + path,
					Severity: diagnostic.SeverityWarning,
					Message:  tb.Reset().Str("archive ").Str(a.Key).Str(": file location not writable: ").Str(path).String(),
					Path:     path,
				})
			}
		}
	}

	if uc := getContainerPath(tree, "system", "update-check"); uc != nil {
		autoApply, _ := uc.Get("auto-apply")
		if autoApply == configTrueValue {
			if platform != nil && platform.Type == host.PlatformGokrazy {
				return diags
			}
			if exe, err := os.Executable(); err == nil {
				dir := filepath.Dir(exe)
				if writeErr := probeWritable(dir); writeErr != nil {
					diags = append(diags, diagnostic.Diagnostic{
						Code:     codeDoctorWriteDestination,
						Fix:      "chmod u+w " + dir,
						Severity: diagnostic.SeverityWarning,
						Message:  tb.Reset().Str("self-update auto-apply: binary parent not writable: ").Str(dir).String(),
						Path:     exe,
					})
				}
			}
		}
	}

	return diags
}

func checkNTPPersistPath(persistPath string, platform *host.PlatformInfo) []diagnostic.Diagnostic {
	if platform == nil || persistPath == "" || !strings.HasPrefix(persistPath, "/perm/") {
		return nil
	}
	if platform.Type == host.PlatformGokrazy || platform.Type == host.PlatformUnknown || platform.Type == host.PlatformDarwin {
		return nil
	}
	return []diagnostic.Diagnostic{platformMismatch(
		"NTP persist-path uses gokrazy /perm storage on "+platform.Type.String(),
		"environment/ntp/persist-path",
		"non-/perm writable path for "+platform.Type.String(),
		persistPath,
	)}
}
