// Design: docs/architecture/cli/tui-launcher.md -- interactive no-arg launcher
// Related: ../../internal/component/cli/menu/menu.go -- the menu model ze and le share

//go:build ze_core

package main

import (
	"os"
	"sort"
	"strings"

	"golang.org/x/term"

	cli "github.com/ze-software/ze/internal/component/cli/client"
	"github.com/ze-software/ze/internal/component/cli/menu"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
)

var stdinIsTerminal = func() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// buildTopLevel builds the launcher's first screen from verbTree, the tree
// cli.BuildCommandTree answers.
func buildTopLevel(verbTree *command.Node) menu.Level {
	cmdEntries := command.HelpEntries(verbTree, nil)
	yangItems := make([]menu.Item, 0, len(cmdEntries))
	for _, e := range cmdEntries {
		node := command.FindNode(verbTree, []string{e.Name})
		hasChildren := node != nil && len(node.Children) > 0
		yangItems = append(yangItems, menu.Item{
			Name:     e.Name,
			Desc:     menu.TruncateDesc(e.Desc),
			Path:     []string{e.Name},
			Terminal: !hasChildren,
		})
	}

	var items []menu.Item
	for _, se := range registry.ListRootBySection() {
		title := registry.SectionTitle(se.Section)
		if title == "" {
			title = se.Section
		}
		items = append(items, menu.Item{Name: title, Header: true})

		cmdItems := make([]menu.Item, 0, len(se.Commands))
		for _, rc := range se.Commands {
			cmdItems = append(cmdItems, menu.Item{
				Name:     rc.Name,
				Desc:     menu.TruncateDesc(rc.Meta.ShortHelp),
				Path:     []string{rc.Name},
				Terminal: true,
			})
		}

		if se.Section == registry.SectionOperations {
			cmdItems = append(cmdItems, yangItems...)
			sort.Slice(cmdItems, func(i, j int) bool { return cmdItems[i].Name < cmdItems[j].Name })
		}

		items = append(items, cmdItems...)
	}
	return menu.Level{Title: "ze", Items: items}
}

// newZeMenu answers the launcher's model at its first screen, top, over the
// verb tree each drilled level is built from.
func newZeMenu(top menu.Level, tree *command.Node) menu.Model {
	return menu.New("ze", top, tree)
}

// runTUILauncher runs the menu and answers the chosen command line, empty when
// none was chosen. The error is the YANG loader's refusal of the schema the
// menu is built from, or the terminal program's failure to run.
func runTUILauncher() (string, error) {
	tree, err := cli.BuildCommandTree(false)
	if err != nil {
		return "", err
	}
	chosen, err := menu.Run(newZeMenu(buildTopLevel(tree), tree))
	if err != nil {
		return "", err
	}
	return strings.Join(chosen, " "), nil
}
