// Design: docs/architecture/cli/tui-launcher.md -- the screen ze's launcher draws
// VALIDATES: ze's launcher screens are byte-identical across the move of the menu model
// PREVENTS: a shared menu package changing what a ze user sees

//go:build ze_core

package main

import (
	"bytes"
	"flag"
	"os"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/ze-software/ze/internal/component/cli/menu"
	"github.com/ze-software/ze/internal/component/command"
)

var updateMenuGolden = flag.Bool("update-menu-golden", false, "rewrite testdata/menu-view.golden")

// TestZeMenuViewUnchanged pins the screens ze's launcher draws, so moving the
// menu model into a shared package cannot change what a ze user sees. Method:
// render the top level, a filtered top level, and a drilled level with a
// breadcrumb from one fixed item set, and compare the bytes with a golden
// captured from the model before it moved.
func TestZeMenuViewUnchanged(t *testing.T) {
	m := newZeMenu(testMenuTop(), testMenuTree())
	var screens []byte
	screens = append(screens, m.View().Content...)
	screens = append(screens, "\n----\n"...)

	filtered := pressMenu(t, m, tea.KeyPressMsg{Code: -1, Text: "v"})
	screens = append(screens, filtered.View().Content...)
	screens = append(screens, "\n----\n"...)

	drilled := pressMenu(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	screens = append(screens, drilled.View().Content...)

	const golden = "testdata/menu-view.golden"
	if *updateMenuGolden {
		if err := os.WriteFile(golden, screens, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(screens, want) {
		t.Errorf("ze's menu screens changed\n got:\n%s\nwant:\n%s", screens, want)
	}
}

// testMenuTop is the fixed first screen the pin renders.
func testMenuTop() menu.Level {
	return menu.Level{Title: "ze", Items: []menu.Item{
		{Name: "show", Desc: "Show operational data", Path: []string{"show"}},
		{Name: "set", Desc: "Modify configuration", Path: []string{"set"}},
		{Name: "Operations", Desc: "5 commands", Path: []string{"operations"}},
		{Name: "doctor", Desc: "Run health checks", Path: []string{"doctor"}, Terminal: true},
		{Name: "version", Desc: "Show version", Path: []string{"version"}, Terminal: true},
	}}
}

// testMenuTree is the verb tree the pin drills into: show holds two commands.
func testMenuTree() *command.Node {
	return &command.Node{Children: map[string]*command.Node{
		"show": {Name: "show", Children: map[string]*command.Node{
			"bgp":       {Name: "bgp", ShortHelp: "BGP state. Peers and routes"},
			"interface": {Name: "interface", ShortHelp: "Interface state", Children: map[string]*command.Node{"brief": {Name: "brief"}}},
		}},
	}}
}

// pressMenu sends one key to the menu and answers the model it leaves.
func pressMenu(t *testing.T, m menu.Model, key tea.KeyPressMsg) menu.Model {
	t.Helper()
	updated, _ := m.Update(key)
	result, ok := updated.(menu.Model)
	if !ok {
		t.Fatalf("Update answered a %T, not the menu", updated)
	}
	return result
}
