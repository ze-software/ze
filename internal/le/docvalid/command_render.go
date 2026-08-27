// Design: docs/architecture/core-design.md -- published command surfaces come from the live catalog
//
// command_render.go renders the command-contract fragments docvalid validates.
// It deliberately lives beside the validators so generation and validation
// share the same typed catalog in-process.
package docvalid

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var renderCommandSurfaces = renderNativeCommandSurfaces

// RenderCommandSurfaces writes the native command-facing HTML, Markdown, and
// llms surfaces represented by catalogJSON.
func RenderCommandSurfaces(root string, catalogJSON []byte) error {
	commands, err := parseCommandCatalog("command catalog", catalogJSON)
	if err != nil {
		return err
	}
	return renderCommandSurfaces(root, commands)
}

func renderNativeCommandSurfaces(root string, commands []publishedCommand) error {
	slugs := make(map[string]string, len(commands))
	for _, command := range commands {
		slug := commandSurfaceSlug(command.Path)
		if slug == "" {
			return fmt.Errorf("command surface slug is empty for %q", command.Path)
		}
		if previous, exists := slugs[slug]; exists {
			return fmt.Errorf(
				"command surface slug %q collides for %q and %q",
				slug, previous, command.Path,
			)
		}
		slugs[slug] = command.Path
	}

	files := map[string][]byte{
		"reference/cli/index.html":                 renderPrimaryCommandHTML(commands),
		"reference/cli/index.md":                   renderPrimaryCommandMarkdown(commands),
		"reference/command-equivalents/index.html": renderEquivalentIndexHTML(commands),
		"reference/command-equivalents/index.md":   renderEquivalentIndexMarkdown(commands),
		"llms.txt":                                 renderCommandLLMS(commands),
	}
	for _, command := range commands {
		slug := commandSurfaceSlug(command.Path)
		files[filepath.Join("reference", "command-equivalents", slug, "index.html")] = renderEquivalentHTML(command)
		files[filepath.Join("reference", "command-equivalents", slug, "index.md")] = renderEquivalentMarkdown(command)
	}
	for name, content := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("create command surface directory %s: %w", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, content, 0o644); err != nil {
			return fmt.Errorf("write command surface %s: %w", path, err)
		}
	}
	return nil
}

type operatorCatalogRow struct {
	name, class, description string
	availability             []string
}

func renderPrimaryCommandHTML(commands []publishedCommand) []byte {
	var out strings.Builder
	out.WriteString("<!doctype html><html><body>\n<section class=\"cli-pipe-guide\"><table><tbody>\n")
	rows := make(map[string]*operatorCatalogRow)
	for _, command := range commands {
		for _, operator := range command.Operators {
			row := rows[operator.Name]
			if row == nil {
				row = &operatorCatalogRow{name: operator.Name, class: primaryOperatorClassLabel(operator.Class), description: operator.Description}
				rows[operator.Name] = row
			}
			label := commandAvailabilityLabel(operator.Available)
			if !containsString(row.availability, label) {
				row.availability = append(row.availability, label)
			}
			if operator.LocalOnly && !containsString(row.availability, "Local process only") {
				row.availability = append(row.availability, "Local process only")
			}
		}
	}
	names := make([]string, 0, len(rows))
	for name := range rows {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		row := rows[name]
		fmt.Fprintf(&out, "<tr><td><code>%s</code></td><td>%s</td><td>%s</td><td>%s</td></tr>\n",
			html.EscapeString(row.name), html.EscapeString(row.class), html.EscapeString(strings.Join(row.availability, ", ")), html.EscapeString(row.description))
	}
	out.WriteString("</tbody></table></section>\n<table><tbody>\n")
	for _, command := range commands {
		fmt.Fprintf(
			&out,
			"<tr id=\"cmd-%s\"><td><code>%s</code></td><td>%s</td><td>%s</td><td>",
			commandSurfaceSlug(command.Path),
			html.EscapeString(command.Path),
			html.EscapeString(command.Mode),
			html.EscapeString(command.Description),
		)
		if command.AnswerShape != "" {
			fmt.Fprintf(&out, "<span>Answer shape</span><code>%s</code>", html.EscapeString(command.AnswerShape))
		}
		if len(command.AddressFields) != 0 {
			fmt.Fprintf(&out, "<span>Address fields</span><code>%s</code>", html.EscapeString(strings.Join(command.AddressFields, " · ")))
		}
		writePrimaryOperatorGroups(&out, command)
		if len(command.Pipes) != 0 {
			out.WriteString("<strong>Command pipes</strong><div class=\"cli-pipe-chips\">")
			for _, pipe := range command.Pipes {
				fmt.Fprintf(&out, "<code>%s</code>", html.EscapeString(commandPipeDisplayName(pipe)))
			}
			out.WriteString("</div><dl>")
			for _, pipe := range command.Pipes {
				fmt.Fprintf(&out, "<dt><code>%s</code></dt><dd>%s</dd>", html.EscapeString(commandPipeDisplayName(pipe)), html.EscapeString(pipe.Description))
			}
			out.WriteString("</dl>")
		}
		if len(command.Aliases) != 0 {
			out.WriteString("<strong>Aliases</strong><dl>")
			for _, alias := range command.Aliases {
				fmt.Fprintf(&out, "<dt><code>%s</code></dt><dd>%s <code>%s</code></dd>", html.EscapeString(alias.Name), html.EscapeString(alias.Description), html.EscapeString(alias.Expansion))
			}
			out.WriteString("</dl>")
		}
		out.WriteString("</td></tr>\n")
	}
	out.WriteString("</tbody></table>\n</body></html>\n")
	return []byte(out.String())
}

