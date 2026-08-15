// Design: (none -- build tool)
//
// web_assets derives, for each page ze serves, the set of vendored web assets
// that page must load, so a head block renders the imports that page needs
// instead of the union every page needs.
//
// The set comes from the templ component graph: a page's template, every
// @component(...) it reaches transitively, and the hx-* attributes each of
// those components names. Source derivation OVER-approximates, because a
// branch no request takes still contributes its asset. Deriving from rendered
// output instead would UNDER-approximate, and the two errors do not cost the
// same. An asset shipped and unused costs bytes. An asset needed and absent
// gives a page that renders correctly and does nothing in the browser.
// TestPageImportsCoverRenderedAttributes (internal/component/web) reads the
// captured fixtures and is the other direction; a disagreement between them is
// a defect in one of the two.
//
// A page is one of two shapes:
//
//   - a SHELL renders <head>. It names itself in that head block, as
//     pageAssets(pgL2tpList).
//   - a BODY carries the //ze:page marker and renders inside a shell whose head
//     asks for the page at render time, as pageAssets(v.Page). The looking
//     glass is why this shape exists: one layout serves every page, and only
//     the peers page opens an SSE stream.
//
// Usage:
//
//	go run scripts/codegen/web_assets.go          write the per-page import sets
//	go run scripts/codegen/web_assets.go --check  fail when they are stale
//	go run scripts/codegen/web_assets.go --json   print the derived sets, write nothing
//
// An empty page set is never a valid answer: ze serves pages, and a caller that
// read the empty map as "no page loads an asset" would generate a head block
// that loads nothing (ai/rules/evidence.md).

//go:build ignore

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// generatedFile is what each surface carries, beside its sources.
const generatedFile = "page_assets.go"

// The two vendored files a rendered attribute can need. htmx.min.js is the
// core; sse.js is the extension that reads a server-sent event stream. The web
// UI needs no extension: it opens its streams with its own sse-client.js.
const (
	htmxAsset = "htmx.min.js"
	sseAsset  = "sse.js"
)

// surface is one package whose pages this generator derives.
type surface struct {
	// Dir is the package directory, from the repository root.
	Dir string
	// Pkg is the Go package the generated file declares.
	Pkg string
	// Prefix is where this package serves its assets. The generated Go holds
	// the whole URL, because a head block renders one; the JSON output holds
	// the file name, because a checker reading rendered bytes compares those.
	Prefix string
	// GoMarkup marks a surface whose markup is Go string literals rather than
	// templ. Chaos is the only one. It has one page and no component graph, so
	// that page carries every asset the package renders.
	GoMarkup bool
}

// surfaces are the three packages that serve a page of their own.
var surfaces = []surface{
	{Dir: "internal/component/web", Pkg: "web", Prefix: "/assets/"},
	{Dir: "internal/component/lg", Pkg: "lg", Prefix: "/lg/assets/"},
	{Dir: "internal/chaos/web", Pkg: "web", Prefix: "/assets/", GoMarkup: true},
}

// attributePattern finds one htmx attribute: a whitespace byte, the attribute
// name, then the equals sign a value follows. Requiring both ends keeps a file
// name such as sse-client.js out of the match. The value is captured when it is
// a quoted literal, because hx-ext names its extension there.
var attributePattern = regexp.MustCompile(`\s(hx-[a-z-]+|sse-[a-z-]+)=(?:"([^"]*)")?`)

// templDeclaration opens one templ component.
var templDeclaration = regexp.MustCompile(`^templ\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`)

// componentCall finds one @component invocation. The name can be dotted
// (@templ.Raw) or a parameter rather than a component (@content). Both are
// content this generator cannot follow, and opacity is what records that.
var componentCall = regexp.MustCompile(`@([A-Za-z_][A-Za-z0-9_.]*)`)

// selfNamedPage finds the page a head block names for itself.
var selfNamedPage = regexp.MustCompile(`pageAssets\(\s*(pg[A-Za-z0-9_]*)\s*\)`)

// anyPageAssets finds a head block asking for its assets at all.
var anyPageAssets = regexp.MustCompile(`pageAssets\(`)

// pageMarker declares that a component is rendered as a whole page inside a
// shell that names its page at render time.
const pageMarker = "//ze:page"

