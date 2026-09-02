// Design: website/AI.md -- one page per live command, mapping it onto the vendor CLIs
// Detail: equivalents.go loads the curation and publishes the index this page hangs off.
// Related: catalog.go states what Ze itself says about the command.
package site

import (
	"html"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// renderEquivalentDetail publishes one command's own page and its mirror.
func renderEquivalentDetail(
	paths Paths, mapping *equivalentMapping, links pageLinks, row *equivalentRow, vendors []string,
) (string, error) {
	dest := equivalentsDirectory + "/" + row.Slug + "/" + pageIndexFile
	shell := pageShell{
		Title:       row.Command.Path + " - Command Equivalents - Ze",
		Description: commandLede(row.Command),
		Root:        equivalentsDetailRoot,
		Path:        dest,
		Sidebar:     pageSidebar(equivalentsDetailRoot, dest, links),
	}
	page := shell.render(equivalentDetailBody(mapping, row, vendors))
	mirror := equivalentDetailMirror(mapping, row, vendors)
	if err := writePublishedPage(paths.Output, dest, page, mirror); err != nil {
		return "", err
	}
	return "/" + strings.TrimSuffix(dest, pageIndexFile), nil
}

// equivalentDetailBody renders one command's page between <main> and </main>.
func equivalentDetailBody(mapping *equivalentMapping, row *equivalentRow, vendors []string) string {
	var body textbuf.Buffer
	body.Byte('\n').Str(`<section class="md-content command-equivalents command-equivalent-detail `).
		Str(`reveal cat-operate" aria-labelledby="command-equivalent-detail-title">`).Byte('\n')

	// The hero title and its lead are escaped exactly once. The retired
	// renderer escaped the lead twice, so a command holding <args> published a
	// literal "&lt;args&gt;" for a reader to puzzle over.
	body.Str(pageHero(
		html.EscapeString(row.Command.Path),
		html.EscapeString(commandLede(row.Command)),
		"Command map", ` id="command-equivalent-detail-title"`, heroClasses))
	body.Byte('\n').Str(`<div class="cmd-detail-grid">`).Byte('\n')
	body.Str(equivalentZeCard(row.Command))
	body.Str(equivalentIntentCard(row))
	for _, vendor := range vendors {
		body.Str(equivalentVendorCard(mapping, row, vendor))
	}
	body.Str("</div></section>\n")
	return body.String()
}

// equivalentZeCard renders what Ze itself says about the command.
func equivalentZeCard(command *catalogCommand) string {
	var card textbuf.Buffer
	card.Str(`<article class="cmd-detail-card cmd-detail-ze"><h2>Ze command</h2>`).Byte('\n')
	card.Str(`<dl class="cmd-meta">`).Byte('\n')
	writeDetailRow(&card, "Registry path", "<code>"+html.EscapeString(command.Path)+"</code>")
	if command.Usage != "" {
		writeDetailRow(&card, "Usage", "<code>"+html.EscapeString(command.Usage)+"</code>")
	}
	writeDetailRow(&card, "Mode", html.EscapeString(orNotListed(commandModeLabel(command.Mode))))
	writeDetailRow(&card, "Wire method", "<code>"+html.EscapeString(orNotListed(command.WireMethod))+"</code>")
	// The three rows below are written even when the catalog states nothing,
	// because for each of them the absence is itself the answer: no backend
	// list means every backend, and no task level means the MCP default.
	writeDetailRow(&card, "Backends", backendsHTML(command))
	writeDetailRow(&card, "Task support", html.EscapeString(commandTaskSupportLabel(command.TaskSupport)))
	writeDetailRow(&card, "Subcommands", subcommandsHTML(command))
	grouped := operatorsByAvailability(command)
	for _, availability := range availabilityOrder {
		names := grouped[availability]
		if len(names) == 0 {
			continue
		}
		writeDetailRow(&card, detailPipeLabel(availability, command.AnswerShape != ""),
			html.EscapeString(strings.Join(names, ", ")))
	}
	if len(grouped) == 0 && len(command.Pipes) == 0 && len(command.Aliases) == 0 {
		writeDetailRow(&card, "Pipes", "none: this command reaches no pipe layer")
	}
	if len(command.Pipes) != 0 {
		writeDetailRow(&card, "Command pipes", strings.Join(commandPipeLines(command), "<br>"))
	}
	if len(command.Aliases) != 0 {
		writeDetailRow(&card, "Pipe aliases", strings.Join(aliasLines(command), "<br>"))
	}
	if command.AnswerShape != "" {
		writeDetailRow(&card, "Answer shape", html.EscapeString(command.AnswerShape))
	}
	if len(command.AddressFields) != 0 {
		writeDetailRow(&card, "Address fields", html.EscapeString(strings.Join(command.AddressFields, ", ")))
	}
	card.Str("</dl>\n")
	// The summary is the page's lede, so the card explains rather than repeats
	// it. A command that declares no long form has nothing to explain, and an
	// empty heading would read as a claim the command model never made.
	if command.LongHelp != "" {
		card.Str("<h3>Description</h3><p>").Str(strings.ReplaceAll(html.EscapeString(command.LongHelp), "\n", "<br>")).
			Str("</p>\n")
	}
	card.Str("<h3>Arguments</h3>\n").Str(equivalentArgumentTable(command)).Str("\n</article>\n")
	return card.String()
}

// backendsHTML renders the data planes that implement one command. An empty
// list means the command model restricts it to none, so every backend does.
func backendsHTML(command *catalogCommand) string {
	if len(command.Backend) == 0 {
		return backendsUnrestricted
	}
	return codeSpanList(command.Backend)
}

// subcommandsHTML renders what an operator can type after one command.
func subcommandsHTML(command *catalogCommand) string {
	if len(command.Subcommands) == 0 {
		return subcommandsNone
	}
	return codeSpanList(command.Subcommands)
}

// commandPipeLines renders each of a command's own pipes as one line.
func commandPipeLines(command *catalogCommand) []string {
	lines := make([]string, 0, len(command.Pipes))
	for _, pipe := range command.Pipes {
		line := "<code>" + html.EscapeString(pipeDisplayName(pipe)) + "</code>"
		if pipe.Description != "" {
			line += ": " + html.EscapeString(pipe.Description)
		}
		lines = append(lines, line)
	}
	return lines
}

// aliasLines renders each of a command's pipe aliases as one line.
func aliasLines(command *catalogCommand) []string {
	lines := make([]string, 0, len(command.Aliases))
	for _, alias := range command.Aliases {
		line := "<code>" + html.EscapeString(alias.Name) + "</code>"
		if alias.Description != "" {
			line += ": " + html.EscapeString(alias.Description)
		}
		if alias.Expansion != "" {
			line += " (<code>" + html.EscapeString(alias.Expansion) + "</code>)"
		}
		lines = append(lines, line)
	}
	return lines
}

// detailPipeLabel names one availability on a detail page, where the row-data
// group reads differently once the command has declared an answer shape: the
// operators are available on ITS rows rather than on any answer that has rows.
func detailPipeLabel(availability string, hasAnswerShape bool) string {
	switch availability {
	case availabilityAlways:
		return "Pipes, always"
	case availabilityWithRows:
		if hasAnswerShape {
			return "Pipes, on its rows"
		}
		return "Pipes, when the answer has rows"
	case availabilityWhenStreaming:
		return "Pipes, while streaming"
	default:
		return "Pipes, local process only"
	}
}

// writeDetailRow writes one term and its value into a definition list.
func writeDetailRow(out *textbuf.Buffer, term, value string) {
	out.Str("<div><dt>").Str(term).Str("</dt><dd>").Str(value).Str("</dd></div>\n")
}

// The three ways a surface says a value is absent. Each one states WHY it is
// absent, so a reader can tell "nothing is listed" from "nobody declared one".
func orNotListed(value string) string {
	if value == "" {
		return "not listed"
	}
	return value
}

func orNotDeclared(value string) string {
	if value == "" {
		return "not declared"
	}
	return value
}

func orNone(value string) string {
	if value == "" {
		return "none"
	}
	return value
}

// commandLede answers the one line that opens a command's page: the summary the
// command declares, or a statement that the catalog listed none.
func commandLede(command *catalogCommand) string {
	if command.Description == "" {
		return "No description listed."
	}
	return command.Description
}

// equivalentArgumentTable renders the command's own arguments.
func equivalentArgumentTable(command *catalogCommand) string {
	if len(command.Args) == 0 {
		return "<p>No command-specific arguments listed.</p>"
	}
	var out textbuf.Buffer
	out.Str(`<table class="cmd-args"><thead><tr><th>Name</th><th>Type</th>`).
		Str("<th>Required</th><th>Values</th></tr></thead><tbody>\n")
	for _, argument := range command.Args {
		out.Str("<tr><td><code>").Str(html.EscapeString(argument.Name)).Str("</code></td><td>").
			Str(html.EscapeString(argument.Type)).Str("</td><td>").Str(argumentRequiredLabel(argument)).
			Str("</td><td>").Str(argumentValuesHTML(argument)).Str("</td></tr>\n")
	}
	out.Str("</tbody></table>")
	return out.String()
}

// argumentValuesHTML renders the closed set one argument accepts, and what an
// argument whose type states no closed set takes instead.
func argumentValuesHTML(argument catalogArg) string {
	if len(argument.Values) == 0 {
		return argumentValuesAny
	}
	return codeSpanList(argument.Values)
}

// equivalentIntentCard renders the curated migration intents for this command.
func equivalentIntentCard(row *equivalentRow) string {
	if len(row.Entries) == 0 {
		return `<article class="cmd-detail-card"><h2>Mapping status</h2><p>No vendor equivalent has ` +
			"been curated yet for this Ze command.</p></article>\n"
	}
	var card textbuf.Buffer
	card.Str(`<article class="cmd-detail-card"><h2>Mapping intents</h2>`).Byte('\n')
	for _, entry := range row.Entries {
		card.Str(`<section class="cmd-intent"><h3>`).Str(html.EscapeString(entry.Intent)).Str("</h3>\n")
		card.Str("<p><strong>Category:</strong> ").Str(html.EscapeString(entry.Category)).Str("</p>\n")
		if entry.Notes != "" {
			card.Str("<p>").Str(html.EscapeString(entry.Notes)).Str("</p>\n")
		}
		card.Str("</section>\n")
	}
	card.Str("</article>\n")
	return card.String()
}

// equivalentVendorCard renders one vendor's equivalents for this command, with
// the evidence behind each line.
func equivalentVendorCard(mapping *equivalentMapping, row *equivalentRow, vendor string) string {
	sources := mapping.sources()
	var card textbuf.Buffer
	card.Str(`<article class="cmd-detail-card cmd-vendor-detail"><h2>`).
		Str(html.EscapeString(mapping.Vendors[vendor].Label)).Str("</h2>\n")
	listed := false
	for _, entry := range row.Entries {
		rows := entry.Vendors[vendor]
		if len(rows) == 0 {
			continue
		}
		listed = true
		card.Str(`<section class="cmd-vendor-intent"><h3>`).Str(html.EscapeString(entry.Intent)).Str("</h3>\n")
		ordered := make([]vendorEquivalent, 0, len(rows))
		for _, item := range rows {
			ordered = append(ordered, vendorEquivalent{Intent: entry.Intent, Command: item})
		}
		sortEquivalentsByConfidence(ordered)
		for _, item := range ordered {
			card.Str(`<div class="cmd-vendor-line"><code>`).Str(html.EscapeString(item.Command.Command)).
				Str("</code> ").Str(confidenceBadge(item.Command.Confidence)).Byte(' ').
				Str(sourceLinks(item.Command.SourceRefs, sources)).Str("</div>\n")
			if item.Command.Notes != "" {
				card.Str(`<p class="cmd-note">`).Str(html.EscapeString(item.Command.Notes)).Str("</p>\n")
			}
		}
		card.Str("</section>\n")
	}
	if !listed {
		card.Str(`<p class="cmd-no-equivalent">No equivalent is listed for this vendor yet.</p>`).Byte('\n')
	}
	card.Str("</article>\n")
	return card.String()
}

// confidenceBadge renders how well evidenced one vendor command is.
func confidenceBadge(confidence string) string {
	return `<span class="cmd-confidence cmd-confidence-` + html.EscapeString(confidence) + `">` +
		html.EscapeString(confidenceLabel(confidence)) + "</span>"
}

// sourceLinks renders the vendor documents one command cites.
func sourceLinks(refs []string, sources map[string]equivalentSourceDoc) string {
	links := make([]string, 0, len(refs))
	for _, ref := range refs {
		source, known := sources[ref]
		if !known {
			continue
		}
		links = append(links, `<a href="`+html.EscapeString(source.URL)+
			`" target="_blank" rel="noopener">`+html.EscapeString(source.Label)+"</a>")
	}
	if len(links) == 0 {
		return ""
	}
	return `<span class="cmd-sources">source: ` + strings.Join(links, ", ") + "</span>"
}

// equivalentDetailMirror renders one command page's index.md sibling.
func equivalentDetailMirror(mapping *equivalentMapping, row *equivalentRow, vendors []string) string {
	command := row.Command
	grouped := operatorsByAvailability(command)
	var out textbuf.Buffer
	out.Str("# `").Str(markdownCell(command.Path)).Str("`\n\n")
	out.Str(markdownCell(commandLede(command))).Str("\n\n## Ze command\n\n")
	out.Str("- Registry path: `").Str(markdownCell(command.Path)).Str("`\n")
	if command.Usage != "" {
		out.Str("- Usage: `").Str(markdownCell(command.Usage)).Str("`\n")
	}
	out.Str("- Mode: ").Str(markdownCell(commandModeLabel(command.Mode))).Byte('\n')
	out.Str("- Wire method: `").Str(markdownCell(orNotListed(command.WireMethod))).Str("`\n")
	out.Str("- Backends: ").Str(backendsMirror(command)).Byte('\n')
	out.Str("- Task support: ").Str(markdownCell(commandTaskSupportLabel(command.TaskSupport))).Byte('\n')
	out.Str("- Subcommands: ").Str(subcommandsMirror(command)).Byte('\n')
	out.Str("- Answer shape: ").Str(markdownCell(orNotDeclared(command.AnswerShape))).Byte('\n')
	out.Str("- Address fields: ").Str(orNone(strings.Join(command.AddressFields, ", "))).Byte('\n')
	for _, availability := range availabilityOrder {
		out.Str("- ").Str(detailPipeLabel(availability, command.AnswerShape != "")).Str(": ").
			Str(orNone(strings.Join(grouped[availability], ", "))).Byte('\n')
	}
	out.Str("- Command pipes: ").Str(orNone(commandPipeMirrorList(command))).Byte('\n')
	out.Str("- Pipe aliases: ").Str(orNone(aliasMirrorList(command))).Str("\n\n")
	if command.LongHelp != "" {
		out.Str(markdownCell(command.LongHelp)).Str("\n\n")
	}
	out.Str(argumentMirrorTable(command))
	out.Str("## Mapping intents\n\n")
	if len(row.Entries) == 0 {
		out.Str("No vendor equivalent has been curated yet for this Ze command.\n\n")
	}
	for _, entry := range row.Entries {
		out.Str("### ").Str(entry.Intent).Str("\n\nCategory: ").Str(entry.Category).Str("\n\n")
		if entry.Notes != "" {
			out.Str(markdownCell(entry.Notes)).Str("\n\n")
		}
	}
	out.Str("## Vendor equivalents\n\n")
	for _, vendor := range vendors {
		out.Str("### ").Str(mapping.vendorLabel(vendor)).Byte('\n')
		commands := row.vendorCommands(vendor)
		if len(commands) == 0 {
			out.Str("\nNo equivalent listed.\n\n")
			continue
		}
		for _, item := range commands {
			out.Str("- `").Str(markdownCell(item.Command.Command)).Str("` (").Str(item.Command.Confidence).Str(", ").
				Str(orNoSourceRef(strings.Join(item.Command.SourceRefs, ", "))).Str(")\n")
			if item.Intent != "" {
				out.Str("  - Intent: ").Str(markdownCell(item.Intent)).Byte('\n')
			}
			if item.Command.Notes != "" {
				out.Str("  - Note: ").Str(markdownCell(item.Command.Notes)).Byte('\n')
			}
		}
		out.Byte('\n')
	}
	return strings.TrimRight(out.String(), "\n") + "\n"
}

// backendsMirror writes the data planes that implement one command, and what
// an empty list means: the command model restricts it to none.
func backendsMirror(command *catalogCommand) string {
	if len(command.Backend) == 0 {
		return backendsUnrestricted
	}
	return markdownCodeList(command.Backend)
}

// subcommandsMirror writes what an operator can type after one command.
func subcommandsMirror(command *catalogCommand) string {
	if len(command.Subcommands) == 0 {
		return subcommandsNone
	}
	return markdownCodeList(command.Subcommands)
}

// argumentMirrorTable writes the arguments section of one detail mirror.
//
// It is a table rather than the one-line-per-argument form the CLI reference
// mirror takes, because it states the same four columns the page's own table
// states and a mirror reads best beside the page it mirrors.
func argumentMirrorTable(command *catalogCommand) string {
	var out textbuf.Buffer
	out.Str("## Arguments\n\n")
	if len(command.Args) == 0 {
		out.Str("No command-specific arguments listed.\n\n")
		return out.String()
	}
	out.Str("| Name | Type | Required | Values |\n| --- | --- | --- | --- |\n")
	for _, argument := range command.Args {
		values := argumentValuesAny
		if len(argument.Values) != 0 {
			values = markdownCodeList(argument.Values)
		}
		out.Str("| `").Str(markdownCell(argument.Name)).Str("` | ").Str(markdownCell(argument.Type)).Str(" | ").
			Str(argumentRequiredLabel(argument)).Str(" | ").Str(values).Str(" |\n")
	}
	out.Byte('\n')
	return out.String()
}

// orNoSourceRef says that a curated command cites no vendor document, rather
// than leaving the parenthesis holding a comma and nothing else.
func orNoSourceRef(value string) string {
	if value == "" {
		return "no source ref"
	}
	return value
}

// commandPipeMirrorList writes a command's own pipes as one mirror line.
func commandPipeMirrorList(command *catalogCommand) string {
	values := make([]string, 0, len(command.Pipes))
	for _, pipe := range command.Pipes {
		value := "`" + markdownCell(pipeDisplayName(pipe)) + "`"
		if pipe.Description != "" {
			value += ": " + markdownCell(pipe.Description)
		}
		values = append(values, value)
	}
	return strings.Join(values, "; ")
}

// aliasMirrorList writes a command's pipe aliases as one mirror line.
func aliasMirrorList(command *catalogCommand) string {
	values := make([]string, 0, len(command.Aliases))
	for _, alias := range command.Aliases {
		value := "`" + markdownCell(alias.Name) + "`"
		if alias.Description != "" {
			value += ": " + markdownCell(alias.Description)
		}
		if alias.Expansion != "" {
			value += " (`" + markdownCell(alias.Expansion) + "`)"
		}
		values = append(values, value)
	}
	return strings.Join(values, "; ")
}
