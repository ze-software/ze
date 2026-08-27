package wikicatalog

import (
	"bytes"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// Render produces the command catalog's canonical Markdown bytes, including
// the final newline.
func Render(entries []Entry) ([]byte, error) {
	groups := make(map[string][]Entry)
	for _, entry := range entries {
		words := strings.Fields(entry.Path)
		verb := entry.Path
		if len(words) > 0 {
			verb = words[0]
		}
		groups[verb] = append(groups[verb], entry)
	}

	verbs := make([]string, 0, len(groups))
	for verb := range groups {
		verbs = append(verbs, verb)
	}
	slices.Sort(verbs)

	var out bytes.Buffer
	line(&out, "> **Pre-Alpha.** This page is auto-generated from `ze help command --json`.")
	line(&out, "")
	line(&out, "# Command Catalog")
	line(&out, "")
	line(&out, "## Contents")
	line(&out, "")
	for _, verb := range verbs {
		out.WriteString("- [")
		out.WriteString(verb)
		out.WriteString("](#")
		out.WriteString(verb)
		out.WriteString(") (")
		writeInt(&out, len(groups[verb]))
		line(&out, ")")
	}
	line(&out, "")

	for _, verb := range verbs {
		group := groups[verb]
		slices.SortFunc(group, func(left, right Entry) int {
			return strings.Compare(left.Path, right.Path)
		})

		out.WriteString("## ")
		line(&out, verb)
		line(&out, "")
		line(&out, "| Command | Mode | Description |")
		line(&out, "|---------|------|-------------|")
		for _, entry := range group {
			out.WriteString("| `")
			out.WriteString(entry.Path)
			out.WriteString("` | ")
			out.WriteString(entry.Mode)
			out.WriteString(" | ")
			out.WriteString(strings.ReplaceAll(firstLine(entry.Description), "|", "\\|"))
			line(&out, " |")
		}
		line(&out, "")

		for _, entry := range group {
			if !needsDetail(entry) {
				continue
			}
			out.WriteString("### `")
			out.WriteString(entry.Path)
			line(&out, "`")
			if err := renderDetail(&out, entry); err != nil {
				return nil, err
			}
			line(&out, "")
		}
	}

	line(&out, "---")
	line(&out, "")
	out.WriteByte('*')
	writeInt(&out, len(entries))
	line(&out, " commands total.*")
	return out.Bytes(), nil
}

func firstLine(description string) string {
	first, _, _ := strings.Cut(description, "\n")
	return first
}

func needsDetail(entry Entry) bool {
	return len(entry.Args) > 0 || len(entry.Pipes) > 0 || len(entry.Subcommands) > 0 ||
		len(entry.Backend) > 0 || entry.TaskSupport != "" || len(entry.Operators) > 0 ||
		strings.Contains(entry.Description, "\n")
}

func renderDetail(out *bytes.Buffer, entry Entry) error {
	if strings.Contains(entry.Description, "\n") {
		line(out, "")
		for _, descriptionLine := range strings.Split(entry.Description, "\n") {
			line(out, descriptionLine)
		}
	}

	line(out, "")
	out.WriteString("Mode: ")
	out.WriteString(entry.Mode)
	if entry.WireMethod != "" {
		out.WriteString(" | Wire: `")
		out.WriteString(entry.WireMethod)
		out.WriteByte('`')
	}
	out.WriteByte('\n')
	if entry.AnswerShape != "" {
		out.WriteString("Answer shape: `")
		out.WriteString(entry.AnswerShape)
		line(out, "`")
	}
	if len(entry.AddressFields) > 0 {
		out.WriteString("Address fields: ")
		writeCodeList(out, entry.AddressFields)
		out.WriteByte('\n')
	}

	if len(entry.Backend) > 0 {
		line(out, "")
		out.WriteString("**Requires backend:** ")
		writeCodeList(out, entry.Backend)
		out.WriteByte('\n')
	}
	if entry.TaskSupport != "" {
		out.WriteString("**Task support:** ")
		line(out, entry.TaskSupport)
	}

	if len(entry.Args) > 0 {
		line(out, "")
		line(out, "**Arguments:**")
		line(out, "")
		line(out, "| Name | Type | Required | Values |")
		line(out, "|------|------|----------|--------|")
		for _, argument := range entry.Args {
			out.WriteString("| `")
			out.WriteString(argument.Name)
			out.WriteString("` | ")
			out.WriteString(argument.Type)
			out.WriteString(" | ")
			if argument.Mandatory {
				out.WriteString("yes")
			}
			out.WriteString(" | ")
			out.WriteString(strings.Join(argument.Values, ", "))
			line(out, " |")
		}
	}

	if len(entry.Operators) > 0 || len(entry.Pipes) > 0 || len(entry.Aliases) > 0 {
		if err := renderPipes(out, entry); err != nil {
			return err
		}
	} else if entry.WireMethod == "" {
		line(out, "")
		line(out, "**Pipes:** not available (offline command)")
	}

	if len(entry.Subcommands) > 0 {
		line(out, "")
		out.WriteString("**Subcommands:** ")
		writeCodeList(out, entry.Subcommands)
		out.WriteByte('\n')
	}
	return nil
}

func renderPipes(out *bytes.Buffer, entry Entry) error {
	var unknown []string
	for _, operator := range entry.Operators {
		switch operator.Available {
		case "always", "with-rows", "when-streaming":
		default:
			unknown = append(unknown, operatorName(operator.Name))
		}
	}
	if len(unknown) > 0 {
		return fmt.Errorf("unknown operator availability for %s", strings.Join(unknown, ", "))
	}

	line(out, "")
	line(out, "**Pipes:**")
	always := operatorNames(entry.Operators, "always", false)
	withRows := operatorNames(entry.Operators, "with-rows", false)
	streaming := operatorNames(entry.Operators, "when-streaming", false)
	localOnly := operatorNames(entry.Operators, "", true)
	if len(always) > 0 {
		out.WriteString("Always: ")
		writeCodeList(out, always)
		out.WriteByte('\n')
	}
	if len(withRows) > 0 {
		line(out, "")
		out.WriteString("When the answer has rows: ")
		writeCodeList(out, withRows)
		line(out, " -- this command has not declared its answer shape, so each of these applies to an answer that carries rows and is refused by name over one that does not.")
	}
	if len(streaming) > 0 {
		line(out, "")
		out.WriteString("While the command keeps answering: ")
		writeCodeList(out, streaming)
		out.WriteByte('\n')
	}
	if len(localOnly) > 0 {
		line(out, "")
		out.WriteString("Local process only: ")
		writeCodeList(out, localOnly)
		line(out, " -- daemon-expanded SSH and web chains refuse these operators.")
	}
	if len(entry.Aliases) > 0 {
		line(out, "")
		line(out, "Named chains:")
		for _, alias := range entry.Aliases {
			out.WriteString("- `")
			out.WriteString(alias.Name)
			out.WriteString("` -- ")
			out.WriteString(alias.Description)
			out.WriteString(" (`")
			out.WriteString(alias.Expansion)
			line(out, "`)")
		}
	}
	if len(entry.Pipes) > 0 {
		line(out, "")
		line(out, "Command-specific:")
		for _, pipe := range entry.Pipes {
			out.WriteString("- `")
			out.WriteString(pipe.Name)
			out.WriteByte('`')
			if pipe.TakesArg {
				out.WriteString(" `<value>`")
			}
			out.WriteString(" -- ")
			line(out, pipe.Description)
		}
	}
	return nil
}

func operatorNames(operators []Operator, availability string, localOnly bool) []string {
	names := make([]string, 0, len(operators))
	for _, operator := range operators {
		if localOnly {
			if operator.LocalOnly {
				names = append(names, operator.Name)
			}
			continue
		}
		if operator.Available == availability {
			names = append(names, operator.Name)
		}
	}
	return names
}

func operatorName(name string) string {
	if name == "" {
		return "<unnamed>"
	}
	return name
}

func writeCodeList(out *bytes.Buffer, values []string) {
	for index, value := range values {
		if index > 0 {
			out.WriteString(", ")
		}
		out.WriteByte('`')
		out.WriteString(value)
		out.WriteByte('`')
	}
}

func line(out *bytes.Buffer, value string) {
	out.WriteString(value)
	out.WriteByte('\n')
}

func writeInt(out *bytes.Buffer, value int) {
	var digits [20]byte
	out.Write(strconv.AppendInt(digits[:0], int64(value), 10))
}
