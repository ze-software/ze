package server

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
)

// VALIDATES: wasCreated answers "did this handler create the resource" only when the
// handler actually said so, and returns an error for every unreportable shape.
// PREVENTS: the zero-value trap (ai/rules/evidence.md) -- a missing or
// non-bool "created" key silently reading as "not created", which disarms rollback
// and strands a half-built resource stack after a failed compound create.
//
// The nil/nil-data/missing-key rows previously asserted `false` (fail-open). They now
// assert an error: reading those as a truthful "not created" is the defect this guard
// exists to close, so the old expectation encoded the bug.
func TestWasCreated(t *testing.T) {
	tests := []struct {
		name    string
		resp    *plugin.Response
		want    bool
		wantErr bool
	}{
		{"nil response", nil, false, true},
		{"nil data", &plugin.Response{Status: plugin.StatusDone}, false, true},
		{"created true", &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{"created": true}}, true, false},
		{"created false", &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{"created": false}}, false, false},
		{"no created key", &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{"message": "ok"}}, false, true},
		{"data not a plugin.Map", &plugin.Response{Status: plugin.StatusDone, Data: plugin.Text("created")}, false, true},
		{"created not a bool", &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{"created": "true"}}, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := wasCreated(tt.resp)
			if tt.wantErr {
				require.Error(t, err, "an unreportable created state must be an error, never a silent false")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// VALIDATES: a creation handler that does not report "created" aborts the compound
// command with ErrEnsureContract, naming the offending wire method, and the leaf never
// runs.
// PREVENTS: a silently-disarmed rollback. The shape is the one handleCreateVeth
// shipped with (internal/component/iface/cmd/manage.go): a creation handler that
// omits the "created" key. No live rollback was actually disarmed by it --
// ze:ensure-exists appears only on the dummy and bridge nodes
// (internal/component/iface/yang/ze-iface-cmd.yang:25 and :72), and veth carries
// none, so handleCreateVeth is never wrapped today. The contract is enforced here
// so that omission cannot become a silent no-op undo the moment a veth (or any
// other creation handler) is given an ensure-exists parent.
func TestWrapWithEnsureChain_ParentSilentOnCreated_AbortsNoLeaf(t *testing.T) {
	var calls []string

	parent := func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		calls = append(calls, "create-parent")
		return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{"message": "created veth pair"}}, nil
	}
	rollback := func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		calls = append(calls, "rollback-parent")
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}
	leaf := func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		calls = append(calls, "leaf")
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}

	chain := []EnsureStep{{Handler: parent, RollbackHandler: rollback, WireMethod: "ze-iface:interface-create-veth"}}
	wrapped := wrapWithEnsureChain(leaf, chain)

	_, err := wrapped(nil, []string{"100"})
	require.Error(t, err)
	require.ErrorIs(t, err, ErrEnsureContract)
	assert.Contains(t, err.Error(), "ze-iface:interface-create-veth", "the error must name the offending handler")
	assert.Equal(t, []string{"create-parent"}, calls, "leaf must not run when the created state is unreportable")
}

// VALIDATES: when a later step breaks the contract, the earlier steps that DID report
// created=true are rolled back, in reverse order, before the command aborts.
// PREVENTS: a contract violation deep in a chain leaking the parents already created
// above it.
func TestWrapWithEnsureChain_ContractBreakRollsBackEarlierCreated(t *testing.T) {
	var calls []string

	first := func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		calls = append(calls, "create-first")
		return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{"created": true}}, nil
	}
	firstRollback := func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		calls = append(calls, "rollback-first")
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}
	second := func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		calls = append(calls, "create-second")
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}
	secondRollback := func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		calls = append(calls, "rollback-second")
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}
	leaf := func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		calls = append(calls, "leaf")
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}

	chain := []EnsureStep{
		{Handler: first, RollbackHandler: firstRollback, WireMethod: "ze-iface:interface-create-dummy"},
		{Handler: second, RollbackHandler: secondRollback, WireMethod: "ze-iface:interface-create-veth"},
	}
	wrapped := wrapWithEnsureChain(leaf, chain)

	_, err := wrapped(nil, nil)
	require.ErrorIs(t, err, ErrEnsureContract)
	assert.Equal(t, []string{"create-first", "create-second", "rollback-first"}, calls)
}

func TestWrapWithEnsureChain_ParentCreated_LeafSucceeds(t *testing.T) {
	var calls []string

	parent := func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		calls = append(calls, "create-parent")
		return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{"created": true}}, nil
	}
	rollback := func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		calls = append(calls, "rollback-parent")
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}
	leaf := func(_ *CommandContext, args []string) (*plugin.Response, error) {
		calls = append(calls, "leaf")
		return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{"message": "unit created"}}, nil
	}

	chain := []EnsureStep{{Handler: parent, RollbackHandler: rollback}}
	wrapped := wrapWithEnsureChain(leaf, chain)

	resp, err := wrapped(nil, []string{"100"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	assert.Equal(t, []string{"create-parent", "leaf"}, calls)
}

func TestWrapWithEnsureChain_ParentExists_NoRollback(t *testing.T) {
	var calls []string

	parent := func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		calls = append(calls, "create-parent")
		return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{"created": false}}, nil
	}
	rollback := func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		calls = append(calls, "rollback-parent")
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}
	leaf := func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		calls = append(calls, "leaf")
		return nil, errors.New("unit creation failed")
	}

	chain := []EnsureStep{{Handler: parent, RollbackHandler: rollback}}
	wrapped := wrapWithEnsureChain(leaf, chain)

	_, err := wrapped(nil, nil)
	assert.Error(t, err)
	assert.Equal(t, []string{"create-parent", "leaf"}, calls, "no rollback when parent already existed")
}

