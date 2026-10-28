package config

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// fuzzSchema returns a schema for fuzz testing.
// Uses a representative set of node types to exercise all parser paths.
func fuzzSchema() *Schema {
	schema := NewSchema()

	schema.Define("router-id", Leaf(TypeIPv4))
	schema.Define("local-as", Leaf(TypeUint32))

	schema.Define("neighbor", List(TypeIP,
		Field("description", Leaf(TypeString)),
		Field("peer-as", Leaf(TypeUint32)),
		Field("receive-hold-time", LeafWithDefault(TypeUint16, "90")),
		Field("family", Container(
			Field("ipv4", Container(
				Field("unicast", Leaf(TypeBool)),
			)),
		)),
	))

	schema.Define("process", List(TypeString,
		Field("run", Leaf(TypeString)),
	))

	return schema
}

// FuzzConfigParser tests config parser robustness.
//
// VALIDATES: Parser handles arbitrary config strings without crashing.
// PREVENTS: Panic on malformed YANG input, unclosed braces, unexpected tokens.
// SECURITY: Config strings come from user-supplied config files.
func FuzzConfigParser(f *testing.F) {
	schema := fuzzSchema()

	f.Add("router-id 1.2.3.4;")
	f.Add("local-as 65000;")
	f.Add("neighbor 10.0.0.1 { peer-as 65001; }")
	f.Add("neighbor 10.0.0.1 { description \"test peer\"; peer-as 65001; receive-hold-time 180; }")
	f.Add("process foo { run \"/usr/bin/foo\"; }")
	f.Add("")                              // Empty
	f.Add("{}")                            // Braces only
	f.Add("{ { { } } }")                   // Nested braces
	f.Add("unknown-keyword;")              // Unknown keyword
	f.Add("neighbor;")                     // Missing list key
	f.Add("neighbor 10.0.0.1 {")           // Unclosed brace
	f.Add("}")                             // Unmatched close
	f.Add("router-id;")                    // Missing value
	f.Add("router-id 1.2.3.4 1.2.3.5;")    // Extra value
	f.Add("\x00\x01\x02")                  // Binary junk
	f.Add("# comment\nrouter-id 1.2.3.4;") // Comment (if supported)
	f.Add("neighbor 10.0.0.1 { family { ipv4 { unicast true; } } }")

	f.Fuzz(func(t *testing.T, input string) {
		p := NewParser(schema)
		//nolint:errcheck // fuzz: testing panic safety, not error values
		p.Parse(input)
	})
}

// FuzzTokenizer tests config tokenizer robustness.
//
// VALIDATES: tokenizer handles arbitrary strings without crashing.
// PREVENTS: Panic on unterminated strings, binary content, edge-case delimiters.
// SECURITY: tokenizer processes user-supplied config files.
func FuzzTokenizer(f *testing.F) {
	f.Add("word1 word2 word3")
	f.Add("\"quoted string\"")
	f.Add("{ } [ ] ( ) ;")
	f.Add("")
	f.Add("\"unterminated")
	f.Add("'single'")
	f.Add("\t\n  spaces  \r\n")
	f.Add("\x00\xFF")
	f.Add("a\"b\"c")
	f.Add(";;; {{{")

	f.Fuzz(func(t *testing.T, input string) {
		tok := newTokenizer(input)
		// Exhaust all tokens — MUST NOT panic or infinite loop
		for {
			token := tok.next()
			if token.kind == tokenEOF {
				break
			}
		}
	})
}

// FuzzQuoteRoundTrip writes any value with quoteIfNeeded and reads it back
// with the hierarchical tokenizer and with the set-line splitter.
//
// VALIDATES: AC-9 for any value string: both readers return exactly the
// value, as one token.
// PREVENTS: a byte sequence the printer writes bare, or escapes, in a way a
// reader takes apart or decodes to another value.
func FuzzQuoteRoundTrip(f *testing.F) {
	for _, value := range quoteRoundTripValues() {
		f.Add(value)
	}

	f.Fuzz(func(t *testing.T, value string) {
		written := quoteIfNeeded(value)
		if reason := readBackOneToken(written, value); reason != "" {
			t.Fatalf("hierarchical: quoteIfNeeded(%q) = %q: %s", value, written, reason)
		}
		tokens, err := splitQuotedTokens(written)
		if err != nil {
			t.Fatalf("set: quoteIfNeeded(%q) = %q: %v", value, written, err)
		}
		if len(tokens) != 1 || tokens[0] != value {
			t.Fatalf("set: quoteIfNeeded(%q) = %q read back as %q", value, written, tokens)
		}
	})
}

