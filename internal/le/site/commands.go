// Design: website/AI.md -- the CLI reference is one published page of the live catalog
// Detail: catalog.go reads and groups the catalog; shell.go wraps this body.
// Related: equivalents.go publishes the same commands as a vendor map.
package site

import (
	"html"
	"slices"
	"strconv"
	"strings"
)

// The CLI reference is registered from here, so a build discovers it through
// the registry rather than through a call the build states by name.
func init() {
	registerProducer(Producer{Name: "cli-reference", Render: renderCLIReference})
}

// The published route of the CLI reference, and the relative path back to the
// site root from it.
const (
	cliReferenceDest = "reference/cli/" + pageIndexFile
	cliReferenceRoot = "../../"
)

// renderCLIReference publishes reference/cli/index.html and its mirror.
func renderCLIReference(paths Paths) ([]string, error) {
	commands, err := loadCommandCatalog(paths.Output)
	if err != nil {
		return nil, err
	}
	links, err := loadPageLinks(paths.Source)
	if err != nil {
		return nil, err
	}
	groups := groupCommands(commands)

	shell := pageShell{
		Title:       "CLI Reference - Ze",
		Description: cliReferenceDescription(len(commands), len(groups)),
		Root:        cliReferenceRoot,
		Path:        cliReferenceDest,
		Sidebar:     pageSidebar(cliReferenceRoot, cliReferenceDest, links),
	}
	page := shell.render(cliReferenceBody(commands, groups))
	mirror := cliReferenceMirror(commands, groups)
	if err := writePublishedPage(paths.Output, cliReferenceDest, page, mirror); err != nil {
		return nil, err
	}
	return []string{"/" + strings.TrimSuffix(cliReferenceDest, pageIndexFile)}, nil
}

// cliReferenceDescription is the meta description and the social description.
// It states the two numbers a reader wants before they open a 300KB page.
func cliReferenceDescription(commands, groups int) string {
	return "Every ze command, generated live from the binary's own command registry -- " +
		strconv.Itoa(commands) + " commands across " + strconv.Itoa(groups) + " groups."
}

// cliReferenceBody renders the page between <main> and </main>.
func cliReferenceBody(commands []catalogCommand, groups []commandGroup) string {
	var body strings.Builder
	body.WriteString("\n            <section aria-labelledby=\"cli-title\" class=\"md-content reveal cat-operate\">\n")
	body.WriteString(pageHero("CLI Reference", cliReferenceLead(len(commands), len(groups)), "Reference", ` id="cli-title"`, heroClasses))
	body.WriteString("\n")
	body.WriteString(renderOperatorGuide(commands))
	body.WriteString(`                <div class="cli-search-wrap">` + "\n")
	body.WriteString(`                    <input id="cli-search" type="search" autocomplete="off" ` +
		`placeholder="Filter commands (e.g. bgp, traceroute, monitor)..." aria-label="Filter commands" />` + "\n")
	body.WriteString(`                    <div id="cli-suggestions" class="cli-suggestions" hidden></div>` + "\n")
	body.WriteString("                </div>\n")
	for index := range groups {
		writeCommandGroup(&body, &groups[index])
	}
	body.WriteString("            </section>\n")
	return body.String()
}

// cliReferenceLead is the hero paragraph. It is markup rather than plain text,
// because it names the catalog file as a link and the command a reader can run.
func cliReferenceLead(commands, groups int) string {
	return strconv.Itoa(commands) + " commands across " + strconv.Itoa(groups) +
		" groups, generated straight from <code>ze help command --json</code> -- the same live " +
		"command registry the binary itself uses, including the pipe operators available to each " +
		`command. Full machine-readable list: <a href="` + cliReferenceRoot + catalogFile + `">` +
		catalogFile + "</a>."
}

// guideOperator is one row of the shared operator table: the facts that belong
// to the operator rather than to a command that accepts it.
type guideOperator struct {
	name, class, description string
	availability             []string
}

