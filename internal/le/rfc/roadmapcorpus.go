// Design: docs/contributing/spec-workflow.md -- the roadmap Profiles column.
// Related: register.go -- registers readRoadmapCorpus with the roadmap.
// Related: profile.go -- ParseProfiles, the profile parser it reads through.
// Related: meta.go -- ParseMeta, whose two use-case rows declare membership.
// Related: priority_export.go -- RequirementStems.
// Related: priority.go -- useCaseScopes, the declared tiers requiredMembers reads.

package rfc

import (
	"fmt"
	"path"
	"strings"

	"github.com/ze-software/ze/internal/le/rfc/graph"
	"github.com/ze-software/ze/internal/le/spec/roadmap"
)

// readRoadmapCorpus builds the roadmap's Corpus from one revision's corpus
// files, keyed by path, with the same parsers the checkout readers use:
// RequirementStems and ParseMeta over the summaries, ParseProfiles over the
// profiles, and requiredMembers over both. No RFC text is read: membership is
// what each summary declares, never what a citation reaches. A summary or a
// profile that does not parse, and a use-case row naming no use case, is an
// error naming it, because a missing declaration silently shrinks a profile.
func readRoadmapCorpus(files map[string]string) (roadmap.Corpus, error) {
	summaries := map[string]string{}
	profileFiles := map[string]string{}
	corpus := roadmap.Corpus{SummaryStems: map[string]bool{}, ProfilesOf: map[string][]string{}}
	for rel, body := range files {
		dir, name := path.Split(rel)
		switch dir {
		case "rfc/short/":
			stem := strings.TrimSuffix(name, ".md")
			summaries[stem] = body
			corpus.SummaryStems[stem] = true
		case "rfc/profiles/":
			profileFiles[strings.TrimSuffix(name, ".md")] = body
		default:
			return roadmap.Corpus{}, fmt.Errorf("%s is outside rfc/short/ and rfc/profiles/, which the roadmap reads", rel)
		}
	}
	requirementStems, err := RequirementStems(summaries)
	if err != nil {
		return roadmap.Corpus{}, err
	}
	corpus.RequirementStems = requirementStems
	profiles, problems := ParseProfiles(profileFiles)
	if len(problems) != 0 {
		return roadmap.Corpus{}, fmt.Errorf("a profile under rfc/profiles/ did not parse, so its members are unknown: %s", strings.Join(problems, "; "))
	}
	metas := make(map[string]Meta, len(summaries))
	for _, stem := range sortedKeysOf(summaries) {
		rel := summaryRel + "/" + stem + ".md"
		meta, err := ParseMeta(summaries[stem], stem, rel)
		if err != nil {
			return roadmap.Corpus{}, err
		}
		if err := checkUseCases(&meta, profiles, rel); err != nil {
			return roadmap.Corpus{}, err
		}
		metas[stem] = meta
	}
	members := requiredMembers(profiles, metas)
	corpus.ProfileCount = len(members)
	for profile, stems := range members {
		for _, stem := range stems {
			corpus.ProfilesOf[stem] = append(corpus.ProfilesOf[stem], profile)
		}
	}
	return corpus, nil
}

// requiredMembers answers each use-case profile's required members, sorted,
// keyed by profile name: the summaries whose `Use cases` row names it. The
// roadmap column answers which release work a spec serves, and an optional
// listing places nothing in a profile, so the optional tier is not answered.
// The members are the scope `./le rfc tree profile <name>` renders, so the two
// cannot disagree. No edge of the dependency graph is read: a citation adds no
// member.
func requiredMembers(profiles Profiles, metas map[string]Meta) map[string][]string {
	corpus := newPriorityCorpus(graph.Build(nil), profiles, metas, nil, nil, nil)
	out := map[string][]string{}
	for _, scope := range corpus.useCaseScopes(false) {
		out[scope.name] = sortedSet(scope.members)
	}
	return out
}
