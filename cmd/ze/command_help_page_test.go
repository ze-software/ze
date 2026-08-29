//go:build ze_core

// The operator's per-command help page: the invocation form comes from the
// model, and a command that also has subcommands shows both.

package main

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
)

// VALIDATES: AC-5 -- a command node that also has children prints its generated
// usage line, then its child listing.
// PREVENTS: the operator of a nested command reading only "<command> [options]",
// which names no argument the command actually takes.
func TestHelpPrintsUsageForNodeWithChildren(t *testing.T) {
	node := &command.Node{
		Name:        "name",
		WireMethod:  "ze-iface:interface-create-dummy",
		Description: "Create a dummy interface.",
		ArgDefs:     []command.ArgDef{{Name: "name", Kind: command.ArgString, Mandatory: true}},
		Children: map[string]*command.Node{
			"unit":    {Name: "unit", Description: "Add a VLAN sub-interface."},
			"address": {Name: "address", Description: "Add an IP address."},
		},
	}
	page := commandHelpPage([]string{"create", "interface", "dummy", "name"}, node)

	if len(page.Usage) == 0 {
		t.Fatal("the page states no usage")
	}
	if page.Usage[0] != "ze create interface dummy name <name>" {
		t.Errorf("the first usage line reads %q", page.Usage[0])
	}
	if len(page.Usage) != 2 || !strings.HasSuffix(page.Usage[1], "<command> [options]") {
		t.Errorf("the page does not keep the navigation line: %v", page.Usage)
	}
	if len(page.Sections) != 1 || len(page.Sections[0].Entries) != 2 {
		t.Fatalf("the page lists %v", page.Sections)
	}
	if page.Summary != node.Description {
		t.Errorf("the page states the summary %q", page.Summary)
	}
}

// VALIDATES: a command with no children states only the form it accepts.
// PREVENTS: `ze show system sockets <command> [options]`, which offers
// subcommands that do not exist and names none of the three filters.
func TestHelpPrintsOneUsageForALeafCommand(t *testing.T) {
	node := &command.Node{
		Name:        "sockets",
		WireMethod:  "ze-show:system-sockets",
		Description: "Show open sockets.",
		ArgDefs: []command.ArgDef{
			{Name: "protocol", Kind: command.ArgEnum, EnumValues: []string{"tcp", "udp"}},
			{Name: "port", Kind: command.ArgUint, UintBits: 32},
		},
	}
	page := commandHelpPage([]string{"show", "system", "sockets"}, node)

	want := "ze show system sockets [protocol <tcp|udp>] [port <port>]"
	if len(page.Usage) != 1 || page.Usage[0] != want {
		t.Errorf("the page states the usage %v, want [%q]", page.Usage, want)
	}
}

// VALIDATES: a grouping node keeps the navigation line, because it runs no
// command and has no invocation form.
// PREVENTS: a usage line for a path an operator cannot execute.
func TestHelpKeepsTheNavigationLineForAGroupingNode(t *testing.T) {
	node := &command.Node{
		Name:     "interface",
		Children: map[string]*command.Node{"dummy": {Name: "dummy"}},
	}
	page := commandHelpPage([]string{"create", "interface"}, node)

	if len(page.Usage) != 1 || !strings.HasSuffix(page.Usage[0], "<command> [options]") {
		t.Errorf("a grouping node states the usage %v", page.Usage)
	}
}

// VALIDATES: an unknown path still produces a page rather than a panic.
// PREVENTS: a mistyped command taking the daemon's help path down with it.
func TestHelpPageForAnUnknownPath(t *testing.T) {
	page := commandHelpPage([]string{"show", "nonesuch"}, nil)
	if page.Command != "ze show nonesuch" || len(page.Usage) != 1 {
		t.Errorf("an unknown path produced %+v", page)
	}
}
