// Related: community.go -- the registry these tests pin
// Related: text_append.go -- appendCommunityText, the render path
// Related: builder_parse.go -- parseSingleCommunity, the second parse path
//
// VALIDATES: one registration populates both the numeric render table and the
// text parse table, so every community name resolves identically through
// ParseCommunity, parseSingleCommunity, String and AppendText.
// PREVENTS: the return of a second hand-maintained name table. Four copies of
// this knowledge existed: a render switch, a render map, and two parse maps.
// The parse maps had drifted to 5 names against 31. A community name was then
// accepted or rejected by which call site an operator's config took.

package attribute

import (
	"strings"
	"testing"
)

// TestCommunityParsersShareOneVocabulary pins the property that made this
// registry necessary: every accepted spelling must resolve identically through
// both parse entry points.
//
// Before the registry, builder_parse.go carried a private 5-name table while
// text.go carried 31, so "llgr-stale" resolved through one caller and failed
// through another. This test fails for 26 names if that table returns.
func TestCommunityParsersShareOneVocabulary(t *testing.T) {
	if len(communityValues) == 0 {
		t.Fatal("communityValues is empty; the registry did not initialize")
	}
	for name, want := range communityValues {
		got, err := ParseCommunity(name)
		if err != nil {
			t.Errorf("ParseCommunity(%q) failed: %v", name, err)
			continue
		}
		if got != uint32(want) {
			t.Errorf("ParseCommunity(%q) = 0x%08X, want 0x%08X", name, got, uint32(want))
		}

		gotBuilder, err := parseSingleCommunity(name)
		if err != nil {
			t.Errorf("parseSingleCommunity(%q) failed: %v -- the two parsers disagree", name, err)
			continue
		}
		if gotBuilder != uint32(want) {
			t.Errorf("parseSingleCommunity(%q) = 0x%08X, want 0x%08X", name, gotBuilder, uint32(want))
		}
	}
}

// TestCommunityCanonicalNamesRoundTrip checks that what the renderer emits is
// what the parsers accept. A canonical name that renders but does not parse
// would make `show route` output unusable as config input.
func TestCommunityCanonicalNamesRoundTrip(t *testing.T) {
	for value, name := range communityNames {
		rendered := value.String()
		if rendered != name {
			t.Errorf("Community(0x%08X).String() = %q, want the canonical %q", uint32(value), rendered, name)
		}

		appended := string(value.AppendText(nil))
		if appended != name {
			t.Errorf("Community(0x%08X).AppendText = %q, want %q -- render paths disagree", uint32(value), appended, name)
		}

		back, err := ParseCommunity(rendered)
		if err != nil {
			t.Errorf("ParseCommunity(%q) failed on its own rendered form: %v", rendered, err)
			continue
		}
		if back != uint32(value) {
			t.Errorf("round trip of %q gave 0x%08X, want 0x%08X", rendered, back, uint32(value))
		}
	}
}

// TestCommunityUnderscoreSpellingsDerived checks the underscore spellings are
// generated from the canonical names rather than hand-listed. A canonical name
// containing a hyphen must have its underscore twin accepted automatically.
func TestCommunityUnderscoreSpellingsDerived(t *testing.T) {
	checked := 0
	for value, name := range communityNames {
		underscored := strings.ReplaceAll(name, "-", "_")
		if underscored == name {
			continue // no hyphen to convert (e.g. "nopeer", "blackhole")
		}
		checked++
		got, ok := CommunityValue(underscored)
		if !ok {
			t.Errorf("underscore spelling %q not accepted; it must be derived from %q", underscored, name)
			continue
		}
		if got != value {
			t.Errorf("CommunityValue(%q) = 0x%08X, want 0x%08X", underscored, uint32(got), uint32(value))
		}
	}
	if checked == 0 {
		t.Fatal("no hyphenated canonical names found; the derivation is untested")
	}
}

// TestCommunityAliasesParseButNeverRender pins the one deliberate asymmetry:
// aliases are accepted on input and never produced on output.
func TestCommunityAliasesParseButNeverRender(t *testing.T) {
	for alias, value := range communityAliases {
		got, ok := CommunityValue(alias)
		if !ok {
			t.Errorf("alias %q is not accepted", alias)
			continue
		}
		if got != value {
			t.Errorf("CommunityValue(%q) = 0x%08X, want 0x%08X", alias, uint32(got), uint32(value))
		}
		if rendered := value.String(); rendered == alias {
			t.Errorf("alias %q was rendered; output must use the canonical name", alias)
		}
	}
}

