// Design: website/AI.md -- the dependency reference is go.mod's versions under a curated list
// Detail: datapages.go holds the other three data pages and the inline markup vocabulary.
package site

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// The dependency reference registers from here.
func init() {
	registerProducer(Producer{Name: dependenciesDirectory, Render: renderDependencies})
}

// Where the dependency page reads and writes.
const (
	dependenciesDataFile = "dependencies.json"
	// dependenciesDirectory is the page's own name under reference/, and it is
	// the producer's name: one page, one directory, one claim.
	dependenciesDirectory = "dependencies"
	dependenciesDest      = "reference/" + dependenciesDirectory + "/" + pageIndexFile
	dependenciesRoot      = "../../"
	dependenciesRoute     = "/reference/" + dependenciesDirectory + "/"
)

// dependencyData is data/dependencies.json: the curated grouping, which decides
// both the order of the page and which modules appear on it.
//
// go.mod supplies the VERSION of a module and nothing else. It states no group
// and no order, so a module go.mod lists and this file does not is invisible on
// the page, which is what the drift check below refuses.
type dependencyData struct {
	Categories []dependencyCategory `json:"categories"`
}

// dependencyCategory is one collapsible group of the page.
type dependencyCategory struct {
	Name    string             `json:"name"`
	Modules []dependencyModule `json:"modules"`
}

// dependencyModule is one curated entry: the module path and why Ze imports it.
type dependencyModule struct {
	Module string `json:"module"`
	Why    string `json:"why"`
}

// total counts the curated modules across every group.
func (data dependencyData) total() int {
	count := 0
	for _, category := range data.Categories {
		count += len(category.Modules)
	}
	return count
}

// The two shapes a go.mod require block takes. A module is direct when its line
// carries no "// indirect" comment.
var (
	requireBlock = regexp.MustCompile(`(?s)require \(\n(.*?)\n\)`)
	requireLine  = regexp.MustCompile(`^\s*(\S+)\s+(\S+)(\s*//\s*indirect)?\s*$`)
)

// directModuleVersions answers the pinned version of every direct dependency of
// one go.mod.
//
// The file is read rather than `go list` so a site build needs no module cache
// and no network, and so the answer is the same on a machine that has never
// built Ze.
func directModuleVersions(path string) (map[string]string, error) {
	content, err := os.ReadFile(path) //nolint:gosec // a site build reads the checkout it was pointed at
	if err != nil {
		return nil, fmt.Errorf("read go.mod: %w", err)
	}
	versions := make(map[string]string)
	for _, block := range requireBlock.FindAllStringSubmatch(string(content), -1) {
		for line := range strings.SplitSeq(block[1], "\n") {
			fields := requireLine.FindStringSubmatch(line)
			if fields == nil {
				continue
			}
			if fields[3] != "" {
				continue
			}
			versions[fields[1]] = fields[2]
		}
	}
	if len(versions) == 0 {
		return nil, fmt.Errorf("read go.mod: %s declares no direct dependency", path)
	}
	return versions, nil
}

