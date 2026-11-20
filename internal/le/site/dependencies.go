// Design: website/AI.md -- the dependency reference is go.mod's versions under a curated list
// Related: docs/architecture/third-party-credits.md -- the declaration this page reads, and the record generated from it
// Detail: datapages.go holds the other three data pages and the inline markup vocabulary.
package site

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/deps/credits"
)

// The dependency reference registers from here.
func init() {
	registerProducer(Producer{Name: dependenciesDirectory, Render: renderDependencies})
}

// Where the dependency page reads and writes.
const (
	dependenciesDataFile = "dependencies.json"
	// dependenciesDirectory is the page's own name under reference/. It is also
	// the producer's name, the prose token that carries its count, and the
	// route the page published at before it moved: one page, one name.
	dependenciesDirectory = "dependencies"
	dependenciesDest      = "reference/" + dependenciesDirectory + "/" + pageIndexFile
	dependenciesRoot      = "../../"
	dependenciesRoute     = "/reference/" + dependenciesDirectory + "/"
	// goModuleFile names the module file, relative to the checkout root. The
	// dependency page reads its versions and the facts snapshot counts its
	// direct requirements, so the path is stated once.
	goModuleFile = "go.mod"
)

// What a cell says when the declaration holds no value for it. Each is a
// statement a reader can act on, never an empty cell that reads as "none".
const (
	// creditNotRecorded is a licence or holder the declaration leaves empty.
	// It is the spelling `ze thanks` uses for a module with no record
	// (internal/core/credits NotRecorded), so the page and the binary say one
	// thing for one state.
	creditNotRecorded = "not recorded"
	// holderUnstated is a work whose own files name no copyright holder
	// (`copyright-unstated` in the declaration).
	holderUnstated = "No copyright holder stated"
	// creditUnversioned is a copied work the declaration gives no version: a
	// copy taken from a project that tags no release of it.
	creditUnversioned = "not versioned"
)

// The section that lists what the shipped binary carries beyond the curated
// direct modules, and the group its indirect modules sit under.
const (
	shippedSectionTitle = "Also in the shipped binary"
	shippedSectionID    = "shipped-title"
	shippedLinkedGroup  = "Indirect Go Modules"
)

// The declaration is website/data/dependencies.json, read through the credits
// tool's own type and reader (credits.Declaration, credits.ReadDeclaration):
// the curated grouping decides both the order of the page and which modules
// appear on it, and the same file is what `./le deps credits update` generates
// the binary's record from. One schema for one file, so the page cannot read a
// key the record spells another way.
//
// go.mod supplies the VERSION of a module and nothing else. It states no group
// and no order, so a module go.mod lists and the declaration does not is
// invisible on the page, which is what the drift check below refuses.

// readDependencyDeclaration decodes the declaration at path. label names the
// copy in a refusal: the source a person edits, or the copy a build published.
func readDependencyDeclaration(path, label string) (credits.Declaration, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // a site build reads the checkout it was pointed at
	if err != nil {
		return credits.Declaration{}, fmt.Errorf("read %s: %w", label, err)
	}
	decl, err := credits.ReadDeclaration(raw)
	if err != nil {
		return credits.Declaration{}, fmt.Errorf("read %s: %w", label, err)
	}
	return decl, nil
}

// curatedTotal counts the curated modules across every group: the direct
// dependencies the lead sentence counts. The shipped section is not counted,
// because nothing in it is a direct dependency.
func curatedTotal(decl credits.Declaration) int {
	count := 0
	for _, category := range decl.Categories {
		count += len(category.Modules)
	}
	return count
}

// The two shapes a go.mod require block takes, and its language line. A
// module is direct when its line carries no "// indirect" comment.
var (
	requireBlock = regexp.MustCompile(`(?s)require \(\n(.*?)\n\)`)
	requireLine  = regexp.MustCompile(`^\s*(\S+)\s+(\S+)(\s*//\s*indirect)?\s*$`)
	goDirective  = regexp.MustCompile(`(?m)^go\s+(\S+)\s*$`)
)

