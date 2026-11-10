package cli

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/cli/contract"
)

// TestEditorHelpDerivesFromRegisteredHelp proves the editor's three help
// surfaces read one table. Method: the completion list must carry each
// command's help from editorCommands, the overlay's command block and
// EditorHelpEntries must list every form of every command, every commit
// subcommand the parser accepts must appear, and discard's help must name the
// two forms a session discard takes, not "Revert all changes".
func TestEditorHelpDerivesFromRegisteredHelp(t *testing.T) {
	if len(commands) != len(editorCommands) {
		t.Fatalf("completion lists %d commands, the table declares %d", len(commands), len(editorCommands))
	}
	for i, c := range editorCommands {
		if commands[i].Text != c.name || commands[i].ShortHelp != c.help {
			t.Errorf("completion %d = %q %q, table says %q %q", i, commands[i].Text, commands[i].ShortHelp, c.name, c.help)
		}
	}

	overlay := editorHelpCommands()
	entries := EditorHelpEntries()
	names := make(map[string]string, len(entries))
	for _, e := range entries {
		names[e.Name] = e.Desc
		if !strings.Contains(overlay, e.Name) || !strings.Contains(overlay, e.Desc) {
			t.Errorf("overlay is missing %q %q:\n%s", e.Name, e.Desc, overlay)
		}
	}
	for _, c := range editorCommands {
		for _, f := range c.forms {
			if names[f.usage] != f.help {
				t.Errorf("help entries give %q %q, table says %q", f.usage, names[f.usage], f.help)
			}
		}
		if len(c.forms) == 0 && names[c.name] != c.help {
			t.Errorf("help entries give %q %q, table says %q", c.name, names[c.name], c.help)
		}
	}

	for _, sub := range contract.CommitSubcommands() {
		usage := "commit " + sub.Keyword
		if sub.TakesSeconds {
			usage += " <seconds>"
		}
		if _, ok := names[usage]; !ok {
			t.Errorf("help names no %q, which the commit parser accepts", usage)
		}
	}

	if names["discard all"] != "Drop every pending change" {
		t.Errorf("discard all help = %q", names["discard all"])
	}
	if names["discard <path>"] != "Drop the pending changes under path" {
		t.Errorf("discard <path> help = %q", names["discard <path>"])
	}
	if strings.Contains(overlay, "Revert all changes") {
		t.Errorf("overlay still says discard reverts all changes:\n%s", overlay)
	}
}
