package peer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConsumesLine pins the ze-peer-consumed directive set.
//
// This predicate decides whether a .ci line reaches ze-peer. The test runner
// reuses it to reject, at parse time, a check-mode peer block that would leave
// ze-peer with nothing to check (and therefore never listening). If the two ever
// disagree, the silent-vacuous-test defect returns, so the set is pinned here
// rather than left implicit in LoadExpectFile's switch.
//
// VALIDATES: the exact set forwarded to ze-peer, including the exclusions.
// PREVENTS: expect=json being assumed to reach ze-peer
// (plan/spec-fixit-redistribute-establishment-stall.md D1).
func TestConsumesLine(t *testing.T) {
	consumed := []string{
		"expect=bgp:conn=1:seq=1:hex=FFFF001304",
		"action=notification:conn=1:seq=1:text=bye",
		"action=send:conn=1:seq=1:hex=FFFF001304",
		"action=rewrite:conn=1:seq=1:source=new.conf:dest=ze-bgp.conf",
		"action=close:conn=1:seq=1",
		"action=sighup:conn=1:seq=1",
		"action=sigterm:conn=1:seq=1",
		"  expect=bgp:conn=1:seq=1:hex=FFFF  ", // leading/trailing space tolerated
	}
	for _, line := range consumed {
		assert.True(t, ConsumesLine(line), "ze-peer consumes %q", line)
	}

	// These are the runner's business. A peer block containing only these makes
	// ze-peer exit before binding, which is the whole reason this predicate is
	// exported.
	notConsumed := []string{
		`expect=json:conn=1:seq=1:json={ "type": "update" }`,
		"expect=exit:code=0",
		"expect=stdout:contains=hello",
		"expect=stderr:pattern=oops",
		"expect=syslog:pattern=oops",
		"expect=file:path=x:exists=true",
		"reject=stderr:pattern=oops",
		"option=timeout:value=5s",
		"option=asn:value=65533",
		"cmd=api:conn=1:seq=1:text=announce eor ipv4/unicast",
		"# a comment",
		"",
		"   ",
		"garbage-with-no-equals",
	}
	for _, line := range notConsumed {
		assert.False(t, ConsumesLine(line), "ze-peer does NOT consume %q", line)
	}
}

// TestLoadExpectFileMatchesConsumesLine verifies the predicate agrees with what
// LoadExpectFile actually collects, so the runner's parse-time guard predicts
// ze-peer's real behavior rather than a copy of it that can drift.
//
// VALIDATES: LoadExpectFile forwards exactly the ConsumesLine set.
func TestLoadExpectFileMatchesConsumesLine(t *testing.T) {
	content := `option=asn:value=65533
expect=bgp:conn=1:seq=1:hex=FFFF001304
expect=json:conn=1:seq=1:json={ "type": "update" }
action=send:conn=1:seq=2:hex=FFFF001305
expect=exit:code=0
cmd=api:conn=1:seq=1:text=announce eor ipv4/unicast
`
	path := filepath.Join(t.TempDir(), "expect.msg")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	expect, config, err := LoadExpectFile(path)
	require.NoError(t, err)
	require.Equal(t, 65533, config.ASN, "options still parse")

	assert.Equal(t, []string{
		"expect=bgp:conn=1:seq=1:hex=FFFF001304",
		"action=send:conn=1:seq=2:hex=FFFF001305",
	}, expect, "only ze-peer-consumed directives are forwarded")

	// The agreement that matters: every forwarded line satisfies ConsumesLine.
	for _, line := range expect {
		assert.True(t, ConsumesLine(line), "forwarded line must satisfy ConsumesLine: %q", line)
	}
}

// TestLoadExpectFileJSONOnlyYieldsNoExpectations is the direct proof of D1: the
// peer block shape used by test/plugin/bgp-redistribute-announce.ci leaves
// ze-peer with zero expectations, which is what makes it exit before binding.
//
// VALIDATES: an expect=json-only block produces an empty Expect.
// PREVENTS: Re-litigating whether expect=json makes the peer listen. It does not.
func TestLoadExpectFileJSONOnlyYieldsNoExpectations(t *testing.T) {
	content := `option=timeout:value=5s
expect=json:conn=1:seq=1:json={ "type": "update" }
`
	path := filepath.Join(t.TempDir(), "jsononly.msg")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	expect, _, err := LoadExpectFile(path)
	require.NoError(t, err)
	assert.Empty(t, expect, "expect=json is not forwarded to ze-peer, so the peer has nothing to check")
}

// TestLoadExpectFileRouterIDOverride verifies the option that drives an RFC-invalid BGP
// Identifier from a .ci test (option=open:value=router-id:id=<a.b.c.d>).
//
// VALIDATES: a dotted-quad id parses into Config.RouterID as a big-endian uint32.
// VALIDATES: no override leaves RouterID nil, so the mirror-and-increment default stands.
// VALIDATES: a malformed or IPv6 id is ignored rather than producing a bogus identifier.
// PREVENTS: an RFC 6286 .ci test silently sending the DEFAULT identifier and passing
// vacuously because the option never reached the peer.
func TestLoadExpectFileRouterIDOverride(t *testing.T) {
	tests := []struct {
		name   string
		option string
		want   *uint32
	}{
		{"zero identifier", "option=open:value=router-id:id=0.0.0.0", ptrUint32(0)},
		{"dotted quad", "option=open:value=router-id:id=1.2.3.4", ptrUint32(0x01020304)},
		{"max identifier", "option=open:value=router-id:id=255.255.255.255", ptrUint32(0xFFFFFFFF)},
		{"no override", "option=asn:value=65533", nil},
		{"malformed id ignored", "option=open:value=router-id:id=not-an-ip", nil},
		{"ipv6 id ignored", "option=open:value=router-id:id=2001:db8::1", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := tt.option + "\nexpect=bgp:conn=1:seq=1:hex=FFFF001304\n"
			path := filepath.Join(t.TempDir(), "expect.msg")
			require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

			_, config, err := LoadExpectFile(path)
			require.NoError(t, err)

			if tt.want == nil {
				assert.Nil(t, config.RouterID, "no valid override must leave the default in place")
				return
			}
			require.NotNil(t, config.RouterID)
			assert.Equal(t, *tt.want, *config.RouterID)
		})
	}
}

func ptrUint32(v uint32) *uint32 { return new(v) }