// renderOperatorGuide renders the shared table of pipe operators.
//
// The operators are collected across every command, because an operator's class
// and description are properties of the operator rather than of the command that
// accepts it. The availability column is the union, so `save` reads "Always,
// Local process only" once instead of contradicting itself between rows.
//
// The catalog arrives sorted by command path, so the first command that names an
// operator decides its position, and the order is the same on every build.
func renderOperatorGuide(commands []catalogCommand) string {
	rows := make(map[string]*guideOperator, 24)
	var order []*guideOperator
	for index := range commands {
		for _, operator := range commands[index].Operators {
			row := rows[operator.Name]
			if row == nil {
				row = &guideOperator{name: operator.Name, class: operator.Class, description: operator.Description}
				rows[operator.Name] = row
				order = append(order, row)
			}
			row.availability = appendOnce(row.availability, operator.Available)
			if operator.LocalOnly {
				row.availability = appendOnce(row.availability, availabilityLocalOnly)
			}
		}
	}
	if len(order) == 0 {
		return ""
	}
	var guide strings.Builder
	guide.WriteString(`<section class="cli-pipe-guide" aria-labelledby="cli-pipe-guide-title">` + "\n")
	guide.WriteString(`<div class="cli-pipe-guide-head">` + "\n")
	guide.WriteString(`<span class="tag">Pipes</span>` + "\n")
	guide.WriteString(`<div><h2 id="cli-pipe-guide-title">Pipe operators</h2>` + "\n")
	guide.WriteString("<p>Each command row names the operators it accepts after <code>|</code>. " +
		"Availability comes from the live command registry: operators may require row data, " +
		"a streaming answer, or expansion by the operator's local process.</p></div>\n")
	guide.WriteString("</div>\n<details>\n")
	guide.WriteString("<summary>Operator reference <span>" + strconv.Itoa(len(order)) + "</span></summary>\n")
	guide.WriteString("<table><thead><tr><th>Operator</th><th>Class</th><th>Available</th>" +
		"<th>Description</th></tr></thead><tbody>\n")
	for _, row := range order {
		guide.WriteString("<tr><td><code>" + html.EscapeString(row.name) + "</code></td><td>" +
			html.EscapeString(operatorClassLabel(row.class)) + "</td><td>" +
			html.EscapeString(availabilityList(row.availability)) + "</td><td>" +
			html.EscapeString(row.description) + "</td></tr>\n")
	}
	guide.WriteString("</tbody></table>\n</details>\n</section>\n")
	return guide.String()
}

// operatorClassLabel answers the reader's word for one operator class, or the
// raw value when the catalog states a class this site has no word for.
func operatorClassLabel(class string) string {
	if label, known := operatorClassLabels[class]; known {
		return label
	}
	return class
}

// availabilityList answers one operator's availabilities as a reader's phrase,
// in availabilityOrder rather than in the order the commands happened to state.
func availabilityList(values []string) string {
	labels := make([]string, 0, len(values))
	for _, availability := range availabilityOrder {
		if slices.Contains(values, availability) {
			labels = append(labels, availabilityLabels[availability])
		}
	}
	return strings.Join(labels, ", ")
}

// appendOnce adds a value to a set held as a slice, which keeps the insertion
// order a map would lose. The sets here hold at most four values.
func appendOnce(values []string, value string) []string {
	if slices.Contains(values, value) {
		return values
	}
	return append(values, value)
}

// writeCommandGroup writes one open <details> holding one table of commands.
func writeCommandGroup(out *strings.Builder, group *commandGroup) {
	out.WriteString(`<details class="cli-group" id="cli-group-` + commandSlug(group.Label) + `" open>` + "\n")
	out.WriteString("<summary>" + html.EscapeString(group.Label) +
		` <span class="cli-group-count">` + strconv.Itoa(len(group.Commands)) + "</span></summary>\n")
	out.WriteString("<table><thead><tr><th>Command</th><th>Mode</th><th>Description</th>" +
		"<th>Pipes</th></tr></thead><tbody>\n")
	for _, command := range group.Commands {
		writeCommandRow(out, command)
	}
	out.WriteString("</tbody></table></details>\n")
}