// checkDependencyDrift refuses a page that would misstate what Ze depends on.
//
// The retired renderer warned on each mismatch and its build then exited
// non-zero, so a drifted list never reached a reader. It is a refusal here for
// the same reason and by name: a direct dependency with no curated entry is
// simply absent from the page, and an entry for a module go.mod no longer
// requires publishes a dependency Ze does not have.
func checkDependencyDrift(versions map[string]string, data dependencyData) error {
	curated := make(map[string]bool, data.total())
	for _, category := range data.Categories {
		for _, module := range category.Modules {
			if curated[module.Module] {
				return fmt.Errorf("data/%s names %s twice", dependenciesDataFile, module.Module)
			}
			curated[module.Module] = true
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
	sort.Strings(undocumented)
	sort.Strings(retired)

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
	var data dependencyData
	if err := readSourceJSON(paths.Source, dependenciesDataFile, &data); err != nil {
		return nil, err
	}
	versions, err := directModuleVersions(filepath.Join(paths.Repository, "go.mod"))
	if err != nil {
		return nil, err
	}
	if err := checkDependencyDrift(versions, data); err != nil {
		return nil, err
	}
	links, err := loadPageLinks(paths.Source)
	if err != nil {
		return nil, err
	}

	description := "Every direct Go dependency Ze ships with and why, generated from go.mod -- " +
		strconv.Itoa(data.total()) + " packages across " + strconv.Itoa(len(data.Categories)) + " groups."
	shell := pageShell{
		Title:       "Dependencies - Ze",
		Description: description,
		Root:        dependenciesRoot,
		Path:        dependenciesDest,
		Sidebar:     pageSidebar(dependenciesRoot, dependenciesDest, links),
	}
	if err := writePublishedPage(paths.Output, dependenciesDest,
		shell.render(dependenciesBody(data, versions)),
		dependenciesMirror(data, versions)); err != nil {
		return nil, err
	}
	return []string{dependenciesRoute}, nil
}

// dependenciesLead is the sentence the hero and the mirror both open with. The
// code span is the one difference between the two, so it is a parameter.
func dependenciesLead(codeSpan string, total int) string {
	return "Ze is Go, and Go code leans on packages. " + strconv.Itoa(total) +
		" direct dependencies, read straight from " + codeSpan +
		" so the list and versions can't drift -- each one with a plain-English reason it's " +
		"there, grounded in where it's actually imported, not its own pitch."
}

// dependenciesBody renders the page under <main>: the hero, the filter box, and
// one open group for each curated category in the file's own order.
func dependenciesBody(data dependencyData, versions map[string]string) string {
	var body strings.Builder
	body.WriteString("            <section aria-labelledby=\"dependencies-title\" class=\"md-content reveal cat-platform\">\n")
	body.WriteString(pageHero("Dependencies", dependenciesLead("<code>go.mod</code>", data.total()),
		capitalizeWord(categoryPlatform), ` id="dependencies-title"`, heroClasses) + "\n")
	body.WriteString("                <input id=\"dep-search\" type=\"search\" " +
		"placeholder=\"Filter dependencies (e.g. netlink, ssh, prometheus)...\" " +
		"aria-label=\"Filter dependencies\" />\n")
	for _, category := range data.Categories {
		body.WriteString(dependencyGroupHTML(category, versions))
	}
	body.WriteString("            </section>\n")
	return body.String()
}

// dependencyGroupHTML renders one group as a table of its curated modules.
func dependencyGroupHTML(category dependencyCategory, versions map[string]string) string {
	var out strings.Builder
	out.WriteString("<details class=\"dep-group\" open>\n")
	out.WriteString("<summary>" + html.EscapeString(category.Name) +
		" <span class=\"dep-group-count\">" + strconv.Itoa(len(category.Modules)) + "</span></summary>\n")
	out.WriteString("<table><thead><tr><th>Module</th><th>Version</th><th>Why we use it</th></tr></thead><tbody>\n")
	for _, module := range category.Modules {
		out.WriteString("<tr><td><code>" + html.EscapeString(module.Module) + "</code></td><td><code>" +
			html.EscapeString(versions[module.Module]) + "</code></td><td>" +
			inlineMarkup(module.Why) + "</td></tr>\n")
	}
	out.WriteString("</tbody></table></details>\n")
	return out.String()
}

// dependenciesMirror renders the Markdown sibling: one table for each group, in
// the same order the page writes them.
func dependenciesMirror(data dependencyData, versions map[string]string) string {
	var mirror strings.Builder
	mirror.WriteString("# Dependencies\n\n")
	mirror.WriteString(dependenciesLead("`go.mod`", data.total()) + "\n\n")
	for _, category := range data.Categories {
		mirror.WriteString("## " + category.Name + " (" + strconv.Itoa(len(category.Modules)) + ")\n\n")
		mirror.WriteString("| Module | Version | Why we use it |\n| --- | --- | --- |\n")
		for _, module := range category.Modules {
			// A pipe inside a cell would end the cell, so it is escaped. The
			// module path and the version cannot carry one.
			why := strings.ReplaceAll(module.Why, "|", `\|`)
			mirror.WriteString("| `" + module.Module + "` | `" + versions[module.Module] + "` | " + why + " |\n")
		}
		mirror.WriteString("\n")
	}
	return strings.TrimSpace(mirror.String()) + "\n"
}
