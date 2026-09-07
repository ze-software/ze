// Design: docs/architecture/api/commands.md -- the command verb taxonomy
//
// report.go holds what `le command list` ANSWERS, apart from what produced it.
//
// The answer is a row per command, which is structured data every operator can
// act on: `| json` feeds a script, `| match show` keeps one verb's rows,
// `| count` says how many. It also renders ITSELF (Text), because the Markdown
// table is what `./le command list` prints and what a reader pastes into a
// document.

package commandlist

import "github.com/ze-software/ze/internal/core/textbuf"

// Command is one registered command, and it is one ROW of the answer.
type Command struct {
	// Verb is the first word of the CLI path when that word is one of the
	// taxonomy's verbs, and "-" when it is not.
	Verb string `json:"verb"`
	// Path is the CLI path the operator types, or the wire method when no
	// YANG command tree maps one.
	Path string `json:"path"`
	// WireMethod is the ze:command argument the handler registered, and it is
	// ABSENT for a command that has none: a streaming prefix and a TUI command
	// are reached by path alone.
	WireMethod string `json:"wire-method"`
	// Source says where the registration came from: builtin, streaming or cli.
	Source string `json:"source"`
	// Shape is the wire spelling of what the command's ANSWER holds: "doc" for
	// one document, "map" for rows that carry their own keys, "tab" for rows
	// read against column names. It is what the command DECLARED, so it is
	// empty for one that declares nothing, which is what every command written
	// before the declaration channel existed answers.
	Shape string `json:"shape,omitempty"`
	// ColumnOrders are the answer's record keys in the order a person reads
	// them, one list per record shape the command renders. A command that
	// renders an outer record and a list of rows declares two orders, so a flat
	// list could state neither.
	ColumnOrders [][]string `json:"column-orders,omitempty"`
	// AddressFields names the record keys whose value holds an IP address or a
	// prefix. Their presence is what admits `| resolve` and `| origin`.
	AddressFields []string `json:"address-fields,omitempty"`
	// Aliases are the pipe chains the command answers to by name.
	Aliases []Alias `json:"pipe-aliases,omitempty"`
}

// Alias is one pipe chain a command answers to by name.
//
// The expansion sits beside the description because an alias takes no argument
// and names no other alias, so the chain it stands for is the whole of what the
// name does (internal/component/command/alias.go).
type Alias struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Expansion   string `json:"expansion"`
}

// Commands is the whole answer of one run: every registered command, sorted by
// verb and then by path.
//
// It is a slice rather than a struct wrapping one, so `| json` answers the same
// array the `--json` flag of the script answered. The engine reads the rows
// straight out of it (internal/component/command/answer_shape.go, rowsIn).
type Commands []Command

// Text renders the inventory as the Markdown table `./le command list` prints.
// It ends in a newline.
//
// This is the Prose rendering leroot uses for the bare command, and every pipe
// operator bypasses it (internal/le/leroot, Prose).
func (c Commands) Text() string {
	var tb textbuf.Buffer
	tb.Str("# Command Inventory\n\n")
	tb.Str("| Verb | CLI Path | Wire Method | Source | Shape | Column Order | Address Fields | Aliases |\n")
	tb.Str("|------|----------|-------------|--------|-------|--------------|----------------|---------|\n")
	for _, entry := range c {
		tb.Str("| ").Str(entry.Verb).Str(" | ").Str(entry.Path).Str(" | ").
			Str(entry.WireMethod).Str(" | ").Str(entry.Source).Str(" | ").
			Str(cell(entry.Shape)).Str(" | ").
			Str(cell(joinOrders(entry.ColumnOrders))).Str(" | ").
			Str(cell(joinNames(entry.AddressFields))).Str(" | ").
			Str(cell(joinAliases(entry.Aliases))).Str(" |\n")
	}
	tb.Str("\nTotal: ").Int(int64(len(c))).Str(" commands\n")
	return tb.String()
}

// cell answers what one table cell holds. A command that declares nothing
// renders "-" rather than an empty cell, so a reader can tell a declaration of
// nothing from a column the renderer forgot.
func cell(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

// joinOrders renders every declared column order on one line: the names of one
// order separated by commas, and the orders themselves by a semicolon. A
// command that renders two record shapes declares two orders, and a comma
// between them would read as one longer order.
func joinOrders(orders [][]string) string {
	if len(orders) == 0 {
		return ""
	}
	var tb textbuf.Buffer
	for index, order := range orders {
		if index > 0 {
			tb.Str("; ")
		}
		tb.Join(order, ", ")
	}
	return tb.String()
}

func joinNames(names []string) string {
	if len(names) == 0 {
		return ""
	}
	var tb textbuf.Buffer
	tb.Join(names, ", ")
	return tb.String()
}

// joinAliases renders the alias NAMES alone. The expansion each name stands for
// is in the structured answer, which is where a reader who needs it looks: a
// chain in a table cell wraps the row past reading.
func joinAliases(aliases []Alias) string {
	if len(aliases) == 0 {
		return ""
	}
	var tb textbuf.Buffer
	for index := range aliases {
		if index > 0 {
			tb.Str(", ")
		}
		tb.Str(aliases[index].Name)
	}
	return tb.String()
}
