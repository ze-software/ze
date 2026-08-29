// Design: website/AI.md -- a page's title, description and category come from its source
// Detail: docs.go calls each of these once per page; doctransform.go edits the rendered body.
package site

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// The seven topic hues a page title can take. They are the Features page's own
// colors, so a reader learns one convention across the whole site.
//
// Each one is named because a category is written in three places that must
// agree: a row of the recovered registry, a page's own front matter, and the
// cat- class the section element opens with.
const (
	categoryAutomate = "automate"
	categoryObserve  = "observe"
	categoryOperate  = "operate"
	categoryPlatform = "platform"
	categoryRouting  = "routing"
	categorySecure   = "secure"
	categoryServices = "services"
)

// pageCategories is the set a front matter category is checked against.
var pageCategories = map[string]bool{
	categoryAutomate: true, categoryObserve: true, categoryOperate: true,
	categoryPlatform: true, categoryRouting: true, categorySecure: true,
	categoryServices: true,
}

// firstHeading matches the first level 1 heading of a Markdown source, which
// is the page title when the front matter states none.
var firstHeading = regexp.MustCompile(`(?m)^#\s+(.+)$`)

// pageTitle answers the title one page carries: its front matter first, then
// its first heading, and "Ze" for a source with neither.
func pageTitle(metadata map[string]string, source string) string {
	if title := metadata["title"]; title != "" {
		return title
	}
	match := firstHeading.FindStringSubmatch(source)
	if match == nil {
		return "Ze"
	}
	return strings.TrimSpace(match[1])
}

// pageDescription answers the meta description: the registry's own, then the
// source's front matter, then the generic one.
//
// The registry wins because a description written beside the route describes
// the page's public role, and the front matter describes the source.
func pageDescription(page sitePage, metadata map[string]string) string {
	if page.Desc != "" {
		return page.Desc
	}
	if description := metadata["description"]; description != "" {
		return description
	}
	return "Ze documentation."
}

// pageCategory answers the topic hue one page takes, and refuses a category
// the site has no color for.
func pageCategory(page sitePage, metadata map[string]string) (string, error) {
	category := page.Category
	if category == "" {
		category = metadata["category"]
	}
	if category == "" {
		return "", nil
	}
	if !pageCategories[category] {
		return "", fmt.Errorf("category %q is not one of the site's seven: %s", category, strings.Join(sortedCategories(), ", "))
	}
	return category, nil
}

