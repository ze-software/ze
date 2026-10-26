// Design: docs/contributing/rfc-conformance-gates.md -- the priority tree, the exported members and requirement index
// Related: priority.go -- the scope these answer from
// Related: profile.go -- ParseProfiles, the bytes-level profile parser
//
// priority_export.go is the surface another le package reads the priority
// tree through: the use-case profiles' declared members, the
// requirement-id-to-stem index and the rendered priority trees. The members
// are answered over parsed inputs only, for a caller holding a committed
// revision's blobs. The index comes in two forms, one over parsed summaries,
// one over a checkout directory.
package rfc

import (
	"path/filepath"

	"github.com/ze-software/ze/internal/le/rfc/graph"
)

// ProfileTiers is one use-case profile's declared members, each sorted.
type ProfileTiers struct {
	// Required is the summaries whose `Use cases` row names the profile:
	// counted in its figures.
	Required []string
	// Optional is the summaries whose `Optional use cases` row names it:
	// listed, never counted.
	Optional []string
}

// ProfileMembers answers each use-case profile's two tiers, keyed by profile
// name. The answer is the scope `./le rfc tree profile <name>` renders, so the
// two cannot disagree.
//
// profiles is ParseProfiles over the profile files, metas ParseMeta over the
// summaries. No edge of the dependency graph is read: a citation adds no
// member.
func ProfileMembers(profiles Profiles, metas map[string]Meta) map[string]ProfileTiers {
	corpus := newPriorityCorpus(graph.Build(nil), profiles, metas, nil, nil, nil)
	out := map[string]ProfileTiers{}
	for _, scope := range corpus.useCaseScopes(false) {
		out[scope.name] = ProfileTiers{Required: sortedSet(scope.members), Optional: sortedSet(scope.optional)}
	}
	return out
}

// PriorityAnswer is the priority tree of one rendered input: the whole
// corpus, and every profile limited to its own members.
type PriorityAnswer struct {
	// Corpus is the unlimited tree: every graph node and every summary, the
	// nodes a release profile reaches first.
	Corpus *TreeReport
	// Profiles is one tree per profile file: the use cases by name, then the
	// technology aggregates by name.
	Profiles []*TreeReport
}

// PriorityOf answers the trees `./le rfc tree` prints, over an input the
// caller already rendered, so the site publishes exactly what the command
// counts and pays for no second reading of the tree.
func PriorityOf(in *RenderInput) (PriorityAnswer, error) {
	corpus, err := priorityCorpusOf(in)
	if err != nil {
		return PriorityAnswer{}, err
	}
	whole, err := corpus.tree("")
	if err != nil {
		return PriorityAnswer{}, err
	}
	answer := PriorityAnswer{Corpus: whole}
	names := append(sortedKeysOf(corpus.profiles.UseCases), sortedKeysOf(corpus.profiles.Aggregates)...)
	for _, name := range names {
		report, err := corpus.tree(name)
		if err != nil {
			return PriorityAnswer{}, err
		}
		answer.Profiles = append(answer.Profiles, report)
	}
	return answer, nil
}

// RequirementStems answers the summary stem of every requirement id the
// summaries list, from the summaries alone: no tag scan, no evidence. A
// stem such as `sflow-v5` is why the id's shape cannot answer it.
//
// summaries is each rfc/short/<stem>.md text keyed by stem. A summary whose
// checklist does not parse is an error naming it: its ids would be missing,
// and a missing id reads as an unknown one.
func RequirementStems(summaries map[string]string) (map[string]string, error) {
	out := map[string]string{}
	for _, stem := range sortedKeysOf(summaries) {
		requirements, err := parseSummaryText(summaries[stem], stem, summaryRel+"/"+stem+".md")
		if err != nil {
			return nil, err
		}
		for index := range requirements {
			out[requirements[index].RID] = stem
		}
	}
	return out, nil
}

// LoadRequirementStems answers RequirementStems over a checkout's rfc/short/.
func LoadRequirementStems(tree string) (map[string]string, error) {
	stems, err := summaryStems(tree)
	if err != nil {
		return nil, err
	}
	summaries := make(map[string]string, len(stems))
	for stem := range stems {
		rel := summaryRel + "/" + stem + ".md"
		text, err := readFile(filepath.Join(treePath(tree, summaryRel), stem+".md"), rel)
		if err != nil {
			return nil, err
		}
		summaries[stem] = text
	}
	return RequirementStems(summaries)
}
