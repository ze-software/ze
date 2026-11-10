// Design: docs/architecture/config/yang-config-design.md — config editor
// Related: completer.go — the editor command completions derived from this table
// Related: model_render.go — the help overlay rendered from this table

package cli

import (
	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// editorHelpForm is one way to type an editor command, with what it does.
type editorHelpForm struct {
	usage string
	help  string
}

// editorCommand declares one editor command once: the word, the one-line help
// that completion shows, and the typed forms the help overlay and
// `ze config edit --help` list. A command with no forms is listed by its word
// and its one-line help. Every help surface of the editor derives from this
// table, so a command's help changes in one place.
type editorCommand struct {
	name  string
	help  string
	forms []editorHelpForm
}

// editorCommands is the editor's command table, in the order completion and
// help show it. The commit forms come from contract.CommitSubcommands, the
// grammar the commit parser reads.
var editorCommands = []editorCommand{
	{name: cmdSet, help: "Set a configuration value", forms: []editorHelpForm{
		{"set <path> <value>", "Set a configuration value"},
	}},
	{name: cmdDelete, help: "Delete a configuration value", forms: []editorHelpForm{
		{"delete <path>", "Delete a configuration value"},
	}},
	{name: cmdEdit, help: "Enter a subsection context", forms: []editorHelpForm{
		{"edit <path>", "Enter a subsection context"},
		{"edit <list> *", "Edit template for all entries"},
	}},
	{name: cmdShow, help: "Display configuration", forms: []editorHelpForm{
		{"show", "Display configuration (scrollable)"},
		{"show confirmed", "Display committed (on-disk) config"},
		{"show saved", "Display saved draft file"},
		{"show | blame", "Annotate with authorship"},
		{"show | changes [all]", "Pending changes (session or all)"},
		{"show | compare", "Diff against committed config"},
		{"show | compare saved", "Diff against saved draft"},
		{"show | compare rollback N", "Diff against backup N"},
		{"show | compare <user>", "Diff against user's changes"},
		{"show | errors", "Validation issues"},
		{"show | format config", "Display as set commands"},
		{"show | history", "List rollback revisions"},
	}},
	{name: cmdOption, help: "Display settings (columns, errors)", forms: []editorHelpForm{
		{"option <col> enable", "Enable display column (author/date/source/changes)"},
		{"option <col> disable", "Disable display column"},
		{"option all / none", "Enable/disable all columns"},
		{"option errors hints", "Toggle inline diagnostic hints"},
		{"option errors hide", "Hide error annotations"},
	}},
	{name: cmdCommit, help: "Apply, verify, accept or abort the candidate", forms: commitHelpForms()},
	{name: cmdSave, help: "Snapshot work-in-progress"},
	// In an editing session a bare discard is refused (cmdDiscardSession), so
	// the forms name the two arguments it takes.
	{name: cmdDiscard, help: "Drop pending changes: all, or those under a path", forms: []editorHelpForm{
		{"discard all", "Drop every pending change"},
		{"discard <path>", "Drop the pending changes under path"},
	}},
	{name: cmdTop, help: "Return to root context"},
	{name: cmdUp, help: "Go up one level"},
	{name: cmdRollback, help: "Restore from backup", forms: []editorHelpForm{
		{"rollback <N>", "Restore backup N"},
	}},
	{name: cmdExit, help: "Exit current mode"},
	{name: cmdHelp, help: "Show help"},
	{name: cmdRun, help: "Run operational command", forms: []editorHelpForm{
		{"run <cmd>", "Execute operational command (stays in config mode)"},
	}},
	{name: cmdWho, help: "List active editing sessions"},
	{name: cmdDisconnect, help: "Remove another session"},
	{name: cmdDeactivate, help: "Mark a config block inactive"},
	{name: cmdActivate, help: "Reactivate an inactive config block"},
	{name: cmdRename, help: "Rename a list entry", forms: []editorHelpForm{
		{"rename <p> <old> to <new>", "Rename a list entry"},
	}},
	{name: cmdCopy, help: "Copy a list entry", forms: []editorHelpForm{
		{"copy <p> <src> to <dst>", "Copy a list entry"},
	}},
	{name: cmdInsert, help: "Insert into a leaf-list at position"},
}

// commitHelpForms lists every commit subcommand the parser accepts, and its
// force form where force may follow.
func commitHelpForms() []editorHelpForm {
	subs := contract.CommitSubcommands()
	forms := make([]editorHelpForm, 0, 2*len(subs))
	for _, sub := range subs {
		var tb textbuf.Buffer
		tb.Str(cmdCommit).Byte(' ').Str(sub.Keyword)
		if sub.TakesSeconds {
			tb.Str(" <seconds>")
		}
		usage := tb.String()
		forms = append(forms, editorHelpForm{usage, sub.Help})
		if sub.Forceable {
			var force textbuf.Buffer
			forms = append(forms, editorHelpForm{
				force.Str(usage).Str(" force").String(),
				force.Reset().Str(sub.Help).Str(", despite warnings and conflicts (errors still block)").String(),
			})
		}
	}
	return forms
}

// editorCompletions is the first-word completion list, one entry per command.
func editorCompletions() []Completion {
	out := make([]Completion, 0, len(editorCommands))
	for _, c := range editorCommands {
		out = append(out, Completion{Text: c.name, ShortHelp: c.help, Type: completionCommand})
	}
	return out
}

// EditorHelpEntries lists every editor command form with its help, in table
// order, for a help page outside the editor (`ze config edit --help`).
func EditorHelpEntries() []helpfmt.HelpEntry {
	out := make([]helpfmt.HelpEntry, 0, 2*len(editorCommands))
	for _, c := range editorCommands {
		if len(c.forms) == 0 {
			out = append(out, helpfmt.HelpEntry{Name: c.name, Desc: c.help})
			continue
		}
		for _, f := range c.forms {
			out = append(out, helpfmt.HelpEntry{Name: f.usage, Desc: f.help})
		}
	}
	return out
}

// editorHelpCommands renders the "Commands:" block of the editor help overlay
// from EditorHelpEntries, the usage column padded to the widest form.
func editorHelpCommands() string {
	entries := EditorHelpEntries()
	width := 0
	for _, e := range entries {
		width = max(width, len(e.Name))
	}
	var tb textbuf.Buffer
	tb.Str("Commands:\n")
	for _, e := range entries {
		tb.Str("  ").PadRight(e.Name, width+2).Str(e.Desc).Byte('\n')
	}
	return tb.String()
}
