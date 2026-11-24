// Design: docs/architecture/exabgp-bridge.md -- a message as its packet: packets, consolidate, parsed
// Related: migrate_api_events.go -- the grants under test
// Related: migrate_bus.go -- collectFormSpecs and buildWrapperRunCommand

package migration

import (
	"strings"
	"testing"
)

// TestMigrateCarriesTheFormWords proves the `packets`, `consolidate` and
// `parsed` words of a direction block reach the endpoint's wrapper run line as
// `--form` flags while bus message grants reach `receive > allow`.
//
// VALIDATES: receive-consolidate, receive-packets and receive-parsed are
// emitted as `--form` flags beside `allow [ bgp:update ]`; a `parsed` alone in
// the send block emits no `--form`, since parsed is the wrapper's default.
// PREVENTS: a migrated `receive { packets; consolidate; }` script reading the
// decoded line alone, which every migrated script did until 2026-10-22.
func TestMigrateCarriesTheFormWords(t *testing.T) {
	output := migrateSource(t, `
process watcher {
	run ./run/watcher.run;
	encoder json;
}

neighbor 127.0.0.1 {
	router-id 1.2.3.4;
	local-as 1;
	peer-as 1;
	api connection {
		processes [ watcher ];
		receive {
			parsed;
			packets;
			consolidate;
			update;
		}
		send {
			parsed;
			update;
		}
	}
}
`)
	wantRun := `run "ze exabgp plugin --encoder json --form receive-consolidate --form receive-packets --form receive-parsed ./run/watcher.run"`
	if !strings.Contains(output, wantRun) {
		t.Errorf("migrated config does not carry %q:\n%s", wantRun, output)
	}
	if strings.Contains(output, "send-parsed") {
		t.Errorf("migrated config emitted redundant send-parsed form:\n%s", output)
	}
	if !strings.Contains(output, "allow [ bgp:update ]") {
		t.Errorf("migrated config does not carry allow [ bgp:update ]:\n%s", output)
	}
}
