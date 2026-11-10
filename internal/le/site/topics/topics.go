// Design: website/AI.md -- the controlled tag vocabulary update reports and news fragments draw on.
// Related: ../changes.go -- loadTopicVocabulary, which adds the site's category check.
// Related: ../../commit/newsfragment.go -- the news-fragment gate, which judges a fragment's topic.

// Package topics reads website/data/topics.json, the controlled tag
// vocabulary. It is the one reader of that file in le.
//
// It is a leaf: it parses bytes and imports nothing from the site renderer,
// so the commit tooling reads the vocabulary from a commit's prospective tree
// without linking the renderer or running its init. The check that a family's
// category has a site color stays in the site package, which owns the colors.
package topics

import (
	"encoding/json"
	"errors"
	"fmt"
)

// File is the vocabulary's path inside the website source tree, website/.
const File = "data/topics.json"

// Parse reads the vocabulary from content: each tag family, mapped to the
// category its chip takes. name is the file the content came from, used in
// every refusal so the reader knows which file to fix.
//
// A vocabulary that states no tag is refused rather than answered empty: an
// empty map would refuse every topic for a reason the author cannot see.
func Parse(name string, content []byte) (map[string]string, error) {
	var vocabulary struct {
		Tags map[string]string `json:"tags"`
	}
	if err := json.Unmarshal(content, &vocabulary); err != nil {
		return nil, fmt.Errorf("read %s: %w", name, err)
	}
	if len(vocabulary.Tags) == 0 {
		return nil, errors.New(name + ` states no tag; add a "tags" object mapping each tag family to its category`)
	}
	return vocabulary.Tags, nil
}
