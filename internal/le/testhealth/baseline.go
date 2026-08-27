// Design: docs/architecture/testing/test-health.md -- the two ratchets
//
// baseline.go holds the two committed floors and the arithmetic helpers the
// collectors share.
//
// The two ratchets move in opposite directions and that is the whole point.
// The SENSITIVITY floor counts things that must shrink -- inert tests,
// stranded files -- so it may only fall. The QUALITY floor holds percentages
// that must grow, so it may only rise. Each locks in what has been achieved,
// and each makes the opposite move an event rather than a silent edit.
package testhealth

import (
	"bytes"
	"encoding/json"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// sensitivityFloors is the committed sensitivity ratchet, or the absence of
// one. A floor that is not present is not a floor of zero: the two are
// different facts and the status and the suffix both read them apart.
type sensitivityFloors struct {
	assertNothing floor
	tagOrphan     floor
}

// floor is one ratchet floor, which a file may leave out.
type floor struct {
	set   bool
	value int
}

// ratchetStatus answers whether a ratcheted count still honors its floor. With
// no floor recorded, any count at all is worth attention.
func ratchetStatus(actual int, limit floor) string {
	if !limit.set {
		if actual > 0 {
			return statusWarn
		}
		return statusOK
	}
	if actual <= limit.value {
		return statusOK
	}
	return statusWarn
}

// floorSuffix names the floor in a rendered value, and says nothing when there
// is none.
func floorSuffix(limit floor) string {
	if !limit.set {
		return ""
	}
	var tb textbuf.Buffer
	return tb.Str(" (floor ").Int(int64(limit.value)).Byte(')').String()
}

// readSensitivityFloors reads the committed floors, validated.
//
// Every consumer goes through this so a malformed baseline produces one
// diagnosed error rather than a failure from whichever collector touched it
// first. An absent file answers "no floors known", which the write path then
// refuses to mint from unless it was asked to bootstrap.
func readSensitivityFloors(root string) (sensitivityFloors, error) {
	path := filepath.Join(root, filepath.FromSlash(Baseline))
	if !exists(path) {
		return sensitivityFloors{}, nil
	}
	raw, err := os.ReadFile(path) // #nosec G304 -- a repository-relative path of the checkout this tool was pointed at
	if err != nil {
		return sensitivityFloors{}, collectErrorf("%s cannot be read: %w", Baseline, err)
	}

	parsed, err := parseObject(raw)
	if err != nil {
		return sensitivityFloors{}, collectErrorf("%s is not valid JSON: %w", Baseline, err)
	}

	var floors sensitivityFloors
	for _, key := range parsed.keys {
		value := parsed.get(key)
		number, ok := value.(pyNum)
		if !ok || !number.isInt {
			return sensitivityFloors{}, collectErrorf(
				"%s floor %s is %s; an integer is required.",
				Baseline, quote(key), valueText(value))
		}
		if number.i < 0 {
			return sensitivityFloors{}, collectErrorf(
				"%s floor %s is negative: %s", Baseline, quote(key), number.String())
		}
		switch key {
		case keyAssertNothing:
			floors.assertNothing = floor{set: true, value: int(number.i)}
		case keyTagOrphan:
			floors.tagOrphan = floor{set: true, value: int(number.i)}
		}
	}
	return floors, nil
}

// tightenSensitivity lowers the ratchet floors to the counts just measured, and
// answers whether one moved.
//
// A floor may only FALL. Raising one here would let a regression be laundered
// into the baseline simply by running the generator, which is the opposite of
// what a ratchet is for. A MISSING KEY is therefore an error, not a default:
// defaulting it to the current count is exactly how a raise sneaks through.
//
// A MISSING FILE is the same hole one level up -- deleting the baseline and
// regenerating would mint today's counts as the new floors, and the ratchet
// would then pass. Creating the file is a deliberate act, requested with
// bootstrap, so the write path can tell bootstrap from laundering.
func tightenSensitivity(root string, row object, bootstrap bool) (bool, error) {
	measured := map[string]int{
		keyAssertNothing: intOf(row.get("assert_nothing")),
		keyTagOrphan:     intOf(row.get("tag_orphan")),
	}
	path := filepath.Join(root, filepath.FromSlash(Baseline))
	present := exists(path)

	if !present && !bootstrap {
		return false, collectErrorf(
			"%s does not exist. Restore it from git rather than letting this run mint %s as "+
				"the new floors: a deleted baseline would launder any regression. To create it "+
				"deliberately the first time, pass bootstrap.", Baseline, intDict(measured))
	}

	next := maps.Clone(measured)
	changed := true
	if present {
		floors, err := readSensitivityFloors(root)
		if err != nil {
			return false, err
		}
		known := map[string]floor{
			keyAssertNothing: floors.assertNothing,
			keyTagOrphan:     floors.tagOrphan,
		}
		changed = false
		for key, value := range measured {
			old := known[key]
			if !old.set {
				return false, collectErrorf(
					"%s has no %s floor. Refusing to invent one: a missing floor silently "+
						"becomes whatever the count is today, which turns a regression into the "+
						"new baseline. Restore the file from git.", Baseline, quote(key))
			}
			next[key] = min(value, old.value)
			if next[key] != old.value {
				changed = true
			}
		}
	}

	written := object{}
	for _, key := range sortedKeys(next) {
		written.set(key, next[key])
	}
	if err := writeJSONFile(root, Baseline, written); err != nil {
		return false, err
	}
	return changed, nil
}

// qualityFloors is the committed "best so far" for the higher-is-better ratio
// metrics.
//
// Unlike the sensitivity baseline, a MISSING quality floor is not a laundering
// risk and defaults to "no regression yet": the low number it would flag is
// still published on the page regardless, and no exit code is tied to it, so
// deleting the file to silence a warning is self-defeating.
type qualityFloors struct{ values object }

// readQualityFloors reads the locked-in best percentages, validated.
//
// The VALUES are validated here and the script does not validate them, which is
// a fail-open the port closes: a floor of `null` made the metric read `ok`
// forever, and a floor of `true` compared as 1.0 and did the same. A guard whose
// threshold can be silently replaced by something that always passes is not a
// guard (ai/rules/evidence.md).
func readQualityFloors(root string) (qualityFloors, error) {
	path := filepath.Join(root, filepath.FromSlash(QualityBaseline))
	if !exists(path) {
		return qualityFloors{values: object{}}, nil
	}
	raw, err := os.ReadFile(path) // #nosec G304 -- a repository-relative path of the checkout this tool was pointed at
	if err != nil {
		return qualityFloors{}, collectErrorf("%s cannot be read: %w", QualityBaseline, err)
	}
	parsed, err := parseObject(raw)
	if err != nil {
		return qualityFloors{}, collectErrorf("%s is not valid JSON: %w", QualityBaseline, err)
	}
	for _, key := range parsed.keys {
		if _, ok := parsed.get(key).(pyNum); !ok {
			return qualityFloors{}, collectErrorf(
				"%s floor %s is %s; a number is required. A floor that is not a number is "+
					"either ignored or read as one, and both readings make the metric pass "+
					"whatever it measured.", QualityBaseline, quote(key), valueText(parsed.get(key)))
		}
	}
	return qualityFloors{values: parsed}, nil
}

// qualityTolerance absorbs float-rounding noise, so a metric does not warn on a
// 0.05% wobble that is really the same value.
const qualityTolerance = 0.1

// status answers OK unless percent has regressed below the committed best.
func (q qualityFloors) status(key string, percent any) string {
	if percent == nil {
		return statusUnknown
	}
	stored, ok := q.values.get(key).(pyNum)
	if !ok {
		return statusOK
	}
	if numberOf(percent) >= stored.Float()-qualityTolerance {
		return statusOK
	}
	return statusWarn
}

// tightenQuality raises each quality floor to the best value ever seen, and
// never lowers it. It answers whether a floor moved.
//
// Mirror image of tightenSensitivity: improvement is locked in, so a later
// regression shows as a warning.
func tightenQuality(root string, metrics []Metric) (bool, error) {
	old, err := readQualityFloors(root)
	if err != nil {
		return false, err
	}

	next := object{}
	for _, key := range old.values.keys {
		next.set(key, old.values.get(key))
	}

	byKey := make(map[string]Metric, len(metrics))
	for _, metric := range metrics {
		byKey[metric.Key] = metric
	}

	changed := false
	for _, key := range qualityMetrics {
		metric, held := byKey[key]
		if !held {
			continue
		}
		percent := metricPercent(metric)
		if percent == nil {
			continue
		}
		measured := numberOf(percent)
		stored, known := old.values.get(key).(pyNum)
		if known && stored.Float() >= measured {
			continue
		}
		next.set(key, pyNum{f: measured})
		changed = true
	}

	if err := writeJSONFile(root, QualityBaseline, next); err != nil {
		return false, err
	}
	return changed, nil
}

// metricPercent answers the ratio percent a quality metric ratchets on, or nil.
func metricPercent(metric Metric) any {
	for _, key := range [...]string{"proof_density", "kill_rate", "overall"} {
		part, ok := metric.Data.get(key).(object)
		if !ok {
			continue
		}
		if percent := part.get("percent"); percent != nil {
			return percent
		}
	}
	return nil
}

// signedInt is one line of the composable sleep delta ledger.
var signedInt = regexp.MustCompile(`^[+-]?\d+$`)

// parseSleepBaseline answers the ceiling the committed delta ledger states: the
// SUM of every signed-integer line, with comments and blanks ignored. The
// second result is false when no integer line is present, which leaves the
// ratchet unenforced.
//
// internal/le/docwiring states the same function for the check that enforces the
// ratchet, and the two are held together by a case that runs the PYTHON owner
// (verify_wiring_docs.parse_sleep_baseline) over the same table. Importing
// docwiring here would be the other way to hold them together, and it costs
// more than it saves: that package registers a root command and claims gates in
// its init, so every binary carrying this metric would carry the doc-wiring
// tool as well.
func parseSleepBaseline(text string) (int, bool) {
	total := 0
	seen := false
	for line := range strings.SplitSeq(text, "\n") {
		stripped := strings.TrimSpace(line)
		if stripped == "" || strings.HasPrefix(stripped, "#") {
			continue
		}
		if !signedInt.MatchString(stripped) {
			continue
		}
		value, err := strconv.Atoi(stripped)
		if err != nil {
			continue
		}
		total += value
		seen = true
	}
	return total, seen
}

// writeJSONFile writes one artifact in the spelling the script wrote it in:
// indented, key-sorted, and closed by a newline.
func writeJSONFile(root, rel string, value any) error {
	body, err := dumpIndented(value)
	if err != nil {
		return collectErrorf("%s cannot be encoded: %w", rel, err)
	}
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return collectErrorf("%s cannot be created: %w", rel, err)
	}
	var tb textbuf.Buffer
	if err := os.WriteFile(path, []byte(tb.Str(body).Byte('\n').String()), 0o600); err != nil {
		return collectErrorf("%s cannot be written: %w", rel, err)
	}
	return nil
}

