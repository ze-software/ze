package peer

import (
	"encoding/binary"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
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
// (spec-fixit-redistribute-establishment-stall, D1).
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
	require.Equal(t, []OpenASBinding{{AS: 65533}}, config.OpenAS, "options still parse")

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
// VALIDATES: a malformed or IPv6 id fails the file where it is read, rather than
// leaving the default identifier in place.
// PREVENTS: an RFC 6286 .ci test silently sending the DEFAULT identifier and passing
// vacuously because the option never reached the peer.
//
// The two refusal cases asserted the opposite until 2026-09-08: a malformed id was
// dropped in silence and the peer sent the derived default, so a file that asked
// for an invalid identifier tested the valid one. That is the failure the option
// exists to prevent, so the assertion was wrong rather than the guard.
func TestLoadExpectFileRouterIDOverride(t *testing.T) {
	tests := []struct {
		name    string
		option  string
		want    *uint32
		refused string
	}{
		{name: "zero identifier", option: "option=open:value=router-id:id=0.0.0.0", want: ptrUint32(0)},
		{name: "dotted quad", option: "option=open:value=router-id:id=1.2.3.4", want: ptrUint32(0x01020304)},
		{name: "max identifier", option: "option=open:value=router-id:id=255.255.255.255", want: ptrUint32(0xFFFFFFFF)},
		{name: "no override", option: "option=asn:value=65533"},
		{name: "malformed id refused", option: "option=open:value=router-id:id=not-an-ip", refused: "not-an-ip"},
		{name: "ipv6 id refused", option: "option=open:value=router-id:id=2001:db8::1", refused: "2001:db8::1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := tt.option + "\nexpect=bgp:conn=1:seq=1:hex=FFFF001304\n"
			path := filepath.Join(t.TempDir(), "expect.msg")
			require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

			_, config, err := LoadExpectFile(path)
			if tt.refused != "" {
				require.Error(t, err, "an identifier ze-peer cannot honor must fail the file")
				assert.Contains(t, err.Error(), tt.refused, "the error names the value")
				return
			}
			require.NoError(t, err)

			if tt.want == nil {
				assert.Nil(t, config.RouterID, "no override must leave the default in place")
				return
			}
			require.NotNil(t, config.RouterID)
			assert.Equal(t, *tt.want, *config.RouterID)
		})
	}
}

func ptrUint32(v uint32) *uint32 { return new(v) }

// TestLoadExpectFileSendBulkBoundaries pins the max-msg range.
//
// VALIDATES: option=update:value=send-bulk accepts max-msg over the whole legal
// BGP message range and refuses one octet outside it at either end.
// PREVENTS: a max-msg typo being clamped or ignored, which would send a
// STANDARD-sized message where the test needs an RFC 8654 oversize one -- the
// daemon then takes a different branch and the test passes against the wrong
// input.
func TestLoadExpectFileSendBulkBoundaries(t *testing.T) {
	tests := []struct {
		name    string
		maxMsg  string
		want    int
		wantErr bool
	}{
		{"omitted defaults to RFC 4271", "", 0, false},
		{"first valid: bare EOR", "23", 23, false},
		{"invalid below", "22", 0, true},
		{"standard ceiling", "4096", 4096, false},
		{"last valid: RFC 8654 ceiling", "65535", 65535, false},
		{"invalid above", "65536", 0, true},
		{"not a number", "big", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opt := "option=update:value=send-bulk:prefix=10.0.0.0/24:count=10:next-hop=10.0.0.1:origin-as=65001"
			if tt.maxMsg != "" {
				opt += ":max-msg=" + tt.maxMsg
			}
			path := filepath.Join(t.TempDir(), "expect.msg")
			require.NoError(t, os.WriteFile(path, []byte(opt+"\n"), 0o600))

			_, config, err := LoadExpectFile(path)
			if tt.wantErr {
				require.Error(t, err, "a max-msg outside the legal range must fail the load")
				return
			}
			require.NoError(t, err)
			require.Len(t, config.SendBulk, 1)
			assert.Equal(t, tt.want, config.SendBulk[0].MaxMsgLen)
		})
	}
}

