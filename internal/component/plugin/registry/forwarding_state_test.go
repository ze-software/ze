package registry

import "testing"

// VALIDATES: ForwardingStatePreserved, the question the BGP engine asks
// instead of naming a forwarding plugin.
// PREVENTS: the RFC 4724 Forwarding State bit being decided by a hardcoded
// plugin name in the engine, and a tree no plugin answers for returning
// anything but false.

// preservingReg returns a registration that answers the forwarding-state
// question from its own root in the tree.
func preservingReg(name, root string) Registration {
	reg := validReg(name)
	reg.ConfigRoots = []string{root}
	reg.PreservesForwardingState = func(tree map[string]any) bool {
		section, _ := tree[root].(map[string]any)
		return section != nil && section["keeps"] == "yes"
	}
	return reg
}

// TestForwardingStatePreservedAsksTheDeclaringPlugin proves the routing: the
// answer comes from the plugin that declared it, over the tree handed in.
func TestForwardingStatePreservedAsksTheDeclaringPlugin(t *testing.T) {
	t.Cleanup(func() { Reset() })
	Reset()

	if err := Register(preservingReg("writer", "plane")); err != nil {
		t.Fatal(err)
	}

	if !ForwardingStatePreserved(map[string]any{"plane": map[string]any{"keeps": "yes"}}) {
		t.Error("a plugin that says it keeps its routes must answer true")
	}
	if ForwardingStatePreserved(map[string]any{"plane": map[string]any{"keeps": "no"}}) {
		t.Error("a plugin that says it flushes must answer false")
	}
}

// TestForwardingStatePreservedIsFalseWithNoDeclaringPlugin is the fail-closed
// half. Nothing installs routes into a plane that outlives ze, so nothing was
// preserved, and RFC 4724 Section 4.1 allows the bit only where the state
// "has indeed been preserved".
func TestForwardingStatePreservedIsFalseWithNoDeclaringPlugin(t *testing.T) {
	t.Cleanup(func() { Reset() })
	Reset()

	if err := Register(validReg("silent")); err != nil {
		t.Fatal(err)
	}

	if ForwardingStatePreserved(map[string]any{"plane": map[string]any{"keeps": "yes"}}) {
		t.Error("a registry with no declaring plugin must answer false")
	}
}

// TestForwardingStatePreservedIsTrueWhenAnyPlaneKeeps states the OR. One
// forwarding plane retaining the routes is enough for them to be there when
// ze comes back, whatever a second plane does.
func TestForwardingStatePreservedIsTrueWhenAnyPlaneKeeps(t *testing.T) {
	t.Cleanup(func() { Reset() })
	Reset()

	for _, r := range []Registration{
		preservingReg("writer-a", "plane-a"),
		preservingReg("writer-b", "plane-b"),
	} {
		if err := Register(r); err != nil {
			t.Fatal(err)
		}
	}

	tree := map[string]any{
		"plane-a": map[string]any{"keeps": "no"},
		"plane-b": map[string]any{"keeps": "yes"},
	}
	if !ForwardingStatePreserved(tree) {
		t.Error("one plane keeping its routes is enough")
	}
}
