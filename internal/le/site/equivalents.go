// Design: website/AI.md -- one page per live command, mapping it onto the vendor CLIs
// Detail: catalog.go reads the live commands; website/data/command-equivalents.json the curation.
// Related: commands.go publishes the same commands as one reference table.
package site

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// The command-equivalent pages are registered from here, so a build discovers
// them through the registry rather than through a call the build states by name.
func init() {
	registerProducer(Producer{Name: "command-equivalents", Render: renderCommandEquivalents})
}

// The published index of the vendor map, the directory its detail pages sit in,
// and the relative path back to the site root from each of the two depths.
const (
	equivalentsDirectory  = "reference/command-equivalents"
	equivalentsIndexDest  = equivalentsDirectory + "/" + pageIndexFile
	equivalentsIndexRoot  = "../../"
	equivalentsDetailRoot = "../../../"
	equivalentsFile       = "data/command-equivalents.json"
)

// equivalentMapping is the curated vendor map, as website/data holds it.
type equivalentMapping struct {
	SchemaVersion int                         `json:"schema-version"`
	Updated       string                      `json:"updated"`
	Summary       string                      `json:"summary"`
	Vendors       map[string]equivalentVendor `json:"vendors"`
	Entries       []equivalentEntry           `json:"entries"`
	// order holds the vendor ids as the curation WROTE them, which decides the
	// column order of every table and the card order of every detail page. A Go
	// map states no order, so it is read back from the raw document rather than
	// recovered by sorting: the authored order groups the vendors an operator
	// is most likely migrating from first, and sorting would lose that.
	order []string
}

// equivalentVendor is one vendor column: how it is named, and what it roots its
// configuration on, which is the fact that decides how a migration reads.
type equivalentVendor struct {
	Label         string                `json:"label"`
	ShortLabel    string                `json:"short-label"`
	RootingModel  string                `json:"rooting-model"`
	Documentation []equivalentSourceDoc `json:"documentation"`
}

// equivalentSourceDoc is one vendor document a curated command cites.
type equivalentSourceDoc struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	URL   string `json:"url"`
}

// equivalentEntry is one curated migration intent: what an operator wants to
// do, the Ze commands that do it, and each vendor's spelling of the same thing.
type equivalentEntry struct {
	ID       string                         `json:"id"`
	Category string                         `json:"category"`
	Intent   string                         `json:"intent"`
	Notes    string                         `json:"notes"`
	Ze       []string                       `json:"ze"`
	Vendors  map[string][]equivalentCommand `json:"vendors"`
}

// equivalentCommand is one vendor command line, with the evidence behind it.
type equivalentCommand struct {
	Command    string   `json:"command"`
	Mode       string   `json:"mode"`
	Confidence string   `json:"confidence"`
	Notes      string   `json:"notes"`
	SourceRefs []string `json:"source-refs"`
}

// The confidence levels a curated vendor command can state, best evidence
// first. A level nothing names is refused rather than published with a blank
// badge, because the badge IS the evidence a reader judges the line by.
const (
	confidenceVerified = "verified"
	confidenceSeed     = "local-seed"
	confidenceLegacy   = "legacy"
	confidenceUnknown  = "unknown"
)

// confidenceOrder ranks the levels. A vendor cell lists them in this order, so
// a reader who takes the first line takes the best-evidenced one.
var confidenceOrder = map[string]int{
	confidenceVerified: 0, confidenceSeed: 1, confidenceLegacy: 2, confidenceUnknown: 3,
}

// confidenceLabel answers the badge word one level carries. Every level is
// published under its own name; local-seed is the one whose published word
// differs, because "seed" is what the badge has room for.
func confidenceLabel(confidence string) string {
	if confidence == confidenceSeed {
		return "seed"
	}
	return confidence
}

// equivalentRow joins one live command with every curated intent that names it.
type equivalentRow struct {
	Command *catalogCommand
	Slug    string
	Group   string
	Entries []*equivalentEntry
}

