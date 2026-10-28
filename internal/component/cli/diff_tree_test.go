package cli

import (
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
)

// TestTreeDiffListFollowsWorkingOrder proves AC-8: after the operator adds
// `peer alpha` behind the existing `peer zulu`, `show | compare` prints the
// two peers in the order the working content holds them (zulu, then alpha),
// marks the `peer alpha` line as added, and maps that display line to the
// working line that holds `peer alpha`, so a validation error lands on it.
// The method builds the working content in operator order, runs the tree diff
// against a baseline holding only zulu, and reads the display and its mapping.
func TestTreeDiffListFollowsWorkingOrder(t *testing.T) {
	schema, err := config.YANGSchema()
	if err != nil {
		t.Fatalf("YANG schema: %v", err)
	}
	peer := func(name, ip string) string {
		return "\tpeer " + name + " {\n\t\tconnection {\n\t\t\tremote {\n\t\t\t\tip " + ip + "\n\t\t\t}\n\t\t}\n\t}\n"
	}
	original := "bgp {\n" + peer("zulu", "192.0.2.26") + "}\n"
	working := "bgp {\n" + peer("zulu", "192.0.2.26") + peer("alpha", "192.0.2.1") + "}\n"

	content, lineMapping := annotateContentWithTreeDiff(original, working, schema)
	display := strings.Split(content, "\n")

	zuluDisplay, alphaDisplay := 0, 0
	for i, line := range display {
		if strings.Contains(line, "peer zulu {") {
			zuluDisplay = i + 1
		}
		if strings.Contains(line, "peer alpha {") {
			alphaDisplay = i + 1
		}
	}
	if zuluDisplay == 0 {
		t.Fatalf("the compare view lacks `peer zulu {`:\n%s", content)
	}
	if alphaDisplay == 0 {
		t.Fatalf("the compare view lacks `peer alpha {`:\n%s", content)
	}
	if alphaDisplay < zuluDisplay {
		t.Errorf("the compare view prints peer alpha before peer zulu, want the working order:\n%s", content)
	}
	if !strings.HasPrefix(display[alphaDisplay-1], string(diffAdded)) {
		t.Errorf("display line %d %q is not marked added:\n%s", alphaDisplay, display[alphaDisplay-1], content)
	}
	if strings.HasPrefix(display[zuluDisplay-1], string(diffAdded)) {
		t.Errorf("display line %d %q is marked added, but peer zulu is unchanged:\n%s", zuluDisplay, display[zuluDisplay-1], content)
	}

	workingAlpha := 0
	for i, line := range strings.Split(working, "\n") {
		if strings.Contains(line, "peer alpha {") {
			workingAlpha = i + 1
		}
	}
	if got := lineMapping[alphaDisplay]; got != workingAlpha {
		t.Errorf("display line %d (`peer alpha {`) maps to working line %d, want %d\ndisplay:\n%s\nworking:\n%s",
			alphaDisplay, got, workingAlpha, content, working)
	}
}

// TestTreeDiffUserOrderedMoveIsChange checks a two-entry move and its source
// line mapping against the actual working serializer, not a sorted fixture.
func TestTreeDiffUserOrderedMoveIsChange(t *testing.T) {
	schema := config.NewSchema()
	node := config.List(config.TypeString, config.Field("action", config.Leaf(config.TypeString)))
	node.UserOrdered = true
	schema.Define("term", node)
	original := "term a { action accept; }\nterm b { action deny; }\n"
	working := treeDiffWorking(t, schema, "term b { action deny; }\nterm a { action accept; }\n")
	lines, err := computeTreeAnnotatedDiff(original, working, schema)
	if err != nil {
		t.Fatal(err)
	}
	var openings []diffLine
	for _, line := range lines {
		if strings.HasPrefix(line.Text, "term ") {
			openings = append(openings, line)
		}
	}
	want := []diffLine{
		{diffAdded, "term b {"},
		{diffUnchanged, "term a {"},
		{diffRemoved, "term b {"},
	}
	if !slices.Equal(openings, want) {
		t.Fatalf("entry moves = %#v, want %#v", openings, want)
	}
	assertTreeDiffWorkingMapping(t, lines, working)
}

