package rfc

import (
	"strings"
	"testing"
)

// keyTextsSource declares a package-level function beside two methods of the
// same name, the shape of flowspec/unmatchable.go, plus two methods that share
// a name with no package-level function, and one method whose name is unique.
const keyTextsSource = `package flowspec

// CheckMatchable is the package-level rule.
func CheckMatchable(has func(int) bool) error {
	return nil
}

// CheckMatchable asks the rule about a FlowSpec.
func (f *FlowSpec) CheckMatchable() error {
	return CheckMatchable(nil)
}

func (f *FlowSpecVPN) CheckMatchable() error {
	return f.flowSpec.CheckMatchable()
}

func (f *FlowSpec) WriteTo(buf []byte, off int) int { return 0 }

func (g Generic[T]) WriteTo(buf []byte, off int) int { return 1 }

func (f *FlowSpec) Len() int { return 0 }
`

// TestKeyTextsDistinguishesFunctionFromMethods checks the producer-key resolver:
// a bare name shared by a package-level function and methods names the function,
// `Type.Method` names that receiver's method, a bare name two methods share with
// no function is still refused, and the minted key resolves back to the text it
// was minted from.
func TestKeyTextsDistinguishesFunctionFromMethods(t *testing.T) {
	index := newScopeIndex()
	for _, tc := range []struct {
		symbol string
		want   []string // a fragment each resolved text must hold, in order
	}{
		{"CheckMatchable", []string{"func CheckMatchable(has"}},
		{"FlowSpec.CheckMatchable", []string{"func (f *FlowSpec) CheckMatchable"}},
		{"FlowSpecVPN.CheckMatchable", []string{"func (f *FlowSpecVPN) CheckMatchable"}},
		{"WriteTo", []string{"(f *FlowSpec) WriteTo", "(g Generic[T]) WriteTo"}},
		{"Generic.WriteTo", []string{"(g Generic[T]) WriteTo"}},
		{"Len", []string{"(f *FlowSpec) Len"}},
		{"Missing.CheckMatchable", nil},
	} {
		got := index.keyTexts(keyTextsSource, tc.symbol)
		if len(got) != len(tc.want) {
			t.Fatalf("keyTexts(%q) = %d texts, want %d", tc.symbol, len(got), len(tc.want))
		}
		for i, fragment := range tc.want {
			if !strings.Contains(got[i], fragment) {
				t.Errorf("keyTexts(%q)[%d] = %q, want it to hold %q", tc.symbol, i, got[i], fragment)
			}
		}
	}

	source := keyTextsSource
	lookup := func(string) *string { return &source }
	for _, key := range []string{"unmatchable.go::CheckMatchable", "unmatchable.go::FlowSpec.CheckMatchable"} {
		if _, ok, err := resolveKeyText(lookup, index, key, "test"); err != nil || !ok {
			t.Errorf("resolveKeyText(%q) = ok %v, err %v; want the one unit", key, ok, err)
		}
	}
	if _, ok, err := resolveKeyText(lookup, index, "unmatchable.go::WriteTo", "test"); err != nil || ok {
		t.Errorf("resolveKeyText of a name two methods share = ok %v, err %v; want a refusal", ok, err)
	}

	for _, tc := range []struct {
		line int
		want string
	}{
		{5, "CheckMatchable"},
		{10, "FlowSpec.CheckMatchable"},
		{14, "FlowSpecVPN.CheckMatchable"},
		{17, "FlowSpec.WriteTo"},
		{21, "Len"},
	} {
		if got := functionAtLine(index, keyTextsSource, tc.line); got != tc.want {
			t.Errorf("functionAtLine(%d) = %q, want %q", tc.line, got, tc.want)
		}
	}
}
