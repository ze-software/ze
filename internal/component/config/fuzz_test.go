package config

import (
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
		Field("hold-time", LeafWithDefault(TypeUint16, "90")),
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
	f.Add("neighbor 10.0.0.1 { description \"test peer\"; peer-as 65001; hold-time 180; }")
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
// VALIDATES: Tokenizer handles arbitrary strings without crashing.
// PREVENTS: Panic on unterminated strings, binary content, edge-case delimiters.
// SECURITY: Tokenizer processes user-supplied config files.
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
		tok := NewTokenizer(input)
		// Exhaust all tokens — MUST NOT panic or infinite loop
		for {
			token := tok.Next()
			if token.Type == TokenEOF {
				break
			}
		}
	})
}
