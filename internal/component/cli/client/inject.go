// Design: docs/architecture/cli/command-completion.md -- plugin command completion injection

package client

import (
	"strings"

	cmd "github.com/ze-software/ze/internal/component/command"
)

// commandEntry matches the anonymous struct used in buildRuntimeTree and
// buildRuntimeTreeFromDispatch to parse the system command list response.
type commandEntry struct {
	Value string `json:"value"`
	// Help is the one-line SUMMARY of the command, which every surface that
	// shows it on one line reads.
	Help string `json:"help"`
	// LongHelp is the explanation the command's own help page prints, and the
	// `?` key answers. Empty means the command declares none, and no one-line
	// surface reads it at all.
	LongHelp string `json:"long-help"`
	Hidden   bool   `json:"hidden"`
}

// injectPluginCommands adds plugin-registered commands to the completion tree.
// It merges them the way every daemon-side tree does, through
// command.MergeCommandPaths. A command therefore enters the client's tree as it
// enters the SSH and web trees. An existing node is never modified. Each of the two
// texts is written only on a leaf the merge creates, or on one that holds
// nothing in THAT field. A hidden command is skipped, so it never surfaces in
// tab-completion.
//
// The names cross here. The daemon's answer spells the summary "help" and the
// explanation "long-help". The command package spells them Description and
// Help.
func injectPluginCommands(tree *cmd.Node, commands []commandEntry, hidden map[string]bool) {
	entries := make([]cmd.CommandEntry, 0, len(commands))
	for _, c := range commands {
		if hidden[strings.ToLower(c.Value)] {
			continue
		}
		entries = append(entries, cmd.CommandEntry{
			Name:        c.Value,
			Description: c.Help,
			Help:        c.LongHelp,
		})
	}
	cmd.MergeCommandPaths(tree, entries)
}
