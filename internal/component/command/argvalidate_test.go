// Design: docs/architecture/api/commands.md -- YANG-typed argument validation

package command

import (
	"errors"
	"regexp"
	"strings"
	"testing"
)

func TestValidateArgStringEnum(t *testing.T) {
	def := new(mustArgDef(NewEnumArg("mode", []string{"summary", "blocked", "full"}, ArgOptions{})))

	for _, v := range []string{"summary", "blocked", "full"} {
		if err := ValidateArgString(v, def); err != nil {
			t.Errorf("valid enum %q rejected: %v", v, err)
		}
	}

	if err := ValidateArgString("invalid", def); err == nil {
		t.Error("invalid enum value accepted")
	} else if !strings.Contains(err.Error(), "summary") {
		t.Errorf("error should list valid values: %v", err)
	}
}

func TestValidateArgStringUint(t *testing.T) {
	def := new(mustArgDef(NewUintArg("count", 32, nil, ArgOptions{})))

	if err := ValidateArgString("42", def); err != nil {
		t.Errorf("valid uint rejected: %v", err)
	}
	if err := ValidateArgString("0", def); err != nil {
		t.Errorf("zero uint rejected: %v", err)
	}
	if err := ValidateArgString("abc", def); err == nil {
		t.Error("non-numeric accepted as uint")
	}
	if err := ValidateArgString("-1", def); err == nil {
		t.Error("negative accepted as uint")
	}
}

func TestValidateArgStringUintRange(t *testing.T) {
	def := new(mustArgDef(NewUintArg("count", 32, []UintRange{{Min: 1, Max: 10000}}, ArgOptions{})))

	if err := ValidateArgString("1", def); err != nil {
		t.Errorf("min boundary rejected: %v", err)
	}
	if err := ValidateArgString("10000", def); err != nil {
		t.Errorf("max boundary rejected: %v", err)
	}
	if err := ValidateArgString("5000", def); err != nil {
		t.Errorf("mid-range rejected: %v", err)
	}
	if err := ValidateArgString("0", def); err == nil {
		t.Error("below-min accepted")
	}
	if err := ValidateArgString("10001", def); err == nil {
		t.Error("above-max accepted")
	}
}

func TestValidateArgStringUint64Boundary(t *testing.T) {
	def := new(mustArgDef(NewUintArg("limit", 64, nil, ArgOptions{})))

	if err := ValidateArgString("18446744073709551615", def); err != nil {
		t.Errorf("max uint64 rejected: %v", err)
	}
	if err := ValidateArgString("18446744073709551616", def); err == nil {
		t.Error("overflow uint64 accepted")
	}
}

func TestValidateArgStringUnion(t *testing.T) {
	def := new(mustArgDef(NewUnionArg("limit", []ArgDef{mustArgDef(NewUintArg("limit", 64, nil, ArgOptions{})), mustArgDef(NewEnumArg("limit", []string{"max"}, ArgOptions{}))}, ArgOptions{})))

	if err := ValidateArgString("1024", def); err != nil {
		t.Errorf("uint member rejected: %v", err)
	}
	if err := ValidateArgString("max", def); err != nil {
		t.Errorf("enum member rejected: %v", err)
	}
	if err := ValidateArgString("invalid", def); err == nil {
		t.Error("invalid union value accepted")
	} else if !strings.Contains(err.Error(), "max") {
		t.Errorf("union error should hint enum values: %v", err)
	}
}

func TestValidateArgStringPattern(t *testing.T) {
	def := new(mustArgDef(NewStringArg("timeout", nil, []*regexp.Regexp{regexp.MustCompile(`^\d+[smh]?$`)}, ArgOptions{})))

	if err := ValidateArgString("30s", def); err != nil {
		t.Errorf("valid pattern rejected: %v", err)
	}
	if err := ValidateArgString("100", def); err != nil {
		t.Errorf("numeric-only pattern rejected: %v", err)
	}
	if err := ValidateArgString("abc", def); err == nil {
		t.Error("non-matching pattern accepted")
	}
}

// TestValidateArgStringLength: a string argument carrying a YANG length is
// refused below and above its bound and accepted inside it, and the refusal
// names the bound. Length counts characters, so a two-byte character is one.
//
// VALIDATES: RFC 7950 Section 9.4.4 length on a command argument.
// PREVENTS: a value past the declared bound reaching the handler.
func TestValidateArgStringLength(t *testing.T) {
	def := new(mustArgDef(NewStringArg("name", []UintRange{{Min: 2, Max: 4}}, nil, ArgOptions{})))

	for _, v := range []string{"ab", "abcd", "\u00e9\u00e9\u00e9\u00e9"} {
		if err := ValidateArgString(v, def); err != nil {
			t.Errorf("in-bounds %q rejected: %v", v, err)
		}
	}
	for _, v := range []string{"a", "abcde", ""} {
		err := ValidateArgString(v, def)
		if err == nil {
			t.Errorf("out-of-bounds %q accepted", v)
			continue
		}
		if !strings.Contains(err.Error(), "2..4") {
			t.Errorf("error %q does not name the bound 2..4", err)
		}
	}
}

