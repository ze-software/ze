// Design: docs/architecture/config/environment.md — values ze could not use
// Related: env.go — the typed getters that record a value they could not parse
// Related: registry.go — EnvEntry.Type, the type a set value is judged against
//
// A typed getter that cannot parse a value answers its caller's default, so
// the process runs on. That default is a silent answer unless it is recorded:
// the operator who exported ze.bgp.openwait=5 seconds sees ze behave as if
// nothing were set. This file keeps that record, and judges every registered
// entry against its type, so the doctor check that reads InvalidValues
// (internal/component/doctor, doctor-env-invalid-value) names the key, the
// value, the type and what ze used. A secret key's value never leaves the
// package: its record carries Secret and no Value.

package env

import (
	"maps"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// InvalidKind names why ze could not use a value it found in the environment.
type InvalidKind uint8

const (
	// InvalidKindUnspecified is the zero value; no record carries it.
	InvalidKindUnspecified InvalidKind = iota
	// InvalidUnparsable: the value is not of its type, so ze used a default.
	InvalidUnparsable
	// InvalidOutOfRange: the value parses but lies outside the range its
	// reader accepts, so ze used another value (a bound, or the default).
	InvalidOutOfRange
)

// InvalidValue is one environment variable ze could not use as set.
type InvalidValue struct {
	Key     string      // canonical dot-notation key
	Type    string      // the type the value was read or judged as
	Value   string      // the value as set; empty when Secret
	Secret  bool        // the key is registered Secret, so Value is withheld
	Kind    InvalidKind // why the value was not used
	Read    bool        // a reader read the value; false when only judged
	Used    string      // what ze used: the reader's default or bound, or for an unread entry its registered default; empty when neither is known
	Range   string      // for InvalidOutOfRange, the range the reader accepts ("1 to 10000"); empty otherwise
	Example string      // a value of Type to put in the fix
}

// invalidRead is one recorded read: the record, and the raw value it judged,
// kept even for a secret key so InvalidValues can tell whether the key still
// holds that value. The raw value never leaves the package.
type invalidRead struct {
	value  string
	record InvalidValue
}

// typeJudge parses one declared type and names a value it accepts.
type typeJudge struct {
	fits    func(string) bool
	example string
}

// typeJudges holds a judge for every declared type a value can fail. A type
// with no judge (TypeString, which any text satisfies) is never judged.
var typeJudges = map[string]typeJudge{
	TypeBool: {fits: func(s string) bool {
		_, recognized := parseBool(s)
		return recognized
	}, example: "true"},
	TypeDuration: {fits: func(s string) bool {
		_, err := time.ParseDuration(s)
		return err == nil
	}, example: "30s"},
	TypeInt: {fits: func(s string) bool {
		_, err := strconv.Atoi(s)
		return err == nil
	}, example: "10"},
	TypeInt64: {fits: func(s string) bool {
		_, err := strconv.ParseInt(s, 10, 64)
		return err == nil
	}, example: "10"},
}

// invalidReads holds what each read or clamp recorded, one entry per
// canonical key: a key is recorded only after Get accepted it as registered,
// so the map is bounded by the keys the binary's code reads. Guarded by
// invalidMu.
var (
	invalidMu    sync.Mutex
	invalidReads = make(map[string]invalidRead)
)

// InvalidValues answers every environment variable ze could not use as set,
// sorted by key: each read a typed getter could not parse, each out-of-range
// value a reader recorded with RecordOutOfRange, and each registered entry
// whose set value its Type cannot parse though no reader has read it yet. A
// read record is answered only while the key still holds the value it
// judged: once Set (or a rebuilt cache) gives the key another value, that
// current value is judged instead, so a corrected key reads clean. An empty
// answer means every set value of a judged type parses; a key whose Type has
// no judge (TypeString) is never listed. Judging peeks at the cache and does
// not clear a secret from the OS environment, which only Get does.
// Safe for concurrent use.
func InvalidValues() []InvalidValue {
	invalidMu.Lock()
	reads := maps.Clone(invalidReads)
	invalidMu.Unlock()

	found := make(map[string]InvalidValue, len(reads))
	for key, read := range reads {
		entry, listed := registered[key]
		if !listed {
			// A key under a prefix pattern has no entry of its own.
			entry = EnvEntry{Key: key}
		}
		if peek(&entry) != read.value {
			continue
		}
		found[key] = read.record
	}

	for _, entry := range AllEntries() {
		if _, recorded := found[entry.Key]; recorded {
			continue
		}
		// A prefix pattern ("ze.log.<subsystem>") names no variable to judge.
		if strings.Contains(entry.Key, "<") {
			continue
		}
		judge, judged := typeJudges[entry.Type]
		if !judged {
			continue
		}
		value := peek(&entry)
		if value == "" {
			continue
		}
		if judge.fits(value) {
			continue
		}
		used := ""
		if judge.fits(entry.Default) {
			used = entry.Default
		}
		found[entry.Key] = newInvalidValue(entry.Key, entry.Type, value, InvalidUnparsable, used, false)
	}

	values := slices.Collect(maps.Values(found))
	slices.SortFunc(values, func(a, b InvalidValue) int { return strings.Compare(a.Key, b.Key) })
	return values
}

// RecordOutOfRange records that the reader of key accepts only minimum to
// maximum and used used instead, because the environment's value of key lies
// outside that range: below it (a cap of 0 answered with the default) or
// above it (a cap clamped to its maximum). It judges the environment's own
// value: when key holds no integer outside the range (unset, within it, or
// unparsable, which the getter already recorded), nothing is recorded, so a
// caller bounding a value that did not come from the environment records no
// false row. Aborts on an unregistered key, as Get does. Safe for concurrent
// use.
func RecordOutOfRange(key string, minimum, maximum, used int64) {
	canonical := resolveAlias(key)
	mustBeRegistered(canonical)
	entry, listed := registered[canonical]
	if !listed {
		// A key under a prefix pattern has no entry of its own.
		entry = EnvEntry{Key: canonical, Type: TypeInt64}
	}
	value := peek(&entry)
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return
	}
	if n >= minimum && n <= maximum {
		return
	}
	var usedText, rangeText textbuf.Buffer
	rangeText.Int(minimum).Str(" to ").Int(maximum)
	record := newInvalidValue(canonical, entry.Type, value, InvalidOutOfRange, usedText.Int(used).String(), true)
	record.Range = rangeText.String()
	storeInvalidRead(canonical, value, record)
}

