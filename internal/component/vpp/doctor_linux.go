//go:build linux

// Design: plan/learned/1105-vpp-host-tuning.md -- ze doctor check for the boot-time
// hugepage reservation VPP depends on. Reads sysfs/procfs behind overridable
// roots so it is unit-testable against fixtures. Registered from the vpp
// component (owning package) via register_linux.go.

package vpp

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	doctorVPPHugepagesCode = "doctor-vpp-hugepages"
	// doctorHugepagesRootEnv, when set, prefixes every procfs/sysfs path the
	// check reads so functional tests can run it hermetically against fixtures.
	doctorHugepagesRootEnv = "ze.test.doctor.hugepages-root"
)

// hugepageDoctorRoots are the procfs/sysfs paths the hugepage doctor check reads.
// Held in a struct so tests can point them at fixture directories.
type hugepageDoctorRoots struct {
	sysfs   string // /sys/kernel/mm/hugepages
	cmdline string // /proc/cmdline
	meminfo string // /proc/meminfo
	cpuinfo string // /proc/cpuinfo
}

var defaultHugepageDoctorRoots = hugepageDoctorRoots{
	sysfs:   "/sys/kernel/mm/hugepages",
	cmdline: "/proc/cmdline",
	meminfo: "/proc/meminfo",
	cpuinfo: "/proc/cpuinfo",
}

var _ = env.MustRegister(env.EnvEntry{
	Key:         doctorHugepagesRootEnv,
	Type:        "string",
	Description: "Override procfs/sysfs root prefix for the VPP hugepage doctor check (functional tests)",
	Private:     true,
})

// vppHugepagesDoctorCheck is the doctor check registered from register_linux.go.
func vppHugepagesDoctorCheck() diagnostic.DoctorCheck {
	return diagnostic.DoctorCheck{
		Name:         "vpp-hugepages",
		Phase:        diagnostic.DoctorPhasePostConfig,
		Order:        820,
		Component:    "vpp",
		Dependencies: []string{"kernel"},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes:        []string{doctorVPPHugepagesCode},
		Check:        checkVPPHugepages,
	}
}

// hugepageParams are the VPP config values evaluated against the host, plus the
// roots to read and the target arch.
type hugepageParams struct {
	pageSize string // "2M" or "1G"
	mainHeap string // e.g. "1G"
	buffers  int64
	statseg  string // e.g. "512M"
	goarch   string
	roots    hugepageDoctorRoots
}

// checkVPPHugepages is the registered doctor check. It fires only when VPP is
// enabled and reports the host's boot-time hugepage reservation against what the
// VPP config needs.
func checkVPPHugepages(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	params, ok := hugepageParamsFromTree(tree)
	if !ok {
		return nil
	}
	params.goarch = runtime.GOARCH
	params.roots = hugepageRootsFromEnv()
	return evaluateVPPHugepages(params)
}

// hugepageRootsFromEnv returns the default host roots, or fixture roots under a
// prefix when doctorHugepagesRootEnv is set (functional-test hermeticity).
func hugepageRootsFromEnv() hugepageDoctorRoots {
	prefix := strings.TrimSpace(env.Get(doctorHugepagesRootEnv))
	if prefix == "" {
		return defaultHugepageDoctorRoots
	}
	return hugepageDoctorRoots{
		sysfs:   filepath.Join(prefix, "sys", "kernel", "mm", "hugepages"),
		cmdline: filepath.Join(prefix, "proc", "cmdline"),
		meminfo: filepath.Join(prefix, "proc", "meminfo"),
		cpuinfo: filepath.Join(prefix, "proc", "cpuinfo"),
	}
}

// hugepageParamsFromTree extracts the hugepage-relevant VPP settings from the
// config tree. ok is false when VPP is absent or disabled (the check is silent).
func hugepageParamsFromTree(tree *config.Tree) (hugepageParams, bool) {
	vpp := tree.GetContainer("vpp")
	if vpp == nil {
		return hugepageParams{}, false
	}
	if enabled, _ := vpp.Get("enabled"); enabled != "true" {
		return hugepageParams{}, false
	}
	p := hugepageParams{pageSize: "2M", mainHeap: "1G", buffers: 128000, statseg: "512M"}
	if mem := vpp.GetContainer("memory"); mem != nil {
		if v, ok := mem.Get("hugepage-size"); ok && v != "" {
			p.pageSize = v
		}
		if v, ok := mem.Get("main-heap"); ok && v != "" {
			p.mainHeap = v
		}
		if v, ok := mem.Get("buffers"); ok && v != "" {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				p.buffers = n
			}
		}
	}
	if stats := vpp.GetContainer("stats"); stats != nil {
		if v, ok := stats.Get("segment-size"); ok && v != "" {
			p.statseg = v
		}
	}
	return p, true
}

