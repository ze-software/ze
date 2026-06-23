// VALIDATES: spec-ospf-13 AC-1/AC-10 -- the `show ip ospf` process summary reports
// router-id, ABR status (multi-area incl. backbone), ASBR status (redistribution), the
// area list, and the active stub-router (max-metric) state.
// PREVENTS: a summary that misreports ABR/ASBR or omits the stub-router reflection.
package ospf

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEngineProcessSummary(t *testing.T) {
	eng, _ := newRedistEngine(t, `{"ospf":{"router-id":"10.0.0.1","max-metric":{"router-lsa":{"always":"true"}},"redistribute":{"connected":{"source":"connected"}},"areas":{"area":{"0.0.0.0":{"area-id":"0.0.0.0"},"0.0.0.1":{"area-id":"0.0.0.1"}}},"interfaces":{"interface":{"eth0":{"area":"0.0.0.0"},"eth1":{"area":"0.0.0.1"}}}}}`)

	s := eng.processSummary()
	assert.Equal(t, "10.0.0.1", s.RouterID)
	assert.True(t, s.ABR, "interfaces in the backbone plus a second area -> ABR")
	assert.True(t, s.ASBR, "redistribution configured -> ASBR")
	assert.Equal(t, 2, s.AreaCount)
	assert.True(t, s.StubRouter.Always, "max-metric router-lsa always reflected")
	assert.True(t, s.StubRouter.Active, "always -> stub-router active")
}

func TestEngineProcessSummaryPlain(t *testing.T) {
	// Single backbone area, no redistribution, no max-metric -> not ABR/ASBR, stub inactive.
	eng, _ := newRedistEngine(t, `{"ospf":{"router-id":"10.0.0.2","areas":{"area":{"0.0.0.0":{"area-id":"0.0.0.0"}}},"interfaces":{"interface":{"eth0":{"area":"0.0.0.0"}}}}}`)
	s := eng.processSummary()
	assert.False(t, s.ABR)
	assert.False(t, s.ASBR)
	assert.False(t, s.StubRouter.Active)
	assert.Equal(t, 1, s.AreaCount)
}
