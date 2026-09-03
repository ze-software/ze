// Design: docs/architecture/cli/command-completion.md -- plugin command completion injection

package client

import (
	"encoding/json"
	"fmt"
	"strings"

	cmd "github.com/ze-software/ze/internal/component/command"
)

// summaryKey is the JSON key the "system command list" answer publishes a
// command's one-line summary under, and retiredSummaryKey is the key it used
// until 2026-09-03. Both are spelled here because a refusal has to name them,
// and the commandEntry tags below are the declaration they repeat.
const (
	summaryKey        = "description"
	retiredSummaryKey = "help"
)

// commandEntry is one row of the "system command list" answer.
type commandEntry struct {
	Value string `json:"value"`
	// Description is the one-line SUMMARY of the command, which every surface that
	// shows it on one line reads.
	Description string `json:"description"`
	// LongHelp is the explanation the command's own help page prints, and the
	// `?` key answers. Empty means the command declares none, and no one-line
	// surface reads it at all.
	LongHelp string `json:"long-help"`
	Hidden   bool   `json:"hidden"`
	// RetiredHelp holds the retired summary key when a row still carries it.
	// decodeCommandList refuses such an answer, because decoding it would leave
	// Description empty and no reader can tell that from a command that states
	// no summary (ai/rules/principles.md).
	//
	// It is json.RawMessage rather than string so a `"help": ""` is detected
	// too: the field is nil only when the key is absent. Nothing reads its
	// bytes.
	RetiredHelp json.RawMessage `json:"help"`
}

// decodeCommandList parses the "system command list" answer into its rows.
//
// It refuses an answer whose rows carry the retired summary key rather than
// handing back rows whose Description is empty. The daemon and the CLI ship
// together, so a retired key means the two disagree about the wire, and that
// disagreement is reported instead of rendering every command with no summary.
func decodeCommandList(payload []byte) ([]commandEntry, error) {
	var data struct {
		Commands []commandEntry `json:"commands"`
	}
	if err := json.Unmarshal(payload, &data); err != nil {
		return nil, err
	}
	for i := range data.Commands {
		if data.Commands[i].RetiredHelp == nil {
			continue
		}
		return nil, fmt.Errorf("command list row %q carries the retired key %q; the summary is published under %q",
			data.Commands[i].Value, retiredSummaryKey, summaryKey)
	}
	return data.Commands, nil
}

// injectPluginCommands adds plugin-registered commands to the completion tree.
// It merges them the way every daemon-side tree does, through
// command.MergeCommandPaths. A command therefore enters the client's tree as it
// enters the SSH and web trees. An existing node is never modified. Each of the two
// texts is written only on a leaf the merge creates, or on one that holds
// nothing in THAT field. A hidden command is skipped, so it never surfaces in
// tab-completion.
func injectPluginCommands(tree *cmd.Node, commands []commandEntry, hidden map[string]bool) {
	entries := make([]cmd.CommandEntry, 0, len(commands))
	for _, c := range commands {
		if hidden[strings.ToLower(c.Value)] {
			continue
		}
		entries = append(entries, cmd.CommandEntry{
			Name:        c.Value,
			Description: c.Description,
			LongHelp:    c.LongHelp,
		})
	}
	cmd.MergeCommandPaths(tree, entries)
}
