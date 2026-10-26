// Design: docs/contributing/spec-workflow.md -- the roadmap Profiles column.
// Related: register.go -- registers Read with the roadmap.

package rfccorpus

import (
	"fmt"
	"path"
	"strings"

	"github.com/ze-software/ze/internal/le/rfc"
	"github.com/ze-software/ze/internal/le/rfc/graph"
	"github.com/ze-software/ze/internal/le/spec/roadmap"
)

// Read builds the roadmap's Corpus from one revision's corpus files, keyed by
// path, with the same parsers the checkout readers use: rfc.RequirementStems
// over the summaries, rfc.ParseProfiles over the profiles, graph.Parse and
// graph.Build over the texts, and rfc.ProfileClosures over both. A file that
// does not parse is an error naming it, because a document missing from the
// graph silently shrinks every profile that reaches it.
func Read(files map[string]string) (roadmap.Corpus, error) {
	summaries := map[string]string{}
	profileFiles := map[string]string{}
	texts := map[string]graph.Text{}
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
		case "rfc/full/", "rfc/drafts/":
			text, err := graph.Parse(body)
			if err != nil {
				return roadmap.Corpus{}, fmt.Errorf("%s: %w", rel, err)
			}
			texts[strings.TrimSuffix(name, ".txt")] = text
		default:
			return roadmap.Corpus{}, fmt.Errorf("%s is outside rfc/short/, rfc/profiles/, rfc/full/ and rfc/drafts/, which the roadmap reads", rel)
		}
	}
	requirementStems, err := rfc.RequirementStems(summaries)
	if err != nil {
		return roadmap.Corpus{}, err
	}
	corpus.RequirementStems = requirementStems
	profiles, problems := rfc.ParseProfiles(profileFiles, corpus.SummaryStems)
	if len(problems) != 0 {
		return roadmap.Corpus{}, fmt.Errorf("a profile under rfc/profiles/ did not parse, so its closure is unknown: %s", strings.Join(problems, "; "))
	}
	closures := rfc.ProfileClosures(graph.Build(texts), profiles)
	corpus.ProfileCount = len(closures)
	for profile, members := range closures {
		for _, stem := range members {
			corpus.ProfilesOf[stem] = append(corpus.ProfilesOf[stem], profile)
		}
	}
	return corpus, nil
}