// renderCommandEquivalents publishes the vendor map: one index, one detail page
// for each live command, and a Markdown mirror beside each of them.
func renderCommandEquivalents(paths Paths) ([]string, error) {
	commands, err := loadCommandCatalog(paths.Output)
	if err != nil {
		return nil, err
	}
	mapping, err := loadEquivalentMapping(paths.Output)
	if err != nil {
		return nil, err
	}
	links, err := loadPageLinks(paths.Source)
	if err != nil {
		return nil, err
	}
	if err := mapping.validate(commands); err != nil {
		return nil, err
	}
	vendors := mapping.vendorIDs()
	rows, vendorOnly := mapping.rows(commands)

	routes := make([]string, 0, len(rows)+1)
	route, err := renderEquivalentIndex(paths, mapping, links, rows, vendorOnly, vendors)
	if err != nil {
		return nil, err
	}
	routes = append(routes, route)
	for index := range rows {
		route, err := renderEquivalentDetail(paths, mapping, links, &rows[index], vendors)
		if err != nil {
			return nil, err
		}
		routes = append(routes, route)
	}
	if err := removeRetiredDetailPages(paths.Output, rows); err != nil {
		return nil, err
	}
	return routes, nil
}

// loadEquivalentMapping reads the curated vendor map this build published.
func loadEquivalentMapping(output string) (*equivalentMapping, error) {
	path := filepath.Join(output, filepath.FromSlash(equivalentsFile))
	content, err := os.ReadFile(path) //nolint:gosec // a site build reads the artifact it was pointed at
	if err != nil {
		return nil, fmt.Errorf("read the vendor command map %s: %w", path, err)
	}
	mapping := &equivalentMapping{}
	if err := json.Unmarshal(content, mapping); err != nil {
		return nil, fmt.Errorf("read the vendor command map %s: %w", path, err)
	}
	mapping.order, err = jsonFieldKeys(content, "vendors")
	if err != nil {
		return nil, fmt.Errorf("read the vendor command map %s: %w", path, err)
	}
	return mapping, nil
}

// jsonFieldKeys answers the keys of one top-level object field, in the order the
// document writes them.
func jsonFieldKeys(document []byte, field string) ([]string, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(document, &fields); err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(fields[field]))
	if _, err := decoder.Token(); err != nil {
		return nil, err
	}
	var keys []string
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		name, isName := key.(string)
		if !isName {
			return nil, fmt.Errorf("%s holds a key that is not a name", field)
		}
		keys = append(keys, name)
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
	}
	return keys, nil
}

// validate refuses a mapping the pages cannot render honestly.
//
// The two checks that matter are the ones a reader cannot make: a Ze path the
// binary no longer has, which would publish a migration hint for a command
// nobody can run, and a confidence level nothing names, which would publish an
// unlabelled badge where the evidence belongs.
func (mapping *equivalentMapping) validate(commands []catalogCommand) error {
	if mapping.SchemaVersion != 1 {
		return fmt.Errorf("the vendor command map states schema-version %d, want 1", mapping.SchemaVersion)
	}
	if len(mapping.Vendors) == 0 {
		return fmt.Errorf("the vendor command map names no vendor")
	}
	live := make(map[string]bool, len(commands))
	for index := range commands {
		live[commands[index].Path] = true
	}
	for index := range mapping.Entries {
		entry := &mapping.Entries[index]
		if entry.ID == "" {
			return fmt.Errorf("the vendor command map holds an entry with no id")
		}
		for _, path := range entry.Ze {
			if !live[path] {
				return fmt.Errorf("vendor map entry %s names %q, which the live command catalog does not have",
					entry.ID, path)
			}
		}
		for vendor, rows := range entry.Vendors {
			if _, known := mapping.Vendors[vendor]; !known {
				return fmt.Errorf("vendor map entry %s names the unknown vendor %s", entry.ID, vendor)
			}
			for _, row := range rows {
				if _, known := confidenceOrder[row.Confidence]; !known {
					return fmt.Errorf("vendor map entry %s: %s command %q states the unknown confidence %q",
						entry.ID, vendor, row.Command, row.Confidence)
				}
			}
		}
	}
	return nil
}

// vendorIDs answers the vendor columns in the order the curation wrote them.
func (mapping *equivalentMapping) vendorIDs() []string {
	return mapping.order
}

// vendorLabel answers the short name a column header carries.
func (mapping *equivalentMapping) vendorLabel(id string) string {
	vendor := mapping.Vendors[id]
	if vendor.ShortLabel != "" {
		return vendor.ShortLabel
	}
	return vendor.Label
}

