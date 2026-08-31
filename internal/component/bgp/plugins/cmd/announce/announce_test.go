// VALIDATES: Handler arg parsing, prefix validation, family detection, tag length, duration parsing, withdraw dispatch.
// PREVENTS: Invalid prefixes reaching reactor, tag keys exceeding bounds, unknown tokens accepted silently.

package announce

import (
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"

	// Blank import registers the ipv4/flow family + in-process NLRI encoder so the
	// registry-seam path in handleAnnounceFlowspec (encodeFlowspecNLRI) resolves.
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/flowspec"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
)

// captureReactor is a minimal BGPReactor fake: it embeds the interface (so the
// unused methods exist) and records the batch that AnnounceNLRIBatch dispatches.
type captureReactor struct {
	bgptypes.BGPReactor
	sel    *selector.Selector
	batch  bgptypes.NLRIBatch
	sender plugin.Sender
	calls  int
}

func (r *captureReactor) AnnounceNLRIBatch(sel *selector.Selector, batch bgptypes.NLRIBatch, sender plugin.Sender) error {
	r.sel = sel
	r.batch = batch
	r.sender = sender
	r.calls++
	return nil
}

func mustParsePrefix(t *testing.T, s string) netip.Prefix {
	t.Helper()
	p, err := netip.ParsePrefix(s)
	require.NoError(t, err)
	return p
}

func respData(t *testing.T, resp *plugin.Response) plugin.Map {
	t.Helper()
	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok, "resp.Data is not plugin.Map")
	return data
}

