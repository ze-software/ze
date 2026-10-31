// Design: docs/architecture/site-facts.md -- RFC completion data, the last build's value inside each marked element
// Related: rfcdata.go -- the files this reads back from the artifact
// Related: rfcprofiles.go -- the pages whose marked elements it fills
package site

import (
	"encoding/json"
	"errors"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// rfcFigureSet is the RFC completion data one build published, read back
// from data/rfc/ the way website/assets/js/rfc-figures.js reads it. The page
// generator writes its values only as the fallback text of a marked element:
// the browser replaces each one from the same files, so the two can differ
// only by a file republished after the page.
//
// Not safe for concurrent use while it is being loaded; read-only after.
type rfcFigureSet struct {
	bars rfcBarLayout
	// stems holds each per-RFC file decoded as the browser decodes it, so a
	// dotted field resolves the same way on both sides.
	stems    map[string]map[string]any
	profiles map[string]*rfcProfileData
}

// rfcPublishMissing is the command that writes data/rfc/, named in every
// refusal about a file this reader could not use.
const rfcPublishMissing = "run `./le site build` to publish data/rfc/"

// loadRFCFigureSet reads data/rfc/ from the artifact publishRFCData wrote it
// into. A file the index names that cannot be read is refused by path: a
// page filled without it would print a sum that leaves that RFC out.
func loadRFCFigureSet(output string) (*rfcFigureSet, error) {
	var index rfcDataIndex
	if err := readRFCDataFile(output, rfcDataIndexFile, &index); err != nil {
		return nil, err
	}
	set := &rfcFigureSet{bars: index.Bars, stems: make(map[string]map[string]any, len(index.RFCs)),
		profiles: make(map[string]*rfcProfileData, len(index.Profiles))}
	for _, entry := range index.RFCs {
		stem := map[string]any{}
		if err := readRFCDataFile(output, entry.File, &stem); err != nil {
			return nil, err
		}
		set.stems[entry.Stem] = stem
	}
	for _, entry := range index.Profiles {
		profile := &rfcProfileData{}
		if err := readRFCDataFile(output, entry.File, profile); err != nil {
			return nil, err
		}
		set.profiles[entry.Name] = profile
	}
	return set, nil
}

// readRFCDataFile decodes one published file into target.
func readRFCDataFile(output, name string, target any) error {
	path := filepath.Join(output, filepath.FromSlash(name))
	body, err := os.ReadFile(path) //nolint:gosec // a site build reads the artifact it was pointed at
	if err != nil {
		var tb textbuf.Buffer
		return errors.New(tb.Str(name).Str(" could not be read from the artifact (").Str(err.Error()).
			Str("), so the RFC pages cannot print the last build's figures; ").Str(rfcPublishMissing).String())
	}
	if err := json.Unmarshal(body, target); err != nil {
		var tb textbuf.Buffer
		return errors.New(tb.Str(name).Str(" in the artifact is not the RFC data's JSON (").Str(err.Error()).
			Str("); ").Str(rfcPublishMissing).String())
	}
	return nil
}

// members answers the stems a subject names: one RFC stem, or a profile tier
// written profile/<name>/required or profile/<name>/optional.
func (set *rfcFigureSet) members(subject string) ([]string, error) {
	name, isProfile := strings.CutPrefix(subject, "profile/")
	if !isProfile {
		return []string{subject}, nil
	}
	name, tier, found := strings.Cut(name, "/")
	if !found {
		return nil, rfcSubjectRefusal(subject)
	}
	profile := set.profiles[name]
	if profile == nil {
		var tb textbuf.Buffer
		return nil, errors.New(tb.Str("data/rfc/index.json lists no profile named ").Str(name).
			Str(", so a page marking ").Str(subject).Str(" has no figure; add rfc/profiles/").Str(name).
			Str(".md or fix the marker, then ").Str(rfcPublishMissing).String())
	}
	switch tier {
	case "required":
		return profile.Required, nil
	case "optional":
		return profile.Optional, nil
	}
	return nil, rfcSubjectRefusal(subject)
}

// rfcSubjectRefusal names a marker subject the RFC data does not answer.
func rfcSubjectRefusal(subject string) error {
	var tb textbuf.Buffer
	return errors.New(tb.Str(subject).Str(" is not a subject the RFC data answers; write an RFC stem, ").
		Str("profile/<name>/required or profile/<name>/optional (docs/architecture/site-facts.md, \"RFC completion data\")").String())
}

// sum answers the counts of a subject's members as rfc-figures.js sumStems
// does: rfcs, walked and not-walked, then must and should summed over the
// walked members only, count by count.
func (set *rfcFigureSet) sum(subject string) (map[string]any, error) {
	stems, err := set.members(subject)
	if err != nil {
		return nil, err
	}
	out := map[string]any{"rfcs": float64(len(stems))}
	walked, notWalked := 0, 0
	for _, stem := range stems {
		data := set.stems[stem]
		if data == nil {
			var tb textbuf.Buffer
			return nil, errors.New(tb.Str("data/rfc/index.json lists no RFC named ").Str(stem).
				Str(", which ").Str(subject).Str(" counts, so its figure would leave it out; add its rfc/short/ summary, then ").
				Str(rfcPublishMissing).String())
		}
		if held, _ := data["walked"].(bool); !held {
			notWalked++
			continue
		}
		walked++
		for _, key := range []string{"must", "should"} {
			counts, _ := data[key].(map[string]any)
			target, _ := out[key].(map[string]any)
			if target == nil {
				target = map[string]any{}
				out[key] = target
			}
			addRFCCounts(target, counts)
		}
	}
	out["walked"], out["not-walked"] = float64(walked), float64(notWalked)
	out["register"] = rfcRegisterOf(len(stems), notWalked, out)
	return out, nil
}

// rfcRegisterOf answers a set's mandatory register in words, as
// rfc-figures.js registerOf does: complete only when the set holds at least
// one RFC, none of them is unwalked, and neither the Ze correctness gap nor
// the OS testing gap holds a row (spec-rfc-figures-sectioned-presentation).
func rfcRegisterOf(rfcs, notWalked int, sum map[string]any) string {
	if rfcs == 0 {
		return rfcRegisterOpen
	}
	if notWalked != 0 {
		return rfcRegisterOpen
	}
	zeGap, zeHeld := lookupRFCField(sum, "must.ze-gap").(float64)
	osGap, osHeld := lookupRFCField(sum, "must.os-gap").(float64)
	if zeHeld && osHeld && zeGap == 0 && osGap == 0 {
		return rfcRegisterComplete
	}
	return rfcRegisterOpen
}

// The two words of the register field.
const (
	rfcRegisterComplete = "complete"
	rfcRegisterOpen     = "not complete"
)

// addRFCCounts adds every count of source into target: numbers, and the
// per-kind maps one level down, as rfc-figures.js addCounts does.
func addRFCCounts(target, source map[string]any) {
	for key, value := range source {
		switch typed := value.(type) {
		case float64:
			held, _ := target[key].(float64)
			target[key] = held + typed
		case map[string]any:
			nested, _ := target[key].(map[string]any)
			if nested == nil {
				nested = map[string]any{}
				target[key] = nested
			}
			addRFCCounts(nested, typed)
		}
	}
}

// lookupRFCField answers the value at a dotted field of a sum, nil when a
// part of the path is absent.
func lookupRFCField(sum map[string]any, field string) any {
	var value any = sum
	for part := range strings.SplitSeq(field, ".") {
		object, isObject := value.(map[string]any)
		if !isObject {
			return nil
		}
		value = object[part]
	}
	return value
}

// rfcNotOwedByKind matches a per-kind count of the rows not owed: a kind no
// row was excluded by is absent from every file, and its count is 0.
var rfcNotOwedByKind = regexp.MustCompile(`^(must|should)\.not-owed-by-kind\.[^.]+$`)

// value answers the text rfc-figures.js fillValue writes for subject:field.
func (set *rfcFigureSet) value(subject, field string) (string, error) {
	sum, err := set.sum(subject)
	if err != nil {
		return "", err
	}
	found := lookupRFCField(sum, field)
	if count, isCount := found.(float64); isCount {
		return rfcCountText(count), nil
	}
	if words, isWords := found.(string); isWords {
		return words, nil
	}
	if sum["rfcs"] == float64(0) {
		return "no rows", nil
	}
	if sum["walked"] == float64(0) {
		return "not walked", nil
	}
	if found == nil && rfcNotOwedByKind.MatchString(field) {
		return "0", nil
	}
	var tb textbuf.Buffer
	return "", errors.New(tb.Str(field).Str(" is not a field of the RFC data, so ").Str(subject).Str(":").Str(field).
		Str(" has no value; docs/architecture/site-facts.md lists the fields").String())
}

// bar answers the text a bar shows before rfc-figures.js drawBar replaces it:
// each non-zero segment's words and count, in the index's order, the same
// words the drawn bar and its legend carry.
func (set *rfcFigureSet) bar(subject, name string) (string, error) {
	segments, totalFields, err := set.barLayout(name)
	if err != nil {
		return "", err
	}
	sum, err := set.sum(subject)
	if err != nil {
		return "", err
	}
	if sum["rfcs"] == float64(0) {
		return "no rows", nil
	}
	if sum["walked"] == float64(0) {
		return "not walked", nil
	}
	total, err := rfcFieldsSum(sum, totalFields)
	if err != nil {
		return "", err
	}
	var words textbuf.Buffer
	drawn := float64(0)
	for _, segment := range segments {
		count, err := rfcFieldsSum(sum, segment.Fields)
		if err != nil {
			return "", err
		}
		drawn += count
		if count == 0 {
			continue
		}
		if words.Len() != 0 {
			words.Str(", ")
		}
		words.Str(segment.Label).Byte(' ').Str(rfcCountText(count))
	}
	if drawn != total {
		var tb textbuf.Buffer
		return "", errors.New(tb.Str("the ").Str(name).Str(" segments of ").Str(subject).Str(" sum to ").
			Str(rfcCountText(drawn)).Str(", not the ").Str(rfcCountText(total)).
			Str(" rows the files state, so the bar would misdraw them; ").Str(rfcPublishMissing).String())
	}
	if total == 0 {
		return "no rows", nil
	}
	return words.String(), nil
}

// barLayout answers one bar's segments and the fields of its total.
func (set *rfcFigureSet) barLayout(name string) ([]rfcBarSegment, []string, error) {
	switch name {
	case "must":
		return set.bars.Must, set.bars.MustTotal, nil
	case "overview":
		return set.bars.Overview, set.bars.OverviewTotal, nil
	}
	var tb textbuf.Buffer
	return nil, nil, errors.New(tb.Str(name).Str(" is not a bar the RFC data declares; write the marker as ").
		Str("<subject>:must or <subject>:overview").String())
}

// rfcFieldsSum adds the counts at fields, refusing one the sum does not hold.
func rfcFieldsSum(sum map[string]any, fields []string) (float64, error) {
	total := float64(0)
	for _, field := range fields {
		count, isCount := lookupRFCField(sum, field).(float64)
		if !isCount {
			var tb textbuf.Buffer
			return 0, errors.New(tb.Str("the bar layout names ").Str(field).
				Str(", which the RFC data does not hold; ").Str(rfcPublishMissing).String())
		}
		total += count
	}
	return total, nil
}

// rfcCountText writes a count the way rfc-figures.js does
// (toLocaleString("en-GB")): digits grouped by three with commas.
func rfcCountText(count float64) string {
	digits := strconv.Itoa(int(count))
	if len(digits) <= 3 {
		return digits
	}
	var out textbuf.Buffer
	lead := len(digits) % 3
	if lead != 0 {
		out.Str(digits[:lead])
	}
	for at := lead; at < len(digits); at += 3 {
		if out.Len() != 0 {
			out.Byte(',')
		}
		out.Str(digits[at : at+3])
	}
	return out.String()
}

// The marked elements this file fills. A renderer writes them empty, and
// fillHTML and fillMirror write the last build's value inside: a renderer
// never computes a figure, so no page can print one the files do not hold.
var (
	rfcValueElement  = regexp.MustCompile(`<span data-ze-rfc="([^"<>:]+):([^"<>]+)"></span>`)
	rfcBarElement    = regexp.MustCompile(`<div data-ze-rfc-bar="([^"<>:]+):([^"<>]+)"></div>`)
	rfcValueToken    = regexp.MustCompile(`\{\{ze-rfc ([^{}:]+):([^{}]+)\}\}`)
	rfcBarValueToken = regexp.MustCompile(`\{\{ze-rfc-bar ([^{}:]+):([^{}]+)\}\}`)
)

// rfcValueHTML is an element rfc-figures.js fills with subject's field.
func rfcValueHTML(subject, field string) string {
	return `<span data-ze-rfc="` + html.EscapeString(subject+":"+field) + `"></span>`
}

// rfcBarMarkHTML is an element rfc-figures.js draws subject's bar in.
func rfcBarMarkHTML(subject, bar string) string {
	return `<div data-ze-rfc-bar="` + html.EscapeString(subject+":"+bar) + `"></div>`
}

// rfcValueMirror is the place in a Markdown mirror that fillMirror writes
// subject's field into.
func rfcValueMirror(subject, field string) string { return "{{ze-rfc " + subject + ":" + field + "}}" }

// rfcBarMirror is the place in a Markdown mirror that fillMirror writes
// subject's bar into, as text.
func rfcBarMirror(subject, bar string) string { return "{{ze-rfc-bar " + subject + ":" + bar + "}}" }

// fillHTML writes the last build's value inside every marked element of a
// page, refusing the page when one has none.
func (set *rfcFigureSet) fillHTML(page string) (string, error) {
	var failure error
	fill := func(pattern *regexp.Regexp, answer func(subject, field string) (string, error), page string) string {
		return pattern.ReplaceAllStringFunc(page, func(element string) string {
			match := pattern.FindStringSubmatch(element)
			text, err := answer(html.UnescapeString(match[1]), html.UnescapeString(match[2]))
			if err != nil {
				failure = errors.Join(failure, err)
				return element
			}
			closing := strings.LastIndexByte(element, '<')
			return element[:closing] + html.EscapeString(text) + element[closing:]
		})
	}
	page = fill(rfcValueElement, set.value, page)
	page = fill(rfcBarElement, set.bar, page)
	return page, failure
}

// fillMirror writes the last build's value at every mark of a Markdown
// mirror, refusing the mirror when one has none.
func (set *rfcFigureSet) fillMirror(mirror string) (string, error) {
	var failure error
	fill := func(pattern *regexp.Regexp, answer func(subject, field string) (string, error), mirror string) string {
		return pattern.ReplaceAllStringFunc(mirror, func(token string) string {
			match := pattern.FindStringSubmatch(token)
			text, err := answer(match[1], match[2])
			if err != nil {
				failure = errors.Join(failure, err)
				return token
			}
			return rfcMirrorCell(text)
		})
	}
	mirror = fill(rfcValueToken, set.value, mirror)
	mirror = fill(rfcBarValueToken, set.bar, mirror)
	return mirror, failure
}