// sources answers every document a curated command can cite, keyed by its id.
func (mapping *equivalentMapping) sources() map[string]equivalentSourceDoc {
	sources := make(map[string]equivalentSourceDoc)
	for _, vendor := range mapping.Vendors {
		for _, document := range vendor.Documentation {
			sources[document.ID] = document
		}
	}
	return sources
}

// rows joins the live catalog with the curation: one row for each live command,
// and the entries that name no Ze command answered separately as vendor-only
// gaps, because those are the migrations Ze has no answer for yet.
func (mapping *equivalentMapping) rows(commands []catalogCommand) ([]equivalentRow, []*equivalentEntry) {
	byPath := make(map[string][]*equivalentEntry, len(commands))
	var vendorOnly []*equivalentEntry
	for index := range mapping.Entries {
		entry := &mapping.Entries[index]
		if len(entry.Ze) == 0 {
			vendorOnly = append(vendorOnly, entry)
			continue
		}
		for _, path := range entry.Ze {
			byPath[path] = append(byPath[path], entry)
		}
	}
	rows := make([]equivalentRow, 0, len(commands))
	for index := range commands {
		command := &commands[index]
		verb, _, _ := strings.Cut(command.Path, " ")
		rows = append(rows, equivalentRow{
			Command: command,
			Slug:    commandSlug(command.Path),
			Group:   verb,
			Entries: byPath[command.Path],
		})
	}
	return rows, vendorOnly
}

// vendorEquivalent is one vendor command and the migration intent it answers.
// The two travel together because a mirror states both, and a command with no
// intent beside it says nothing about WHY it is the equivalent.
type vendorEquivalent struct {
	Intent  string
	Command equivalentCommand
}

// vendorCommands answers one row's commands for one vendor, best evidence
// first, with a command two intents both name listed once.
func (row *equivalentRow) vendorCommands(vendor string) []vendorEquivalent {
	var commands []vendorEquivalent
	seen := make(map[string]bool)
	for _, entry := range row.Entries {
		for _, item := range entry.Vendors[vendor] {
			key := item.Command + "\x00" + item.Confidence
			if seen[key] {
				continue
			}
			seen[key] = true
			commands = append(commands, vendorEquivalent{Intent: entry.Intent, Command: item})
		}
	}
	sortEquivalentsByConfidence(commands)
	return commands
}

// sortEquivalentsByConfidence orders vendor commands best evidence first, then
// by the command itself, so a reader who takes the first line takes the safest.
func sortEquivalentsByConfidence(commands []vendorEquivalent) {
	sort.SliceStable(commands, func(left, right int) bool {
		leftRank := confidenceOrder[commands[left].Command.Confidence]
		rightRank := confidenceOrder[commands[right].Command.Confidence]
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		return commands[left].Command.Command < commands[right].Command.Command
	})
}

// hasVendorCommands reports whether any vendor column of this row is filled.
func (row *equivalentRow) hasVendorCommands(vendors []string) bool {
	for _, vendor := range vendors {
		if len(row.vendorCommands(vendor)) != 0 {
			return true
		}
	}
	return false
}

