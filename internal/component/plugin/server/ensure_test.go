package server

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
)

func TestWasCreated(t *testing.T) {
	tests := []struct {
		name string
		resp *plugin.Response
		want bool
	}{
		{"nil response", nil, false},
		{"nil data", &plugin.Response{Status: plugin.StatusDone}, false},
		{"created true", &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{"created": true}}, true},
		{"created false", &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{"created": false}}, false},
		{"no created key", &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{"message": "ok"}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, wasCreated(tt.resp))
		})
	}
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
