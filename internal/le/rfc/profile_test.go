// VALIDATES: rfc/profiles/ parses, a use-case profile declares Kind, Title and
// Release only, every summary names an aggregate a profile declares, and a
// use case every summary leaves out is refused (AC-5, AC-7, AC-11).
// PREVENTS: a retired Roots or Absent row read in silence, a malformed profile
// field read as a default, and a release profile counted over citations.

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

// useCaseProfile is the lns use-case profile, with extra rows appended.
func useCaseProfile(extra string) string {
	return "# Profile: lns\n\n| Field | Value |\n|-------|-------|\n" +
		"| Kind | use-case |\n| Title | T |\n| Release | yes |\n" + extra
}

// TestProfileRetiredFieldsRefused: AC-7. A use-case profile carrying a `Roots`
// or an `Absent` row is refused naming the file, the retired field and the
// two summary rows that replaced it, and the profile is absent from the answer.
// Method: one profile per retired field over an otherwise valid file.
func TestProfileRetiredFieldsRefused(t *testing.T) {
	for _, field := range []string{"Roots", "Absent"} {
		t.Run(field, func(t *testing.T) {
			root := writeProfileTree(t, "lns", useCaseProfile("| "+field+" | rfc2661 |\n"))
			profiles, problems, err := LoadProfiles(root)
			if err != nil {
				t.Fatal(err)
			}
			if len(problems) != 1 {
				t.Fatalf("want one problem, got %q", problems)
			}
			for _, want := range []string{"rfc/profiles/lns.md", "`" + field + "` is retired",
				"`Use cases`", "`Optional use cases`"} {
				if !strings.Contains(problems[0], want) {
					t.Errorf("the refusal %q does not name %s", problems[0], want)
				}
			}
			if _, held := profiles.UseCases["lns"]; held {
				t.Error("a refused profile is still in the answer")
			}
		})
	}
	root := writeProfileTree(t, "lns", useCaseProfile(""))
	profiles, problems, err := LoadProfiles(root)
	if err != nil || len(problems) != 0 {
		t.Fatalf("a profile of Kind, Title and Release is refused: %v %q", err, problems)
	}
	if got := profiles.UseCases["lns"]; got.Name != "lns" || !got.Release || got.Title != "T" {
		t.Errorf("use case parsed as %+v", got)
	}
}

// TestProfileRefusals: every malformed shape a profile file can take is
// refused with the file named, and none of them is defaulted.
func TestProfileRefusals(t *testing.T) {
	aggregate := "# Profile: ppp\n\n| Field | Value |\n|--|--|\n| Kind | technology |\n" +
		"| Title | PPP |\n| Heading | PPP access |\n| Intro | - |\n| Shape | wide |\n| Release | n/a |\n"
	cases := map[string]struct{ name, body, want string }{
		"release word": {"lns", strings.Replace(useCaseProfile(""),
			"| Release | yes |", "| Release | maybe |", 1), "neither `yes` nor `no`"},
		"unknown field": {"ppp", aggregate + "| Members | rfc2661 |\n", "is not a field"},
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
			_, problems, err := LoadProfiles(root)
			if err != nil {
				t.Fatal(err)
			}
			if len(problems) != 1 || !strings.Contains(problems[0], one.want) {
				t.Fatalf("want one refusal naming %q, got %q", one.want, problems)
			}
		})
	}
	root := writeProfileTree(t, "ppp", aggregate)
	profiles, problems, err := LoadProfiles(root)
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
	profiles, problems, err := LoadProfiles(root)
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

// TestEmptyUseCaseProfileRefused: AC-5. A use-case profile no summary lists as
// required is refused naming the profile and the row that joins it, an
// optional-only one included, and its rollup never reads complete. Method:
// three use cases over two summaries: one required, one optional-only, one
// named by nothing.
func TestEmptyUseCaseProfileRefused(t *testing.T) {
	profiles := Profiles{UseCases: map[string]UseCase{"lns": {Name: "lns"}, "edge": {Name: "edge"},
		"quiet": {Name: "quiet"}}}
	metas := map[string]Meta{
		"rfc1": {UseCases: []string{"lns"}, OptionalUseCases: []string{"edge"}},
		"rfc2": {OptionalUseCases: []string{"edge"}},
	}
	refusals := checkUseCaseMembers(profiles, metas)
	if len(refusals) != 2 {
		t.Fatalf("want two refusals (edge, quiet), got %q", refusals)
	}
	for index, name := range []string{"edge", "quiet"} {
		for _, want := range []string{"rfc/profiles/" + name + ".md", "| Use cases | " + name + " |"} {
			if !strings.Contains(refusals[index], want) {
				t.Errorf("refusal %q does not name %q", refusals[index], want)
			}
		}
	}
	empty := Rollup{}
	if empty.mandatoryComplete() {
		t.Error("a rollup over no RFC reads mandatory complete: 0 of 0 is not a measurement")
	}
}

// TestEveryProfileMemberHasASummary: AC-11 over the real checkout. Every name
// either use-case row of every summary writes is a declared use case, and
// every member of every use case, in either tier, is a summary that parsed.
// Method: the members ProfileMembers answers, against the parsed summaries.
func TestEveryProfileMemberHasASummary(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	profiles, metas := realProfilesAndMetas(t, root)
	members := ProfileMembers(profiles, metas)
	for _, name := range sortedKeysOf(profiles.UseCases) {
		tiers := members[name]
		if len(tiers.Required) == 0 {
			t.Errorf("use case %s has no required member", name)
		}
		for _, stem := range append(append([]string{}, tiers.Required...), tiers.Optional...) {
			if _, held := metas[stem]; !held {
				t.Errorf("use case %s lists %s, which has no summary that parsed", name, stem)
			}
		}
	}
}

// TestRouteServerProfileExcludesCitationClosure: AC-11 over the real
// checkout. No member of any release profile, in either tier, belongs to an
// aggregate the route-server citation closure used to drag in, unless the
// profile's own role is that technology. Method: route-server, route-collector
// and looking-glass members, against the aggregates of PPP, DHCP, IS-IS, DNS,
// tunnels, IPsec, MPLS and OSPF.
func TestRouteServerProfileExcludesCitationClosure(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	profiles, metas := realProfilesAndMetas(t, root)
	members := ProfileMembers(profiles, metas)
	foreign := map[string]bool{"ppp-access": true, "dhcp-provisioning": true, "isis": true, "dns": true,
		"tunnels": true, "ikev2-ipsec": true, "mpls": true, "ospf": true}
	for _, name := range []string{"route-server", "route-collector", "looking-glass"} {
		tiers := members[name]
		for _, stem := range append(append([]string{}, tiers.Required...), tiers.Optional...) {
			if foreign[metas[stem].Aggregate] {
				t.Errorf("%s lists %s, of aggregate %s, which a BGP role does not implement",
					name, stem, metas[stem].Aggregate)
			}
		}
	}
}

// realProfilesAndMetas answers the checkout's parsed profiles and summaries,
// failing on any problem in either.
func realProfilesAndMetas(t *testing.T, root string) (Profiles, map[string]Meta) {
	t.Helper()
	profiles, problems, err := LoadProfiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Fatalf("profiles do not parse:\n%s", strings.Join(problems, "\n"))
	}
	metas, metaProblems, err := summaryMetas(root, nil, profiles)
	if err != nil {
		t.Fatal(err)
	}
	if len(metaProblems) != 0 {
		t.Fatalf("summaries refused:\n%s", strings.Join(sortedValuesOf(metaProblems), "\n"))
	}
	return profiles, metas
}