// goModule is what the dependency page reads from one go.mod.
type goModule struct {
	// direct holds the version of every direct requirement.
	direct map[string]string
	// required holds the version of every requirement, indirect included: the
	// version of a module the shipped binary links without Ze importing it.
	required map[string]string
	// language is the go directive's version, empty when the file has none.
	language string
}

// readGoModule parses the require blocks and the go directive of one go.mod.
//
// The file is read rather than `go list` so a site build needs no module cache
// and no network, and so the answer is the same on a machine that has never
// built Ze.
func readGoModule(path string) (goModule, error) {
	content, err := os.ReadFile(path) //nolint:gosec // a site build reads the checkout it was pointed at
	if err != nil {
		return goModule{}, fmt.Errorf("read go.mod: %w", err)
	}
	module := goModule{direct: make(map[string]string), required: make(map[string]string)}
	for _, block := range requireBlock.FindAllStringSubmatch(string(content), -1) {
		for line := range strings.SplitSeq(block[1], "\n") {
			fields := requireLine.FindStringSubmatch(line)
			if fields == nil {
				continue
			}
			module.required[fields[1]] = fields[2]
			if fields[3] != "" {
				continue
			}
			module.direct[fields[1]] = fields[2]
		}
	}
	if len(module.direct) == 0 {
		return goModule{}, fmt.Errorf("read go.mod: %s declares no direct dependency", path)
	}
	if found := goDirective.FindStringSubmatch(string(content)); found != nil {
		module.language = found[1]
	}
	return module, nil
}

// directModuleVersions answers the pinned version of every direct dependency of
// one go.mod.
func directModuleVersions(path string) (map[string]string, error) {
	module, err := readGoModule(path)
	if err != nil {
		return nil, err
	}
	return module.direct, nil
}

// checkDependencyDrift refuses a page that would misstate what Ze depends on.
//
// The retired renderer warned on each mismatch and its build then exited
// non-zero, so a drifted list never reached a reader. It is a refusal here for
// the same reason and by name: a direct dependency with no curated entry is
// simply absent from the page, and an entry for a module go.mod no longer
// requires publishes a dependency Ze does not have.
func checkDependencyDrift(versions map[string]string, decl credits.Declaration) error {
	curated := make(map[string]bool, curatedTotal(decl))
	for _, category := range decl.Categories {
		for index := range category.Modules {
			name := category.Modules[index].Module
			if curated[name] {
				return fmt.Errorf("data/%s names %s twice", dependenciesDataFile, name)
			}
			curated[name] = true
		}
	}

	var undocumented, retired []string
	for module := range versions {
		if !curated[module] {
			undocumented = append(undocumented, module)
		}
	}
	for module := range curated {
		if versions[module] == "" {
			retired = append(retired, module)
		}
	}
	slices.Sort(undocumented)
	slices.Sort(retired)

	if len(undocumented) != 0 {
		return fmt.Errorf("go.mod requires %s directly with no entry in data/%s: add one",
			strings.Join(undocumented, ", "), dependenciesDataFile)
	}
	if len(retired) != 0 {
		return fmt.Errorf("data/%s has %s, which go.mod no longer requires directly: remove the entry",
			dependenciesDataFile, strings.Join(retired, ", "))
	}
	return nil
}

// renderDependencies publishes the dependency reference and its mirror.
func renderDependencies(paths Paths) ([]string, error) {
	decl, err := readDependencyDeclaration(filepath.Join(paths.Source, "data", dependenciesDataFile),
		"data/"+dependenciesDataFile)
	if err != nil {
		return nil, err
	}
	module, err := readGoModule(filepath.Join(paths.Repository, goModuleFile))
	if err != nil {
		return nil, err
	}
	if err := checkDependencyDrift(module.direct, decl); err != nil {
		return nil, err
	}
	shipped, err := shippedGroups(decl, module)
	if err != nil {
		return nil, err
	}
	links, err := loadPageLinks(paths.Source)
	if err != nil {
		return nil, err
	}

	description := "Ze's direct Go dependencies and why each is used, with versions from go.mod: " +
		strconv.Itoa(curatedTotal(decl)) + " packages across " + strconv.Itoa(len(decl.Categories)) + " groups."
	shell := pageShell{
		Title:       "Dependencies - Ze",
		Description: description,
		Root:        dependenciesRoot,
		Path:        dependenciesDest,
		Sidebar:     pageSidebar(dependenciesRoot, dependenciesDest, links),
	}
	if err := writePublishedPage(paths.Output, dependenciesDest,
		shell.render(dependenciesBody(decl, module.direct, shipped)),
		dependenciesMirror(decl, module.direct, shipped)); err != nil {
		return nil, err
	}
	return []string{dependenciesRoute}, nil
}

