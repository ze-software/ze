// Design: docs/architecture/cli/tui-launcher.md -- interactive no-arg launcher

//go:build ze_core

package main

import (
	"os"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"golang.org/x/term"

	cli "github.com/ze-software/ze/internal/component/cli/client"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var stdinIsTerminal = func() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

type menuItem struct {
	name     string
	desc     string
	path     []string
	terminal bool
	header   bool
}

type menuLevel struct {
	title string
	items []menuItem
}

type menuModel struct {
	stack    []menuLevel
	filter   string
	cursor   int
	offset   int
	width    int
	height   int
	chosen   []string
	quitting bool
}

const menuDescMaxLen = 50

func truncateDesc(s string) string {
	if i := strings.IndexAny(s, ".;"); i > 0 && i < menuDescMaxLen {
		return s[:i]
	}
	if len(s) <= menuDescMaxLen {
		return s
	}
	return s[:menuDescMaxLen]
}

func buildTopLevel() menuLevel {
	verbTree := cli.BuildCommandTree(false)
	cmdEntries := command.HelpEntries(verbTree, nil)
	yangItems := make([]menuItem, 0, len(cmdEntries))
	for _, e := range cmdEntries {
		node := command.FindNode(verbTree, []string{e.Name})
		hasChildren := node != nil && len(node.Children) > 0
		yangItems = append(yangItems, menuItem{
			name:     e.Name,
			desc:     truncateDesc(e.Desc),
			path:     []string{e.Name},
			terminal: !hasChildren,
		})
	}

	var items []menuItem
	for _, se := range registry.ListRootBySection() {
		title := registry.SectionTitle(se.Section)
		if title == "" {
			title = se.Section
		}
		items = append(items, menuItem{name: title, header: true})

		cmdItems := make([]menuItem, 0, len(se.Commands))
		for _, rc := range se.Commands {
			cmdItems = append(cmdItems, menuItem{
				name:     rc.Name,
				desc:     truncateDesc(rc.Meta.Description),
				path:     []string{rc.Name},
				terminal: true,
			})
		}

		if se.Section == registry.SectionOperations {
			cmdItems = append(cmdItems, yangItems...)
			sort.Slice(cmdItems, func(i, j int) bool { return cmdItems[i].name < cmdItems[j].name })
		}

		items = append(items, cmdItems...)
	}
	return menuLevel{title: "ze", items: items}
}

func buildYANGLevel(path []string) menuLevel {
	verbTree := cli.BuildCommandTree(false)
	node := command.FindNode(verbTree, path)
	if node == nil || len(node.Children) == 0 {
		return menuLevel{title: textbuf.Join(path, " "), items: nil}
	}

	names := make([]string, 0, len(node.Children))
	for name := range node.Children {
		names = append(names, name)
	}
	sort.Strings(names)

	items := make([]menuItem, 0, len(names))
	for _, name := range names {
		child := node.Children[name]
		childPath := make([]string, len(path)+1)
		copy(childPath, path)
		childPath[len(path)] = name

		items = append(items, menuItem{
			name:     name,
			desc:     truncateDesc(child.Description),
			path:     childPath,
			terminal: len(child.Children) == 0,
		})
	}

	return menuLevel{title: textbuf.Join(path, " "), items: items}
}

func (m menuModel) currentLevel() menuLevel {
	if len(m.stack) == 0 {
		return menuLevel{}
	}
	return m.stack[len(m.stack)-1]
}

func (m menuModel) totalItems() int {
	return len(m.currentLevel().items)
}

func (m menuModel) isVisible(item menuItem) bool {
	if item.header {
		return m.filter == ""
	}
	if m.filter == "" {
		return true
	}
	return strings.HasPrefix(item.name, m.filter)
}

func (m menuModel) nextVisible(from, dir int) int {
	items := m.currentLevel().items
	n := len(items)
	if n == 0 {
		return 0
	}
	pos := from
	for range n {
		pos = (pos + dir + n) % n
		if m.isVisible(items[pos]) && !items[pos].header {
			return pos
		}
	}
	return from
}

func (m menuModel) firstVisible() int {
	for i, item := range m.currentLevel().items {
		if m.isVisible(item) && !item.header {
			return i
		}
	}
	return 0
}

func (m menuModel) hasVisibleItems() bool {
	for _, item := range m.currentLevel().items {
		if m.isVisible(item) && !item.header {
			return true
		}
	}
	return false
}

func (m menuModel) visibleCount() int {
	return max(m.height-6, 3)
}

func (m menuModel) Init() tea.Cmd {
	return nil
}

func (m menuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m menuModel) handleKey(km tea.KeyPressMsg) (tea.Model, tea.Cmd) { //nolint:cyclop // flat switch is clearer than extraction
	switch { //nolint:staticcheck // QF1002: default branch tests km.Text not km.Code
	case km.Code == tea.KeyUp:
		if !m.hasVisibleItems() {
			return m, nil
		}
		m.cursor = m.nextVisible(m.cursor, -1)
		m = m.adjustScroll()

	case km.Code == tea.KeyDown:
		if !m.hasVisibleItems() {
			return m, nil
		}
		m.cursor = m.nextVisible(m.cursor, 1)
		m = m.adjustScroll()

	case km.Code == tea.KeyEnter:
		if !m.hasVisibleItems() {
			return m, nil
		}
		item := m.currentLevel().items[m.cursor]
		if item.header {
			return m, nil
		}
		if item.terminal {
			m.chosen = item.path
			m.quitting = true
			return m, tea.Quit
		}
		m.filter = ""
		m = m.drillInto(item)

	case km.Code == tea.KeyBackspace:
		if m.filter != "" {
			m.filter = m.filter[:len(m.filter)-1]
			m.cursor = m.firstVisible()
			m.offset = 0
			m = m.adjustScroll()
		} else if len(m.stack) > 1 {
			m.stack = m.stack[:len(m.stack)-1]
			m.cursor = m.firstVisible()
			m.offset = 0
		}

	case km.Code == tea.KeyLeft:
		m.filter = ""
		if len(m.stack) > 1 {
			m.stack = m.stack[:len(m.stack)-1]
			m.cursor = m.firstVisible()
			m.offset = 0
		}

	case km.Code == tea.KeyEscape:
		switch {
		case m.filter != "":
			m.filter = ""
			m.cursor = m.firstVisible()
			m.offset = 0
		case len(m.stack) > 1:
			m.stack = m.stack[:len(m.stack)-1]
			m.cursor = m.firstVisible()
			m.offset = 0
		default:
			m.quitting = true
			return m, tea.Quit
		}

	default:
		if len(km.Text) == 1 && km.Text[0] >= ' ' {
			m.filter += strings.ToLower(km.Text)
			m.cursor = m.firstVisible()
			m.offset = 0
			m = m.adjustScroll()
		}
	}

	return m, nil
}

func (m menuModel) drillInto(item menuItem) menuModel {
	level := buildYANGLevel(item.path)
	m.stack = append(m.stack, level)
	m.offset = 0
	m.cursor = m.firstVisible()
	return m
}

func (m menuModel) cursorVisibleIndex() int {
	vi := 0
	for _, item := range m.currentLevel().items[:m.cursor] {
		if m.isVisible(item) {
			vi++
		}
	}
	return vi
}

func (m menuModel) adjustScroll() menuModel {
	vi := m.cursorVisibleIndex()
	vis := m.visibleCount()
	if vi < m.offset {
		m.offset = vi
	} else if vi >= m.offset+vis {
		m.offset = vi - vis + 1
	}
	return m
}

const menuNameColumnWidth = 18

var (
	menuTitleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	menuSectionStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	menuActiveStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("73"))
	menuNameStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("73"))
	menuDescStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	menuHintStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	menuArrowStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	menuBreadcrumb   = lipgloss.NewStyle().Foreground(lipgloss.Color("33"))
)

