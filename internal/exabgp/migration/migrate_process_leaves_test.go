// Design: docs/architecture/exabgp-bridge.md -- process leaves and per-neighbor bus scope migration tests
// Related: migrate.go -- migrateProcesses and exabgpEncoder/exabgpOnExit/exabgpRespawn
// Related: migrate_bus.go -- endpoint emission and receive/send filtering
//
// VALIDATES: every leaf of an ExaBGP `process` block (`run`, `encoder`,
// `respawn`, `on-exit`) and every neighbor-to-process `api` relation reaches
// its `bus { endpoint <name> { ... } }` block cleanly.
// PREVENTS: scripts receiving events from unrelated neighbors, losing `--encoder
// json`, or silently inventing encoders or `on-exit` policies.

package migration

import (
	"strings"
	"testing"
)

// migrateSource parses one ExaBGP config and answers the migrated ze config.
func migrateSource(t *testing.T, source string) string {
	t.Helper()
	tree, err := ParseExaBGPConfig(source)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := MigrateFromExaBGP(tree)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return SerializeTree(result.Tree)
}

// TestMigrateCarriesTheEncoderAndTheFeedsOfEachProcess is the api-multiple-api
// shape. Each script reaches its own bus endpoint with its encoder flag (only
// json; text is default) and with the single neighbor that named it.
//
// VALIDATES: Separate endpoints keep separate send > to and receive > from none
// scopes, with `--encoder json` only on the json process.
// PREVENTS: Cross-neighbor leakage between two ExaBGP processes.
func TestMigrateCarriesTheEncoderAndTheFeedsOfEachProcess(t *testing.T) {
	output := migrateSource(t, `
process public {
	run ./run/api-multiple-public.run;
	encoder text;
}

process private {
	run ./run/api-multiple-private.run;
	encoder json;
}

neighbor 127.0.0.1 {
	router-id 1.2.3.4;
	local-as 1;
	peer-as 1;
	api {
		processes [ public ];
	}
}

neighbor 192.168.0.1 {
	router-id 1.2.3.4;
	local-as 2;
	peer-as 2;
	api {
		processes [ private ];
	}
}
`)

	want := []string{
		"endpoint public {",
		`run "ze exabgp plugin ./run/api-multiple-public.run"`,
		"endpoint private {",
		`run "ze exabgp plugin --encoder json ./run/api-multiple-private.run"`,
	}
	for _, line := range want {
		if !strings.Contains(output, line) {
			t.Errorf("migrated config does not carry %q:\n%s", line, output)
		}
	}

	// Neither script may be scoped to the other's neighbor: peer-1 is 127.0.0.1
	// (public) and peer-2 is 192.168.0.1 (private).
	public := endpointBlock(t, output, "public")
	if !strings.Contains(public, "/bgp/group/default/peer/peer-1") || !strings.Contains(public, "from none") {
		t.Errorf("public endpoint missing peer-1 send scope or from none:\n%s", public)
	}
	if strings.Contains(public, "/bgp/group/default/peer/peer-2") {
		t.Errorf("the public endpoint is fed by the private neighbor:\n%s", public)
	}

	private := endpointBlock(t, output, "private")
	if !strings.Contains(private, "/bgp/group/default/peer/peer-2") || !strings.Contains(private, "from none") {
		t.Errorf("private endpoint missing peer-2 send scope or from none:\n%s", private)
	}
	if strings.Contains(private, "/bgp/group/default/peer/peer-1") {
		t.Errorf("the private endpoint is fed by the public neighbor:\n%s", private)
	}
}

// TestMigrateCarriesTheEventsEachNeighborGrants is the api-check shape: an api
// block that names the message kinds it feeds. ExaBGP grants exactly those, so
// the script sees no OPEN and no KEEPALIVE, and the endpoint's receive allow
// list carries only `bgp:update`.
//
// VALIDATES: receive/send update grants map to `allow [ bgp:update ]` with no
// spurious messages and no direction restriction when both directions are granted.
// PREVENTS: Leaking ungranted BGP message kinds to an ExaBGP script.
func TestMigrateCarriesTheEventsEachNeighborGrants(t *testing.T) {
	output := migrateSource(t, `
process watcher {
	run ./run/watcher.run;
	encoder text;
}

neighbor 127.0.0.1 {
	router-id 1.2.3.4;
	local-as 1;
	peer-as 1;
	api connection {
		processes [ watcher ];
		receive {
			parsed;
			update;
		}
		send {
			parsed;
			update;
		}
	}
}
`)

	if want := "allow [ bgp:update ]"; !strings.Contains(output, want) {
		t.Errorf("migrated config does not carry %q:\n%s", want, output)
	}
	for _, unwanted := range []string{"bgp:open", "bgp:keepalive", "bgp:state", "bgp:connected", "tag direction"} {
		if strings.Contains(output, unwanted) {
			t.Errorf("migrated config carries unexpected %q:\n%s", unwanted, output)
		}
	}
}

