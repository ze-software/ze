// VALIDATES: rfc/profiles/ parses, every use-case root is a summary or listed absent,
// and every summary names an aggregate a profile declares (AC-9, AC-17).
// PREVENTS: a profile root that drops out of a closure in silence, and a
// malformed profile field read as a default.

package rfc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// writeProfileTree writes one profile file into a fresh tree and answers it.
func writeProfileTree(t *testing.T, name, body string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, profileRel)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".md"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

// useCaseProfile is the lns use-case profile with the given roots and absent
// cell.
func useCaseProfile(roots, absent string) string {
	return "# Profile: lns\n\n| Field | Value |\n|-------|-------|\n" +
		"| Kind | use-case |\n| Title | T |\n| Release | yes |\n" +
		"| Roots | " + roots + " |\n| Absent | " + absent + " |\n"
}

// TestProfileUnknownRootRefused: AC-9. A root with no summary, not listed as
// absent, is refused naming the profile and the stem; the method is one
// profile over a stem set that lacks the root.
func TestProfileUnknownRootRefused(t *testing.T) {
	root := writeProfileTree(t, "lns", useCaseProfile("rfc2661, rfc9998", "RFC 1570"))
	profiles, problems, err := LoadProfiles(root, map[string]bool{"rfc2661": true})
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 1 {
		t.Fatalf("want one problem, got %q", problems)
	}
	for _, want := range []string{"rfc/profiles/lns.md", `'lns'`, `'rfc9998'`} {
		if !strings.Contains(problems[0], want) {
			t.Errorf("the refusal %q does not name %s", problems[0], want)
		}
	}
	if _, held := profiles.UseCases["lns"]; held {
		t.Error("a refused profile is still in the answer")
	}
}

// TestProfileRefusals: every malformed shape a profile file can take is
// refused with the file named, and none of them is defaulted.
func TestProfileRefusals(t *testing.T) {
	stems := map[string]bool{"rfc2661": true, "rfc1570": true}
	aggregate := "# Profile: ppp\n\n| Field | Value |\n|--|--|\n| Kind | technology |\n" +
		"| Title | PPP |\n| Heading | PPP access |\n| Intro | - |\n| Shape | wide |\n| Release | n/a |\n"
	cases := map[string]struct{ name, body, want string }{
		"absent RFC held":  {"lns", useCaseProfile("rfc2661", "RFC 1570"), "exists"},
		"absent malformed": {"lns", useCaseProfile("rfc2661", "1570"), "is not `RFC <number>`"},
		"release word": {"lns", strings.Replace(useCaseProfile("rfc2661", "-"),
			"| Release | yes |", "| Release | maybe |", 1), "neither `yes` nor `no`"},
		"unknown field": {"ppp", aggregate + "| Roots | rfc2661 |\n", "is not a field"},
		"missing field": {"ppp", strings.Replace(aggregate, "| Shape | wide |\n", "", 1), "`Shape` is absent"},
		"bad shape":     {"ppp", strings.Replace(aggregate, "| Shape | wide |", "| Shape | tall |", 1), "neither `wide` nor `brief`"},
		"bad release": {"ppp", strings.Replace(aggregate, "| Release | n/a |", "| Release | yes |", 1),
			"only a use-case profile"},
		"bad kind":   {"ppp", strings.Replace(aggregate, "technology", "area", 1), "neither `use-case`"},
		"bad header": {"ppp", strings.Replace(aggregate, "# Profile: ppp", "# Profile: other", 1), "first line"},
	}
	for label, one := range cases {
		t.Run(label, func(t *testing.T) {
			root := writeProfileTree(t, one.name, one.body)
			_, problems, err := LoadProfiles(root, stems)
			if err != nil {
				t.Fatal(err)
			}
			if len(problems) != 1 || !strings.Contains(problems[0], one.want) {
				t.Fatalf("want one refusal naming %q, got %q", one.want, problems)
			}
		})
	}
	root := writeProfileTree(t, "ppp", aggregate)
	profiles, problems, err := LoadProfiles(root, stems)
	if err != nil || len(problems) != 0 {
		t.Fatalf("a well-formed aggregate is refused: %v %q", err, problems)
	}
	if got := profiles.Aggregates["ppp"]; got.Shape != ShapeWide || got.Intro != "" || got.Heading != "PPP access" {
		t.Errorf("aggregate parsed as %+v", got)
	}
}

// TestCorpusProfilesParse: AC-17 over the real rfc/profiles/. Every file
// parses, the eight use-case profiles exist with the release marks the owner
// set on 2026-10-25, and every summary names a declared aggregate.
func TestCorpusProfilesParse(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	stems, err := summaryStems(root)
	if err != nil {
		t.Fatal(err)
	}
	profiles, problems, err := LoadProfiles(root, stems)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Fatalf("profiles do not parse:\n%s", strings.Join(problems, "\n"))
	}
	release := map[string]bool{
		"route-server": true, "route-collector": true, "looking-glass": true, "lns": true,
		"fttx-edge": true, "vpn-endpoint": true, "vpn-concentrator": false, "ddos-flowspec": false,
	}
	if len(profiles.UseCases) != len(release) {
		t.Errorf("want %d use-case profiles, got %d", len(release), len(profiles.UseCases))
	}
	for name, want := range release {
		useCase, held := profiles.UseCases[name]
		if !held {
			t.Errorf("use-case profile %s is missing", name)
			continue
		}
		if useCase.Release != want {
			t.Errorf("%s: release is %v, want %v", name, useCase.Release, want)
		}
	}
	metas, metaProblems, err := summaryMetas(root, stems, profiles)
	if err != nil {
		t.Fatal(err)
	}
	if len(metaProblems) != 0 {
		t.Fatalf("summaries refused:\n%s", strings.Join(sortedValuesOf(metaProblems), "\n"))
	}
	for stem, meta := range metas {
		if _, held := profiles.Aggregates[meta.Aggregate]; !held {
			t.Errorf("%s names aggregate %q, which no profile declares", stem, meta.Aggregate)
		}
	}
}
