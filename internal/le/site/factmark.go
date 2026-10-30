// Design: docs/architecture/site-facts.md -- the facts snapshot as pages read it back
// Related: facts.go -- the siteFacts model this file reads back
// Related: plaintext.go -- the check that refuses a value published outside its marker
// Related: docpage.go -- Markdown sources resolve their tokens through substitute

package site

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	xhtml "golang.org/x/net/html"
)

// siteFactSnapshot is data/site-facts.json as a page producer reads it back:
// the one published copy of every value that changes over time.
//
// Its keys are the snapshot's own dotted JSON paths, so the set of values a
// page can name is the set the siteFacts model publishes, with no table of
// names between them. The keys are read from the PUBLISHED FILE rather than by
// reflection over the siteFacts type, because the file is json.Marshal of that
// model, so both answer the same struct keys, and only the file can answer the
// keys of a map-typed section, which exist in a value and never in a type.
//
// A nil facts map is the first build into an empty tree, which carries no
// snapshot yet. Every token is then left alone rather than guessed or refused.
type siteFactSnapshot struct {
	path  string
	facts map[string]any
}

// valueTokenPattern matches one {{ze:<dotted.key>}} token in a page source.
//
// It accepts any name rather than the key grammar, so a malformed or retired
// name is refused by substitute instead of reaching a reader as braces.
var valueTokenPattern = regexp.MustCompile(`\{\{ze:([^{}\s]+)\}\}`)

// loadSiteFactSnapshot reads the facts snapshot this build already published.
//
// The snapshot is read from the ARTIFACT rather than from the source tree,
// because it is the build's own output: publishSiteFacts writes it before any
// page producer runs and the producers read it back. An absent file is a first
// build, not an error.
func loadSiteFactSnapshot(output string) (siteFactSnapshot, error) {
	path := filepath.Join(output, filepath.FromSlash(factsFile))
	content, err := os.ReadFile(path) //nolint:gosec // a site build reads the artifact it was pointed at
	if errors.Is(err, os.ErrNotExist) {
		// refuse-by-default: guards substitute -- a nil facts map is the first build, which leaves every token alone
		return siteFactSnapshot{path: path}, nil
	}
	if err != nil {
		return siteFactSnapshot{}, err
	}
	snapshot, err := parseSiteFactSnapshot(content)
	if err != nil {
		return siteFactSnapshot{}, fmt.Errorf("read %s: %w", path, err)
	}
	snapshot.path = path
	return snapshot, nil
}

// parseSiteFactSnapshot answers one snapshot from its published JSON.
func parseSiteFactSnapshot(content []byte) (siteFactSnapshot, error) {
	var facts map[string]any
	if err := json.Unmarshal(content, &facts); err != nil {
		return siteFactSnapshot{}, err
	}
	if facts == nil {
		return siteFactSnapshot{}, fmt.Errorf("the facts snapshot holds no object")
	}
	return siteFactSnapshot{path: factsFile, facts: facts}, nil
}

// value answers one dotted key as the string a page shows.
func (snapshot siteFactSnapshot) value(key string) (string, bool) {
	return factValue(snapshot.facts, key)
}

// keys answers every dotted key whose value a page can show, sorted.
//
// The walk is iterative over the snapshot's own objects, so a hostile or
// corrupt artifact cannot make it recurse; each object is visited once.
func (snapshot siteFactSnapshot) keys() []string {
	type pending struct {
		prefix string
		object map[string]any
	}
	var keys []string
	stack := []pending{{object: snapshot.facts}}
	for len(stack) != 0 {
		top := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for name, child := range top.object {
			key := top.prefix + name
			if object, isObject := child.(map[string]any); isObject {
				stack = append(stack, pending{prefix: key + ".", object: object})
				continue
			}
			if _, shown := factValue(snapshot.facts, key); shown {
				keys = append(keys, key)
			}
		}
	}
	slices.Sort(keys)
	return keys
}

// markedFact answers one snapshot value as the element a page shows it in.
//
// It is the ONE place a published value becomes markup. The span names its key,
// so site.js can replace the text from data/site-facts.json at load and the
// site check can tell a marked value from a plain one. Both the key and the
// value are escaped: the value is data, never markup.
func markedFact(key, value string) string {
	return `<span data-ze-stat="` + html.EscapeString(key) + `">` + html.EscapeString(value) + `</span>`
}

