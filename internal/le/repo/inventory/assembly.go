// Design: docs/architecture/plugin/feature-gates.md -- retained direct transfers to the existing stdlib boundary.
// Related: assemblysyntax.go -- finite body grammar; assemblylinkage.go -- selected symbol noninterference.
package repoinventory

import (
	"fmt"
	"go/token"
	"strconv"
	"strings"
)

// bodylessProof is the certificate of a retained bodyless function. Its two
// variants are bodylessTransfer and bodylessLeaf; certifyAssembly returns nil
// only beside an error.
type bodylessProof interface{ bodylessProof() }

// bodylessTransfer is the proved standard-library destination of a retained
// bodyless function: the linker symbol it reaches and that symbol's import path.
type bodylessTransfer struct {
	destination string
	identity    string
}

// bodylessLeaf is a retained assembly body that transfers control nowhere
// outside itself and reads symbols only as leafSymbolReads admits (leafBody).
// variables names, once each, the same-package Go variables it reads, which
// assemblyResult joins as inputs of the call; it is empty when the body reads
// none.
type bodylessLeaf struct {
	variables []string
}

func (bodylessTransfer) bodylessProof() {}
func (bodylessLeaf) bodylessProof()     {}

// assemblySymbolRole names one symbol a linkage directive must not touch, and
// the role the refusal gives it.
type assemblySymbolRole struct {
	role   string
	symbol string
}

// certifyAssembly proves a retained bodyless entry in one of three ways, each
// not remapped or implemented elsewhere in the retained selected population:
// one pull-style linkname to a standard-library symbol, a complete leaf body,
// or a complete zero-frame NOSPLIT trampoline to a standard-library
// destination. It executes no instructions.
func (s *Snapshot) certifyAssembly(function sourceFunction, index *assemblyIndex) (bodylessProof, error) {
	prefix, found := index.prefixes[function.pkg]
	if !found {
		return nil, fmt.Errorf("assembly entry package is not in the selected source population")
	}
	entry := prefix + "." + function.decl.Name.Name
	declarations := index.declarations[entry]
	if len(declarations) != 1 {
		return nil, fmt.Errorf("assembly entry %s has ambiguous Go declarations", entry)
	}
	if declarations[0] != function.decl {
		return nil, fmt.Errorf("assembly entry %s has conflicting Go ownership", entry)
	}
	if index.objects[entry] {
		return nil, fmt.Errorf("assembly entry %s conflicts with a Go object", entry)
	}
	definitions := index.definitions[entry]
	pulled := -1
	var transfer bodylessTransfer
	if own := entryLinkages(index, entry); len(own) == 1 && len(definitions) == 0 {
		// The approved trust contract (INVENTORY-EXTERNAL-EFFECTS.md) puts
		// go:linkname outside external authority: code outside the retained
		// source is trusted not to use it against registration.
		// A bodyless declaration pulled from a standard-library symbol therefore
		// runs standard-library code, which is not analyzer input and reaches
		// registration only through the values passed to it, exactly like a
		// direct call. A module target could name retained code and is refused.
		pulled = own[0]
		linked, err := standardLinkname(index, index.linkages[pulled].target)
		if err != nil {
			return nil, fmt.Errorf("%s: bodyless entry %s: %w", index.linkages[pulled].site, entry, err)
		}
		transfer = linked
	} else {
		if len(definitions) != 1 {
			return nil, fmt.Errorf("assembly entry %s has %d selected definitions, not one proved body", entry, len(definitions))
		}
		definition := &definitions[0]
		body := s.blobs[definition.path][definition.bodyStart:definition.bodyEnd]
		// The approved trust contract (INVENTORY-EXTERNAL-EFFECTS.md) trusts
		// code not to fabricate references to retained registration functions:
		// unsafe pointer fabrication, go:linkname, runtime symbol tricks and
		// arbitrary native capability fabrication are outside it. A leaf calls
		// nothing, branches only to its own labels and names a symbol only to
		// read a same-file RODATA table or a same-package Go variable, so it can
		// act only on its registers, its stack frame, the memory its arguments
		// point to and the values those variables hold. That is the effect
		// model of an external call with its arguments and those variables
		// tracked, which assemblyResult applies to it. A body that is not a
		// leaf takes the trampoline proof, and its refusal carries both causes.
		variables, leafErr := leafDefinition(index, definition, body, function.decl.Name.Name, prefix)
		if leafErr == nil {
			if err := linkageFree(index, pulled, assemblySymbolRole{role: "entry", symbol: entry}); err != nil {
				return nil, err
			}
			return bodylessLeaf{variables: variables}, nil
		}
		notLeaf := func(err error) error {
			return fmt.Errorf("%w; not a proved leaf: %w", err, leafErr)
		}
		if err := trampolineHeader(definition.header, function.decl.Name.Name); err != nil {
			return nil, notLeaf(fmt.Errorf("%s: %w", definition.path, err))
		}
		destination, err := trampolineDestination(body, definition.path, prefix, definition.macros)
		if err != nil {
			return nil, notLeaf(err)
		}
		identity, err := standardAssemblyImport(function, destination)
		if err != nil {
			return nil, notLeaf(err)
		}
		transfer = bodylessTransfer{destination: destination, identity: identity}
	}
	destination := transfer.destination
	if len(index.definitions[destination]) != 0 {
		return nil, fmt.Errorf("assembly destination %s is defined by selected repository assembly", destination)
	}
	if len(index.declarations[destination]) != 0 {
		return nil, fmt.Errorf("assembly destination %s has a selected repository Go declaration", destination)
	}
	if index.objects[destination] {
		return nil, fmt.Errorf("assembly destination %s has a selected repository Go object", destination)
	}
	entryRole := assemblySymbolRole{role: "entry", symbol: entry}
	destinationRole := assemblySymbolRole{role: "destination", symbol: destination}
	if err := linkageFree(index, pulled, entryRole, destinationRole); err != nil {
		return nil, err
	}
	return transfer, nil
}

