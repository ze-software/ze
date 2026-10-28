package config

import (
	"strings"
	"testing"
)

// quoteRoundTripValues returns the generated set of values the quoting tests
// write and read back: every byte from 0x00 to 0xff alone, at the start, in
// the middle and at the end of a word, plus the values that motivated the fix
// and the spellings that must stay bare.
func quoteRoundTripValues() []string {
	values := []string{
		"", "x]y(z", "a[b", "l1\nl2", "a\rb", "(open", ")close", "a(b)c",
		"redirect:[2001:db8::1]:100", "x:[", "x:[]", "x:[a b]", `"`, `\`, `a\"b`,
		`a\nb`, "#x", "x#", "'", " lead", "trail ", "inactive:", "é", "\xff\xfe",
		"[", "]", "{", "}", ";", "a;b", "65001", "192.0.2.1", "2001:db8::1",
		"a\v", "\fa", "a ", "\u0085a", "a ", // Spaces only strings.TrimSpace knows.
	}
	for c := range 256 {
		b := string([]byte{byte(c)})
		values = append(values, b, b+"a", "a"+b, "a"+b+"a")
	}
	return values
}

// TestQuoteIfNeededRoundTrip writes each generated value with QuoteIfNeeded
// and reads it back with the hierarchical config tokenizer.
//
// VALIDATES: AC-9 at the token level: the tokenizer reads the written text as
// exactly one word or string token equal to the value, followed by nothing.
// A value that needs no quote stays bare, so no output changes for it.
// PREVENTS: a value holding `[`, `]`, a newline, a carriage return, or a
// leading `(` or `)` written bare and read back as several tokens.
func TestQuoteIfNeededRoundTrip(t *testing.T) {
	for _, value := range quoteRoundTripValues() {
		written := QuoteIfNeeded(value)
		if err := readBackOneToken(written, value); err != "" {
			t.Errorf("QuoteIfNeeded(%q) = %q: %s", value, written, err)
		}
	}

	for _, bare := range []string{"65001", "192.0.2.1", "2001:db8::1", "a(b)c", "redirect:[2001:db8::1]:100", "inactive:"} {
		if got := QuoteIfNeeded(bare); got != bare {
			t.Errorf("QuoteIfNeeded(%q) = %q, want it bare", bare, got)
		}
	}
}

// readBackOneToken tokenizes written and answers why it is not the single
// token value, or "" when it is.
func readBackOneToken(written, value string) string {
	tok := newTokenizer(written)
	first := tok.next()
	if first.kind != tokenWord && first.kind != tokenString {
		return "first token is " + first.kind.String()
	}
	if first.value != value {
		return "read back as " + strings.ReplaceAll(first.value, "\n", `\n`)
	}
	rest := tok.next()
	if rest.kind == tokenSemicolon {
		rest = tok.next() // The synthetic semicolon a value at end of input arms.
	}
	if rest.kind != tokenEOF {
		return "a second token follows: " + rest.kind.String()
	}
	return ""
}

// TestQuoteRoundTripHierarchical writes each generated value as a leaf value
// and as a string list key through Serialize, parses the text back, and
// formats it again.
//
// VALIDATES: AC-9 and the meaning and idempotence halves of AC-10 through the
// hierarchical printer and parser: the value read back is the value stored,
// and a second Serialize prints the same bytes.
// PREVENTS: fmt writing a file it then refuses, or one that reads back a
// different value.
func TestQuoteRoundTripHierarchical(t *testing.T) {
	schema := testSchema()
	for _, value := range quoteRoundTripValues() {
		tree := NewTree()
		neighbor := NewTree()
		neighbor.Set("description", value)
		tree.AddListEntry("neighbor", "192.0.2.1", neighbor)
		if value != "" {
			process := NewTree()
			process.Set("run", value)
			tree.AddListEntry("process", value, process)
		}

		written := Serialize(tree, schema)
		parsed, err := NewParser(schema).Parse(written)
		if err != nil {
			t.Errorf("value %q: written text does not parse: %v\n%s", value, err, written)
			continue
		}
		if got, _ := parsed.GetList("neighbor")["192.0.2.1"].Get("description"); got != value {
			t.Errorf("value %q: description read back as %q\n%s", value, got, written)
		}
		if value != "" {
			process, ok := parsed.GetList("process")[value]
			if !ok {
				t.Errorf("value %q: process key not read back\n%s", value, written)
				continue
			}
			if got, _ := process.Get("run"); got != value {
				t.Errorf("value %q: run read back as %q\n%s", value, got, written)
			}
		}
		if again := Serialize(parsed, schema); again != written {
			t.Errorf("value %q: second Serialize differs:\n%s\nvs\n%s", value, written, again)
		}
	}
}

// TestQuoteRoundTripSet writes each generated value as a leaf value through
// SerializeSet and SerializeSetWithMeta and reads it back with SetParser.
//
// VALIDATES: A-4: the set printers share QuoteIfNeeded, and the set parser
// reads each written value back as the value stored, escapes included, in
// the set form and in the set-meta form the editor stores.
// PREVENTS: a value that the editor's commit writes and the next load reads
// as a different value, or as a line the parser splits.
func TestQuoteRoundTripSet(t *testing.T) {
	schema := testSchema()
	for _, value := range quoteRoundTripValues() {
		tree := NewTree()
		neighbor := NewTree()
		neighbor.Set("description", value)
		tree.AddListEntry("neighbor", "192.0.2.1", neighbor)

		for _, form := range []struct {
			name    string
			written string
			parse   func(*SetParser, string) (*Tree, error)
		}{
			{"set", SerializeSet(tree, schema), (*SetParser).Parse},
			{"set-meta", SerializeSetWithMeta(tree, NewMetaTree(), schema), func(p *SetParser, s string) (*Tree, error) {
				parsed, _, err := p.ParseWithMeta(s)
				return parsed, err
			}},
		} {
			parsed, err := form.parse(NewSetParser(schema), form.written)
			if err != nil {
				t.Errorf("%s value %q: written text does not parse: %v\n%s", form.name, value, err, form.written)
				continue
			}
			entry, ok := parsed.GetList("neighbor")["192.0.2.1"]
			if !ok {
				t.Errorf("%s value %q: neighbor not read back\n%s", form.name, value, form.written)
				continue
			}
			if got, _ := entry.Get("description"); got != value {
				t.Errorf("%s value %q: description read back as %q\n%s", form.name, value, got, form.written)
			}
		}
	}
}