// recordUnparsable records that a typed getter read value for key, could not
// parse it as typ, and answered used, its caller's default.
func recordUnparsable(key, typ, value, used string) {
	canonical := resolveAlias(key)
	storeInvalidRead(canonical, value, newInvalidValue(canonical, typ, value, InvalidUnparsable, used, true))
}

// storeInvalidRead stores one read's outcome and the value it judged under
// its canonical key.
func storeInvalidRead(canonical, value string, record InvalidValue) {
	invalidMu.Lock()
	invalidReads[canonical] = invalidRead{value: value, record: record}
	invalidMu.Unlock()
}

// newInvalidValue builds a record, withholding a secret key's value and
// choosing the fix example: what ze used when that is a value of typ, else a
// value typ accepts.
func newInvalidValue(canonical, typ, value string, kind InvalidKind, used string, read bool) InvalidValue {
	record := InvalidValue{Key: canonical, Type: typ, Kind: kind, Read: read, Used: used}
	if IsSecret(canonical) {
		record.Secret = true
	} else {
		record.Value = value
	}
	judge := typeJudges[typ]
	record.Example = judge.example
	if used != "" && judge.fits != nil && judge.fits(used) {
		record.Example = used
	}
	return record
}

// peek answers the cached value of entry under its key, else under its first
// set alias, the spellings Get reads. Unlike Get it never clears a secret from
// the OS environment.
func peek(entry *EnvEntry) string {
	ensureCache()
	cacheMu.RLock()
	defer cacheMu.RUnlock()
	if v := cache[normalize(entry.Key)]; v != "" {
		return v
	}
	for _, alias := range entry.Aliases {
		if v := cache[normalize(alias)]; v != "" {
			return v
		}
	}
	return ""
}

// resetInvalidReads forgets every recorded read. ResetCache calls it, so a
// test that rebuilds the environment starts from no record.
func resetInvalidReads() {
	invalidMu.Lock()
	invalidReads = make(map[string]invalidRead)
	invalidMu.Unlock()
}
