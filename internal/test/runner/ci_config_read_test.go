package runner

import (
	"strings"
	"testing"
)

// TestTokenizeConfigMatchesZesSeparators pins the copied separator set.
//
// VALIDATES: every character ze's own tokenizer treats as a separator is one
// here. `readWord` (internal/component/config/tokenizer.go) breaks a word on
// space, tab, newline, carriage return, `{`, `}`, `[`, `]`, `;`, `"`, `'` and
// `#`; `skipWhitespaceAndComments` skips the first four and runs `#` to the end
// of the line.
// PREVENTS: the reader accepting a strict subset of what ze accepts, where every
// gap answers ABSENT. `local\t65000` is a valid ze configuration whose local AS
// the previous reader never saw, because it matched `keyword + " "`; the peer was
// then classified as not-eBGP and exempted from the guard that exists to refuse
// it. `peer\tname {` was worse: the peer left the population entirely.
//
// This test is the check `ai/rules/principles.md` requires beside an unavoidable
// copy. Ze's tokenizer is unexported, so the harness cannot call it; what it can
// do is state the set in one place and hold every member here.
func TestTokenizeConfigMatchesZesSeparators(t *testing.T) {
	// Every character readWord breaks a word on, between two words, must produce
	// two word tokens. The tab is the one that mattered: it is a valid ze
	// separator that the previous reader did not know about.
	for _, separator := range []string{" ", "\t", "\n", "\r", ";", "[", "]"} {
		t.Run("separator "+strings.TrimSpace(separator), func(t *testing.T) {
			tokens, err := tokenizeConfig("local" + separator + "65000")
			if err != nil {
				t.Fatalf("tokenizeConfig: %v", err)
			}
			if len(tokens) != 2 || tokens[0].word != "local" || tokens[1].word != "65000" {
				t.Errorf("%q separated into %v, want two words", separator, tokens)
			}
		})
	}

	// A parenthesis is NOT in readWord's set, so it does not break a word; scan()
	// only returns one as its own token where a token starts. Matching ze here
	// rather than separating on it is the difference between a copy and a guess.
	tokens, err := tokenizeConfig("local(65000")
	if err != nil {
		t.Fatalf("tokenizeConfig: %v", err)
	}
	if len(tokens) != 1 || tokens[0].word != "local(65000" {
		t.Errorf("a mid-word parenthesis produced %v, want one word: readWord does not break on it", tokens)
	}
	if tokens, err = tokenizeConfig("local ( 65000"); err != nil {
		t.Fatalf("tokenizeConfig: %v", err)
	} else if len(tokens) != 2 {
		t.Errorf("a standalone parenthesis produced %v, want it dropped between two words", tokens)
	}

	// A brace separates AND is kept, because block structure is what the
	// lookups navigate.
	tokens, err = tokenizeConfig("session{local 1}")
	if err != nil {
		t.Fatalf("tokenizeConfig: %v", err)
	}
	want := []configToken{{word: "session"}, {brace: '{'}, {word: "local"}, {word: "1"}, {brace: '}'}}
	if len(tokens) != len(want) {
		t.Fatalf("braces tokenized to %v, want %v", tokens, want)
	}
	for i := range want {
		if tokens[i] != want[i] {
			t.Errorf("token %d = %v, want %v", i, tokens[i], want[i])
		}
	}

	// A comment runs to the end of the line, so a brace inside one does not
	// count. This is what made `braceBody`'s comment-blindness disappear rather
	// than needing a guard of its own.
	config, err := readConfig("bgp {\n\t# a comment with a } brace in it\n\tsession {\n\t\tasn { local 65000 }\n\t}\n}")
	if err != nil {
		t.Fatalf("readConfig with a braced comment: %v", err)
	}
	as, declared := config.topLevel("bgp").topLevel("session").topLevel("asn").leaf("local")
	if !declared || as != "65000" {
		t.Errorf("local = %q, %v; want 65000, true past a comment holding a brace", as, declared)
	}

	// A quoted value is one word, quotes excluded, so a separator inside it does
	// not split it.
	tokens, err = tokenizeConfig(`description "router 2 with four routes"`)
	if err != nil {
		t.Fatalf("tokenizeConfig: %v", err)
	}
	if len(tokens) != 2 || tokens[1].word != "router 2 with four routes" {
		t.Errorf("quoted value tokenized to %v, want one word carrying the whole string", tokens)
	}

	// The escape set is ze's own (readString). A quote written \" does NOT close
	// the string, so a reader that missed it would cut a configuration ze reads
	// whole into two.
	for _, escaped := range []struct {
		name string
		text string
		want string
	}{
		{"an escaped quote does not close the string", `description "say \"hi\" twice"`, `say "hi" twice`},
		{"a backslash escapes itself", `description "one\\two"`, `one\two`},
		{"n and t are the two named escapes", `description "one\ntwo\tthree"`, "one\ntwo\tthree"},
		{"any other escaped character stands for itself", `description "a\qb"`, "aqb"},
	} {
		t.Run(escaped.name, func(t *testing.T) {
			tokens, err := tokenizeConfig(escaped.text)
			if err != nil {
				t.Fatalf("tokenizeConfig: %v", err)
			}
			if len(tokens) != 2 || tokens[1].word != escaped.want {
				t.Errorf("tokenized to %v, want one word %q", tokens, escaped.want)
			}
		})
	}
}