func TestPrefixToFamily(t *testing.T) {
	tests := []struct {
		prefix string
		want   family.Family
	}{
		{"192.0.2.0/24", family.IPv4Unicast},
		{"10.0.0.0/8", family.IPv4Unicast},
		{"0.0.0.0/0", family.IPv4Unicast},
		{"192.0.2.1/32", family.IPv4Unicast},
		{"2001:db8::/32", family.IPv6Unicast},
		{"::1/128", family.IPv6Unicast},
		{"::/0", family.IPv6Unicast},
	}
	for _, tt := range tests {
		t.Run(tt.prefix, func(t *testing.T) {
			p := mustParsePrefix(t, tt.prefix)
			got := prefixToFamily(p)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input   string
		want    time.Duration
		wantErr bool
	}{
		{"300s", 300 * time.Second, false},
		{"5m", 5 * time.Minute, false},
		{"1h", time.Hour, false},
		{"300", 300 * time.Second, false},
		{"0", 0, false},
		{"1", time.Second, false},
		{"abc", 0, true},
		{"", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseDuration(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

// VALIDATES: AC-12 -- a bare seconds count that cannot be represented as a
// time.Duration is rejected, never wrapped into a negative duration.
// PREVENTS: `time.Duration(secs) * time.Second` overflowing int64, which turns a
// huge withdraw delay into an immediate (or negative) one.
func TestParseDurationRejectsOverflow(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		{"last valid seconds", "9223372036", 9223372036 * time.Second, false},
		{"first invalid above", "9223372037", 0, true},
		{"max uint64", "18446744073709551615", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDuration(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				assert.Zero(t, got, "a rejected duration must not leak a value")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			assert.Positive(t, got, "a valid duration must never come back negative")
		})
	}
}

func TestSplitFlowspecArgs(t *testing.T) {
	// VALIDATES: component/action/opts split; boundary -- rate-limit requires a
	// value; an action is mandatory (no fabricated default).
	tests := []struct {
		name       string
		args       []string
		wantComp   []string
		wantAction []string
		wantOpts   []string
		wantErr    bool
	}{
		{
			"rate-limit with tag",
			[]string{"destination-ipv4", "10.0.0.0/24", "protocol", "=6", "rate-limit", "100000", "tag", "ddos", "udp"},
			[]string{"destination-ipv4", "10.0.0.0/24", "protocol", "=6"},
			[]string{"traffic-rate", "0", "100000", "bytes"},
			[]string{"tag", "ddos", "udp"},
			false,
		},
		{
			"discard",
			[]string{"destination-ipv4", "10.0.0.0/24", "discard"},
			[]string{"destination-ipv4", "10.0.0.0/24"},
			[]string{"discard"},
			[]string{},
			false,
		},
		{
			// RFC 8955 Section 7 makes the action an extended community, and
			// rate-limit and discard are spellings of one. The community form
			// is the general case, so an action route.ParseExtendedCommunities
			// defines and the two keywords do not, such as redirect, is now
			// reachable rather than refused before it is ever parsed.
			"community carries the action verbatim",
			[]string{"destination-ipv4", "10.0.0.0/24", "community", "traffic-rate", "65001", "9600", "bytes", "for", "300s"},
			[]string{"destination-ipv4", "10.0.0.0/24"},
			[]string{"traffic-rate", "65001", "9600", "bytes"},
			[]string{"for", "300s"},
			false,
		},
		{
			// An action's token count varies, so only the option keywords say
			// where it ends. With none present it runs to the end.
			"community with no trailing options",
			[]string{"destination-ipv4", "10.0.0.0/24", "community", "redirect", "65001", "100"},
			[]string{"destination-ipv4", "10.0.0.0/24"},
			[]string{"redirect", "65001", "100"},
			[]string{},
			false,
		},
		{"rate-limit missing value", []string{"destination-ipv4", "10.0.0.0/24", "rate-limit"}, nil, nil, nil, true},
		{"community missing action", []string{"destination-ipv4", "10.0.0.0/24", "community"}, nil, nil, nil, true},
		{"community missing action before options", []string{"destination-ipv4", "10.0.0.0/24", "community", "for", "300s"}, nil, nil, nil, true},
		{"no action", []string{"destination-ipv4", "10.0.0.0/24"}, nil, nil, nil, true},
		{"opts before action", []string{"destination-ipv4", "10.0.0.0/24", "tag", "x", "y"}, nil, nil, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			comp, action, opts, err := splitFlowspecArgs(tt.args)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantComp, comp)
			assert.Equal(t, tt.wantAction, action)
			assert.Equal(t, tt.wantOpts, opts)
		})
	}
}

func TestFlowspecFamilyName(t *testing.T) {
	tests := []struct {
		name       string
		components []string
		want       string
	}{
		{"v4 destination", []string{"destination-ipv4", "10.0.0.0/24", "protocol", "=6"}, "ipv4/flow"},
		{"v6 destination", []string{"destination-ipv6", "2001:db8::/32"}, "ipv6/flow"},
		{"v6 source", []string{"source-ipv6", "2001:db8::/48", "destination-ipv6", "2001:db8:1::/48"}, "ipv6/flow"},
		{"no prefix defaults v4", []string{"protocol", "=17", "destination-port", "=53"}, "ipv4/flow"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, flowspecFamilyName(tt.components))
		})
	}
}

func TestEncodeFlowspecNLRIBuildsWireRoute(t *testing.T) {
	// Integration: handleAnnounceFlowspec builds the FlowSpec NLRI via the family
	// registration seam (encodeFlowspecNLRI). Proves the v4 and v6 paths produce a
	// valid flow NLRI without a direct dependency on the flowspec plugin.
	cases := []struct {
		name       string
		components []string
		wantFamily string
	}{
		{"v4", []string{"destination-ipv4", "192.0.2.0/24", "protocol", "=6", "destination-port", "=80"}, "ipv4/flow"},
		{"v6", []string{"destination-ipv6", "2001:db8::/32", "protocol", "=17"}, "ipv6/flow"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fam, ok := family.LookupFamily(flowspecFamilyName(tc.components))
			require.True(t, ok, "%s family must be registered", tc.wantFamily)
			n, err := encodeFlowspecNLRI(fam, tc.components)
			require.NoError(t, err)
			require.NotNil(t, n)
			assert.Equal(t, tc.wantFamily, n.Family().String())
		})
	}
}

