// Design: docs/guide/config-editor.md -- repair a refused stored candidate.

package cli

import (
	"bytes"
	"strings"
	"testing"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
)

// TestSessionCandidateValueCanBeRepairedAfterRefusal follows an API edit
// through refusal and correction to the running config, including reopening
// the editor over the same persisted candidate.
func TestSessionCandidateValueCanBeRepairedAfterRefusal(t *testing.T) {
	for _, reopen := range []bool{false, true} {
		name := "same editor"
		if reopen {
			name = "reopened editor"
		}
		t.Run(name, func(t *testing.T) {
			fixture := newWindowFixture(t)
			model := fixture.model(t, "alice")
			path := []string{"bgp", "peer", "peer1", "session", "capability"}
			if err := model.editor.SetValue([]string{"bgp", "peer", "peer1", "connection", "local"}, "ip", "auto"); err != nil {
				t.Fatal(err)
			}
			if err := model.editor.SetValue([]string{"bgp"}, "router-id", "9.9.9.9"); err != nil {
				t.Fatal(err)
			}
			if err := model.editor.SetValue(path, "route-refresh", "requre"); err != nil {
				t.Fatal(err)
			}
			before := fixture.active(t)
			result, err := model.dispatchCommand("commit now")
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(result.statusMessage, "commit blocked: 1 error(s)") {
				t.Fatalf("candidate was not refused: %s\n%s", result.statusMessage, result.output)
			}
			if !strings.Contains(result.output, "fix: set bgp peer peer1 session capability route-refresh require") {
				t.Fatalf("missing correction: %s", result.output)
			}
			if fixture.active(t) != before {
				t.Fatal("refusal changed the active config")
			}
			changePath := ChangePath(fixture.configPath, model.editor.session.User)
			pendingBefore, err := fixture.store.ReadFile(changePath)
			if err != nil {
				t.Fatal(err)
			}
			if reopen {
				session := model.editor.session
				model = fixture.model(t, "alice")
				model.editor.SetSession(session)
				guard, err := fixture.store.AcquireLock(fixture.configPath)
				if err != nil {
					t.Fatal(err)
				}
				reloadErr := model.editor.reloadSessionView(guard)
				releaseErr := guard.Release()
				if reloadErr != nil {
					t.Fatal(reloadErr)
				}
				if releaseErr != nil {
					t.Fatal(releaseErr)
				}
			}
			pending := model.editor.PendingChanges(model.editor.SessionID())
			found := false
			for _, change := range pending {
				if change.Path == "bgp peer peer1 session capability route-refresh" {
					found = change.Value == "requre"
				}
			}
			if !found {
				t.Fatalf("stored invalid leaf disappeared from pending changes: %+v", pending)
			}
			pendingAfter, err := fixture.store.ReadFile(changePath)
			if err != nil {
				t.Fatal(err)
			}
			if string(pendingAfter) != string(pendingBefore) {
				t.Fatal("reading the pending candidate rewrote it")
			}
			if err := model.editor.SetValue(path, "route-refresh", "disable"); err != nil {
				t.Fatalf("cannot correct the refused leaf: %v", err)
			}
			result, err = model.dispatchCommand("commit now")
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(result.statusMessage, "blocked") {
				t.Fatalf("corrected candidate was refused: %s\n%s", result.statusMessage, result.output)
			}
			tree, _, err := parseConfigWithFormat(fixture.active(t), model.editor.schema)
			if err != nil {
				t.Fatal(err)
			}
			bgp := tree.GetContainer("bgp")
			if bgp == nil {
				t.Fatal("commit lost bgp")
			}
			if routerID, _ := bgp.Get("router-id"); routerID != "9.9.9.9" {
				t.Fatalf("commit lost the other pending edit: %q", routerID)
			}
			if value := getValueAtPath(tree, model.editor.schema, append(path, "route-refresh")); value != "disable" {
				t.Fatalf("commit did not publish the repaired value: %q", value)
			}
			if changes := model.editor.PendingChanges(model.editor.SessionID()); len(changes) != 0 {
				t.Fatalf("successful commit retained changes: %+v", changes)
			}
		})
	}
}

