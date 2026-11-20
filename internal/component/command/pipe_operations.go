// Design: docs/architecture/api/commands.md — what a command takes after a pipe
// Related: pipe_catalog.go — the generic operators and their contracts
// Related: pipe_filter.go — the operations a command declares and runs itself
// Related: declared.go — the offline join of what a command declares
// Related: completer.go — pipe completion offers this list
//
// pipe_operations.go answers ONE question for one command: what may follow a
// pipe. `show command help`, `ze help command --json`, the le catalogs and the
// completer all read it, so no surface keeps a list of its own.

package command

// The sources a pipe operation entry names: where the word comes from.
const (
	// PipeSourceGeneric is a catalog operator the command did not declare.
	PipeSourceGeneric = "generic"
	// PipeSourceOverride is a name the command declares that the catalog also
	// holds: the command runs it in place of the generic, held to the
	// generic's argument contract and answer shape.
	PipeSourceOverride = "override"
	// PipeSourceCommand is a name the command declares that the catalog does
	// not hold: an operation of the command's own.
	PipeSourceCommand = "command"
	// PipeSourceAlias is a named chain: an in-tree, global or plugin alias.
	PipeSourceAlias = "alias"
)

// The availability values a pipe operation entry carries.
const (
	// PipeAvailableAlways: the operation applies whatever the answer holds,
	// known before the command runs.
	PipeAvailableAlways = "always"
	// PipeAvailableWithRows: a row operator on a command that declared no
	// shape, applied to the answer in hand and refused by name when it has no
	// rows.
	PipeAvailableWithRows = "with-rows"
	// PipeAvailableWhenStreaming: the operation acts on a sequence of answers,
	// so it means something only where the command keeps answering.
	PipeAvailableWhenStreaming = "when-streaming"
)

// PipeOperationEntry is one word a command answers to after a pipe, with its
// contract. The JSON keys are the published `pipe-operations` entry, the same
// on `show command help` and in `ze help command --json`.
type PipeOperationEntry struct {
	Name string `json:"name"`
	// Source is one of the PipeSource values.
	Source string `json:"source"`
	// Argument is the argument kind spelling (PipeArgKind.String): the
	// catalog's for a generic and an override, the declaration's own for a
	// command operation, and none for an alias.
	Argument string `json:"argument"`
	// Available is one of the PipeAvailable values.
	Available string `json:"available"`
	// Class is the catalog class of a generic and an override, absent for a
	// command operation and an alias, which have none.
	Class string `json:"class,omitempty"`
	// Overridable marks a generic a command may declare under its own name.
	Overridable bool `json:"overridable,omitempty"`
	// LocalOnly marks a generic that MUST be expanded by the process the
	// operator started. Daemon-expanded SSH and web chains refuse it.
	LocalOnly   bool   `json:"local-only,omitempty"`
	Description string `json:"description"`
	// Expansion is the chain an alias stands for, aliases only.
	Expansion string `json:"expansion,omitempty"`
}

// PipeOperationsForCommand answers what may follow a pipe on command, over the
// registries this process holds. Inside a daemon that is every started
// plugin's Stage 1 declaration as well as the in-tree ones, which is what
// `show command help` answers and what the completer offers.
//
// A reader that starts no daemon reads DeclaredForCommand and passes it to
// PipeOperations instead, because only that join sees a compiled-in plugin's
// registration.
//
// Safe for concurrent use: every registry it reads takes its own lock.
func PipeOperationsForCommand(command string) []PipeOperationEntry {
	shape, shapeDeclared := ShapeForCommand(command)
	return PipeOperations(Declared{
		Shape:         shape,
		ShapeDeclared: shapeDeclared,
		AddressFields: AddressFieldsForCommand(command),
		Aliases:       AliasesForCommand(command),
		Operations:    PipeFiltersForCommand(command),
	})
}