func TestWrapWithEnsureChain_LeafFails_RollsBackCreated(t *testing.T) {
	var calls []string

	parent := func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		calls = append(calls, "create-parent")
		return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{"created": true}}, nil
	}
	rollback := func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		calls = append(calls, "rollback-parent")
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}
	leaf := func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		calls = append(calls, "leaf")
		return nil, errors.New("unit creation failed")
	}

	chain := []EnsureStep{{Handler: parent, RollbackHandler: rollback}}
	wrapped := wrapWithEnsureChain(leaf, chain)

	_, err := wrapped(nil, nil)
	assert.Error(t, err)
	assert.Equal(t, []string{"create-parent", "leaf", "rollback-parent"}, calls)
}

func TestWrapWithEnsureChain_LeafStatusError_RollsBack(t *testing.T) {
	var calls []string

	parent := func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		calls = append(calls, "create-parent")
		return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{"created": true}}, nil
	}
	rollback := func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		calls = append(calls, "rollback-parent")
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}
	leaf := func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		calls = append(calls, "leaf")
		return &plugin.Response{Status: plugin.StatusError, Error: "invalid VLAN ID"}, nil
	}

	chain := []EnsureStep{{Handler: parent, RollbackHandler: rollback}}
	wrapped := wrapWithEnsureChain(leaf, chain)

	resp, err := wrapped(nil, nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Equal(t, []string{"create-parent", "leaf", "rollback-parent"}, calls)
}

func TestWrapWithEnsureChain_ParentFails_AbortNoLeaf(t *testing.T) {
	var calls []string

	parent := func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		calls = append(calls, "create-parent")
		return nil, errors.New("backend unavailable")
	}
	rollback := func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		calls = append(calls, "rollback-parent")
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}
	leaf := func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		calls = append(calls, "leaf")
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}

	chain := []EnsureStep{{Handler: parent, RollbackHandler: rollback}}
	wrapped := wrapWithEnsureChain(leaf, chain)

	_, err := wrapped(nil, nil)
	assert.Error(t, err)
	assert.Equal(t, []string{"create-parent"}, calls, "leaf must not run when parent fails")
}

func TestBuildEnsureChain(t *testing.T) {
	nop := &plugin.Response{Status: plugin.StatusDone}
	handlerA := func(_ *CommandContext, _ []string) (*plugin.Response, error) { return nop, nil }
	handlerB := func(_ *CommandContext, _ []string) (*plugin.Response, error) { return nop, nil }
	handlerDelete := func(_ *CommandContext, _ []string) (*plugin.Response, error) { return nop, nil }

	tree := &command.Node{
		Children: map[string]*command.Node{
			"create": {
				Name: "create",
				Children: map[string]*command.Node{
					"interface": {
						Name: "interface",
						Children: map[string]*command.Node{
							"dummy": {
								Name:         "dummy",
								WireMethod:   "ze-iface:interface-create-dummy",
								EnsureExists: "ze-iface:interface-delete",
								Children: map[string]*command.Node{
									"unit": {
										Name:       "unit",
										WireMethod: "ze-iface:interface-unit-add",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	wireToHandler := map[string]Handler{
		"ze-iface:interface-create-dummy": handlerA,
		"ze-iface:interface-unit-add":     handlerB,
		"ze-iface:interface-delete":       handlerDelete,
	}

	t.Run("compound path has ensure chain", func(t *testing.T) {
		chain := buildEnsureChain(tree, "create interface dummy unit", wireToHandler)
		require.Len(t, chain, 1)
		assert.NotNil(t, chain[0].Handler)
		assert.NotNil(t, chain[0].RollbackHandler)
	})

	t.Run("direct path has no ensure chain", func(t *testing.T) {
		chain := buildEnsureChain(tree, "create interface unit", wireToHandler)
		assert.Empty(t, chain)
	})

	t.Run("parent-only path has no chain", func(t *testing.T) {
		chain := buildEnsureChain(tree, "create interface dummy", wireToHandler)
		assert.Empty(t, chain, "ensure-exists node itself should not trigger its own chain")
	})

	t.Run("nil tree returns nil", func(t *testing.T) {
		chain := buildEnsureChain(nil, "create interface dummy unit", wireToHandler)
		assert.Nil(t, chain)
	})
}

func TestWrapWithEnsureChain_ParentTypeConflict_AbortNoLeaf(t *testing.T) {
	var calls []string

	parent := func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		calls = append(calls, "create-parent")
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "interface br0 exists with type bridge, not dummy",
		}, nil
	}
	rollback := func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		calls = append(calls, "rollback-parent")
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}
	leaf := func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		calls = append(calls, "leaf")
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}

	chain := []EnsureStep{{Handler: parent, RollbackHandler: rollback}}
	wrapped := wrapWithEnsureChain(leaf, chain)

	resp, err := wrapped(nil, nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Error, "not dummy")
	assert.Equal(t, []string{"create-parent"}, calls, "leaf must not run when parent type conflicts")
}
