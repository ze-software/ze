package cmd

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	"codeberg.org/thomas-mangin/ze/internal/component/resolve"
)

// --- requireArg ---

// VALIDATES: requireArg returns the first argument when args is non-empty.
// PREVENTS: Argument parsing failing on valid single-arg input.
func TestRequireArg_Valid(t *testing.T) {
	val, errResp := requireArg([]string{"example.com"}, "hostname")
	assert.Nil(t, errResp)
	assert.Equal(t, "example.com", val)
}

// VALIDATES: requireArg returns error response when args is empty.
// PREVENTS: Missing argument silently succeeding with zero value.
func TestRequireArg_Empty(t *testing.T) {
	val, errResp := requireArg([]string{}, "hostname")
	require.NotNil(t, errResp)
	assert.Equal(t, plugin.StatusError, errResp.Status)
	assert.Contains(t, errResp.Data, "hostname")
	assert.Equal(t, "", val)
}

// VALIDATES: requireArg returns error response when args is nil.
// PREVENTS: Nil slice panic.
func TestRequireArg_Nil(t *testing.T) {
	val, errResp := requireArg(nil, "asn")
	require.NotNil(t, errResp)
	assert.Equal(t, plugin.StatusError, errResp.Status)
	assert.Contains(t, errResp.Data, "asn")
	assert.Equal(t, "", val)
}

// --- requireASN ---

// VALIDATES: requireASN parses valid uint32 ASN values.
// PREVENTS: Valid ASN strings being rejected.
func TestRequireASN_Valid(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want uint32
	}{
		{name: "small ASN", args: []string{"65001"}, want: 65001},
		{name: "zero ASN", args: []string{"0"}, want: 0},
		{name: "max uint32", args: []string{"4294967295"}, want: 4294967295},
		{name: "4-byte ASN", args: []string{"400000"}, want: 400000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asn, errResp := requireASN(tt.args)
			assert.Nil(t, errResp)
			assert.Equal(t, tt.want, asn)
		})
	}
}

// VALIDATES: requireASN returns error for non-numeric input.
// PREVENTS: Non-numeric ASN silently returning zero.
func TestRequireASN_NonNumeric(t *testing.T) {
	asn, errResp := requireASN([]string{"abc"})
	require.NotNil(t, errResp)
	assert.Equal(t, plugin.StatusError, errResp.Status)
	assert.Contains(t, errResp.Data, "invalid ASN")
	assert.Equal(t, uint32(0), asn)
}

// VALIDATES: requireASN returns error for empty args.
// PREVENTS: Missing ASN argument not producing an error.
func TestRequireASN_Empty(t *testing.T) {
	asn, errResp := requireASN([]string{})
	require.NotNil(t, errResp)
	assert.Equal(t, plugin.StatusError, errResp.Status)
	assert.Equal(t, uint32(0), asn)
}

// VALIDATES: requireASN returns error for values exceeding uint32 range.
// PREVENTS: Overflow on ASN larger than 4294967295.
func TestRequireASN_Overflow(t *testing.T) {
	asn, errResp := requireASN([]string{"4294967296"})
	require.NotNil(t, errResp)
	assert.Equal(t, plugin.StatusError, errResp.Status)
	assert.Contains(t, errResp.Data, "invalid ASN")
	assert.Equal(t, uint32(0), asn)
}

// --- errResponse ---

// VALIDATES: errResponse wraps message in StatusError response with nil error.
// PREVENTS: Error responses propagating Go errors that break dispatcher flow.
func TestErrResponse(t *testing.T) {
	resp, err := errResponse("something failed")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Equal(t, "something failed", resp.Data)
}

// --- dnsResult ---

