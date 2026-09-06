// Tests for the neighbor-level api blocks and for the neighbor leaves that
// move into ze's behavior container.

package migration

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
)

// migrateText parses and migrates one ExaBGP config, and answers the ze config
// text it produces.
func migrateText(t *testing.T, input string) (string, *MigrateResult) {
	t.Helper()

	tree, err := ParseExaBGPConfig(input)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := MigrateFromExaBGP(tree)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return SerializeTree(result.Tree), result
}

// TestMigrateNamedAPIBlock proves the name ExaBGP allows after `api` is
// accepted and changes nothing.
//
// VALIDATES: `api <name> { processes [ p ]; }` binds the same process as the
// anonymous form.
// PREVENTS: the parser refusing a named block with "expected '{' after api",
// which stopped five of the ported ExaBGP compatibility configs.
func TestMigrateNamedAPIBlock(t *testing.T) {
	named := `
process announce-routes {
	run ./run/announce.run;
	encoder json;
}
neighbor 127.0.0.1 {
	local-as 1;
	peer-as 1;
	api announce {
		processes [ announce-routes ];
	}
}
`
	anonymous := strings.Replace(named, "api announce {", "api {", 1)

	namedOutput, _ := migrateText(t, named)
	anonymousOutput, _ := migrateText(t, anonymous)

	if !strings.Contains(namedOutput, "attach process exabgp-bridge") {
		t.Fatalf("named api block did not attach the bridge:\n%s", namedOutput)
	}
	if namedOutput != anonymousOutput {
		t.Errorf("the name changed the migrated config:\nnamed:\n%s\nanonymous:\n%s", namedOutput, anonymousOutput)
	}
}

// TestMigrateSeveralAPIBlocksAreRead proves every api block of a neighbor is
// read, not just the first.
//
// VALIDATES: a second api block asking for `receive { update; }` is what
// injects the RIB plugin, and a process named twice is attached once.
// PREVENTS: the flatten ExaBGP performs over a neighbor's api blocks
// (ParseAPI.flatten) being reduced here to "the first block wins", which
// silently drops the settings of every other block.
func TestMigrateSeveralAPIBlocksAreRead(t *testing.T) {
	input := `
process announce-routes {
	run ./run/announce.run;
	encoder json;
}
neighbor 127.0.0.1 {
	local-as 1;
	peer-as 1;
	api speaking {
		processes [ announce-routes ];
	}
	api listening {
		processes [ announce-routes ];
		receive {
			parsed;
			update;
		}
	}
}
`
	output, result := migrateText(t, input)

	if !result.RIBInjected {
		t.Errorf("the second api block asks for received updates, so the RIB plugin is owed:\n%s", output)
	}
	if got := strings.Count(output, "attach process exabgp-bridge"); got != 1 {
		t.Errorf("the bridge is attached %d times, want once:\n%s", got, output)
	}
}

// TestMigrateTemplateAPIBlockInherited proves an api block written in a
// template reaches the peer that inherits it.
//
// VALIDATES: the api entries of a template survive inheritance expansion.
// PREVENTS: a migrated config that starts the operator's script and attaches it
// to no peer, so no announcement reaches the wire.
func TestMigrateTemplateAPIBlockInherited(t *testing.T) {
	input := `
process check-and-announce {
	run ./run/api-check.run;
	encoder text;
}
template {
	neighbor controler {
		api connection {
			processes [ check-and-announce ];
		}
	}
}
neighbor 127.0.0.1 {
	inherit controler;
	local-as 65512;
	peer-as 65512;
}
`
	output, _ := migrateText(t, input)

	if !strings.Contains(output, "attach process exabgp-bridge") {
		t.Errorf("the inherited api block attached no process:\n%s", output)
	}
}

// TestMigrateManualEOR proves the ExaBGP neighbor leaf reaches ze's behavior
// container, and that ze's own schema accepts what is written.
//
// VALIDATES: `manual-eor true` migrates to `behavior { manual-eor true }`, the
// leaf ze-bgp-conf.yang declares for the same purpose.
// PREVENTS: the leaf being dropped, which turns a config that says "the script
// sends End-of-RIB" into one that says nothing at all.
func TestMigrateManualEOR(t *testing.T) {
	input := `
neighbor 127.0.0.1 {
	local-as 1;
	peer-as 1;
	manual-eor true;
}
`
	output, _ := migrateText(t, input)

	if !strings.Contains(output, "manual-eor true") {
		t.Fatalf("manual-eor is missing from the migrated config:\n%s", output)
	}

	// The ze parser is the reader of this text, so it decides whether the leaf
	// was written where ze declares it.
	schema, err := config.YANGSchema()
	if err != nil {
		t.Fatalf("load ze schema: %v", err)
	}
	if _, err := config.NewParser(schema).Parse(output); err != nil {
		t.Fatalf("ze refuses the migrated config: %v\n%s", err, output)
	}
}

