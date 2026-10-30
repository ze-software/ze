// Design: docs/architecture/site-facts.md -- the facts snapshot as pages read it back
// Related: factmark.go -- substituteHTML, the pass that calls these
//
// factfallback.go is the hand-written marked element: data-ze-stat="<key>"
// around fallback text in a page source. The build writes the snapshot's value
// into it when the snapshot holds one, and leaves the fallback while the key's
// section is null, which is the same rule site.js applies in the browser
// (fillRepoStats leaves an element whose key looks up to null).

package site

import (
	"errors"
	"reflect"
	"strconv"
	"strings"

	xhtml "golang.org/x/net/html"
)

// markerAttribute is the attribute that marks a value of data/site-facts.json.
const markerAttribute = "data-ze-stat"

// markerKey answers the data-ze-stat key of the start tag the tokenizer
// stands on, and false when the tag carries none.
func markerKey(tokenizer *xhtml.Tokenizer, hasAttributes bool) (string, bool) {
	for hasAttributes {
		var name, value []byte
		name, value, hasAttributes = tokenizer.TagAttr()
		if string(name) == markerAttribute {
			return string(value), true
		}
	}
	return "", false
}

// readMarkedText consumes the fallback text of one marked element and answers
// it with the element's raw closing tag.
//
// A marked element holds text only. Markup inside it would be replaced along
// with the text, so it is refused by name rather than dropped from the page,
// and so is a {{ze:<key>}} token, which would put one marked element inside
// another. Both are refused whether or not the snapshot holds the key's value
// yet, so a page that builds today still builds on the day it does. The walk
// ends at the element's own closing tag or at the end of the source, so it is
// bounded by the source's length.
func readMarkedText(tokenizer *xhtml.Tokenizer, element, key string) (string, string, error) {
	var fallback strings.Builder
	for {
		kind := tokenizer.Next()
		switch kind {
		case xhtml.TextToken:
			text := tokenizer.Raw()
			if strings.Contains(string(text), "{{ze:") {
				return "", "", errors.New("the " + markerAttribute + "=" + strconv.Quote(key) + " element holds a {{ze:}} token (" +
					strconv.Quote(string(text)) + "); a marked element holds its fallback text only, so write the value itself")
			}
			fallback.Write(text)
			continue
		case xhtml.EndTagToken:
			name, _ := tokenizer.TagName()
			if string(name) == element {
				return fallback.String(), string(tokenizer.Raw()), nil
			}
		case xhtml.ErrorToken:
			return "", "", errors.New("the " + markerAttribute + "=" + strconv.Quote(key) + " element is never closed")
		case xhtml.StartTagToken, xhtml.SelfClosingTagToken, xhtml.CommentToken, xhtml.DoctypeToken:
		}
		return "", "", errors.New("the " + markerAttribute + "=" + strconv.Quote(key) + " element holds markup (" +
			strconv.Quote(string(tokenizer.Raw())) + "); a marked element holds its fallback text only, so move the markup outside it")
	}
}

// markedValue answers what one hand-marked element shows.
//
// held is false when the key's path reaches a section the snapshot holds as
// null, such as the release while the committed file states none: the element
// keeps the fallback its author wrote. A path past that null is still checked,
// against the siteFacts model, so a misspelled key is refused today rather than
// on the day the section first holds a value. A key neither the snapshot nor
// the model names is refused, as an unknown token is.
func (snapshot siteFactSnapshot) markedValue(key string) (string, bool, error) {
	var value any = snapshot.facts
	for part := range strings.SplitSeq(key, ".") {
		if value == nil {
			if !modelNamesFactKey(reflect.TypeFor[siteFacts](), key) {
				return "", false, snapshot.unknownMarker(key)
			}
			return "", false, nil
		}
		object, isObject := value.(map[string]any)
		if !isObject {
			return "", false, snapshot.unknownMarker(key)
		}
		next, present := object[part]
		if !present {
			return "", false, snapshot.unknownMarker(key)
		}
		value = next
	}
	if value == nil {
		return "", false, nil
	}
	answer, err := snapshot.answer(key)
	if err != nil {
		return "", false, err
	}
	return answer, true, nil
}

// modelNamesFactKey reports whether the siteFacts model declares one dotted
// key: each segment a field's JSON name, through pointers, ending on a field
// that is not a section. A map-typed section names its keys only in a value,
// so any key under one is accepted. The walk is bounded by the key's segments.
func modelNamesFactKey(model reflect.Type, key string) bool {
	for part := range strings.SplitSeq(key, ".") {
		for model.Kind() == reflect.Pointer {
			model = model.Elem()
		}
		if model.Kind() == reflect.Map {
			return true
		}
		if model.Kind() != reflect.Struct {
			return false
		}
		field, declared := jsonField(model, part)
		if !declared {
			return false
		}
		model = field.Type
	}
	for model.Kind() == reflect.Pointer {
		model = model.Elem()
	}
	return model.Kind() != reflect.Struct
}

// jsonField answers the field of a struct type whose JSON name is name.
func jsonField(model reflect.Type, name string) (reflect.StructField, bool) {
	for field := range model.Fields() {
		tagged, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		if tagged == name {
			return field, true
		}
	}
	return reflect.StructField{}, false
}

// unknownMarker answers the refusal of one marked element whose key the
// snapshot does not name: the key, the closest key it does name, and the
// command that lists every key.
func (snapshot siteFactSnapshot) unknownMarker(key string) error {
	keys := snapshot.keys()
	var message strings.Builder
	message.WriteString("unknown marked value " + markerAttribute + "=" + strconv.Quote(key) + ": " + factsFile +
		" names no value " + strconv.Quote(key) + ". A marker names a dotted key of that file")
	if closest := closestFactKey(key, keys); closest != "" {
		message.WriteString(", such as " + markerAttribute + "=" + strconv.Quote(closest))
	}
	message.WriteString("; it holds " + strconv.Itoa(len(keys)) + " keys, listed by: jq -r 'paths(scalars) | join(\".\")' " +
		snapshot.path)
	return errors.New(message.String())
}
