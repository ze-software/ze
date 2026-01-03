package migration

import (
	"testing"

	"github.com/exa-networks/zebgp/pkg/config"
	"github.com/stretchr/testify/require"
)

// TestMigrateAPIBlocksSingleProcess verifies single process migration.
//
// VALIDATES: api { processes [ foo ]; } → api foo { }
//
// PREVENTS: Lost process bindings during migration.
func TestMigrateAPIBlocksSingleProcess(t *testing.T) {
	input := `
process foo { run ./test; encoder text; }
peer 10.0.0.1 {
    router-id 1.2.3.4;
    local-as 65001;
    peer-as 65002;
    api {
        processes [ foo ];
    }
}
`
	tree := parseWithLegacySchema(t, input)
	migrated, err := MigrateAPIBlocks(tree)
	require.NoError(t, err)

	// Check that old anonymous api block is removed
	peer := migrated.GetList("peer")["10.0.0.1"]
	require.NotNil(t, peer)

	apiList := peer.GetList("api")
	require.NotContains(t, apiList, config.KeyDefault, "old anonymous block should be removed")
	require.Contains(t, apiList, "foo", "new named block should exist")
}

// TestMigrateAPIBlocksMultipleProcesses verifies multiple process migration.
//
// VALIDATES: api { processes [ foo bar ]; } → api foo { }; api bar { }
//
// PREVENTS: Lost processes when multiple are specified.
func TestMigrateAPIBlocksMultipleProcesses(t *testing.T) {
	input := `
process foo { run ./foo; }
process bar { run ./bar; }
peer 10.0.0.1 {
    router-id 1.2.3.4;
    local-as 65001;
    peer-as 65002;
    api {
        processes [ foo bar ];
    }
}
`
	tree := parseWithLegacySchema(t, input)
	migrated, err := MigrateAPIBlocks(tree)
	require.NoError(t, err)

	peer := migrated.GetList("peer")["10.0.0.1"]
	apiList := peer.GetList("api")

	require.NotContains(t, apiList, config.KeyDefault)
	require.Contains(t, apiList, "foo", "foo binding should exist")
	require.Contains(t, apiList, "bar", "bar binding should exist")
}

// TestMigrateAPIBlocksWithNeighborChanges verifies neighbor-changes flag migration.
//
// VALIDATES: neighbor-changes; → receive { state; }
//
// PREVENTS: Lost state change subscriptions.
func TestMigrateAPIBlocksWithNeighborChanges(t *testing.T) {
	input := `
process watcher { run ./watcher; }
peer 10.0.0.1 {
    router-id 1.2.3.4;
    local-as 65001;
    peer-as 65002;
    api {
        processes [ watcher ];
        neighbor-changes;
    }
}
`
	tree := parseWithLegacySchema(t, input)
	migrated, err := MigrateAPIBlocks(tree)
	require.NoError(t, err)

	peer := migrated.GetList("peer")["10.0.0.1"]
	apiList := peer.GetList("api")
	watcherAPI := apiList["watcher"]
	require.NotNil(t, watcherAPI)

	// Check receive { state; } was created
	receive := watcherAPI.GetContainer("receive")
	require.NotNil(t, receive, "receive block should exist")

	stateVal, ok := receive.Get("state")
	require.True(t, ok, "state flag should exist")
	require.Equal(t, "true", stateVal)
}

// TestMigrateAPIBlocksWithoutNeighborChanges verifies no receive block when not needed.
//
// VALIDATES: No neighbor-changes → no receive block.
//
// PREVENTS: Spurious receive blocks.
func TestMigrateAPIBlocksWithoutNeighborChanges(t *testing.T) {
	input := `
process foo { run ./test; }
peer 10.0.0.1 {
    router-id 1.2.3.4;
    local-as 65001;
    peer-as 65002;
    api {
        processes [ foo ];
    }
}
`
	tree := parseWithLegacySchema(t, input)
	migrated, err := MigrateAPIBlocks(tree)
	require.NoError(t, err)

	peer := migrated.GetList("peer")["10.0.0.1"]
	fooAPI := peer.GetList("api")["foo"]
	require.NotNil(t, fooAPI)

	// No neighbor-changes → no receive block
	receive := fooAPI.GetContainer("receive")
	require.Nil(t, receive, "receive block should not exist without neighbor-changes")
}