// TestValidateArgStringDisjointLengths: a length of several ranges accepts a
// value in any range and refuses one in a gap, naming every range.
//
// VALIDATES: RFC 7950 Section 9.4.4 "Multiple values or ranges can be given".
// PREVENTS: only the first range of a disjoint length being enforced.
func TestValidateArgStringDisjointLengths(t *testing.T) {
	def := new(mustArgDef(NewStringArg("key", []UintRange{{Min: 1, Max: 2}, {Min: 5, Max: 6}}, nil, ArgOptions{})))

	for _, v := range []string{"a", "ab", "abcde", "abcdef"} {
		if err := ValidateArgString(v, def); err != nil {
			t.Errorf("in-range %q rejected: %v", v, err)
		}
	}
	err := ValidateArgString("abc", def)
	if err == nil {
		t.Fatal("value in the gap between ranges accepted")
	}
	if !strings.Contains(err.Error(), "1..2 | 5..6") {
		t.Errorf("error %q does not name both ranges", err)
	}
}

// TestValidateArgStringAllPatterns: every pattern a string argument carries
// must match, not only the first.
//
// VALIDATES: RFC 7950 Section 9.4.5 "all such expressions have to match".
// PREVENTS: a second (or inherited) pattern constraining nothing.
func TestValidateArgStringAllPatterns(t *testing.T) {
	def := new(mustArgDef(NewStringArg("id", nil, []*regexp.Regexp{
		regexp.MustCompile(`^[a-z0-9]+$`),
		regexp.MustCompile(`^[a-z].*$`),
	}, ArgOptions{})))
	if err := ValidateArgString("a1", def); err != nil {
		t.Errorf("value matching both patterns rejected: %v", err)
	}
	if err := ValidateArgString("1a", def); err == nil {
		t.Error("value failing the second pattern accepted")
	}
}

func TestValidateArgStringMaxLength(t *testing.T) {
	def := new(mustArgDef(NewStringArg("x", nil, nil, ArgOptions{})))
	long := strings.Repeat("a", maxArgLength+1)
	if err := ValidateArgString(long, def); err == nil {
		t.Error("over-length arg accepted")
	}
}

// TestValidateArgStringUint64AcceptsMax: a 64-bit definition accepts the
// largest uint64. A width of zero, which once defaulted to 64, is now refused
// by NewUintArg (TestArgDefConstructorRefuses, "width 0").
func TestValidateArgStringUint64AcceptsMax(t *testing.T) {
	def := new(mustArgDef(NewUintArg("x", 64, nil, ArgOptions{})))
	if err := ValidateArgString("42", def); err != nil {
		t.Errorf("42 refused: %v", err)
	}
	if err := ValidateArgString("18446744073709551615", def); err != nil {
		t.Errorf("max uint64 refused: %v", err)
	}
}

func TestValidateArgStringDisjointRange(t *testing.T) {
	def := new(mustArgDef(NewUintArg("x", 32, []UintRange{{Min: 1, Max: 100}, {Min: 200, Max: 300}}, ArgOptions{})))
	if err := ValidateArgString("50", def); err != nil {
		t.Errorf("value in first range rejected: %v", err)
	}
	if err := ValidateArgString("250", def); err != nil {
		t.Errorf("value in second range rejected: %v", err)
	}
	if err := ValidateArgString("150", def); err == nil {
		t.Error("value in gap between ranges accepted")
	}
}

func TestValidateArgStringSanity(t *testing.T) {
	def := new(mustArgDef(NewStringArg("host", nil, nil, ArgOptions{})))
	if err := ValidateArgString("192.168.1.1", def); err != nil {
		t.Errorf("plain string rejected: %v", err)
	}
}

// TestValidateArgStringRejectsInvalidKind proves a malformed internal argument
// definition cannot authorize an arbitrary raw token.
func TestValidateArgStringRejectsInvalidKind(t *testing.T) {
	defer func() {
		if got := recover(); got != "BUG: invalid command argument kind" {
			t.Fatalf("panic = %v, want invalid argument kind assertion", got)
		}
	}()
	// Zero-contract literal: only a same-package test can write a kind no
	// constructor produces, and it marks the value constructed to reach the
	// kind switch rather than the zero refusal.
	err := ValidateArgString("anything", &ArgDef{name: "x", kind: ArgKind(255), constructed: true})
	t.Fatalf("invalid argument kind returned %v instead of asserting", err)
}

// TestZeroArgDefRefused proves the zero ArgDef is refused rather than read as
// an unrestricted string (D-4).
//
// VALIDATES: ValidateArgs and ValidateArgString refuse a definition no
// constructor built, with an error wrapping ErrArgDef, even beside a valid one.
// PREVENTS: an ArgDef{} accepting every token, which is fail-open.
func TestZeroArgDefRefused(t *testing.T) {
	if _, err := ValidateArgs([]string{"anything"}, []ArgDef{{}}, nil); !errors.Is(err, ErrArgDef) {
		t.Errorf("ValidateArgs over a zero definition: err = %v, want ErrArgDef", err)
	}
	valid := mustArgDef(NewStringArg("label", nil, nil, ArgOptions{}))
	if _, err := ValidateArgs([]string{"label", "x"}, []ArgDef{valid, {}}, nil); !errors.Is(err, ErrArgDef) {
		t.Errorf("ValidateArgs with a zero definition beside a valid one: err = %v, want ErrArgDef", err)
	}
	if err := ValidateArgString("anything", &ArgDef{}); !errors.Is(err, ErrArgDef) {
		t.Errorf("ValidateArgString over a zero definition: err = %v, want ErrArgDef", err)
	}
}
