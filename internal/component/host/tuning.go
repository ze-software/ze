// Design: docs/architecture/host/tuning.md -- runtime hardware tuning
// Overview: inventory.go — Detector reads current state; Tuner writes desired state

package host

// TuningConfig holds the desired hardware tuning state extracted from
// the YANG config tree. Zero-valued fields mean "do not touch".
type TuningConfig struct {
	CPUGovernor string
	IRQAffinity []IRQAffinityConfig
	Ethtool     []EthtoolConfig
}

// IRQAffinityConfig binds a NIC's IRQs to specific CPUs.
type IRQAffinityConfig struct {
	Interface string
	CPUs      string
}

// EthtoolConfig holds per-interface ethtool ring settings.
type EthtoolConfig struct {
	Interface string
	RingRx    int
	RingTx    int
}

// TuningResult reports what the tuning engine applied.
type TuningResult struct {
	Applied []string
	Errors  []TuningError
}

// The tuning operations. Each value names one operation in a TuningError and in
// the Applied line that reports the same operation when it succeeds.
const (
	tuningOpEthtoolRing = "ethtool-ring"
	tuningOpGovernor    = "governor"
	tuningOpIRQAffinity = "irq-affinity"
	tuningOpUnsupported = "tuning"
)

// TuningError records a single failed tuning operation.
type TuningError struct {
	Operation string
	Subject   string
	Err       error
}

// Error implements the error interface.
func (e TuningError) Error() string {
	return "tuning " + e.Operation + " " + e.Subject + ": " + e.Err.Error()
}

// Unwrap returns the underlying error.
func (e TuningError) Unwrap() error {
	return e.Err
}

// ApplyTuning applies the desired tuning configuration. It reads the
// current state first and only writes changed values (idempotent).
// Returns a result describing what was applied and any errors.
func ApplyTuning(cfg TuningConfig) TuningResult {
	return applyTuning(cfg)
}