// removeRetiredDetailPages deletes the directory of a command the binary no
// longer has.
//
// Without this a renamed command leaves its old page published forever: the
// producer stops writing it, nothing else claims it, and the seed carries it
// into every later build. That is the failure this whole spec exists to remove,
// so a producer removes what it stops owning.
func removeRetiredDetailPages(output string, rows []equivalentRow) error {
	live := make(map[string]bool, len(rows))
	for index := range rows {
		live[rows[index].Slug] = true
	}
	root := filepath.Join(output, filepath.FromSlash(equivalentsDirectory))
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() || live[entry.Name()] {
			continue
		}
		if err := os.RemoveAll(filepath.Join(root, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

// renderEquivalentIndex publishes the searchable map of every live command.
func renderEquivalentIndex(
	paths Paths, mapping *equivalentMapping, links pageLinks,
	rows []equivalentRow, vendorOnly []*equivalentEntry, vendors []string,
) (string, error) {
	shell := pageShell{
		Title: "Command Equivalents - Ze",
		Description: "One-line Ze command map with side-by-side " +
			strings.Join(mapping.vendorLabels(vendors), ", ") + " equivalents.",
		Root:    equivalentsIndexRoot,
		Path:    equivalentsIndexDest,
		Sidebar: pageSidebar(equivalentsIndexRoot, equivalentsIndexDest, links),
	}
	page := shell.render(equivalentIndexBody(mapping, rows, vendorOnly, vendors))
	mirror := equivalentIndexMirror(mapping, rows, vendorOnly, vendors)
	if err := writePublishedPage(paths.Output, equivalentsIndexDest, page, mirror); err != nil {
		return "", err
	}
	return "/" + strings.TrimSuffix(equivalentsIndexDest, pageIndexFile), nil
}

// vendorLabels answers the short names of several vendors, in the given order.
func (mapping *equivalentMapping) vendorLabels(vendors []string) []string {
	labels := make([]string, 0, len(vendors))
	for _, vendor := range vendors {
		labels = append(labels, mapping.vendorLabel(vendor))
	}
	return labels
}

// equivalentIndexBody renders the index between <main> and </main>.
func equivalentIndexBody(
	mapping *equivalentMapping, rows []equivalentRow,
	vendorOnly []*equivalentEntry, vendors []string,
) string {
	reviewed, equivalent := 0, 0
	for index := range rows {
		if len(rows[index].Entries) != 0 {
			reviewed++
		}
		if rows[index].hasVendorCommands(vendors) {
			equivalent++
		}
	}
	var body strings.Builder
	body.WriteString("\n" + `<section aria-labelledby="command-equivalents-title" ` +
		`class="md-content command-equivalents reveal cat-operate">` + "\n")
	body.WriteString(pageHero("Command Equivalents",
		"One line per live Ze command, with vendor CLI surfaced first when a curated equivalent "+
			"exists. The full catalog still includes reviewed gaps; a dash means no equivalent has "+
			"been listed yet.",
		"Reference", ` id="command-equivalents-title"`, heroClasses))
	body.WriteString("\n" + `<div class="cmd-eq-stats">` + "\n")
	writeStat(&body, len(rows), "live Ze commands")
	writeStat(&body, equivalent, "commands with vendor CLI")
	writeStat(&body, reviewed, "reviewed command rows")
	writeStat(&body, len(mapping.Entries), "curated mapping intents")
	writeStat(&body, len(vendorOnly), "vendor-only gap notes")
	body.WriteString("</div>\n")
	body.WriteString(equivalentCoverage(mapping, rows, vendors, reviewed, equivalent))
	body.WriteString(equivalentVendorSelector(mapping, vendors))
	body.WriteString(equivalentSearchShelf(mapping, vendors))
	body.WriteString(equivalentSpotlight(mapping, rows, vendors))
	body.WriteString(`<section class="cmd-eq-full-catalog" aria-labelledby="cmd-eq-full-catalog-title">` + "\n")
	body.WriteString(`<h2 id="cmd-eq-full-catalog-title">Full live command catalog</h2>` + "\n")
	body.WriteString("<noscript><p>JavaScript is disabled. Browser find works across the " +
		"side-by-side command table.</p></noscript>\n")
	for _, group := range groupEquivalentRows(rows) {
		body.WriteString(equivalentIndexGroup(mapping, group, vendors))
	}
	body.WriteString(equivalentVendorOnly(mapping, vendorOnly, vendors))
	body.WriteString("</section>\n")
	body.WriteString(equivalentSources(mapping))
	body.WriteString("</section>\n")
	return body.String()
}

// writeStat writes one headline number of the index.
func writeStat(out *strings.Builder, count int, label string) {
	out.WriteString("<div><strong>" + strconv.Itoa(count) + "</strong><span>" +
		html.EscapeString(label) + "</span></div>\n")
}

// equivalentGroup is one verb's worth of rows in the full catalog.
type equivalentGroup struct {
	Label string
	Rows  []*equivalentRow
}

// groupEquivalentRows splits the rows by their command's verb, in verb order.
func groupEquivalentRows(rows []equivalentRow) []equivalentGroup {
	byVerb := make(map[string][]*equivalentRow, len(rows))
	var verbs []string
	for index := range rows {
		row := &rows[index]
		if _, seen := byVerb[row.Group]; !seen {
			verbs = append(verbs, row.Group)
		}
		byVerb[row.Group] = append(byVerb[row.Group], row)
	}
	sort.Strings(verbs)
	groups := make([]equivalentGroup, 0, len(verbs))
	for _, verb := range verbs {
		groups = append(groups, equivalentGroup{Label: verb, Rows: byVerb[verb]})
	}
	return groups
}

// equivalentCoverage renders the per-vendor coverage panel.
func equivalentCoverage(
	mapping *equivalentMapping, rows []equivalentRow, vendors []string, reviewed, equivalent int,
) string {
	var panel strings.Builder
	panel.WriteString(`<section class="cmd-eq-overview" aria-labelledby="cmd-eq-overview-title">` + "\n")
	panel.WriteString(`<div class="cmd-eq-overview-copy">` + "\n")
	panel.WriteString(`<h2 id="cmd-eq-overview-title">Where equivalents exist</h2>` + "\n")
	panel.WriteString("<p>" + strconv.Itoa(equivalent) + " Ze commands have at least one vendor CLI " +
		"equivalent today. " + strconv.Itoa(reviewed) + " commands have been reviewed for migration " +
		"intent, including gaps where a direct vendor command is not listed.</p>\n")
	panel.WriteString("<p>The rows with actual vendor CLI are pulled forward below so the useful " +
		"equivalents are visible before the complete generated catalog.</p>\n")
	panel.WriteString("</div>\n")
	panel.WriteString(`<div class="cmd-eq-coverage-grid" aria-label="Vendor equivalent coverage">` + "\n")
	total := 0
	for _, vendor := range vendors {
		mapped, lines := 0, 0
		for index := range rows {
			count := len(rows[index].vendorCommands(vendor))
			lines += count
			if count != 0 {
				mapped++
			}
		}
		total += lines
		panel.WriteString("<article><span>" + html.EscapeString(mapping.vendorLabel(vendor)) +
			"</span><strong>" + strconv.Itoa(mapped) + "</strong><small>Ze " +
			plural(mapped, "command") + ", " + plural(lines, "command line") + "</small></article>\n")
	}
	panel.WriteString(`<article class="cmd-eq-coverage-total"><span>Total</span><strong>` +
		strconv.Itoa(total) + "</strong><small>vendor command lines</small></article>\n")
	panel.WriteString("</div>\n</section>\n")
	return panel.String()
}

// equivalentVendorSelector renders the control that hides and shows columns.
func equivalentVendorSelector(mapping *equivalentMapping, vendors []string) string {
	labels := mapping.vendorLabels(vendors)
	preferred := labels[0]
	for _, vendor := range vendors {
		if vendor == "vyos" {
			preferred = mapping.vendorLabel(vendor)
		}
	}
	return `<div class="column-selector cmd-eq-column-selector" data-column-selector ` +
		`data-column-selector-target=".command-equivalents .cmd-eq-table" ` +
		`data-column-selector-columns="` + html.EscapeString(strings.Join(labels, ",")) + `" ` +
		`data-column-selector-default="` + html.EscapeString(preferred) + `" ` +
		`data-column-selector-mode="buttons" data-column-selector-actions="true" ` +
		`data-column-selector-label="Compare vendors" data-column-selector-kind="vendors">` +
		`<p class="column-selector-status" data-column-selector-status aria-live="polite"></p>` +
		"</div>\n"
}

// equivalentSearchShelf renders the search box above the tables.
func equivalentSearchShelf(mapping *equivalentMapping, vendors []string) string {
	var shelf strings.Builder
	shelf.WriteString(`<section class="cmd-eq-search-shelf" aria-labelledby="cmd-eq-search-title">` + "\n")
	shelf.WriteString(`<h2 id="cmd-eq-search-title">Search the command map</h2>` + "\n")
	shelf.WriteString(`<p class="cmd-eq-panel-note">Search every generated Ze command, reviewed ` +
		`mapping note, and listed vendor command. Rows without vendor CLI remain visible in the ` +
		"full catalog so missing coverage is explicit.</p>\n")
	shelf.WriteString(`<div class="cmd-eq-search-wrap">` + "\n")
	shelf.WriteString(`<input id="cmd-eq-search" type="search" autocomplete="off" placeholder="Search Ze, ` +
		html.EscapeString(strings.Join(mapping.vendorLabels(vendors), ", ")) +
		` commands..." aria-label="Search command equivalents" />` + "\n")
	shelf.WriteString(`<div id="cmd-eq-search-count" class="cmd-eq-search-count" aria-live="polite"></div>` + "\n")
	shelf.WriteString("</div>\n</section>\n")
	return shelf.String()
}

// equivalentSpotlight renders the table of rows that have a vendor command.
//
// Its rows carry no id, because the full catalog below states the canonical one
// for each command and two elements cannot share an id.
func equivalentSpotlight(mapping *equivalentMapping, rows []equivalentRow, vendors []string) string {
	mapped := make([]*equivalentRow, 0, len(rows))
	for index := range rows {
		if rows[index].hasVendorCommands(vendors) {
			mapped = append(mapped, &rows[index])
		}
	}
	if len(mapped) == 0 {
		return ""
	}
	var panel strings.Builder
	panel.WriteString(`<details class="cmd-eq-panel cmd-eq-mapped-first" open>` + "\n")
	panel.WriteString(`<summary>Commands with vendor CLI <span class="cmd-eq-count">` +
		strconv.Itoa(len(mapped)) + "</span></summary>\n")
	panel.WriteString(`<p class="cmd-eq-panel-note">This table contains only rows where at least one ` +
		"vendor column has a curated command. Use the full catalog below to inspect reviewed gaps " +
		"and every live Ze command.</p>\n")
	panel.WriteString(equivalentTable(mapping, mapped, vendors, false))
	panel.WriteString("</details>\n")
	return panel.String()
}

// equivalentIndexGroup renders one verb's table inside the full catalog.
func equivalentIndexGroup(mapping *equivalentMapping, group equivalentGroup, vendors []string) string {
	equivalent, reviewed := 0, 0
	for _, row := range group.Rows {
		if row.hasVendorCommands(vendors) {
			equivalent++
		}
		if len(row.Entries) != 0 {
			reviewed++
		}
	}
	var counts []string
	if equivalent != 0 {
		counts = append(counts, strconv.Itoa(equivalent)+" equivalent")
	}
	if reviewed != 0 && reviewed != equivalent {
		counts = append(counts, strconv.Itoa(reviewed)+" reviewed")
	}
	counts = append(counts, strconv.Itoa(len(group.Rows))+" total")

	var out strings.Builder
	out.WriteString(`<details class="cmd-eq-group" id="cmd-eq-group-` + commandSlug(group.Label) + `" open>` + "\n")
	out.WriteString("<summary>" + html.EscapeString(group.Label) + ` <span class="cmd-eq-count">` +
		html.EscapeString(strings.Join(counts, ", ")) + "</span></summary>\n")
	out.WriteString(equivalentTable(mapping, group.Rows, vendors, true))
	out.WriteString("</details>\n")
	return out.String()
}

// equivalentTable renders one side-by-side table. Only the full catalog gives
// its rows an id, so each command has exactly one anchor on the page.
func equivalentTable(
	mapping *equivalentMapping, rows []*equivalentRow, vendors []string, anchored bool,
) string {
	var out strings.Builder
	out.WriteString(`<div class="cmd-eq-table-wrap">` + "\n")
	out.WriteString(`<table class="cmd-eq-table cmd-eq-compact"><thead><tr><th>Ze</th>`)
	for _, vendor := range vendors {
		out.WriteString("<th>" + html.EscapeString(mapping.vendorLabel(vendor)) + "</th>")
	}
	out.WriteString("<th>Details</th></tr></thead><tbody>\n")
	for _, row := range rows {
		writeEquivalentIndexRow(&out, row, vendors, anchored)
	}
	out.WriteString("</tbody></table></div>\n")
	return out.String()
}

// writeEquivalentIndexRow writes one command as one row of a side-by-side table.
//
// The Ze cell carries exactly one <code>, holding the registry path, so the row
// a reader sees and the anchor it answers to are one value.
func writeEquivalentIndexRow(out *strings.Builder, row *equivalentRow, vendors []string, anchored bool) {
	out.WriteString("<tr")
	if anchored {
		out.WriteString(` id="cmd-eq-` + row.Slug + `"`)
	}
	class := "cmd-eq-no-vendor"
	if row.hasVendorCommands(vendors) {
		class = "cmd-eq-has-vendor"
	}
	out.WriteString(` class="` + class + `" data-search="` + html.EscapeString(row.searchText(vendors)) + `">`)
	out.WriteString(`<td class="cmd-eq-ze"><a href="` + row.Slug + `/"><code>` +
		html.EscapeString(row.Command.Path) + `</code></a><span class="cmd-mode">` +
		html.EscapeString(commandModeLabel(row.Command.Mode)) + "</span></td>")
	for _, vendor := range vendors {
		out.WriteString("<td>")
		commands := row.vendorCommands(vendor)
		if len(commands) == 0 {
			out.WriteString(`<span class="cmd-no-equivalent">-</span>`)
		}
		for _, item := range commands {
			out.WriteString(`<div class="cmd-compact-command"><code>` +
				html.EscapeString(item.Command.Command) + "</code></div>")
		}
		out.WriteString("</td>")
	}
	out.WriteString(`<td class="cmd-eq-detail-link"><a href="` + row.Slug + `/">details</a></td></tr>` + "\n")
}

// searchText answers the lowercase haystack the page's own filter searches.
func (row *equivalentRow) searchText(vendors []string) string {
	// Both halves of the command's help are indexed. A reader searching this
	// page wants the most text, and the long form is where a command names the
	// thing an operator remembers it by.
	terms := []string{
		row.Command.Path, row.Command.Usage, row.Command.Description,
		row.Command.LongHelp, row.Command.Mode, row.Command.WireMethod, row.Group,
	}
	for _, entry := range row.Entries {
		terms = append(terms, entry.Intent, entry.Category, entry.Notes)
	}
	for _, vendor := range vendors {
		for _, item := range row.vendorCommands(vendor) {
			terms = append(terms, vendor, item.Command.Command, item.Command.Confidence)
		}
	}
	return strings.ToLower(strings.Join(strings.Fields(strings.Join(terms, " ")), " "))
}

// equivalentVendorOnly renders the intents no Ze command answers yet.
func equivalentVendorOnly(mapping *equivalentMapping, entries []*equivalentEntry, vendors []string) string {
	if len(entries) == 0 {
		return ""
	}
	var out strings.Builder
	out.WriteString(`<details class="cmd-eq-group cmd-eq-vendor-only" id="cmd-eq-vendor-only" open>` + "\n")
	out.WriteString("<summary>Vendor-only gaps</summary>\n")
	out.WriteString(`<div class="cmd-eq-table-wrap">` + "\n")
	out.WriteString(`<table class="cmd-eq-table cmd-eq-compact"><thead><tr><th>Intent</th>`)
	for _, vendor := range vendors {
		out.WriteString("<th>" + html.EscapeString(mapping.vendorLabel(vendor)) + "</th>")
	}
	out.WriteString("<th>Notes</th></tr></thead><tbody>\n")
	for _, entry := range entries {
		out.WriteString("<tr><td><strong>" + html.EscapeString(entry.Intent) +
			`</strong><span class="cmd-mode">` + html.EscapeString(entry.Category) + "</span></td>")
		for _, vendor := range vendors {
			out.WriteString("<td>")
			rows := entry.Vendors[vendor]
			if len(rows) == 0 {
				out.WriteString(`<span class="cmd-no-equivalent">-</span>`)
			}
			for _, item := range rows {
				out.WriteString(`<div class="cmd-compact-command"><code>` +
					html.EscapeString(item.Command) + "</code></div>")
			}
			out.WriteString("</td>")
		}
		out.WriteString("<td>" + html.EscapeString(entry.Notes) + "</td></tr>\n")
	}
	out.WriteString("</tbody></table></div></details>\n")
	return out.String()
}

// equivalentSources renders the vendor documents the curation cites.
func equivalentSources(mapping *equivalentMapping) string {
	var out strings.Builder
	out.WriteString(`<details class="cmd-eq-sources"><summary>Data sources and confidence</summary>` + "\n")
	out.WriteString("<p>Source links and confidence labels are shown on each command detail page. " +
		"The index only shows side-by-side command text.</p>\n")
	out.WriteString("<h2>Vendor documents</h2>\n")
	for _, id := range mapping.vendorIDs() {
		vendor := mapping.Vendors[id]
		out.WriteString("<h3>" + html.EscapeString(vendor.Label) + "</h3><ul>\n")
		for _, document := range vendor.Documentation {
			out.WriteString(`<li><a href="` + html.EscapeString(document.URL) +
				`" target="_blank" rel="noopener">` + html.EscapeString(document.Label) +
				"</a> <code>" + html.EscapeString(document.ID) + "</code></li>\n")
		}
		out.WriteString("</ul>\n")
	}
	out.WriteString("</details>\n")
	return out.String()
}

// equivalentIndexMirror renders the index page's index.md sibling.
func equivalentIndexMirror(
	mapping *equivalentMapping, rows []equivalentRow,
	vendorOnly []*equivalentEntry, vendors []string,
) string {
	reviewed, equivalent := 0, 0
	mapped := make([]*equivalentRow, 0, len(rows))
	for index := range rows {
		if len(rows[index].Entries) != 0 {
			reviewed++
		}
		if rows[index].hasVendorCommands(vendors) {
			equivalent++
			mapped = append(mapped, &rows[index])
		}
	}
	var out strings.Builder
	out.WriteString("# Command Equivalents\n\n")
	out.WriteString(strconv.Itoa(len(rows)) + " live Ze commands. " + strconv.Itoa(equivalent) +
		" have vendor CLI today. " + strconv.Itoa(reviewed) + " have been reviewed for migration " +
		"intent. Vendor commands are curated migration hints, not exhaustive vendor CLI catalogs.\n\n")
	out.WriteString("## Commands with vendor CLI\n\nThese rows have at least one listed vendor command.\n\n")
	writeEquivalentMirrorTable(&out, mapping, mapped, vendors)
	out.WriteString("\n## Full live command catalog\n\n")
	out.WriteString("Rows without vendor CLI remain visible so missing coverage is explicit.\n\n")
	for _, group := range groupEquivalentRows(rows) {
		out.WriteString("\n### " + group.Label + "\n\n")
		writeEquivalentMirrorTable(&out, mapping, group.Rows, vendors)
	}
	if len(vendorOnly) != 0 {
		out.WriteString("\n## Vendor-only gaps\n\n")
		out.WriteString("| Intent | " + strings.Join(mapping.vendorLabels(vendors), " | ") + " | Notes |\n")
		out.WriteString("| --- | " + strings.Repeat("--- | ", len(vendors)) + "--- |\n")
		for _, entry := range vendorOnly {
			out.WriteString("| " + markdownCell(entry.Intent))
			for _, vendor := range vendors {
				out.WriteString(" | " + vendorOnlyMirrorCell(entry.Vendors[vendor]))
			}
			out.WriteString(" | " + markdownCell(entry.Notes) + " |\n")
		}
	}
	return strings.TrimRight(out.String(), "\n") + "\n"
}

// writeEquivalentMirrorTable writes one side-by-side table into a mirror.
func writeEquivalentMirrorTable(
	out *strings.Builder, mapping *equivalentMapping, rows []*equivalentRow, vendors []string,
) {
	out.WriteString("| Ze | Mode | " + strings.Join(mapping.vendorLabels(vendors), " | ") + " | Details |\n")
	out.WriteString("| --- | --- | " + strings.Repeat("--- | ", len(vendors)) + "--- |\n")
	for _, row := range rows {
		out.WriteString("| `" + markdownCell(row.Command.Path) + "` | " +
			markdownCell(commandModeLabel(row.Command.Mode)))
		for _, vendor := range vendors {
			out.WriteString(" | " + vendorMirrorCell(row.vendorCommands(vendor)))
		}
		out.WriteString(" | [details](" + row.Slug + "/) |\n")
	}
}

// vendorMirrorCell writes one vendor's commands as one Markdown table cell.
func vendorMirrorCell(commands []vendorEquivalent) string {
	if len(commands) == 0 {
		return "-"
	}
	spans := make([]string, 0, len(commands))
	for _, item := range commands {
		spans = append(spans, "`"+markdownCell(item.Command.Command)+"`")
	}
	return strings.Join(spans, "<br>")
}

// vendorOnlyMirrorCell writes one vendor's commands for an intent no Ze command
// answers, where there is no row to carry the intent alongside them.
func vendorOnlyMirrorCell(commands []equivalentCommand) string {
	if len(commands) == 0 {
		return "-"
	}
	spans := make([]string, 0, len(commands))
	for _, item := range commands {
		spans = append(spans, "`"+markdownCell(item.Command)+"`")
	}
	return strings.Join(spans, "<br>")
}