// writeCommandRow writes one command as one table row.
//
// The first cell carries the registry PATH and nothing else. The row's id is
// derived from that same path, so the anchor a reader links and the text they
// read are one value; the invocation form goes in the description cell, where
// the command model's own usage line belongs.
func writeCommandRow(out *strings.Builder, command *catalogCommand) {
	out.WriteString(`<tr id="cmd-` + commandSlug(command.Path) + `"><td><code>` +
		html.EscapeString(command.Path) + "</code></td>")
	out.WriteString(`<td><span class="cli-mode cli-mode-` + html.EscapeString(command.Mode) + `">` +
		html.EscapeString(commandModeLabel(command.Mode)) + "</span></td><td>")
	out.WriteString(strings.ReplaceAll(html.EscapeString(command.Description), "\n", "<br>"))
	if command.Usage != "" {
		out.WriteString("<br><code>" + html.EscapeString(command.Usage) + "</code>")
	}
	out.WriteString("</td><td>")
	writeCommandPipeCell(out, command)
	out.WriteString("</td></tr>\n")
}

// writeCommandPipeCell writes the Pipes cell: a summary a reader can scan
// closed, and the detail behind it.
func writeCommandPipeCell(out *strings.Builder, command *catalogCommand) {
	grouped := operatorsByAvailability(command)
	if len(grouped) == 0 && len(command.Pipes) == 0 && len(command.Aliases) == 0 &&
		command.AnswerShape == "" && len(command.AddressFields) == 0 {
		out.WriteString(`<span class="cli-pipe-none">None</span>`)
		return
	}
	out.WriteString(`<details class="cli-pipes"><summary>` +
		html.EscapeString(commandPipeSummary(command)) + `</summary><div class="cli-pipe-detail">`)
	if command.AnswerShape != "" {
		out.WriteString("<p><span>Answer shape</span><code>" +
			html.EscapeString(command.AnswerShape) + "</code></p>")
	}
	if len(command.AddressFields) != 0 {
		out.WriteString("<p><span>Address fields</span><code>" +
			html.EscapeString(strings.Join(command.AddressFields, " · ")) + "</code></p>")
	}
	if len(command.Pipes) != 0 {
		out.WriteString(`<strong>Command pipes</strong><div class="cli-pipe-chips">`)
		for _, pipe := range command.Pipes {
			out.WriteString(`<code title="` + html.EscapeString(pipe.Description) + `">` +
				html.EscapeString(pipeDisplayName(pipe)) + "</code>")
		}
		out.WriteString("</div>")
		out.WriteString(`<details class="cli-pipe-descriptions"><summary>Command pipe descriptions</summary><dl>`)
		for _, pipe := range command.Pipes {
			out.WriteString("<dt><code>" + html.EscapeString(pipeDisplayName(pipe)) + "</code></dt><dd>" +
				html.EscapeString(pipe.Description) + "</dd>")
		}
		out.WriteString("</dl></details>")
	}
	if len(command.Aliases) != 0 {
		out.WriteString("<strong>Aliases</strong><dl>")
		for _, alias := range command.Aliases {
			out.WriteString("<dt><code>" + html.EscapeString(alias.Name) + "</code></dt><dd>" +
				html.EscapeString(alias.Description) + " <code>" +
				html.EscapeString(alias.Expansion) + "</code></dd>")
		}
		out.WriteString("</dl>")
	}
	for _, availability := range availabilityOrder {
		names := grouped[availability]
		if len(names) == 0 {
			continue
		}
		out.WriteString("<p><span>" + html.EscapeString(availabilityLabels[availability]) + "</span><code>" +
			html.EscapeString(strings.Join(names, " · ")) + "</code></p>")
	}
	out.WriteString("</div></details>")
}

// commandPipeSummary is the one line a reader sees with the pipe detail closed.
func commandPipeSummary(command *catalogCommand) string {
	var parts []string
	if count := len(command.Pipes); count != 0 {
		parts = append(parts, plural(count, "command pipe"))
	}
	if count := len(command.Aliases); count != 0 {
		// "alias" is the one irregular plural on this page, so it is spelled
		// out rather than made to fit the regular rule plural states.
		word := " aliases"
		if count == 1 {
			word = " alias"
		}
		parts = append(parts, strconv.Itoa(count)+word)
	}
	if count := len(command.Operators); count != 0 {
		parts = append(parts, plural(count, "operator"))
	}
	if command.AnswerShape != "" {
		parts = append(parts, "answer: "+command.AnswerShape)
	}
	if count := len(command.AddressFields); count != 0 {
		parts = append(parts, plural(count, "address field"))
	}
	if len(parts) == 0 {
		return "None"
	}
	return strings.Join(parts, " · ")
}