// VALIDATES: dnsResult returns StatusDone with joined records on success.
// PREVENTS: Successful DNS resolution returning error status.
func TestDnsResult_Success(t *testing.T) {
	records := []string{"192.168.1.1", "192.168.1.2"}
	resp, err := dnsResult(records, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	assert.Equal(t, "192.168.1.1\n192.168.1.2", resp.Data)
}

// VALIDATES: dnsResult returns StatusError when resolveErr is non-nil.
// PREVENTS: DNS resolution errors being swallowed.
func TestDnsResult_Error(t *testing.T) {
	resp, err := dnsResult(nil, errors.New("NXDOMAIN"))
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Equal(t, "NXDOMAIN", resp.Data)
}

// VALIDATES: dnsResult handles empty record list.
// PREVENTS: Empty DNS response causing panic.
func TestDnsResult_EmptyRecords(t *testing.T) {
	resp, err := dnsResult([]string{}, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	assert.Equal(t, "", resp.Data)
}

// VALIDATES: dnsResult handles single record.
// PREVENTS: Single-element join producing unexpected separators.
func TestDnsResult_SingleRecord(t *testing.T) {
	resp, err := dnsResult([]string{"10.0.0.1"}, nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	assert.Equal(t, "10.0.0.1", resp.Data)
}

// --- Nil-resolver guard tests for all 9 handlers ---

// handlerEntry is a table entry for nil-resolver guard tests.
type handlerEntry struct {
	name    string
	handler func(*pluginserver.CommandContext, []string) (*plugin.Response, error)
	args    []string // valid args that would succeed with a real resolver
	errMsg  string   // expected error message substring
}

func allHandlers() []handlerEntry {
	return []handlerEntry{
		{name: "dns-a", handler: handleDNSA, args: []string{"example.com"}, errMsg: "DNS resolver not available"},
		{name: "dns-aaaa", handler: handleDNSAAAA, args: []string{"example.com"}, errMsg: "DNS resolver not available"},
		{name: "dns-txt", handler: handleDNSTXT, args: []string{"example.com"}, errMsg: "DNS resolver not available"},
		{name: "dns-ptr", handler: handleDNSPTR, args: []string{"192.168.1.1"}, errMsg: "DNS resolver not available"},
		{name: "cymru-asn-name", handler: handleCymruASNName, args: []string{"65001"}, errMsg: "Cymru resolver not available"},
		{name: "peeringdb-max-prefix", handler: handlePeeringDBMaxPrefix, args: []string{"65001"}, errMsg: "PeeringDB client not available"},
		{name: "peeringdb-as-set", handler: handlePeeringDBASSet, args: []string{"65001"}, errMsg: "PeeringDB client not available"},
		{name: "irr-expand", handler: handleIRRExpand, args: []string{"AS-SET"}, errMsg: "IRR client not available"},
		{name: "irr-prefix", handler: handleIRRPrefix, args: []string{"AS-SET"}, errMsg: "IRR client not available"},
	}
}

// VALIDATES: All handlers return error response when resolvers is nil.
// PREVENTS: Nil-pointer dereference when hub has not initialized resolvers.
func TestHandlers_NilResolvers(t *testing.T) {
	// Save and restore the package-level resolvers.
	saved := resolvers
	t.Cleanup(func() { resolvers = saved })

	SetResolvers(nil)

	for _, h := range allHandlers() {
		t.Run(h.name, func(t *testing.T) {
			resp, err := h.handler(nil, h.args)
			require.NoError(t, err)
			require.NotNil(t, resp)
			assert.Equal(t, plugin.StatusError, resp.Status)
			assert.Contains(t, resp.Data, h.errMsg)
		})
	}
}

// VALIDATES: All handlers return error response when resolvers struct has nil fields.
// PREVENTS: Nil-pointer dereference on zero-value Resolvers with individual nil resolvers.
func TestHandlers_ZeroValueResolvers(t *testing.T) {
	saved := resolvers
	t.Cleanup(func() { resolvers = saved })

	SetResolvers(&resolve.Resolvers{})

	for _, h := range allHandlers() {
		t.Run(h.name, func(t *testing.T) {
			resp, err := h.handler(nil, h.args)
			require.NoError(t, err)
			require.NotNil(t, resp)
			assert.Equal(t, plugin.StatusError, resp.Status)
			assert.Contains(t, resp.Data, h.errMsg)
		})
	}
}

// --- validateTarget ---

func TestValidateTarget_Valid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"ipv4", "192.168.1.1"},
		{"ipv6", "::1"},
		{"hostname", "example.com"},
		{"subdomain", "foo.bar.example.com"},
		{"hyphen", "my-host.example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NoError(t, validateTarget(tt.input))
		})
	}
}

