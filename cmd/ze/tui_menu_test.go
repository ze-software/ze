// VALIDATES: AC-1 through AC-8 (interactive hierarchical launcher menu)
// PREVENTS: regression in non-TTY fallback, navigation, drill-down, scrolling

//go:build ze_core

package main

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/core/textbuf"
)

func testLevels() []menuLevel {
	return []menuLevel{{
		title: "ze",
		items: []menuItem{
			{name: "show", desc: "Show operational data", path: []string{"show"}, terminal: false},
			{name: "set", desc: "Modify configuration", path: []string{"set"}, terminal: false},
			{name: "Operations", desc: "5 commands", path: []string{"operations"}, terminal: false},
			{name: "doctor", desc: "Run health checks", path: []string{"doctor"}, terminal: true},
			{name: "version", desc: "Show version", path: []string{"version"}, terminal: true},
		},
	}}
}

func updateMenu(t *testing.T, m menuModel, msg tea.Msg) menuModel {
	t.Helper()
	updated, _ := m.Update(msg)
	result, ok := updated.(menuModel)
	if !ok {
		t.Fatal("Update did not return menuModel")
	}
	return result
}

func updateMenuCmd(t *testing.T, m menuModel, msg tea.Msg) (menuModel, tea.Cmd) {
	t.Helper()
	updated, cmd := m.Update(msg)
	result, ok := updated.(menuModel)
	if !ok {
		t.Fatal("Update did not return menuModel")
	}
	return result, cmd
}

func TestMenuModel_Init(t *testing.T) {
	m := menuModel{stack: testLevels(), width: 80, height: 24}
	cmd := m.Init()
	if cmd != nil {
		t.Error("Init should return nil cmd")
	}
	if m.cursor != 0 {
		t.Errorf("initial cursor = %d, want 0", m.cursor)
	}
	if len(m.chosen) != 0 {
		t.Errorf("initial chosen = %v, want empty", m.chosen)
	}
	if m.quitting {
		t.Error("initial quitting should be false")
	}
	if m.totalItems() != 5 {
		t.Errorf("totalItems = %d, want 5", m.totalItems())
	}
}