func TestHandleAnnounceFlowspec(t *testing.T) {
	// Drives the full CLI verb through a fake reactor and asserts the dispatched
	// NLRIBatch: correct family, a single flow NLRI, and next-hop self (required
	// for the FlowSpec MP_REACH_NLRI). Covers rate-limit, discard, and v6.
	reg := NewRegistry(func(*selector.Selector, bgptypes.NLRIBatch, plugin.Sender) error { return nil })
	ctx := &pluginserver.CommandContext{}

	cases := []struct {
		name       string
		args       []string
		wantFamily string
	}{
		{"rate-limit v4", []string{"destination-ipv4", "192.0.2.0/24", "protocol", "=6", "destination-port", "=80", "rate-limit", "9600"}, "ipv4/flow"},
		{"discard v4", []string{"destination-ipv4", "203.0.113.5/32", "discard"}, "ipv4/flow"},
		{"rate-limit v6", []string{"destination-ipv6", "2001:db8::/32", "rate-limit", "1000"}, "ipv6/flow"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rctr := &captureReactor{}
			resp, err := handleAnnounceFlowspec(ctx, rctr, reg, tc.args)
			require.NoError(t, err)
			require.NotNil(t, resp)
			require.Equal(t, 1, rctr.calls, "verb must dispatch exactly one batch")
			assert.Equal(t, tc.wantFamily, rctr.batch.Family.String())
			require.Len(t, rctr.batch.NLRIs, 1)
			assert.True(t, rctr.batch.NextHop.IsSelf(), "flowspec origination uses next-hop self")
		})
	}

	// An action is mandatory: components with no rate-limit/discard is an error.
	rctr := &captureReactor{}
	_, err := handleAnnounceFlowspec(ctx, rctr, reg, []string{"destination-ipv4", "192.0.2.0/24"})
	require.Error(t, err, "missing action must error")
	assert.Equal(t, 0, rctr.calls, "nothing dispatched on error")
}

// TestFlowspecActionAcceptsACommunity drives the community spelling of the
// action through the same handler and fake reactor the two sugar spellings use.
//
// VALIDATES: RFC 8955 Section 7 makes the action an extended community, so
// `community <action>` reaches route.ParseExtendedCommunities and every form it
// defines is reachable. `rate-limit` and `discard` keep producing the exact
// attribute bytes they produced before, which is what "unchanged for the
// operator" means at the wire.
// PREVENTS: two failures that a "the command still ran" assertion misses. The
// sugar quietly encoding something else once it shares a path with the general
// form, and the general form silently DROPPING the tail: every action parser
// answers how many tokens it read, that count was discarded, and an operator
// typo after a complete action would have been announced as a rule they never
// described.
func TestFlowspecActionAcceptsACommunity(t *testing.T) {
	reg := NewRegistry(func(*selector.Selector, bgptypes.NLRIBatch, plugin.Sender) error { return nil })
	ctx := &pluginserver.CommandContext{}

	announce := func(t *testing.T, args []string) []byte {
		t.Helper()
		rctr := &captureReactor{}
		resp, err := handleAnnounceFlowspec(ctx, rctr, reg, args)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.Equal(t, 1, rctr.calls)
		return rctr.batch.Attrs.Build()
	}

	const prefix = "192.0.2.0/24"

	// A rate of 9600 through the sugar and through the community are the same
	// announcement, so the attribute sections are byte-identical.
	sugar := announce(t, []string{"destination-ipv4", prefix, "rate-limit", "9600"})
	spelled := announce(t, []string{"destination-ipv4", prefix, "community", "traffic-rate", "0", "9600", "bytes"})
	assert.Equal(t, sugar, spelled, "rate-limit is a spelling of a traffic-rate community")

	discard := announce(t, []string{"destination-ipv4", prefix, "discard"})
	zeroRate := announce(t, []string{"destination-ipv4", prefix, "community", "traffic-rate", "0", "0", "bytes"})
	assert.Equal(t, discard, zeroRate, "discard is a spelling of a traffic-rate of zero")

	// redirect is an action the two keywords cannot spell, and reaching it is
	// the point of the community form.
	redirect := announce(t, []string{"destination-ipv4", prefix, "community", "redirect", "65001", "100"})
	assert.NotEqual(t, discard, redirect, "redirect encodes an action of its own")

	// The trailing options still cut the action short.
	withOpts := announce(t, []string{"destination-ipv4", prefix, "community", "redirect", "65001", "100", "for", "300s"})
	assert.Equal(t, redirect, withOpts, "the trailing options are not action tokens")

	for _, tc := range []struct {
		name string
		args []string
	}{
		{"a word after a complete action", []string{"destination-ipv4", prefix, "community", "redirect", "65001", "100", "junk"}},
		{"a word after the sugar's own form", []string{"destination-ipv4", prefix, "community", "traffic-rate", "0", "9600", "bytes", "junk"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rctr := &captureReactor{}
			_, err := handleAnnounceFlowspec(ctx, rctr, reg, tc.args)
			require.ErrorIs(t, err, errFlowspecActionExtraTokens)
			assert.Equal(t, 0, rctr.calls, "nothing is announced when a token is left over")
		})
	}
}

