// Design: docs/features/ai-first.md — system readiness checks for agent tooling
// Related: doctor.go — readiness check runner and output contract
// Related: checks_platform.go — platformMismatch helper used by checkNTPPersistPath

// Storage checks: blob store integrity, free disk space, and writability of
// every config-declared file destination (NTP persist-path, BFD persist-dir,
// resolv.conf parent, file:// archives, self-update binary directory).

package doctor

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/host"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/paths"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/zefs"
)

// defaultNTPPersistPath is the gokrazy default for environment/ntp persist-path.
const defaultNTPPersistPath = "/perm/ze/timefile"

func checkStoreIntegrity() []diagnostic.Diagnostic {
	configDir := paths.DefaultConfigDir()
	if configDir == "" {
		return nil
	}
	storePath := filepath.Join(configDir, "database.zefs")
	if _, err := os.Stat(storePath); err != nil {
		return nil
	}

	var tb textbuf.Buffer
	report, err := zefs.Check(storePath)
	if err != nil {
		return []diagnostic.Diagnostic{{
			Code:     "doctor-store-integrity",
			Severity: diagnostic.SeverityError,
			Message:  tb.Str("store integrity check failed: ").Err(err).String(),
		}}
	}

	if report.ContainerError != "" {
		return []diagnostic.Diagnostic{{
			Code:     "doctor-store-integrity",
			Severity: diagnostic.SeverityError,
			Message:  tb.Reset().Str("store corrupt: ").Str(report.ContainerError).String(),
		}}
	}

	if report.CorruptEntries > 0 {
		return []diagnostic.Diagnostic{{
			Code:     "doctor-store-integrity",
			Severity: diagnostic.SeverityError,
			Message:  tb.Reset().Str("store has ").Int(int64(report.CorruptEntries)).Str(" corrupt entries").String(),
		}}
	}

	return nil
}
func checkDiskSpace() []diagnostic.Diagnostic {
	configDir := paths.DefaultConfigDir()
	if configDir == "" {
		return nil
	}
	var stat syscall.Statfs_t
	if err := syscall.Statfs(configDir, &stat); err != nil {
		return nil
	}
	if stat.Blocks == 0 {
		return nil
	}
	pctFree := (stat.Bavail * 100) / stat.Blocks
	if pctFree < 5 {
		pctStr := textbuf.UintStr(pctFree, "%")
		return []diagnostic.Diagnostic{{
			Code:     "doctor-disk-space",
			Severity: diagnostic.SeverityWarning,
			Message:  textbuf.StrUintStr("config partition has ", pctFree, "% free space"),
			Path:     configDir,
			Expected: ">= 5%",
			Actual:   pctStr,
		}}
	}
	return nil
}

var probeWritable = probeWritableDir

func probeWritableDir(dir string) error {
	if dir == "" {
		return errors.New("empty path")
	}
	f, err := os.CreateTemp(dir, ".ze-doctor-probe-*")
	if err != nil {
		return err
	}
	name := f.Name()
	closeErr := f.Close()
	removeErr := os.Remove(name)
	if closeErr != nil {
		return closeErr
	}
	return removeErr
}

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
					Code:     "doctor-write-destination",
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
					Code:     "doctor-write-destination",
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
					Code:     "doctor-write-destination",
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
					Code:     "doctor-write-destination",
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
						Code:     "doctor-write-destination",
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