// TestLoadExpectFileSendBulkRejectsBadSpec pins the fail-closed contract.
//
// VALIDATES: every malformed send-bulk key fails the load rather than yielding a
// zero-valued spec.
// PREVENTS: the worst shape for this directive -- a spec that degrades to
// count=0 sends NOTHING, so a test asserting a route was not forwarded passes
// because no route was ever offered (ai/rules/evidence.md).
func TestLoadExpectFileSendBulkRejectsBadSpec(t *testing.T) {
	base := map[string]string{
		"prefix":    "10.0.0.0/24",
		"count":     "10",
		"next-hop":  "10.0.0.1",
		"origin-as": "65001",
	}
	tests := []struct {
		name string
		key  string
		val  string
	}{
		{"missing prefix", "prefix", ""},
		{"prefix is a bare address", "prefix", "10.0.0.0"},
		{"count zero", "count", "0"},
		{"count negative", "count", "-1"},
		{"count not a number", "count", "many"},
		{"missing next-hop", "next-hop", ""},
		{"next-hop not an address", "next-hop", "nowhere"},
		{"missing origin-as", "origin-as", ""},
		{"origin-as past 32 bits", "origin-as", "4294967296"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sb strings.Builder
			sb.WriteString("option=update:value=send-bulk")
			for k, v := range base {
				if k == tt.key {
					v = tt.val
				}
				sb.WriteString(":" + k + "=" + v)
			}
			opt := sb.String()
			path := filepath.Join(t.TempDir(), "expect.msg")
			require.NoError(t, os.WriteFile(path, []byte(opt+"\n"), 0o600))

			_, config, err := LoadExpectFile(path)
			require.Error(t, err, "a malformed send-bulk spec must fail the load, not send nothing")
			assert.Nil(t, config, "a failed load must not hand back a half-built config")
		})
	}
}

// TestSendBulkBuildsOneOversizeMessage pins the reason this directive exists.
//
// VALIDATES: a spec whose max-msg is raised to the RFC 8654 ceiling produces ONE
// message larger than RFC 4271 allows, rather than several standard ones.
// PREVENTS: the generator silently splitting the prefixes, which would hand the
// daemon many small UPDATEs -- a different input, decided per message, and one
// that cannot reach a path whose trigger is a single oversize body.
func TestSendBulkBuildsOneOversizeMessage(t *testing.T) {
	spec := InjectSpec{
		Prefix:    netip.MustParsePrefix("10.0.0.0/24"),
		Count:     16373,
		NextHop:   netip.MustParseAddr("10.0.0.1"),
		ASN:       65001,
		MaxMsgLen: bgpExtMsgLen,
	}
	data, msgs, err := buildUpdates(spec)
	require.NoError(t, err)
	assert.Equal(t, 1, msgs, "16373 /24 prefixes must fit one extended message")
	require.Len(t, data, bgpExtMsgLen)

	// The length field is the daemon's own framing input, so assert on it
	// rather than on len(data) alone.
	assert.Equal(t, uint16(bgpExtMsgLen), binary.BigEndian.Uint16(data[16:18]))
	assert.Equal(t, byte(MsgUPDATE), data[18])

	// The body is exactly maxUpdateBody (reactor/forward_build.go), which is what
	// makes any egress addition overflow. If this number moves, the .ci that
	// depends on it (test/plugin/modify-oversize-suppress.ci) is testing
	// something else.
	assert.Equal(t, 65516, len(data)-HeaderLen)

	// The same spec at the default ceiling must NOT be one message, or the
	// max-msg knob would be doing nothing.
	spec.MaxMsgLen = 0
	_, stdMsgs, err := buildUpdates(spec)
	require.NoError(t, err)
	assert.Greater(t, stdMsgs, 1, "at the RFC 4271 ceiling the same prefixes must split")
}

