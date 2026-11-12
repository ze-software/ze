// Design: docs/architecture/api/process-protocol.md -- multi-process coordination and respawn
// Related: manager.go -- wireMetrics publishes a set, clearProcessCallbacks and Respawn take it back
// Related: delivery.go -- Deliver and the delivery loop call through the set
//
// The metrics callbacks a ProcessManager gives one process. The manager swaps
// them while the process runs (a restart of a running plugin clears them), and
// four goroutines read them, so they live behind one atomic pointer rather than
// in four plain fields each reader guarded with a different lock.

package process

import "github.com/ze-software/ze/internal/component/plugin"

// processMetricHooks is one set of metrics callbacks for a process. A set is
// never changed once published through Process.metricHooks: the manager
// publishes a new set or nil, and a reader loads the pointer once and calls
// through the set it loaded. A nil member is a callback this set does not
// carry, which only a test builds.
type processMetricHooks struct {
	stageChange    func(plugin.PluginStage) // after a stage transition
	delivered      func()                   // after a successful Deliver enqueue
	deliveryFailed func()                   // after a fire-and-forget batch failed
	deliverySlow   func()                   // after a batch took over DeliverySlowThreshold
}

// noteStageChange calls the stage-change metrics callback, when one is set.
// Safe for concurrent use.
func (p *Process) noteStageChange(stage plugin.PluginStage) {
	if hooks := p.metricHooks.Load(); hooks != nil && hooks.stageChange != nil {
		hooks.stageChange(stage)
	}
}

// noteDelivered calls the delivered metrics callback, when one is set. Safe
// for concurrent use.
func (p *Process) noteDelivered() {
	if hooks := p.metricHooks.Load(); hooks != nil && hooks.delivered != nil {
		hooks.delivered()
	}
}

// noteDeliveryFailed calls the failed-batch metrics callback, when one is
// set. Safe for concurrent use.
func (p *Process) noteDeliveryFailed() {
	if hooks := p.metricHooks.Load(); hooks != nil && hooks.deliveryFailed != nil {
		hooks.deliveryFailed()
	}
}

// noteDeliverySlow calls the slow-batch metrics callback, when one is set.
// Safe for concurrent use.
func (p *Process) noteDeliverySlow() {
	if hooks := p.metricHooks.Load(); hooks != nil && hooks.deliverySlow != nil {
		hooks.deliverySlow()
	}
}

// clearMetricHooks takes the metrics callbacks back, so a process the manager
// let go cannot re-create a deleted metric label through SetStage or Deliver.
// A reader that loaded the set before this call may still make one last call
// through it. Safe for concurrent use.
func (p *Process) clearMetricHooks() {
	p.metricHooks.Store(nil)
}
