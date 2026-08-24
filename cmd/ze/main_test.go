//go:build ze_core

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// TestAvailablePlugins verifies the available plugin list is derived from the
// plugin registry.
//
// VALIDATES: Plugin discovery output uses the registry as its source of truth.
// PREVENTS: A second hardcoded production plugin list in cmd/ze.
func TestAvailablePlugins(t *testing.T) {
	got := plugin.AvailableInternalPlugins()
	require.NotEmpty(t, got, "AvailableInternalPlugins must return registered names")
	assert.Equal(t, registry.Names(), got)
}

// TestLooksLikeConfig removed with the looksLikeConfig helper it tested (spec-fixit-config-file-positional-grammar AC-2, Thomas-confirmed remove-the-sink). The free-form position-1 config-path sink was deleted from zeDispatch; config launch now goes through `ze start <config-file>`. AC-2/AC-6 are proven by test/ui/bare-config-no-autoload.ci.

// TestDetectConfigType and TestDetectConfigTypeFileError removed with the detectConfigType helper both tested. ProbeConfigType no longer selects a runtime -- every config the YANG schema accepts boots on one daemon path (cmd/ze/hub/main.go Run) -- so the helper and the --web config-type gate it fed both went. The probe's own behavior is covered by TestProbeConfigType (internal/component/config/probe_test.go), the boot path by test/plugin/config-validate-agrees-with-boot.ci, and the unreadable-config case by the "error: read config" branch the daemon now reaches for every config.

// TestIsLocalhostPprof moved to pprof_test.go, which carries pprof.go's own
// `!tinygo && ze_core` constraint. This file carries `ze_core` and not
// `!tinygo`, so under `-tags 'ze_core tinygo'` the test selected a function that
// build does not define.
