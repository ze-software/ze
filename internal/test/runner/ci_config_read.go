// Design: docs/architecture/testing/ci-format.md — reading the ze configuration a .ci embeds
// Overview: peer_asn.go — derives each ze-peer's AS from the peers it reads here
// Related: tunnel_endpoint_lint_test.go — the interface lint that reads tunnel blocks
// Related: internal/component/config/tokenizer.go — the tokenizer whose separator set this copies
//
// A .ci carries its ze configuration inline, and two runner surfaces need to
// read parts of it before any daemon runs: the tunnel-endpoint lint, and the
// peer AS derivation. Neither can load it through the BGP configuration loader,
// which needs the YANG schema, the plugin registry and a resolved tree.
//
// # Why this reads tokens rather than matching text
//
// It used to match `keyword + " "` at a token start and answer "" for anything
// else. Three things followed from that, and all three were the same defect:
// a leaf written `local\tlocal-as` with a TAB was not found, because ze's
// tokenizer separates on tabs and this did not; a block written `peer\tname {`
// made the peer vanish from the population a guard walked; and a `#` comment
// holding a brace unbalanced the brace counter. Every one of them came back as
// "" -- the same answer the reader gives for a configuration that genuinely
// declares nothing.
//
// So the text is TOKENIZED once, and every lookup below runs over tokens. After
// tokenizing there is no "did not match": a keyword is in the stream or it is
// not. The only unreadable input left is a malformed text, and readConfig
// REFUSES that rather than answering with an empty stream
// (ai/rules/principles.md).
//
// # The separator set is a copy, and it names its source
//
// Ze's tokenizer is unexported, so this cannot call it. The sets below are
// copied from `readWord` and `skipWhitespaceAndComments`
// (internal/component/config/tokenizer.go) rather than guessed, and
// TestTokenizeConfigMatchesZesSeparators pins every member of both. Exporting
// ze's tokenizer to serve a test harness would put a product API change inside a
// harness change, and importing the config package would pull the YANG schema
// and the plugin registry into this one.

package runner

import (
	"errors"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// configSeparators are the characters that END a word.
//
// Copied from `readWord` (internal/component/config/tokenizer.go), which breaks
// on exactly this set.
const configSeparators = " \t\n\r{}[];\"'#"

// configToken is one token of a ze configuration: a word, or a brace.
//
// Brackets, parentheses and semicolons separate words and carry nothing this
// package reads, so they are consumed and dropped. Braces are kept because the
// block structure is what the lookups below navigate.
type configToken struct {
	word  string
	brace byte
}

// configFile is a tokenized ze configuration, or one block of one.
//
// Every lookup on it is scoped to its OWN top level. A block nested inside it
// answers no question asked of it, which is what keeps a group's declarations
// separate from those of the peers written inside that group.
type configFile struct {
	tokens []configToken
	// present is false only for a block that is NOT THERE. A block that is
	// there and empty is present, and answers "nothing declared" to every
	// lookup. Keeping the two apart is what lets a caller whose schema makes a
	// leaf mandatory tell "the config omitted it" from "I did not find the
	// stanza" (ai/rules/principles.md).
	present bool
}

// readConfig tokenizes a ze configuration.
//
// It refuses a text it cannot cut into tokens, and a text whose braces do not
// balance. Both are configurations the daemon would refuse too, and answering an
// empty stream for either would put this package back where it started: unable
// to tell "the configuration declares nothing" from "I could not read the
// configuration".
func readConfig(raw string) (configFile, error) {
	tokens, err := tokenizeConfig(raw)
	if err != nil {
		return configFile{}, err
	}
	depth := 0
	for _, token := range tokens {
		switch token.brace {
		case '{':
			depth++
		case '}':
			depth--
		}
		if depth < 0 {
			return configFile{}, errors.New("a closing brace with no block open")
		}
	}
	if depth != 0 {
		var why textbuf.Buffer
		why.Str("the braces do not balance: ").Int(int64(depth)).Str(" block(s) left open")
		return configFile{}, errors.New(why.String())
	}
	return configFile{tokens: tokens, present: true}, nil
}

// tokenizeConfig cuts a ze configuration into words and braces.
func tokenizeConfig(raw string) ([]configToken, error) {
	var tokens []configToken
	at := 0
	for at < len(raw) {
		ch := raw[at]

		// skipWhitespaceAndComments: space, tab and carriage return separate,
		// a newline separates, and `#` runs to the end of the line.
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			at++
			continue
		}
		if ch == '#' {
			for at < len(raw) && raw[at] != '\n' {
				at++
			}
			continue
		}

		// The punctuation ze's scan() returns as its own token. Only the braces
		// mean anything to a reader of blocks and leaves.
		if ch == '{' || ch == '}' {
			tokens = append(tokens, configToken{brace: ch})
			at++
			continue
		}
		if ch == '[' || ch == ']' || ch == '(' || ch == ')' || ch == ';' {
			at++
			continue
		}

		// readString: a quoted value is one word, quotes excluded.
		if ch == '"' || ch == '\'' {
			word, next, ok := readQuoted(raw, at)
			if !ok {
				var why textbuf.Buffer
				why.Str("an unterminated ").Byte(ch).Str(" string")
				return nil, errors.New(why.String())
			}
			tokens = append(tokens, configToken{word: word})
			at = next
			continue
		}

		// readWord: everything up to the next separator.
		end := at
		for end < len(raw) && !strings.ContainsRune(configSeparators, rune(raw[end])) {
			end++
		}
		tokens = append(tokens, configToken{word: raw[at:end]})
		at = end
	}
	return tokens, nil
}

