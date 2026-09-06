// GOAL: every leaf of an ExaBGP `process` block that changes what ze does
// reaches the migrated config, and the neighbor-to-process relation reaches it
// too.
//
// METHOD: migrate the shape test/exabgp-compat/etc/api-multiple-api.conf
// declares -- two processes, two neighbors, each naming one -- and read the
// bridge root the migration wrote.
//
// PREVENTS: a config asking for the text event format being migrated into a
// bridge that answers JSON, which is what happened until 2026-09-06 because
// nothing read the `encoder` leaf; and a two-process config whose scripts each
// receive both neighbors' events, because the relation reached nothing that
// could filter.

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
// shape. Each script must reach the bridge with its own encoder and with the
// one neighbor that named it.
//
// Neither api block names a message kind, and ExaBGP reads that as a grant of
// NOTHING (ParseAPI.flatten takes `data.get(action, False)`), so each feed
// block carries an empty event list rather than every event.
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
		"process private {",
		"encoder json",
		"feed 192.168.0.1 {",
		"process public {",
		"encoder text",
		"feed 127.0.0.1 {",
	}
	for _, line := range want {
		if !strings.Contains(output, line) {
			t.Errorf("migrated config does not carry %q:\n%s", line, output)
		}
	}

	// Neither script may be fed by the other's neighbor: a script fed by both
	// receives both neighbors' events, which is the defect the relation exists
	// to remove.
	public := processBlock(t, output, "public")
	if strings.Contains(public, "192.168.0.1") {
		t.Errorf("the public script is fed by the private neighbor:\n%s", public)
	}
	private := processBlock(t, output, "private")
	if strings.Contains(private, "127.0.0.1") {
		t.Errorf("the private script is fed by the public neighbor:\n%s", private)
	}
}

// TestMigrateCarriesTheEventsEachNeighborGrants is the api-check shape: an api
// block that names the message kinds it feeds. ExaBGP grants exactly those, so
// the script sees no OPEN and no KEEPALIVE, and the bridge needs the grant to
// know that.
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

	if want := "event [ receive-update send-update ]"; !strings.Contains(output, want) {
		t.Errorf("migrated config does not carry %q:\n%s", want, output)
	}
	for _, unwanted := range []string{"receive-open", "send-keepalive", "neighbor-changes"} {
		if strings.Contains(output, unwanted) {
			t.Errorf("migrated config grants %q, which the api block did not:\n%s", unwanted, output)
		}
	}
}

// processBlock answers the body of one `process <name> { ... }` block of a
// migrated config, so a check on one script cannot pass on another's text.
func processBlock(t *testing.T, output, name string) string {
	t.Helper()
	_, rest, found := strings.Cut(output, "process "+name+" {")
	if !found {
		t.Fatalf("migrated config has no process %s:\n%s", name, output)
	}
	body, _, closed := strings.Cut(rest, "\n\t\t}")
	if !closed {
		t.Fatalf("process %s block is not closed:\n%s", name, output)
	}
	return body
}

// TestMigrateLeavesTheEncoderUnwrittenWhenExaBGPStatedNone checks the migration
// states no encoder rather than an invented one. ExaBGP 6 answers every process
// in JSON whatever its leaf says, and ze declares the 6.0.0 envelope, so an
// ExaBGP config that stated nothing keeps the JSON ze already answered.
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

	if strings.Contains(output, "encoder") {
		t.Errorf("migrated config states an encoder the ExaBGP config did not:\n%s", output)
	}
}

// TestMigrateRefusesToInventAnEncoder checks an unreadable word is REPORTED
// rather than replaced. A migration that silently wrote `json` over a word it
// did not understand would hand the operator a config that runs and answers the
// wrong format.
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
	if output := SerializeTree(result.Tree); strings.Contains(output, "encoder") {
		t.Errorf("migrated config states an encoder for a word it could not read:\n%s", output)
	}
}
