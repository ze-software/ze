package cos

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"

	coreCos "github.com/ze-software/ze/internal/core/cos"
	"github.com/ze-software/ze/internal/core/slogutil"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// VALIDATES: verifyCoSConfig skips unrelated config roots.
// PREVENTS: false errors from non-cos config sections.
func TestVerifyCoSConfigSkipsOtherRoots(t *testing.T) {
	sections := []sdk.ConfigSection{
		{Root: "bgp", Data: `{"bgp":{}}`},
	}
	err := verifyCoSConfig(sections)
	assert.NoError(t, err)
}

// VALIDATES: showProfiles returns sorted profiles with sorted map entries.
// PREVENTS: show class-of-service returning empty or unsorted output.
func TestShowProfiles(t *testing.T) {
	t.Cleanup(func() { coreCos.Replace(nil) })

	coreCos.Replace(map[string]coreCos.Profile{
		"business": {
			IngressMap: map[uint32]uint32{5: 5, 7: 7},
			EgressMap:  map[uint32]uint32{5: 5},
		},
		"residential": {
			IngressMap: map[uint32]uint32{0: 0, 6: 6},
		},
	})

	result := showProfiles()
	data, err := json.Marshal(result)
	assert.NoError(t, err)
	var parsed []map[string]any
	assert.NoError(t, json.Unmarshal(data, &parsed))
	assert.Len(t, parsed, 2)
	assert.Equal(t, "business", parsed[0]["name"])
	assert.Equal(t, "residential", parsed[1]["name"])
}

// VALIDATES: warnIfExternal logs a warning exactly when cos is not running
// in-process -- ConfigureEventBus (the only place dynamicHandler is set) is
// wired through plugin.GetInternalPluginRunner and never invoked for an
// external plugin, so an external cos silently never pushes EventBus-
// triggered VLAN QoS map updates with no error anywhere until this warning.
// PREVENTS: an operator running `plugin { external cos { ... } }` getting no
// indication that dynamic QoS updates are disabled (fork round-2 finding 4).
func TestWarnIfExternal(t *testing.T) {
	t.Cleanup(func() { setLogger(slogutil.DiscardLogger()) })

	var buf bytes.Buffer
	setLogger(slog.New(slog.NewTextHandler(&buf, nil)))
	warnIfExternal(false)
	assert.Contains(t, buf.String(), "dynamic per-interface QoS map updates")

	buf.Reset()
	setLogger(slog.New(slog.NewTextHandler(&buf, nil)))
	warnIfExternal(true)
	assert.Empty(t, buf.String(), "internal cos must not log the external-mode warning")
}
