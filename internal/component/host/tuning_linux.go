// Design: docs/architecture/host/tuning.md -- runtime hardware tuning
// Overview: tuning.go — TuningConfig, TuningResult types
// Related: cpu_linux.go — reads scaling governor (read-side counterpart)

//go:build linux

package host

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	procIRQDir     = "/proc/irq"
	sysClassNetDir = "/sys/class/net"
)

func applyTuning(cfg TuningConfig) TuningResult {
	var r TuningResult
	if cfg.CPUGovernor != "" {
		applyGovernor(cfg.CPUGovernor, &r)
	}
	for _, irq := range cfg.IRQAffinity {
		applyIRQAffinity(irq, &r)
	}
	for _, eth := range cfg.Ethtool {
		applyEthtoolRing(eth, &r)
	}
	return r
}

func applyGovernor(governor string, r *TuningResult) {
	base := "/sys/devices/system/cpu"
	entries, err := os.ReadDir(base)
	if err != nil {
		r.Errors = append(r.Errors, TuningError{
			Operation: tuningOpGovernor,
			Subject:   "cpufreq",
			Err:       err,
		})
		return
	}

	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "cpu") {
			continue
		}
		rest := e.Name()[3:]
		if _, err := strconv.Atoi(rest); err != nil {
			continue
		}
		govPath := filepath.Join(base, e.Name(), "cpufreq", "scaling_governor")
		current, readErr := os.ReadFile(govPath) //nolint:gosec // sysfs path
		if readErr != nil {
			continue
		}
		if strings.TrimSpace(string(current)) == governor {
			continue
		}
		if writeErr := os.WriteFile(govPath, []byte(governor), 0o644); writeErr != nil { //nolint:gosec // sysfs node
			r.Errors = append(r.Errors, TuningError{
				Operation: tuningOpGovernor,
				Subject:   e.Name(),
				Err:       writeErr,
			})
			continue
		}
		verify, _ := os.ReadFile(govPath) //nolint:gosec // sysfs path
		if strings.TrimSpace(string(verify)) != governor {
			r.Errors = append(r.Errors, TuningError{
				Operation: tuningOpGovernor,
				Subject:   e.Name(),
				Err:       fmt.Errorf("write did not take effect: wanted %s, got %s", governor, strings.TrimSpace(string(verify))),
			})
			continue
		}
		r.Applied = append(r.Applied, tuningOpGovernor+" "+e.Name()+"="+governor)
	}
}

func applyIRQAffinity(cfg IRQAffinityConfig, r *TuningResult) {
	irqs, err := findInterfaceIRQs(cfg.Interface)
	if err != nil {
		r.Errors = append(r.Errors, TuningError{
			Operation: tuningOpIRQAffinity,
			Subject:   cfg.Interface,
			Err:       err,
		})
		return
	}
	if len(irqs) == 0 {
		r.Errors = append(r.Errors, TuningError{
			Operation: tuningOpIRQAffinity,
			Subject:   cfg.Interface,
			Err:       fmt.Errorf("no MSI IRQs found for %s", cfg.Interface),
		})
		return
	}

	applied := false
	for _, irqNum := range irqs {
		affinityPath := filepath.Join(procIRQDir, irqNum, "smp_affinity_list")
		current, readErr := os.ReadFile(affinityPath) //nolint:gosec // procfs path
		if readErr != nil {
			continue
		}
		if strings.TrimSpace(string(current)) == cfg.CPUs {
			continue
		}
		if writeErr := os.WriteFile(affinityPath, []byte(cfg.CPUs), 0o644); writeErr != nil { //nolint:gosec // procfs path
			r.Errors = append(r.Errors, TuningError{
				Operation: tuningOpIRQAffinity,
				Subject:   cfg.Interface + "/irq" + irqNum,
				Err:       writeErr,
			})
			continue
		}
		applied = true
	}
	if applied {
		r.Applied = append(r.Applied, tuningOpIRQAffinity+" "+cfg.Interface+" cpus="+cfg.CPUs)
	}
}

// findInterfaceIRQs returns the MSI IRQ numbers for a NIC by reading
// /sys/class/net/<iface>/device/msi_irqs/. Returns nil if the
// interface has no MSI IRQs (e.g. virtual interfaces).
func findInterfaceIRQs(iface string) ([]string, error) {
	msiDir := filepath.Join(sysClassNetDir, iface, "device", "msi_irqs")
	entries, err := os.ReadDir(msiDir)
	if err != nil {
		return nil, fmt.Errorf("read msi_irqs for %s: %w", iface, err)
	}
	irqs := make([]string, 0, len(entries))
	for _, e := range entries {
		if _, parseErr := strconv.Atoi(e.Name()); parseErr == nil {
			irqs = append(irqs, e.Name())
		}
	}
	return irqs, nil
}

func applyEthtoolRing(cfg EthtoolConfig, r *TuningResult) {
	if cfg.RingRx <= 0 && cfg.RingTx <= 0 {
		return
	}
	result, err := setEthtoolRing(cfg.Interface, cfg.RingRx, cfg.RingTx)
	if err != nil {
		r.Errors = append(r.Errors, TuningError{
			Operation: tuningOpEthtoolRing,
			Subject:   cfg.Interface,
			Err:       err,
		})
		return
	}
	if result != "" {
		r.Applied = append(r.Applied, result)
	}
}

func setEthtoolRing(iface string, rx, tx int) (string, error) {
	fd, err := openEthtoolSocket()
	if err != nil {
		return "", fmt.Errorf("ethtool socket: %w", err)
	}
	defer closeEthtoolSocket(fd)

	rp, err := getEthtoolRingParam(fd, iface)
	if err != nil {
		return "", fmt.Errorf("get ring params %s: %w", iface, err)
	}

	changed := false
	if rx > 0 && uint32(rx) != rp.rxPending { //nolint:gosec // bounded by YANG range
		rp.rxPending = uint32(rx) //nolint:gosec // bounded by YANG range
		changed = true
	}
	if tx > 0 && uint32(tx) != rp.txPending { //nolint:gosec // bounded by YANG range
		rp.txPending = uint32(tx) //nolint:gosec // bounded by YANG range
		changed = true
	}
	if !changed {
		return "", nil
	}

	if err := setEthtoolRingParam(fd, iface, rp); err != nil {
		return "", fmt.Errorf("set ring params %s: %w", iface, err)
	}
	var b textbuf.Buffer
	return b.Reset().Str(tuningOpEthtoolRing + " ").Str(iface).Str(" rx=").Int(int64(rp.rxPending)).Str(" tx=").Int(int64(rp.txPending)).String(), nil
}
