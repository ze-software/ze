package web

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"html/template"
	"path"
	"strings"
	"testing"

	"github.com/a-h/templ"

	"github.com/ze-software/ze/internal/test/golden"
)

// fidelityTemplatesFS holds the html/template tree this phase replaces. It is
// embedded HERE, not in render.go, because the package under test no longer
// parses a template and AC-4 forbids it doing so. The tree and this file are
// deleted together when the phase closes.
//
//go:embed templates
var fidelityTemplatesFS embed.FS

// fidelityTemplates parses the tree the way NewRenderer used to. One set per
// config template, with the leaf_input partial beside it. One set for the
// fragments, so they can reference each other. One per l2tp page.
func fidelityTemplates(t *testing.T) (map[string]*template.Template, *template.Template, map[string]*template.Template) {
	t.Helper()

	funcMap := template.FuncMap{
		"sub":          func(a, b int) int { return a - b },
		"fieldid":      formFieldID,
		"fieldname":    formFieldName,
		"fieldchecked": formFieldChecked,
		"fieldlabel":   humanizeFieldLabel,
		"t":            Translate,
		"joinpath": func(p []string, upTo int) string {
			if upTo >= len(p) {
				return strings.Join(p, "/")
			}

			return strings.Join(p[:upTo+1], "/")
		},
		"splitopts": splitFieldOptions,
		// fieldFor rendered one leaf editor inside detail.html. The editor was
		// already a templ component when this phase started, so the funcMap
		// entry reaches the same registry the ported markup calls directly.
		"fieldFor": func(f any) template.HTML {
			field, ok := f.(FieldMeta)
			if !ok {
				return ""
			}

			r, err := NewRenderer()
			if err != nil {
				return ""
			}

			return r.renderComponent("field", fieldComponent(field))
		},
	}

	configNames := []string{
		"container.html", "list.html", "flex.html", "freeform.html", "inline_list.html",
		"breadcrumb.html", "commit.html", "notification.html", "command.html", "command_form.html",
	}

	configs := make(map[string]*template.Template, len(configNames))

	for _, name := range configNames {
		parsed, err := template.New(name).Funcs(funcMap).ParseFS(fidelityTemplatesFS,
			"templates/"+name, "templates/leaf_input.html")
		if err != nil {
			t.Fatalf("parse config template %s: %v", name, err)
		}

		configs[name] = parsed
	}

	fragments, err := template.New("fragments").Funcs(funcMap).ParseFS(fidelityTemplatesFS,
		"templates/component/*.html")
	if err != nil {
		t.Fatalf("parse fragment templates: %v", err)
	}

	l2tps := make(map[string]*template.Template, 2)

	for _, name := range []string{"list.html", "detail.html"} {
		parsed, parseErr := template.New(name).Funcs(funcMap).ParseFS(fidelityTemplatesFS,
			"templates/l2tp/"+name)
		if parseErr != nil {
			t.Fatalf("parse l2tp template %s: %v", name, parseErr)
		}

		l2tps[name] = parsed
	}

	return configs, fragments, l2tps
}

// fidelityExecute renders one template through the set that holds it, which is
// how NewRenderer grouped them.
func fidelityExecute(t *testing.T, buf *bytes.Buffer, file, name string, data any) error {
	t.Helper()

	configs, fragments, l2tps := fidelityTemplates(t)

	rel := strings.TrimPrefix(file, "templates/")
	dir := path.Dir(rel)
	base := path.Base(rel)

	switch dir {
	case "component":
		return fragments.ExecuteTemplate(buf, name, data)

	case "l2tp":
		parsed, ok := l2tps[base]
		if !ok {
			return fmt.Errorf("no l2tp template named %s", base)
		}

		return parsed.Execute(buf, data)

	case ".":
		if parsed, ok := configs[base]; ok && name == base {
			return parsed.Execute(buf, data)
		}

		if parsed := configs["container.html"].Lookup(name); parsed != nil {
			return parsed.Execute(buf, data)
		}

		parsed, err := template.New(base).ParseFS(fidelityTemplatesFS, file)
		if err != nil {
			return fmt.Errorf("parse standalone %s: %w", file, err)
		}

		return parsed.Execute(buf, data)
	}

	return fmt.Errorf("no group holds template %q from %s", name, file)
}

// SCAFFOLDING. This file proves phase 3b of plan/spec-web-templ-migration.md
// and is deleted when that phase closes. AC-2 forbids re-baselining a fixture
// before the unit it belongs to has passed here.
//
// Each case renders the SAME data through both engines and compares the
// normalized forms. golden.NormalizeHTML erases whitespace layout and doctype
// case, which is all templ's generator changes about markup.
//
// Two spelling axes it does not cover are rewritten on the html/template side.
// Each is a rewrite into templ's spelling of the same decoded value:
//
//   - html/template's htmlReplacementTable (GOROOT html/template/html.go)
//     escapes +, = and ` as character references. templ.EscapeString is
//     html.EscapeString, whose replacer covers five characters and none of
//     those three. A reference and its character decode alike.
//   - a single-quoted attribute value becomes double-quoted, because
//     writeExpressionAttribute (vendor/github.com/a-h/templ/generator/generator.go)
//     writes double quotes and no other form. A " inside the value is then a
//     character reference, which HTML decodes before anything reads it.
//
// &lt; is deliberately NOT rewritten. AC-5 is about a value escaped exactly
// once, and erasing that spelling would erase the evidence for it.