// TestPeerOptionASNRefusesUnrepresentable is AC-4 and the option=asn boundary row.
//
// VALIDATES: option=asn accepts the whole AS space, 1 to 4294967295, and fails
// the file for anything outside it with an error naming the option and the
// value. A four-octet value is representable rather than refusable, because
// RFC 6793 Section 3 puts AS_TRANS in the two-octet field and the real AS in the
// capability.
// PREVENTS: the range guard that used to skip both writes above 65535, which let
// a .ci ask for AS 4200000000 and get a peer opening with ze's own AS. That is a
// value silently wrong rather than an error (ai/rules/principles.md).
func TestPeerOptionASNRefusesUnrepresentable(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  uint32
		// names is the substring the refusal must carry. An empty string here
		// means the case is accepted, never that any error will do: a substring
		// check against "" passes over every message, which cannot fail.
		names string
	}{
		{name: "one", value: "1", want: 1},
		{name: "two octet boundary", value: "65535", want: 65535},
		{name: "just above two octets", value: "65536", want: 65536},
		{name: "largest AS", value: "4294967295", want: 4294967295},
		{name: "zero", value: "0", names: "value=0 is outside"},
		{name: "negative", value: "-1", names: `value="-1" is not a number`},
		{name: "above the largest AS", value: "4294967296", names: "value=4294967296 is outside"},
		{name: "not a number", value: "abc", names: `value="abc" is not a number`},
		{name: "empty", value: "", names: `value="" is not a number`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := "option=asn:value=" + tt.value + "\nexpect=bgp:conn=1:seq=1:hex=FFFF001304\n"
			path := filepath.Join(t.TempDir(), "expect.msg")
			require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

			_, config, err := LoadExpectFile(path)
			if tt.names != "" {
				require.Error(t, err, "an AS ze-peer cannot honor must fail the file")
				assert.Contains(t, err.Error(), "option=asn", "the error names the option")
				assert.Contains(t, err.Error(), tt.names, "the error names the value it refused")
				return
			}
			require.NoError(t, err)
			require.Equal(t, []OpenASBinding{{AS: tt.want}}, config.OpenAS)
		})
	}
}

// TestPeerOptionASNBindsAnAddress covers the peer= key.
//
// VALIDATES: option=asn:peer=<ip> parses into a keyed declaration, and a
// malformed address fails the file.
// PREVENTS: the key being read as a value ze-peer has no branch for, which would
// bind the AS to nothing and answer every connection with it.
func TestPeerOptionASNBindsAnAddress(t *testing.T) {
	path := filepath.Join(t.TempDir(), "expect.msg")
	require.NoError(t, os.WriteFile(path,
		[]byte("option=asn:peer=127.0.0.2:value=65002\nexpect=bgp:conn=1:seq=1:hex=FFFF001304\n"), 0o600))

	_, config, err := LoadExpectFile(path)
	require.NoError(t, err)
	require.Len(t, config.OpenAS, 1)
	assert.Equal(t, "127.0.0.2", config.OpenAS[0].Addr.String())
	assert.Equal(t, uint32(65002), config.OpenAS[0].AS)

	require.NoError(t, os.WriteFile(path,
		[]byte("option=asn:peer=not-an-ip:value=65002\nexpect=bgp:conn=1:seq=1:hex=FFFF001304\n"), 0o600))
	_, _, err = LoadExpectFile(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not-an-ip")
}

// TestPeerOptionAddCapabilityRefusesWhatItCannotSend covers the add-capability
// boundary rows.
//
// VALIDATES: a bad code, bad hex, and a value above the 255 octets RFC 5492
// Section 4 can state each fail the file.
// PREVENTS: three silent drops. An out-of-range code produced no override at
// all, hex.DecodeString's error was discarded so bad hex became an empty
// capability value, and a 256-octet value truncated to its low length octet.
func TestPeerOptionAddCapabilityRefusesWhatItCannotSend(t *testing.T) {
	tests := []struct {
		name   string
		option string
		names  string
	}{
		{"code zero", "option=open:value=add-capability:code=0:hex=00", "0"},
		{"code above one octet", "option=open:value=drop-capability:code=256", "256"},
		{"code not a number", "option=open:value=drop-capability:code=nine", "nine"},
		{"value not hex", "option=open:value=add-capability:code=65:hex=zzzz", "zzzz"},
		{"value above 255 octets", "option=open:value=add-capability:code=65:hex=" + strings.Repeat("00", 256), "256"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "expect.msg")
			content := tt.option + "\nexpect=bgp:conn=1:seq=1:hex=FFFF001304\n"
			require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

			_, _, err := LoadExpectFile(path)
			require.Error(t, err, "a capability ze-peer cannot send must fail the file")
			assert.Contains(t, err.Error(), tt.names, "the error names the value")
		})
	}
}