// readQuoted reads the quoted string starting at at, and returns its value, the
// index just past its closing quote, and whether it is terminated.
//
// The escape set is ze's own, copied from `readString`
// (internal/component/config/tokenizer.go): a backslash escapes the next
// character, `\n` and `\t` become a newline and a tab, and every other escaped
// character stands for itself. A closing quote written `\"` therefore does NOT
// end the string, and a reader that missed that would cut a configuration ze
// reads whole into two, which is the divergence class this file exists to close.
//
// Ze's own reader treats an unterminated string as running to the end of the
// input. This one reports it instead, and the caller refuses the file. That is a
// deliberate divergence in the safe direction: the harness must never turn a
// text it could not read into an answer.
func readQuoted(raw string, at int) (word string, next int, ok bool) {
	quote := raw[at]
	var value []byte
	for scan := at + 1; scan < len(raw); scan++ {
		ch := raw[scan]
		if ch == quote {
			return string(value), scan + 1, true
		}
		if ch != '\\' || scan+1 >= len(raw) {
			value = append(value, ch)
			continue
		}
		scan++
		switch raw[scan] {
		case 'n':
			value = append(value, '\n')
		case 't':
			value = append(value, '\t')
		default:
			value = append(value, raw[scan])
		}
	}
	return "", 0, false
}

// configBlock is one `<keyword> [<arg>] { ... }` of a configuration.
type configBlock struct {
	arg  string
	body configFile
}

// blocks returns every block the given keyword opens, at any depth, in the order
// they appear.
func (c configFile) blocks(keyword string) []configBlock {
	var out []configBlock
	for at := range len(c.tokens) {
		block, arg, ok := c.blockAt(at, keyword)
		if !ok {
			continue
		}
		// The scan does NOT skip past the block it just took: a peer nested in a
		// group has to be found by the same walk that finds the group.
		out = append(out, configBlock{arg: arg, body: block})
	}
	return out
}

// topLevel returns the first block the given keyword opens at this file's OWN
// top level, or an empty one.
//
// An absent block and an empty block answer alike here, and that is correct: a
// caller asks what the configuration DECLARES, and neither declares anything.
// The distinction this package must not lose is "could not read", and readConfig
// has already made that impossible by refusing rather than returning tokens.
func (c configFile) topLevel(keyword string) configFile {
	depth := 0
	for at := range len(c.tokens) {
		switch c.tokens[at].brace {
		case '{':
			depth++
			continue
		case '}':
			depth--
			continue
		}
		if depth != 0 {
			continue
		}
		if block, _, ok := c.blockAt(at, keyword); ok {
			return block
		}
	}
	return configFile{}
}

// blockAt reports whether a block the given keyword opens starts at index at,
// and returns its body and its argument.
func (c configFile) blockAt(at int, keyword string) (body configFile, arg string, ok bool) {
	if c.tokens[at].word != keyword {
		return configFile{}, "", false
	}
	open := at + 1
	if open < len(c.tokens) && c.tokens[open].brace == 0 {
		arg = c.tokens[open].word
		open++
	}
	if open >= len(c.tokens) || c.tokens[open].brace != '{' {
		return configFile{}, "", false
	}
	depth := 0
	for scan := open; scan < len(c.tokens); scan++ {
		switch c.tokens[scan].brace {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return configFile{tokens: c.tokens[open+1 : scan], present: true}, arg, true
			}
		}
	}
	// readConfig balanced the braces before any lookup ran, so this is
	// unreachable for a file that reached here.
	return configFile{}, "", false
}

// leaf returns the value of a `keyword value` statement at this file's OWN top
// level, and whether the statement is there.
//
// Scoped to the top level because a group holds the peers written inside it: an
// unscoped read took a nested peer's `asn` as the group's default whenever the
// group's own session was written below that peer.
func (c configFile) leaf(keyword string) (string, bool) {
	depth := 0
	for at := range len(c.tokens) {
		switch c.tokens[at].brace {
		case '{':
			depth++
			continue
		case '}':
			depth--
			continue
		}
		if depth != 0 || c.tokens[at].word != keyword {
			continue
		}
		if at+1 >= len(c.tokens) || c.tokens[at+1].brace != 0 {
			continue
		}
		return c.tokens[at+1].word, true
	}
	return "", false
}

// blockIP returns the `ip` leaf of the named sub-block, e.g. the 192.0.2.1 of
// `local { ip 192.0.2.1; }`, and whether the leaf is there. Every caller wants
// that leaf and no other, so the name is not a parameter.
func (c configFile) blockIP(block string) (string, bool) {
	return c.topLevel(block).leaf("ip")
}
