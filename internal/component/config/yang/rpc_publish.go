// Design: docs/architecture/config/yang-config-design.md — YANG schema handling

package yang

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	gyang "github.com/openconfig/goyang/pkg/yang"

	"github.com/ze-software/ze/internal/core/ipc"
)

// RPCPublication is every rpc statement the loaded schema holds, sorted into
// the three ways one can be named on the wire.
type RPCPublication struct {
	// Commands holds one row for each wire method a ze:command node declares
	// and points at an rpc with ze:rpc. An rpc several nodes point at appears
	// once under each of their methods. Sorted by wire method.
	Commands []RPCMeta
	// Protocol holds the rpcs that declare their own wire method with
	// ze:method: the plugin IPC protocol, which no command node reaches.
	// Sorted by wire method.
	Protocol []RPCMeta
	// Unnamed holds the rpcs that no node points at and that declare no
	// ze:method. Each is published under no name at all, so a caller that
	// reads it has nothing to send. Sorted by module, then rpc.
	Unnamed []RPCMeta
	// Unlinked holds, as "method -> module:rpc", each pointer whose target
	// module this process did not load. A binary links the -cmd module of a
	// component without the -api module of another, so the pointer is not
	// wrong there, only unanswerable; a process that links every module (the
	// command contract gate) refuses each one. Sorted.
	Unlinked []string
}

// ErrRPCPointer marks a ze:rpc pointer the schema cannot honor: a target
// that is malformed or names no loaded rpc, a pointer on a node that declares
// no ze:command, one wire method pointing at two rpcs, or an rpc that is both
// pointed at and carries its own ze:method.
var ErrRPCPointer = errors.New("ze:rpc pointer")

// ErrRPCMethod marks a ze:command or ze:method argument that ipc.ParseMethod
// refuses, so the schema cannot publish it as a wire method.
var ErrRPCMethod = errors.New("malformed wire method")

// PublishedRPCs answers the wire name of every rpc statement the loader holds.
//
// The ze:command node is the one declaration of a command's wire method, and
// its ze:rpc statement names the rpc that documents the command's input and
// output, so an rpc carries the method of whichever node points at it. No
// method is built from a module's file name. Every refusal is returned joined,
// so one run names every broken pointer.
func PublishedRPCs(schema *Resolved) (RPCPublication, error) {
	var pub RPCPublication

	declared := make(map[string]RPCMeta)
	var order []string
	for _, module := range schema.ModuleNames() {
		for _, meta := range ExtractRPCs(schema, module) {
			key := meta.Module + ":" + meta.Name
			declared[key] = meta
			order = append(order, key)
		}
	}

	pointers := make(map[string]string) // wire method -> "module:rpc"
	var errs []error
	for _, module := range schema.ModuleNames() {
		if !strings.HasSuffix(module, cmdModuleSuffix) {
			continue
		}
		collectRPCPointers(schema.GetEntry(module), module, pointers, &errs)
	}

	loaded := make(map[string]bool)
	for _, module := range schema.ModuleNames() {
		loaded[module] = true
	}

	pointed := make(map[string]bool, len(pointers))
	for method, target := range pointers {
		meta, ok := declared[target]
		if module, _, _ := strings.Cut(target, ":"); !ok && !loaded[module] {
			pub.Unlinked = append(pub.Unlinked, method+" -> "+target)
			continue
		}
		if !ok {
			errs = append(errs, fmt.Errorf("%w: %s points at %s, which its module does not declare", ErrRPCPointer, method, target))
			continue
		}
		if meta.WireMethod != "" {
			errs = append(errs, fmt.Errorf("%w: %s points at %s, which declares its own ze:method %s", ErrRPCPointer, method, target, meta.WireMethod))
			continue
		}
		pointed[target] = true
		meta.WireMethod = method
		pub.Commands = append(pub.Commands, meta)
	}

	for _, key := range order {
		meta := declared[key]
		switch {
		case meta.WireMethod != "":
			if _, _, err := ipc.ParseMethod(meta.WireMethod); err != nil {
				errs = append(errs, fmt.Errorf("%w: %s:%s declares ze:method %q: %w", ErrRPCMethod, meta.Module, meta.Name, meta.WireMethod, err))
				continue
			}
			pub.Protocol = append(pub.Protocol, meta)
		case !pointed[key]:
			pub.Unnamed = append(pub.Unnamed, meta)
		}
	}

	slices.Sort(pub.Unlinked)
	sortByWireMethod(pub.Commands)
	sortByWireMethod(pub.Protocol)
	sort.Slice(pub.Unnamed, func(i, j int) bool {
		if pub.Unnamed[i].Module != pub.Unnamed[j].Module {
			return pub.Unnamed[i].Module < pub.Unnamed[j].Module
		}
		return pub.Unnamed[i].Name < pub.Unnamed[j].Name
	})
	return pub, errors.Join(errs...)
}

