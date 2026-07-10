// VALIDATES: zeConfigFileName gives the first ze-daemon stdin block the canonical
// ze-bgp.conf name (whatever the block is called) and each additional distinct
// block its own ze-<block>.conf, while reusing a block returns its assigned file;
// sanitizeConfigBlock reduces a block name to a filesystem-safe token.
// PREVENTS: two concurrent `ze -` daemons in one test clobbering a single
// ze-bgp.conf (which makes an IKE responder+initiator pair load the same config
// and never establish), or a rewrite/single-daemon test losing ze-bgp.conf.

package runner

import "testing"

func TestZeConfigFileName(t *testing.T) {
	rec := &Record{}
	if got := zeConfigFileName(rec, "config"); got != "ze-bgp.conf" {
		t.Errorf("first block = %q, want ze-bgp.conf", got)
	}
	if got := zeConfigFileName(rec, "responder"); got != "ze-responder.conf" {
		t.Errorf("second distinct block = %q, want ze-responder.conf", got)
	}
	// Reusing a block (a restart) returns its already-assigned file.
	if got := zeConfigFileName(rec, "config"); got != "ze-bgp.conf" {
		t.Errorf("reused first block = %q, want ze-bgp.conf", got)
	}
	if got := zeConfigFileName(rec, "responder"); got != "ze-responder.conf" {
		t.Errorf("reused second block = %q, want ze-responder.conf", got)
	}
}

func TestZeConfigFileNameFirstBlockCanonical(t *testing.T) {
	// Whatever the first block is named, it maps to ze-bgp.conf so single-daemon
	// and action=rewrite:dest=ze-bgp.conf tests read the file they target.
	for _, block := range []string{"ze-bgp", "ze-conf", "config", "ze-fw"} {
		rec := &Record{}
		if got := zeConfigFileName(rec, block); got != "ze-bgp.conf" {
			t.Errorf("first block %q = %q, want ze-bgp.conf", block, got)
		}
	}
}

func TestSanitizeConfigBlock(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"responder", "responder"},
		{"ze-conf", "ze-conf"},
		{"initiator_2", "initiator_2"},
		{"a/b", "a-b"},
		{"a b.c", "a-b-c"},
		{"", "daemon"},
	} {
		if got := sanitizeConfigBlock(tc.in); got != tc.want {
			t.Errorf("sanitizeConfigBlock(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