// PipeOperations answers what may follow a pipe on a command that declares
// what declared holds: the generics the chain validator admits for it, in
// catalog order, each declared override in its generic's place, then the
// command's own operations and its aliases, in the order declared holds them
// (sorted by name, as both producers of a Declared answer them).
//
// The availability of a generic is the chain validator's rule
// (validatePipesForSurface), stated per operator, so a listed generic is one
// the validator admits and an omitted one is one it refuses before the command
// runs. TestPipeOperationsAgreeWithValidator holds the two together.
func PipeOperations(declared Declared) []PipeOperationEntry {
	own := make(map[string]PipeFilter, len(declared.Operations))
	for _, operation := range declared.Operations {
		own[operation.Name] = operation
	}
	entries := make([]PipeOperationEntry, 0, len(pipeCatalog)+len(declared.Operations)+len(declared.Aliases))
	for index := range pipeCatalog {
		generic := &pipeCatalog[index]
		if override, overrides := own[generic.Name]; overrides {
			entries = append(entries, overrideOperationEntry(generic, override))
			continue
		}
		available, admitted := genericAvailability(generic, declared)
		if !admitted {
			continue
		}
		entries = append(entries, genericOperationEntry(generic, available))
	}
	for _, operation := range declared.Operations {
		if _, generic := knownPipeOps[operation.Name]; generic {
			continue
		}
		entries = append(entries, PipeOperationEntry{
			Name: operation.Name, Source: PipeSourceCommand, Argument: operation.Argument.String(),
			Available: PipeAvailableAlways, Description: operation.Description,
		})
	}
	for _, alias := range declared.Aliases {
		entries = append(entries, PipeOperationEntry{
			Name: alias.Name, Source: PipeSourceAlias, Argument: ArgNone.String(),
			Available: PipeAvailableAlways, Description: alias.Description, Expansion: alias.Expansion,
		})
	}
	return entries
}

// PipeLayerReached answers whether any surface for command reaches the pipe
// layer, and so whether its pipe operations are published at all.
//
// A command the CLIENT serves in its own process reaches the pipe layer only
// if it answers with DATA. A plain local handler suppresses the list only when
// no daemon handler serves the path, because a daemon surface reaches that
// handler independently of the local shortcut: `show version` keeps a printing
// local shortcut while its separately registered daemon handler pipes. A
// local-data command (`show data cat`) answers DATA on every route.
//
// daemonServes reports whether the daemon registers a handler for the exact
// path. This package cannot answer it: the builtin RPC table lives in the
// dispatcher, which imports this package.
func PipeLayerReached(command string, daemonServes func(path string) bool) bool {
	if !HasLocal(command) {
		return true
	}
	if HasLocalData(command) {
		return true
	}
	return daemonServes(command)
}

// genericAvailability answers a generic's availability on a command that
// declares what declared holds, and whether the chain validator admits it
// there at all (validateStreamOps, validateDeclaredShape).
func genericAvailability(generic *PipeOperator, declared Declared) (string, bool) {
	if generic.NeedsAddressField {
		// validateDeclaredShape refuses `| resolve` and `| origin` unless the
		// command declares a shape and the fields that hold an address.
		if !declared.ShapeDeclared {
			return "", false
		}
		if len(declared.AddressFields) == 0 {
			return "", false
		}
	}
	switch generic.Class {
	case ClassStream:
		// validateStreamOps admits it on a streaming route alone.
		return PipeAvailableWhenStreaming, true
	case ClassGlobal:
		return PipeAvailableAlways, true
	case ClassData:
		if !declared.ShapeDeclared {
			return PipeAvailableWithRows, true
		}
		if generic.Applies(declared.Shape) {
			return PipeAvailableAlways, true
		}
		// The declared shape cannot support it, so it is refused before the
		// command runs and is not published as supported at all.
		return "", false
	default:
		panic("BUG: unknown pipe class")
	}
}

func genericOperationEntry(generic *PipeOperator, available string) PipeOperationEntry {
	return PipeOperationEntry{
		Name: generic.Name, Source: PipeSourceGeneric, Argument: generic.Arg.String(),
		Available: available, Class: generic.Class.String(), Overridable: generic.Overridable(),
		LocalOnly: generic.LocalOnly, Description: generic.Description,
	}
}

// overrideOperationEntry answers a declared override in its generic's place.
// The command answers it itself, so it is always available, and its own
// description replaces the generic's: the RIB's `count` counts routes.
func overrideOperationEntry(generic *PipeOperator, override PipeFilter) PipeOperationEntry {
	return PipeOperationEntry{
		Name: generic.Name, Source: PipeSourceOverride, Argument: generic.Arg.String(),
		Available: PipeAvailableAlways, Class: generic.Class.String(), Description: override.Description,
	}
}
