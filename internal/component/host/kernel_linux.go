// Design: plan/spec-host-0-inventory.md — hardware inventory detection

//go:build linux

package host

import (
	"bufio"
	"os"
	"runtime"
	"slices"
	"sort"
	"strings"
	"time"
)

// archSecurityFlags are the CPU flags that reflect kernel-relevant
// security posture. The set is intentionally small; operators care
// about "is IBT on" more than the full /proc/cpuinfo flags blob.
var archSecurityFlags = []string{
	"smep", "smap", "ibt", "user_shstk", "ibrs", "ibrs_enhanced", "ssbd",
}

// DetectKernel aggregates /proc/version, /proc/cmdline, boot time
// (via /proc/stat btime), microcode revision (via cpuinfo), and a
// filtered list of security-relevant CPU flags.
func (d *Detector) DetectKernel() (*KernelInfo, error) {
	info := &KernelInfo{
		Architecture: runtime.GOARCH,
	}
	info.Release = readFileString(d.procPath("sys/kernel/osrelease"))
	info.Version = readFileString(d.procPath("version"))
	info.Cmdline = readFileString(d.procPath("cmdline"))
	info.MicrocodeRevision = readMicrocodeRevision(d.procPath("cpuinfo"))

	if btime, ok := readBootTime(d.procPath("stat")); ok {
		info.BootTimeUnix = btime
		info.BootTime = time.Unix(btime, 0).UTC().Format(time.RFC3339)
	}

	info.ArchFlags = filterSecurityFlags(readCPUFlags(d.procPath("cpuinfo")))
	return info, nil
}

// DetectHost reports hostname, uptime, and timezone of the running
// process. Uptime comes from /proc/uptime (first field).
func (d *Detector) DetectHost() (*HostInfo, error) {
	info := &HostInfo{}
	if host, err := os.Hostname(); err == nil {
		info.Hostname = host
	}
	info.UptimeSeconds = readUptimeSeconds(d.procPath("uptime"))
	if zone, _ := time.Now().Zone(); zone != "" {
		info.Timezone = zone
	}
	return info, nil
}

// readBootTime extracts `btime <unix>` from /proc/stat. Returns ok=false
// when absent.
func readBootTime(path string) (int64, bool) {
	f, err := os.Open(path)
	if err != nil {
		return 0, false
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "btime ") {
			continue
		}
		v, perr := parseInt64(strings.TrimPrefix(line, "btime "))
		if perr {
			return 0, false
		}
		return v, true
	}
	return 0, false
}

// parseInt64 returns (value, err-true-on-failure). Small local helper
// so we don't drag strconv into callers that only want "is the first
// token a number".
func parseInt64(s string) (int64, bool) {
	var v int64
	for _, c := range strings.TrimSpace(s) {
		if c < '0' || c > '9' {
			return 0, true
		}
		v = v*10 + int64(c-'0')
	}
	return v, false
}

// readMicrocodeRevision returns the first `microcode` value found in
// /proc/cpuinfo. All cores report the same revision on uniform CPUs;
// on hybrid parts the first is the boot CPU which is representative.
func readMicrocodeRevision(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		k, v, ok := strings.Cut(scanner.Text(), ":")
		if !ok {
			continue
		}
		if strings.TrimSpace(k) == "microcode" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// readCPUFlags returns the first `flags` value found in /proc/cpuinfo
// split on whitespace. All cores share the same flags list on uniform
// CPUs; the boot CPU's flags are representative.
func readCPUFlags(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		k, v, ok := strings.Cut(scanner.Text(), ":")
		if !ok {
			continue
		}
		if strings.TrimSpace(k) == "flags" {
			return strings.Fields(strings.TrimSpace(v))
		}
	}
	return nil
}

// filterSecurityFlags keeps only the archSecurityFlags that are
// actually present, sorted for stable JSON output.
func filterSecurityFlags(all []string) []string {
	var out []string
	for _, f := range archSecurityFlags {
		if slices.Contains(all, f) {
			out = append(out, f)
		}
	}
	sort.Strings(out)
	return out
}

// readUptimeSeconds reads /proc/uptime's first field (fractional
// seconds) and truncates to integer. Returns 0 on error.
func readUptimeSeconds(path string) uint64 {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	f := strings.Fields(string(b))
	if len(f) == 0 {
		return 0
	}
	sec, _, _ := strings.Cut(f[0], ".")
	v, perr := parseInt64(sec)
	if perr || v < 0 {
		return 0
	}
	return uint64(v) //nolint:gosec // parseInt64 rejects negative values on the previous line
}
