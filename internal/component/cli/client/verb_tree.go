// Design: docs/architecture/api/commands.md -- the verb-relative command tree
// Related: ../../../../cmd/ze/internal/cmdutil/cmdutil.go -- the only consumer
// that must translate back to absolute paths (AbsoluteVerbPath).
//
// `ze show bgp rib status` and the interactive `show bgp rib status` walk two
// different trees. This file owns the first: a tree RELATIVE to one verb, where
// the command above is `bgp rib status`. verbContextPath maps an absolute CLI
// path into that tree and AbsoluteVerbPath maps it back out.
package client

import (
	"strings"
	"sync"

	cmd "github.com/ze-software/ze/internal/component/command"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// BuildVerbCommandTree builds the command tree for a direct verb context
// (`ze show`, `ze clear`, `ze request`, ...). Commands registered under the
// same verb are rooted under that verb and then exposed relative to it.
// Read-only commands that are not rooted under "show" remain available under
// `ze show` unchanged.
func BuildVerbCommandTree(verb string) *Command {
	rpcs := AllCLIRPCs()
	infos := make([]cmd.RPCInfo, 0, len(rpcs))
	descriptions := make(map[string]string)
	argDefs := make(map[string][]cmd.ArgDef)

	for _, reg := range rpcs {
		paths := cliWireToPaths[reg.WireMethod]
		if len(paths) == 0 {
			continue
		}
		for _, cliPath := range paths {
			effective, ok := verbContextPath(cliPath, verb)
			if !ok {
				continue
			}
			infos = append(infos, cmd.RPCInfo{
				CLICommand: effective,
				ReadOnly:   pluginserver.IsReadOnlyPath(cliPath),
			})
			recordContextDescriptions(descriptions, cliPath, effective)
			if defs := pathArgDefs[cliPath]; len(defs) > 0 && len(argDefs[effective]) == 0 {
				argDefs[effective] = defs
			}
		}
	}

	tree := cmd.BuildTree(infos, false)
	applyDescriptions(tree, descriptions)
	applyArgDefs(tree, argDefs)
	if yangCmdTree != nil {
		yangVerb := yangCmdTree.Children[verb]
		cmd.MergeYANGNodes(tree, yangVerb)
	}
	wireValueHints(tree)
	return tree
}

// AbsoluteVerbPath maps command words that are RELATIVE to verb -- the form
// BuildVerbCommandTree exposes -- back to the absolute CLI path words that the
// daemon dispatcher, the local-handler registry, and the offline-fallback
// registry are all keyed on.
//
// It is the inverse of verbContextPath and reads the same registrations, so the
// two cannot drift. Two shapes reach the tree and they invert differently: a
// command rooted under the verb had `verb ` stripped and gets it back, while a
// read-only command rooted under ANOTHER verb (`monitor ping`, reachable as
// `ze show monitor ping`) was carried in whole and is returned unchanged.
// Relative words no registered command declares get the verb prefixed, which is
// what an offline-only command such as `show version` needs.
//
// declared reports whether a registered command declares that exact path. A
// path with children and no declaration is a grouping container (`show bgp`),
// which a caller renders as a subcommand list instead of dispatching. Node
// descriptions cannot answer that question: MergeYANGNodes gives a grouping
// container its YANG description too, so an empty description marks nothing.
func AbsoluteVerbPath(verb string, rel []string) (words []string, declared bool) {
	if len(rel) == 0 {
		return nil, false
	}
	relPath := textbuf.Join(rel, " ")
	carried := ""
	for _, reg := range AllCLIRPCs() {
		for _, cliPath := range cliWireToPaths[reg.WireMethod] {
			effective, ok := verbContextPath(cliPath, verb)
			if !ok || effective != relPath {
				continue
			}
			if effective != cliPath {
				// The verb was stripped to build the tree: put it back.
				return strings.Fields(cliPath), true
			}
			carried = cliPath
		}
	}
	if carried != "" {
		return strings.Fields(carried), true
	}
	out := make([]string, 0, len(rel)+1)
	out = append(out, verb)
	return append(out, rel...), false
}

// declaredCommands is the set of ABSOLUTE CLI paths some registered built-in
// declares. It is the same population AbsoluteVerbPath scans, keyed for a
// direct question rather than an inverse one, so the two cannot disagree.
//
// Built on first use, not at init: every ze:command registration happens in an
// init() of the owning package, and package init order across the binary is
// undefined. No dispatch runs before init() completes, so first use is always
// after the last registration.
var declaredCommands = sync.OnceValue(func() map[string]struct{} {
	set := make(map[string]struct{})
	for _, reg := range AllCLIRPCs() {
		for _, cliPath := range cliWireToPaths[reg.WireMethod] {
			set[cliPath] = struct{}{}
		}
	}
	return set
})

// IsDeclaredCommand reports whether a registered built-in declares this exact
// absolute CLI path -- the path the daemon's dispatcher is keyed on.
//
// registry.LookupLocal asks this to refuse a local handler that would swallow a
// declared child of its own path (see the shadow rule there). It is the same
// fact AbsoluteVerbPath returns as `declared`, asked of an absolute path
// directly, because the local-handler registry is keyed on absolute paths and
// has no verb to be relative to.
func IsDeclaredCommand(path string) bool {
	_, ok := declaredCommands()[path]
	return ok
}

func verbContextPath(cliPath, verb string) (string, bool) {
	if verb == "show" {
		if rest, ok := strings.CutPrefix(cliPath, "show "); ok {
			return rest, true
		}
		if pluginserver.IsReadOnlyPath(cliPath) {
			return cliPath, true
		}
		return "", false
	}
	var tb textbuf.Buffer
	prefix := tb.Str(verb).Byte(' ').String()
	rest, ok := strings.CutPrefix(cliPath, prefix)
	if !ok || rest == "" {
		return "", false
	}
	return rest, true
}

func recordContextDescriptions(dst map[string]string, cliPath, effective string) {
	origParts := strings.Fields(cliPath)
	effParts := strings.Fields(effective)
	if len(origParts) == 0 || len(effParts) == 0 {
		return
	}
	offset := len(origParts) - len(effParts)
	if offset < 0 {
		return
	}
	for i := 1; i <= len(effParts); i++ {
		effPrefix := textbuf.Join(effParts[:i], " ")
		origPrefix := textbuf.Join(origParts[:offset+i], " ")
		if dst[effPrefix] != "" {
			continue
		}
		if desc := pathDescriptions[origPrefix]; desc != "" {
			dst[effPrefix] = desc
		}
	}
}
