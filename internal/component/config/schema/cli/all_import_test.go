package cli

import (
	// buildSchemaRegistry reads the bgp module from the plugin registry rather
	// than importing internal/component/bgp/yang (that import was an always-on
	// pin defeating //go:build ze_bgp), so the bgp PLUGIN must be registered for
	// these tests to see it. cmd/ze gets this through the generated
	// all_ze_bgp.go; plugin/all cannot be used here because it imports this very
	// package, which would be an import cycle in test.
	_ "github.com/ze-software/ze/internal/component/bgp/plugin"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/gr"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/hostname"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/rib/yang"
	_ "github.com/ze-software/ze/internal/core/ipc/yang"
)
