// Design: docs/architecture/config/transaction-protocol.md -- current running intent.
package server

import (
	"reflect"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
)

// inactiveReloadTree keeps the ordinary effective leaf unchanged while changing
// only which automatic member the operator deliberately disabled.
func inactiveReloadTree(disabled string) *config.Tree {
	root, bgp, direction := config.NewTree(), config.NewTree(), config.NewTree()
	root.SetContainer("bgp", bgp)
	bgp.SetContainer("policy-intent", direction)
	direction.SetMemberStates("filters", []config.MemberState{
		{Value: "operator:keep"}, {Value: disabled, Inactive: true},
	})
	return root
}

// TestReloadInactiveIntentOnlyReachesReconcile drives the real reload server,
// transaction and plugin transport with a metadata-only configuration change.
// Public active settings are identical, but the internal intent still requires
// a full reconcile and root participation; private metadata must reach neither
// verify nor apply payloads.
func TestReloadInactiveIntentOnlyReachesReconcile(t *testing.T) {
	before, candidate := inactiveReloadTree("automatic:a"), inactiveReloadTree("automatic:b")
	if !reflect.DeepEqual(before.ToPluginMap(), candidate.ToPluginMap()) {
		t.Fatal("fixture changed ordinary active settings; it is not metadata-only")
	}
	running, next := before.ToRunningMap(), candidate.ToRunningMap()
	reactor := &mockReloadReactor{tree: running}
	plugins := []pluginDef{{name: "intent-owner", roots: []string{"bgp"}}}
	s := newTestReloadServer(t, reactor, plugins)
	if err := s.ReloadConfig(t.Context(), next); err != nil {
		t.Fatal(err)
	}
	reactor.mu.Lock()
	applied, published := reactor.applyTree, reactor.setTree
	reactor.mu.Unlock()
	if applied == nil || !reflect.DeepEqual(applied, next) {
		t.Error("metadata-only change did not reach full ApplyConfigDiff")
	}
	if published == nil || !reflect.DeepEqual(published, next) {
		t.Error("metadata-only candidate did not become the running authority")
	}
	responder := plugins[0].responder
	responder.mu.Lock()
	defer responder.mu.Unlock()
	if responder.verifyCalls != 1 || responder.applyCalls != 1 {
		t.Fatalf("root participation verify=%d apply=%d, want one each", responder.verifyCalls, responder.applyCalls)
	}
	for _, section := range responder.verifySections {
		if strings.Contains(section.Data, "!inactive-members") || strings.Contains(section.Data, "automatic:") {
			t.Errorf("private inactive intent leaked to verify: %s", section.Data)
		}
	}
	for _, section := range responder.applySections {
		for _, body := range []string{section.Added, section.Removed, section.Changed} {
			if strings.Contains(body, "!inactive-members") || strings.Contains(body, "automatic:") {
				t.Errorf("private inactive intent leaked to apply: %s", body)
			}
		}
	}
}