// fidelityCase is one unit rendered both ways.
type fidelityCase struct {
	// name identifies the case in test output.
	name string
	// file and tmpl locate the html/template side.
	file string
	tmpl string
	// data is what the html/template side renders with.
	data any
	// got is the templ component rendering the same values.
	got templ.Component
}

// TestTemplPortFidelityPhase3B is the AC-2 instrument for this phase.
//
// VALIDATES: every unit ported in phase 3b renders what its html/template
// rendered, for the same data, under golden.NormalizeHTML.
// PREVENTS: a fixture re-baselined onto templ's bytes before anything proved
// those bytes are the same page. A re-baseline is a rewrite of the evidence.
// It MUST follow this test passing.
func TestTemplPortFidelityPhase3B(t *testing.T) {
	for _, c := range fidelityCases() {
		t.Run(c.name, func(t *testing.T) {
			var before bytes.Buffer
			if execErr := fidelityExecute(t, &before, c.file, c.tmpl, c.data); execErr != nil {
				t.Fatalf("html/template render %q from %s: %v", c.tmpl, c.file, execErr)
			}

			if strings.TrimSpace(before.String()) == "" {
				t.Fatalf("html/template rendered nothing for %q; the case proves nothing", c.tmpl)
			}

			var after bytes.Buffer
			if renderErr := c.got.Render(context.Background(), &after); renderErr != nil {
				t.Fatalf("templ render for %q: %v", c.tmpl, renderErr)
			}

			want := templSpelling(golden.NormalizeHTML(before.String()))
			have := golden.NormalizeHTML(after.String())

			if want != have {
				t.Errorf("port is not faithful for %s\n--- html/template\n%s\n--- templ\n%s\n--- first difference at %d",
					c.name, want, have, firstDifference(want, have))
			}
		})
	}
}

// templSpelling rewrites the spellings html/template uses and templ does not.
// See the file comment for why each is a rewrite and not a relaxation.
func templSpelling(src string) string {
	src = strings.NewReplacer(
		"&#43;", "+",
		"&#61;", "=",
		"&#96;", "`",
	).Replace(src)

	return rewriteAttributes(src)
}

// rewriteAttributes rewrites every attribute value into the spelling templ
// writes for the same decoded value. A single-quoted value becomes
// double-quoted with any " inside it encoded, and a bare & becomes &amp;.
//
// It is scoped to the inside of a tag on purpose. A ' inside a double-quoted
// value is content, not a quote: hx-trigger="keyup[key=='Enter']" holds two,
// and a rewriter that reads them as delimiters corrupts the side it is meant to
// leave alone.
func rewriteAttributes(src string) string {
	var b strings.Builder

	inTag := false

	for i := 0; i < len(src); i++ {
		c := src[i]

		switch {
		case !inTag && c == '<':
			inTag = true
		case inTag && c == '>':
			inTag = false
		case inTag && (c == '"' || c == '\''):
			value, next := attributeValue(src, i)
			b.WriteByte('"')
			b.WriteString(value)
			b.WriteByte('"')

			i = next

			continue
		}

		b.WriteByte(c)
	}

	return b.String()
}

// attributeValue reads the quoted value starting at i and returns it in templ's
// spelling, with the index of its closing quote.
func attributeValue(src string, i int) (string, int) {
	quote := src[i]

	end := strings.IndexByte(src[i+1:], quote)
	if end < 0 {
		return src[i+1:], len(src) - 1
	}

	value := src[i+1 : i+1+end]

	if quote == '\'' {
		value = strings.ReplaceAll(value, `"`, "&#34;")
	}

	return encodeBareAmpersand(value), i + end + 1
}

// encodeBareAmpersand writes an & that starts no character reference as &amp;,
// which is what templ writes for the same character. An existing reference is
// left alone, so a double escape still reads as a difference.
func encodeBareAmpersand(value string) string {
	var b strings.Builder

	for i := range len(value) {
		if value[i] != '&' {
			b.WriteByte(value[i])

			continue
		}

		if startsReference(value[i:]) {
			b.WriteByte('&')

			continue
		}

		b.WriteString("&amp;")
	}

	return b.String()
}

// startsReference reports whether s begins a character reference, which is an
// & then a name or a number, then a semicolon.
func startsReference(s string) bool {
	end := strings.IndexByte(s, ';')
	if end < 1 || end > referenceMaxLen {
		return false
	}

	for _, r := range s[1:end] {
		named := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
		if !named && r != '#' {
			return false
		}
	}

	return true
}

// referenceMaxLen bounds the search for a reference's semicolon. The longest
// named reference in HTML is 32 characters.
const referenceMaxLen = 34

// firstDifference returns the byte offset where two strings first differ, so a
// failure over a long page names the place rather than the page.
func firstDifference(a, b string) int {
	n := min(len(a), len(b))

	for i := range n {
		if a[i] != b[i] {
			return i
		}
	}

	return n
}