// TestPeerOptionASNRefusesTwoUnkeyedDeclarations pins what a block may state.
//
// VALIDATES: two option=asn lines with no peer= key on either fail the file,
// naming both ASNs; two KEYED lines are the way to state one AS per session.
// PREVENTS: the order of two contradictory lines deciding which AS reaches the
// wire, which is a silent answer to a question the block asked twice.
func TestPeerOptionASNRefusesTwoUnkeyedDeclarations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "expect.msg")
	require.NoError(t, os.WriteFile(path,
		[]byte("option=asn:value=65001\noption=asn:value=65002\nexpect=bgp:conn=1:seq=1:hex=FFFF001304\n"), 0o600))

	_, _, err := LoadExpectFile(path)
	require.Error(t, err, "two unkeyed declarations must fail the file")
	assert.Contains(t, err.Error(), "65001")
	assert.Contains(t, err.Error(), "65002")

	require.NoError(t, os.WriteFile(path,
		[]byte("option=asn:peer=127.0.0.1:value=65001\noption=asn:peer=127.0.0.2:value=65002\n"+
			"expect=bgp:conn=1:seq=1:hex=FFFF001304\n"), 0o600))
	_, config, err := LoadExpectFile(path)
	require.NoError(t, err, "one AS per session is stated by keying each line")
	assert.Len(t, config.OpenAS, 2)
}

