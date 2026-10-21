// Design: docs/architecture/config/syntax.md -- FlowSpec match criteria and operators
// RFC: rfc/short/rfc8955.md -- numeric operator (Section 4.2.1.1), ports, packet length, ICMP, DSCP
// RFC: rfc/short/rfc8956.md -- flow label (Section 3.7)

package flowspec

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/nlri/flowspec"
)

// TestConfigNumericComponentsTakeOperators builds every numeric criterion
// other than protocol from the config path and compares the component octets.
//
// VALIDATES: buildFlowSpecComponents reads port, destination-port,
// source-port, packet-length, icmp-type, icmp-code, dscp and fragment with the update
// text grammar (parseNumericTermsText): a range joined by `&` encodes the
// second term with the a bit, a list is OR terms, a two-octet value carries
// len=01, and an ICMP name stands in for its number.
// PREVENTS: a second config operator table drifting from the update text one.
func TestConfigNumericComponentsTakeOperators(t *testing.T) {
	t.Parallel()
	// Each operator octet is e(0x80) a(0x40) len(0x30) 0 lt(0x04) gt(0x02)
	// eq(0x01), RFC 8955 Section 4.2.1.1. The component type leads.
	cases := []struct {
		key   string
		value string
		ipv6  bool
		want  string
	}{
		{flowspec.KwDestPort, ">=1024&<=65535", false, "05130400d5ffff"},
		{flowspec.KwDestPort, "=80", false, "058150"},
		{flowspec.KwPort, "[ =80 =8080 ]", false, "040150911f90"},
		{flowspec.KwSourcePort, ">8080&<8088", false, "06121f90d41f98"},
		{flowspec.KwPacketLength, ">=64&<=1500", false, "0a0340d505dc"},
		{flowspec.KwPacketLength, "<=65535", true, "0a95ffff"},
		{flowspec.KwICMPType, "=echo-request", false, "078108"},
		{flowspec.KwICMPType, ">=3&<=5", false, "070303c505"},
		{flowspec.KwICMPCode, "[ host-unreachable 3 ]", false, "0801018103"},
		{flowspec.KwDSCP, "[ 10 46 ]", false, "0b010a812e"},
		{flowspec.KwDSCP, ">=10&<=46", false, "0b030ac52e"},
		{flowspec.KwFragment, "[ first-fragment last-fragment ]", false, "0c00048008"},
		{flowspec.KwFragment, "!is-fragment", false, "0c8202"},
	}
	for _, tc := range cases {
		fs, dropped := buildFlowSpecComponents(map[string][]string{tc.key: {tc.value}}, tc.ipv6)
		if len(dropped) != 0 {
			t.Errorf("%s %s: refused %v, want it built", tc.key, tc.value, dropped)
			continue
		}
		if len(fs.Components()) != 1 {
			t.Errorf("%s %s: built %d components, want 1", tc.key, tc.value, len(fs.Components()))
			continue
		}
		if got := hex.EncodeToString(fs.Components()[0].Bytes()); got != tc.want {
			t.Errorf("%s %s: component %s, want %s", tc.key, tc.value, got, tc.want)
		}
	}
}

// TestConfigFlowLabelTakesOperators builds flow-label ranges and the largest
// 20-bit label from the config path.
//
// VALIDATES: flow-label reads the update text grammar and accepts 1048575, the
// largest value of the 20-bit field (RFC 8956 Section 3.7).
// PREVENTS: flow-label refusing a range the wire carries.
func TestConfigFlowLabelTakesOperators(t *testing.T) {
	t.Parallel()
	for _, value := range []string{">=10&<=20", "[ 1 1048575 ]"} {
		fs, dropped := buildFlowSpecComponents(map[string][]string{flowspec.KwFlowLabel: {value}}, true)
		if len(dropped) != 0 || len(fs.Components()) != 1 {
			t.Errorf("flow-label %s: dropped %v, want one component", value, dropped)
		}
	}
}

// TestConfigNumericRefusalNamesTheValueAndFix feeds malformed values to every
// numeric criterion and asserts each is refused with the criterion, the value
// and the fix.
//
// VALIDATES: a port or packet length past 65535, an ICMP value past 255, a
// flow label past 1048575, a dangling `&` and a leading `&` are each refused,
// and the refusal names the criterion and what to write instead.
// PREVENTS: a value wider than its field reaching the wire as a different
// value, and a refusal naming only the key.
func TestConfigNumericRefusalNamesTheValueAndFix(t *testing.T) {
	t.Parallel()
	cases := []struct {
		key   string
		value string
		ipv6  bool
		want  string
	}{
		{flowspec.KwDestPort, "65536", false, "65535"},
		{flowspec.KwPort, ">=1024&<=70000", false, "70000"},
		{flowspec.KwSourcePort, ">6&", false, "<op><value>"},
		{flowspec.KwDestPort, "&>6", false, "drop the &"},
		{flowspec.KwDestPort, "=http", false, "http"},
		{flowspec.KwPacketLength, "65536", false, "65535"},
		{flowspec.KwPacketLength, "65536", true, "65535"},
		{flowspec.KwICMPType, "256", false, "255"},
		{flowspec.KwICMPCode, "=bogus", false, "bogus"},
		{flowspec.KwDSCP, "64", false, "63"},
		{flowspec.KwFlowLabel, "1048576", true, "1048575"},
		{flowspec.KwFragment, "&first-fragment", false, "drop the &"},
		{flowspec.KwFragment, "bogus", false, "is-fragment"},
	}
	for _, tc := range cases {
		fs, dropped := buildFlowSpecComponents(map[string][]string{tc.key: {tc.value}}, tc.ipv6)
		if len(dropped) != 1 {
			built := ""
			if len(fs.Components()) == 1 {
				built = hex.EncodeToString(fs.Components()[0].Bytes())
			}
			t.Errorf("%s %s: dropped %v (built %s), want it refused", tc.key, tc.value, dropped, built)
			continue
		}
		if !strings.HasPrefix(dropped[0], tc.key+" ") || !strings.Contains(dropped[0], tc.want) {
			t.Errorf("%s %s: refusal %q does not name the criterion and %q", tc.key, tc.value, dropped[0], tc.want)
		}
	}
}