// TestTreeDiffListShapesFollowWorking checks every list-shaped printer with
// reorder-only, changed, added and removed entries, including nested lists.
func TestTreeDiffListShapesFollowWorking(t *testing.T) {
	block := config.List(config.TypeString, config.Field("value", config.Leaf(config.TypeString)))
	grouped := config.List(config.TypeString, config.Field("value", config.Leaf(config.TypeString)))
	grouped.KeyName = "key"
	cases := []struct {
		name  string
		node  config.Node
		entry func(string, string) string
	}{
		{"block", block, func(key, value string) string { return "item " + key + " { value " + value + "; }\n" }},
		{"grouped", grouped, func(key, value string) string { return "item " + key + " { value " + value + "; }\n" }},
		{"flex", config.Flex(config.Field("value", config.Leaf(config.TypeString))), func(key, value string) string { return "item " + key + " { value " + value + "; }\n" }},
		{"inline", config.InlineList(config.TypeString, config.Field("value", config.Leaf(config.TypeString))), func(key, value string) string { return "item " + key + " value " + value + "\n" }},
		{"members", config.List(config.TypeString, config.Field("value", &config.ValueOrArrayNode{Type: config.TypeString})), func(key, value string) string { return "item " + key + " { value [ " + value + " ]; }\n" }},
		{"nested", config.List(config.TypeString, config.Field("child", block)), func(key, value string) string { return "item " + key + " { child nested { value " + value + "; } }\n" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			schema := config.NewSchema()
			schema.Define("item", tc.node)
			original := tc.entry("zulu", "old") + tc.entry("alpha", "old") + tc.entry("mike", "old")
			for _, changed := range []bool{false, true} {
				value := "old"
				if changed {
					value = "new"
				}
				working := treeDiffWorking(t, schema, tc.entry("mike", "old")+tc.entry("zulu", value)+tc.entry("alpha", "old"))
				lines, err := computeTreeAnnotatedDiff(original, working, schema)
				if err != nil {
					t.Fatal(err)
				}
				marks := 0
				for _, line := range lines {
					if line.Marker != diffUnchanged {
						marks++
						if !changed {
							t.Errorf("plain reorder marked %q: %q", line.Marker, line.Text)
						}
					}
				}
				if changed && marks == 0 {
					t.Error("value edit has no change marker")
				}
				assertTreeDiffWorkingMapping(t, lines, working)
			}
			working := treeDiffWorking(t, schema, tc.entry("zulu", "old")+tc.entry("bravo", "new")+tc.entry("mike", "old"))
			lines, err := computeTreeAnnotatedDiff(original, working, schema)
			if err != nil {
				t.Fatal(err)
			}
			var entries []string
			for _, line := range lines {
				for _, key := range []string{"zulu", "alpha", "bravo", "mike"} {
					if strings.Contains(line.Text, key) {
						entries = append(entries, key)
						if key == "alpha" && line.Marker != diffRemoved {
							t.Errorf("deleted entry is marked %q", line.Marker)
						}
						if key == "bravo" && line.Marker != diffAdded {
							t.Errorf("new entry is marked %q", line.Marker)
						}
					}
				}
			}
			if !slices.Equal(entries, []string{"zulu", "alpha", "bravo", "mike"}) {
				t.Errorf("deletion predecessor order = %v", entries)
			}
			assertTreeDiffWorkingMapping(t, lines, working)
		})
	}
}

// TestTreeDiffNestedOrderAndFlexValues keeps repeated Flex statements and
// nested entry order visible without mistaking presentation order for a change.
func TestTreeDiffNestedOrderAndFlexValues(t *testing.T) {
	schema := config.NewSchema()
	child := config.List(config.TypeString, config.Field("value", config.Leaf(config.TypeString)))
	schema.Define("item", config.Flex(config.Field("child", child)))
	original := "item first\nitem second\nitem zulu { child b { value old; } child a { value old; } }\nitem alpha { }\n"
	working := treeDiffWorking(t, schema, "item first\nitem second\nitem alpha { }\nitem zulu { child a { value old; } child b { value old; } }\n")
	lines, err := computeTreeAnnotatedDiff(original, working, schema)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range lines {
		if line.Marker != diffUnchanged {
			t.Errorf("plain nested reorder marked %q on %q", line.Marker, line.Text)
		}
	}
	assertTreeDiffWorkingMapping(t, lines, working)
}

