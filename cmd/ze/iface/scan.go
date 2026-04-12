// Design: docs/features/interfaces.md -- interface discovery CLI

package iface

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/helpfmt"
	"codeberg.org/thomas-mangin/ze/internal/component/command"
	ifacepkg "codeberg.org/thomas-mangin/ze/internal/component/iface"
)

// cmdScan walks the OS for network interfaces, classifies each by Ze type,
// and renders the result in the chosen output format. The default is a
// nushell-style table via command.ApplyTable; --json emits raw JSON for
// programmatic consumers; --yaml emits YAML via command.RenderYAML; and
// --config emits Ze config syntax via iface.EmitConfig so operators can
// pipe the result into `ze config edit` to adopt discovered interfaces.
//
// Output formatting routes through the same pipe framework that handles
// daemon-dispatched verb output, so the table/yaml renderings are
// identical to what operators see for `ze show ...` commands.
//
// Uses the backend that Run() already loaded, so this handler is purely
// about discovery + output selection.
func cmdScan(args []string) int {
	fs := flag.NewFlagSet("ze interface scan", flag.ContinueOnError)
	jsonOutput := fs.Bool("json", false, "Output as raw JSON (programmatic default)")
	yamlOutput := fs.Bool("yaml", false, "Output as YAML")
	configOutput := fs.Bool("config", false, "Output as Ze config syntax (same format as ze init writes)")
	managedOnly := fs.Bool("managed", false, "Only show interface kinds Ze can create/delete (dummy, veth, bridge, tunnel, wireguard) -- hides ethernet and loopback")
	fs.Usage = func() {
		p := helpfmt.Page{
			Command: "ze interface scan",
			Summary: "Scan the OS for network interfaces and classify them by Ze type",
			Usage:   []string{"ze interface scan [options]"},
			Sections: []helpfmt.HelpSection{
				{Title: "Options", Entries: []helpfmt.HelpEntry{
					{Name: "--config", Desc: "Emit Ze config syntax (same format as ze init)"},
					{Name: "--json", Desc: "Emit raw JSON"},
					{Name: "--yaml", Desc: "Emit YAML"},
					{Name: "--managed", Desc: "Only show Ze-managed kinds (dummy, veth, bridge, tunnel, wireguard)"},
				}},
			},
			Examples: []string{
				"ze interface scan",
				"ze interface scan --config",
				"ze interface scan --json",
				"ze interface scan --yaml",
				"ze interface scan --managed",
			},
		}
		p.Write()
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	if err := validateScanFlags(*jsonOutput, *yamlOutput, *configOutput); err != nil {
		if _, werr := fmt.Fprintln(os.Stderr, "error:", err); werr != nil {
			return 1
		}
		return 1
	}

	discovered, err := ifacepkg.DiscoverInterfaces()
	if err != nil {
		if _, werr := fmt.Fprintf(os.Stderr, "error: %v\n", err); werr != nil {
			return 1
		}
		return 1
	}

	if *managedOnly {
		discovered = filterManaged(discovered)
	}

	return renderScan(discovered, *jsonOutput, *yamlOutput, *configOutput)
}

// validateScanFlags rejects mutually exclusive format selections.
func validateScanFlags(jsonOut, yamlOut, configOut bool) error {
	n := 0
	for _, b := range []bool{jsonOut, yamlOut, configOut} {
		if b {
			n++
		}
	}
	if n > 1 {
		return fmt.Errorf("at most one of --json, --yaml, --config may be set")
	}
	return nil
}

// filterManaged drops interface kinds Ze does not create or delete
// (ethernet, loopback), leaving only the kinds an operator can
// meaningfully configure through Ze: dummy, veth, bridge, tunnel,
// wireguard.
func filterManaged(discovered []ifacepkg.DiscoveredInterface) []ifacepkg.DiscoveredInterface {
	filtered := make([]ifacepkg.DiscoveredInterface, 0, len(discovered))
	for i := range discovered {
		switch discovered[i].Type {
		case "dummy", "veth", "bridge", "tunnel", "wireguard": //nolint:goconst // CLI dispatch strings
			filtered = append(filtered, discovered[i])
		}
	}
	return filtered
}

// renderScan selects the output format and writes the result to stdout.
// Default (no flag) goes through the shared pipe table renderer so the
// output matches `ze show ...` verb rendering conventions.
func renderScan(discovered []ifacepkg.DiscoveredInterface, jsonOut, yamlOut, configOut bool) int {
	if configOut {
		if _, err := fmt.Print(ifacepkg.EmitConfig(discovered)); err != nil {
			return 1
		}
		return 0
	}

	// Every non-config format starts from the same JSON encoding so the
	// shared pipe framework treats the output identically to any other
	// Ze command that produces a JSON array.
	raw, err := json.Marshal(discovered)
	if err != nil {
		if _, werr := fmt.Fprintf(os.Stderr, "error: marshal: %v\n", err); werr != nil {
			return 1
		}
		return 1
	}
	jsonStr := string(raw)

	switch {
	case jsonOut:
		// Programmatic default: emit JSON as-is (one line).
		if _, err := fmt.Println(jsonStr); err != nil {
			return 1
		}

	case yamlOut:
		var data any
		if err := json.Unmarshal(raw, &data); err != nil {
			if _, werr := fmt.Fprintf(os.Stderr, "error: unmarshal: %v\n", err); werr != nil {
				return 1
			}
			return 1
		}
		if _, err := fmt.Print(command.RenderYAML(data)); err != nil {
			return 1
		}

	default:
		// Human default: box-drawing table via the shared pipe renderer.
		// Identical output to `ze show interface scan` when that command
		// is added as a YANG-dispatched verb.
		if _, err := fmt.Print(command.ApplyTable(jsonStr)); err != nil {
			return 1
		}
	}

	return 0
}