// TestMigrateManualEORInheritedFromTemplate proves a template carries the leaf
// to the neighbors that inherit it.
//
// VALIDATES: manual-eor survives inheritance expansion.
// PREVENTS: a leaf that works when written on the neighbor and vanishes when
// written once for a group of them.
func TestMigrateManualEORInheritedFromTemplate(t *testing.T) {
	input := `
template {
	neighbor controler {
		manual-eor true;
	}
}
neighbor 127.0.0.1 {
	inherit controler;
	local-as 1;
	peer-as 1;
}
`
	output, _ := migrateText(t, input)

	if !strings.Contains(output, "manual-eor true") {
		t.Errorf("the inherited manual-eor is missing from the migrated config:\n%s", output)
	}
}

// TestMigrateProcessesMatch proves a pattern selects the process it names, so
// the peer attaches the bridge that runs it.
//
// VALIDATES: `processes-match [ "^add" ]` binds the declared `add-remove`
// process, and writes the same peer the literal `processes [ add-remove ]`
// form writes.
// PREVENTS: the leaf being read as nothing, which produced a peer with no
// attached process. Every route the script then announced was refused at
// dispatch: "this peer does not attach that process with the send permission
// it needs" (internal/component/bgp/reactor/send_permission.go).
func TestMigrateProcessesMatch(t *testing.T) {
	matched := `
process add-remove {
	run ./run/api-announce.run;
	encoder json;
}
neighbor 127.0.0.1 {
	local-as 1;
	peer-as 1;
	api {
		processes-match [ "^add" ];
	}
}
`
	literal := strings.Replace(matched, `processes-match [ "^add" ];`, "processes [ add-remove ];", 1)

	matchedOutput, _ := migrateText(t, matched)
	literalOutput, _ := migrateText(t, literal)

	if !strings.Contains(matchedOutput, "attach process exabgp-bridge") {
		t.Fatalf("the pattern attached no process:\n%s", matchedOutput)
	}
	if matchedOutput != literalOutput {
		t.Errorf("the pattern form and the literal form disagree:\nprocesses-match:\n%s\nprocesses:\n%s",
			matchedOutput, literalOutput)
	}
}

// TestMigrateProcessesMatchAnchoring pins where ExaBGP anchors the pattern.
//
// VALIDATES: the pattern is anchored at the start of the process name and free
// at the end, which is what Python's re.match does, and it is case-sensitive.
// PREVENTS: Go's unanchored regexp semantics selecting a process ExaBGP would
// not have selected, so the migrated peer runs a script the ExaBGP config left
// out.
func TestMigrateProcessesMatchAnchoring(t *testing.T) {
	cases := []struct {
		name    string
		pattern string
		matches bool
	}{
		{"start of the name", "^add", true},
		{"start without the caret", "add", true},
		{"whole name", "add-remove", true},
		{"alternation", "nothing|add", true},
		{"inside the name but not at the start", "remove", false},
		{"other case", "ADD", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := matchProcessNames("announce", []string{tc.pattern},
				[]ExternalProcess{{Name: "add-remove", RunCmd: "./run/api-announce.run"}})
			if !tc.matches {
				if err == nil {
					t.Fatalf("pattern %q selected %v, want nothing", tc.pattern, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("pattern %q selected nothing: %v", tc.pattern, err)
			}
			if len(got) != 1 || got[0] != "add-remove" {
				t.Errorf("pattern %q selected %v, want [add-remove]", tc.pattern, got)
			}
		})
	}
}

// TestMigrateProcessesMatchSelectsNothing proves a pattern set that matches no
// declared process stops the migration.
//
// VALIDATES: the error names the pattern and the process names it was matched
// against.
// PREVENTS: a migrated config whose peer attaches nothing, which reads as a
// successful migration and carries no route. ExaBGP refuses the same config
// rather than running it (src/exabgp/configuration/configuration.py, validate).
func TestMigrateProcessesMatchSelectsNothing(t *testing.T) {
	input := `
process add-remove {
	run ./run/api-announce.run;
	encoder json;
}
neighbor 127.0.0.1 {
	local-as 1;
	peer-as 1;
	api {
		processes-match [ "^watch" ];
	}
}
`
	tree, err := ParseExaBGPConfig(input)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	_, err = MigrateFromExaBGP(tree)
	if err == nil {
		t.Fatal("the migration accepted a pattern that selects no process")
	}
	for _, want := range []string{"^watch", "add-remove"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error does not name %q: %v", want, err)
		}
	}
}

// TestMigrateProcessesAndProcessesMatchRefused proves the two lists are not
// silently merged.
//
// VALIDATES: an api block carrying both lists is refused.
// PREVENTS: the migration inventing a rule ExaBGP does not have. ExaBGP calls
// them mutually exclusive and stops (configuration.py, validate), so a config
// that carries both never ran, and any behavior chosen here would be new.
func TestMigrateProcessesAndProcessesMatchRefused(t *testing.T) {
	input := `
process add-remove {
	run ./run/api-announce.run;
	encoder json;
}
neighbor 127.0.0.1 {
	local-as 1;
	peer-as 1;
	api {
		processes [ add-remove ];
		processes-match [ "^add" ];
	}
}
`
	tree, err := ParseExaBGPConfig(input)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	_, err = MigrateFromExaBGP(tree)
	if err == nil {
		t.Fatal("the migration accepted both process lists on one api block")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("the error does not say why: %v", err)
	}
}