// TestTreeDiffMoveKeepsInactiveCommentsAndMasking moves an inactive entry with
// comments and a secret; neither move rendering nor source mapping may expose it.
func TestTreeDiffMoveKeepsInactiveCommentsAndMasking(t *testing.T) {
	schema := config.NewSchema()
	secret := config.Leaf(config.TypeString)
	secret.Sensitive = true
	node := config.List(config.TypeString, config.Field("secret", secret))
	node.UserOrdered = true
	schema.Define("item", node)
	original := "item a { secret hidden-a; }\ninactive: item b { secret hidden-b; }\n"
	working := treeDiffWorking(t, schema, "# before b\ninactive: item b { # opening\nsecret hidden-b; # trailing\n}\nitem a { secret hidden-a; }\n")
	lines, err := computeTreeAnnotatedDiff(original, working, schema)
	if err != nil {
		t.Fatal(err)
	}
	moves := 0
	for _, line := range lines {
		if strings.Contains(line.Text, "hidden-") {
			t.Errorf("secret exposed: %q", line.Text)
		}
		if strings.Contains(line.Text, "inactive: item b {") {
			if line.Marker != diffAdded && line.Marker != diffRemoved {
				t.Errorf("inactive moved entry marked %q", line.Marker)
			}
			moves++
		}
	}
	if moves != 2 {
		t.Errorf("inactive move printed %d times, want removed and added", moves)
	}
	_, mapping := renderDiffLines(lines, working)
	source := strings.Split(working, "\n")
	for display, position := range mapping {
		if strings.Contains(lines[display-1].Text, "item ") && !strings.Contains(source[position-1], "item ") {
			t.Errorf("entry line %q mapped to %q", lines[display-1].Text, source[position-1])
		}
	}
}

// TestTreeDiffListMoveSelection checks that insertion/deletion alone moves no
// common key, and that rotations retain their longest common subsequence.
func TestTreeDiffListMoveSelection(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		before, after, moved []string
	}{
		{"add-delete", []string{"a", "b", "c"}, []string{"x", "a", "c", "y"}, nil},
		{"rotate-first", []string{"a", "b", "c"}, []string{"c", "a", "b"}, []string{"c"}},
		{"rotate-last", []string{"a", "b", "c"}, []string{"b", "c", "a"}, []string{"a"}},
		{"empty-key", []string{"", "a", "b"}, []string{"b", "", "a"}, []string{"b"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			moved := diffListMoves(tc.before, tc.after)
			if len(moved) != len(tc.moved) {
				t.Fatalf("moved = %v, want %v", moved, tc.moved)
			}
			for _, key := range tc.moved {
				if !moved[key] {
					t.Errorf("key %q not moved: %v", key, moved)
				}
			}
		})
	}
}

// TestTreeDiffStatementMappingKeepsQuotedHashes checks that trailing comments
// do not shift a changed row and that '#' inside its quoted value stays data.
func TestTreeDiffStatementMappingKeepsQuotedHashes(t *testing.T) {
	working := "# heading\nitem \"a # b\" # note\nitem \"a # c\"\n"
	diffs := []diffLine{
		{diffRemoved, `item "old"`},
		{diffModified, `item "a # b"`},
		{diffUnchanged, `item "a # c"`},
	}
	content, mapping := renderDiffLines(diffs, working)
	if _, ok := mapping[1]; ok {
		t.Error("removed row has a working line")
	}
	if mapping[2] != 2 || mapping[3] != 3 {
		t.Fatalf("mapping = %v, want changed->2 and unchanged->3", mapping)
	}
	issues := []ConfigValidationError{{Message: "bad value", Severity: severityError, Line: 2}}
	styled := strings.Split(highlightValidationIssues(content, issues, nil, mapping, false), "\n")
	plain := strings.Split(content, "\n")
	for i := range plain {
		if (styled[i] != plain[i]) != (i == 1) {
			t.Errorf("highlight at display row %d = %t, want %t", i+1, styled[i] != plain[i], i == 1)
		}
	}
}

