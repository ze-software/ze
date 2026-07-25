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

// VALIDATES: verifyCoSConfig parses profiles and registers them.
// PREVENTS: broken wiring between InProcessConfigVerifier and cos registry.
func TestVerifyCoSConfig(t *testing.T) {
	t.Cleanup(coreCos.Clear)

	sections := []sdk.ConfigSection{
		{
			Root: "class-of-service",
			Data: `{"class-of-service":{"ieee-802.1p":{"test":{
				"ingress":{"pcp":{"0":{"priority":"1"}}},
				"egress":{"priority":{"1":{"pcp":"0"}}}
			}}}}`,
		},
	}

	err := verifyCoSConfig(sections)
	assert.NoError(t, err)

	p, ok := coreCos.Lookup("test")
	assert.True(t, ok)
	assert.Equal(t, map[uint32]uint32{0: 1}, p.IngressMap)
	assert.Equal(t, map[uint32]uint32{1: 0}, p.EgressMap)
}

// VALIDATES: verifyCoSConfig skips unrelated config roots.
// PREVENTS: false errors from non-cos config sections.
func TestVerifyCoSConfigSkipsOtherRoots(t *testing.T) {
	t.Cleanup(coreCos.Clear)

	sections := []sdk.ConfigSection{
		{Root: "bgp", Data: `{"bgp":{}}`},
	}
	err := verifyCoSConfig(sections)
	assert.NoError(t, err)
}

// VALIDATES: stale profiles are cleared when class-of-service section is removed.
// PREVENTS: config reload accepting a profile reference after the profile definition
// was deleted, because the registry still held entries from the previous verification.
func TestVerifyCoSConfigClearsStaleProfiles(t *testing.T) {
	t.Cleanup(coreCos.Clear)

	// First verification registers a profile.
	sections := []sdk.ConfigSection{
		{
			Root: "class-of-service",
			Data: `{"class-of-service":{"ieee-802.1p":{"stale":{
				"ingress":{"pcp":{"0":{"priority":"0"}}}
			}}}}`,
		},
	}
	err := verifyCoSConfig(sections)
	assert.NoError(t, err)
	_, ok := coreCos.Lookup("stale")
	assert.True(t, ok, "profile should exist after first verification")

	// Second verification with empty section (user removed profiles).
	sections = []sdk.ConfigSection{
		{Root: "class-of-service", Data: "{}"},
	}
	err = verifyCoSConfig(sections)
	assert.NoError(t, err)
	_, ok = coreCos.Lookup("stale")
	assert.False(t, ok, "stale profile must be cleared after re-verification")
}

// VALIDATES: showProfiles returns sorted profiles with sorted map entries.
// PREVENTS: show class-of-service returning empty or unsorted output.
func TestShowProfiles(t *testing.T) {
	t.Cleanup(coreCos.Clear)

	coreCos.Register("business", coreCos.Profile{
		IngressMap: map[uint32]uint32{5: 5, 7: 7},
		EgressMap:  map[uint32]uint32{5: 5},
	})
	coreCos.Register("residential", coreCos.Profile{
		IngressMap: map[uint32]uint32{0: 0, 6: 6},
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

// VALIDATES: stale profiles are cleared even when no class-of-service section is sent.
// PREVENTS: daemon reload leaving ghost profiles when the section is removed entirely.
func TestVerifyCoSConfigClearsOnNoSection(t *testing.T) {
	t.Cleanup(coreCos.Clear)

	coreCos.Register("ghost", coreCos.Profile{
		IngressMap: map[uint32]uint32{0: 0},
	})

	err := verifyCoSConfig(nil)
	assert.NoError(t, err)
	_, ok := coreCos.Lookup("ghost")
	assert.False(t, ok, "profiles must be cleared even with no sections")
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