// TestRegisterCommunityNameReachesBothDirections is the regression guard for
// the defect that started this work. A name registered by a plugin was visible
// to String() and invisible to AppendText() and to every parser. The render
// path was a hardcoded switch, and the parse tables were separate.
func TestRegisterCommunityNameReachesBothDirections(t *testing.T) {
	const (
		value = Community(0x0BAD0001)
		name  = "registry-probe"
	)
	if _, taken := communityNames[value]; taken {
		t.Fatalf("test value 0x%08X is already registered; pick another", uint32(value))
	}
	if err := RegisterCommunityName(value, name); err != nil {
		t.Fatalf("RegisterCommunityName: %v", err)
	}
	t.Cleanup(func() {
		delete(communityNames, value)
		delete(communityValues, name)
		delete(communityValues, "registry_probe")
	})

	if got := value.String(); got != name {
		t.Errorf("String() = %q, want %q", got, name)
	}
	if got := string(value.AppendText(nil)); got != name {
		t.Errorf("AppendText = %q, want %q -- the render path ignored the registry", got, name)
	}
	if got, err := ParseCommunity(name); err != nil || got != uint32(value) {
		t.Errorf("ParseCommunity(%q) = 0x%08X, %v -- want 0x%08X", name, got, err, uint32(value))
	}
	if got, err := parseSingleCommunity(name); err != nil || got != uint32(value) {
		t.Errorf("parseSingleCommunity(%q) = 0x%08X, %v -- want 0x%08X", name, got, err, uint32(value))
	}
	if got, ok := CommunityValue("registry_probe"); !ok || got != value {
		t.Errorf("underscore spelling of a registered name not accepted")
	}
}

// TestRegisterCommunityAliasRejectsConflicts checks the guard fails closed.
// An alias MUST NOT be repointed at a different value. It MUST NOT attach to
// a community that carries no canonical name.
func TestRegisterCommunityAliasRejectsConflicts(t *testing.T) {
	if err := RegisterCommunityAlias(Community(0x0BAD0002), "orphan-alias"); err == nil {
		t.Error("alias for an unnamed community was accepted; the guard must fail closed")
	}
	if _, ok := CommunityValue("orphan-alias"); ok {
		t.Error("a rejected alias was still recorded")
	}
	if err := RegisterCommunityAlias(CommunityNoExport, "gshut"); err == nil {
		t.Error(`repointing the existing alias "gshut" at another value was accepted`)
	}
	if got, ok := CommunityValue("gshut"); !ok || got != CommunityGracefulShutdown {
		t.Error("a rejected alias overwrote the existing mapping")
	}
	if err := RegisterCommunityAlias(CommunityGracefulShutdown, "gshut"); err != nil {
		t.Errorf("re-registering an alias with its own value must be idempotent, got %v", err)
	}
}

// TestCommunityNumericFallbackBoundaries covers the values either side of the
// registry. The numeric ASN:value form MUST hold at both ends of the 16-bit
// fields. An unregistered value MUST NOT render as a name.
func TestCommunityNumericFallbackBoundaries(t *testing.T) {
	for _, tc := range []struct {
		value Community
		want  string
	}{
		{0x00000000, "0:0"},
		{0x00000001, "0:1"},
		{0x0000FFFF, "0:65535"},
		{0x00010000, "1:0"},
		{0xFFFEFFFF, "65534:65535"},
	} {
		if _, registered := communityNames[tc.value]; registered {
			t.Fatalf("0x%08X is registered; it cannot test the numeric fallback", uint32(tc.value))
		}
		if got := tc.value.String(); got != tc.want {
			t.Errorf("Community(0x%08X).String() = %q, want %q", uint32(tc.value), got, tc.want)
		}
		if got := string(tc.value.AppendText(nil)); got != tc.want {
			t.Errorf("Community(0x%08X).AppendText = %q, want %q", uint32(tc.value), got, tc.want)
		}
		back, err := ParseCommunity(tc.want)
		if err != nil || back != uint32(tc.value) {
			t.Errorf("ParseCommunity(%q) = 0x%08X, %v -- want 0x%08X", tc.want, back, err, uint32(tc.value))
		}
	}
}
