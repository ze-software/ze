package peer

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

// VALIDATES: matchRule supports exact, prefix, and contains modes.
// PREVENTS: prefix/contains matching broken or case-sensitive.
func TestMatchRule(t *testing.T) {
	tests := []struct {
		name     string
		check    string
		received string
		want     bool
	}{
		{"exact_match", "AABBCC", "AABBCC", true},
		{"exact_case_insensitive", "aabbcc", "AABBCC", true},
		{"exact_mismatch", "AABBCC", "AABBDD", false},
		{"prefix_match", "prefix:AABB", "AABBCC", true},
		{"prefix_case_insensitive", "prefix:aabb", "AABBCC", true},
		{"prefix_mismatch", "prefix:CCDD", "AABBCC", false},
		{"prefix_full", "prefix:AABBCC", "AABBCC", true},
		{"contains_match", "contains:BBCC", "AABBCCDD", true},
		{"contains_case_insensitive", "contains:bbcc", "AABBCCDD", true},
		{"contains_mismatch", "contains:EEFF", "AABBCCDD", false},
		{"contains_at_start", "contains:AABB", "AABBCCDD", true},
		{"contains_at_end", "contains:CCDD", "AABBCCDD", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchRule(tt.check, tt.received)
			if got != tt.want {
				t.Errorf("matchRule(%q, %q) = %v, want %v", tt.check, tt.received, got, tt.want)
			}
		})
	}
}

// VALIDATES: parseExpectRule handles prefix= and contains= in expect=bgp lines.
// PREVENTS: New syntax rejected by parser.
func TestParseExpectRule_PrefixContains(t *testing.T) {
	tests := []struct {
		name        string
		rule        string
		wantConn    int
		wantSeq     int
		wantContent string
		wantErr     bool
	}{
		{
			name:        "hex",
			rule:        "expect=bgp:conn=1:seq=1:hex=AABBCC",
			wantConn:    1,
			wantSeq:     1,
			wantContent: "AABBCC",
		},
		{
			name:        "prefix",
			rule:        "expect=bgp:conn=2:seq=1:prefix=AABB",
			wantConn:    2,
			wantSeq:     1,
			wantContent: "prefix:AABB",
		},
		{
			name:        "contains",
			rule:        "expect=bgp:conn=1:seq=2:contains=CCDD",
			wantConn:    1,
			wantSeq:     2,
			wantContent: "contains:CCDD",
		},
		{
			name:    "missing_all",
			rule:    "expect=bgp:conn=1:seq=1",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn, seq, content, err := parseExpectRule(tt.rule)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if conn != tt.wantConn {
				t.Errorf("conn = %d, want %d", conn, tt.wantConn)
			}
			if seq != tt.wantSeq {
				t.Errorf("seq = %d, want %d", seq, tt.wantSeq)
			}
			if content != tt.wantContent {
				t.Errorf("content = %q, want %q", content, tt.wantContent)
			}
		})
	}
}

// mkTestUpdate builds an UPDATE Message whose body is the given hex.
// Bodies must not be 4 or 11 bytes long (IsEOR would classify them as EOR).
func mkTestUpdate(t *testing.T, bodyHex string) *Message {
	t.Helper()
	body, err := hex.DecodeString(bodyHex)
	require.NoError(t, err)
	total := HeaderLen + len(body)
	header := make([]byte, 0, HeaderLen)
	header = append(header, Marker...)
	header = append(header, byte(total>>8), byte(total&0xFF), MsgUPDATE)
	return &Message{Header: header, Body: body}
}

// VALIDATES: parseExpectRule accepts ordered= and normalizes like contains=.
// PREVENTS: ordered expectations rejected by the peer parser.
func TestParseExpectRule_Ordered(t *testing.T) {
	conn, seq, content, err := parseExpectRule("expect=bgp:conn=1:seq=2:ordered=180a0000")
	require.NoError(t, err)
	require.Equal(t, 1, conn)
	require.Equal(t, 2, seq)
	require.Equal(t, "ordered:180A0000", content)
}