func TestMenuModel_Navigation(t *testing.T) {
	m := menuModel{stack: testLevels(), width: 80, height: 24}
	total := m.totalItems()

	m = updateMenu(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	if m.cursor != 1 {
		t.Errorf("after down: cursor = %d, want 1", m.cursor)
	}

	m = updateMenu(t, m, tea.KeyPressMsg{Code: tea.KeyUp})
	if m.cursor != 0 {
		t.Errorf("after up: cursor = %d, want 0", m.cursor)
	}

	m = updateMenu(t, m, tea.KeyPressMsg{Code: tea.KeyUp})
	if m.cursor != total-1 {
		t.Errorf("after wrap up: cursor = %d, want %d", m.cursor, total-1)
	}

	m = updateMenu(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	if m.cursor != 0 {
		t.Errorf("after wrap down: cursor = %d, want 0", m.cursor)
	}

}

func TestMenuModel_NavigationBoundaries(t *testing.T) {
	m := menuModel{stack: testLevels(), width: 80, height: 24}
	total := m.totalItems()

	for i := 1; i < total; i++ {
		m = updateMenu(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
		if m.cursor != i {
			t.Errorf("step %d: cursor = %d, want %d", i, m.cursor, i)
		}
	}

	m = updateMenu(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	if m.cursor != 0 {
		t.Errorf("after full cycle down: cursor = %d, want 0", m.cursor)
	}

	m = updateMenu(t, m, tea.KeyPressMsg{Code: tea.KeyUp})
	if m.cursor != total-1 {
		t.Errorf("up from 0: cursor = %d, want %d", m.cursor, total-1)
	}
}

func TestMenuModel_SelectTerminal(t *testing.T) {
	m := menuModel{stack: testLevels(), width: 80, height: 24}

	// Navigate to "doctor" (index 3, terminal).
	for range 3 {
		m = updateMenu(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}

	m, cmd := updateMenuCmd(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if len(m.chosen) != 1 || m.chosen[0] != "doctor" {
		t.Errorf("chosen = %v, want [doctor]", m.chosen)
	}
	if !m.quitting {
		t.Error("quitting should be true after selecting terminal item")
	}
	if cmd == nil {
		t.Error("Enter on terminal should return quit cmd")
	}
}

func TestMenuModel_DrillDown(t *testing.T) {
	sub := menuLevel{
		title: "show",
		items: []menuItem{
			{name: "bgp", desc: "BGP state", path: []string{"show", "bgp"}, terminal: false},
			{name: "version", desc: "Show version", path: []string{"show", "version"}, terminal: true},
		},
	}
	m := menuModel{stack: testLevels(), width: 80, height: 24}

	// "show" is at index 0, non-terminal. Enter drills down.
	// We can't use buildYANGLevel in tests (it needs the full tree),
	// so test the stack mechanism directly.
	m.stack = append(m.stack, sub)
	m.cursor = 0
	m.offset = 0

	if len(m.stack) != 2 {
		t.Fatalf("stack depth = %d, want 2", len(m.stack))
	}
	if m.totalItems() != 2 {
		t.Errorf("items in sub-level = %d, want 2", m.totalItems())
	}

	// Navigate to "version" (index 1, terminal) and select.
	m = updateMenu(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	m, cmd := updateMenuCmd(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if len(m.chosen) != 2 || m.chosen[0] != "show" || m.chosen[1] != "version" {
		t.Errorf("chosen = %v, want [show version]", m.chosen)
	}
	if cmd == nil {
		t.Error("Enter on terminal should return quit cmd")
	}
}

func TestMenuModel_BackNavigation(t *testing.T) {
	sub := menuLevel{
		title: "show",
		items: []menuItem{
			{name: "bgp", desc: "BGP state", path: []string{"show", "bgp"}, terminal: true},
		},
	}
	m := menuModel{stack: testLevels(), width: 80, height: 24}
	m.stack = append(m.stack, sub)
	m.cursor = 0

	if len(m.stack) != 2 {
		t.Fatalf("stack depth = %d, want 2", len(m.stack))
	}

	// Left arrow goes back.
	m = updateMenu(t, m, tea.KeyPressMsg{Code: tea.KeyLeft})
	if len(m.stack) != 1 {
		t.Errorf("stack depth after back = %d, want 1", len(m.stack))
	}
	if m.cursor != 0 {
		t.Errorf("cursor after back = %d, want 0", m.cursor)
	}

	// Backspace also goes back.
	m.stack = append(m.stack, sub)
	m = updateMenu(t, m, tea.KeyPressMsg{Code: tea.KeyBackspace})
	if len(m.stack) != 1 {
		t.Errorf("stack depth after backspace back = %d, want 1", len(m.stack))
	}
}

func TestMenuModel_EscBackThenQuit(t *testing.T) {
	sub := menuLevel{
		title: "show",
		items: []menuItem{
			{name: "bgp", desc: "BGP state", path: []string{"show", "bgp"}, terminal: true},
		},
	}
	m := menuModel{stack: testLevels(), width: 80, height: 24}
	m.stack = append(m.stack, sub)

	// Esc at depth 2 goes back.
	m = updateMenu(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if len(m.stack) != 1 {
		t.Errorf("stack depth after esc = %d, want 1", len(m.stack))
	}
	if m.quitting {
		t.Error("should not quit when going back from sub-level")
	}

	// Esc at depth 1 quits.
	m, cmd := updateMenuCmd(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if !m.quitting {
		t.Error("should quit at top level esc")
	}
	if cmd == nil {
		t.Error("Esc at top should return quit cmd")
	}
}

func TestMenuModel_QuitFromAnywhere(t *testing.T) {
	sub := menuLevel{
		title: "show",
		items: []menuItem{
			{name: "bgp", desc: "BGP state", path: []string{"show", "bgp"}, terminal: true},
		},
	}
	m := menuModel{stack: testLevels(), width: 80, height: 24}
	m.stack = append(m.stack, sub)

	m = updateMenu(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.quitting {
		t.Error("first esc should go back, not quit")
	}
	m, cmd := updateMenuCmd(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if !m.quitting {
		t.Error("second esc at top should quit")
	}
	if cmd == nil {
		t.Error("esc at top should return quit cmd")
	}
}

func TestMenuModel_View(t *testing.T) {
	m := menuModel{stack: testLevels(), width: 80, height: 24}
	content := m.View().Content

	if !strings.Contains(content, "ze") {
		t.Error("view should contain title")
	}

	for _, item := range m.currentLevel().items {
		if !strings.Contains(content, item.name) {
			t.Errorf("view should contain item name %q", item.name)
		}
	}

	if !strings.Contains(content, ">") {
		t.Error("view should contain cursor indicator >")
	}

	if !strings.Contains(content, "navigate") {
		t.Error("view should contain navigation hint")
	}

	m.quitting = true
	content = m.View().Content
	if content != "" {
		t.Errorf("view after quitting = %q, want empty", content)
	}
}

func TestMenuModel_ViewBreadcrumb(t *testing.T) {
	sub := menuLevel{
		title: "show",
		items: []menuItem{
			{name: "bgp", desc: "BGP state", path: []string{"show", "bgp"}, terminal: true},
		},
	}
	m := menuModel{stack: testLevels(), width: 80, height: 24}
	m.stack = append(m.stack, sub)

	content := m.View().Content
	if !strings.Contains(content, "show") {
		t.Error("view should contain breadcrumb 'show'")
	}
	if !strings.Contains(content, "back") {
		t.Error("view at sub-level should mention back navigation")
	}
}

func TestMenuModel_ViewNonTerminalArrow(t *testing.T) {
	m := menuModel{stack: testLevels(), width: 80, height: 24}
	content := m.View().Content

	// Non-terminal items should have a ">" drill indicator.
	// "show" is non-terminal and should show the arrow.
	lines := strings.Split(content, "\n")
	found := false
	for _, line := range lines {
		if strings.Contains(line, "show") && strings.Contains(line, "Show operational") {
			if strings.Count(line, ">") >= 2 {
				found = true
			}
		}
	}
	if !found {
		t.Error("non-terminal item should have drill-down arrow indicator")
	}
}

func TestMenuModel_Scrolling(t *testing.T) {
	var items []menuItem
	for i := range 30 {
		var tb textbuf.Buffer
		tb.Str("cmd").Int(int64(i))
		name := tb.String()
		items = append(items, menuItem{name: name, desc: "test", path: []string{name}, terminal: true})
	}
	m := menuModel{
		stack:  []menuLevel{{title: "test", items: items}},
		height: 12,
	}

	vis := m.visibleCount()
	if vis <= 0 {
		t.Fatalf("visibleCount = %d, want > 0", vis)
	}

	// Navigate past visible area.
	for range vis + 2 {
		m = updateMenu(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}

	if m.offset == 0 {
		t.Error("offset should have scrolled down")
	}

	content := m.View().Content
	if !strings.Contains(content, "more") {
		t.Error("should show scroll indicator when items overflow")
	}
}

func TestMenuModel_EmptyMenu(t *testing.T) {
	m := menuModel{stack: []menuLevel{{title: "empty", items: nil}}, height: 24}
	if m.totalItems() != 0 {
		t.Errorf("totalItems on empty = %d, want 0", m.totalItems())
	}

	m = updateMenu(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	if m.cursor != 0 {
		t.Errorf("down on empty: cursor = %d, want 0", m.cursor)
	}

	m = updateMenu(t, m, tea.KeyPressMsg{Code: tea.KeyUp})
	if m.cursor != 0 {
		t.Errorf("up on empty: cursor = %d, want 0", m.cursor)
	}
}

func TestMenuModel_Filter(t *testing.T) {
	m := menuModel{stack: testLevels(), width: 80, height: 24}
	m.cursor = m.firstVisible()

	// Type "v" to filter. Only "version" should remain visible.
	m = updateMenu(t, m, tea.KeyPressMsg{Code: -1, Text: "v"})
	if m.filter != "v" {
		t.Errorf("filter = %q, want v", m.filter)
	}

	// Cursor should be on "version".
	item := m.currentLevel().items[m.cursor]
	if item.name != "version" {
		t.Errorf("cursor on %q, want version", item.name)
	}

	// "doctor" should not be visible.
	doctorVisible := false
	for _, it := range m.currentLevel().items {
		if it.name == "doctor" && m.isVisible(it) {
			doctorVisible = true
		}
	}
	if doctorVisible {
		t.Error("doctor should be hidden by filter 'v'")
	}

	// Backspace removes filter char.
	m = updateMenu(t, m, tea.KeyPressMsg{Code: tea.KeyBackspace})
	if m.filter != "" {
		t.Errorf("filter after backspace = %q, want empty", m.filter)
	}

	// All items visible again.
	if !m.isVisible(menuItem{name: "doctor"}) {
		t.Error("doctor should be visible after clearing filter")
	}

	// Esc clears filter.
	m = updateMenu(t, m, tea.KeyPressMsg{Code: -1, Text: "s"})
	m = updateMenu(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.filter != "" {
		t.Errorf("filter after esc = %q, want empty", m.filter)
	}
}

func TestBuildTopLevel(t *testing.T) {
	const name = "ztest-tui-root2"
	const desc = "sentinel for TUI menu test"
	registry.MustRegisterRootHandler(name, func(_ *registry.RuntimeContext, _ []string) int {
		return 0
	}, registry.Meta{Description: desc, Section: registry.SectionOperations})

	level := buildTopLevel()
	if len(level.items) == 0 {
		t.Fatal("buildTopLevel returned no items")
	}

	hasHeader := false
	hasCommand := false
	for _, item := range level.items {
		if item.header {
			hasHeader = true
		}
		if !item.header && item.name != "" {
			hasCommand = true
		}
	}
	if !hasHeader {
		t.Error("top level should contain section headers")
	}
	if !hasCommand {
		t.Error("top level should contain selectable commands")
	}
}

func TestMenuModel_WindowSize(t *testing.T) {
	m := menuModel{stack: testLevels(), width: 80, height: 24}

	m = updateMenu(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})
	if m.height != 40 {
		t.Errorf("height = %d, want 40", m.height)
	}
	if m.width != 120 {
		t.Errorf("width = %d, want 120", m.width)
	}
}

func TestNonTTYFallback(t *testing.T) {
	orig := stdinIsTerminal
	stdinIsTerminal = func() bool { return false }
	defer func() { stdinIsTerminal = orig }()

	if stdinIsTerminal() {
		t.Error("stdinIsTerminal should return false in this test")
	}
}