// TestAnnounceRefusesAWordAfterTheOptions drives the reported defect through
// each announce verb's own handler, which is where an operator's tokens arrive
// from the dispatcher.
//
// VALIDATES: a word the options region does not claim stops the command with an
// error naming that word, and nothing is dispatched to the reactor.
// PREVENTS: the reported failure and its two siblings. `announce flowspec
// destination-ipv4 1.1.1.1/32 discard rate-limit 500` put a plain discard on the
// wire and threw `rate-limit 500` away in silence, so the operator asked for a
// rate limit, got a blackhole, and was told the command was done.
func TestAnnounceRefusesAWordAfterTheOptions(t *testing.T) {
	reg := NewRegistry(func(*selector.Selector, bgptypes.NLRIBatch, plugin.Sender) error { return nil })
	ctx := &pluginserver.CommandContext{}

	for _, tc := range []struct {
		name  string
		args  []string
		token string
		run   func(rctr *captureReactor, args []string) (*plugin.Response, error)
	}{
		{
			name:  "flowspec rate-limit after discard",
			args:  []string{"destination-ipv4", "1.1.1.1/32", "discard", "rate-limit", "500"},
			token: "rate-limit",
			run: func(rctr *captureReactor, args []string) (*plugin.Response, error) {
				return handleAnnounceFlowspec(ctx, rctr, reg, args)
			},
		},
		{
			name:  "flowspec word after the tag",
			args:  []string{"destination-ipv4", "1.1.1.1/32", "discard", "tag", "m", "d", "junk"},
			token: "junk",
			run: func(rctr *captureReactor, args []string) (*plugin.Response, error) {
				return handleAnnounceFlowspec(ctx, rctr, reg, args)
			},
		},
		{
			name:  "unicast word after the duration",
			args:  []string{"198.51.100.0/24", "for", "300s", "junk"},
			token: "junk",
			run: func(rctr *captureReactor, args []string) (*plugin.Response, error) {
				return handleAnnounceUnicast(ctx, rctr, reg, args)
			},
		},
		{
			name:  "blackhole word after the tag",
			args:  []string{"198.51.100.0/24", "tag", "m", "d", "junk"},
			token: "junk",
			run: func(rctr *captureReactor, args []string) (*plugin.Response, error) {
				return handleAnnounceBlackhole(ctx, rctr, reg, args)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rctr := &captureReactor{}
			_, err := tc.run(rctr, tc.args)
			require.ErrorIs(t, err, errTrailingOptUnclaimed)
			assert.Contains(t, err.Error(), tc.token, "the error names the token the operator typed")
			assert.Equal(t, 0, rctr.calls, "nothing is announced when a token is left over")
		})
	}
}

