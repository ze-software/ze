// Design: docs/architecture/cli/tui-launcher.md -- the drill-down menu ze and le share
// Related: prompt.go -- the one-value prompt the same programs ask through

// Package menu is the drill-down terminal menu a program opens when it is run
// with no arguments at a terminal. It holds the model only: the items, the
// navigation stack, scrolling, filtering and the styles. What the first screen
// lists and what runs when a command is chosen belong to the program, so ze
// builds its first screen from its verbs and root sections, and le from its
// derived command tree, and each runs the chosen line its own way.
package menu

import (
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// Item is one line of a menu level. A Header is a section title nobody can
// select. A Terminal item ends the walk: choosing it answers its Path. Any
// other item opens the level of its Path in the menu's tree.
type Item struct {
	Name     string
	Desc     string
	Path     []string
	Terminal bool
	Header   bool
}

// Level is one screen: its breadcrumb Title, the long Description of the node
// it shows (printed above the items, wrapped to the terminal; empty prints
// nothing), and its items, in display order.
type Level struct {
	Title       string
	Description string
	Items       []Item
}

// Model is the menu's state. It is a value: every key answers a new Model, so
// it is not shared between goroutines and needs no lock.
type Model struct {
	title    string        // the program name the first breadcrumb shows
	stack    []Level       // the levels walked, the first screen at the bottom
	tree     *command.Node // the tree every drilled level is built from
	levelOf  func(*command.Node, []string) Level
	filter   string
	cursor   int
	offset   int
	width    int
	height   int
	chosen   []string
	quitting bool
}

// New answers the menu at its first screen, top, titled with the program name
// title. Every level below it is LevelOf(tree, path), so a tree with no node at
// a path opens an empty level rather than a guess.
func New(title string, top Level, tree *command.Node) Model {
	m := Model{title: title, stack: []Level{top}, tree: tree, levelOf: LevelOf, width: 80, height: 24}
	m.cursor = m.firstVisible()
	return m
}

// NewDeclared answers the menu over a tree whose every node declares its text:
// a ShortHelp of one line, shown whole in the right column, and a Description
// printed above the level the reader enters (DeclaredLevelOf). le's tree is
// one; ze's YANG help is not, so ze opens New and keeps its derived lines.
func NewDeclared(title string, tree *command.Node) Model {
	m := Model{title: title, stack: []Level{DeclaredLevelOf(tree, nil)}, tree: tree, levelOf: DeclaredLevelOf, width: 80, height: 24}
	m.cursor = m.firstVisible()
	return m
}

// DeclaredLevelOf answers the screen for path in a tree that declares its
// text: each item's Desc is the child's ShortHelp whole, and the level carries
// the entered node's Description.
func DeclaredLevelOf(tree *command.Node, path []string) Level {
	level := LevelOf(tree, path)
	node := command.FindNode(tree, path)
	if node == nil {
		return level
	}
	level.Description = node.Description
	for i := range level.Items {
		level.Items[i].Desc = node.Children[level.Items[i].Name].ShortHelp
	}
	return level
}

// Chosen answers the path of the terminal item the reader chose, and nil when
// they quit without choosing.
func (m Model) Chosen() []string {
	return m.chosen
}

// Run runs the menu on the process terminal and answers the chosen path, nil
// when the reader quit. The error is the terminal program's failure to run.
func Run(m Model) ([]string, error) {
	result, err := tea.NewProgram(m).Run()
	if err != nil {
		return nil, fmt.Errorf("menu: %w", err)
	}
	final, ok := result.(Model)
	if !ok {
		return nil, fmt.Errorf("menu: the terminal program answered a %T, not the menu", result)
	}
	return final.Chosen(), nil
}

// menuDescMaxLen is the longest description a menu line shows.
const menuDescMaxLen = 50

// TruncateDesc answers the first sentence of s, or its first menuDescMaxLen
// bytes, whichever is shorter.
func TruncateDesc(s string) string {
	if i := strings.IndexAny(s, ".;"); i > 0 && i < menuDescMaxLen {
		return s[:i]
	}
	if len(s) <= menuDescMaxLen {
		return s
	}
	return s[:menuDescMaxLen]
}

// LevelOf answers the screen for path in tree: one item per child, sorted by
// name, terminal when the child has no children of its own.
func LevelOf(tree *command.Node, path []string) Level {
	node := command.FindNode(tree, path)
	if node == nil {
		return Level{Title: textbuf.Join(path, " "), Items: nil}
	}
	if len(node.Children) == 0 {
		return Level{Title: textbuf.Join(path, " "), Items: nil}
	}

	names := make([]string, 0, len(node.Children))
	for name := range node.Children {
		names = append(names, name)
	}
	slices.Sort(names)

	items := make([]Item, 0, len(names))
	for _, name := range names {
		child := node.Children[name]
		childPath := make([]string, len(path)+1)
		copy(childPath, path)
		childPath[len(path)] = name

		items = append(items, Item{
			Name:     name,
			Desc:     TruncateDesc(child.ShortHelp),
			Path:     childPath,
			Terminal: len(child.Children) == 0,
		})
	}

	return Level{Title: textbuf.Join(path, " "), Items: items}
}

func (m Model) currentLevel() Level {
	if len(m.stack) == 0 {
		return Level{}
	}
	return m.stack[len(m.stack)-1]
}

func (m Model) totalItems() int {
	return len(m.currentLevel().Items)
}

func (m Model) isVisible(item Item) bool {
	if item.Header {
		return m.filter == ""
	}
	if m.filter == "" {
		return true
	}
	return strings.HasPrefix(item.Name, m.filter)
}

func (m Model) nextVisible(from, dir int) int {
	items := m.currentLevel().Items
	n := len(items)
	if n == 0 {
		return 0
	}
	pos := from
	for range n {
		pos = (pos + dir + n) % n
		if m.isVisible(items[pos]) && !items[pos].Header {
			return pos
		}
	}
	return from
}

func (m Model) firstVisible() int {
	for i, item := range m.currentLevel().Items {
		if m.isVisible(item) && !item.Header {
			return i
		}
	}
	return 0
}

func (m Model) hasVisibleItems() bool {
	for _, item := range m.currentLevel().Items {
		if m.isVisible(item) && !item.Header {
			return true
		}
	}
	return false
}

func (m Model) visibleCount() int {
	return max(m.height-6, 3)
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (m Model) handleKey(km tea.KeyPressMsg) (tea.Model, tea.Cmd) { //nolint:cyclop // flat switch is clearer than extraction
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
		item := m.currentLevel().Items[m.cursor]
		if item.Header {
			return m, nil
		}
		if item.Terminal {
			m.chosen = item.Path
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

	// q quits from any level, but only while no filter is typed: once the
	// reader is filtering, q is a letter of the name they are typing.
	case km.Text == "q" && m.filter == "":
		m.quitting = true
		return m, tea.Quit

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

func (m Model) drillInto(item Item) Model {
	level := m.levelOf(m.tree, item.Path)
	m.stack = append(m.stack, level)
	m.offset = 0
	m.cursor = m.firstVisible()
	return m
}

func (m Model) cursorVisibleIndex() int {
	vi := 0
	for _, item := range m.currentLevel().Items[:m.cursor] {
		if m.isVisible(item) {
			vi++
		}
	}
	return vi
}

func (m Model) adjustScroll() Model {
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

func (m Model) View() tea.View {
	if m.quitting {
		return tea.NewView("")
	}

	level := m.currentLevel()
	var b textbuf.Buffer
	b.Reset()

	b.Str(menuTitleStyle.Render(m.title))
	if len(m.stack) > 1 {
		for i := 1; i < len(m.stack); i++ {
			b.Str(" > ")
			b.Str(menuBreadcrumb.Render(m.stack[i].Title))
		}
	}
	b.Str("\n\n")
	// The entered node's long text heads its options, so a reader learns what
	// the level is for before choosing in it. ze's levels carry none.
	if level.Description != "" {
		for _, line := range helpfmt.Wrap(level.Description, m.width-2, 2) {
			b.Str(menuDescStyle.Render(line))
			b.Byte('\n')
		}
		b.Byte('\n')
	}

	var visible []int
	for i, item := range level.Items {
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
		item := level.Items[visible[vi]]

		if item.Header {
			if vi > m.offset {
				b.Byte('\n')
			}
			b.Str(menuSectionStyle.Render(item.Name))
			b.Byte('\n')
			continue
		}

		tb.Reset()
		tb.PadRight(item.Name, menuNameColumnWidth)
		paddedName := tb.String()

		desc := item.Desc
		if len(desc) > descBudget {
			tb.Reset()
			tb.Str(desc[:descBudget-3]).Str("...")
			desc = tb.String()
		}

		arrow := ""
		if !item.Terminal {
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