// TestMigrateAPIBlocksPreservesNewSyntax verifies new syntax is not modified.
//
// VALIDATES: api foo { receive { update; } } is preserved.
//
// PREVENTS: Corrupting already-migrated configs.
func TestMigrateAPIBlocksPreservesNewSyntax(t *testing.T) {
	input := `
process foo { run ./test; }
peer 10.0.0.1 {
    router-id 1.2.3.4;
    local-as 65001;
    peer-as 65002;
    api foo {
        receive { update; }
    }
}
`
	tree := parseWithLegacySchema(t, input)
	migrated, err := MigrateAPIBlocks(tree)
	require.NoError(t, err)

	peer := migrated.GetList("peer")["10.0.0.1"]
	apiList := peer.GetList("api")

	require.NotContains(t, apiList, config.KeyDefault, "no anonymous block")
	require.Contains(t, apiList, "foo", "foo binding preserved")

	// Verify receive block preserved
	fooAPI := apiList["foo"]
	receive := fooAPI.GetContainer("receive")
	require.NotNil(t, receive)
	_, ok := receive.Get("update")
	require.True(t, ok, "update flag should be preserved")
}

// TestMigrateAPIBlocksInTemplate verifies migration in template blocks.
//
// VALIDATES: api blocks in template.group are migrated.
//
// PREVENTS: Template api blocks being ignored.
func TestMigrateAPIBlocksInTemplate(t *testing.T) {
	input := `
process collector { run ./collector; }
template {
    group default {
        api {
            processes [ collector ];
            neighbor-changes;
        }
    }
}
`
	tree := parseWithLegacySchema(t, input)
	migrated, err := MigrateAPIBlocks(tree)
	require.NoError(t, err)

	tmpl := migrated.GetContainer("template")
	require.NotNil(t, tmpl)

	defaultGroup := tmpl.GetList("group")["default"]
	require.NotNil(t, defaultGroup)

	apiList := defaultGroup.GetList("api")
	require.NotContains(t, apiList, config.KeyDefault)
	require.Contains(t, apiList, "collector")

	// Check receive { state; }
	collectorAPI := apiList["collector"]
	receive := collectorAPI.GetContainer("receive")
	require.NotNil(t, receive)
	_, ok := receive.Get("state")
	require.True(t, ok)
}

// TestMigrateAPIBlocksNil verifies nil input handling.
//
// VALIDATES: Nil input returns ErrNilTree.
//
// PREVENTS: Panic on nil input.
func TestMigrateAPIBlocksNil(t *testing.T) {
	_, err := MigrateAPIBlocks(nil)
	require.ErrorIs(t, err, ErrNilTree)
}

// TestMigrateAPIBlocksNoAPIBlock verifies no-op when no api block.
//
// VALIDATES: Peers without api blocks are unchanged.
//
// PREVENTS: Spurious modifications.
func TestMigrateAPIBlocksNoAPIBlock(t *testing.T) {
	input := `
peer 10.0.0.1 {
    router-id 1.2.3.4;
    local-as 65001;
    peer-as 65002;
}
`
	tree := parseWithLegacySchema(t, input)
	migrated, err := MigrateAPIBlocks(tree)
	require.NoError(t, err)

	peer := migrated.GetList("peer")["10.0.0.1"]
	require.NotNil(t, peer)

	apiList := peer.GetList("api")
	require.Empty(t, apiList, "no api blocks should exist")
}

// TestMigrateAPIBlocksProcessesMatch verifies processes-match migration.
//
// VALIDATES: processes-match [ "^foo" ] → api "^foo" { }
//
// PREVENTS: Lost regex patterns during migration.
func TestMigrateAPIBlocksProcessesMatch(t *testing.T) {
	input := `
peer 10.0.0.1 {
    router-id 1.2.3.4;
    local-as 65001;
    peer-as 65002;
    api {
        processes-match [ "^collector.*" ];
    }
}
`
	tree := parseWithLegacySchema(t, input)
	migrated, err := MigrateAPIBlocks(tree)
	require.NoError(t, err)

	peer := migrated.GetList("peer")["10.0.0.1"]
	apiList := peer.GetList("api")

	require.NotContains(t, apiList, config.KeyDefault)
	require.Contains(t, apiList, "^collector.*", "pattern should be migrated as api key")
}

