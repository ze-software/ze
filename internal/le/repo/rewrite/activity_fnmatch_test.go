package reporewrite

import "testing"

// TestFnmatchAnswersTheGlobOnEveryCall verifies the activity exclude matcher
// answers each glob the same way on the first call, which compiles the
// pattern, and on every later call, which reuses the compiled expression.
//
// The method runs every case three times over the default exclude shapes plus a
// bracket class, a negated class, a literal dot, and a class that does not
// compile, which matches nothing.
func TestFnmatchAnswersTheGlobOnEveryCall(t *testing.T) {
	cases := []struct {
		pattern string
		value   string
		want    bool
	}{
		{"tmp/*", "tmp/session/a.go", true},
		{"tmp/*", "internal/tmp/a.go", false},
		{"*/vendor/*", "cmd/vendor/x.go", true},
		{"*/vendor/*", "vendor/x.go", false},
		{"pages/activity.html", "pages/activity.html", true},
		{"pages/activity.html", "pages/activityXhtml", false},
		{"file?.go", "file1.go", true},
		{"file?.go", "file12.go", false},
		{"[ab].go", "a.go", true},
		{"[ab].go", "c.go", false},
		{"[!ab].go", "c.go", true},
		{"[!ab].go", "a.go", false},
		{"[z-a].go", "a.go", false},
	}
	for round := range 3 {
		for _, tc := range cases {
			if got := fnmatch(tc.pattern, tc.value); got != tc.want {
				t.Errorf("round %d: fnmatch(%q, %q) = %v, want %v", round, tc.pattern, tc.value, got, tc.want)
			}
		}
	}
}
