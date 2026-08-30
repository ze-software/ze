// Design: docs/architecture/traffic/traffic-usage.md -- traffic-usage plugin identity, logger, namespace

// Package trafficusage implements the `traffic-usage` system plugin: eBPF TCX
// per-(port,protocol) and (opt-in) per-IP byte accounting on operator-selected
// interfaces, exported as Prometheus metrics and viewable via `show traffic-usage`.
//
// The eBPF programs are assembled in pure Go (github.com/cilium/ebpf
// asm.Instructions) and loaded from memory: there is no C source, no committed
// object file, and no clang/LLVM build step. This deliberately diverges from
// the l2tp XDP-policing plugin, which commits bpf2go .o files. The cost is that
// the BPF logic is hand-written assembly with no compiler, so BPF_PROG_TEST_RUN
// per-path tests are load-bearing. See plan/spec-traffic-usage.md.
//
// The plugin never drops or modifies traffic; it only counts bytes. IPv4 only,
// matching the upstream lan-bandwidth-exporter it is ported from.
package trafficusage

import (
	"log/slog"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// Name is the registered plugin name (hyphen form). The Go package is trafficusage.
const Name = "traffic-usage"

// configRoot is the nested YANG config path this plugin owns (traffic/usage).
// The plugin augments the shared `traffic` container with `usage`, so the
// delivered section is wrapped as {"traffic":{"usage":{...}}}. This string must
// match the augment target in yang/ze-traffic-usage-conf.yang and the
// ConfigRoots/WantsConfig entries.
const configRoot = "traffic/usage"

// dependencyInterface is the plugin this one requires, labelInterface and
// labelProtocol are the metric labels its series carry, and keyInterface and
// keyProtocol are the response payload keys. They repeat two spellings across
// three surfaces: a rename of one must not rename the others.
const (
	dependencyInterface = "interface"
	labelInterface      = "interface"
	labelProtocol       = "protocol"
	keyInterface        = "interface"
	keyProtocol         = "protocol"
)

// loggerPtr holds the package logger, primed to a discard logger so calls made
// before ConfigureEngineLogger are safe.
var loggerPtr atomic.Pointer[slog.Logger]

func init() {
	loggerPtr.Store(slogutil.DiscardLogger())
}

func logger() *slog.Logger { return loggerPtr.Load() }

// setLogger installs the package logger (called from ConfigureEngineLogger). A
// nil logger is ignored so the primed discard logger is never cleared.
func setLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}