// collectRPCPointers walks one -cmd module's entry tree and records the
// ze:rpc target of every node that carries one, keyed by the node's wire
// method. A method several nodes declare may point at one rpc only.
func collectRPCPointers(entry *gyang.Entry, module string, pointers map[string]string, errs *[]error) {
	if entry == nil {
		return
	}
	names := make([]string, 0, len(entry.Dir))
	for name := range entry.Dir {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		child := entry.Dir[name]
		if target := GetRPCExtension(child); target != "" {
			method := GetCommandExtension(child)
			switch {
			case method == "":
				*errs = append(*errs, fmt.Errorf("%w: node %s in %s points at %s but declares no ze:command", ErrRPCPointer, child.Path(), module, target))
			case !validWireMethod(method):
				*errs = append(*errs, fmt.Errorf("%w: node %s in %s declares ze:command %q", ErrRPCMethod, child.Path(), module, method))
			case !validWireMethod(target):
				*errs = append(*errs, fmt.Errorf("%w: %s points at %q, which is not module:rpc-name", ErrRPCPointer, method, target))
			case pointers[method] != "" && pointers[method] != target:
				*errs = append(*errs, fmt.Errorf("%w: %s points at both %s and %s", ErrRPCPointer, method, pointers[method], target))
			default:
				pointers[method] = target
			}
		}
		collectRPCPointers(child, module, pointers, errs)
	}
}

// validWireMethod reports whether a ze:command method or a ze:rpc target has
// the module:rpc-name shape ipc.ParseMethod accepts: both halves present, no
// whitespace or second colon, and no longer than ipc.MaxMethodLength.
func validWireMethod(method string) bool {
	_, _, err := ipc.ParseMethod(method)
	return err == nil
}

func sortByWireMethod(rpcs []RPCMeta) {
	sort.Slice(rpcs, func(i, j int) bool { return rpcs[i].WireMethod < rpcs[j].WireMethod })
}

// GetRPCExtension reads the ze:rpc pointer of a command node: the rpc, as
// module:rpc-name, that documents the node's input and output. It answers
// empty for a node that points at none.
func GetRPCExtension(entry *gyang.Entry) string {
	if entry == nil {
		return ""
	}
	return extensionArgument(entry.Exts, ":rpc")
}

// GetMethodExtension reads the ze:method statement of an rpc or a
// notification: the wire method it declares for itself because no command
// node reaches it. It answers empty when the statement declares none.
func GetMethodExtension(exts []*gyang.Statement) string {
	return extensionArgument(exts, ":method")
}

// extensionArgument answers the argument of the first statement whose
// keyword ends in suffix, which matches whatever prefix the module imported
// ze-extensions under.
func extensionArgument(exts []*gyang.Statement, suffix string) string {
	for _, ext := range exts {
		if ext != nil && strings.HasSuffix(ext.Keyword, suffix) {
			return ext.Argument
		}
	}
	return ""
}