// dependenciesLead is the sentence the hero and the mirror both open with. The
// code span is the one difference between the two, so it is a parameter.
func dependenciesLead(codeSpan string, total int) string {
	return "Ze uses " + strconv.Itoa(total) + " direct Go dependencies. The list and " +
		"versions come from " + codeSpan + ", and each entry explains how Ze uses that " +
		"dependency, with references to the code that imports it."
}

// shippedLead is the paragraph the shipped section opens with, on the page
// and in the mirror. codeSpan renders a command or file name in each form.
func shippedLead(codeSpan func(string) string) string {
	return "The shipped " + codeSpan("ze") + " binary also carries these works: the Go modules " +
		"the direct dependencies bring in, Go itself, the web interface assets, and code copied " +
		"or ported into Ze. None of them is a direct dependency, so the count above leaves them " +
		"out. Module versions come from " + codeSpan("go.mod") + "; the others are the versions " +
		"of the copies Ze carries. " + codeSpan("ze thanks") + " lists the works one binary " +
		"carries, and " + codeSpan("ze thanks licence") + " prints their licence texts."
}

// shippedWork is one row of the shipped section: a linked module or a work
// that is not a module, with the values the page prints for it.
type shippedWork struct {
	name    string
	version string
	// unversioned marks a copied work the declaration gives no version, so
	// its cell says so rather than printing an empty version.
	unversioned bool
	licence     string
	note        string
	copyright   string
	unstated    bool
	source      string
}

// shippedGroup is one group of the shipped section, in declaration order.
type shippedGroup struct {
	name  string
	works []shippedWork
}

// shippedGroups answers the shipped section: the declaration's linked modules
// under one group, then its works under their own groups, each in the order
// the declaration first names it.
//
// A version the page cannot state is refused rather than printed empty: a
// linked module go.mod requires at no version, and a toolchain work while
// go.mod states no go directive.
func shippedGroups(decl credits.Declaration, module goModule) ([]shippedGroup, error) {
	var groups []shippedGroup
	if len(decl.Linked) != 0 {
		linked := shippedGroup{name: shippedLinkedGroup}
		for index := range decl.Linked {
			entry := &decl.Linked[index]
			version := module.required[entry.Module]
			if version == "" {
				return nil, fmt.Errorf("data/%s lists %s under linked, which go.mod does not require, "+
					"so the page cannot state its version: run ./le deps credits check, which names the fix",
					dependenciesDataFile, entry.Module)
			}
			linked.works = append(linked.works, shippedWork{
				name: entry.Module, version: version, licence: entry.Licence, note: entry.LicenceNote,
				copyright: entry.Copyright, unstated: entry.CopyrightUnstated, source: entry.Source,
			})
		}
		groups = append(groups, linked)
	}
	for index := range decl.Works {
		work := &decl.Works[index]
		version, err := shippedWorkVersion(work, module)
		if err != nil {
			return nil, err
		}
		row := shippedWork{
			name: work.Name, version: version, unversioned: version == "", licence: work.Licence,
			copyright: work.Copyright, unstated: work.CopyrightUnstated, source: work.Source,
		}
		at := slices.IndexFunc(groups, func(group shippedGroup) bool { return group.name == work.Group })
		if at < 0 {
			groups = append(groups, shippedGroup{name: work.Group})
			at = len(groups) - 1
		}
		groups[at].works = append(groups[at].works, row)
	}
	return groups, nil
}

