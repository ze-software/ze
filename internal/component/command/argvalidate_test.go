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
		Name:    "timeout",
		Kind:    ArgString,
		Pattern: regexp.MustCompile(`^\d+[smh]?$`),
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
