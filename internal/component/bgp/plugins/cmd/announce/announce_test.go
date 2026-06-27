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

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

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

func TestParseTrailingOptsUnknownTokenStops(t *testing.T) {
	opts, err := parseTrailingOpts([]string{"bogus", "tag", "a", "b"})
	require.NoError(t, err)
	assert.Equal(t, "", opts.tagKey)
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

func TestHandlewithdrawTagKeyValue(t *testing.T) {
	r, _ := newTestRegistry()
	mustAnnounce(t, r, "m", "a", "*", family.IPv4Unicast, "cli", 0)
	mustAnnounce(t, r, "m", "b", "*", family.IPv4Unicast, "cli", 0)
	mustAnnounce(t, r, "other", "x", "*", family.IPv4Unicast, "cli", 0)

	resp, err := handlewithdrawTag(r, []string{"m", "a"})
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)
	data := respData(t, resp)
	assert.Equal(t, 1, data["withdrawn"])
	assert.Equal(t, 2, r.Len())
}

func TestHandlewithdrawTagKeyWildcard(t *testing.T) {
	r, _ := newTestRegistry()
	mustAnnounce(t, r, "m", "a", "*", family.IPv4Unicast, "cli", 0)
	mustAnnounce(t, r, "m", "b", "*", family.IPv4Unicast, "cli", 0)
	mustAnnounce(t, r, "other", "x", "*", family.IPv4Unicast, "cli", 0)

	resp, err := handlewithdrawTag(r, []string{"m", "*"})
	require.NoError(t, err)
	data := respData(t, resp)
	assert.Equal(t, 2, data["withdrawn"])
	assert.Equal(t, 1, r.Len())
}

func TestHandlewithdrawTagKeyOnly(t *testing.T) {
	r, _ := newTestRegistry()
	mustAnnounce(t, r, "m", "a", "*", family.IPv4Unicast, "cli", 0)
	mustAnnounce(t, r, "m", "b", "*", family.IPv4Unicast, "cli", 0)

	resp, err := handlewithdrawTag(r, []string{"m"})
	require.NoError(t, err)
	data := respData(t, resp)
	assert.Equal(t, 2, data["withdrawn"])
}

func TestHandlewithdrawTagStar(t *testing.T) {
	r, _ := newTestRegistry()
	mustAnnounce(t, r, "a", "1", "*", family.IPv4Unicast, "cli", 0)
	mustAnnounce(t, r, "b", "2", "*", family.IPv4Unicast, "cli", 0)

	resp, err := handlewithdrawTag(r, []string{"*"})
	require.NoError(t, err)
	data := respData(t, resp)
	assert.Equal(t, 2, data["withdrawn"])
	assert.Equal(t, 0, r.Len())
}

func TestHandlewithdrawTagMissingArgs(t *testing.T) {
	r, _ := newTestRegistry()
	_, err := handlewithdrawTag(r, nil)
	assert.Error(t, err)
}

func TestHandleWithdrawIDValid(t *testing.T) {
	r, _ := newTestRegistry()
	id := mustAnnounce(t, r, "a", "1", "*", family.IPv4Unicast, "cli", 0)

	resp, err := handleWithdrawID(r, []string{"1"})
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)
	_ = id
	assert.Equal(t, 0, r.Len())
}

func TestHandleWithdrawIDNotFound(t *testing.T) {
	r, _ := newTestRegistry()
	_, err := handleWithdrawID(r, []string{"999"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestHandleWithdrawIDInvalid(t *testing.T) {
	r, _ := newTestRegistry()
	_, err := handleWithdrawID(r, []string{"abc"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid id")
}

func TestHandleWithdrawIDMissing(t *testing.T) {
	r, _ := newTestRegistry()
	_, err := handleWithdrawID(r, nil)
	assert.Error(t, err)
}

func TestHandlewithdrawAllEmpty(t *testing.T) {
	r, _ := newTestRegistry()
	resp, err := handlewithdrawAll(r, nil)
	require.NoError(t, err)
	data := respData(t, resp)
	assert.Equal(t, 0, data["withdrawn"])
}

func TestHandlewithdrawAllWithEntries(t *testing.T) {
	r, _ := newTestRegistry()
	mustAnnounce(t, r, "a", "1", "upstream", family.IPv4Unicast, "cli", 0)
	mustAnnounce(t, r, "b", "2", "peer-a", family.IPv4Unicast, "cli", 0)

	resp, err := handlewithdrawAll(r, nil)
	require.NoError(t, err)
	data := respData(t, resp)
	assert.Equal(t, 2, data["withdrawn"])
	assert.Equal(t, 0, r.Len())
}

func TestHandlewithdrawAllWithSelector(t *testing.T) {
	r, _ := newTestRegistry()
	mustAnnounce(t, r, "a", "1", "upstream", family.IPv4Unicast, "cli", 0)
	mustAnnounce(t, r, "b", "2", "peer-a", family.IPv4Unicast, "cli", 0)

	resp, err := handlewithdrawAll(r, []string{"selector", "upstream"})
	require.NoError(t, err)
	data := respData(t, resp)
	assert.Equal(t, 1, data["withdrawn"])
	assert.Equal(t, 1, r.Len())
}