// parseObject decodes one JSON document that must be an object, keeping the
// int-versus-float distinction Python keeps.
func parseObject(raw []byte) (object, error) {
	value, err := parseValue(raw)
	if err != nil {
		return object{}, err
	}
	parsed, ok := value.(object)
	if !ok {
		return object{}, errNotAnObject
	}
	return parsed, nil
}

// errNotAnObject says a document that must be an object was something else.
var errNotAnObject = errors.New("the document is not a JSON object")

// parseValue decodes one JSON document into this package's own value types.
func parseValue(raw []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var decoded any
	if err := decoder.Decode(&decoded); err != nil {
		return nil, err
	}
	return convert(decoded), nil
}

// convert rewrites a decoded document into the value types this package
// compares and re-encodes.
func convert(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := object{}
		for _, key := range sortedAnyKeys(typed) {
			out.set(key, convert(typed[key]))
		}
		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, convert(item))
		}
		return out
	case json.Number:
		if whole, err := strconv.ParseInt(typed.String(), 10, 64); err == nil {
			return pyNum{isInt: true, i: whole}
		}
		fraction, err := strconv.ParseFloat(typed.String(), 64)
		if err != nil {
			return typed.String()
		}
		return pyNum{f: fraction}
	default:
		return value
	}
}

// sortedKeys answers a map's keys in code-point order.
func sortedKeys(values map[string]int) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// sortedAnyKeys answers a decoded object's keys in code-point order.
func sortedAnyKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// valueText renders one value the way Python's `str` renders it, which is what
// every rendered cell and every diagnostic uses.
func valueText(value any) string {
	switch typed := value.(type) {
	case nil:
		return "None"
	case bool:
		if typed {
			return "True"
		}
		return "False"
	case string:
		return typed
	case int:
		return textbuf.StringInt(int64(typed))
	case float64:
		return pyFloatRepr(typed)
	case pyNum:
		return typed.String()
	default:
		return "None"
	}
}

// numberOf answers a value's numeric worth, and zero for a value that has none.
func numberOf(value any) float64 {
	switch typed := value.(type) {
	case int:
		return float64(typed)
	case float64:
		return typed
	case pyNum:
		return typed.Float()
	case bool:
		if typed {
			return 1
		}
		return 0
	default:
		return 0
	}
}

// intOf answers an integer value stored in a record.
func intOf(value any) int {
	if held, ok := value.(int); ok {
		return held
	}
	return int(numberOf(value))
}

// quote renders a value the way Python's `repr` renders a string, which is the
// spelling every baseline diagnostic uses for a key.
func quote(value string) string {
	var tb textbuf.Buffer
	return tb.Byte('\'').Str(value).Byte('\'').String()
}

// intDict renders a measured pair the way Python renders a dict in a message.
func intDict(values map[string]int) string {
	var tb textbuf.Buffer
	tb.Byte('{')
	for index, key := range sortedKeys(values) {
		if index > 0 {
			tb.Str(", ")
		}
		tb.Str(quote(key)).Str(": ").Int(int64(values[key]))
	}
	return tb.Byte('}').String()
}