// TestMigrateAPIBlocksMixedProcessesAndMatch verifies both processes and processes-match.
//
// VALIDATES: Both processes and processes-match are migrated.
//
// PREVENTS: One overwriting the other.
func TestMigrateAPIBlocksMixedProcessesAndMatch(t *testing.T) {
	input := `
process foo { run ./foo; }
peer 10.0.0.1 {
    router-id 1.2.3.4;
    local-as 65001;
    peer-as 65002;
    api {
        processes [ foo ];
        processes-match [ "^bar.*" ];
        neighbor-changes;
    }
}
`
	tree := parseWithLegacySchema(t, input)
	migrated, err := MigrateAPIBlocks(tree)
	require.NoError(t, err)

	peer := migrated.GetList("peer")["10.0.0.1"]
	apiList := peer.GetList("api")

	require.Contains(t, apiList, "foo", "process should be migrated")
	require.Contains(t, apiList, "^bar.*", "pattern should be migrated")

	// Both should have receive { state; }
	fooAPI := apiList["foo"]
	require.NotNil(t, fooAPI.GetContainer("receive"))

	barAPI := apiList["^bar.*"]
	require.NotNil(t, barAPI.GetContainer("receive"))
}

// TestMigrateAPIBlocksEmptyProcessesError verifies error on empty processes.
//
// VALIDATES: api { } without processes or processes-match returns error.
//
// PREVENTS: Silent acceptance of invalid config.
func TestMigrateAPIBlocksEmptyProcessesError(t *testing.T) {
	input := `
peer 10.0.0.1 {
    router-id 1.2.3.4;
    local-as 65001;
    peer-as 65002;
    api {
        neighbor-changes;
    }
}
`
	tree := parseWithLegacySchema(t, input)
	_, err := MigrateAPIBlocks(tree)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrEmptyProcesses)
}

// TestMigrateAPIBlocksEmptyArrayError verifies error on empty processes array.
//
// VALIDATES: api { processes [ ]; } returns error.
//
// PREVENTS: Silent acceptance of empty array.
func TestMigrateAPIBlocksEmptyArrayError(t *testing.T) {
	input := `
peer 10.0.0.1 {
    router-id 1.2.3.4;
    local-as 65001;
    peer-as 65002;
    api {
        processes [ ];
    }
}
`
	tree := parseWithLegacySchema(t, input)
	_, err := MigrateAPIBlocks(tree)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrEmptyProcesses)
}

// TestMigrateAPIBlocksDuplicateProcessError verifies error on duplicate process.
//
// VALIDATES: api { processes [ foo foo ]; } returns error.
//
// PREVENTS: Silent deduplication or overwriting.
func TestMigrateAPIBlocksDuplicateProcessError(t *testing.T) {
	input := `
process foo { run ./foo; }
peer 10.0.0.1 {
    router-id 1.2.3.4;
    local-as 65001;
    peer-as 65002;
    api {
        processes [ foo foo ];
    }
}
`
	tree := parseWithLegacySchema(t, input)
	_, err := MigrateAPIBlocks(tree)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrDuplicateProcess)
	require.Contains(t, err.Error(), "foo")
}

// TestMigrateAPIBlocksDuplicateAcrossFieldsError verifies error on duplicate across fields.
//
// VALIDATES: Same name in processes and processes-match returns error.
//
// PREVENTS: Conflicting bindings.
func TestMigrateAPIBlocksDuplicateAcrossFieldsError(t *testing.T) {
	input := `
process foo { run ./foo; }
peer 10.0.0.1 {
    router-id 1.2.3.4;
    local-as 65001;
    peer-as 65002;
    api {
        processes [ foo ];
        processes-match [ foo ];
    }
}
`
	tree := parseWithLegacySchema(t, input)
	_, err := MigrateAPIBlocks(tree)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrDuplicateProcess)
}