// component is one templ component: the markup it names, and what it renders
// that this generator cannot see.
type component struct {
	// Name is the templ component's name.
	Name string
	// File is its source, from the repository root.
	File string
	// Assets are the assets its own markup names.
	Assets []string
	// Invokes are the components it renders by name.
	Invokes []string
	// Opaque marks a component that renders content it does not name: a
	// templ.Component parameter, or templ.Raw of a string another package
	// rendered. Its asset set is everything the package can render.
	Opaque bool
	// Shell marks a component that renders <head>.
	Shell bool
	// Marked marks a component carrying the page marker.
	Marked bool
	// Names is the page a shell names for itself, empty when the shell asks for
	// a page at render time.
	Names string
	// Asks marks a shell that renders pageAssets at all. A shell that does not
	// is still hand-writing its script tags.
	Asks bool
}

// page is one derived import set.
type page struct {
	// Key names the page in the JSON output: the source that renders its head,
	// and the body when that shell serves more than one page.
	Key string
	// ID is the pageID value the generated file carries.
	ID string
	// Constant is the generated Go identifier for that ID.
	Constant string
	// Origin is the source file the page's head lives in.
	Origin string
	// Assets are the vendored files the page must load.
	Assets []string
}

// assetsFor names the assets one attribute needs.
func assetsFor(name, value string) []string {
	switch {
	case strings.HasPrefix(name, "sse-"):
		return []string{sseAsset}
	case name == "hx-ext" && strings.Contains(value, "sse"):
		// The extension is named in the value, so an element carrying only
		// hx-ext="sse" still needs the extension file.
		return []string{htmxAsset, sseAsset}
	case strings.HasPrefix(name, "hx-"):
		return []string{htmxAsset}
	default:
		return nil
	}
}

// markupAssets returns every asset the attributes in text need.
func markupAssets(text string) []string {
	var found []string

	for _, match := range attributePattern.FindAllStringSubmatch(text, -1) {
		found = append(found, assetsFor(match[1], match[2])...)
	}

	return sorted(found)
}

// sorted returns the input sorted, without repeats, and never nil.
func sorted(in []string) []string {
	out := slices.Clone(in)
	if out == nil {
		out = []string{}
	}

	slices.Sort(out)

	return slices.Compact(out)
}

// parseTempl reads one .templ file into its components.
func parseTempl(root, dir, name string) ([]*component, error) {
	source := filepath.ToSlash(filepath.Join(dir, name))

	body, err := os.ReadFile(filepath.Join(root, dir, name))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", source, err)
	}

	var (
		found   []*component
		marked  bool
		current *component
		block   textbuf.Buffer
	)

	for _, line := range strings.Split(string(body), "\n") {
		if current == nil {
			switch {
			case strings.TrimSpace(line) == pageMarker:
				marked = true
			case templDeclaration.MatchString(line):
				current = &component{
					Name:   templDeclaration.FindStringSubmatch(line)[1],
					File:   source,
					Marked: marked,
					Opaque: strings.Contains(line, "templ.Component"),
				}

				block.Reset()

				marked = false
			case strings.TrimSpace(line) != "" && !strings.HasPrefix(strings.TrimSpace(line), "//"):
				// Any other code ends the comment block a marker sits in.
				marked = false
			}

			continue
		}

		if line == "}" {
			closeComponent(current, block.String())

			found = append(found, current)
			current = nil

			continue
		}

		block.Str(line).Byte('\n')
	}

	if current != nil {
		return nil, fmt.Errorf("%s: templ %s has no closing brace at column 0", source, current.Name)
	}

	return found, nil
}

// closeComponent records what one component's body says.
func closeComponent(c *component, body string) {
	c.Assets = markupAssets(body)
	c.Shell = strings.Contains(body, "<head>")
	c.Asks = anyPageAssets.MatchString(body)

	if named := selfNamedPage.FindStringSubmatch(body); named != nil {
		c.Names = named[1]
	}

	for _, match := range componentCall.FindAllStringSubmatch(body, -1) {
		c.Invokes = append(c.Invokes, match[1])
	}
}

