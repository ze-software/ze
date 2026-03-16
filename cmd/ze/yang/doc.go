// Design: docs/architecture/config/yang-config-design.md -- command documentation
// Related: tree.go -- RPC doc extraction

package yang

import (
	"fmt"
	"io"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

// FormatDocCommand writes documentation for a specific command.
func FormatDocCommand(w io.Writer, cliCommand string) error {
	docs, err := AllRPCDocs()
	if err != nil {
		return err
	}

	// Match by CLI command (case-insensitive).
	for _, d := range docs {
		if strings.EqualFold(d.CLICommand, cliCommand) {
			return writeDocEntry(w, d)
		}
	}

	return fmt.Errorf("unknown command: %s", cliCommand)
}

// FormatDocList writes a list of all commands with descriptions.
func FormatDocList(w io.Writer) error {
	docs, err := AllRPCDocs()
	if err != nil {
		return err
	}
	if len(docs) == 0 {
		_, err := fmt.Fprintln(w, "No commands registered.")
		return err
	}

	if _, err := fmt.Fprintf(w, "%-40s %-6s %s\n", "Command", "Mode", "Description"); err != nil {
		return err
	}
	for _, d := range docs {
		mode := "rw"
		if d.ReadOnly {
			mode = "ro"
		}
		if _, err := fmt.Fprintf(w, "%-40s %-6s %s\n", d.CLICommand, mode, d.Help); err != nil {
			return err
		}
	}
	return nil
}

func writeDocEntry(w io.Writer, d RPCDoc) error {
	mode := "read-write"
	if d.ReadOnly {
		mode = "read-only"
	}

	if _, err := fmt.Fprintf(w, "%s\n  %s (%s)\n", d.CLICommand, d.Help, mode); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "\n  Wire method: %s\n", d.WireMethod); err != nil {
		return err
	}

	if len(d.Input) > 0 {
		if _, err := fmt.Fprintf(w, "\n  Parameters (input):\n"); err != nil {
			return err
		}
		if err := writeLeafTable(w, d.Input); err != nil {
			return err
		}
	}

	if len(d.Output) > 0 {
		if _, err := fmt.Fprintf(w, "\n  Parameters (output):\n"); err != nil {
			return err
		}
		if err := writeLeafTable(w, d.Output); err != nil {
			return err
		}
	}

	return nil
}

// writeLeafTable writes a formatted table of YANG leaf parameters.
func writeLeafTable(w io.Writer, leaves []yang.LeafMeta) error {
	for _, leaf := range leaves {
		mandatory := ""
		if leaf.Mandatory {
			mandatory = " [mandatory]"
		}
		desc := leaf.Description
		if len(desc) > 50 {
			desc = desc[:47] + "..."
		}
		if _, err := fmt.Fprintf(w, "    %-20s %-14s%s  %s\n", leaf.Name, leaf.Type, mandatory, desc); err != nil {
			return err
		}
	}
	return nil
}
