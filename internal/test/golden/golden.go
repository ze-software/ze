// Design: (none -- test utility, no architecture doc)

// Package golden compares captured bytes against fixtures committed beside the
// package under test.
//
// Three kinds of capture use it, and each answers a question the others cannot.
// The TEMPLATE capture executes one parsed template against test-authored data.
// The HANDLER capture issues an HTTP request and records the response an
// operator receives. It therefore covers the view model the handler builds,
// and the wrappers it renders through. The MARKUP capture renders a Go HTML builder
// over fixed input, because the handler capture must normalize whatever the
// machine decides. No one of the three proves another.
//
// Two packages capture this way, internal/component/web and
// internal/component/lg. Each keeps its own spec, its own fixture data and its
// own execute or serve function. Five things are shared, and they live here.
//
//   - the walk over the template FS.
//   - the check that the spec and the FS agree.
//   - the check that a capture covers a live name set, both directions.
//   - the fixture path rule.
//   - the byte comparison.
package golden

import (
	"bytes"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// update rewrites the captured fixtures instead of comparing against them. It
// is inert in a normal run. Recapture a deliberate markup change by running the
// owning package's golden test with `-update-golden`.
var update = flag.Bool("update-golden", false,
	"rewrite the golden fixtures in testdata/golden from the current templates")

// Updating reports whether -update-golden was given. A capture run writes every
// fixture, so the checks that judge a comparison do not apply to it.
func Updating() bool { return *update }

// defineRe matches a {{define "name"}} action. The pattern is an interpreted
// string, not a raw one. A raw one puts a `+` directly before a quote, and the
// repository's write hook reads that as string concatenation.
var defineRe = regexp.MustCompile("\\{\\{-?\\s*define\\s+\"([^\"]+)\"")

// actionRe matches one {{ ... }} action, trim markers included.
var actionRe = regexp.MustCompile(`(?s)\{\{.*?\}\}`)

// templRe matches a templ component declaration. templ requires the keyword at
// the start of a line, so an anchored pattern reads exactly what the generator
// reads.
var templRe = regexp.MustCompile(`(?m)^templ\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`)

// templExt is the templ source suffix. A file carrying it declares its
// renderable units with the templ keyword rather than with {{define}}.
const templExt = ".templ"

// Variant is one data set a template renders with. A template with a branch
// gets one variant per branch, so the captured bytes cover the branch body and
// not only the markup around it.
type Variant struct {
	// Name is the fixture suffix. Empty gives the fixture no suffix.
	Name string
	Data any
}

// Unit is one renderable template inside a template file.
type Unit struct {
	// Name is the template name, as the renderer resolves it. For a templ
	// source that is the component's Go identifier.
	Name string
	// Fixture overrides the fixture stem. It is empty for a template whose
	// fixture is named after it. Set it where a rename would move fixtures
	// that nothing else about the change moves. A templ component named
	// peersTableBody keeps the fixture peers_table_body, captured from the
	// html/template it replaced.
	Fixture  string
	Variants []Variant
}

// stem is the fixture name a unit's variants hang off.
func (u Unit) stem() string {
	if u.Fixture != "" {
		return u.Fixture
	}

	return u.Name
}

// FixtureName joins a unit and one of its variants into the name the fixture
// carries. One rule, so the coverage check and the render loop cannot disagree
// about which fixture a variant writes.
func (u Unit) FixtureName(v Variant) string {
	if v.Name == "" {
		return u.stem()
	}

	var tb textbuf.Buffer

	return tb.Str(u.stem()).Str("--").Str(v.Name).String()
}

// Spec maps each file in a template FS to the templates it defines and the data
// each one renders with.
type Spec map[string][]Unit

// Set is one package's template tree and the spec that must cover it.
type Set struct {
	// FS holds the templates, usually the package's embedded FS.
	FS fs.FS
	// Dir is the directory inside FS that holds the templates.
	Dir string
	// Ext keeps the walk to files carrying one suffix. It is empty for an FS
	// that holds nothing but templates. Set it for a templ source tree. That
	// tree lives beside the package's Go files, not in a directory of its own.
	Ext string
	// Spec is the capture plan for that tree.
	Spec Spec
	// SpecVar names the Go variable a failure tells the reader to edit.
	SpecVar string
}

// Files walks the template tree and returns every file it holds, sorted. The
// walk is the authority on which templates exist, so a template added later is
// covered without an edit to the spec.
func (s Set) Files(t *testing.T) []string {
	t.Helper()

	var files []string

	err := fs.WalkDir(s.FS, s.Dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// A directory named testdata holds fixtures, and the go tool excludes it
		// from every walk of its own. A template in there belongs to a test, not
		// to the package's rendered surface, and the captured fixtures live
		// under it too.
		if d.IsDir() {
			if d.Name() == "testdata" {
				return fs.SkipDir
			}

			return nil
		}

		if s.Ext == "" || strings.HasSuffix(p, s.Ext) {
			files = append(files, p)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", s.Dir, err)
	}

	sort.Strings(files)

	if len(files) == 0 {
		t.Fatalf("walk %s found no files", s.Dir)
	}

	return files
}

// AssertCoversFS fails when the spec and the FS disagree, naming every template
// on either side of the difference. A template the harness cannot render must
// fail here rather than be left out of the capture. A capture that covers part
// of the set reports green and proves nothing about the rest.
func (s Set) AssertCoversFS(t *testing.T, files []string) {
	t.Helper()

	for _, finding := range s.coverage(files) {
		t.Error(finding)
	}
}

// coverage returns one finding per disagreement between the spec and the FS, in
// a fixed order. It reads the FS and reports. It holds no *testing.T, which is
// what lets this package's own tests drive it over a fixture FS.
func (s Set) coverage(files []string) []error {
	var findings []error

	seen := make(map[string]bool, len(files))
	fixtures := make(map[string]string)

	for _, file := range files {
		seen[file] = true

		units, ok := s.Spec[file]
		if !ok {
			findings = append(findings, fmt.Errorf(
				"template file %s has no golden coverage; add its templates to %s", file, s.SpecVar))

			continue
		}

		want, err := s.definedNames(file)
		if err != nil {
			findings = append(findings, err)

			continue
		}

		got := make(map[string]bool, len(units))

		for _, u := range units {
			if got[u.Name] {
				findings = append(findings, fmt.Errorf("template %q listed twice for %s", u.Name, file))

				continue
			}

			got[u.Name] = true

			if len(u.Variants) == 0 {
				findings = append(findings, fmt.Errorf("template %q in %s has no data variant", u.Name, file))
			}

			for _, v := range u.Variants {
				fixture := u.FixtureName(v)

				// The clash is keyed on the fixture NAME, never on its path.
				// The path carries the file's directory, so two same-named
				// templates in two directories reach two fixtures and no clash
				// fires. A parsed set holds one template per name, so one of
				// the two is never rendered and its fixture records the bytes
				// of the other.
				if prev, clash := fixtures[fixture]; clash {
					findings = append(findings, fmt.Errorf(
						"fixture %q is captured for both %s and %s; a parsed set holds one template per name, so the later parse wins and one of the two is never rendered",
						fixture, prev, file))
				}

				fixtures[fixture] = file
			}
		}

		for _, name := range sortedKeys(want) {
			if !got[name] {
				findings = append(findings, fmt.Errorf("template %q defined in %s is not captured", name, file))
			}
		}

		for _, name := range sortedKeys(got) {
			if !want[name] {
				findings = append(findings, fmt.Errorf("template %q captured for %s is not defined there", name, file))
			}
		}
	}

	for _, file := range sortedKeys(s.Spec) {
		if !seen[file] {
			findings = append(findings, fmt.Errorf(
				"%s names %s, which the embedded FS does not hold", s.SpecVar, file))
		}
	}

	return findings
}

// definedNames returns the template names a file contributes. A file with
// {{define}} actions contributes those names. A file that renders a body of its
// own contributes the template named after the file, which is how ParseFS names
// that body. A file can do both.
func (s Set) definedNames(file string) (map[string]bool, error) {
	data, err := fs.ReadFile(s.FS, file)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", file, err)
	}

	src := string(data)

	names := make(map[string]bool)

	if strings.HasSuffix(file, templExt) {
		// templ compiles each component into a Go function, so the file
		// renders no body of its own and the {{define}} rules below do not
		// apply.
		for _, m := range templRe.FindAllStringSubmatch(src, -1) {
			names[m[1]] = true
		}

		return names, nil
	}

	for _, m := range defineRe.FindAllStringSubmatch(src, -1) {
		names[m[1]] = true
	}

	if len(names) == 0 || bodyOutsideDefines(src) {
		names[path.Base(file)] = true
	}

	return names, nil
}

// bodyOutsideDefines reports whether a file renders a body of its own, outside
// every {{define}} block.
//
// The rule is conservative: an action at the top level counts as a body, and
// only a comment renders nothing. A file whose top level holds one assignment
// is therefore asked for a fixture it does not need. That direction keeps
// markup covered. The other direction loses it.
func bodyOutsideDefines(src string) bool {
	depth := 0
	end := 0

	for _, span := range actionRe.FindAllStringIndex(src, -1) {
		if depth == 0 && strings.TrimSpace(src[end:span[0]]) != "" {
			return true
		}

		end = span[1]

		switch actionKeyword(src[span[0]:span[1]]) {
		case "define":
			depth++
		case "if", "range", "with", "block":
			if depth == 0 {
				return true
			}

			depth++
		case "end":
			if depth == 0 {
				// Unbalanced. Read it as a body rather than swallow the rest of
				// the file into a block that never opened.
				return true
			}

			depth--
		case "comment":
		default:
			if depth == 0 {
				return true
			}
		}
	}

	return depth == 0 && strings.TrimSpace(src[end:]) != ""
}

// actionKeyword returns the leading word of one {{ ... }} action, "comment" for
// {{/* ... */}}, and an empty string for an action that starts with anything
// else.
func actionKeyword(action string) string {
	body := strings.TrimLeft(strings.TrimPrefix(strings.TrimPrefix(action, "{{"), "-"), " \t\r\n")

	if strings.HasPrefix(body, "/*") {
		return "comment"
	}

	end := strings.IndexFunc(body, func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < 'A' || r > 'Z')
	})
	if end < 0 {
		end = len(body)
	}

	return body[:end]
}