// templComponents reads every .templ file of one surface.
func templComponents(root string, s surface) (map[string]*component, []string, error) {
	entries, err := os.ReadDir(filepath.Join(root, s.Dir))
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", s.Dir, err)
	}

	var (
		components = map[string]*component{}
		order      []string
	)

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".templ" {
			continue
		}

		parsed, err := parseTempl(root, s.Dir, entry.Name())
		if err != nil {
			return nil, nil, err
		}

		for _, c := range parsed {
			if _, clash := components[c.Name]; clash {
				return nil, nil, fmt.Errorf("%s declares templ %s twice", s.Dir, c.Name)
			}

			components[c.Name] = c

			order = append(order, c.Name)
		}
	}

	if len(components) == 0 {
		return nil, nil, fmt.Errorf("%s holds no templ component", s.Dir)
	}

	// A component invoking a name this package does not declare renders content
	// the walk cannot follow: a templ.Component parameter, or templ.Raw of
	// markup another package produced.
	for _, c := range components {
		for _, invoked := range c.Invokes {
			if _, known := components[invoked]; !known {
				c.Opaque = true
			}
		}
	}

	slices.Sort(order)

	return components, order, nil
}

// surfaceUnion returns every asset the sources of one surface name. It is what
// an opaque component contributes, and what an unknown page gets at run time.
func surfaceUnion(root string, s surface) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(root, s.Dir))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", s.Dir, err)
	}

	var found []string

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !readableMarkup(name) {
			continue
		}

		body, err := os.ReadFile(filepath.Join(root, s.Dir, name))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", filepath.ToSlash(filepath.Join(s.Dir, name)), err)
		}

		found = append(found, markupAssets(string(body))...)
	}

	return sorted(found), nil
}

// readableMarkup says whether a file can hold markup ze serves. A generated
// *_templ.go repeats its own .templ, and a test file renders nothing an
// operator receives.
func readableMarkup(name string) bool {
	switch {
	case strings.HasSuffix(name, "_test.go"), strings.HasSuffix(name, "_templ.go"), name == generatedFile:
		return false
	case filepath.Ext(name) == ".templ", filepath.Ext(name) == ".go":
		return true
	default:
		return false
	}
}

// closure returns every asset reachable from one component. A cycle is safe:
// the answer is the union over the reachable set, and visited bounds the walk.
//
// skipOpaque drops the start component's own opacity. A shell that asks for its
// page at render time renders content the handler names, so its content is
// resolved by the page rather than unknowable.
func closure(components map[string]*component, start string, union []string, skipOpaque bool) []string {
	var (
		found   []string
		visited = map[string]bool{}
		queue   = []string{start}
	)

	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]

		if visited[name] {
			continue
		}

		visited[name] = true

		c, known := components[name]
		if !known {
			continue
		}

		found = append(found, c.Assets...)

		if c.Opaque && !(skipOpaque && name == start) {
			found = append(found, union...)
		}

		queue = append(queue, c.Invokes...)
	}

	return sorted(found)
}

// constantFor names the generated Go identifier of one page.
func constantFor(id string) string {
	var tb textbuf.Buffer

	return tb.Str("pg").Str(strings.ToUpper(id[:1])).Str(id[1:]).String()
}

// pageKeyFor names one body page in the JSON output: the shell that renders its
// head, then the body.
func pageKeyFor(shellFile, body string) string {
	var tb textbuf.Buffer

	return tb.Str(shellFile).Byte('#').Str(body).String()
}

// templPages derives the pages of one templ surface.
func templPages(root string, s surface) ([]page, error) {
	components, order, err := templComponents(root, s)
	if err != nil {
		return nil, err
	}

	union, err := surfaceUnion(root, s)
	if err != nil {
		return nil, err
	}

	var pages []page

	for _, name := range order {
		shell := components[name]
		if !shell.Shell {
			continue
		}

		if !shell.Asks {
			return nil, fmt.Errorf("%s: templ %s renders <head> and does not render pageAssets, "+
				"so it still hand-writes its script tags", shell.File, shell.Name)
		}

		own := closure(components, shell.Name, union, shell.Names == "")

		if shell.Names != "" {
			if want := constantFor(shell.Name); shell.Names != want {
				return nil, fmt.Errorf("%s: templ %s names %s in its head block, and its own page is %s",
					shell.File, shell.Name, shell.Names, want)
			}

			pages = append(pages, page{
				Key:      shell.File,
				ID:       shell.Name,
				Constant: constantFor(shell.Name),
				Origin:   shell.File,
				Assets:   own,
			})

			continue
		}

		// The shell asks for its page at render time, so each marked body is a
		// page of its own: the chrome that shell renders, plus that body.
		for _, body := range order {
			if !components[body].Marked {
				continue
			}

			pages = append(pages, page{
				Key:      pageKeyFor(shell.File, body),
				ID:       body,
				Constant: constantFor(body),
				Origin:   shell.File,
				Assets:   sorted(append(own, closure(components, body, union, false)...)),
			})
		}
	}

	if err := checkMarked(components, order, pages); err != nil {
		return nil, err
	}

	return pages, nil
}

