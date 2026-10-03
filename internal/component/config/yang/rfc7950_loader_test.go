package yang

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// loadModuleTexts feeds every module text to a fresh Loader through
// AddModuleFromText, in order, and returns the first refusal from either the
// parse or Resolve. A nil result means Ze accepted the whole set.
func loadModuleTexts(t *testing.T, texts ...string) error {
	t.Helper()
	loader := NewLoader()
	for i, text := range texts {
		if err := loader.AddModuleFromText(moduleFileName(text, i), text); err != nil {
			return err
		}
	}
	return loader.Resolve()
}

// moduleFileName derives the file name goyang expects from the module
// statement, so a set of modules can import each other by name.
func moduleFileName(text string, i int) string {
	fields := strings.Fields(text)
	for j, f := range fields {
		if f == "module" || f == "submodule" {
			if j+1 < len(fields) {
				return fields[j+1] + ".yang"
			}
		}
	}
	return "m" + string(rune('a'+i)) + ".yang"
}

const rfc7950Base = `module ext { namespace "urn:ext"; prefix ext;
  typedef port { type uint16; }
  grouping g { leaf x { type string; } }
  extension mark { argument text; }
  container top { leaf a { type string; } }
}`

// moduleLoadCase is one module set the loader MUST refuse (wantErr) or MUST
// load (!wantErr), with the id and polarity it proves.
type moduleLoadCase struct {
	name    string
	texts   []string
	wantErr string
}

// TestRFC7950ModuleLoadRefused feeds the loader a module set that violates a
// rule of RFC 7950 and expects the refusal to name the fault. Only the cases
// tagged with an RFC requirement prove a whole row; the untagged cases pin the
// part of a row goyang does enforce while the rest of that row is a gap
// (plan/spec-config-yang-loader-structural-checks.md). The rules are enforced by
// goyang at load; the boundary Ze owns is Loader.AddModuleFromText and
// Loader.Resolve, and this test proves that boundary refuses.
func TestRFC7950ModuleLoadRefused(t *testing.T) {
	cases := []moduleLoadCase{
		{name: "external reference without import", texts: []string{rfc7950Base,
			`module m { namespace "urn:m"; prefix m; leaf p { type ext:port; } }`}, wantErr: "unknown prefix"},
		// RFC requirement: RFC7950-6.1.3-1 negative — a quoted string whose backslash is followed by a character outside n, t, " and \ is refused.
		{name: "bad escape", texts: []string{`module m { namespace "urn:m"; prefix m; leaf a { type string; description "bad \q escape"; } }`}, wantErr: "escape"},
		{name: "duplicate leaf", texts: []string{`module m { namespace "urn:m"; prefix m; container c { leaf a { type string; } leaf a { type uint8; } } }`}, wantErr: "duplicate"},
		// RFC requirement: RFC7950-6.5-1 negative — a reference to an external typedef without its prefix is refused as an unknown type.
		{name: "external typedef without prefix", texts: []string{rfc7950Base,
			`module m { namespace "urn:m"; prefix m; import ext { prefix ext; } leaf p { type port; } }`}, wantErr: "unknown type"},
		{name: "import with own prefix", texts: []string{rfc7950Base,
			`module m { namespace "urn:m"; prefix m; import ext { prefix m; } leaf a { type m:port; } }`}, wantErr: "unknown type"},
		{name: "typedef without type", texts: []string{`module m { namespace "urn:m"; prefix m; typedef t { description "no type"; } leaf a { type t; } }`}, wantErr: "type"},
		// RFC requirement: RFC7950-7.6.3-1 negative — a leaf without a type substatement is refused.
		{name: "leaf without type", texts: []string{`module m { namespace "urn:m"; prefix m; leaf a { description "no type"; } }`}, wantErr: "type"},
		{name: "duplicate case identifier", texts: []string{`module m { namespace "urn:m"; prefix m; choice c { case x { leaf a { type string; } } case x { leaf b { type uint8; } } } }`}, wantErr: "duplicate"},
		{name: "action inside rpc", texts: []string{`module m { namespace "urn:m"; prefix m; rpc r { action a { } } }`}, wantErr: "action"},
		{name: "notification inside rpc", texts: []string{`module m { namespace "urn:m"; prefix m; rpc r { notification n { } } }`}, wantErr: "notification"},
		{name: "augment target missing", texts: []string{`module m { namespace "urn:m"; prefix m; container c { leaf a { type string; } } augment "/nope" { leaf b { type string; } } }`}, wantErr: "not found"},
		{name: "identity base undefined", texts: []string{`module m { namespace "urn:m"; prefix m; identity i { base nope; } leaf a { type string; } }`}, wantErr: "nope"},
		// RFC requirement: RFC7950-7.19-1 negative — an extension whose substatement is not a YANG statement is refused.
		{name: "extension with non-YANG substatement", texts: []string{`module m { namespace "urn:m"; prefix m; extension e { bogus "x"; } leaf a { type string; } }`}, wantErr: "bogus"},
		{name: "range descending", texts: []string{`module m { namespace "urn:m"; prefix m; leaf a { type uint8 { range "10..5"; } } }`}, wantErr: "range"},
		{name: "range bound not a number", texts: []string{`module m { namespace "urn:m"; prefix m; leaf a { type uint8 { range "a..b"; } } }`}, wantErr: "range"},
		// RFC requirement: RFC7950-9.3.4-1 negative — a decimal64 type without fraction-digits is refused, and one with fraction-digits outside 1..18 is refused.
		{name: "decimal64 without fraction-digits", texts: []string{`module m { namespace "urn:m"; prefix m; leaf a { type decimal64; } }`}, wantErr: "[1..18]"},
		{name: "decimal64 fraction-digits 19", texts: []string{`module m { namespace "urn:m"; prefix m; leaf a { type decimal64 { fraction-digits 19; } } }`}, wantErr: "out of range [1..18]"},
		// RFC requirement: RFC7950-9.4.4-1 negative — a negative length bound, and length parts that are not ascending, are each refused.
		{name: "length negative", texts: []string{`module m { namespace "urn:m"; prefix m; leaf a { type string { length "-1..5"; } } }`}, wantErr: "length"},
		{name: "length descending", texts: []string{`module m { namespace "urn:m"; prefix m; leaf a { type string { length "5..1"; } } }`}, wantErr: "length"},
		{name: "enum duplicate name", texts: []string{`module m { namespace "urn:m"; prefix m; leaf a { type enumeration { enum x; enum x; } } }`}, wantErr: "already assigned"},
		// RFC requirement: RFC7950-9.6.4.2-1 negative — an enum after one valued 2147483647 that carries no value, two enums with one value, and a value outside the int32 range are each refused.
		{name: "enum after max without value", texts: []string{`module m { namespace "urn:m"; prefix m; leaf a { type enumeration { enum x { value 2147483647; } enum y; } } }`}, wantErr: "value"},
		{name: "enum duplicate value", texts: []string{`module m { namespace "urn:m"; prefix m; leaf a { type enumeration { enum x { value 1; } enum y { value 1; } } } }`}, wantErr: "conflict on value"},
		{name: "enum value beyond int32", texts: []string{`module m { namespace "urn:m"; prefix m; leaf a { type enumeration { enum x { value 2147483648; } } } }`}, wantErr: "too large"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := loadModuleTexts(t, tc.texts...)
			require.Error(t, err, "loader must refuse the violating module set")
			assert.Contains(t, strings.ToLower(err.Error()), tc.wantErr)
		})
	}
}

