//go:build linux

package vpp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// TestDoctorVPPHugepages verifies the four boot-reservation conditions (AC-9
// error on missing/zero pages, AC-10 warning on insufficiency, AC-11 warning on
// kernel clamp, AC-12 warning on 1G without pdpe1gb) and that the check is
// silent when VPP is absent or disabled.
func TestDoctorVPPHugepages(t *testing.T) {
	newRoots := func(t *testing.T) hugepageDoctorRoots {
		t.Helper()
		dir := t.TempDir()
		return hugepageDoctorRoots{
			sysfs:   filepath.Join(dir, "sysfs"),
			cmdline: filepath.Join(dir, "cmdline"),
			meminfo: filepath.Join(dir, "meminfo"),
			cpuinfo: filepath.Join(dir, "cpuinfo"),
		}
	}
	writeNr := func(t *testing.T, roots hugepageDoctorRoots, dirName, nr string) {
		t.Helper()
		d := filepath.Join(roots.sysfs, dirName)
		if err := os.MkdirAll(d, 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "nr_hugepages"), []byte(nr), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeFile := func(t *testing.T, path, content string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	base := func(roots hugepageDoctorRoots, pageSize string) hugepageParams {
		return hugepageParams{
			pageSize: pageSize, mainHeap: "1G", buffers: 128000, statseg: "512M",
			goarch: "amd64", roots: roots,
		}
	}

	t.Run("missing reservation is an error (AC-9)", func(t *testing.T) {
		roots := newRoots(t) // sysfs dir does not exist
		diags := evaluateVPPHugepages(base(roots, "2M"))
		if !hasSeverity(diags, diagnostic.SeverityError) {
			t.Errorf("expected error diagnostic, got %v", diags)
		}
	})

	t.Run("zero reserved is an error (AC-9)", func(t *testing.T) {
		roots := newRoots(t)
		writeNr(t, roots, "hugepages-2048kB", "0")
		diags := evaluateVPPHugepages(base(roots, "2M"))
		if !hasSeverity(diags, diagnostic.SeverityError) {
			t.Errorf("expected error diagnostic, got %v", diags)
		}
	})

	t.Run("sufficient reservation is silent", func(t *testing.T) {
		roots := newRoots(t)
		writeNr(t, roots, "hugepages-2048kB", "1000") // 1000*2MiB ~ 2GiB > need
		diags := evaluateVPPHugepages(base(roots, "2M"))
		if len(diags) != 0 {
			t.Errorf("expected no diagnostics, got %v", diags)
		}
	})

	t.Run("insufficient reservation warns (AC-10)", func(t *testing.T) {
		roots := newRoots(t)
		writeNr(t, roots, "hugepages-2048kB", "10") // 20MiB << need
		diags := evaluateVPPHugepages(base(roots, "2M"))
		if !warnContains(diags, "below the estimated") {
			t.Errorf("expected insufficiency warning, got %v", diags)
		}
	})

	t.Run("kernel clamp warns (AC-11)", func(t *testing.T) {
		roots := newRoots(t)
		writeNr(t, roots, "hugepages-2048kB", "1000") // enough to clear AC-9/AC-10
		writeFile(t, roots.cmdline, "console=ttyS0 default_hugepagesz=2M hugepagesz=2M hugepages=2000 loglevel=8")
		writeFile(t, roots.meminfo, "MemTotal: 8192 kB\nHugePages_Total:    1000\nHugePages_Free:    1000\n")
		diags := evaluateVPPHugepages(base(roots, "2M"))
		if !warnContains(diags, "clamped") {
			t.Errorf("expected clamp warning, got %v", diags)
		}
	})

	t.Run("1G without pdpe1gb warns (AC-12)", func(t *testing.T) {
		roots := newRoots(t)
		writeNr(t, roots, "hugepages-1048576kB", "10") // 10 GiB, clears AC-9/AC-10
		writeFile(t, roots.cpuinfo, "processor\t: 0\nflags\t: fpu vme de pse tsc msr\n")
		diags := evaluateVPPHugepages(base(roots, "1G"))
		if !warnContains(diags, "pdpe1gb") {
			t.Errorf("expected pdpe1gb warning, got %v", diags)
		}
	})

	t.Run("1G with pdpe1gb does not warn about the flag", func(t *testing.T) {
		roots := newRoots(t)
		writeNr(t, roots, "hugepages-1048576kB", "10")
		writeFile(t, roots.cpuinfo, "processor\t: 0\nflags\t: fpu vme pdpe1gb rdtscp lm\n")
		diags := evaluateVPPHugepages(base(roots, "1G"))
		if warnContains(diags, "pdpe1gb") {
			t.Errorf("did not expect a pdpe1gb warning, got %v", diags)
		}
	})

	t.Run("absent vpp is silent", func(t *testing.T) {
		diags := checkVPPHugepages(diagnostic.DoctorCheckContext{Tree: config.NewTree()})
		if diags != nil {
			t.Errorf("expected nil for absent vpp, got %v", diags)
		}
	})

	t.Run("disabled vpp is silent", func(t *testing.T) {
		tree := config.NewTree()
		vpp := config.NewTree()
		vpp.Set("enabled", "false")
		tree.SetContainer("vpp", vpp)
		diags := checkVPPHugepages(diagnostic.DoctorCheckContext{Tree: tree})
		if diags != nil {
			t.Errorf("expected nil for disabled vpp, got %v", diags)
		}
	})
}

func hasSeverity(diags []diagnostic.Diagnostic, sev diagnostic.Severity) bool {
	for i := range diags {
		if diags[i].Severity == sev {
			return true
		}
	}
	return false
}

func warnContains(diags []diagnostic.Diagnostic, substr string) bool {
	for i := range diags {
		if diags[i].Severity == diagnostic.SeverityWarning && strings.Contains(diags[i].Message, substr) {
			return true
		}
	}
	return false
}