// TestAnnounceReadsEveryTrailingOption is the positive half of the pair above:
// the options an announce form does claim still reach the announcement.
//
// VALIDATES: `tag <key> <value>`, `for <duration>`, both together, and neither,
// each over the flowspec handler an operator's tokens arrive at.
// PREVENTS: a refusal wide enough to reject the grammar it is guarding.
func TestAnnounceReadsEveryTrailingOption(t *testing.T) {
	reg := NewRegistry(func(*selector.Selector, bgptypes.NLRIBatch, plugin.Sender) error { return nil })
	ctx := &pluginserver.CommandContext{}
	const prefix = "1.1.1.1/32"

	for _, tc := range []struct {
		name    string
		args    []string
		wantTag string
	}{
		{"no options", []string{"destination-ipv4", prefix, "discard"}, ""},
		{"tag only", []string{"destination-ipv4", prefix, "discard", "tag", "mitigation", "ddos-udp"}, "mitigation=ddos-udp"},
		{"duration only", []string{"destination-ipv4", prefix, "discard", "for", "300s"}, ""},
		{"tag and duration", []string{"destination-ipv4", prefix, "rate-limit", "500", "tag", "m", "d", "for", "300s"}, "m=d"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rctr := &captureReactor{}
			resp, err := handleAnnounceFlowspec(ctx, rctr, reg, tc.args)
			require.NoError(t, err)
			require.Equal(t, 1, rctr.calls, "the announcement is dispatched")
			if tc.wantTag == "" {
				assert.NotContains(t, respData(t, resp), "tag")
				return
			}
			assert.Equal(t, tc.wantTag, respData(t, resp)["tag"])
		})
	}
}

func TestParseTrailingOptsTag(t *testing.T) {
	opts, err := parseTrailingOpts([]string{"tag", "mitigation", "ddos-udp"})
	require.NoError(t, err)
	assert.Equal(t, "mitigation", opts.tagKey)
	assert.Equal(t, "ddos-udp", opts.tagValue)
	assert.Equal(t, time.Duration(0), opts.duration)
}

func TestParseTrailingOptsTagAndDuration(t *testing.T) {
	opts, err := parseTrailingOpts([]string{"tag", "m", "d", "for", "300s"})
	require.NoError(t, err)
	assert.Equal(t, "m", opts.tagKey)
	assert.Equal(t, "d", opts.tagValue)
	assert.Equal(t, 300*time.Second, opts.duration)
}

func TestParseTrailingOptsDurationOnly(t *testing.T) {
	opts, err := parseTrailingOpts([]string{"for", "60s"})
	require.NoError(t, err)
	assert.Equal(t, "", opts.tagKey)
	assert.Equal(t, 60*time.Second, opts.duration)
}

func TestParseTrailingOptsEmpty(t *testing.T) {
	opts, err := parseTrailingOpts(nil)
	require.NoError(t, err)
	assert.Equal(t, "", opts.tagKey)
	assert.Equal(t, time.Duration(0), opts.duration)
}

// TestParseTrailingOptsUnknownTokenErrors pins the opposite of what this test
// asserted until 2026-08-31, when it said an unknown token ends the parse and
// the words after it are discarded without an error.
//
// VALIDATES: a word no option keyword claims is refused, and the message names
// that word so the operator can see which one it was.
// PREVENTS: the loss of every token after the unknown one. The fixture is the
// shape that hid the defect: `tag a b` follows the unknown word and would have
// been read had the parse continued, so a silent stop answers with an
// announcement carrying neither the tag nor the word the operator typed.
func TestParseTrailingOptsUnknownTokenErrors(t *testing.T) {
	_, err := parseTrailingOpts([]string{"bogus", "tag", "a", "b"})
	require.ErrorIs(t, err, errTrailingOptUnclaimed)
	assert.Contains(t, err.Error(), "bogus", "the error names the token the operator typed")
}

