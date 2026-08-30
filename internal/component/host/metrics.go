// Design: docs/architecture/host/inventory.md -- Prometheus export of inventory
// Related: inventory.go -- Inventory struct and Detect() entry point
// Related: cached.go -- CachedDetector wraps Detect with TTL
// Related: diff.go -- DiffInventory compares snapshots for change events

package host

import (
	"time"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/report"
)

// labelName is the Prometheus label that carries the name of the NIC, block
// device, or thermal sensor a sample belongs to.
const labelName = "name"

// HostMetrics holds the Prometheus gauges and gauge-vecs for host
// inventory data. Create one via RegisterMetrics, then call
// CollectOnce to snapshot the current hardware state into the gauges.
type HostMetrics struct {
	memTotal     metrics.Gauge
	memAvailable metrics.Gauge
	cpuLogical   metrics.Gauge
	cpuPhysical  metrics.Gauge
	uptime       metrics.Gauge
	eccCorr      metrics.Gauge
	eccUncorr    metrics.Gauge

	nicSpeed   metrics.GaugeVec
	nicCarrier metrics.GaugeVec

	storageSize metrics.GaugeVec

	thermalTemp metrics.GaugeVec

	cached *CachedDetector
	prev   *Inventory
	stopCh chan struct{}
}

// RegisterMetrics creates and registers all host inventory gauges on
// the given registry. Uses the provided CachedDetector for efficient
// inventory access and emits hardware-change events to the report bus.
func RegisterMetrics(reg metrics.Registry, cd *CachedDetector) *HostMetrics {
	return &HostMetrics{
		memTotal:     reg.Gauge("ze_host_memory_total_bytes", "Total physical memory in bytes"),
		memAvailable: reg.Gauge("ze_host_memory_available_bytes", "Available physical memory in bytes"),
		cpuLogical:   reg.Gauge("ze_host_cpu_logical_count", "Number of logical CPUs"),
		cpuPhysical:  reg.Gauge("ze_host_cpu_physical_cores", "Number of physical CPU cores"),
		uptime:       reg.Gauge("ze_host_uptime_seconds", "Host uptime in seconds"),
		eccCorr:      reg.Gauge("ze_host_ecc_correctable_errors_total", "ECC correctable error count (gauge of external counter)"),
		eccUncorr:    reg.Gauge("ze_host_ecc_uncorrectable_errors_total", "ECC uncorrectable error count (gauge of external counter)"),

		nicSpeed:   reg.GaugeVec("ze_host_nic_link_speed_mbps", "NIC link speed in Mbps", []string{labelName}),
		nicCarrier: reg.GaugeVec("ze_host_nic_carrier", "NIC carrier state (1=up, 0=down)", []string{labelName}),

		storageSize: reg.GaugeVec("ze_host_storage_size_bytes", "Block device size in bytes", []string{labelName}),

		thermalTemp: reg.GaugeVec("ze_host_thermal_temp_mc", "Thermal sensor reading in millicelsius", []string{labelName, "device"}),

		cached: cd,
	}
}

// CollectOnce runs inventory detection via the CachedDetector, sets
// all gauge values, and emits report bus events for any hardware
// changes since the last collection.
func (m *HostMetrics) CollectOnce() {
	inv, err := m.cached.Detect()
	if err != nil {
		return
	}
	m.emitDiffEvents(m.prev, inv)
	m.prev = inv
	m.collectFrom(inv)
}

// StartRefresh launches a background goroutine that calls CollectOnce
// at the given interval. Call Stop to terminate the goroutine.
func (m *HostMetrics) StartRefresh(interval time.Duration) {
	m.stopCh = make(chan struct{})
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				m.CollectOnce()
			case <-m.stopCh:
				return
			}
		}
	}()
}

// Stop terminates the background refresh goroutine. Safe to call
// without StartRefresh (no-op if stopCh is nil).
func (m *HostMetrics) Stop() {
	if m.stopCh != nil {
		close(m.stopCh)
	}
}

func (m *HostMetrics) emitDiffEvents(prev, curr *Inventory) {
	events := DiffInventory(prev, curr)
	for _, e := range events {
		switch e.Code {
		case "ecc-error":
			report.RaiseError("host", e.Code, e.Subject, e.Message, e.Detail)
		default:
			report.RaiseWarning("host", e.Code, e.Subject, e.Message, e.Detail)
		}
	}
}

// collectFrom populates gauges from a pre-built Inventory. Separated
// from CollectOnce so tests can inject a synthetic Inventory without
// requiring a real sysfs tree.
func (m *HostMetrics) collectFrom(inv *Inventory) {
	if inv == nil {
		return
	}

	if inv.Memory != nil {
		m.memTotal.Set(float64(inv.Memory.TotalBytes))
		m.memAvailable.Set(float64(inv.Memory.AvailableBytes))
		m.eccCorr.Set(float64(inv.Memory.ECCCorrectableErrors))
		m.eccUncorr.Set(float64(inv.Memory.ECCUncorrectableErrors))
	}

	if inv.CPU != nil {
		m.cpuLogical.Set(float64(inv.CPU.LogicalCPUs))
		m.cpuPhysical.Set(float64(inv.CPU.PhysicalCores))
	}

	if inv.Host != nil {
		m.uptime.Set(float64(inv.Host.UptimeSeconds))
	}

	for i := range inv.NICs {
		nic := &inv.NICs[i]
		m.nicSpeed.With(nic.Name).Set(float64(nic.LinkSpeedMbps))
		carrier := 0.0
		if nic.Carrier {
			carrier = 1.0
		}
		m.nicCarrier.With(nic.Name).Set(carrier)
	}

	if inv.Storage != nil {
		for _, dev := range inv.Storage.Devices {
			m.storageSize.With(dev.Name).Set(float64(dev.SizeBytes))
		}
	}

	if inv.Thermal != nil {
		for _, s := range inv.Thermal.Sensors {
			m.thermalTemp.With(s.Name, s.Device).Set(float64(s.TempMC))
		}
	}
}