// FuzzFormatRoundTrip formats any input that parses, under the full YANG
// schema, the way `ze config fmt` does.
//
// VALIDATES: AC-10 for any input: the formatted text parses to the same map,
// formatting it again changes no byte, and the comments of the two trees are
// the same multiset of texts.
// PREVENTS: a comment the printer drops or duplicates, a comment text that
// breaks out of its line (a `\r`, a very long line, binary bytes), and an
// output a second fmt rewrites.
// Method: seeded with every `.conf` under test/ that carries a comment, plus
// hand-written shapes for each comment slot.
func FuzzFormatRoundTrip(f *testing.F) {
	schema, err := YANGSchema()
	if err != nil {
		f.Fatalf("YANG schema: %v", err)
	}
	f.Add("# ze-schema: 26.10.07\n# head\n\nbgp { # open\n\t# lead\n\trouter-id 192.0.2.1 # trail\n\t# closing\n} # after\n# end\n")
	f.Add("bgp {\n\trouter-id 192.0.2.1 # one\n\trouter-id 192.0.2.2 # two\n}\n")
	f.Add("# \r\x00\xff\nbgp { router-id 192.0.2.1 }\n")
	for _, seed := range commentedCorpusSeeds(f) {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		source, err := NewParser(schema).Parse(input)
		if err != nil {
			return // Only an input that parses is formatted.
		}
		formatted := Serialize(source, schema)
		reparsed, err := NewParser(schema).Parse(formatted)
		if err != nil {
			t.Fatalf("formatted text does not parse: %v\n%s", err, formatted)
		}
		if !reflect.DeepEqual(source.ToMap(), reparsed.ToMap()) {
			t.Fatalf("formatted text lowers to another map\n%s", formatted)
		}
		if again := Serialize(reparsed, schema); again != formatted {
			t.Fatalf("second format changes the output:\n%s\nvs\n%s", formatted, again)
		}
		sourceComments, reparsedComments := treeCommentTexts(source), treeCommentTexts(reparsed)
		if !slices.Equal(sourceComments, reparsedComments) {
			t.Fatalf("comments differ: source %q, formatted %q\n%s", sourceComments, reparsedComments, formatted)
		}
	})
}

// commentedCorpusSeeds answers the text of every `.conf` under test/ that
// carries a `#`, test/draft skipped as every repo-wide gate does.
func commentedCorpusSeeds(f *testing.F) []string {
	f.Helper()
	var seeds []string
	root := filepath.Join("..", "..", "..", "test")
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && entry.Name() == "draft" {
			return filepath.SkipDir
		}
		if entry.IsDir() || filepath.Ext(path) != ".conf" {
			return nil
		}
		body, err := os.ReadFile(path) //nolint:gosec // path comes from walking the repository's own test tree
		if err != nil {
			return err
		}
		if strings.Contains(string(body), "#") {
			seeds = append(seeds, string(body))
		}
		return nil
	})
	if err != nil {
		f.Fatalf("walk %s: %v", root, err)
	}
	if len(seeds) == 0 {
		f.Fatalf("no commented .conf under %s: the seed walk found nothing", root)
	}
	return seeds
}

// treeCommentTexts answers every comment text a tree carries, at every depth,
// sorted: the multiset AC-10 compares. The recursion is bounded by the depth
// of the parsed text, which the schema bounds.
func treeCommentTexts(tree *Tree) []string {
	tree.mu.RLock()
	defer tree.mu.RUnlock()
	c := tree.comments
	texts := slices.Concat(c.Statement.Leading, c.Statement.Trailing, c.Opening, c.Head, c.Closing)
	for _, leaf := range tree.leafComments {
		texts = append(texts, leaf.Leading...)
		texts = append(texts, leaf.Trailing...)
	}
	for _, child := range tree.containers {
		texts = append(texts, treeCommentTexts(child)...)
	}
	for _, entries := range tree.lists {
		for _, entry := range entries {
			texts = append(texts, treeCommentTexts(entry)...)
		}
	}
	slices.Sort(texts)
	return texts
}