// factLookup reads snapshot values for a producer that writes them itself,
// rather than through a token in a source.
//
// A producer names many values while it builds one page, so the lookup keeps
// the first refusal and the producer asks failure() once, before it publishes.
// A value it could not answer is the empty string, and failure() then says why,
// so no page built from a refused lookup is ever written.
//
// Not safe for concurrent use: one producer owns one lookup.
type factLookup struct {
	snapshot siteFactSnapshot
	refusal  error
}

// marked answers one key as the element a page shows it in (markedFact).
func (lookup *factLookup) marked(key string) string {
	value := lookup.plain(key)
	if lookup.refusal != nil {
		return ""
	}
	return markedFact(key, value)
}

// plain answers one key as the plain value, for a Markdown mirror, llms.txt,
// or an attribute a span cannot go in.
func (lookup *factLookup) plain(key string) string {
	if lookup.refusal != nil {
		return ""
	}
	if lookup.snapshot.facts == nil {
		lookup.refusal = fmt.Errorf("%s is not published yet, so no page can show %s: run ./le site build",
			lookup.snapshot.path, key)
		return ""
	}
	value, err := lookup.snapshot.answer(key)
	if err != nil {
		lookup.refusal = err
		return ""
	}
	return value
}

// answer resolves one key to the value a page shows, or refuses it by name.
//
// A key the snapshot does not name is refused (unknownToken), and so is a key
// whose value is the empty string: a page would show an empty span where a
// figure belongs, and a reader cannot tell a figure nobody derived from one
// left blank on purpose. Every reader of a value, a token or a producer's
// lookup, resolves it here, so neither refusal can be skipped.
func (snapshot siteFactSnapshot) answer(key string) (string, error) {
	value, found := snapshot.value(key)
	if !found {
		return "", snapshot.unknownToken(key)
	}
	if value == "" {
		return "", fmt.Errorf("%s states %s as an empty value, so no page can show it: "+
			"fix the derivation that writes %s, then run ./le site build", snapshot.path, key, key)
	}
	return value, nil
}

// failure answers the first key this lookup could not answer, or nil when it
// answered every one. A producer MUST ask it before it publishes the page.
func (lookup *factLookup) failure() error {
	return lookup.refusal
}

// substitute resolves every {{ze:<dotted.key>}} token of one Markdown source.
//
// marked asks for the markedFact span around each value, for the HTML page; the
// Markdown mirror takes the plain value instead. A token whose key the snapshot
// does not name is refused by name: it can never resolve, so leaving it would
// publish the literal braces to a reader. A build with no snapshot leaves every
// token alone, because that build has not computed the values yet rather than
// holding a source that is wrong.
func (snapshot siteFactSnapshot) substitute(source string, marked bool) (string, error) {
	if snapshot.facts == nil {
		return source, nil
	}
	if !strings.Contains(source, "{{ze:") {
		return source, nil
	}
	var refusal error
	replaced := valueTokenPattern.ReplaceAllStringFunc(source, func(token string) string {
		key := valueTokenPattern.FindStringSubmatch(token)[1]
		value, err := snapshot.answer(key)
		if err != nil {
			if refusal == nil {
				refusal = err
			}
			return token
		}
		if marked {
			return markedFact(key, value)
		}
		return value
	})
	if refusal != nil {
		return "", refusal
	}
	return replaced, nil
}

