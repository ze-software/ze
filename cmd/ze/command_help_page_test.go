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

// VALIDATES: AC-4 -- the operator's per-command page carries the node's two
// declared help texts, the summary on the header line and the long explanation
// in the body, and the child rows carry their own summaries.
// PREVENTS: the long explanation reaching the one-line header, which is what
// shipped while the whole description was assigned to Page.Summary.
func TestHelpPageCarriesBothDeclaredHelpTexts(t *testing.T) {
	node := &command.Node{
		Name:        "bgp",
		Description: "Inspect the BGP protocol engine.",
		LongHelp:    "One subtree per session.\nEach answers over the negotiated families.",
		Children: map[string]*command.Node{
			"rib":  {Name: "rib", Description: "Show the BGP RIB."},
			"peer": {Name: "peer", Description: "Show the configured peers."},
		},
	}
	page := commandHelpPage([]string{"show", "bgp"}, node)

	if page.Summary != "Inspect the BGP protocol engine." {
		t.Errorf("the header summary is %q", page.Summary)
	}
	if page.LongHelp != "One subtree per session.\nEach answers over the negotiated families." {
		t.Errorf("the body help is %q", page.LongHelp)
	}
	if strings.Contains(page.Summary, "\n") {
		t.Errorf("the one-line header carries a newline: %q", page.Summary)
	}

	var rendered strings.Builder
	page.WriteTo(&rendered, false)
	for _, want := range []string{
		"Inspect the BGP protocol engine.",
		"  One subtree per session.",
		"  Each answers over the negotiated families.",
		"Show the BGP RIB.",
		"Show the configured peers.",
	} {
		if !strings.Contains(rendered.String(), want) {
			t.Errorf("the rendered page is missing %q:\n%s", want, rendered.String())
		}
	}

	// The order is the summary, then the long explanation, then the children.
	// A parent that printed only its children left its own authored text
	// unreachable to the operator who asked about that exact path.
	out := rendered.String()
	summary := strings.Index(out, "Inspect the BGP protocol engine.")
	long := strings.Index(out, "One subtree per session.")
	children := strings.Index(out, "Show the configured peers.")
	if summary > long {
		t.Errorf("the long help came before the summary (summary=%d help=%d)\n%s", summary, long, out)
	}
	if long > children {
		t.Errorf("a child summary came before the node's own help (help=%d children=%d)\n%s", long, children, out)
	}
}
