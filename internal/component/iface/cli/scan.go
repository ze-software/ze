// Design: docs/features/interfaces.md -- interface discovery CLI

package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/component/command"
	ifacepkg "github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/helpfmt"
)

// cmdScan walks the OS for network interfaces, classifies each by Ze type, and
// prints the result.
//
// The scan answers with DATA. register.go serves that answer as
// `show interface scan`, so `| json`, `| yaml` and `| table` are three
// renderings of ONE payload, and this handler prints the same payload through
// the same renderer (ai/rules/cli.md). It carried a `--json` flag and a
// `--yaml` flag before, which were two hand-written renderings of the one the
// pipe layer already offers six of.
//
// `--config` is not a rendering of the answer: it emits Ze config syntax
// through iface.EmitConfig, so an operator can pipe the result into
// `ze config edit` and adopt what the box already has.
//
// Uses the backend that Run() already loaded.
func cmdScan(args []string) int {
	fs := flag.NewFlagSet("ze interface scan", flag.ContinueOnError)
	configOutput := fs.Bool("config", false, "Output as Ze config syntax (same format as ze init writes)")
	managedOnly := fs.Bool("managed", false, "Only show interface kinds Ze can create/delete (dummy, veth, bridge, tunnel, wireguard) -- hides ethernet and loopback")
	fs.Usage = func() {
		p := helpfmt.Page{
			Command: "ze interface scan",
			Summary: "Scan the OS for network interfaces and classify them by Ze type",
			Usage:   []string{"ze interface scan [options]"},
			Sections: []helpfmt.HelpSection{
				{Title: helpSectionOptions, Entries: []helpfmt.HelpEntry{
					{Name: "--config", Desc: "Emit Ze config syntax (same format as ze init)"},
					{Name: "--managed", Desc: "Only show Ze-managed kinds (dummy, veth, bridge, tunnel, wireguard)"},
				}},
			},
			Examples: []string{
				"ze interface scan",
				"ze interface scan --config",
				"ze interface scan --managed",
				`ze cli -c "show interface scan | json"`,
				`ze cli -c "show interface scan | yaml"`,
			},
		}
		p.WriteErr()
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	discovered, err := ifacepkg.DiscoverInterfaces()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	if *managedOnly {
		discovered = filterManaged(discovered)
	}

	if *configOutput {
		if _, err := fmt.Print(ifacepkg.EmitConfig(discovered)); err != nil {
			return 1
		}
		return 0
	}

	return command.RenderLocalAnswer(cmdPathShowInterfaceScan, scanAnswer(discovered))
}

// dataScan answers `show interface scan` with every interface the OS carries,
// classified by Ze type.
//
// It loads the backend itself: the registry reaches this handler directly,
// while the `ze interface scan` spelling arrives through Run, which has
// already loaded one.
//
// It takes no arguments. The answer is every interface, and a reader who wants
// a subset narrows it with `| match <text>` or `| display <fields>`.
func dataScan(_ []string) (any, int) {
	if err := ifacepkg.LoadBackend(backendNetlink); err != nil {
		fmt.Fprintln(os.Stderr, "error: load netlink backend:", err)
		return nil, 1
	}
	defer func() {
		if err := ifacepkg.CloseBackend(); err != nil {
			fmt.Fprintln(os.Stderr, "warning: close netlink backend:", err)
		}
	}()

	discovered, err := ifacepkg.DiscoverInterfaces()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return nil, 1
	}
	return scanAnswer(discovered), 0
}

// scanAnswer is the answer both spellings carry: the rows, in the type the
// daemon's own `show interface scan` handler answers with
// (internal/component/iface/cmd/show_interface.go, handleShowInterfaceScan),
// so the local answer and the daemon answer are the same shape rather than two
// shapes one command can produce.
func scanAnswer(discovered []ifacepkg.DiscoveredInterface) plugin.Slice[ifacepkg.DiscoveredInterface] {
	return plugin.Slice[ifacepkg.DiscoveredInterface](discovered)
}

// filterManaged drops interface kinds Ze does not create or delete
// (ethernet, loopback), leaving only the kinds an operator can
// meaningfully configure through Ze: dummy, veth, bridge, tunnel,
// wireguard.
func filterManaged(discovered []ifacepkg.DiscoveredInterface) []ifacepkg.DiscoveredInterface {
	filtered := make([]ifacepkg.DiscoveredInterface, 0, len(discovered))
	for i := range discovered {
		switch discovered[i].Type {
		case ifaceTypeDummy, ifaceTypeVeth, ifaceTypeBridge, "tunnel", "wireguard", "xfrm":
			filtered = append(filtered, discovered[i])
		}
	}
	return filtered
}