// substituteHTML resolves every token of one hand-written HTML source.
//
// A token in a text node of the body becomes a marked span. A token anywhere a
// span cannot go becomes the plain, escaped value: inside a tag (an attribute),
// inside <head> (the title and the descriptions a link preview reads), and
// inside a raw-text element (script, style, textarea). A span there would be
// markup inside markup (risk R-1 of the spec that introduced tokens in HTML).
//
// An element the author marked by hand, data-ze-stat="<key>" around fallback
// text, gets the snapshot's value as its text when the snapshot holds one, and
// keeps its fallback while the key's section is null (markedValue): the lab
// page's ISO size reads "under 150 MB" until a release carries an ISO. Either
// way the element's text is what the snapshot states or the author's
// fallback, never a value of an earlier build left behind.
//
// The page is rebuilt from each token's raw bytes, so everything that is not a
// token reaches the artifact byte for byte as its author wrote it.
func (snapshot siteFactSnapshot) substituteHTML(source string) (string, error) {
	if snapshot.facts == nil {
		return source, nil
	}
	if !strings.Contains(source, "{{ze:") && !strings.Contains(source, markerAttribute) {
		return source, nil
	}
	var page strings.Builder
	page.Grow(len(source))
	tokenizer := xhtml.NewTokenizer(strings.NewReader(source))
	inHead, rawText := false, ""
	for {
		kind := tokenizer.Next()
		if kind == xhtml.ErrorToken {
			if errors.Is(tokenizer.Err(), io.EOF) {
				return page.String(), nil
			}
			return "", tokenizer.Err()
		}
		raw := string(tokenizer.Raw())
		marked := false
		switch kind {
		case xhtml.StartTagToken:
			name, hasAttributes := tokenizer.TagName()
			inHead, rawText = htmlTokenContext(string(name), true, inHead, rawText)
			key, isMarker := markerKey(tokenizer, hasAttributes)
			if !isMarker {
				break
			}
			value, held, err := snapshot.markedValue(key)
			if err != nil {
				return "", err
			}
			if !held {
				break
			}
			closing, err := skipMarkedText(tokenizer, string(name), key)
			if err != nil {
				return "", err
			}
			page.WriteString(raw + html.EscapeString(value) + closing)
			continue
		case xhtml.EndTagToken:
			name, _ := tokenizer.TagName()
			inHead, rawText = htmlTokenContext(string(name), false, inHead, rawText)
		case xhtml.TextToken:
			marked = !inHead && rawText == ""
		case xhtml.ErrorToken, xhtml.SelfClosingTagToken, xhtml.CommentToken, xhtml.DoctypeToken:
		}
		resolved, err := snapshot.substituteHTMLToken(raw, marked)
		if err != nil {
			return "", err
		}
		page.WriteString(resolved)
	}
}

// htmlTokenContext answers where the tokenizer stands after one tag: inside
// <head> or not, and the raw-text element it is inside, if any.
func htmlTokenContext(name string, opening, inHead bool, rawText string) (bool, string) {
	switch name {
	case "head":
		return opening, rawText
	case "body":
		return false, rawText
	case "script", "style", "textarea", "title":
		if opening {
			return inHead, name
		}
		return inHead, ""
	}
	return inHead, rawText
}

// substituteHTMLToken resolves the tokens of one raw HTML token: marked spans
// for a body text node, the escaped plain value everywhere else.
func (snapshot siteFactSnapshot) substituteHTMLToken(raw string, marked bool) (string, error) {
	if marked {
		return snapshot.substitute(raw, true)
	}
	if !strings.Contains(raw, "{{ze:") {
		return raw, nil
	}
	var refusal error
	replaced := valueTokenPattern.ReplaceAllStringFunc(raw, func(token string) string {
		key := valueTokenPattern.FindStringSubmatch(token)[1]
		value, err := snapshot.answer(key)
		if err != nil {
			if refusal == nil {
				refusal = err
			}
			return token
		}
		return html.EscapeString(value)
	})
	if refusal != nil {
		return "", refusal
	}
	return replaced, nil
}

// unknownToken answers the refusal of one token whose key the snapshot does
// not name: the token, the file that decides, the closest key it does name,
// and the command that lists every key.
func (snapshot siteFactSnapshot) unknownToken(key string) error {
	keys := snapshot.keys()
	var message strings.Builder
	message.WriteString("unknown site value token {{ze:" + key + "}}: " + factsFile +
		" names no value " + strconv.Quote(key) + ". A token names a dotted key of that file")
	if closest := closestFactKey(key, keys); closest != "" {
		message.WriteString(", such as {{ze:" + closest + "}}")
	}
	message.WriteString("; it holds " + strconv.Itoa(len(keys)) + " keys, listed by: jq -r 'paths(scalars) | join(\".\")' " +
		snapshot.path)
	return errors.New(message.String())
}

// closestFactKey answers the first key, in sorted order, that spells the name
// once separators are ignored, so a retired name such as interop-scenarios
// points at interop.scenarios. It answers "" when no key contains it.
func closestFactKey(name string, keys []string) string {
	folded := foldFactKey(name)
	if folded == "" {
		return ""
	}
	for _, key := range keys {
		if strings.Contains(foldFactKey(key), folded) {
			return key
		}
	}
	return ""
}

// foldFactKey drops the separators a token name may be spelled with.
func foldFactKey(name string) string {
	return strings.NewReplacer("-", "", "_", "", ".", "").Replace(strings.ToLower(name))
}
