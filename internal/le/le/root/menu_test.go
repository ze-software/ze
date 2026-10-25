// VALIDATES: AC-42 (the menu le opens is the derived tree, walked by arrows and Enter), AC-43 (no menu off a terminal)
// PREVENTS: a menu that lists what dispatch does not hold, or a menu opened for an agent on a pipe

package leroot

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/ze-software/ze/internal/component/cli/menu"
)

// TestLeMenuItemsComeFromTheTree proves le's first screen lists exactly the
// first words of the derived tree, and that drilling an area answers its
// actions as terminal items whose path is the line dispatch runs. Method:
// build the levels the menu builds and compare them with Tree().
func TestLeMenuItemsComeFromTheTree(t *testing.T) {
	registerPromptProbe()
	tree := Tree()
	top := menu.LevelOf(tree, nil)
	names := make([]string, 0, len(top.Items))
	for _, item := range top.Items {
		names = append(names, item.Name)
	}
	want := make([]string, 0, len(tree.Children))
	for name := range tree.Children {
		want = append(want, name)
	}
	slices.Sort(want)
	if !slices.Equal(names, want) {
		t.Errorf("the first screen lists %v, want the tree's first words %v", names, want)
	}

	level := menu.LevelOf(tree, []string{promptProbe})
	if len(level.Items) != 1 {
		t.Fatalf("the probe area's level holds %v, want its one action", level.Items)
	}
	action := level.Items[0]
	if !action.Terminal || !slices.Equal(action.Path, []string{promptProbe, "run"}) {
		t.Errorf("the action item is %+v, want a terminal item at [%s run]", action, promptProbe)
	}
}

// TestLeMenuArrowsAndEnterDrillToAnAction proves the menu le opens is walked
// with the arrow keys and Enter: Down and Up move to an area, Enter on it
// drills into its actions without choosing anything, and Enter on an action
// chooses the line dispatch runs and quits the menu. AC-55 (D-P7-short-long):
// the right column shows each item's declared short whole, and the area's long
// description appears above its level only once Enter has drilled into it.
// Method: build the model openMenu builds over Tree() and press the keys a
// reader presses, reading the rendered view before and after Enter.
func TestLeMenuArrowsAndEnterDrillToAnAction(t *testing.T) {
	registerPromptProbe()
	tree := Tree()
	top := menu.DeclaredLevelOf(tree, nil)
	at := slices.IndexFunc(top.Items, func(item menu.Item) bool { return item.Name == promptProbe })
	if at < 0 {
		t.Fatalf("the first screen %v does not list the probe area %q", top.Items, promptProbe)
	}
	probe := tree.Children[promptProbe]
	if top.Items[at].Desc != probe.ShortHelp {
		t.Errorf("the probe's right column reads %q, want its declared short %q whole", top.Items[at].Desc, probe.ShortHelp)
	}

	var model tea.Model = menu.NewDeclared("le", tree)
	press := func(code rune) tea.Cmd {
		t.Helper()
		var cmd tea.Cmd
		model, cmd = model.Update(tea.KeyPressMsg{Code: code})
		return cmd
	}
	chosen := func() []string {
		t.Helper()
		m, ok := model.(menu.Model)
		if !ok {
			t.Fatalf("Update answered %T, want menu.Model", model)
		}
		return m.Chosen()
	}
	view := func() string {
		t.Helper()
		m, ok := model.(menu.Model)
		if !ok {
			t.Fatalf("Update answered %T, want menu.Model", model)
		}
		return m.View().Content
	}

	// One past the area, then back up onto it, so both arrows are pressed.
	for range at + 1 {
		press(tea.KeyDown)
	}
	press(tea.KeyUp)
	if screen := view(); !strings.Contains(screen, probe.ShortHelp) || strings.Contains(screen, probe.Description) {
		t.Errorf("before Enter the screen should show the probe's short %q and not its long %q:\n%s",
			probe.ShortHelp, probe.Description, screen)
	}
	if cmd := press(tea.KeyEnter); cmd != nil || len(chosen()) != 0 {
		t.Fatalf("Enter on the area chose %v (quit cmd %v), want a drill into its actions", chosen(), cmd != nil)
	}
	action := probe.Children["run"]
	if screen := view(); !strings.Contains(screen, probe.Description) || !strings.Contains(screen, action.ShortHelp) {
		t.Errorf("after Enter the screen should show the area's long %q and the action's short %q:\n%s",
			probe.Description, action.ShortHelp, screen)
	}
	if cmd := press(tea.KeyEnter); cmd == nil {
		t.Error("Enter on the action did not quit the menu")
	}
	if got := chosen(); !slices.Equal(got, []string{promptProbe, "run"}) {
		t.Errorf("the menu chose %v, want [%s run]", got, promptProbe)
	}
}

// TestBareLeOnAPipePrintsTheRootPage proves a bare le off a terminal prints
// the root page and answers 0, and that `le | json` at a terminal is still the
// payload, not the menu: only a line with no word at all at a terminal opens
// it. Method: replace the terminal question and capture stdout.
func TestBareLeOnAPipePrintsTheRootPage(t *testing.T) {
	saved := interactive
	t.Cleanup(func() { interactive = saved })

	interactive = func() bool { return false }
	code := 1
	page := captureStdout(t, func() { code = Dispatch("le", nil) })
	if code != 0 || !strings.Contains(page, "le ") {
		t.Errorf("bare le on a pipe answered %d with %q, want 0 and the root page", code, page)
	}

	interactive = func() bool { return true }
	payload := captureStdout(t, func() { code = Dispatch("le", []string{"|", "json"}) })
	if code != 0 || !strings.HasPrefix(strings.TrimSpace(payload), "{") {
		t.Errorf("le | json at a terminal answered %d with %q, want the JSON manifest", code, payload)
	}
}