// shippedWorkVersion answers the version the page prints for a work that is
// not a module, and an empty version for a copied work the declaration gives
// none, which the caller marks unversioned. The toolchain work declares none
// either, because a binary reads Go's version from its own build information;
// the page states the Go release go.mod pins, which is the toolchain a build
// of this tree uses.
func shippedWorkVersion(work *credits.DeclaredWork, module goModule) (string, error) {
	if work.Toolchain {
		if module.language == "" {
			return "", fmt.Errorf("go.mod states no go directive, so the page cannot state the version of %s, "+
				"which data/%s marks as the Go toolchain: add the go line to go.mod", work.Name, dependenciesDataFile)
		}
		return "go" + module.language, nil
	}
	return work.Version, nil
}

// versionCellHTML renders a shipped work's version as code, or the statement
// that the declaration gives it none.
func versionCellHTML(work *shippedWork) string {
	if work.unversioned {
		return creditUnversioned
	}
	return "<code>" + html.EscapeString(work.version) + "</code>"
}

// versionCellMarkdown is versionCellHTML for the mirror.
func versionCellMarkdown(work *shippedWork) string {
	if work.unversioned {
		return creditUnversioned
	}
	return "`" + work.version + "`"
}

// dependenciesBody renders the page under <main>: the hero, the filter box,
// one open group for each curated category in the file's own order, and the
// shipped section. The filter reads every .dep-group, so it finds a shipped
// work as it finds a curated module.
func dependenciesBody(decl credits.Declaration, versions map[string]string, shipped []shippedGroup) string {
	var body textbuf.Buffer
	body.Reset().Str("            <section aria-labelledby=\"dependencies-title\" class=\"md-content reveal cat-platform\">\n")
	body.Str(pageHero("Dependencies", dependenciesLead("<code>go.mod</code>", curatedTotal(decl)),
		categoryLabel(categoryPlatform), ` id="dependencies-title"`, heroClasses)).Byte('\n')
	body.Str("                <input id=\"dep-search\" type=\"search\" ").
		Str("placeholder=\"Filter dependencies (e.g. netlink, ssh, prometheus)...\" ").
		Str("aria-label=\"Filter dependencies\" />\n")
	for _, category := range decl.Categories {
		body.Str(dependencyGroupHTML(category, versions))
	}
	if len(shipped) != 0 {
		body.Str("<h2 id=\"").Str(shippedSectionID).Str("\">").Str(shippedSectionTitle).Str("</h2>\n")
		body.Str("<p>").Str(shippedLead(func(name string) string { return "<code>" + name + "</code>" })).
			Str("</p>\n")
		for _, group := range shipped {
			body.Str(shippedGroupHTML(group))
		}
	}
	body.Str("            </section>\n")
	return body.String()
}

// dependencyGroupHTML renders one group as a table of its curated modules.
func dependencyGroupHTML(category credits.Category, versions map[string]string) string {
	var out textbuf.Buffer
	out.Reset().Str("<details class=\"dep-group\" open>\n")
	out.Str("<summary>").Str(html.EscapeString(category.Name)).
		Str(" <span class=\"dep-group-count\">").Int(int64(len(category.Modules))).Str("</span></summary>\n")
	out.Str("<table><thead><tr><th>Module</th><th>Version</th><th>Licence</th><th>Copyright</th>").
		Str("<th>Why we use it</th></tr></thead><tbody>\n")
	for index := range category.Modules {
		module := &category.Modules[index]
		out.Str(`<tr><td><code class="concept-software">`).Str(html.EscapeString(module.Module)).Str("</code></td><td><code>").
			Str(html.EscapeString(versions[module.Module])).Str("</code></td><td>").
			Str(licenceCellHTML(module.Licence, module.LicenceNote)).Str("</td><td>").
			Str(html.EscapeString(holderCell(module.Copyright, module.CopyrightUnstated))).Str("</td><td>").
			Str(inlineMarkup(module.Why)).Str("</td></tr>\n")
	}
	out.Str("</tbody></table></details>\n")
	return out.String()
}

