package le

// migratedRefusalsBatchC is the golden of the Phase 3 batch C areas: the nine
// spec lifecycle members and ai tokens
// (plan/post-release/engineering/spec-le-cli-usability.md, AC-17). Each row
// carries the code the line answers through the table. A refusal the table's
// PARSER makes answers 2 (D-P3-stray, D-P3-dash), so a row whose hand-rolled
// parser answered 1 says so in its comment. A refusal the action's BODY makes
// keeps the code it answered before its migration. Every spec member already
// refused each of these lines with 2, so only ai tokens changes a code. Every
// line is refused before any work starts: nothing claims, records, or reads
// the transcript store.
var migratedRefusalsBatchC = []migratedRefusal{
	{"spec current", []string{probeWord}, 2},
	{"spec current", []string{"show", probeWord}, 2},
	{"spec claim", nil, 2},                                   // body: claim's usage, 2 before
	{"spec claim", []string{probeWord}, 2},                   // the first word is not `spec`
	{"spec claim", []string{"spec"}, 2},                      // body: no spec file, 2 before
	{"spec claim", []string{"spec", "a.md", probeWord}, 2},   // a word after the spec file
	{"spec claim", []string{"spec", "-plan/x/spec-y.md"}, 2}, // D-P3-dash
	{"spec release", []string{probeWord}, 2},
	{"spec wip", []string{probeWord}, 2},
	{"spec state", []string{probeWord}, 2},
	{"spec state", []string{"current", probeWord}, 2},
	{"spec state", []string{"latest"}, 2}, // body: no spec, 2 before
	{"spec state", []string{"latest", "spec"}, 2},
	{"spec model", []string{probeWord}, 2},
	{"spec model", []string{"current", "transcript"}, 2},
	{"spec model", []string{"current", "transcript", "relative.jsonl"}, 2}, // body: not absolute, 2 before
	{"spec review", []string{probeWord}, 2},
	{"spec review", []string{"hash"}, 2}, // body: no file, 2 before
	{"spec review", []string{"record", "rounds", "-1"}, 2},
	{"spec review", []string{"record", "spec", "a", "spec", "b"}, 2},
	{"spec review", []string{"check", "spec"}, 2},
	{"spec status", []string{probeWord}, 2},
	{"spec status", []string{"closure"}, 2},                  // body: no closure action, 2 before
	{"spec status", []string{"closure", "list", "check"}, 2}, // body: two closure actions, 2 before
	{"spec status", []string{"closure", "check", "spec"}, 2},
	{"spec citation", []string{probeWord}, 2},
	{"spec citation", []string{"anchors"}, 2}, // body: no spec, 2 before
	{"spec citation", []string{"anchors", "spec"}, 2},
	{"ai tokens", []string{probeWord, "1"}, 2},                    // parser: 1 before
	{"ai tokens", []string{"cap"}, 2},                             // parser: 1 before
	{"ai tokens", []string{"project", "-home-thomas-Code-ze"}, 2}, // D-P3-slug: a slug was read as the value before
	{"ai tokens", []string{"cap", "many"}, 2},                     // a malformed uint, refused at parse (D-Q8): 1 before
	{"ai tokens", []string{"top", "0"}, 1},
}
