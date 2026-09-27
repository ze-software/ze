// Design: docs/guide/static-routes.md -- a next-hop's bfd-profile is checked at commit
//
// The goal is that a next-hop naming a BFD profile the candidate config does not
// define, or one a single-hop session may not use, is refused at commit rather
// than at session start, where the route would run unmonitored with only a log
// line. The method runs the static verifier over the sections the commit
// delivers: the static section and the bfd section static reads.
package static

import (
	"strings"
	"testing"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"

	// The bfd plugin registers the real profile check (api.CheckProfile).
	_ "github.com/ze-software/ze/internal/component/bfd"
)

// staticBFDSections returns the commit's sections for a next-hop that names
// profile, beside a bfd section defining "fast" and the passive "quiet".
func staticBFDSections(profile string, withBFD bool) []sdk.ConfigSection {
	sections := []sdk.ConfigSection{{
		Root: pluginName,
		Data: wrap("10.0.0.0/8", `{"next":{"hop":{"10.0.0.1":{"bfd-profile":"`+profile+`"}}}}`),
	}}
	if withBFD {
		sections = append(sections, sdk.ConfigSection{
			Root: "bfd",
			Data: `{"bfd":{"profile":{"fast":{"detect-multiplier":"5"},"quiet":{"passive":"true"}}}}`,
		})
	}
	return sections
}

func TestStaticCommitRefusesUnusableBFDProfile(t *testing.T) {
	cases := []struct {
		name    string
		profile string
		withBFD bool
		refusal string
	}{
		{"defined profile", "fast", true, ""},
		{"undefined profile", "typo", true, `profile "typo" is not defined`},
		{"no bfd section", "fast", false, "the config has no bfd section"},
		{"passive profile on a single hop", "quiet", true, `profile "quiet" sets passive`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := verifyStaticSections(staticBFDSections(tc.profile, tc.withBFD), defReg())
			if tc.refusal == "" {
				if err != nil {
					t.Fatalf("verify refused a usable profile: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.refusal) {
				t.Fatalf("verify error = %v, want %q", err, tc.refusal)
			}
			if !strings.Contains(err.Error(), "10.0.0.1") {
				t.Errorf("verify error = %v, want the next-hop named", err)
			}
		})
	}
}