// checkMarked refuses a page marker no shell serves. A marked component with no
// page of its own is a page whose import set nothing derives.
func checkMarked(components map[string]*component, order []string, pages []page) error {
	served := map[string]bool{}
	for _, p := range pages {
		served[p.ID] = true
	}

	for _, name := range order {
		if components[name].Marked && !served[name] {
			return fmt.Errorf("%s: templ %s carries %s and no shell of the package asks for a page at "+
				"render time, so nothing renders its assets", components[name].File, name, pageMarker)
		}
	}

	return nil
}

// goPages derives the page of a surface whose markup is Go string literals.
// Chaos renders one shell, so its page carries everything the package renders.
func goPages(root string, s surface) ([]page, error) {
	union, err := surfaceUnion(root, s)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(filepath.Join(root, s.Dir))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", s.Dir, err)
	}

	var pages []page

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || filepath.Ext(name) != ".go" || !readableMarkup(name) {
			continue
		}

		source := filepath.ToSlash(filepath.Join(s.Dir, name))

		body, err := os.ReadFile(filepath.Join(root, s.Dir, name))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", source, err)
		}

		writer, holds := headWriter(string(body))
		if !holds {
			continue
		}

		pages = append(pages, page{
			Key:      source,
			ID:       writer,
			Constant: constantFor(writer),
			Origin:   source,
			Assets:   union,
		})
	}

	return pages, nil
}

// headWriter names the function whose body holds a head block, which is the
// shell of a surface that writes its markup in Go.
func headWriter(body string) (string, bool) {
	var current string

	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "func ") {
			current = goFuncName(line)
		}

		if strings.Contains(line, "<head>") && current != "" {
			return current, true
		}
	}

	return "", false
}

// goFuncName reads the name out of one func declaration line, method or not.
func goFuncName(line string) string {
	rest := strings.TrimPrefix(line, "func ")

	if strings.HasPrefix(rest, "(") {
		if end := strings.Index(rest, ")"); end >= 0 {
			rest = strings.TrimSpace(rest[end+1:])
		}
	}

	if end := strings.IndexAny(rest, "([ "); end >= 0 {
		rest = rest[:end]
	}

	return rest
}

// surfacePages derives the pages of one surface, whichever shape it is.
func surfacePages(root string, s surface) ([]page, error) {
	if s.GoMarkup {
		return goPages(root, s)
	}

	return templPages(root, s)
}

// render writes the generated file of one surface.
func render(s surface, pages []page, union []string) ([]byte, error) {
	var b textbuf.Buffer

	b.Str("// Code generated by scripts/codegen/web_assets.go; DO NOT EDIT.\n\n")
	b.Str("// The vendored assets each page of this package loads, derived from the markup\n")
	b.Str("// every page reaches. To change a set, change the markup. Then: make generate\n\n")
	b.Str("package ").Str(s.Pkg).Str("\n\n")
	b.Str("// pageID names one page whose head block loads an asset set.\n")
	b.Str("type pageID string\n\n")
	b.Str("// The pages of this package, each named after the component that renders it.\n")
	b.Str("const (\n")

	for _, p := range pages {
		b.Str("\t// ").Str(p.Constant).Str(" is rendered by ").Str(p.Origin).Str(".\n")
		b.Str("\t").Str(p.Constant).Str(" pageID = ").Quoted(p.ID).Byte('\n')
	}

	b.Str(")\n\n")
	b.Str("// pageAssetSets maps each page onto the assets its head block loads.\n")
	b.Str("var pageAssetSets = map[pageID][]string{\n")

	for _, p := range pages {
		b.Str("\t").Str(p.Constant).Str(": {").Str(quoteList(served(s, p.Assets))).Str("},\n")
	}

	b.Str("}\n\n")
	b.Str("// everyAsset is every asset this package renders. It is what an unknown page\n")
	b.Str("// gets: a page that loads one file too many costs bytes, and a page that loads\n")
	b.Str("// nothing renders correctly and does nothing in the browser.\n")
	b.Str("var everyAsset = []string{").Str(quoteList(served(s, union))).Str("}\n\n")
	b.Str("// pageAssets returns the assets one page must load.\n")
	b.Str("func pageAssets(page pageID) []string {\n")
	b.Str("\tassets, known := pageAssetSets[page]\n")
	b.Str("\tif !known {\n\t\treturn everyAsset\n\t}\n\n")
	b.Str("\treturn assets\n}\n")

	out, err := format.Source(b.Bytes())
	if err != nil {
		return nil, fmt.Errorf("format the generated file for %s: %w", s.Dir, err)
	}

	return out, nil
}

