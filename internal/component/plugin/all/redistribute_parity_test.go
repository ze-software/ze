// VALIDATES: every registered redistribute PRODUCER protocol also has a
// registered config SOURCE (so `import <name>` resolves), and the sources for
// plugins that historically registered only when their engine ran
// (connected/kernel/l2tp/static) are now registered at init() — visible to
// `ze config validate`, which imports plugins but never starts engines. This
// test runs in the all package, which blank-imports the full plugin set, so the
// registries are complete WITHOUT any engine running.
// PREVENTS: a producer shipped without its config source (the static-source
// bug), and regression to run-time-only source registration that `ze config
// validate` cannot see.
package all

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/redistevents"
)

func TestEveryRedistributeProducerHasSource(t *testing.T) {
	producers := redistevents.Producers()
	require.NotEmpty(t, producers, "expected redistribute producers registered via all.go imports")

	// protocol -> has a registered config source
	hasSource := map[string]bool{}
	for _, name := range configredist.SourceNames() {
		if src, ok := configredist.LookupSource(name); ok {
			hasSource[src.Protocol] = true
		}
	}

	for _, id := range producers {
		proto := redistevents.ProtocolName(id)
		require.NotEmptyf(t, proto, "producer id %d has no protocol name", id)
		assert.Truef(t, hasSource[proto],
			"redistribute producer %q has no registered config source; `import %s` would be rejected", proto, proto)
	}
}

func TestRunTimePluginsRegisterSourceAtInit(t *testing.T) {
	// These plugins historically registered their source only when their engine
	// ran (runConnectedPlugin / runKernelPlugin / l2tp Subsystem.Start), leaving
	// them invisible to `ze config validate`. They must now register at init();
	// no engine is started in this test, so import alone must suffice.
	for _, name := range []string{"connected", "kernel", "l2tp", "static"} {
		_, ok := configredist.LookupSource(name)
		assert.Truef(t, ok, "redistribute source %q not registered at init; `import %s` would fail config validate", name, name)
	}
}
