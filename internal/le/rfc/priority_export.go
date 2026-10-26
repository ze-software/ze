// Design: docs/contributing/rfc-conformance-gates.md -- the priority tree, the exported closure and requirement index
// Related: priority.go -- the closure these answer from
// Related: profile.go -- ParseProfiles, the bytes-level profile parser
//
// priority_export.go is the surface another le package reads the priority
// tree through: the use-case profile closures and the requirement-id-to-stem
// index. Each comes in two forms, one over parsed inputs for a caller holding
// a committed revision's blobs, one over a checkout directory.
package rfc

import (
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/rfc/graph"
)

// ProfileClosures answers each use-case profile's closure, keyed by profile
// name: its roots and every document reachable from them over builds-on
// edges and over the updates edges pointing at them, sorted. Absent RFCs
// are not members: they are UseCase.Absent. The answer is the closure
// `./le rfc tree profile <name>` walks, so the two cannot disagree.
//
// g is graph.Build over the texts (graph.Parse per text), profiles is
// ParseProfiles over the profile files.
func ProfileClosures(g *graph.Graph, profiles Profiles) map[string][]string {
	corpus := newPriorityCorpus(g, profiles, nil, nil, nil, nil)
	out := map[string][]string{}
	for _, scope := range corpus.useCaseScopes(false) {
		out[scope.name] = sortedKeysOf(scope.reached)
	}
	return out
}

// LoadProfileClosures answers ProfileClosures over a checkout. A profile that
// does not parse is refused rather than dropped: a profile missing from the
// answer would read as one whose closure holds nothing.
func LoadProfileClosures(tree string) (map[string][]string, error) {
	profiles, problems, err := LoadProfiles(tree, nil)
	if err != nil {
		return nil, err
	}
	if len(problems) != 0 {
		var tb textbuf.Buffer
		return nil, parseErr(tb.Str("a profile under ").Str(profileRel).Str("/ did not parse, so its closure is unknown: ").
			Str(strings.Join(problems, "; ")).Str(". Fix the file, then run ./le rfc check"))
	}
	g, err := graph.Load(tree)
	if err != nil {
		return nil, err
	}
	return ProfileClosures(g, profiles), nil
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
