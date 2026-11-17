// Design: docs/architecture/site-facts.md -- a display count is not a duration with the same digits
package site

import "testing"

// TestDisplayCountDoesNotMatchDuration keeps the plain-figure guard effective
// for a copied count without treating a benchmark's suffixed duration as that
// count. The mixed case also proves that the duration does not hide a copy.
func TestDisplayCountDoesNotMatchDuration(t *testing.T) {
	figures := []publishedFigure{{Key: "rfc.proven_display", Text: "2,209"}}
	for _, test := range []struct {
		text string
		want int
	}{
		{"2,209ms", 0},
		{"2,209", 1},
		{"2,209 requirements", 1},
		{"2,209ms and 2,209 requirements", 1},
	} {
		t.Run(test.text, func(t *testing.T) {
			got := figuresIn("page.html", test.text, figures)
			if len(got) != test.want {
				t.Errorf("findings for %q = %v, want %d", test.text, got, test.want)
			}
		})
	}
}
