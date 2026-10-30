// Design: website/AI.md -- a page's title, description and category come from its source
// Detail: docs.go calls each of these once per page; doctransform.go edits the rendered body.
package site

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// The shared subject vocabulary for pages and cards. Meta is the neutral
// category for mixed subjects and project work.
//
// Each one is named because a category is written in three places that must
// agree: a row of the recovered registry, a page's own front matter, and the
// cat- class the section element opens with.
const (
	categoryAutomate = "automate"
	categoryMeta     = "meta"
	categoryObserve  = "observe"
	categoryOperate  = "operate"
	categoryPlatform = "platform"
	categoryRouting  = "routing"
	categorySecure   = "secure"
	categoryServices = "services"
)

// pageCategories is the vocabulary every authored category is checked against.
var pageCategories = map[string]bool{
	categoryAutomate: true, categoryObserve: true, categoryOperate: true,
	categoryPlatform: true, categoryRouting: true, categorySecure: true,
	categoryServices: true, categoryMeta: true,
}

// categoryLabel gives a category its display name without changing its identifier.
func categoryLabel(category string) string {
	if category == categoryPlatform {
		return "Setup"
	}
	if category == "" {
		return ""
	}
	return strings.ToUpper(category[:1]) + category[1:]
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
	if err := validateCategory(category); err != nil {
		return "", err
	}
	return category, nil
}

// validateCategory refuses a category the shared stylesheet cannot use.
func validateCategory(category string) error {
	if !pageCategories[category] {
		return fmt.Errorf("category %q is not one of the site's categories: %s", category, strings.Join(sortedCategories(), ", "))
	}
	return nil
}

// validateSubjectCategory keeps the seven-subject catalogs closed to neutral
// entries: their filters and the homepage overview cover those subjects only.
func validateSubjectCategory(category string) error {
	if category == categoryMeta {
		return fmt.Errorf("category %q is neutral; this catalog requires one of the seven subject categories", category)
	}
	return validateCategory(category)
}

// cardCategory validates an optional authored category. Omission means mixed or
// unspecified content, so it takes the neutral category rather than a subject.
func cardCategory(category string) (string, error) {
	if category == "" {
		return categoryMeta, nil
	}
	if err := validateCategory(category); err != nil {
		return "", err
	}
	return category, nil
}

// sortedCategories names the categories in one order, so an error reads
// the same every time it is raised.
func sortedCategories() []string {
	names := make([]string, 0, len(pageCategories))
	for name := range pageCategories {
		names = append(names, name)
	}
	slices.Sort(names)
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