// TestPeerOptionRefusesContradictoryOpenDeclarations covers the two pairs whose
// halves are legal apart and contradictory together.
//
// VALIDATES: a four-octet `option=asn` beside `drop-capability:code=65` fails the
// file, because RFC 6793 Section 3 makes that capability the only carrier of an
// AS above 65535; and two `add-capability:code=65` lines fail it, because the
// OPEN would carry two four-octet AS capabilities.
// PREVENTS: two silent answers. The first sent AS_TRANS with nothing carrying the
// AS the .ci asked for. The second let the FIRST stated capability decide the AS
// by accident of order, in the declaration that outranks `option=asn`.
func TestPeerOptionRefusesContradictoryOpenDeclarations(t *testing.T) {
	tests := []struct {
		name    string
		lines   []string
		names   string
		refused bool
	}{
		{
			name:    "four-octet AS with capability 65 dropped",
			lines:   []string{"option=asn:value=4200000000", "option=open:value=drop-capability:code=65"},
			names:   "4200000000",
			refused: true,
		},
		{
			name:    "two stated capability 65 lines",
			lines:   []string{"option=open:value=add-capability:code=65:hex=0000FDE9", "option=open:value=add-capability:code=65:hex=0000FDEA"},
			names:   "add-capability:code=65",
			refused: true,
		},
		{
			name:  "a two-octet AS with capability 65 dropped",
			lines: []string{"option=asn:value=65001", "option=open:value=drop-capability:code=65"},
		},
		{
			name:    "a stated capability 65 that contradicts option=asn",
			lines:   []string{"option=asn:value=65010", "option=open:value=add-capability:code=65:hex=0000FDE9"},
			names:   "declares the AS twice",
			refused: true,
		},
		{
			name:  "a stated capability 65 that agrees with option=asn",
			lines: []string{"option=asn:value=65001", "option=open:value=add-capability:code=65:hex=0000FDE9"},
		},
		{
			// The AS_TRANS refusal was disarmed by a capability-65 line that
			// carries no AS: the count said "an AS was stated" while
			// statedOpenAS said it was not, so the file loaded clean and the
			// OPEN went out claiming AS_TRANS with nothing carrying the real AS.
			name:    "a four-octet AS, capability 65 dropped, and a stated 65 that carries no AS",
			lines:   []string{"option=asn:value=4200000000", "option=open:value=drop-capability:code=65", "option=open:value=add-capability:code=65:hex=1122"},
			names:   "4200000000",
			refused: true,
		},
		{
			// The same shape with a real AS in the stated capability IS the
			// carrier, so it stays legal.
			name:  "a four-octet AS, capability 65 dropped, and a stated 65 that carries it",
			lines: []string{"option=asn:value=4200000000", "option=open:value=drop-capability:code=65", "option=open:value=add-capability:code=65:hex=FA56EA00"},
		},
		{
			// No drop-capability at all. A STATED capability 65 removes ze-peer's
			// own four-octet carrier just as a drop does, because reconcileParams
			// skips the builder's TLV for any code the .ci stated. The guard read
			// only the drop, so this loaded clean and the OPEN went out claiming
			// AS_TRANS with a two-octet capability 65 carrying 0x1122 and nothing
			// carrying 4200000000.
			name:    "a four-octet AS and a stated 65 that carries no AS, with no drop",
			lines:   []string{"option=asn:value=4200000000", "option=open:value=add-capability:code=65:hex=1122"},
			names:   "4200000000",
			refused: true,
		},
		{
			// The two-octet AS beside the same stated capability stays legal: the
			// My Autonomous System field carries it on its own, so nothing is lost
			// when the four-octet capability does not go out.
			name:  "a two-octet AS and a stated 65 that carries no AS",
			lines: []string{"option=asn:value=65001", "option=open:value=add-capability:code=65:hex=1122"},
		},
		{
			name:  "capability 65 dropped and stated back",
			lines: []string{"option=asn:value=4200000000", "option=open:value=drop-capability:code=65", "option=open:value=add-capability:code=65:hex=FA56EA00"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "expect.msg")
			content := strings.Join(tt.lines, "\n") + "\nexpect=bgp:conn=1:seq=1:hex=FFFF001304\n"
			require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

			_, config, err := LoadExpectFile(path)
			if tt.refused {
				require.Error(t, err, "a combination ze-peer cannot honor must fail the file")
				assert.Contains(t, err.Error(), tt.names, "the error names what it refused")
				return
			}
			require.NoError(t, err)

			// The same combination reaching New from a flag and a file must be
			// refused at the second boundary too.
			config.Expect = []string{"expect=bgp:conn=1:seq=1:hex=FFFF001304"}
			_, err = New(config)
			require.NoError(t, err)
		})
	}

	// The pair split across the two sources: the AS from --asn, the drop from the
	// file. Only New sees both.
	path := filepath.Join(t.TempDir(), "expect.msg")
	require.NoError(t, os.WriteFile(path,
		[]byte("option=open:value=drop-capability:code=65\nexpect=bgp:conn=1:seq=1:hex=FFFF001304\n"), 0o600))
	expect, config, err := LoadExpectFile(path)
	require.NoError(t, err, "the file alone states no contradiction")

	config.Expect = expect
	config.OpenAS = append(config.OpenAS, OpenASBinding{AS: 4200000000})
	_, err = New(config)
	require.Error(t, err, "New sees both halves and must refuse")
	assert.Contains(t, err.Error(), "4200000000")
}
