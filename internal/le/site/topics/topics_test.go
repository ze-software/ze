package topics

import (
	"maps"
	"strings"
	"testing"
)

// TestParseReadsTheTagFamilies parses vocabulary bytes the way the site build
// and the news-fragment gate both do.
//
// VALIDATES: the tags object is answered as family to category, other keys
// are ignored; malformed JSON and a vocabulary with no tag are refused, each
// refusal naming the file it came from.
// PREVENTS: an empty vocabulary answered as a valid one, which would refuse
// every topic for a reason nobody can see, and a refusal that does not say
// which file to fix.
func TestParseReadsTheTagFamilies(t *testing.T) {
	t.Parallel()
	const name = "website/data/topics.json"
	vocabulary, err := Parse(name, []byte(`{"categories":["routing"],"tags":{"BGP":"routing","MCP":"automate"}}`))
	if err != nil {
		t.Fatalf("Parse refused a valid vocabulary: %v", err)
	}
	if want := map[string]string{"BGP": "routing", "MCP": "automate"}; !maps.Equal(vocabulary, want) {
		t.Fatalf("Parse = %v, want %v", vocabulary, want)
	}

	for _, content := range []string{`{"tags":`, `{"tags":{}}`, `{"categories":["routing"]}`} {
		if _, err := Parse(name, []byte(content)); err == nil {
			t.Errorf("Parse accepted %s", content)
		} else if !strings.Contains(err.Error(), name) {
			t.Errorf("the refusal of %s does not name %s: %v", content, name, err)
		}
	}
}
