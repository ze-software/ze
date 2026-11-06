// Design: docs/architecture/plugin/feature-gates.md -- a leaf assembly body may read a same-file read-only table or a same-package Go variable.
// Related: assemblyleaf.go -- leafInstruction; externalassembly.go -- assemblyResult joins the variables read.
package repoinventory

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// The three assembler definition words a read-only table check tells apart.
const (
	assemblyTextWord   = "TEXT"
	assemblyDataWord   = "DATA"
	assemblyGlobalWord = "GLOBL"
)

// leafRead is one symbol operand a leaf instruction reads.
type leafRead struct {
	symbol string // the operand's symbol as written, middle dot and division slash kept
	static bool   // the operand names a file-static symbol<>
}

// leafCompares are the amd64 opcodes with no destination operand: each reads
// every operand and writes only flags, so a symbol in the last slot is a read.
var leafCompares = map[string]bool{
	"CMPB": true, "CMPW": true, "CMPL": true, "CMPQ": true,
	"TESTB": true, "TESTW": true, "TESTL": true, "TESTQ": true,
	"BTW": true, "BTL": true, "BTQ": true,
	"UCOMISS": true, "UCOMISD": true, "COMISS": true, "COMISD": true,
	"PTEST": true, "VPTEST": true,
}

// leafExchanges reports the opcodes that write a source operand as well as
// the destination, so no operand of theirs is only read.
func leafExchanges(opcode string) bool {
	if strings.HasPrefix(opcode, "XCHG") {
		return true
	}
	if strings.HasPrefix(opcode, "XADD") {
		return true
	}
	return strings.HasPrefix(opcode, "CMPXCHG")
}

// leafSymbolReads admits a symbol operand only as a read. In Go amd64
// assembly the destination is the last operand, so a symbol there is a write
// unless the opcode has no destination (leafCompares). A symbol in an
// instruction of more than two operands refuses, because the immediate-last
// forms (CMPPS and its kin) put the destination elsewhere. An address of a
// symbol (LEA, or a `$` constant) refuses, because an address can be written
// through or called later. The caller MUST have refused CALL and branches.
func leafSymbolReads(opcode string, tokens []string) ([]leafRead, error) {
	operands := leafOperands(tokens)
	var reads []leafRead
	for offset, operand := range operands {
		if !slices.Contains(operand, "SB") {
			continue
		}
		if leafExchanges(opcode) {
			return nil, fmt.Errorf("assembly leaf %s writes every operand, so it cannot only read symbol memory", opcode)
		}
		if operand[0] == "$" {
			return nil, fmt.Errorf("assembly leaf %s takes the address of a symbol through a $ constant", opcode)
		}
		if strings.HasPrefix(opcode, "LEA") {
			return nil, fmt.Errorf("assembly leaf %s takes the address of a symbol", opcode)
		}
		if len(operands) > 2 {
			return nil, fmt.Errorf("assembly leaf %s has %d operands, so the slot it writes is not proved", opcode, len(operands))
		}
		if offset == len(operands)-1 && !leafCompares[opcode] {
			return nil, fmt.Errorf("assembly leaf %s writes symbol memory through its last operand", opcode)
		}
		read, err := leafSymbolOperand(operand)
		if err != nil {
			return nil, err
		}
		reads = append(reads, read)
	}
	return reads, nil
}

// leafOperands splits an instruction's operand tokens at the commas outside
// parentheses. An operand is never empty: an empty one is returned as a
// single comma token, which no rule admits as a symbol.
func leafOperands(tokens []string) [][]string {
	var operands [][]string
	depth := 0
	start := 0
	for offset, token := range tokens {
		switch token {
		case "(":
			depth++
		case ")":
			depth--
		case ",":
			if depth == 0 {
				operands = append(operands, leafOperand(tokens, start, offset))
				start = offset + 1
			}
		}
	}
	if start < len(tokens) {
		operands = append(operands, tokens[start:])
	}
	return operands
}

func leafOperand(tokens []string, start, end int) []string {
	if start == end {
		return tokens[end : end+1]
	}
	return tokens[start:end]
}

// leafSymbolOperand accepts exactly `sym[<>][+N|-N](SB)`. An index register
// tail is refused, so the read stays at a constant offset from the symbol.
// A constant offset past the symbol's end reads another object: that is
// fabricating a reference outside the access path, which the trust contract
// (INVENTORY-EXTERNAL-EFFECTS.md) puts outside external authority, exactly as
// for an offset past the memory an argument points to.
func leafSymbolOperand(operand []string) (leafRead, error) {
	form := strings.Join(operand, "")
	if _, err := assemblySymbol(operand[0], ""); err != nil {
		return leafRead{}, fmt.Errorf("assembly leaf symbol operand %s has an unsupported address form", form)
	}
	read := leafRead{symbol: operand[0]}
	position := 1
	if len(operand) >= position+2 && operand[position] == "<" && operand[position+1] == ">" {
		read.static = true
		position += 2
	}
	if len(operand) > position+1 && (operand[position] == "+" || operand[position] == "-") {
		if _, err := strconv.ParseUint(operand[position+1], 0, 31); err != nil {
			return leafRead{}, fmt.Errorf("assembly leaf symbol operand %s has an unsupported offset", form)
		}
		position += 2
	}
	if !slices.Equal(operand[position:], []string{"(", "SB", ")"}) {
		return leafRead{}, fmt.Errorf("assembly leaf symbol operand %s has an unsupported address form", form)
	}
	return read, nil
}