func TestParseTrailingOptsTagMissingValue(t *testing.T) {
	_, err := parseTrailingOpts([]string{"tag", "key-only"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tag requires <key> <value>")
}

func TestParseTrailingOptsTagTooLong(t *testing.T) {
	longKey := strings.Repeat("x", maxTagLen+1)
	_, err := parseTrailingOpts([]string{"tag", longKey, "value"})
	assert.ErrorIs(t, err, errTagTooLong)
}

func TestParseTrailingOptsTagValueTooLong(t *testing.T) {
	longVal := strings.Repeat("x", maxTagLen+1)
	_, err := parseTrailingOpts([]string{"tag", "key", longVal})
	assert.ErrorIs(t, err, errTagTooLong)
}

func TestParseTrailingOptsTagAtMaxLen(t *testing.T) {
	maxKey := strings.Repeat("x", maxTagLen)
	opts, err := parseTrailingOpts([]string{"tag", maxKey, "val"})
	require.NoError(t, err)
	assert.Equal(t, maxKey, opts.tagKey)
}

func TestParseTrailingOptsDurationMissing(t *testing.T) {
	_, err := parseTrailingOpts([]string{"for"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "for requires <duration>")
}

func TestParseTrailingOptsDurationZero(t *testing.T) {
	_, err := parseTrailingOpts([]string{"for", "0"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duration must be positive")
}

func TestParseTrailingOptsDurationNegative(t *testing.T) {
	_, err := parseTrailingOpts([]string{"for", "-5s"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duration must be positive")
}

func TestWithdrawByTagKeyValue(t *testing.T) {
	r, _ := newTestRegistry()
	mustAnnounce(t, r, "m", "a", "*", family.IPv4Unicast, "cli", 0)
	mustAnnounce(t, r, "m", "b", "*", family.IPv4Unicast, "cli", 0)
	mustAnnounce(t, r, "other", "x", "*", family.IPv4Unicast, "cli", 0)

	resp, err := withdrawByTag(r, "", []string{"m", "a"})
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)
	data := respData(t, resp)
	assert.Equal(t, 1, data["withdrawn"])
	assert.Equal(t, 2, r.Len())
}

func TestWithdrawByTagKeyWildcard(t *testing.T) {
	r, _ := newTestRegistry()
	mustAnnounce(t, r, "m", "a", "*", family.IPv4Unicast, "cli", 0)
	mustAnnounce(t, r, "m", "b", "*", family.IPv4Unicast, "cli", 0)
	mustAnnounce(t, r, "other", "x", "*", family.IPv4Unicast, "cli", 0)

	resp, err := withdrawByTag(r, "", []string{"m", "*"})
	require.NoError(t, err)
	data := respData(t, resp)
	assert.Equal(t, 2, data["withdrawn"])
	assert.Equal(t, 1, r.Len())
}

func TestWithdrawByTagKeyOnly(t *testing.T) {
	r, _ := newTestRegistry()
	mustAnnounce(t, r, "m", "a", "*", family.IPv4Unicast, "cli", 0)
	mustAnnounce(t, r, "m", "b", "*", family.IPv4Unicast, "cli", 0)

	resp, err := withdrawByTag(r, "", []string{"m"})
	require.NoError(t, err)
	data := respData(t, resp)
	assert.Equal(t, 2, data["withdrawn"])
}

func TestWithdrawByTagStar(t *testing.T) {
	r, _ := newTestRegistry()
	mustAnnounce(t, r, "a", "1", "*", family.IPv4Unicast, "cli", 0)
	mustAnnounce(t, r, "b", "2", "*", family.IPv4Unicast, "cli", 0)

	resp, err := withdrawByTag(r, "", []string{"*"})
	require.NoError(t, err)
	data := respData(t, resp)
	assert.Equal(t, 2, data["withdrawn"])
	assert.Equal(t, 0, r.Len())
}

func TestWithdrawByTagMissingArgs(t *testing.T) {
	r, _ := newTestRegistry()
	_, err := withdrawByTag(r, "", nil)
	assert.Error(t, err)
}

func TestWithdrawByIDValid(t *testing.T) {
	r, _ := newTestRegistry()
	id := mustAnnounce(t, r, "a", "1", "*", family.IPv4Unicast, "cli", 0)

	resp, err := withdrawByID(r, "", "1")
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)
	_ = id
	assert.Equal(t, 0, r.Len())
}

func TestWithdrawByIDNotFound(t *testing.T) {
	r, _ := newTestRegistry()
	_, err := withdrawByID(r, "", "999")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestWithdrawByIDInvalid(t *testing.T) {
	r, _ := newTestRegistry()
	_, err := withdrawByID(r, "", "abc")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid id")
}

func TestWithdrawByIDMissing(t *testing.T) {
	r, _ := newTestRegistry()
	_, err := withdrawByID(r, "", "")
	assert.Error(t, err)
}

func TestWithdrawEveryEmpty(t *testing.T) {
	r, _ := newTestRegistry()
	resp, err := withdrawEvery(r, "")
	require.NoError(t, err)
	data := respData(t, resp)
	assert.Equal(t, 0, data["withdrawn"])
}

func TestWithdrawEveryWithEntries(t *testing.T) {
	r, _ := newTestRegistry()
	mustAnnounce(t, r, "a", "1", "upstream", family.IPv4Unicast, "cli", 0)
	mustAnnounce(t, r, "b", "2", "peer-a", family.IPv4Unicast, "cli", 0)

	resp, err := withdrawEvery(r, "")
	require.NoError(t, err)
	data := respData(t, resp)
	assert.Equal(t, 2, data["withdrawn"])
	assert.Equal(t, 0, r.Len())
}

func TestWithdrawEveryWithSelector(t *testing.T) {
	r, _ := newTestRegistry()
	mustAnnounce(t, r, "a", "1", "upstream", family.IPv4Unicast, "cli", 0)
	mustAnnounce(t, r, "b", "2", "peer-a", family.IPv4Unicast, "cli", 0)

	resp, err := withdrawEvery(r, "upstream")
	require.NoError(t, err)
	data := respData(t, resp)
	assert.Equal(t, 1, data["withdrawn"])
	assert.Equal(t, 1, r.Len())
}

// TestHandleAnnounceUnicastRejectsLinkLocalNextHop drives the RFC 2545 Section 3
// next-hop form guard from the CLI verb that reaches it, rather than from the
// helper alone.
//
// RFC 2545 Section 3: "A BGP speaker shall advertise to its peer in the Network
// Address of Next Hop field the global IPv6 address of the next hop, potentially
// followed by the link-local IPv6 address of the next hop." Ze appends the second
// address itself when the section's condition holds, so a link-local supplied as
// THE next hop has no global address to follow.
//
// VALIDATES: `announce unicast <prefix> next-hop fe80::cafe` errors and dispatches
// nothing.
// PREVENTS: the CLI reaching the encoder with a link-local as the sole next hop,
// which would put it in the field's first slot.
func TestHandleAnnounceUnicastRejectsLinkLocalNextHop(t *testing.T) {
	reg := NewRegistry(func(*selector.Selector, bgptypes.NLRIBatch, plugin.Sender) error { return nil })
	ctx := &pluginserver.CommandContext{}
	rctr := &captureReactor{}

	_, err := handleAnnounceUnicast(ctx, rctr, reg, []string{"2001:db8:1::1/128", "next-hop", "fe80::cafe"})

	require.Error(t, err)
	assert.ErrorIs(t, err, attribute.ErrLinkLocalNextHop)
	assert.Equal(t, 0, rctr.calls, "nothing dispatched on a refused next hop")
}

// TestHandleAnnounceUnicastAcceptsGlobalNextHop is the other side of the guard.
//
// VALIDATES: a global IPv6 next hop, an IPv4 next hop, and `self` all reach the
// reactor. Without this row the guard could refuse everything and still look
// correct.
func TestHandleAnnounceUnicastAcceptsGlobalNextHop(t *testing.T) {
	reg := NewRegistry(func(*selector.Selector, bgptypes.NLRIBatch, plugin.Sender) error { return nil })
	ctx := &pluginserver.CommandContext{}

	cases := []struct {
		nextHop string
		prefix  string
	}{
		{"2001:db8::ffff", "2001:db8:1::1/128"},
		{"::1", "2001:db8:1::1/128"},
		{"192.0.2.1", "198.51.100.0/24"},
		{"self", "2001:db8:1::1/128"},
	}
	for _, tc := range cases {
		t.Run(tc.nextHop, func(t *testing.T) {
			rctr := &captureReactor{}
			_, err := handleAnnounceUnicast(ctx, rctr, reg, []string{tc.prefix, "next-hop", tc.nextHop})
			require.NoError(t, err)
			assert.Equal(t, 1, rctr.calls, "verb must dispatch exactly one batch")
		})
	}
}
