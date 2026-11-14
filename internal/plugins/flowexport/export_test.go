// Design: docs/architecture/flowexport/flow-export-0-umbrella.md -- exporter lifecycle
// Related: rfc7011_withdrawal_test.go -- the external test that drives these
// Related: sflow_v5_agent_advice_test.go -- the external test that runs the engine
//
// The external test package links the encoder subpackages, which an internal
// test cannot import because each one imports flowexport to register itself.
// These wrappers hand that package the unexported exporter and the engine
// entry, compiled into the test binary only.

package flowexport

import (
	"log/slog"
	"net"
)

// RunEngine runs the plugin's engine over conn, the entry the registry
// starts the internal plugin through.
func RunEngine(conn net.Conn) int { return runEngine(conn) }

// SwapLogger installs the plugin logger and answers the one it replaced, so
// a test reads what runEngine logs and restores the logger after it.
func SwapLogger(l *slog.Logger) *slog.Logger { return loggerPtr.Swap(l) }

// Exporter is the unexported exporter, named for the external test package.
type Exporter = exporter

// NewWiredExporter builds an exporter and assigns it the registered encoders,
// the two steps configure performs before it swaps the exporter in.
func NewWiredExporter(cfg *Config) (*Exporter, error) {
	exp, err := newExporter(cfg)
	if err != nil {
		return nil, err
	}
	wireEncoders(exp, cfg)
	return exp, nil
}

// NotifySnapshot feeds one counter snapshot, as the rate tracker does.
func (e *exporter) NotifySnapshot(snap CounterSnapshot) { e.notifySnapshot(snap) }

// ExportFlows feeds one batch of flow records, as the conntrack worker does.
func (e *exporter) ExportFlows(flows []ConntrackFlow) { e.exportFlows(flows) }

// Stop tears the exporter down, as configure does for the exporter it replaces.
func (e *exporter) Stop() { e.stop() }