// TestReadConfigRefusesWhatItCannotCut is the reader's own third state.
//
// VALIDATES: an unterminated string and unbalanced braces each fail, rather than
// producing an empty or truncated token stream.
// PREVENTS: the defect this whole file was rewritten to remove. A reader that
// answers "" for input it could not read cannot be told apart from a
// configuration that declares nothing, and every guard built on it inherits that
// blindness (ai/rules/principles.md).
func TestReadConfigRefusesWhatItCannotCut(t *testing.T) {
	for _, broken := range []struct {
		name  string
		text  string
		names string
	}{
		{"unterminated double quote", `description "never closed`, "unterminated"},
		// The closing quote is escaped, so the string never terminates. Ze's own
		// reader runs it to the end of the input; this one refuses, because a
		// harness must never turn a text it could not read into an answer.
		{"a string whose only closing quote is escaped", `description "never closed\"`, "unterminated"},
		{"unterminated single quote", "description 'never closed", "unterminated"},
		{"a block left open", "bgp {\n\tpeer p {\n}\n", "do not balance"},
		{"a brace with no block open", "bgp {\n}\n}\n", "no block open"},
	} {
		t.Run(broken.name, func(t *testing.T) {
			_, err := readConfig(broken.text)
			if err == nil {
				t.Fatal("a text the reader cannot cut must fail, never come back empty")
			}
			if !strings.Contains(err.Error(), broken.names) {
				t.Errorf("error = %q, want it to say %q", err, broken.names)
			}
		})
	}
}

// TestConfigLookupsAreScopedToTheirOwnLevel is the fix for a container reading
// its nested peers.
//
// VALIDATES: `leaf` and `topLevel` see only the block's OWN level, so a nested
// block's declarations are never read as the container's, in either write order.
// PREVENTS: a group taking a nested peer's `asn` or `connection` as its default,
// which a sibling peer then inherited. `localAS` inherits the same way, so the
// eBGP verdict the coverage guard turns on could come from the wrong peer.
func TestConfigLookupsAreScopedToTheirOwnLevel(t *testing.T) {
	nested := "\tpeer inner {\n\t\tsession {\n\t\t\tasn {\n\t\t\t\tlocal 65111\n\t\t\t\tremote 65222\n\t\t\t}\n\t\t}\n\t}\n"
	own := "\tsession {\n\t\tasn {\n\t\t\tlocal 65000\n\t\t}\n\t}\n"

	for _, order := range []struct {
		name string
		text string
	}{
		{"own session first", "group edge {\n" + own + nested + "}\n"},
		{"nested peer first", "group edge {\n" + nested + own + "}\n"},
	} {
		t.Run(order.name, func(t *testing.T) {
			config, err := readConfig(order.text)
			if err != nil {
				t.Fatalf("readConfig: %v", err)
			}
			group := config.topLevel("group")
			as, declared := group.topLevel("session").topLevel("asn").leaf("local")
			if !declared || as != "65000" {
				t.Errorf("the group's own local AS = %q, %v; want 65000, true", as, declared)
			}
			if _, found := group.topLevel("session").topLevel("asn").leaf("remote"); found {
				t.Error("the group declares no remote AS; the nested peer's must not be read as its own")
			}

			// leaf carries its own scope, separately from topLevel: a leaf one
			// level down is not this block's declaration. Without it, a session
			// block would answer with its asn block's leaves.
			session := group.topLevel("session")
			if _, found := session.leaf("local"); found {
				t.Error("`local` sits inside `asn`, so the session block itself declares none")
			}
			if _, found := session.topLevel("asn").leaf("local"); !found {
				t.Error("the asn block does declare it")
			}
		})
	}
}

// TestConfigPresentSeparatesAbsentFromEmpty pins the third state on a block.
//
// VALIDATES: a block that is not there is absent, and one that is there and
// empty is present.
// PREVENTS: the tunnel lint reading "I did not find the stanza" as "the config
// omitted an optional leaf". `remote/ip` is `mandatory true` there, so the two
// answers have opposite meanings.
func TestConfigPresentSeparatesAbsentFromEmpty(t *testing.T) {
	config, err := readConfig("tunnel t {\n\tencapsulation {\n\t}\n}\n")
	if err != nil {
		t.Fatalf("readConfig: %v", err)
	}
	tunnel := config.topLevel("tunnel")
	if encap := tunnel.topLevel("encapsulation"); !encap.present {
		t.Error("an empty block that IS there must be present")
	}
	if missing := tunnel.topLevel("session"); missing.present {
		t.Error("a block that is not there must not be present")
	}
}