func TestValidateTarget_Invalid(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		errContains string
	}{
		{"empty", "", "must not be empty"},
		{"space", "foo bar", "invalid character"},
		{"semicolon", "foo;bar", "invalid character"},
		{"pipe", "foo|bar", "invalid character"},
		{"long", strings.Repeat("a", 254), "253-character"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTarget(tt.input)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errContains)
		})
	}
}

// --- validateIP ---

func TestValidateSourceIP_Valid(t *testing.T) {
	assert.NoError(t, validateSourceIP("192.168.1.1"))
	assert.NoError(t, validateSourceIP("::1"))
}

func TestValidateSourceIP_Invalid(t *testing.T) {
	err := validateSourceIP("not-an-ip")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a valid IP address")
}

// --- validateUint ---

func TestValidateUint_Valid(t *testing.T) {
	assert.NoError(t, validateUint("5", "count", 1, 100))
	assert.NoError(t, validateUint("1", "count", 1, 100))
	assert.NoError(t, validateUint("100", "count", 1, 100))
}

func TestValidateUint_Invalid(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		lo, hi      uint64
		errContains string
	}{
		{"not number", "abc", 1, 100, "not a valid number"},
		{"too low", "0", 1, 100, "out of range"},
		{"too high", "101", 1, 100, "out of range"},
		{"negative", "-1", 1, 100, "not a valid number"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateUint(tt.input, "count", tt.lo, tt.hi)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errContains)
		})
	}
}

// --- Ping validation ---

func TestHandlePing_InvalidTarget(t *testing.T) {
	resp, err := handlePing(&pluginserver.CommandContext{}, []string{"foo;bar"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Data, "invalid character")
}

func TestHandlePing_InvalidSource(t *testing.T) {
	resp, err := handlePing(&pluginserver.CommandContext{}, []string{"192.168.1.1", "source", "not-ip"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Data, "not a valid IP address")
}

func TestHandlePing_InvalidCount(t *testing.T) {
	resp, err := handlePing(&pluginserver.CommandContext{}, []string{"192.168.1.1", "count", "abc"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Data, "not a valid number")
}

func TestHandlePing_CountOutOfRange(t *testing.T) {
	resp, err := handlePing(&pluginserver.CommandContext{}, []string{"192.168.1.1", "count", "200"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Data, "out of range")
}

func TestHandlePing_SizeOutOfRange(t *testing.T) {
	resp, err := handlePing(&pluginserver.CommandContext{}, []string{"192.168.1.1", "size", "99999"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Data, "out of range")
}

func TestHandlePing_UnknownOption(t *testing.T) {
	resp, err := handlePing(&pluginserver.CommandContext{}, []string{"192.168.1.1", "bogus", "val"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Data, "unknown option")
}

func TestHandlePing_TrailingKeyword(t *testing.T) {
	resp, err := handlePing(&pluginserver.CommandContext{}, []string{"192.168.1.1", "count"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Data, "requires a value")
}

// --- Traceroute validation ---

func TestHandleTraceroute_InvalidTarget(t *testing.T) {
	resp, err := handleTraceroute(&pluginserver.CommandContext{}, []string{"foo;bar"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Data, "invalid character")
}

func TestHandleTraceroute_InvalidSource(t *testing.T) {
	resp, err := handleTraceroute(&pluginserver.CommandContext{}, []string{"192.168.1.1", "source", "not-ip"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Data, "not a valid IP address")
}

func TestHandleTraceroute_UnknownOption(t *testing.T) {
	resp, err := handleTraceroute(&pluginserver.CommandContext{}, []string{"192.168.1.1", "bogus"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Data, "unknown option")
}

func TestHandleTraceroute_SourceMissingValue(t *testing.T) {
	resp, err := handleTraceroute(&pluginserver.CommandContext{}, []string{"192.168.1.1", "source"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Data, "requires a value")
}
