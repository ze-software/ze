// Design: docs/architecture/config/syntax.md -- Enumeration values
// Related: setparser.go -- the set parser walk whose refusals these frame
// Related: setparser_meta.go -- the set-meta walk, framed the same way
// Related: leaf_value.go -- the refusal each restriction builds

package config

import (
	"slices"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// lineAt answers the tokens of the line being parsed that precede rest, a
// suffix of them: the setting path the walk consumed up to rest. The walk
// passes suffixes of lineTokens down, as keywordsBefore reads them.
func (p *SetParser) lineAt(rest []string) []string {
	consumed := len(p.lineTokens) - len(rest)
	if consumed < 0 {
		return nil
	}
	return p.lineTokens[:consumed]
}

// lineVerb is the verb of the line being parsed, the word its fix opens
// with: a `nop` line's fix is a `nop` line, which only a file holds, and an
// `inactive` line's fix is an `inactive` line.
func (p *SetParser) lineVerb() string {
	if p.nopLine {
		return cmdNop
	}
	if p.inactiveLine {
		return cmdInactive
	}
	return cmdSet
}

// framedRefusal answers err framed for the set line it refuses when err is
// an enum, union, range, length or pattern refusal, nil for any other error.
// The problem names path; the fix is the line's verb, before, the correction
// in place of the refused word, then after, what the line holds past it.
func (p *SetParser) framedRefusal(err error, path, before, after []string) *configyang.ValidationError {
	refusal, ok := operatorRefusal(err)
	if !ok {
		return nil
	}
	var fix textbuf.Buffer
	fix.Str(p.lineVerb())
	for _, word := range before {
		fix.Byte(' ').Str(QuoteIfNeeded(word))
	}
	fix.Byte(' ').Str(refusal.CorrectionText())
	for _, word := range after {
		fix.Byte(' ').Str(QuoteIfNeeded(word))
	}
	return refusal.WithPath(path).WithFix(fix.String())
}

// valueLineRefusal ties err, the refusal of the value or values rest holds
// for the leaf the line names before them, to lineNum. A value refusal is
// framed for the line (framedRefusal); any other keeps its text, "invalid
// value for <leaf>: <err>".
func (p *SetParser) valueLineRefusal(lineNum int, name string, rest []string, err error) error {
	at := p.lineAt(rest)
	if refusal := p.framedRefusal(err, at, at, nil); refusal != nil {
		return newLineError(lineNum, refusal)
	}
	return lineErrorf(lineNum, "invalid value for %s: %w", name, err)
}

// keyLineRefusal ties err, the refusal the entry creator gave the first key
// value tokens[0] of the list the line names before tokens, to lineNum. The
// problem names the key after the list, and the fix is the line with the key
// replaced and the rest of the line kept. Any other refusal keeps its text,
// "invalid key for <list>: <err>".
func (p *SetParser) keyLineRefusal(lineNum int, name string, tokens []string, err error) error {
	listPath := p.lineAt(tokens)
	if len(tokens) > 0 {
		path := append(slices.Clip(listPath), tokens[0])
		if refusal := p.framedRefusal(err, path, listPath, tokens[1:]); refusal != nil {
			return newLineError(lineNum, refusal)
		}
	}
	return lineErrorf(lineNum, "invalid key for %s: %w", name, err)
}

// lineKeyRefusal judges tokens[0], the first key value a set line types for
// the list the line names before tokens, as typed, whether or not the entry
// it names is stored, and answers keyLineRefusal's refusal, or nil. The entry
// creator judges a key only when it creates an entry, so without this a typed
// `default` or `x#2` naming an entry already stored would be accepted by a set
// line; the set parser refused it before the creator judged keys, and every
// value refused before stays refused. The refusal is the creator's, framed
// the same way, so the two read the same.
func (p *SetParser) lineKeyRefusal(lineNum int, list *ListNode, name string, tokens []string) error {
	if err := ValidateListKey(list, p.lineAt(tokens), tokens[0]); err != nil {
		return p.keyLineRefusal(lineNum, name, tokens, err)
	}
	return nil
}

// addressLineRefusal ties err, the refusal ParseListAddress gave the address
// tokens hold for the list the line names before them, to lineNum. A value
// refusal of a later key leaf names the path through that leaf, and its fix
// is the line with the leaf's value replaced. Any other refusal keeps its
// text.
func (p *SetParser) addressLineRefusal(lineNum int, tokens []string, err error) error {
	refusal, ok := operatorRefusal(err)
	if !ok {
		return newLineError(lineNum, err)
	}
	for i := 1; i+1 < len(tokens); i += 2 {
		if tokens[i] != refusal.Leaf() {
			continue
		}
		listPath := p.lineAt(tokens)
		path := append(slices.Clip(listPath), tokens[:i+1]...)
		return newLineError(lineNum, p.framedRefusal(err, path, path, tokens[i+2:]))
	}
	return newLineError(lineNum, err)
}
