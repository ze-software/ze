package attribute

import (
	"testing"
)

const maxFuzzTextInput = 4096

func skipOversizedFuzzText(t *testing.T, s string) {
	t.Helper()
	if len(s) > maxFuzzTextInput {
		t.Skip()
	}
}

// FuzzParseOrigin tests origin parsing robustness.
//
// VALIDATES: Parser handles arbitrary strings without crashing.
// PREVENTS: Panic on malformed origin input.
// SECURITY: Origin comes from user/API input.
func FuzzParseOrigin(f *testing.F) {
	// Seed corpus
	f.Add("igp")
	f.Add("egp")
	f.Add("incomplete")
	f.Add("?")
	f.Add("IGP")
	f.Add("")
	f.Add("invalid")
	f.Add("igp igp")
	f.Add("\x00\x00")

	f.Fuzz(func(t *testing.T, s string) {
		skipOversizedFuzzText(t, s)
		b := NewBuilder()
		_ = b.ParseOrigin(s) // MUST NOT panic
	})
}

// FuzzParseMED tests MED parsing robustness.
//
// VALIDATES: Parser handles arbitrary strings without crashing.
// PREVENTS: Panic on malformed MED input.
// SECURITY: MED comes from user/API input.
func FuzzParseMED(f *testing.F) {
	f.Add("0")
	f.Add("100")
	f.Add("4294967295")
	f.Add("")
	f.Add("-1")
	f.Add("abc")
	f.Add("4294967296")
	f.Add("1.5")
	f.Add("\x00")

	f.Fuzz(func(t *testing.T, s string) {
		skipOversizedFuzzText(t, s)
		b := NewBuilder()
		_ = b.ParseMED(s) // MUST NOT panic
	})
}

// FuzzParseLocalPref tests LOCAL_PREF parsing robustness.
//
// VALIDATES: Parser handles arbitrary strings without crashing.
// PREVENTS: Panic on malformed LOCAL_PREF input.
// SECURITY: LOCAL_PREF comes from user/API input.
func FuzzParseLocalPref(f *testing.F) {
	f.Add("100")
	f.Add("0")
	f.Add("4294967295")
	f.Add("")
	f.Add("-1")
	f.Add("abc")

	f.Fuzz(func(t *testing.T, s string) {
		skipOversizedFuzzText(t, s)
		b := NewBuilder()
		_ = b.ParseLocalPref(s) // MUST NOT panic
	})
}

// FuzzParseASPath tests AS_PATH parsing robustness.
//
// VALIDATES: Parser handles arbitrary strings without crashing.
// PREVENTS: Panic on malformed AS_PATH input.
// SECURITY: AS_PATH comes from user/API input.
func FuzzParseASPath(f *testing.F) {
	f.Add("[65001 65002]")
	f.Add("[65001,65002]")
	f.Add("65001")
	f.Add("65001 65002 65003")
	f.Add("[]")
	f.Add("")
	f.Add("[abc]")
	f.Add("[[65001]]")
	f.Add("[65001 65002")
	f.Add("65001]")
	f.Add("\x00\x00\x00")

	f.Fuzz(func(t *testing.T, s string) {
		skipOversizedFuzzText(t, s)
		b := NewBuilder()
		_ = b.ParseASPath(s) // MUST NOT panic
	})
}

// FuzzParseCommunity tests community parsing robustness.
//
// VALIDATES: Parser handles arbitrary strings without crashing.
// PREVENTS: Panic on malformed community input.
// SECURITY: Community comes from user/API input.
func FuzzParseCommunity(f *testing.F) {
	f.Add("65000:100")
	f.Add("no-export")
	f.Add("no-advertise")
	f.Add("[65000:100 65000:200]")
	f.Add("")
	f.Add("invalid")
	f.Add("65000:")
	f.Add(":100")
	f.Add("65000:100:200")
	f.Add("999999:100")
	f.Add("\x00:\x00")

	f.Fuzz(func(t *testing.T, s string) {
		skipOversizedFuzzText(t, s)
		b := NewBuilder()
		_ = b.ParseCommunity(s) // MUST NOT panic
	})
}

// FuzzParseLargeCommunity tests large community parsing robustness.
//
// VALIDATES: Parser handles arbitrary strings without crashing.
// PREVENTS: Panic on malformed large community input.
// SECURITY: Large community comes from user/API input.
func FuzzParseLargeCommunity(f *testing.F) {
	f.Add("65000:1:2")
	f.Add("[65000:1:2 65001:3:4]")
	f.Add("")
	f.Add("65000:1")
	f.Add("65000:1:2:3")
	f.Add("abc:1:2")
	f.Add("\x00:\x00:\x00")

	f.Fuzz(func(t *testing.T, s string) {
		skipOversizedFuzzText(t, s)
		b := NewBuilder()
		_ = b.ParseLargeCommunity(s) // MUST NOT panic
	})
}

// FuzzParseExtCommunity tests extended community parsing robustness.
//
// VALIDATES: Parser handles arbitrary strings without crashing.
// PREVENTS: Panic on malformed extended community input.
// SECURITY: Extended community comes from user/API input.
func FuzzParseExtCommunity(f *testing.F) {
	f.Add("target:65000:100")
	f.Add("origin:65000:100")
	f.Add("rt:65000:100")
	f.Add("soo:65000:100")
	f.Add("")
	f.Add("invalid:65000:100")
	f.Add("target:65000")
	f.Add("target:abc:100")
	f.Add("target:1.2.3.4:100")
	f.Add("\x00:\x00:\x00")

	f.Fuzz(func(t *testing.T, s string) {
		skipOversizedFuzzText(t, s)
		b := NewBuilder()
		_ = b.ParseExtCommunity(s) // MUST NOT panic
	})
}