// TestRFC7950ModuleLoadAccepted feeds the loader the conforming counterpart of
// each violation TestRFC7950ModuleLoadRefused refuses and expects the set to
// load through AddModuleFromText and Resolve with no error, so the refusals
// above are pinned to the fault and not to the loader refusing everything.
func TestRFC7950ModuleLoadAccepted(t *testing.T) {
	cases := []moduleLoadCase{
		{name: "imported external reference", texts: []string{rfc7950Base,
			`module m { namespace "urn:m"; prefix m; import ext { prefix ext; } leaf p { type ext:port; } }`}},
		// RFC requirement: RFC7950-6.1.3-1 positive — a quoted string carrying the four escapes n, t, " and \ loads.
		{name: "valid escapes", texts: []string{`module m { namespace "urn:m"; prefix m; leaf a { type string; description "tab\t nl\n quote\" back\\ end"; } }`}},
		// RFC requirement: RFC7950-6.2-1 positive — a leaf whose identifier is 64 characters long loads and is found in the entry tree under that name.
		{name: "64-char identifier", texts: []string{`module m { namespace "urn:m"; prefix m; leaf ` + strings.Repeat("a", 64) + ` { type string; } }`}},
		{name: "distinct names", texts: []string{`module m { namespace "urn:m"; prefix m; typedef t { type string; } typedef u { type uint8; } container c { leaf a { type t; } leaf b { type u; } } }`}},
		{name: "extension with own and import prefix", texts: []string{rfc7950Base,
			`module m { namespace "urn:m"; prefix m; import ext { prefix ext; } extension e { argument text; } leaf a { type string; m:e "own"; ext:mark "imported"; } }`}},
		// RFC requirement: RFC7950-6.5-1 positive — an external typedef referenced with its import prefix and a local typedef referenced without one both load.
		{name: "prefixed external and bare local", texts: []string{rfc7950Base,
			`module m { namespace "urn:m"; prefix m; import ext { prefix ext; } typedef l { type string; } leaf p { type ext:port; } leaf q { type l; } }`}},
		{name: "distinct import prefixes", texts: []string{rfc7950Base,
			`module ext2 { namespace "urn:ext2"; prefix ext2; typedef q { type string; } }`,
			`module m { namespace "urn:m"; prefix m; import ext { prefix e1; } import ext2 { prefix e2; } leaf a { type e1:port; } leaf b { type e2:q; } }`}},
		{name: "typedef with type", texts: []string{`module m { namespace "urn:m"; prefix m; typedef t { type uint8; } leaf a { type t; } }`}},
		// RFC requirement: RFC7950-7.6.3-1 positive — a leaf with a type substatement naming a built-in type loads.
		{name: "leaf with type", texts: []string{`module m { namespace "urn:m"; prefix m; leaf a { type string; } }`}},
		{name: "choice with distinct cases", texts: []string{`module m { namespace "urn:m"; prefix m; choice c { case x { leaf a { type string; } } case y { leaf b { type uint8; } } } }`}},
		{name: "grouping chain", texts: []string{`module m { namespace "urn:m"; prefix m; grouping g { leaf a { type string; } } grouping h { uses g; leaf b { type string; } } uses h; }`}},
		{name: "action under keyed list", texts: []string{`module m { namespace "urn:m"; prefix m; yang-version 1.1; list l { key "a"; leaf a { type string; } action act { } } }`}},
		{name: "notification under keyed list", texts: []string{`module m { namespace "urn:m"; prefix m; yang-version 1.1; list l { key "a"; leaf a { type string; } notification n { } } }`}},
		{name: "augment container", texts: []string{`module m { namespace "urn:m"; prefix m; container c { leaf a { type string; } } augment "/c" { leaf b { type string; } } }`}},
		{name: "identity chain", texts: []string{`module m { namespace "urn:m"; prefix m; identity base-i; identity i { base base-i; } leaf a { type string; } }`}},
		// RFC requirement: RFC7950-7.19-1 positive — an extension whose substatements are YANG statements loads.
		{name: "extension with YANG substatements", texts: []string{`module m { namespace "urn:m"; prefix m; extension e { argument text; description "an extension"; } leaf a { type string; } }`}},
		{name: "range ascending disjoint", texts: []string{`module m { namespace "urn:m"; prefix m; leaf a { type uint8 { range "min..5 | 10..max"; } } }`}},
		// RFC requirement: RFC7950-9.3.4-1 positive — a decimal64 type with fraction-digits 2 loads.
		{name: "decimal64 with fraction-digits", texts: []string{`module m { namespace "urn:m"; prefix m; leaf a { type decimal64 { fraction-digits 2; } } }`}},
		// RFC requirement: RFC7950-9.4.4-1 positive — a length with non-negative, ascending, disjoint parts loads.
		{name: "length ascending", texts: []string{`module m { namespace "urn:m"; prefix m; leaf a { type string { length "1..5 | 10..20"; } } }`}},
		{name: "enumeration distinct names", texts: []string{`module m { namespace "urn:m"; prefix m; leaf a { type enumeration { enum x; enum y; } } }`}},
		{name: "enum restriction subset", texts: []string{`module m { namespace "urn:m"; prefix m; typedef e { type enumeration { enum x; enum y; } } leaf a { type e { enum x; } } }`}},
		// RFC requirement: RFC7950-9.6.4.2-1 positive — an enumeration whose values are unique, within the int32 range, and explicitly given after the maximum value loads.
		{name: "enum values explicit", texts: []string{`module m { namespace "urn:m"; prefix m; leaf a { type enumeration { enum x { value 2147483647; } enum y { value -2147483648; } } } }`}},
		{name: "union with types", texts: []string{`module m { namespace "urn:m"; prefix m; leaf a { type union { type uint8; type string; } } }`}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, loadModuleTexts(t, tc.texts...), "loader must accept the conforming module set")
		})
	}
	loader := NewLoader()
	long := strings.Repeat("a", 64)
	require.NoError(t, loader.AddModuleFromText("m.yang", `module m { namespace "urn:m"; prefix m; leaf `+long+` { type string; } }`))
	require.NoError(t, loader.Resolve())
	entry := loader.GetEntry("m")
	require.NotNil(t, entry)
	assert.NotNil(t, entry.Dir[long], "the 64-character identifier must be found in the entry tree")
}
