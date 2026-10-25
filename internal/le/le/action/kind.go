// Design: docs/architecture/core-design.md -- an le area, as one command
// Related: leaction.go -- the Parameter that declares its Kind
//
// The closed set of value kinds an le keyword can take. Help, completion, the
// manifest and `le help ai` read the kind a parameter declares, so the set is
// declared once, here, each kind with its meaning and its syntax check.
package leaction

import (
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Kind says what a parameter's value IS, so every reader of the grammar (a
// help page, a completion callback, a prompt, a JSON consumer) learns it from
// the declaration rather than from the action body.
//
// The zero value is KindUnspecified, which New refuses on every parameter
// that is not retired, so a parameter whose author said nothing never reaches
// a reader (docs/contributing/ze-go-style.md, "Types that cannot lie").
type Kind uint8

const (
	// KindUnspecified is what a Parameter holds before its author writes the
	// field. Only a retired keyword may keep it, because nothing renders one.
	KindUnspecified Kind = iota
	// KindSwitch is a keyword with no value: its presence is its meaning.
	KindSwitch
	// KindString is free text the action body reads as it is.
	KindString
	// KindEnum is one member of the parameter's declared Values.
	KindEnum
	// KindList is a comma-separated list. With declared Values, every item is
	// one of them.
	KindList
	// KindUint is an unsigned decimal integer that fits 64 bits.
	KindUint
	// KindDuration is a Go duration such as 90s or 2h30m.
	KindDuration
	// KindPath is a file or directory path. It may name one that does not
	// exist yet, because an output path usually does not.
	KindPath
	// KindRevision is a git revision: a commit, a branch, a tag, HEAD~2.
	KindRevision
	// KindSpec is the path of a spec file under plan/.
	KindSpec
	// KindLabel is an identifier: one token with no whitespace.
	KindLabel
	// KindArea is the words of an le command, such as `spec claim`.
	KindArea
	// KindWords is the words of a whole le command line the action runs.
	KindWords
)

// kindDeclaration is one row of the kind table: the name a reader sees, the
// one line that says what the value is, and the syntax check.
type kindDeclaration struct {
	name    string
	meaning string
	// check refuses a value whose SYNTAX is wrong. It never asks whether a
	// path exists or a revision resolves: an output path does not exist yet,
	// and the body that uses the value is the one that can judge it (R-17).
	check func(value string, values []string) error
}

// errValue is the cause every syntax refusal wraps, so a caller can tell a
// malformed value from any other error.
var errValue = errors.New("malformed value")

// kinds is the closed set, indexed by Kind. KindUnspecified has no row, so
// declarationOf refuses it.
var kinds = map[Kind]kindDeclaration{
	KindSwitch:   {"switch", "a keyword alone; its presence is the whole meaning", checkNoValue},
	KindString:   {"string", "free text, read as typed", checkAnything},
	KindEnum:     {"enum", "one of the listed values", checkMember},
	KindList:     {"list", "comma-separated items, each one of the listed values when values are listed", checkList},
	KindUint:     {"uint", "an unsigned decimal integer of at most 64 bits", checkUint},
	KindDuration: {"duration", "a Go duration such as 90s or 2h30m", checkDuration},
	KindPath:     {"path", "a file or directory path, which may not exist yet", checkNotEmpty},
	KindRevision: {"revision", "a git revision: a commit, a branch, a tag or HEAD~N", checkToken},
	KindSpec:     {"spec", "the path of a spec file under plan/", checkNotEmpty},
	KindLabel:    {"label", "an identifier: one token with no whitespace", checkToken},
	KindArea:     {"area", "the words of an le command", checkNotEmpty},
	KindWords:    {"words", "a whole le command line, run as le <words>", checkNotEmpty},
}

// Kinds answers every declared kind, in declaration order, so a reader of the
// set (a test, `le help ai`) never holds a copy of it.
func Kinds() []Kind {
	out := make([]Kind, 0, len(kinds))
	for kind := range kinds {
		out = append(out, kind)
	}
	slices.Sort(out)
	return out
}

// declarationOf answers the kind's row. A kind outside the set is a Ze defect,
// because only this package's constants reach a Parameter.
func declarationOf(kind Kind) kindDeclaration {
	declaration, declared := kinds[kind]
	if !declared {
		panic("BUG: leaction: a parameter kind is not in the declared set")
	}
	return declaration
}

// String answers the kind's name, for a person and for the manifest.
func (k Kind) String() string {
	if k == KindUnspecified {
		return "unspecified"
	}
	return declarationOf(k).name
}

// Meaning answers the one line that says what a value of this kind is.
func (k Kind) Meaning() string {
	return declarationOf(k).meaning
}

// Check refuses a value whose syntax this kind does not accept. values are
// the parameter's declared Values, read by enum and list.
func (k Kind) Check(value string, values []string) error {
	return declarationOf(k).check(value, values)
}

// MarshalJSON publishes the kind by name, which is what a manifest reader
// branches on.
func (k Kind) MarshalJSON() ([]byte, error) {
	return json.Marshal(k.String())
}

// checkNoValue refuses any value, because a switch takes none.
func checkNoValue(value string, _ []string) error {
	if value != "" {
		return errors.Join(errValue, errors.New("a switch takes no value, got "+strconv.Quote(value)))
	}
	return nil
}

// checkAnything accepts every value: free text has no syntax.
func checkAnything(string, []string) error {
	return nil
}

// checkNotEmpty refuses only the empty value, which names nothing.
func checkNotEmpty(value string, _ []string) error {
	if value == "" {
		return errors.Join(errValue, errors.New("the value is empty"))
	}
	return nil
}

// checkToken refuses an empty value and one holding whitespace.
func checkToken(value string, values []string) error {
	if err := checkNotEmpty(value, values); err != nil {
		return err
	}
	if strings.ContainsAny(value, " \t\n") {
		return errors.Join(errValue, errors.New(strconv.Quote(value)+" holds whitespace"))
	}
	return nil
}

// checkMember refuses a value that is not one of values.
func checkMember(value string, values []string) error {
	if slices.Contains(values, value) {
		return nil
	}
	return errors.Join(errValue, errors.New(strconv.Quote(value)+" is not one of "+strings.Join(values, ", ")))
}

// checkList refuses an empty item, and an item outside values when values
// are declared.
func checkList(value string, values []string) error {
	for item := range strings.SplitSeq(value, ",") {
		if item == "" {
			return errors.Join(errValue, errors.New("the list "+strconv.Quote(value)+" holds an empty item"))
		}
		if len(values) == 0 {
			continue
		}
		if err := checkMember(item, values); err != nil {
			return err
		}
	}
	return nil
}

// checkUint refuses a value that is not an unsigned decimal of 64 bits.
func checkUint(value string, _ []string) error {
	if _, err := strconv.ParseUint(value, 10, 64); err != nil {
		return errors.Join(errValue, err)
	}
	return nil
}

// checkDuration refuses a value time.ParseDuration does not read.
func checkDuration(value string, _ []string) error {
	if _, err := time.ParseDuration(value); err != nil {
		return errors.Join(errValue, err)
	}
	return nil
}