// endpointBlock answers the body of one `endpoint <name> { ... }` block of a
// migrated config, so a check on one script cannot pass on another's text.
func endpointBlock(t *testing.T, output, name string) string {
	t.Helper()
	_, rest, found := strings.Cut(output, "\tendpoint "+name+" {\n")
	if !found {
		t.Fatalf("migrated config has no endpoint %s:\n%s", name, output)
	}
	body, _, closed := strings.Cut(rest, "\n\t}\n")
	if !closed {
		t.Fatalf("endpoint %s block is not closed:\n%s", name, output)
	}
	return body
}

// TestMigrateLeavesTheEncoderUnwrittenWhenExaBGPStatedNone checks the migration
// omits `--encoder` when the ExaBGP process left `encoder` unset, letting the
// wrapper's default (`text`) decide.
//
// VALIDATES: Omitted ExaBGP `encoder` writes no `--encoder` option.
// PREVENTS: Migration inventing an `--encoder` flag the ExaBGP config did not state.
func TestMigrateLeavesTheEncoderUnwrittenWhenExaBGPStatedNone(t *testing.T) {
	output := migrateSource(t, `
process quiet {
	run ./run/quiet.run;
}

neighbor 10.0.0.1 {
	router-id 1.1.1.1;
	local-as 65001;
	peer-as 65002;
	api {
		processes [ quiet ];
	}
}
`)

	if strings.Contains(output, "--encoder") {
		t.Errorf("migrated config states an encoder the ExaBGP config did not:\n%s", output)
	}
}

// TestMigrateRefusesToInventAnEncoder checks an unreadable word is reported as
// a warning rather than replaced.
//
// VALIDATES: Unknown `encoder` emits a warning naming process and word and writes
// no `--encoder` option.
// PREVENTS: Silently rewriting an invalid encoder to json or text.
func TestMigrateRefusesToInventAnEncoder(t *testing.T) {
	tree, err := ParseExaBGPConfig(`
process odd {
	run ./run/odd.run;
	encoder yaml;
}

neighbor 10.0.0.1 {
	router-id 1.1.1.1;
	local-as 65001;
	peer-as 65002;
	api {
		processes [ odd ];
	}
}
`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := MigrateFromExaBGP(tree)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}

	warned := false
	for _, warning := range result.Warnings {
		if strings.Contains(warning, "odd") && strings.Contains(warning, "yaml") {
			warned = true
		}
	}
	if !warned {
		t.Errorf("warnings = %v, want one naming the process and the word", result.Warnings)
	}
	if output := SerializeTree(result.Tree); strings.Contains(output, "--encoder") {
		t.Errorf("migrated config states an encoder for a word it could not read:\n%s", output)
	}
}

// TestMigrateOnExit checks `on-exit keep` becomes `external { on-exit keep }`,
// while `on-exit withdraw` and unset (under default API version) omit `on-exit`
// because withdrawal is the endpoint default (E-2, E-6, Q-E11).
//
// VALIDATES: AC-41 / S38 on-exit emission on external endpoints.
// PREVENTS: Writing redundant `on-exit withdraw` or leaking `--on-exit` onto `run`.
func TestMigrateOnExit(t *testing.T) {
	const neighbor = `
neighbor 10.0.0.1 {
	router-id 1.1.1.1;
	local-as 65001;
	peer-as 65002;
	api {
		processes [ helper ];
	}
}
`
	keepOut := migrateSource(t, "process helper {\n\trun ./run/helper.run;\n\ton-exit keep;\n}\n"+neighbor)
	if !strings.Contains(keepOut, "on-exit keep") {
		t.Errorf("on-exit keep not carried:\n%s", keepOut)
	}
	if strings.Contains(keepOut, "--on-exit") {
		t.Errorf("migrated run line leaked --on-exit:\n%s", keepOut)
	}

	withdrawOut := migrateSource(t, "process helper {\n\trun ./run/helper.run;\n\ton-exit withdraw;\n}\n"+neighbor)
	if strings.Contains(withdrawOut, "on-exit") {
		t.Errorf("on-exit withdraw is the default and should be omitted:\n%s", withdrawOut)
	}

	unsetOut := migrateSource(t, "process helper {\n\trun ./run/helper.run;\n}\n"+neighbor)
	if strings.Contains(unsetOut, "on-exit") {
		t.Errorf("migrated config states an on-exit the ExaBGP config did not:\n%s", unsetOut)
	}
}