// cliReferenceMirror renders the page's index.md sibling.
//
// It is written from the same groups the HTML uses, so the two cannot disagree
// about how the commands are organized.
func cliReferenceMirror(commands []catalogCommand, groups []commandGroup) string {
	var out strings.Builder
	out.WriteString("# CLI Reference\n\n")
	out.WriteString(strconv.Itoa(len(commands)) + " commands across " + strconv.Itoa(len(groups)) +
		" groups, generated straight from `ze help command --json` -- the same live command registry " +
		"the binary itself uses, so this list cannot drift from what the binary actually supports. " +
		"Full machine-readable list (path, mode, description, pipe operators, command pipes, and " +
		"aliases for every command): [" + catalogFile + "](" + siteBase + catalogFile + ").\n\n")
	for index := range groups {
		group := &groups[index]
		out.WriteString("## " + group.Label + " (" + strconv.Itoa(len(group.Commands)) + ")\n\n")
		out.WriteString("| Command | Mode | Description | Pipes |\n| --- | --- | --- | --- |\n")
		for _, command := range group.Commands {
			out.WriteString("| `" + markdownCell(command.Path) + "` | " +
				markdownCell(commandModeLabel(command.Mode)) + " | " +
				commandMirrorDescription(command) + " | " +
				commandMirrorPipes(command) + " |\n")
		}
		out.WriteString("\n")
	}
	return strings.TrimRight(out.String(), "\n") + "\n"
}

// commandMirrorDescription writes one command's description, and its usage line
// after it, as one table cell.
func commandMirrorDescription(command *catalogCommand) string {
	description := markdownCell(command.Description)
	if command.Usage == "" {
		return description
	}
	usage := "`" + markdownCell(command.Usage) + "`"
	if description == "" {
		return usage
	}
	return description + "<br>" + usage
}

// commandMirrorPipes writes one command's pipe contract as one table cell.
func commandMirrorPipes(command *catalogCommand) string {
	var parts []string
	if command.AnswerShape != "" {
		parts = append(parts, "Answer shape: "+markdownCodeList([]string{command.AnswerShape}))
	}
	if len(command.AddressFields) != 0 {
		parts = append(parts, "Address fields: "+markdownCodeList(command.AddressFields))
	}
	if len(command.Pipes) != 0 {
		names := make([]string, 0, len(command.Pipes))
		for _, pipe := range command.Pipes {
			names = append(names, pipeDisplayName(pipe))
		}
		parts = append(parts, "Command: "+markdownCodeList(names))
	}
	if len(command.Aliases) != 0 {
		expansions := make([]string, 0, len(command.Aliases))
		for _, alias := range command.Aliases {
			expansions = append(expansions, alias.Name+" -> "+alias.Expansion)
		}
		parts = append(parts, "Aliases: "+markdownCodeList(expansions))
	}
	grouped := operatorsByAvailability(command)
	for _, availability := range availabilityOrder {
		if names := grouped[availability]; len(names) != 0 {
			parts = append(parts, availabilityLabels[availability]+": "+markdownCodeList(names))
		}
	}
	if len(parts) == 0 {
		return "None"
	}
	return strings.Join(parts, "<br>")
}

// markdownCell folds one value onto a single line and escapes the pipe that
// would otherwise split the table cell it sits in.
func markdownCell(value string) string {
	return strings.ReplaceAll(strings.Join(strings.Fields(value), " "), "|", `\|`)
}

// markdownCodeList writes several values as a comma-separated list of code
// spans, each safe inside a table cell.
func markdownCodeList(values []string) string {
	spans := make([]string, 0, len(values))
	for _, value := range values {
		spans = append(spans, "`"+markdownCell(value)+"`")
	}
	return strings.Join(spans, ", ")
}