// served returns the URL a page loads each asset from.
func served(s surface, assets []string) []string {
	var (
		tb    textbuf.Buffer
		paths = make([]string, 0, len(assets))
	)

	for _, name := range assets {
		tb.Reset()
		paths = append(paths, tb.Str(s.Prefix).Str(name).String())
	}

	return paths
}

// quoteList writes one Go string list, without its braces.
func quoteList(in []string) string {
	quoted := make([]string, 0, len(in))
	for _, one := range in {
		quoted = append(quoted, strconv.Quote(one))
	}

	return strings.Join(quoted, ", ")
}

// derive returns every page of every surface, and the bytes each surface's
// generated file owes.
func derive(root string) (map[string][]string, map[string][]byte, error) {
	var (
		sets      = map[string][]string{}
		generated = map[string][]byte{}
	)

	for _, s := range surfaces {
		pages, err := surfacePages(root, s)
		if err != nil {
			return nil, nil, err
		}

		if len(pages) == 0 {
			return nil, nil, fmt.Errorf("%s: no page renders a head block", s.Dir)
		}

		union, err := surfaceUnion(root, s)
		if err != nil {
			return nil, nil, err
		}

		out, err := render(s, pages, union)
		if err != nil {
			return nil, nil, err
		}

		generated[filepath.ToSlash(filepath.Join(s.Dir, generatedFile))] = out

		for _, p := range pages {
			sets[p.Key] = p.Assets
		}
	}

	return sets, generated, nil
}

// run derives the sets and reports them in the mode the caller asked for.
func run(check, asJSON bool) error {
	root, err := repoRoot()
	if err != nil {
		return err
	}

	pages, generated, err := derive(root)
	if err != nil {
		return err
	}

	if len(pages) == 0 {
		return fmt.Errorf("no page carries an asset set, and ze serves pages")
	}

	// Reporting is READ-ONLY. A test asks for the sets while it runs, and a
	// query that rewrote the tree would leave the working tree changed by the
	// act of reading it.
	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")

		if err := enc.Encode(pages); err != nil {
			return fmt.Errorf("write the derived sets: %w", err)
		}

		return nil
	}

	for _, path := range sortedKeys(generated) {
		if err := writeOrCheck(root, path, generated[path], check); err != nil {
			return err
		}
	}

	return nil
}

// sortedKeys returns the map's keys in order, so a report reads the same way on
// every run.
func sortedKeys(in map[string][]byte) []string {
	keys := make([]string, 0, len(in))
	for key := range in {
		keys = append(keys, key)
	}

	slices.Sort(keys)

	return keys
}

// writeOrCheck writes one generated file, or reports that it is stale.
func writeOrCheck(root, path string, want []byte, check bool) error {
	full := filepath.Join(root, filepath.FromSlash(path))

	if !check {
		if err := os.WriteFile(full, want, 0o644); err != nil { //nolint:gosec // generated source
			return fmt.Errorf("write %s: %w", path, err)
		}

		return nil
	}

	got, err := os.ReadFile(full)
	if err != nil {
		return fmt.Errorf("read %s: %w; run: make generate", path, err)
	}

	if !bytes.Equal(got, want) {
		return fmt.Errorf("%s is stale: it disagrees with the markup its package renders; run: make generate", path)
	}

	return nil
}

// repoRoot returns the directory holding go.mod, walking up from the current
// working directory.
func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found above %s", dir)
		}

		dir = parent
	}
}

func main() {
	check := flag.Bool("check", false, "report whether the generated import sets are current, and exit non-zero when they are not")
	asJSON := flag.Bool("json", false, "print the derived per-page asset sets as JSON on stdout")
	flag.Parse()

	if err := run(*check, *asJSON); err != nil {
		fmt.Fprintf(os.Stderr, "web_assets: %v\n", err)
		os.Exit(1)
	}
}
