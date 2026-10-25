// VALIDATES: AC-1 through AC-8 (interactive hierarchical launcher menu)
// PREVENTS: regression in non-TTY fallback, navigation, drill-down, scrolling

package menu

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/ze-software/ze/internal/core/textbuf"
)

func testLevels() []Level {
	return []Level{{
		Title: "ze",
		Items: []Item{
			{Name: "show", Desc: "Show operational data", Path: []string{"show"}, Terminal: false},
			{Name: "set", Desc: "Modify configuration", Path: []string{"set"}, Terminal: false},
			{Name: "Operations", Desc: "5 commands", Path: []string{"operations"}, Terminal: false},
			{Name: "doctor", Desc: "Run health checks", Path: []string{"doctor"}, Terminal: true},
			{Name: "version", Desc: "Show version", Path: []string{"version"}, Terminal: true},
		},
	}}
}

func updateMenu(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	updated, _ := m.Update(msg)
	result, ok := updated.(Model)
	if !ok {
		t.Fatal("Update did not return Model")
	}
	return result
}

func updateMenuCmd(t *testing.T, m Model, msg tea.Msg) (Model, tea.Cmd) {
	t.Helper()
	updated, cmd := m.Update(msg)
	result, ok := updated.(Model)
	if !ok {
		t.Fatal("Update did not return Model")
	}
	return result, cmd
}

func TestMenuModel_Init(t *testing.T) {
	m := Model{title: "ze", stack: testLevels(), width: 80, height: 24}
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
	m := Model{title: "ze", stack: testLevels(), width: 80, height: 24}
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
	m := Model{title: "ze", stack: testLevels(), width: 80, height: 24}
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
	m := Model{title: "ze", stack: testLevels(), width: 80, height: 24}

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
	sub := Level{
		Title: "show",
		Items: []Item{
			{Name: "bgp", Desc: "BGP state", Path: []string{"show", "bgp"}, Terminal: false},
			{Name: "version", Desc: "Show version", Path: []string{"show", "version"}, Terminal: true},
		},
	}
	m := Model{title: "ze", stack: testLevels(), width: 80, height: 24}

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
	sub := Level{
		Title: "show",
		Items: []Item{
			{Name: "bgp", Desc: "BGP state", Path: []string{"show", "bgp"}, Terminal: true},
		},
	}
	m := Model{title: "ze", stack: testLevels(), width: 80, height: 24}
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
	sub := Level{
		Title: "show",
		Items: []Item{
			{Name: "bgp", Desc: "BGP state", Path: []string{"show", "bgp"}, Terminal: true},
		},
	}
	m := Model{title: "ze", stack: testLevels(), width: 80, height: 24}
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
	sub := Level{
		Title: "show",
		Items: []Item{
			{Name: "bgp", Desc: "BGP state", Path: []string{"show", "bgp"}, Terminal: true},
		},
	}
	m := Model{title: "ze", stack: testLevels(), width: 80, height: 24}
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
	m := Model{title: "ze", stack: testLevels(), width: 80, height: 24}
	content := m.View().Content

	if !strings.Contains(content, "ze") {
		t.Error("view should contain title")
	}

	for _, item := range m.currentLevel().Items {
		if !strings.Contains(content, item.Name) {
			t.Errorf("view should contain item name %q", item.Name)
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
	sub := Level{
		Title: "show",
		Items: []Item{
			{Name: "bgp", Desc: "BGP state", Path: []string{"show", "bgp"}, Terminal: true},
		},
	}
	m := Model{title: "ze", stack: testLevels(), width: 80, height: 24}
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
	m := Model{title: "ze", stack: testLevels(), width: 80, height: 24}
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
	var items []Item
	for i := range 30 {
		var tb textbuf.Buffer
		tb.Str("cmd").Int(int64(i))
		name := tb.String()
		items = append(items, Item{Name: name, Desc: "test", Path: []string{name}, Terminal: true})
	}
	m := Model{
		stack:  []Level{{Title: "test", Items: items}},
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
	m := Model{stack: []Level{{Title: "empty", Items: nil}}, height: 24}
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
	m := Model{title: "ze", stack: testLevels(), width: 80, height: 24}
	m.cursor = m.firstVisible()

	// Type "v" to filter. Only "version" should remain visible.
	m = updateMenu(t, m, tea.KeyPressMsg{Code: -1, Text: "v"})
	if m.filter != "v" {
		t.Errorf("filter = %q, want v", m.filter)
	}

	// Cursor should be on "version".
	item := m.currentLevel().Items[m.cursor]
	if item.Name != "version" {
		t.Errorf("cursor on %q, want version", item.Name)
	}

	// "doctor" should not be visible.
	doctorVisible := false
	for _, it := range m.currentLevel().Items {
		if it.Name == "doctor" && m.isVisible(it) {
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
	if !m.isVisible(Item{Name: "doctor"}) {
		t.Error("doctor should be visible after clearing filter")
	}

	// Esc clears filter.
	m = updateMenu(t, m, tea.KeyPressMsg{Code: -1, Text: "s"})
	m = updateMenu(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.filter != "" {
		t.Errorf("filter after esc = %q, want empty", m.filter)
	}
}

func TestMenuModel_WindowSize(t *testing.T) {
	m := Model{title: "ze", stack: testLevels(), width: 80, height: 24}

	m = updateMenu(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})
	if m.height != 40 {
		t.Errorf("height = %d, want 40", m.height)
	}
	if m.width != 120 {
		t.Errorf("width = %d, want 120", m.width)
	}
}

// TestMenuModel_QQuitsUnlessFiltering proves the q key quits from any level
// while no filter is typed, and is a filter letter once one is. Method: press q
// on a drilled level, then on a level with a filter typed.
func TestMenuModel_QQuitsUnlessFiltering(t *testing.T) {
	m := New("le", testLevels()[0], nil)
	m = updateMenu(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if len(m.stack) != 2 {
		t.Fatalf("enter on show should drill, stack = %d", len(m.stack))
	}
	quit, cmd := updateMenuCmd(t, m, tea.KeyPressMsg{Code: -1, Text: "q"})
	if !quit.quitting || cmd == nil {
		t.Error("q on a drilled level with no filter should quit")
	}
	if quit.Chosen() != nil {
		t.Errorf("quitting chose %v, want nothing", quit.Chosen())
	}

	filtering := updateMenu(t, New("le", testLevels()[0], nil), tea.KeyPressMsg{Code: -1, Text: "s"})
	filtering, cmd = updateMenuCmd(t, filtering, tea.KeyPressMsg{Code: -1, Text: "q"})
	if filtering.quitting || cmd != nil {
		t.Error("q while filtering should be a filter letter, not quit")
	}
	if filtering.filter != "sq" {
		t.Errorf("filter = %q, want sq", filtering.filter)
	}
}

// TestMenuTitleIsTheProgramName proves the first breadcrumb is the program
// the menu was opened for, so `ze le` and `le` each show their own name.
func TestMenuTitleIsTheProgramName(t *testing.T) {
	if content := New("ze le", testLevels()[0], nil).View().Content; !strings.Contains(content, "ze le") {
		t.Errorf("view does not name the program:\n%s", content)
	}
}
