// Design: docs/architecture/ospf/ospf-4-component-config.md -- panic containment in the engine
// Related: opaque_registry.go -- safeOpaqueCall, a registered consumer's callback
// Related: ri_registry.go -- invokeRITLVBuilder, a registered Router Information TLV builder
// Related: ext_subtlv.go -- invokeExtSubTLVBuild and renderExtSubTLV, registered sub-TLV codecs
// Related: decode_view.go -- safeOpaqueDetailDecode, a show-command opaque decoder
// Related: decode_view_v3.go -- safeV3Decode, a show-command OSPFv3 LSA decoder

package ospf

import (
	"sync"

	"github.com/ze-software/ze/internal/core/taskguard"
)

// enginePanicTasks is the OSPF engine's panic boundary: one taskguard.Guard and
// the tasks it declares. A contained panic is logged at ERROR with its stack,
// counted in ze_task_panics_total{component="ospf",task} and crash-recorded,
// whatever the caller does next. The transport's interface tasks have their own
// guard (transport/panic_guard.go) with distinct task names.
type enginePanicTasks struct {
	guard *taskguard.Guard
	// consumer is one call into a callback another package registered with the
	// engine (an opaque consumer, a Router Information TLV builder, an Extended
	// Link or Prefix sub-TLV codec). Its panic drops that callback's
	// contribution to the one LSA or origination; the consumer's own error
	// counter rises, and the engine and every adjacency keep running (RFC 5250
	// framework isolation).
	consumer taskguard.Task
	// showDecode is one decode or render of a stored LSA body for a show
	// command. Its panic leaves that row undecoded; nothing else changes.
	showDecode taskguard.Task
}

const (
	consumerRestart   = "the registered consumer's contribution is dropped for this LSA or origination only, and it is called again on the next one"
	showDecodeRestart = "the show command answers that LSA undecoded, and the next show command decodes it again"
)

// enginePanics answers the engine's panic boundary, built on first use so the
// guard logs through the logger the plugin installed with setLogger.
var enginePanics = sync.OnceValue(newEnginePanicTasks)

func newEnginePanicTasks() *enginePanicTasks {
	guard := taskguard.New("ospf", logger())
	return &enginePanicTasks{
		guard:      guard,
		consumer:   guard.Task("consumer", consumerRestart),
		showDecode: guard.Task("show-decode", showDecodeRestart),
	}
}

// onContained adapts step to the guard's end function: the guard has already
// logged, counted and recorded the panic when step runs, so step only resets the
// caller's result or steps the caller's own error counter.
func onContained(step func()) func(*taskguard.Panic) {
	return func(*taskguard.Panic) { step() }
}

// unitContext names the unit a contained panic ended, for the guard's log line
// and crash record.
func unitContext(unit string) func() taskguard.Context {
	return func() taskguard.Context { return taskguard.Context{Unit: unit} }
}
