// Design: docs/architecture/exabgp-bridge.md -- a message as its packet: packets, consolidate, parsed
// Related: migrate_api_events.go -- the grants under test

package migration

import (
	"strings"
	"testing"
)

// TestMigrateCarriesTheFormWords proves the `packets`, `consolidate` and
// `parsed` words of a direction block reach the feed as grants, as ExaBGP links
// them to the process (Configuration._link at the pin).
//
// VALIDATES: receive-consolidate, receive-packets and receive-parsed are
// granted beside receive-update; a `parsed` alone in the send block is not
// written, since the bridge writes the decoded line to a direction that names
// no form.
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
	want := "event [ receive-consolidate receive-packets receive-parsed receive-update send-update ]"
	if !strings.Contains(output, want) {
		t.Errorf("migrated config does not carry %q:\n%s", want, output)
	}
}