func writePrimaryOperatorGroups(out *strings.Builder, command publishedCommand) {
	for _, group := range []struct{ availability, label string }{
		{"always", "Always"}, {"with-rows", "With rows"}, {"when-streaming", "While streaming"}, {"local-only", "Local process only"},
	} {
		names := commandOperatorNames(command, group.availability)
		if len(names) != 0 {
			fmt.Fprintf(out, "<span>%s</span><code>%s</code>", group.label, html.EscapeString(strings.Join(names, " · ")))
		}
	}
}

func renderPrimaryCommandMarkdown(commands []publishedCommand) []byte {
	var out strings.Builder
	out.WriteString("# CLI command catalog\n\n| Command | Mode | Description | Contract |\n|---|---|---|---|\n")
	for _, command := range commands {
		metadata := make([]string, 0, 8)
		if command.AnswerShape != "" {
			metadata = append(metadata, "Answer shape: "+markdownCodeLiteral(command.AnswerShape))
		}
		if len(command.AddressFields) != 0 {
			metadata = append(metadata, "Address fields: "+markdownCodeList(command.AddressFields))
		}
		for _, group := range []struct{ availability, label string }{{"always", "Always"}, {"with-rows", "With rows"}, {"when-streaming", "While streaming"}, {"local-only", "Local process only"}} {
			if names := commandOperatorNames(command, group.availability); len(names) != 0 {
				metadata = append(metadata, group.label+": "+markdownCodeList(names))
			}
		}
		if len(command.Pipes) != 0 {
			names := make([]string, 0, len(command.Pipes))
			for _, pipe := range command.Pipes {
				names = append(names, commandPipeDisplayName(pipe))
			}
			metadata = append(metadata, "Command: "+markdownCodeList(names))
		}
		if len(command.Aliases) != 0 {
			aliases := make([]string, 0, len(command.Aliases))
			for _, alias := range command.Aliases {
				aliases = append(aliases, alias.Name+" -> "+alias.Expansion)
			}
			metadata = append(metadata, "Aliases: "+markdownCodeList(aliases))
		}
		out.WriteString("| ")
		out.WriteString(markdownCodeLiteral(commandMarkdownValue(command.Path)))
		out.WriteString(" | ")
		out.WriteString(commandMarkdownValue(command.Mode))
		out.WriteString(" | ")
		out.WriteString(markdownLiteralProse(command.Description))
		out.WriteString(" | ")
		out.WriteString(strings.Join(metadata, "<br>"))
		out.WriteString(" |\n")
	}
	return []byte(out.String())
}

func renderEquivalentIndexHTML(commands []publishedCommand) []byte {
	var out strings.Builder
	out.WriteString("<!doctype html><html><body><table><tbody>\n")
	for _, command := range commands {
		fmt.Fprintf(&out, "<tr id=\"cmd-eq-%s\"><td><code>%s</code></td></tr>\n", commandSurfaceSlug(command.Path), html.EscapeString(command.Path))
	}
	out.WriteString("</tbody></table></body></html>\n")
	return []byte(out.String())
}

