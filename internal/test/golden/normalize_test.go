package golden

import "testing"

// TestNormalizeHTMLErasesLayoutOnly proves NormalizeHTML is not vacuous.
//
// VALIDATES: NormalizeHTML erases whitespace layout and doctype case, and
// nothing else. Every content difference a port can introduce survives it.
// PREVENTS: a normalizer that reports every port faithful because it erased the
// content the port was supposed to preserve. This is the instrument AC-2 of
// plan/spec-web-templ-migration.md rests on, so a vacuous one makes the whole
// comparison vacuous.
func TestNormalizeHTMLErasesLayoutOnly(t *testing.T) {
	same := []struct {
		name string
		a    string
		b    string
	}{
		{"doctype case", "<!DOCTYPE html>\n<html lang=\"en\">", "<!doctype html><html lang=\"en\">"},
		{"newline between blocks", "</div>\n<div>", "</div><div>"},
		{"newline against a space", "<td>a</td>\n<td>b</td>", "<td>a</td> <td>b</td>"},
		{"newline inside a tag", "<button a=\"1\"\n  b=\"2\">x</button>", "<button a=\"1\" b=\"2\">x</button>"},
		{"padding at the edges of a cell", "<td>\n  AS Path\n</td>", "<td>AS Path</td>"},
		{"a run of spaces between words", "<td>a   b</td>", "<td>a b</td>"},
		{"newline between two inline tags", "</a>\n<a>", "</a> <a>"},
	}

	for _, tt := range same {
		t.Run("same/"+tt.name, func(t *testing.T) {
			if got, want := NormalizeHTML(tt.a), NormalizeHTML(tt.b); got != want {
				t.Errorf("normalized forms differ\n  a: %q\n  b: %q", got, want)
			}
		})
	}

	differ := []struct {
		name string
		a    string
		b    string
	}{
		{"attribute value", `<a href="/lg/peers">P</a>`, `<a href="/lg/peer">P</a>`},
		{"text node", "<td>established</td>", "<td>idle</td>"},
		{"element name", "<h1>Peers</h1>", "<h2>Peers</h2>"},
		{"attribute dropped", `<tr class="best-route" id="x">`, `<tr id="x">`},
		{"whitespace inside a quoted value", `<a class="a  b">P</a>`, `<a class="a b">P</a>`},
		{"space between an expression and an inline span", "<span>AS 64500 <span>N</span></span>", "<span>AS 64500<span>N</span></span>"},
		{"whole element dropped", "<td>a</td><td>b</td>", "<td>a</td>"},
		{"an entity against a raw ampersand", "<td>a &amp; b</td>", "<td>a & b</td>"},
		{"self-closing against paired", "<td><br/></td>", "<td><br></br></td>"},
	}

	for _, tt := range differ {
		t.Run("differ/"+tt.name, func(t *testing.T) {
			if got, want := NormalizeHTML(tt.a), NormalizeHTML(tt.b); got == want {
				t.Errorf("NormalizeHTML erased a real difference; both became %q", got)
			}
		})
	}
}

// TestNormalizeHTMLKeepsWhitespaceInsidePre pins the one place where a newline
// and a space are different content.
//
// VALIDATES: a newline against a space survives normalization inside <pre> and
// inside <textarea>, and is still erased everywhere else. The rules outside the
// element are unchanged, and they resume after the close tag.
// PREVENTS: the port of internal/component/web/templates/commit.html reading as
// faithful after it lost the newlines that break the config diff into lines.
// templ drops that newline unless the port writes it as { "\n" }, and a
// collapsing normalizer cannot see the loss.
func TestNormalizeHTMLKeepsWhitespaceInsidePre(t *testing.T) {
	differ := []struct {
		name string
		a    string
		b    string
	}{
		{
			"a diff line break inside pre",
			"<pre class=\"d\"><span>a</span>\n<span>b</span></pre>",
			"<pre class=\"d\"><span>a</span> <span>b</span></pre>",
		},
		{
			"a newline lost altogether inside pre",
			"<pre><span>a</span>\n<span>b</span></pre>",
			"<pre><span>a</span><span>b</span></pre>",
		},
		{"indentation inside pre", "<pre>  a</pre>", "<pre>a</pre>"},
		{"a run of spaces inside pre", "<pre>a   b</pre>", "<pre>a b</pre>"},
		{"a newline inside a textarea", "<textarea>a\nb</textarea>", "<textarea>a b</textarea>"},
		{
			"a newline inside a span inside pre",
			"<pre><span>a\nb</span></pre>",
			"<pre><span>a b</span></pre>",
		},
	}

	for _, tt := range differ {
		t.Run("differ/"+tt.name, func(t *testing.T) {
			if got, want := NormalizeHTML(tt.a), NormalizeHTML(tt.b); got == want {
				t.Errorf("NormalizeHTML erased a difference inside a preserved element; both became %q", got)
			}
		})
	}

	same := []struct {
		name string
		a    string
		b    string
	}{
		{"the same difference outside pre", "<span>a</span>\n<span>b</span>", "<span>a</span> <span>b</span>"},
		{"a newline before pre", "</div>\n<pre>a</pre>", "</div><pre>a</pre>"},
		{"a newline after the close tag", "<pre>a</pre>\n<div>", "<pre>a</pre><div>"},
		{"the rules resume after the close tag", "<pre>a</pre><span>b</span>\n<span>c</span>", "<pre>a</pre><span>b</span> <span>c</span>"},
	}

	for _, tt := range same {
		t.Run("same/"+tt.name, func(t *testing.T) {
			if got, want := NormalizeHTML(tt.a), NormalizeHTML(tt.b); got != want {
				t.Errorf("normalized forms differ\n  a: %q\n  b: %q", got, want)
			}
		})
	}
}

// TestNormalizeHTMLKeepsAGreaterThanInAValue pins the tag scanner.
//
// VALIDATES: a '>' inside a quoted attribute value does not end the tag, so the
// attribute stays one token and its content is compared.
// PREVENTS: a naive scanner splitting the tag there and turning the rest of the
// attribute into text, which the whitespace rules would then rewrite.
func TestNormalizeHTMLKeepsAGreaterThanInAValue(t *testing.T) {
	withGT := `<div title="a > b">x</div>`

	if got := NormalizeHTML(withGT); got != withGT {
		t.Errorf("NormalizeHTML rewrote a quoted '>': got %q, want %q", got, withGT)
	}

	if NormalizeHTML(withGT) == NormalizeHTML(`<div title="a > c">x</div>`) {
		t.Error("NormalizeHTML erased a difference inside a quoted attribute value")
	}
}