// TestBareSessionCommitKeepsRefusedCandidateEditable covers direct editor
// callers that have no model or registered pre-commit callback. A failed
// attempt must not move invalid values into an unreadable shared draft.
func TestBareSessionCommitKeepsRefusedCandidateEditable(t *testing.T) {
	for _, route := range commitPaths {
		t.Run(route.name, func(t *testing.T) {
			configPath := writeTestConfig(t, validBGPConfig)
			store := newTestTreeStore(t, configPath)
			ed, err := NewEditorWithStorage(store, configPath)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := ed.Close(); err != nil {
					t.Error(err)
				}
			})
			ed.SetSession(NewEditSession("alice", "api"))
			path := []string{"bgp", "peer", "peer1", "session", "capability"}
			if err := ed.SetValue(path, "route-refresh", "requre"); err != nil {
				t.Fatal(err)
			}
			before, err := store.ReadFile(configPath)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := route.commit(ed); err == nil {
				t.Fatal("direct editor commit accepted an invalid enum")
			}
			after, err := store.ReadFile(configPath)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(after, before) {
				t.Fatal("refusal changed the committed config")
			}
			if _, err := store.ReadFile(ChangePath(configPath, ed.session.User)); err != nil {
				t.Fatalf("refusal consumed the stored candidate: %v", err)
			}
			if store.Exists(DraftPath(configPath)) {
				t.Fatal("refusal published an invalid shared draft")
			}
			if err := ed.SetValue(path, "route-refresh", "disable"); err != nil {
				t.Fatalf("cannot repair the refused candidate: %v", err)
			}
			result, err := route.commit(ed)
			if err != nil {
				t.Fatal(err)
			}
			if result.Applied != 1 {
				t.Fatalf("corrected commit applied %d changes", result.Applied)
			}
		})
	}
}

// TestSessionCandidateMemberCanBeDeletedAfterRefusal keeps a valid member
// while removing a refused member, through both plain and bracket leaf-lists.
func TestSessionCandidateMemberCanBeDeletedAfterRefusal(t *testing.T) {
	const module = `module ze-candidate-repair-conf {
  yang-version 1.1;
  namespace "urn:ze:candidate-repair";
  prefix probe;
  import ze-extensions { prefix ze; }
  container candidate-repair {
    leaf-list plain { type enumeration { enum blue; enum red; } }
    leaf-list bracket {
      type enumeration { enum blue; enum red; }
      ze:syntax "bracket";
    }
  }
}`
	t.Cleanup(configyang.RegisterModuleForTest("ze-candidate-repair-conf.yang", module))
	for _, key := range []string{"plain", "bracket"} {
		t.Run(key, func(t *testing.T) {
			fixture := newWindowFixture(t)
			model := fixture.model(t, "alice")
			if err := model.editor.SetValue([]string{"bgp", "peer", "peer1", "connection", "local"}, "ip", "auto"); err != nil {
				t.Fatal(err)
			}
			path := []string{"candidate-repair"}
			if err := model.editor.SetValue(path, key, "blue"); err != nil {
				t.Fatal(err)
			}
			badValue := "bogus"
			if key == "bracket" {
				badValue = "blue bogus"
			}
			if err := model.editor.SetValue(path, key, badValue); err != nil {
				t.Fatal(err)
			}
			before := fixture.active(t)
			result, err := model.dispatchCommand("commit now")
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(result.statusMessage, "commit blocked: 1 error(s)") {
				t.Fatalf("candidate was not refused: %s\n%s", result.statusMessage, result.output)
			}
			command := "delete candidate-repair " + key + " bogus"
			if !strings.Contains(result.output, "fix: "+command) {
				t.Fatalf("missing member correction: %s", result.output)
			}
			if fixture.active(t) != before {
				t.Fatal("refusal changed the active config")
			}
			if _, err := model.dispatchCommand(command); err != nil {
				t.Fatalf("cannot delete the refused member: %v", err)
			}
			result, err = model.dispatchCommand("commit now")
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(result.statusMessage, "blocked") {
				t.Fatalf("corrected candidate was refused: %s\n%s", result.statusMessage, result.output)
			}
			tree, _, err := parseConfigWithFormat(fixture.active(t), model.editor.schema)
			if err != nil {
				t.Fatal(err)
			}
			probe := tree.GetContainer("candidate-repair")
			if probe == nil {
				t.Fatal("commit lost the valid member")
			}
			members := probe.GetMultiValues(key)
			if len(members) != 1 {
				t.Fatalf("corrected members = %q", members)
			}
			if members[0] != "blue" {
				t.Fatalf("commit lost the valid member: %q", members)
			}
		})
	}
}
