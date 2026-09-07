//go:build ze_core && ze_vrrp

// VALIDATES: AC-3 for a plugin command the YANG model DOES describe. Such a
// command carries no wire method, so it reaches the catalog through
// registry.Registration.Commands like any other plugin command, and it MUST
// keep the model's authored long help and a usage line that is not empty.
// PREVENTS: two regressions the first version of this enumeration had. It
// published the plugin's one-line summary over the model's authored help, and
// it set the usage line from command.Usage, which answers nil for a node with
// no wire method and so replaced a good invocation form with "".
//
// Tagged ze_vrrp because `show vrrp interface` is behind that gate. The normal
// unit run carries every feature tag (internal/le/gotoolchain,
// Toolchain.TestTags).

package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHelpCommandKeepsTheModelHelpOfAPluginCommand(t *testing.T) {
	var out bytes.Buffer
	require.Equal(t, 0, renderHelpCommand(&out, []string{flagJSON}))

	var entries []commandEntry
	require.NoError(t, json.Unmarshal(out.Bytes(), &entries))

	const path = "show vrrp interface"
	for _, entry := range entries {
		if entry.Path != path {
			continue
		}
		assert.NotEmpty(t, entry.LongHelp, "%q lost the long help its node declares", path)
		assert.Equal(t, path, entry.Usage, "%q published an empty usage line", path)
		return
	}
	t.Fatalf("the catalog names no %q", path)
}