// linkageFree refuses the first selected directive that remaps or opens one
// of symbols. The directive at offset pulled established the transfer and is
// its own evidence, so it is skipped; -1 skips none.
func linkageFree(index *assemblyIndex, pulled int, symbols ...assemblySymbolRole) error {
	for offset, linkage := range index.linkages {
		if offset == pulled {
			continue
		}
		for _, symbol := range symbols {
			if linkageTouches(linkage, symbol.symbol) {
				return fmt.Errorf("%s: linkage remaps or opens assembly %s %s", linkage.site, symbol.role, symbol.symbol)
			}
		}
	}
	return nil
}

// entryLinkages returns the offsets of every selected directive whose local
// name is entry. More than one is ambiguous and leaves the transfer unproved.
func entryLinkages(index *assemblyIndex, entry string) []int {
	var offsets []int
	for offset, linkage := range index.linkages {
		if linkage.local == entry {
			offsets = append(offsets, offset)
		}
	}
	return offsets
}

// standardLinkname proves that a pull-style linkname target names a
// standard-library symbol and no package of the retained selected population.
func standardLinkname(index *assemblyIndex, target string) (bodylessTransfer, error) {
	identity, err := standardLinkTarget(target)
	if err != nil {
		return bodylessTransfer{}, err
	}
	// A dotless retained package (an executable's "main") shares the spelling.
	for _, prefix := range index.prefixes {
		if prefix == identity {
			return bodylessTransfer{}, fmt.Errorf("linkname target %s names a selected repository package", target)
		}
	}
	return bodylessTransfer{destination: target, identity: identity}, nil
}

// standardLinkTarget reads the import path of a linkname target whose package
// is in the standard library: its first path element holds no dot, and its
// linker prefix is the import path unchanged.
func standardLinkTarget(target string) (string, error) {
	separator := strings.LastIndexByte(target, '.')
	if separator < 1 {
		return "", fmt.Errorf("linkname target %s has no proved package identity", target)
	}
	if !token.IsIdentifier(target[separator+1:]) {
		return "", fmt.Errorf("linkname target %s has unsupported symbol identity", target)
	}
	identity := target[:separator]
	first, _, _ := strings.Cut(identity, "/")
	if strings.Contains(first, ".") {
		return "", fmt.Errorf("linkname target %s names a module package, which may hold retained code", target)
	}
	if linkerPathPrefix(identity) != identity {
		return "", fmt.Errorf("linkname target %s has unsupported package identity", target)
	}
	if identity == "unsafe" {
		return "", fmt.Errorf("linkname target %s manufactures an unsafe reference", target)
	}
	return identity, nil
}

func linkageTouches(linkage assemblyLinkage, symbol string) bool {
	if linkage.local == symbol {
		return true
	}
	if linkage.target == symbol {
		return true
	}
	// ABI-wrapper suffixes cannot establish a distinct implementation owner.
	if strings.HasPrefix(linkage.target, symbol+".") {
		return true
	}
	return false
}