func renderEquivalentIndexMarkdown(commands []publishedCommand) []byte {
	var out strings.Builder
	out.WriteString("# Command Equivalents\n\n")
	for _, command := range commands {
		fmt.Fprintf(&out, "- [%s](%s/)\n", markdownCodeLiteral(command.Path), commandSurfaceSlug(command.Path))
	}
	return []byte(out.String())
}

func renderEquivalentHTML(command publishedCommand) []byte {
	var out strings.Builder
	out.WriteString("<!doctype html><html><body>\n<article class=\"cmd-detail-card cmd-detail-ze\">\n")
	fmt.Fprintf(&out, "<div><dt>Registry path</dt><dd><code>%s</code></dd></div>", html.EscapeString(command.Path))
	if command.AnswerShape != "" {
		fmt.Fprintf(&out, "<div><dt>Answer shape</dt><dd>%s</dd></div>", html.EscapeString(command.AnswerShape))
	}
	if len(command.AddressFields) != 0 {
		fmt.Fprintf(&out, "<div><dt>Address fields</dt><dd>%s</dd></div>", html.EscapeString(strings.Join(command.AddressFields, ", ")))
	}
	for _, group := range []struct{ availability, label string }{
		{"always", "Pipes, always"},
		{"with-rows", equivalentAvailabilityLabel("with-rows", command.AnswerShape != "")},
		{"when-streaming", "Pipes, while streaming"},
		{"local-only", "Pipes, local process only"},
	} {
		if names := commandOperatorNames(command, group.availability); len(names) != 0 {
			fmt.Fprintf(&out, "<div><dt>%s</dt><dd>%s</dd></div>", group.label, html.EscapeString(strings.Join(names, ", ")))
		}
	}
	if len(command.Pipes) != 0 {
		values := make([]string, 0, len(command.Pipes))
		for _, pipe := range command.Pipes {
			values = append(values, "<code>"+html.EscapeString(commandPipeDisplayName(pipe))+"</code>: "+html.EscapeString(pipe.Description))
		}
		fmt.Fprintf(&out, "<div><dt>Command pipes</dt><dd>%s</dd></div>", strings.Join(values, "<br>"))
	}
	if len(command.Aliases) != 0 {
		values := make([]string, 0, len(command.Aliases))
		for _, alias := range command.Aliases {
			values = append(values, "<code>"+html.EscapeString(alias.Name)+"</code>: "+html.EscapeString(alias.Description)+" (<code>"+html.EscapeString(alias.Expansion)+"</code>)")
		}
		fmt.Fprintf(&out, "<div><dt>Pipe aliases</dt><dd>%s</dd></div>", strings.Join(values, "<br>"))
	}
	out.WriteString("\n</article>\n</body></html>\n")
	return []byte(out.String())
}

func isCommonMarkASCIIPunctuation(value byte) bool {
	return value >= '!' && value <= '/' ||
		value >= ':' && value <= '@' ||
		value >= '[' && value <= '`' ||
		value >= '{' && value <= '~'
}

func markdownCodeLiteral(value string) string {
	longestRun := 0
	for index := 0; index < len(value); {
		if value[index] != '`' {
			index++
			continue
		}
		end := index + 1
		for end < len(value) && value[end] == '`' {
			end++
		}
		longestRun = max(longestRun, end-index)
		index = end
	}
	delimiter := strings.Repeat("`", longestRun+1)
	padding := ""
	if strings.HasPrefix(value, "`") || strings.HasSuffix(value, "`") ||
		strings.HasPrefix(value, " ") && strings.HasSuffix(value, " ") &&
			strings.Trim(value, " ") != "" {
		padding = " "
	}
	return delimiter + padding + value + padding + delimiter
}

func markdownLiteralProse(value string) string {
	value = strings.NewReplacer(
		"\r\n", " ",
		"\r", " ",
		"\n", " ",
	).Replace(value)
	var escaped strings.Builder
	escaped.Grow(len(value))
	for index := 0; index < len(value); index++ {
		character := value[index]
		if character == '\\' {
			escaped.WriteString(`\\`)
			continue
		}
		if isCommonMarkASCIIPunctuation(character) {
			escaped.WriteByte('\\')
		}
		escaped.WriteByte(character)
	}
	return escaped.String()
}