func (m menuModel) View() tea.View {
	if m.quitting {
		return tea.NewView("")
	}

	level := m.currentLevel()
	var b textbuf.Buffer
	b.Reset()

	b.Str(menuTitleStyle.Render("ze"))
	if len(m.stack) > 1 {
		for i := 1; i < len(m.stack); i++ {
			b.Str(" > ")
			b.Str(menuBreadcrumb.Render(m.stack[i].title))
		}
	}
	b.Str("\n\n")

	var visible []int
	for i, item := range level.items {
		if m.isVisible(item) {
			visible = append(visible, i)
		}
	}

	var tb textbuf.Buffer

	vis := m.visibleCount()
	end := min(m.offset+vis, len(visible))

	if m.offset > 0 {
		b.Str(menuHintStyle.Render("  ... more above"))
		b.Byte('\n')
	}

	descBudget := max(m.width-menuNameColumnWidth-6, 10)

	for vi := m.offset; vi < end; vi++ {
		item := level.items[visible[vi]]

		if item.header {
			if vi > m.offset {
				b.Byte('\n')
			}
			b.Str(menuSectionStyle.Render(item.name))
			b.Byte('\n')
			continue
		}

		tb.Reset()
		tb.PadRight(item.name, menuNameColumnWidth)
		paddedName := tb.String()

		desc := item.desc
		if len(desc) > descBudget {
			tb.Reset()
			tb.Str(desc[:descBudget-3]).Str("...")
			desc = tb.String()
		}

		arrow := ""
		if !item.terminal {
			arrow = menuArrowStyle.Render(" >")
		}

		tb.Reset()
		if visible[vi] == m.cursor {
			tb.Str("> ").Str(menuActiveStyle.Render(paddedName)).Str(menuDescStyle.Render(desc)).Str(arrow)
		} else {
			tb.Str("  ").Str(menuNameStyle.Render(paddedName)).Str(menuDescStyle.Render(desc)).Str(arrow)
		}
		b.Str(tb.String())
		b.Byte('\n')
	}

	if end < len(visible) {
		b.Str(menuHintStyle.Render("  ... more below"))
		b.Byte('\n')
	}

	b.Byte('\n')
	if m.filter != "" {
		tb.Reset()
		tb.Str("filter: ").Str(m.filter)
		b.Str(menuHintStyle.Render(tb.String()))
		b.Str("  ")
	}
	if len(m.stack) > 1 {
		b.Str(menuHintStyle.Render("↑/↓ navigate • enter select • ← back • esc quit"))
	} else {
		b.Str(menuHintStyle.Render("↑/↓ navigate • enter select • esc quit"))
	}
	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}

func runTUILauncher() string {
	top := buildTopLevel()
	m := menuModel{
		stack:  []menuLevel{top},
		width:  80,
		height: 24,
	}
	m.cursor = m.firstVisible()

	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		return ""
	}

	final, ok := result.(menuModel)
	if !ok {
		return ""
	}
	if len(final.chosen) == 0 {
		return ""
	}
	return strings.Join(final.chosen, " ")
}
