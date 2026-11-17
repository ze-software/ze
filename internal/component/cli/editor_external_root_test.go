// Design: docs/architecture/config/yang-config-design.md -- publishing the modules external plugins declare
// Related: editor.go -- newEditor, which parses with the schema of the process's module set
// Related: completer.go -- NewCompleter, which takes the same set

package cli

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
)

// externalRootConfig is a config that declares an external plugin and holds a
// section under the root that plugin's module defines.
const externalRootConfig = `plugin {
	external acme-monitor {
		run "./acme-monitor-program"
		encoder json
	}
}

acme-monitor {
	target probe-a
	interval 5
}
`

// TestEditorCompletesExternalRootFromThePublishedSet: in a process that
// published an external plugin's module, as a running daemon does at start,
// an editor opened on a config with a section under that plugin's root parses
// the section, and a completer offers the leaves the module defines. Method:
// build a completer before the publish as the control, publish, then open the
// editor and build a second completer.
//
// VALIDATES: AC-13 of spec-plugin-external-yang-at-startup, the editor half --
// the daemon's session editor parses and completes under an external root from
// the published set, with no read of the plugin.
// PREVENTS: an operator who can load a config under an external root and
// cannot edit it, because the editor answers from compiled-in modules alone.
func TestEditorCompletesExternalRootFromThePublishedSet(t *testing.T) {
	t.Cleanup(config.ResetDeclaredSchemasForTest())

	before := newTestCompleter(t)
	offeredBefore := fmt.Sprintf("%+v", before.Complete(config.NewTree(), "set acme-monitor ", nil))
	require.NotContains(t, offeredBefore, "interval", "the compiled-in schema already completes the external root")

	require.NoError(t, config.PublishExternalModules([]configyang.ExternalModule{{
		Plugin: "acme-monitor", Module: "acme-monitor-conf", Namespace: "urn:acme:monitor",
		Text: `module acme-monitor-conf {
    namespace "urn:acme:monitor";
    prefix acme;
    container acme-monitor {
        leaf target { type string; }
        leaf interval { type uint8 { range "1..10"; } }
    }
}`,
	}}))

	editor, err := NewEditorFromContent([]byte(externalRootConfig), "ze.conf")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, editor.Close()) })
	require.NotNil(t, editor.schema.Get("acme-monitor"), "the editor's schema misses the root of the published module")
	require.True(t, editor.treeValid, "the editor refused the section the published module defines")

	after := newTestCompleter(t)
	offered := fmt.Sprintf("%+v", after.Complete(editor.Tree(), "set acme-monitor ", nil))
	assert.Contains(t, offered, "target")
	assert.Contains(t, offered, "interval")
}
