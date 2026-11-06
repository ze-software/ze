// Design: docs/architecture/plugin/feature-gates.md -- retained direct transfers to the existing stdlib boundary.
// Related: assemblysyntax.go -- finite body grammar; assemblylinkage.go -- selected symbol noninterference.
package repoinventory

import (
	"fmt"
	"go/token"
	"strconv"
	"strings"
)

// bodylessTransfer is the proved standard-library destination of a retained
// bodyless function: the linker symbol it reaches and that symbol's import path.
type bodylessTransfer struct {
	destination string
	identity    string
}

// certifyAssembly proves only a direct transfer from a retained bodyless entry
// to a standard-library destination that is not remapped or implemented
// elsewhere in the retained selected population: a complete zero-frame
// NOSPLIT trampoline, or one pull-style linkname. It executes no instructions.
func (s *Snapshot) certifyAssembly(function sourceFunction, index *assemblyIndex) (bodylessTransfer, error) {
	prefix, found := index.prefixes[function.pkg]
	if !found {
		return bodylessTransfer{}, fmt.Errorf("assembly entry package is not in the selected source population")
	}
	entry := prefix + "." + function.decl.Name.Name
	declarations := index.declarations[entry]
	if len(declarations) != 1 {
		return bodylessTransfer{}, fmt.Errorf("assembly entry %s has ambiguous Go declarations", entry)
	}
	if declarations[0] != function.decl {
		return bodylessTransfer{}, fmt.Errorf("assembly entry %s has conflicting Go ownership", entry)
	}
	if index.objects[entry] {
		return bodylessTransfer{}, fmt.Errorf("assembly entry %s conflicts with a Go object", entry)
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
			return bodylessTransfer{}, fmt.Errorf("%s: bodyless entry %s: %w", index.linkages[pulled].site, entry, err)
		}
		transfer = linked
	} else {
		if len(definitions) != 1 {
			return bodylessTransfer{}, fmt.Errorf("assembly entry %s has %d selected definitions, not one proved body", entry, len(definitions))
		}
		definition := &definitions[0]
		if err := trampolineHeader(definition.header, function.decl.Name.Name); err != nil {
			return bodylessTransfer{}, fmt.Errorf("%s: %w", definition.path, err)
		}
		body := s.blobs[definition.path][definition.bodyStart:definition.bodyEnd]
		destination, err := trampolineDestination(body, definition.path, prefix, definition.macros)
		if err != nil {
			return bodylessTransfer{}, err
		}
		identity, err := standardAssemblyImport(function, destination)
		if err != nil {
			return bodylessTransfer{}, err
		}
		transfer = bodylessTransfer{destination: destination, identity: identity}
	}
	destination := transfer.destination
	if len(index.definitions[destination]) != 0 {
		return bodylessTransfer{}, fmt.Errorf("assembly destination %s is defined by selected repository assembly", destination)
	}
	if len(index.declarations[destination]) != 0 {
		return bodylessTransfer{}, fmt.Errorf("assembly destination %s has a selected repository Go declaration", destination)
	}
	if index.objects[destination] {
		return bodylessTransfer{}, fmt.Errorf("assembly destination %s has a selected repository Go object", destination)
	}
	for offset, linkage := range index.linkages {
		// The one directive that established the transfer is its own evidence.
		if offset == pulled {
			continue
		}
		if linkageTouches(linkage, entry) {
			return bodylessTransfer{}, fmt.Errorf("%s: linkage remaps or opens assembly entry %s", linkage.site, entry)
		}
		if linkageTouches(linkage, destination) {
			return bodylessTransfer{}, fmt.Errorf("%s: linkage remaps or opens assembly destination %s", linkage.site, destination)
		}
	}
	return transfer, nil
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
	want := []string{"TEXT", "·" + name, "(", "SB", ")", ","}
	for offset, token := range want {
		if tokens[offset] != token {
			return fmt.Errorf("unsupported assembly entry symbol or TEXT grammar")
		}
	}
	if tokens[6] != "NOSPLIT" && tokens[6] != "4" {
		return fmt.Errorf("unsupported assembly TEXT flags; only canonical NOSPLIT is proved")
	}
	if tokens[7] != "," {
		return fmt.Errorf("unsupported assembly TEXT flags")
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
	if len(tokens[11]) > 1 && tokens[11][0] == '0' {
		return fmt.Errorf("assembly argument size must be canonical decimal")
	}
	if _, err := strconv.ParseUint(tokens[11], 10, 31); err != nil {
		return fmt.Errorf("unsupported assembly argument size: %w", err)
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