// TestTreeDiffFlexValueBoundaries preserves canonical token spelling and the
// working-line mapping for repeated values containing tokenizer delimiters.
func TestTreeDiffFlexValueBoundaries(t *testing.T) {
	schema := config.NewSchema()
	schema.Define("action", config.Flex())
	const working = "action first\n" +
		"action \"\"\n" +
		"action \"#hash\"\n" +
		"action \"quoted \\\"value\\\"\"\n" +
		"action \"line\\nbreak\"\n" +
		"action path\\part\n" +
		"action \"path\\\\ part\"\n" +
		"action \"tab\\tvalue\"\n" +
		"action \"[bracket]\"\n" +
		"action \"multi word\"\n" +
		"action first\n"
	want := strings.Split(strings.TrimSuffix(working, "\n"), "\n")
	for _, tc := range []struct {
		name     string
		original string
		marker   diffMarker
	}{
		{name: "added", marker: diffAdded},
		{name: "unchanged", original: working, marker: diffUnchanged},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lines, err := computeTreeAnnotatedDiff(tc.original, working, schema)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, line := range lines {
				got = append(got, line.Text)
				if line.Marker != tc.marker {
					t.Errorf("marker for %q = %q, want %q", line.Text, line.Marker, tc.marker)
				}
			}
			if !slices.Equal(got, want) {
				t.Errorf("statements = %q, want %q", got, want)
			}
			assertTreeDiffWorkingMapping(t, lines, working)
		})
	}
}

// TestTreeDiffBracketLeafInsideList keeps each bracket statement's name and
// indent through changed-entry traversal, including unchanged leaf context.
// Exact rows and working-line mappings catch a nested buffer read erasing
// the statement prefix before the bracket body is appended.
func TestTreeDiffBracketLeafInsideList(t *testing.T) {
	schema := config.NewSchema()
	schema.Define("item", config.List(config.TypeString,
		config.Field("description", config.Leaf(config.TypeString)),
		config.Field("members", &config.BracketLeafListNode{Type: config.TypeString}),
	))
	for _, tc := range []struct {
		name          string
		before, after string
		want          string
		marker        diffMarker
	}{
		{"changed", "members [ alpha zulu ];", "members [ zulu alpha ];", "\tmembers [ zulu alpha ]", diffModified},
		{"added", "", "members [ zulu alpha ];", "\tmembers [ zulu alpha ]", diffAdded},
		{"removed", "members [ zulu alpha ];", "", "\tmembers [ zulu alpha ]", diffRemoved},
		{"unchanged", "members [ zulu alpha ];", "members [ zulu alpha ];", "\tmembers [ zulu alpha ]", diffUnchanged},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := "item zulu { description before; " + tc.before + " }\nitem alpha { description stable; }\n"
			after := "item zulu { description after; " + tc.after + " }\nitem alpha { description stable; }\n"
			working := treeDiffWorking(t, schema, after)
			lines, err := computeTreeAnnotatedDiff(before, working, schema)
			if err != nil {
				t.Fatal(err)
			}
			found := 0
			for _, line := range lines {
				if line.Text != tc.want {
					continue
				}
				found++
				if line.Marker != tc.marker {
					t.Errorf("bracket marker = %q, want %q", line.Marker, tc.marker)
				}
			}
			if found != 1 {
				t.Errorf("found %d exact bracket rows %q, want one; diff: %+v", found, tc.want, lines)
			}
			assertTreeDiffWorkingMapping(t, lines, working)
		})
	}
}

// treeDiffWorking obtains the statement shape that the editor actually shows.
func treeDiffWorking(t *testing.T, schema *config.Schema, source string) string {
	t.Helper()
	tree, err := config.NewParser(schema).Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	return config.Serialize(tree, schema)
}

// assertTreeDiffWorkingMapping proves that each surviving row is the exact
// working statement it maps to and that removed rows consume no working line.
func assertTreeDiffWorkingMapping(t *testing.T, lines []diffLine, working string) {
	t.Helper()
	_, mapping := renderDiffLines(lines, working)
	statements := workingStatementLines(working)
	source := strings.Split(working, "\n")
	mapped := 0
	for i, line := range lines {
		position, ok := mapping[i+1]
		if line.Marker == diffRemoved {
			if ok {
				t.Errorf("removed row %q maps to working line %d", line.Text, position)
			}
			continue
		}
		mapped++
		if !ok {
			t.Errorf("working row %q has no mapping", line.Text)
			continue
		}
		if line.Text != source[position-1] {
			t.Errorf("row %q maps to %q at %d", line.Text, source[position-1], position)
		}
	}
	if mapped != len(statements) {
		t.Errorf("mapped %d statements, working has %d:\n%s", mapped, len(statements), working)
	}
}

// workingStatementLines answers the 1-based numbers of the lines of working
// that hold a statement, in order: every line but a blank one and a comment
// on its own line.
func workingStatementLines(working string) []int {
	var numbers []int
	for i, line := range splitDiffLines(working) {
		if isCommentOrBlankLine(line) {
			continue
		}
		numbers = append(numbers, i+1)
	}
	return numbers
}