func renderEquivalentMarkdown(command publishedCommand) []byte {
	var out strings.Builder
	fmt.Fprintf(&out, "# %s\n\n## Ze command\n\n- Registry path: %s\n",
		markdownCodeLiteral(command.Path), markdownCodeLiteral(command.Path))
	if command.AnswerShape != "" {
		fmt.Fprintf(&out, "- Answer shape: %s\n", commandMarkdownValue(command.AnswerShape))
	}
	if len(command.AddressFields) != 0 {
		fmt.Fprintf(&out, "- Address fields: %s\n", strings.Join(command.AddressFields, ", "))
	}
	for _, group := range []struct{ availability, label string }{{"always", "Pipes, always"}, {"with-rows", "Pipes, on rows"}, {"when-streaming", "Pipes, while streaming"}, {"local-only", "Pipes, local process only"}} {
		if names := commandOperatorNames(command, group.availability); len(names) != 0 {
			fmt.Fprintf(&out, "- %s: %s\n", group.label, strings.Join(names, ", "))
		}
	}
	if len(command.Pipes) == 0 {
		out.WriteString("- Command pipes: none\n")
	} else {
		values := make([]string, 0, len(command.Pipes))
		for _, pipe := range command.Pipes {
			value := markdownCodeLiteral(commandPipeDisplayName(pipe))
			if pipe.Description != "" {
				value += ": " + markdownLiteralProse(pipe.Description)
			}
			values = append(values, value)
		}
		fmt.Fprintf(&out, "- Command pipes: %s\n", strings.Join(values, "; "))
	}
	if len(command.Aliases) == 0 {
		out.WriteString("- Pipe aliases: none\n")
	} else {
		values := make([]string, 0, len(command.Aliases))
		for _, alias := range command.Aliases {
			value := markdownCodeLiteral(alias.Name)
			if alias.Description != "" {
				value += ": " + markdownLiteralProse(alias.Description)
			}
			if alias.Expansion != "" {
				value += " (" + markdownCodeLiteral(alias.Expansion) + ")"
			}
			values = append(values, value)
		}
		fmt.Fprintf(&out, "- Pipe aliases: %s\n", strings.Join(values, "; "))
	}
	out.WriteString("\n## Mapping intents\n")
	return []byte(out.String())
}

func renderCommandLLMS(commands []publishedCommand) []byte {
	var out strings.Builder
	out.WriteString("# Ze\n\n## CLI command surface\n\n")
	for _, command := range commands {
		meta := []string{command.Mode}
		if command.WireMethod != "" {
			meta = append(meta, "wire "+command.WireMethod)
		}
		pipeGroups := make([]string, 0, 4)
		for _, availability := range []string{"always", "with-rows", "when-streaming", "local-only"} {
			if names := commandOperatorNames(command, availability); len(names) != 0 {
				pipeGroups = append(pipeGroups, availability+": "+strings.Join(names, " "))
			}
		}
		if len(pipeGroups) != 0 {
			meta = append(meta, "pipes "+strings.Join(pipeGroups, ", "))
		}
		if command.AnswerShape != "" {
			meta = append(meta, "shape "+command.AnswerShape)
		}
		if len(command.AddressFields) != 0 {
			meta = append(meta, "address-fields "+strings.Join(command.AddressFields, " "))
		}
		if len(command.Pipes) != 0 {
			names := make([]string, 0, len(command.Pipes))
			for _, pipe := range command.Pipes {
				names = append(names, markdownCodeLiteral(pipe.Name))
			}
			meta = append(meta, "filters "+strings.Join(names, " "))
		}
		if len(command.Aliases) != 0 {
			values := make([]string, 0, len(command.Aliases))
			for _, alias := range command.Aliases {
				values = append(values,
					markdownCodeLiteral(alias.Name)+"="+markdownCodeLiteral(alias.Expansion))
			}
			meta = append(meta, "aliases "+strings.Join(values, ", "))
		}
		if len(command.Args) != 0 {
			values := make([]string, 0, len(command.Args))
			for _, arg := range command.Args {
				values = append(values, arg.Name+":"+arg.Type)
			}
			meta = append(meta, "args "+strings.Join(values, ", "))
		}
		fmt.Fprintf(&out, "- %s (%s): %s\n", markdownCodeLiteral(command.Path), strings.Join(meta, "; "), markdownLiteralProse(command.Description))
	}
	return []byte(out.String())
}

func commandPipeDisplayName(pipe publishedCommandPipe) string {
	if pipe.TakesArg {
		return pipe.Name + " <value>"
	}
	return pipe.Name
}

func markdownCodeList(values []string) string {
	quoted := make([]string, len(values))
	for index, value := range values {
		quoted[index] = markdownCodeLiteral(value)
	}
	return strings.Join(quoted, ", ")
}