// shippedGroupHTML renders one group of the shipped section. A work's name
// links to its source, the location the licence record names for it.
func shippedGroupHTML(group shippedGroup) string {
	var out textbuf.Buffer
	out.Reset().Str("<details class=\"dep-group\" open>\n")
	out.Str("<summary>").Str(html.EscapeString(group.name)).
		Str(" <span class=\"dep-group-count\">").Int(int64(len(group.works))).Str("</span></summary>\n")
	out.Str("<table><thead><tr><th>Name</th><th>Version</th><th>Licence</th><th>Copyright</th>").
		Str("</tr></thead><tbody>\n")
	for index := range group.works {
		work := &group.works[index]
		out.Str(`<tr><td><a href="`).Str(html.EscapeString(work.source)).Str(`"><code class="concept-software">`).
			Str(html.EscapeString(work.name)).Str("</code></a></td><td>").
			Str(versionCellHTML(work)).Str("</td><td>").
			Str(licenceCellHTML(work.licence, work.note)).Str("</td><td>").
			Str(html.EscapeString(holderCell(work.copyright, work.unstated))).Str("</td></tr>\n")
	}
	out.Str("</tbody></table></details>\n")
	return out.String()
}

// licenceCellHTML renders a licence cell: the SPDX expression as code, then
// the declaration's licence note, which says what the identifier alone cannot.
func licenceCellHTML(licence, note string) string {
	if licence == "" {
		return creditNotRecorded
	}
	cell := "<code>" + html.EscapeString(licence) + "</code>"
	if note != "" {
		cell += " (" + html.EscapeString(note) + ")"
	}
	return cell
}

// licenceCellMarkdown is licenceCellHTML for the mirror.
func licenceCellMarkdown(licence, note string) string {
	if licence == "" {
		return creditNotRecorded
	}
	cell := "`" + licence + "`"
	if note != "" {
		cell += " (" + markdownCell(note) + ")"
	}
	return cell
}

// holderCell answers the copyright cell's text: the declared holders, the
// statement that the work's files name none, or that the declaration holds
// no holder.
func holderCell(copyright string, unstated bool) string {
	if unstated {
		return holderUnstated
	}
	if copyright == "" {
		return creditNotRecorded
	}
	return copyright
}

// dependenciesMirror renders the Markdown sibling: one table for each group, in
// the same order the page writes them, then the shipped section.
func dependenciesMirror(decl credits.Declaration, versions map[string]string, shipped []shippedGroup) string {
	var mirror textbuf.Buffer
	mirror.Reset().Str("# Dependencies\n\n")
	mirror.Str(dependenciesLead("`go.mod`", curatedTotal(decl))).Str("\n\n")
	for _, category := range decl.Categories {
		mirror.Str("## ").Str(category.Name).Str(" (").Int(int64(len(category.Modules))).Str(")\n\n")
		mirror.Str("| Module | Version | Licence | Copyright | Why we use it |\n| --- | --- | --- | --- | --- |\n")
		for index := range category.Modules {
			module := &category.Modules[index]
			// The module path and the version cannot carry a pipe; the
			// holders, the note and the reason can.
			mirror.Str("| `").Str(module.Module).Str("` | `").Str(versions[module.Module]).Str("` | ").
				Str(licenceCellMarkdown(module.Licence, module.LicenceNote)).Str(" | ").
				Str(markdownCell(holderCell(module.Copyright, module.CopyrightUnstated))).Str(" | ").
				Str(strings.ReplaceAll(module.Why, "|", `\|`)).Str(" |\n")
		}
		mirror.Byte('\n')
	}
	if len(shipped) != 0 {
		mirror.Str("## ").Str(shippedSectionTitle).Str("\n\n")
		mirror.Str(shippedLead(func(name string) string { return "`" + name + "`" })).Str("\n\n")
		for _, group := range shipped {
			mirror.Str("### ").Str(group.name).Str(" (").Int(int64(len(group.works))).Str(")\n\n")
			mirror.Str("| Name | Version | Licence | Copyright |\n| --- | --- | --- | --- |\n")
			for index := range group.works {
				work := &group.works[index]
				mirror.Str("| [`").Str(markdownCell(work.name)).Str("`](").Str(work.source).Str(") | ").
					Str(versionCellMarkdown(work)).Str(" | ").Str(licenceCellMarkdown(work.licence, work.note)).Str(" | ").
					Str(markdownCell(holderCell(work.copyright, work.unstated))).Str(" |\n")
			}
			mirror.Byte('\n')
		}
	}
	return strings.TrimSpace(mirror.String()) + "\n"
}