// VALIDATES: ordered expectations tolerate NLRI packing — one received
// message consumes several consecutive ordered needles, in order.
// PREVENTS: legal BGP UPDATE packing (fwdBucketMerge) failing per-message
// framing assertions (functional test 224 fast-fail shape).
func TestOrderedExpectationsPackingTolerant(t *testing.T) {
	c, err := NewChecker([]string{
		"expect=bgp:conn=1:seq=1:ordered=180A0000",
		"expect=bgp:conn=1:seq=1:ordered=180A0001",
		"expect=bgp:conn=1:seq=1:ordered=180A0002",
		"expect=bgp:conn=1:seq=1:ordered=180A0003",
	})
	require.NoError(t, err)
	c.Init()

	// One needle alone, then two packed, then the last.
	require.True(t, c.Expected(mkTestUpdate(t, "0000000000"+"180A0000")))
	require.True(t, c.Expected(mkTestUpdate(t, "0000000000"+"180A0001"+"180A0002")))
	require.True(t, c.Expected(mkTestUpdate(t, "0000000000"+"180A0003")))
	require.True(t, c.Completed())
}

// VALIDATES: an ordered needle arriving before its predecessors does not
// match — out-of-order delivery is a mismatch.
// PREVENTS: the checker silently accepting the FIFO violation the forward
// pool fix exists to prevent (test 224 slow-fail shape: 0006 before 0004).
func TestOrderedExpectationsRejectOutOfOrder(t *testing.T) {
	c, err := NewChecker([]string{
		"expect=bgp:conn=1:seq=1:ordered=180A0000",
		"expect=bgp:conn=1:seq=1:ordered=180A0001",
		"expect=bgp:conn=1:seq=1:ordered=180A0002",
	})
	require.NoError(t, err)
	c.Init()

	require.True(t, c.Expected(mkTestUpdate(t, "0000000000"+"180A0000")))
	// 0002 arrives while the front needle is 0001: no consume, mismatch.
	require.False(t, c.Expected(mkTestUpdate(t, "0000000000"+"180A0002")))
	require.False(t, c.Completed())
}

// VALIDATES: intra-message needle order is enforced via the advancing
// cursor — a packed message carrying needles in the wrong internal order
// satisfies only the needles found at or after the cursor.
// PREVENTS: substring matching accepting reordered NLRIs inside one packed
// UPDATE.
func TestOrderedExpectationsIntraMessageOrder(t *testing.T) {
	c, err := NewChecker([]string{
		"expect=bgp:conn=1:seq=1:ordered=180A0000",
		"expect=bgp:conn=1:seq=1:ordered=180A0001",
	})
	require.NoError(t, err)
	c.Init()

	// 0001 precedes 0000 inside the message: the cursor consumes 0000 (found
	// later in the stream) and then cannot find 0001 after it.
	require.True(t, c.Expected(mkTestUpdate(t, "0000000000"+"180A0001"+"180A0000")))
	require.False(t, c.Completed(), "0001 must remain pending after intra-message reorder")
}

// VALIDATES: ordered needles match only at even (byte-aligned) hex offsets —
// a needle straddling two wire bytes is not a match.
// PREVENTS: consumeOrdered consuming a needle on a half-byte artifact of the
// hex text that does not exist on the wire.
func TestOrderedExpectationsByteAligned(t *testing.T) {
	c, err := NewChecker([]string{
		"expect=bgp:conn=1:seq=1:ordered=80A00001",
	})
	require.NoError(t, err)
	c.Init()

	// The body hex contains "80A00001" only at an ODD offset (inside
	// "180A000018" starting at nibble 1): not a wire byte sequence.
	require.False(t, c.Expected(mkTestUpdate(t, "00"+"180A000018")))
	require.False(t, c.Completed())

	// The same needle at an even offset matches.
	require.True(t, c.Expected(mkTestUpdate(t, "0000"+"80A00001")))
	require.True(t, c.Completed())
}

// VALIDATES: plain checks (EOR hex) and ordered needles coexist in one seq
// group — the EOR matches whenever it arrives, data messages consume the
// ordered subqueue.
// PREVENTS: grouping the EOR with ordered needles breaking either match.
func TestOrderedExpectationsMixedWithPlainEOR(t *testing.T) {
	c, err := NewChecker([]string{
		"expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00170200000000",
		"expect=bgp:conn=1:seq=1:ordered=180A0000",
		"expect=bgp:conn=1:seq=1:ordered=180A0001",
	})
	require.NoError(t, err)
	c.Init()

	require.True(t, c.Expected(mkTestUpdate(t, "0000000000"+"180A0000")))
	// EOR (empty UPDATE) mid-stream matches the plain hex check.
	require.True(t, c.Expected(&Message{
		Header: append(append([]byte{}, Marker...), 0x00, 0x17, MsgUPDATE),
		Body:   []byte{0x00, 0x00, 0x00, 0x00},
	}))
	require.True(t, c.Expected(mkTestUpdate(t, "0000000000"+"180A0001")))
	require.True(t, c.Completed())
}
