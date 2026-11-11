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
	summaryKey        = "short-help"
	retiredSummaryKey = "help"
)

// commandEntry is one row of the "system command list" answer.
type commandEntry struct {
	Value string `json:"value"`
	// ShortHelp is the one-line SUMMARY of the command, which every surface that
	// shows it on one line reads.
	ShortHelp string `json:"short-help"`
	// Description is the explanation the command's own help page prints, and the
	// `?` key answers. Empty means the command declares none, and no one-line
	// surface reads it at all.
	Description string `json:"description"`
	Hidden      bool   `json:"hidden"`
	// Completable says the daemon completes this command's argument from its
	// runtime state (wireDaemonArgCompletion).
	Completable bool `json:"completable"`
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

// daemonQuery sends one command line to the daemon and answers its raw JSON.
type daemonQuery func(line string) (string, error)

// dispatchQuery adapts a dispatch function to a daemonQuery, releasing the
// answer's transport once its text is read.
func dispatchQuery(dispatch CommandFunc) daemonQuery {
	return func(line string) (string, error) {
		output, err := dispatch(line)
		if output.TransportComplete != nil {
			defer output.TransportComplete()
		}
		return output.Text, err
	}
}

// wireDaemonArgCompletion hangs a value hint on the node of every command the
// daemon marks completable, so Tab after `request plugin restart` offers what
// the running daemon holds. The hint asks `system command complete args
// "<cmd>"` at each Tab and lists every candidate; the tree completer matches
// the typed prefix. A daemon that does not answer offers nothing, because a
// completion is best-effort and a candidate guessed here would be wrong.
func wireDaemonArgCompletion(tree *cmd.Node, commands []commandEntry, query daemonQuery) {
	for i := range commands {
		if !commands[i].Completable {
			continue
		}
		node := cmd.FindNode(tree, strings.Fields(commands[i].Value))
		if node == nil {
			continue
		}
		line := `system command complete args "` + commands[i].Value + `"`
		node.ValueHints = func() []cmd.Suggestion {
			return fetchArgCompletions(query, line)
		}
	}
}

// fetchArgCompletions runs one `system command complete args` line and
// converts its candidates to value suggestions.
func fetchArgCompletions(query daemonQuery, line string) []cmd.Suggestion {
	output, err := query(line)
	// refuse-by-default: guards wireDaemonArgCompletion -- an unreachable daemon offers no candidate, never a guessed one
	if err != nil {
		return nil
	}
	var data struct {
		Completions []struct {
			Value     string `json:"value"`
			ShortHelp string `json:"short-help"`
		} `json:"completions"`
	}
	// refuse-by-default: guards wireDaemonArgCompletion -- an answer that does not decode offers no candidate
	if json.Unmarshal([]byte(output), &data) != nil {
		return nil
	}
	suggestions := make([]cmd.Suggestion, 0, len(data.Completions))
	for _, candidate := range data.Completions {
		suggestions = append(suggestions, cmd.Suggestion{
			Text:        candidate.Value,
			Description: candidate.ShortHelp,
			Type:        cmd.SuggestionValue,
		})
	}
	return suggestions
}

// injectPluginCommands adds plugin-registered commands to the completion tree.
// It merges them the way every daemon-side tree does, through
// command.MergeCommandPaths. A command therefore enters the client's tree as it
// enters the SSH and web trees. An existing node is never modified. Each of the two
// texts is written only on a leaf the merge creates, or on one that holds
// nothing in THAT field. A command the surface predicate does not list (a
// hidden one) is skipped, so it never surfaces in tab-completion.
func injectPluginCommands(tree *cmd.Node, commands []commandEntry, hidden map[string]bool) {
	entries := make([]cmd.CommandEntry, 0, len(commands))
	for _, c := range commands {
		// The surface predicate decides, as it does for the daemon's trees.
		listed, err := cmd.ListedOnDaemonSurfaces(cmd.CommandRegistration{
			Path: c.Value, Kind: cmd.KindPlugin, Hidden: hidden[strings.ToLower(c.Value)],
		})
		if err != nil {
			// A plugin registration always has a kind and every daemon surface
			// answers it alike, so only a defect in the predicate lands here.
			panic("BUG: injectPluginCommands: " + err.Error())
		}
		if !listed {
			continue
		}
		entries = append(entries, cmd.CommandEntry{
			Name:        c.Value,
			ShortHelp:   c.ShortHelp,
			Description: c.Description,
		})
	}
	cmd.MergeCommandPaths(tree, entries)
}