// FixturePath maps a template file and a fixture name onto the fixture's path.
// The fixture tree mirrors the template tree below Dir.
func (s Set) FixturePath(root, file, fixture string) string {
	rel := strings.TrimPrefix(strings.TrimPrefix(file, s.Dir), "/")

	dir := path.Dir(rel)
	if dir == "." {
		dir = ""
	}

	var tb textbuf.Buffer

	return filepath.Join(root, dir, tb.Str(strings.TrimSuffix(fixture, ".html")).Str(".html").String())
}

// Compare diffs rendered bytes against the fixture, or rewrites the fixture
// when -update-golden is set.
func Compare(t *testing.T, path string, got []byte) {
	t.Helper()

	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("create %s: %v", filepath.Dir(path), err)
		}

		if err := os.WriteFile(path, got, 0o600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}

		return
	}

	want, err := os.ReadFile(path) //nolint:gosec // path is built from the embedded template tree
	if err != nil {
		t.Fatalf("read fixture %s; capture it with -update-golden: %v", path, err)
	}

	if bytes.Equal(got, want) {
		return
	}

	t.Errorf("rendered bytes differ from %s\n%s", path, diff(want, got))
}

// diff reports the first line that differs, so a failure names the change
// instead of printing two whole pages.
func diff(want, got []byte) string {
	wantLines := strings.Split(string(want), "\n")
	gotLines := strings.Split(string(got), "\n")

	var b strings.Builder

	for i := 0; i < len(wantLines) || i < len(gotLines); i++ {
		var w, g string
		if i < len(wantLines) {
			w = wantLines[i]
		}

		if i < len(gotLines) {
			g = gotLines[i]
		}

		if w == g {
			continue
		}

		b.WriteString("first difference at line ")
		b.WriteString(strconv.Itoa(i + 1))
		b.WriteString("\n  fixture: ")
		b.WriteString(w)
		b.WriteString("\n  render:  ")
		b.WriteString(g)
		b.WriteString("\n")

		break
	}

	b.WriteString("length: fixture ")
	b.WriteString(strconv.Itoa(len(want)))
	b.WriteString(", render ")
	b.WriteString(strconv.Itoa(len(got)))

	return b.String()
}

// sortedKeys returns a map's keys in a fixed order, so a run reports the same
// findings in the same sequence.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	return keys
}