// TestMigrateAPIBlocksCollisionError verifies error when old syntax conflicts with new.
//
// VALIDATES: api { processes [ foo ]; } with existing api foo { } returns error.
//
// PREVENTS: Silent data loss from overwriting existing named blocks.
func TestMigrateAPIBlocksCollisionError(t *testing.T) {
	input := `
process foo { run ./foo; }
peer 10.0.0.1 {
    router-id 1.2.3.4;
    local-as 65001;
    peer-as 65002;
    api {
        processes [ foo ];
    }
    api foo {
        receive { update; }
    }
}
`
	tree := parseWithLegacySchema(t, input)
	_, err := MigrateAPIBlocks(tree)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrAPICollision)
	require.Contains(t, err.Error(), "foo")
	require.Contains(t, err.Error(), "peer 10.0.0.1")
}

// TestMigrateAPIBlocksNeighborIntegration verifies full migration.
//
// VALIDATES: neighbor X { api {...} } → peer X { api name {...} }
//
// PREVENTS: API blocks lost during neighbor→peer rename.
func TestMigrateAPIBlocksNeighborIntegration(t *testing.T) {
	input := `
process watcher { run ./watcher; }
neighbor 10.0.0.1 {
    router-id 1.2.3.4;
    local-as 65001;
    peer-as 65002;
    api {
        processes [ watcher ];
        neighbor-changes;
    }
}
`
	tree := parseWithLegacySchema(t, input)

	// Use full migration
	result, err := Migrate(tree)
	require.NoError(t, err)
	migrated := result.Tree

	// Should now be peer, not neighbor
	require.Empty(t, migrated.GetList("neighbor"))
	peer := migrated.GetList("peer")["10.0.0.1"]
	require.NotNil(t, peer)

	// API should be migrated to new syntax
	apiList := peer.GetList("api")
	require.NotContains(t, apiList, config.KeyDefault)
	require.Contains(t, apiList, "watcher")

	watcherAPI := apiList["watcher"]
	receive := watcherAPI.GetContainer("receive")
	require.NotNil(t, receive)
	_, ok := receive.Get("state")
	require.True(t, ok)
}

// TestMigrateAPIBlocksNamedWithProcesses verifies named api blocks with processes field.
//
// VALIDATES: "api speaking { processes [...] }" migrates to "api foo { }".
//
// PREVENTS: Named blocks with processes inside being skipped.
func TestMigrateAPIBlocksNamedWithProcesses(t *testing.T) {
	input := `
process foo { run ./test; }
peer 10.0.0.1 {
    api speaking {
        processes [ foo ];
        receive {
            update;
        }
    }
}
`
	tree := parseWithLegacySchema(t, input)
	migrated, err := MigrateAPIBlocks(tree)
	require.NoError(t, err)

	peer := migrated.GetList("peer")["10.0.0.1"]
	require.NotNil(t, peer)

	apiList := peer.GetList("api")
	require.NotContains(t, apiList, "speaking", "old named block should be removed")
	require.Contains(t, apiList, "foo", "process name should become api key")

	fooAPI := apiList["foo"]
	receive := fooAPI.GetContainer("receive")
	require.NotNil(t, receive)
	_, ok := receive.Get("update")
	require.True(t, ok)
}

// TestMigrateAPIBlocksFormatParsed verifies parsed flag migration.
//
// VALIDATES: "receive { parsed; update; }" becomes "content { format parsed; } receive { update; }".
//
// PREVENTS: Format flags remaining in receive block.
func TestMigrateAPIBlocksFormatParsed(t *testing.T) {
	input := `
process foo { run ./test; }
peer 10.0.0.1 {
    api foo {
        receive {
            parsed;
            update;
        }
    }
}
`
	tree := parseWithLegacySchema(t, input)
	migrated, err := MigrateAPIBlocks(tree)
	require.NoError(t, err)

	peer := migrated.GetList("peer")["10.0.0.1"]
	apiList := peer.GetList("api")
	fooAPI := apiList["foo"]

	// Check content block with format
	content := fooAPI.GetContainer("content")
	require.NotNil(t, content, "content block should exist")
	format, ok := content.Get("format")
	require.True(t, ok)
	require.Equal(t, "parsed", format)

	// Check receive block has update but NOT parsed
	receive := fooAPI.GetContainer("receive")
	require.NotNil(t, receive)
	_, hasUpdate := receive.Get("update")
	require.True(t, hasUpdate)
	_, hasParsed := receive.Get("parsed")
	require.False(t, hasParsed, "parsed should be removed from receive")
}

