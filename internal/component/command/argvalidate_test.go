// Design: docs/architecture/api/commands.md -- YANG-typed argument validation

package command

import (
	"regexp"
	"strings"
	"testing"
)

func TestValidateArgStringEnum(t *testing.T) {
	def := &ArgDef{
		Name:       "mode",
		Kind:       ArgEnum,
		EnumValues: []string{"summary", "blocked", "full"},
	}

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
	def := &ArgDef{
		Name:     "count",
		Kind:     ArgUint,
		UintBits: 32,
	}

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
	def := &ArgDef{
		Name:     "count",
		Kind:     ArgUint,
		UintBits: 32,
		Ranges:   []UintRange{{Min: 1, Max: 10000}},
	}

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
	def := &ArgDef{
		Name:     "limit",
		Kind:     ArgUint,
		UintBits: 64,
	}

	if err := ValidateArgString("18446744073709551615", def); err != nil {
		t.Errorf("max uint64 rejected: %v", err)
	}
	if err := ValidateArgString("18446744073709551616", def); err == nil {
		t.Error("overflow uint64 accepted")
	}
}

func TestValidateArgStringUnion(t *testing.T) {
	def := &ArgDef{
		Name: "limit",
		Kind: ArgUnion,
		UnionDefs: []ArgDef{
			{Kind: ArgUint, UintBits: 64},
			{Kind: ArgEnum, EnumValues: []string{"max"}},
		},
	}

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
	def := &ArgDef{
		Name:     "timeout",
		Kind:     ArgString,
		Patterns: []*regexp.Regexp{regexp.MustCompile(`^\d+[smh]?$`)},
	}

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
	def := &ArgDef{Name: "name", Kind: ArgString, Lengths: []UintRange{{Min: 2, Max: 4}}}

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
	def := &ArgDef{Name: "key", Kind: ArgString, Lengths: []UintRange{{Min: 1, Max: 2}, {Min: 5, Max: 6}}}

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
	def := &ArgDef{Name: "id", Kind: ArgString, Patterns: []*regexp.Regexp{
		regexp.MustCompile(`^[a-z0-9]+$`),
		regexp.MustCompile(`^[a-z].*$`),
	}}
	if err := ValidateArgString("a1", def); err != nil {
		t.Errorf("value matching both patterns rejected: %v", err)
	}
	if err := ValidateArgString("1a", def); err == nil {
		t.Error("value failing the second pattern accepted")
	}
}

func TestValidateArgStringMaxLength(t *testing.T) {
	def := &ArgDef{Name: "x", Kind: ArgString}
	long := strings.Repeat("a", maxArgLength+1)
	if err := ValidateArgString(long, def); err == nil {
		t.Error("over-length arg accepted")
	}
}

func TestValidateArgStringUintBitsZeroDefaultsTo64(t *testing.T) {
	def := &ArgDef{Name: "x", Kind: ArgUint}
	if err := ValidateArgString("42", def); err != nil {
		t.Errorf("UintBits=0 should default to 64-bit: %v", err)
	}
	if err := ValidateArgString("18446744073709551615", def); err != nil {
		t.Errorf("UintBits=0 should accept max uint64: %v", err)
	}
}

func TestValidateArgStringDisjointRange(t *testing.T) {
	def := &ArgDef{
		Name:     "x",
		Kind:     ArgUint,
		UintBits: 32,
		Ranges:   []UintRange{{Min: 1, Max: 100}, {Min: 200, Max: 300}},
	}
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
	def := &ArgDef{Name: "host", Kind: ArgString}
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
	err := ValidateArgString("anything", &ArgDef{Kind: ArgKind(255)})
	t.Fatalf("invalid argument kind returned %v instead of asserting", err)
}