// leafVariables resolves every symbol a leaf reads and returns the Go names
// of the same-package variables among them, each once, in body order.
//
// The approved trust contract (INVENTORY-EXTERNAL-EFFECTS.md) trusts code
// not to fabricate references to retained registration functions or memory
// outside its access paths. A read names the memory it reaches, so it adds an
// access path only to that symbol. A same-file RODATA table whose data holds
// no symbol address is constant bytes the linker places in read-only memory,
// so reading it adds no capability. A same-package Go variable can hold any
// value the package stored in it, so it is returned for assemblyResult to
// join as an input of the call, exactly as an argument is, and whatever it can
// reach stays tracked. Any other symbol refuses.
func leafVariables(index *assemblyIndex, definition *assemblyDefinition, prefix string, reads []leafRead) ([]string, error) {
	var variables []string
	for _, read := range reads {
		variable, err := leafSymbolTarget(index, definition.path, prefix, read)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", definition.path, err)
		}
		if variable == "" {
			continue
		}
		if slices.Contains(variables, variable) {
			continue
		}
		variables = append(variables, variable)
	}
	return variables, nil
}

// leafSymbolTarget returns the Go name of the same-package variable read, or
// "" for a proved same-file read-only table.
func leafSymbolTarget(index *assemblyIndex, path, prefix string, read leafRead) (string, error) {
	name, err := assemblySymbol(read.symbol, prefix)
	if err != nil {
		return "", err
	}
	if err := linkageFree(index, -1, assemblySymbolRole{role: "read", symbol: name}); err != nil {
		return "", err
	}
	if read.static {
		return "", leafReadOnlyTable(index, name, read.symbol, path)
	}
	if local, same := strings.CutPrefix(read.symbol, "·"); same {
		return leafPackageSymbol(index, name, local, read.symbol, path)
	}
	if strings.Contains(read.symbol, "·") {
		return "", fmt.Errorf("assembly leaf reads %s, another package's symbol", read.symbol)
	}
	return "", leafReadOnlyTable(index, name, read.symbol, path)
}

// leafPackageSymbol classifies a `·local` symbol of the leaf's own package:
// a Go variable no assembly defines is returned by name, and anything else
// must be a read-only table of the file at path, which refuses a function.
func leafPackageSymbol(index *assemblyIndex, name, local, symbol, path string) (string, error) {
	if strings.Contains(local, "·") {
		return "", fmt.Errorf("assembly leaf reads %s, another package's symbol", symbol)
	}
	if strings.Contains(local, "∕") {
		return "", fmt.Errorf("assembly leaf reads %s, another package's symbol", symbol)
	}
	if !index.objects[name] {
		return "", leafReadOnlyTable(index, name, symbol, path)
	}
	if len(index.definitions[name]) != 0 {
		return "", fmt.Errorf("assembly leaf reads %s, a Go variable that assembly also defines", symbol)
	}
	return local, nil
}

// leafReadOnlyTable proves that symbol is a read-only constant table of the
// file at path: every definition of it is in that file, exactly one GLOBL
// carries RODATA, no TEXT defines it, no Go declaration owns it, and no DATA
// of it holds a symbol address.
func leafReadOnlyTable(index *assemblyIndex, name, symbol, path string) error {
	if len(index.declarations[name]) != 0 {
		return fmt.Errorf("assembly leaf reads %s, which names a function", symbol)
	}
	if index.objects[name] {
		return fmt.Errorf("assembly leaf reads %s, which a Go variable of another spelling owns", symbol)
	}
	globals := 0
	for _, record := range index.definitions[name] {
		if record.path != path {
			return fmt.Errorf("assembly leaf reads %s, which %s also defines", symbol, record.path)
		}
		switch record.header[0] {
		case assemblyTextWord:
			return fmt.Errorf("assembly leaf reads %s, which names a function", symbol)
		case assemblyGlobalWord:
			globals++
			flags, err := leafGlobalFlags(record.header)
			if err != nil {
				return fmt.Errorf("assembly leaf reads %s: %w", symbol, err)
			}
			if !flags["RODATA"] {
				return fmt.Errorf("assembly leaf reads %s, whose GLOBL is not RODATA", symbol)
			}
		case assemblyDataWord:
			// The definition's own address is the one (SB) a constant DATA holds.
			if strings.Count(strings.Join(record.header, " "), " SB ") != 1 {
				return fmt.Errorf("assembly leaf reads %s, whose DATA holds a symbol address", symbol)
			}
		}
	}
	if globals != 1 {
		return fmt.Errorf("assembly leaf reads %s, which is neither a same-file RODATA table nor a same-package Go variable", symbol)
	}
	return nil
}

// leafGlobalFlags reads the flag expression of `GLOBL sym[<>](SB), FLAGS,
// $size`: only the names NOPTR and RODATA, each at most once, joined by `|`
// or `+` and optionally parenthesized. A GLOBL without flags has none.
func leafGlobalFlags(header []string) (map[string]bool, error) {
	closing := slices.Index(header, ")")
	if closing < 0 {
		return nil, fmt.Errorf("unsupported GLOBL grammar")
	}
	fields := header[closing+1:]
	if len(fields) == 0 || fields[0] != "," {
		return nil, fmt.Errorf("unsupported GLOBL grammar")
	}
	var expression []string
	if end := slices.Index(fields[1:], ","); end >= 0 {
		expression = fields[1 : end+1]
	}
	flags := make(map[string]bool)
	for _, token := range expression {
		switch token {
		case "(", ")", "|", "+":
			continue
		case "NOPTR", "RODATA":
			if flags[token] {
				return nil, fmt.Errorf("GLOBL repeats flag %s", token)
			}
			flags[token] = true
		default:
			return nil, fmt.Errorf("unsupported GLOBL flag %s", token)
		}
	}
	return flags, nil
}