// TestMigrateAPIBlocksFormatFull verifies consolidate flag migration.
//
// VALIDATES: "receive { parsed; packets; consolidate; }" becomes "content { format full; }".
//
// PREVENTS: consolidate not triggering full format.
func TestMigrateAPIBlocksFormatFull(t *testing.T) {
	input := `
process foo { run ./test; }
peer 10.0.0.1 {
    api foo {
        receive {
            consolidate;
            parsed;
            packets;
            update;
        }
    }
}
`
	tree := parseWithLegacySchema(t, input)
	migrated, err := MigrateAPIBlocks(tree)
	require.NoError(t, err)

	peer := migrated.GetList("peer")["10.0.0.1"]
	apiList := peer.GetList("api")
	fooAPI := apiList["foo"]

	content := fooAPI.GetContainer("content")
	require.NotNil(t, content)
	format, ok := content.Get("format")
	require.True(t, ok)
	require.Equal(t, "full", format)
}

// TestMigrateAPIBlocksFormatRaw verifies packets-only migration.
//
// VALIDATES: "receive { packets; update; }" becomes "content { format raw; }".
//
// PREVENTS: packets-only not recognized as raw format.
func TestMigrateAPIBlocksFormatRaw(t *testing.T) {
	input := `
process foo { run ./test; }
peer 10.0.0.1 {
    api foo {
        receive {
            packets;
            update;
        }
    }
}
`
	tree := parseWithLegacySchema(t, input)
	migrated, err := MigrateAPIBlocks(tree)
	require.NoError(t, err)

	peer := migrated.GetList("peer")["10.0.0.1"]
	apiList := peer.GetList("api")
	fooAPI := apiList["foo"]

	content := fooAPI.GetContainer("content")
	require.NotNil(t, content)
	format, ok := content.Get("format")
	require.True(t, ok)
	require.Equal(t, "raw", format)
}

// TestMigrateAPIBlocksSendBlock verifies send block migration.
//
// VALIDATES: send block message types are preserved.
//
// PREVENTS: send block being dropped during migration.
func TestMigrateAPIBlocksSendBlock(t *testing.T) {
	input := `
process foo { run ./test; }
peer 10.0.0.1 {
    api foo {
        receive {
            update;
        }
        send {
            update;
            refresh;
        }
    }
}
`
	tree := parseWithLegacySchema(t, input)
	migrated, err := MigrateAPIBlocks(tree)
	require.NoError(t, err)

	peer := migrated.GetList("peer")["10.0.0.1"]
	apiList := peer.GetList("api")
	fooAPI := apiList["foo"]

	send := fooAPI.GetContainer("send")
	require.NotNil(t, send)
	_, hasUpdate := send.Get("update")
	require.True(t, hasUpdate)
	_, hasRefresh := send.Get("refresh")
	require.True(t, hasRefresh)
}

// TestMigrateAPIBlocksNeighborChanges verifies neighbor-changes flag migration.
//
// VALIDATES: neighbor-changes becomes receive { state; }.
//
// PREVENTS: neighbor-changes flag being lost during migration.
func TestMigrateAPIBlocksNeighborChanges(t *testing.T) {
	input := `
process foo { run ./test; }
peer 10.0.0.1 {
    api {
        processes [ foo ];
        neighbor-changes;
    }
}
`
	tree := parseWithLegacySchema(t, input)
	migrated, err := MigrateAPIBlocks(tree)
	require.NoError(t, err)

	peer := migrated.GetList("peer")["10.0.0.1"]
	apiList := peer.GetList("api")
	fooAPI := apiList["foo"]

	receive := fooAPI.GetContainer("receive")
	require.NotNil(t, receive)
	_, hasState := receive.Get("state")
	require.True(t, hasState, "state flag should be set from neighbor-changes")
}

// parseWithLegacySchema parses config with legacy schema for testing.
func parseWithLegacySchema(t *testing.T, input string) *config.Tree {
	t.Helper()
	p := config.NewParser(config.LegacyBGPSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err)
	return tree
}