// evaluateVPPHugepages performs the host reads and returns diagnostics for
// AC-9..AC-12. It short-circuits to a single error when no pages are reserved.
func evaluateVPPHugepages(p hugepageParams) []diagnostic.Diagnostic {
	sizeKB, sizeBytes, ok := hugepageSizeKB(p.pageSize)
	if !ok {
		return nil // page-size is validated at config time; nothing to check here
	}
	var diags []diagnostic.Diagnostic
	var tb textbuf.Buffer

	nr, present := readNrHugepages(p.roots.sysfs, sizeKB)
	if !present || nr == 0 {
		return []diagnostic.Diagnostic{{
			Code:     doctorVPPHugepagesCode,
			Severity: diagnostic.SeverityError,
			Message: tb.Reset().Str("VPP hugepages: no ").Str(p.pageSize).
				Str(" hugepages reserved on this host; reserve them via image.hugepages or the boot cmdline").String(),
		}}
	}
	reservedBytes := nr * sizeBytes

	// AC-11: the boot cmdline requested more pages than the kernel reserved.
	if requested := parseCmdlineHugepages(p.roots.cmdline); requested > 0 {
		if total, okTotal := parseMeminfoHugePagesTotal(p.roots.meminfo); okTotal && total < requested {
			diags = append(diags, diagnostic.Diagnostic{
				Code:     doctorVPPHugepagesCode,
				Severity: diagnostic.SeverityWarning,
				Message: tb.Reset().Str("VPP hugepages: boot cmdline requested ").Int(requested).
					Str(" pages but only ").Int(total).Str(" are reserved (kernel clamped)").String(),
			})
		}
	}

	// AC-10: the reservation is smaller than VPP's estimated need.
	need := parseSizeBytes(p.mainHeap) + p.buffers*2048 + parseSizeBytes(p.statseg)
	if need > 0 && reservedBytes < need {
		diags = append(diags, diagnostic.Diagnostic{
			Code:     doctorVPPHugepagesCode,
			Severity: diagnostic.SeverityWarning,
			Message: tb.Reset().Str("VPP hugepages: reserved ").Int(reservedBytes).
				Str(" bytes is below the estimated VPP need of ").Int(need).
				Str(" bytes (main-heap + buffers*2048 + statseg)").String(),
		})
	}

	// AC-12: 1G pages configured on an amd64 CPU without pdpe1gb.
	if p.pageSize == "1G" && p.goarch == "amd64" && !cpuinfoHasFlag(p.roots.cpuinfo, "pdpe1gb") {
		diags = append(diags, diagnostic.Diagnostic{
			Code:     doctorVPPHugepagesCode,
			Severity: diagnostic.SeverityWarning,
			Message:  "VPP hugepages: 1G pages configured but this CPU lacks the pdpe1gb flag; 1G pages will not be reserved",
		})
	}

	return diags
}

func hugepageSizeKB(pageSize string) (sizeKB int, sizeBytes int64, ok bool) {
	switch pageSize {
	case "2M":
		return 2048, 2 * 1024 * 1024, true
	case "1G":
		return 1048576, 1024 * 1024 * 1024, true
	default:
		return 0, 0, false
	}
}

func readNrHugepages(sysfsRoot string, sizeKB int) (int64, bool) {
	var tb textbuf.Buffer
	dir := tb.Reset().Str("hugepages-").Int(int64(sizeKB)).Str("kB").String()
	data, err := os.ReadFile(filepath.Join(sysfsRoot, dir, "nr_hugepages")) //nolint:gosec // sysfs/fixture root
	if err != nil {
		return 0, false
	}
	n, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

// parseCmdlineHugepages returns the last hugepages=N value on the kernel cmdline
// (last wins, matching kernel semantics), or 0 when absent.
func parseCmdlineHugepages(path string) int64 {
	data, err := os.ReadFile(path) //nolint:gosec // procfs/fixture root
	if err != nil {
		return 0
	}
	var result int64
	for tok := range strings.FieldsSeq(string(data)) {
		if v, ok := strings.CutPrefix(tok, "hugepages="); ok {
			if n, perr := strconv.ParseInt(v, 10, 64); perr == nil {
				result = n
			}
		}
	}
	return result
}

func parseMeminfoHugePagesTotal(path string) (int64, bool) {
	data, err := os.ReadFile(path) //nolint:gosec // procfs/fixture root
	if err != nil {
		return 0, false
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if rest, ok := strings.CutPrefix(line, "HugePages_Total:"); ok {
			n, perr := strconv.ParseInt(strings.TrimSpace(rest), 10, 64)
			if perr != nil {
				return 0, false
			}
			return n, true
		}
	}
	return 0, false
}

// parseSizeBytes parses a VPP size string ("1G", "512M", "128K", or a bare byte
// count) into bytes, returning 0 when unparseable.
func parseSizeBytes(s string) int64 {
	if s == "" {
		return 0
	}
	mult := int64(1)
	numStr := s
	switch s[len(s)-1] {
	case 'G', 'g':
		mult = 1 << 30
		numStr = s[:len(s)-1]
	case 'M', 'm':
		mult = 1 << 20
		numStr = s[:len(s)-1]
	case 'K', 'k':
		mult = 1 << 10
		numStr = s[:len(s)-1]
	}
	n, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		return 0
	}
	return n * mult
}

func cpuinfoHasFlag(path, flag string) bool {
	data, err := os.ReadFile(path) //nolint:gosec // procfs/fixture root
	if err != nil {
		return false
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if !strings.HasPrefix(line, "flags") {
			continue
		}
		for f := range strings.FieldsSeq(line) {
			if f == flag {
				return true
			}
		}
	}
	return false
}
