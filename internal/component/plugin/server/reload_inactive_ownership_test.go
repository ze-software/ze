// Design: docs/architecture/config/transaction-protocol.md -- private intent selects real owners.
package server

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
)

func inactiveOwnerTree(disabled string, nested bool) map[string]any {
	root, owner := config.NewTree(), config.NewTree()
	if nested {
		root.SetContainer("lifecycle-intent", owner)
	} else {
		owner = root
	}
	owner.SetMemberStates("filters", []config.MemberState{{Value: disabled, Inactive: true}})
	return root.ToRunningMap()
}

// TestInactiveIntentSelectsRealRootAndAncestorOwners drives actual reload
// participant selection and callback transport for all-inactive leaf roots.
// Whole ancestor diffs must not hide the absent ordinary leaf, and no private
// path may be exposed as a public section root or config deletion.
func TestInactiveIntentSelectsRealRootAndAncestorOwners(t *testing.T) {
	rootBefore, rootAfter := inactiveOwnerTree("automatic:a", false), inactiveOwnerTree("automatic:b", false)
	before, after := inactiveOwnerTree("automatic:a", true), inactiveOwnerTree("automatic:b", true)
	cases := []struct {
		name          string
		before, after map[string]any
		root          string
	}{
		{"root metadata only", rootBefore, rootAfter, "filters"},
		{"nested direct leaf", before, after, "lifecycle-intent/filters"},
		{"whole added ancestor", map[string]any{}, after, "lifecycle-intent/filters"},
		{"whole removed ancestor", before, map[string]any{}, "lifecycle-intent/filters"},
		{"replaced ancestor", before, map[string]any{"lifecycle-intent": "disabled"}, "lifecycle-intent/filters"},
		{"replacement ancestor", map[string]any{"lifecycle-intent": "disabled"}, after, "lifecycle-intent/filters"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reactor := &mockReloadReactor{tree: tc.before}
			plugins := []pluginDef{{name: "private-intent-owner", roots: []string{tc.root}}}
			s := newTestReloadServer(t, reactor, plugins)
			if err := s.ReloadConfig(t.Context(), tc.after); err != nil {
				t.Fatal(err)
			}
			reactor.mu.Lock()
			applied := reactor.applyTree != nil
			reactor.mu.Unlock()
			if !applied {
				t.Error("private intent did not reach full reconcile")
			}
			responder := plugins[0].responder
			responder.mu.Lock()
			defer responder.mu.Unlock()
			if responder.verifyCalls != 1 || responder.applyCalls != 1 {
				t.Errorf("real leaf owner did not participate: verify=%d apply=%d", responder.verifyCalls, responder.applyCalls)
			}
			for _, section := range responder.verifySections {
				if section.Root != tc.root {
					t.Errorf("verify root=%q, want real root %q", section.Root, tc.root)
				}
			}
			for _, section := range responder.applySections {
				if section.Root != tc.root {
					t.Errorf("apply root=%q, want real root %q", section.Root, tc.root)
				}
			}
			for _, payload := range []any{responder.verifySections, responder.applySections} {
				body, err := json.Marshal(payload)
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(string(body), "!inactive-members") || strings.Contains(string(body), "automatic:") {
					t.Errorf("private intent leaked in callback payload: %s", body)
				}
			}
		})
	}
}

// TestPublicDiffKeepsSameNamedListEntries covers real data whose arbitrary list
// entry key equals the reserved spelling. Only actual array metadata is private.
func TestPublicDiffKeepsSameNamedListEntries(t *testing.T) {
	entry := func(value string) map[string]any {
		return map[string]any{"entries": map[string]any{"!inactive-members": map[string]any{"value": value}}}
	}
	empty := map[string]any{"entries": map[string]any{}}
	before, after := entry("old"), entry("new")
	for _, diff := range []*config.ConfigDiff{config.DiffMaps(empty, after), config.DiffMaps(before, empty), config.DiffMaps(before, after)} {
		sections := buildDiffSections(diff, []string{"entries/!inactive-members"})
		if len(sections) != 1 || sections[0].Root != "entries/!inactive-members" {
			t.Fatalf("legitimate same-name entry lost its root: %v", sections)
		}
		body := sections[0].Added + sections[0].Removed + sections[0].Changed
		if !strings.Contains(body, "value") {
			t.Errorf("legitimate same-name entry data was erased: %s", body)
		}
	}
}
