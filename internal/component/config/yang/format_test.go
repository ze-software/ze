package yang

import "testing"

// TestTypeFormatReader pins the source LeafValueFormat reads a pattern's
// format from, in its order: the nearest typedef's ze:help, the first sentence
// of its description, the leaf's own ze:help, then the typedef's name alone,
// and nothing for an inline pattern on a leaf with no help (AC-37, AC-40).
// Method: read each probe leaf, whose type takes exactly one of those routes,
// and compare the text, the type name and the placeholder.
func TestTypeFormatReader(t *testing.T) {
	v := refusalProbe(t)
	probe := v.schema.GetEntry("ze-test-refusal-conf").Dir["probe"]
	if probe == nil {
		t.Fatal("the probe container did not resolve")
	}

	cases := []struct {
		leaf, text, typeName, placeholder string
	}{
		{"rate", "Traffic rate (for example 10mbit)", "rate", "<rate>"},
		{"described", "Lower-case letters only", "described", "<described>"},
		{"bare-help", "An upper-case word", "bare", "<bare>"},
		{"bare", "", "bare", "<bare>"},
		{"inline-help", "Digits only", "", "<value>"},
		{"inline", "", "", "<value>"},
	}
	for _, c := range cases {
		t.Run(c.leaf, func(t *testing.T) {
			leaf := probe.Dir[c.leaf]
			if leaf == nil {
				t.Fatalf("probe declares no %s", c.leaf)
			}
			format := LeafValueFormat(leaf, leaf.Type)
			if format.Text() != c.text {
				t.Errorf("text %q, want %q", format.Text(), c.text)
			}
			if format.TypeName() != c.typeName {
				t.Errorf("type name %q, want %q", format.TypeName(), c.typeName)
			}
			if format.Placeholder() != c.placeholder {
				t.Errorf("placeholder %q, want %q", format.Placeholder(), c.placeholder)
			}
		})
	}

	sentences := map[string]string{
		"One sentence.":                      "One sentence",
		"First one. Second one.":             "First one",
		"Rate string (for example 10.5mbit)": "Rate string (for example 10.5mbit)",
		"Spread\n   over   lines. And more.": "Spread over lines",
		"":                                   "",
	}
	for text, want := range sentences {
		if got := firstSentence(text); got != want {
			t.Errorf("firstSentence(%q) = %q, want %q", text, got, want)
		}
	}
}
