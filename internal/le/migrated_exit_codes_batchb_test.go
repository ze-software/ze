package le

// migratedRefusalsBatchB is the golden of the Phase 3 batch B areas
// (plan/post-release/engineering/spec-le-cli-usability.md, AC-17). Each row
// carries the code the line answers through the table. A refusal the table's
// PARSER makes answers 2 (D-P3-stray, D-P3-dash), so a row whose hand-rolled
// parser answered 1 says so in its comment. A refusal the action's BODY makes
// keeps the code it answered before its migration. Every line is refused
// before any work starts: nothing runs, writes, posts or reads the tree's
// changes.
var migratedRefusalsBatchB = []migratedRefusal{
	{"job", []string{probeWord}, 2},
	{"job", []string{"run", "label"}, 2},
	{"job", []string{"run", "label", "x"}, 2}, // body: no command keyword, 2 before
	{"verify summary", []string{probeWord}, 2},
	{"repo inventory", []string{probeWord}, 2},  // parser: 1 before
	{"repo bootstraps", []string{probeWord}, 2}, // parser: 1 before
	{"test stress-repro", []string{probeWord}, 2},
	{"test stress-repro", []string{"run", "suite", "-help"}, 2}, // D-P3-dash
	{"test stress-repro", []string{"run", "suite", "bgp", "iterations", "-1"}, 2},
	{"test stress-repro", []string{"run", "suite", "bgp", "suite", "bgp"}, 2},
	{"test stress-repro", []string{"run", "test", "x"}, 2}, // body: no suite, 2 before
	{"repo working-tree", []string{"3"}, 2},                // parser: 1 before
	{"repo working-tree", []string{"max-areas"}, 2},        // parser: 1 before
	{"repo working-tree", []string{"max-areas", "-1"}, 2},  // parser: 1 before
	{"repo working-tree", []string{"max-areas", "many"}, 1},
	{"go extract", []string{"a.go"}, 2},                   // parser: 1 before
	{"go extract", []string{"source", "a.go", "dest"}, 2}, // parser: 1 before
	{"go extract", []string{"source", "a.go", "symbol", "X"}, 1},
	{"weekly", []string{"publish"}, 2},           // parser: 1 before
	{"weekly", []string{"resume-from", "-1"}, 2}, // parser: 1 before
	{"weekly", []string{"channel", "general"}, 1},
	{"weekly", []string{"confirm", "force"}, 1},
}