// sortedCategories names the seven categories in one order, so an error reads
// the same every time it is raised.
func sortedCategories() []string {
	names := make([]string, 0, len(pageCategories))
	for name := range pageCategories {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// tableColumnsKey is the one front matter flag a page source can set, which
// turns off the per-column controls the site adds to a wide table.
const tableColumnsKey = "table-columns"

// tableColumnsEnabled reads that flag. A source that states nothing keeps the
// controls; a value the vocabulary does not name is refused rather than read
// as false, because a typo that silently turns a feature off is invisible.
func tableColumnsEnabled(metadata map[string]string) (bool, error) {
	value, stated := metadata[tableColumnsKey]
	if !stated {
		return true, nil
	}
	switch strings.ToLower(value) {
	case "1", "true", "yes", "on":
		return true, nil
	case "0", "false", "no", "off":
		return false, nil
	}
	return false, fmt.Errorf("front matter %q must be one of: true, false, yes, no, on, off, 1, 0", tableColumnsKey)
}

// numberTokenSpecs maps a prose token name to its path in the facts snapshot.
//
// A website-owned source writes {{ze:unit-tests}} rather than a count, so
// published prose cannot drift from the snapshot. A docs/ source must not use
// one: those files also render raw on the code host, where the token would
// show through.
var numberTokenSpecs = map[string]string{
	"cli-commands":         "cli_commands",
	"changes":              "changes",
	"config-sections":      "config_sections",
	"dependencies":         "dependencies",
	"e2e-tests":            "tests.e2e_display",
	"editor-tests":         "tests.editor_display",
	"features":             "features.core_experimental",
	"fuzz-targets":         "tests.fuzz_display",
	"interop-scenarios":    "interop.scenarios_display",
	"interop-targets":      "interop.target_display",
	"repo-design-comments": "repo.design_comments_display",
	"repo-detail-comments": "repo.detail_comments_display",
	"repo-go-packages":     "repo.go_packages_display",
	"rfc-enrolled":         "rfc.enrolled_display",
	"rfc-gated-must":       "rfc.gated_must_display",
	"rfc-requirements":     "rfc.requirements_display",
	"rfc-summaries":        "rfc.summaries_display",
	"unit-tests":           "tests.unit_display",
}

// numberTokenPattern matches one {{ze:name}} token in a page source.
var numberTokenPattern = regexp.MustCompile(`\{\{ze:([a-z0-9-]+)\}\}`)

// numberTokens are the live counts a page's prose tokens resolve to, read once
// for a whole build.
//
// An EMPTY set is what a first build into an empty tree reads, because the
// artifact carries no facts snapshot yet. Substitution then leaves every token
// alone rather than guessing a number, which is the same answer it gives for a
// snapshot that states no value for one token.
type numberTokens map[string]string

// loadNumberTokens reads the facts snapshot the previous build published and
// flattens the keys the prose tokens name.
//
// The snapshot is read from the ARTIFACT rather than from the source tree,
// because it is the build's own output: the facts producer writes it and the
// page producers read it back. An absent file is not an error.
func loadNumberTokens(output string) (numberTokens, error) {
	path := filepath.Join(output, "data", "site-facts.json")
	tokens := numberTokens{}
	content, err := os.ReadFile(path) //nolint:gosec // a site build reads the artifact it was pointed at
	if os.IsNotExist(err) {
		return tokens, nil
	}
	if err != nil {
		return nil, err
	}
	var facts map[string]any
	if err := json.Unmarshal(content, &facts); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	for name, key := range numberTokenSpecs {
		if value, found := factValue(facts, key); found {
			tokens[name] = value
		}
	}
	return tokens, nil
}

// factValue answers one dotted key of the facts snapshot as the string a page
// shows. A key naming an object, or naming nothing, answers not-found.
func factValue(facts map[string]any, key string) (string, bool) {
	var value any = facts
	for part := range strings.SplitSeq(key, ".") {
		object, isObject := value.(map[string]any)
		if !isObject {
			return "", false
		}
		next, present := object[part]
		if !present {
			return "", false
		}
		value = next
	}
	switch typed := value.(type) {
	case string:
		return typed, true
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), true
	}
	return "", false
}

// substitute resolves every {{ze:name}} token of one page source.
//
// marked asks for a data-ze-stat span around each number, which is what lets a
// build check a published number against the snapshot it came from. The
// Markdown mirror takes the plain number instead.
//
// A token this site has no spec for is refused by name: it can never resolve,
// so leaving it would publish the literal braces to a reader. A build with no
// snapshot leaves every token alone, because that is a build that has not
// computed the numbers yet rather than a source that is wrong.
func (tokens numberTokens) substitute(source string, marked bool) (string, error) {
	if !strings.Contains(source, "{{ze:") {
		return source, nil
	}
	var unknown []string
	replaced := numberTokenPattern.ReplaceAllStringFunc(source, func(token string) string {
		name := numberTokenPattern.FindStringSubmatch(token)[1]
		key, known := numberTokenSpecs[name]
		if !known {
			unknown = append(unknown, name)
			return token
		}
		value, resolved := tokens[name]
		if !resolved {
			return token
		}
		if !marked {
			return value
		}
		return `<span data-ze-stat="` + html.EscapeString(key) + `">` + html.EscapeString(value) + `</span>`
	})
	if len(unknown) != 0 {
		return "", fmt.Errorf("unknown site number token {{ze:%s}}; the site states %d tokens", unknown[0], len(numberTokenSpecs))
	}
	return replaced, nil
}

// imageReference matches a local image a page source shows, written either as
// Markdown or as an img element.
var imageReference = regexp.MustCompile(`!\[[^\]]*\]\(\s*([^)\s]+)|<img\b[^>]*?\bsrc="([^"]+)"|<img\b[^>]*?\bsrc='([^']+)'`)

// copyReferencedImages publishes the images a page shows from beside its
// source to beside its page.
//
// A source at docs/guide/chaos-testing.md publishes to guides/chaos-testing/,
// one directory deeper, so a relative reference resolves against the source
// directory on one side and the page's own directory on the other. Copying the
// file keeps the link valid at both ends.
//
// Only a same-tree relative reference is published. A remote URL, a rooted
// path and a parent path each name something this page does not own.
func copyReferencedImages(sourcePath, destinationPath, source string) error {
	for _, match := range imageReference.FindAllStringSubmatch(source, -1) {
		reference := match[1] + match[2] + match[3]
		if reference == "" || strings.Contains(reference, "://") {
			continue
		}
		if strings.HasPrefix(reference, "/") || strings.HasPrefix(reference, "#") ||
			strings.HasPrefix(reference, "data:") || strings.HasPrefix(reference, "..") {
			continue
		}
		from := filepath.Join(filepath.Dir(sourcePath), filepath.FromSlash(reference))
		info, err := os.Stat(from)
		if err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("%s shows the image %s, which is not a file at %s", path.Base(sourcePath), reference, from)
		}
		to := filepath.Join(filepath.Dir(destinationPath), filepath.FromSlash(reference))
		if err := copyFileTo(from, to); err != nil {
			return err
		}
	}
	return nil
}

// copyFileTo copies one file, creating the directories above it.
func copyFileTo(from, to string) error {
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil { //nolint:gosec // published web content: a web server, often another account, serves these bytes
		return err
	}
	source, err := os.Open(from) //nolint:gosec // a site build reads the checkout it was pointed at
	if err != nil {
		return err
	}
	defer source.Close() //nolint:errcheck // the read side has nothing to report on close

	target, err := os.OpenFile(to, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644) //nolint:gosec // published web content: a web server, often another account, serves these bytes
	if err != nil {
		return err
	}
	if _, err := io.Copy(target, source); err != nil {
		_ = target.Close()
		return fmt.Errorf("copy %s to %s: %w", from, to, err)
	}
	return target.Close()
}