func trampolineHeader(tokens []string, name string) error {
	if len(tokens) != 12 {
		return fmt.Errorf("unsupported assembly TEXT flags, frame or ABI grammar")
	}
	if err := textHeaderEntry(tokens, name); err != nil {
		return err
	}
	if tokens[8] != "$" {
		return fmt.Errorf("unsupported assembly frame")
	}
	if tokens[9] != "0" {
		return fmt.Errorf("assembly trampoline requires a zero frame")
	}
	if tokens[10] != "-" {
		return fmt.Errorf("unsupported assembly argument size")
	}
	return assemblySize(tokens[11], "argument")
}

// textHeaderEntry checks the entry symbol and the one canonical flag of a
// TEXT header. The caller MUST have checked that tokens holds at least eight.
func textHeaderEntry(tokens []string, name string) error {
	if err := textHeaderSymbol(tokens, name); err != nil {
		return err
	}
	if tokens[6] != "NOSPLIT" && tokens[6] != "4" {
		return fmt.Errorf("unsupported assembly TEXT flags; only canonical NOSPLIT is proved")
	}
	if tokens[7] != "," {
		return fmt.Errorf("unsupported assembly TEXT flags")
	}
	return nil
}

// textHeaderSymbol checks the `TEXT ·name(SB),` entry of a TEXT header. The
// caller MUST have checked that tokens holds at least six.
func textHeaderSymbol(tokens []string, name string) error {
	want := []string{"TEXT", "·" + name, "(", "SB", ")", ","}
	for offset, token := range want {
		if tokens[offset] != token {
			return fmt.Errorf("unsupported assembly entry symbol or TEXT grammar")
		}
	}
	return nil
}

// assemblySize accepts one canonical decimal frame or argument size.
func assemblySize(value, what string) error {
	if len(value) > 1 && value[0] == '0' {
		return fmt.Errorf("assembly %s size must be canonical decimal", what)
	}
	if _, err := strconv.ParseUint(value, 10, 31); err != nil {
		return fmt.Errorf("unsupported assembly %s size: %w", what, err)
	}
	return nil
}

func trampolineDestination(raw []byte, name, prefix string, macros map[string]bool) (string, error) {
	destination := ""
	err := assemblyStatements(raw, name, func(statement assemblyStatement) error {
		tokens := statement.tokens
		// The complete unit's preprocessing was already validated by the census.
		if tokens[0] == "#" {
			if assemblyConditionalDirective(tokens) != assemblyConditionalNone {
				return fmt.Errorf("%s: entered assembly body contains conditional", name)
			}
			return nil
		}
		if err := macroFreeTokens(tokens, macros); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if destination != "" {
			return fmt.Errorf("%s: assembly body has extra instructions or directives", name)
		}
		if len(tokens) != 5 {
			return fmt.Errorf("%s: assembly body is not a direct JMP", name)
		}
		if tokens[0] != "JMP" {
			return fmt.Errorf("%s: assembly body is not a direct JMP", name)
		}
		if tokens[2] != "(" {
			return fmt.Errorf("%s: unsupported assembly destination syntax", name)
		}
		if tokens[3] != "SB" {
			return fmt.Errorf("%s: assembly destination is not an external symbol", name)
		}
		if tokens[4] != ")" {
			return fmt.Errorf("%s: unsupported assembly destination syntax", name)
		}
		resolved, err := assemblySymbol(tokens[1], prefix)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		destination = resolved
		return nil
	})
	if err != nil {
		return "", err
	}
	if destination == "" {
		return "", fmt.Errorf("%s: assembly body has no proved transfer", name)
	}
	return destination, nil
}

// standardAssemblyImport compares the linker package prefix with the actual
// retained import path. A Go-local alias or an empty import-map target is not
// evidence that an assembly qualifier denotes the standard library.
func standardAssemblyImport(function sourceFunction, destination string) (string, error) {
	separator := strings.LastIndexByte(destination, '.')
	if separator < 1 {
		return "", fmt.Errorf("assembly destination %s has no proved package identity", destination)
	}
	if !token.IsIdentifier(destination[separator+1:]) {
		return "", fmt.Errorf("assembly destination %s has unsupported symbol identity", destination)
	}
	prefix := destination[:separator]
	identity := ""
	matches := 0
	for _, imported := range function.file.Imports {
		name, err := strconv.Unquote(imported.Path.Value)
		if err != nil {
			return "", fmt.Errorf("read assembly boundary import: %w", err)
		}
		first, _, _ := strings.Cut(name, "/")
		if strings.Contains(first, ".") {
			continue
		}
		if linkerPathPrefix(name) == prefix {
			identity = name
			matches++
		}
	}
	if matches != 1 {
		return "", fmt.Errorf("assembly destination %s does not identify one declared standard-library import", destination)
	}
	return identity, nil
}
