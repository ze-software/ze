// Design: docs/architecture/api/process-protocol.md — plugin SDK methods
// Related: completer.go — YANG-driven config editor completion
// Related: completer_command.go — operational command completion

package cli

import (
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// pluginCompleter provides tab completion for plugin SDK methods.
// Used by `ze bgp plugin cli` interactive mode after 5-stage negotiation.
type pluginCompleter struct {
	methods []pluginMethod
}

type pluginMethod struct {
	name string
	help string
	args string // argument pattern hint
}

// pluginSDKMethods lists all plugin SDK methods available after stage 5.
// argHex is the argument list of a method that decodes one hex-encoded blob.
const argHex = "<hex>"

var pluginSDKMethods = []pluginMethod{
	{name: "update-route", help: "Inject route update to matching peers", args: "<selector> <command>"},
	{name: "dispatch-command", help: "Dispatch command through engine", args: "<command>"},
	{name: "subscribe-events", help: "Subscribe to engine events", args: "[events...] [peers...] [format]"},
	{name: "unsubscribe-events", help: "Unsubscribe from engine events", args: ""},
	{name: "decode-nlri", help: "Decode NLRI from hex", args: "<family> <hex>"},
	{name: "encode-nlri", help: "Encode NLRI from args", args: "<family> <args...>"},
	{name: "decode-mp-reach", help: "Decode MP_REACH_NLRI from hex", args: argHex},
	{name: "decode-mp-unreach", help: "Decode MP_UNREACH_NLRI from hex", args: argHex},
	{name: "decode-update", help: "Decode full UPDATE from hex", args: argHex},
	{name: "bye", help: "Signal clean shutdown", args: "[reason]"},
}

// newPluginCompleter creates a completer for plugin SDK methods.
func newPluginCompleter() *pluginCompleter {
	return &pluginCompleter{methods: pluginSDKMethods}
}

// Complete returns completions for the given input.
func (c *pluginCompleter) Complete(input string) []Completion {
	if input == "" {
		comps := make([]Completion, len(c.methods))
		for i, m := range c.methods {
			comps[i] = Completion{Text: m.name, Description: m.help}
		}
		return comps
	}

	var comps []Completion
	for _, m := range c.methods {
		if len(input) <= len(m.name) && m.name[:len(input)] == input {
			comps = append(comps, Completion{Text: m.name, Description: m.help})
		} else if len(input) > len(m.name) && input[:len(m.name)] == m.name && input[len(m.name)] == ' ' {
			// Already typed the method name — show argument hint
			if m.args != "" {
				comps = append(comps, Completion{Text: m.args, Description: m.help, Type: completionHint})
			}
			return comps
		}
	}
	return comps
}

// GhostText returns the best single completion for inline display.
func (c *pluginCompleter) GhostText(input string) string {
	if input == "" {
		return ""
	}
	for _, m := range c.methods {
		if len(input) < len(m.name) && m.name[:len(input)] == input {
			return m.name[len(input):]
		}
	}
	return ""
}

// Explain returns what the named method declares. The argument pattern is on
// the first line and the help is on the second.
//
// It answers false for a name the SDK does not carry. Both halves come from the
// method's own declaration, so an unknown name leaves nothing to read.
func (c *pluginCompleter) Explain(input string) (string, bool) {
	name := strings.TrimSpace(input)
	if name == "" {
		return "", false
	}

	for _, m := range c.methods {
		if m.name != name {
			continue
		}
		var tb textbuf.Buffer
		tb.Str(m.name)
		if m.args != "" {
			tb.Byte(' ').Str(m.args)
		}
		tb.Byte('\n').Str(m.help)
		return tb.String(), true
	}
	return "", false
}
