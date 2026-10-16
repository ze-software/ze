// Design: docs/architecture/config/syntax.md -- FlowSpec config route parsing
// Overview: plugin_encode_text.go -- the config and text parsers that read the core vocabulary tables
package flowspec

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/nlri/flowspec"
)

// TestConfigProtocolNamesResolve drives the CONFIG parser over every protocol
// name this package accepts and asserts each one produces a match.
//
// VALIDATES: parseFlowProtocolMatches reads ProtocolNameToNumber, so the config
// path knows every name the text encoder and the JSON decoder know.
// PREVENTS: the divergence that shipped before this test existed. The config
// parser held a private table carrying esp and ah but NOT icmpv6, ospf or sctp.
// An unmatched name is neither an error nor a log in that parser, it is a
// `continue`, so `protocol =sctp` produced a FlowSpec rule with NO protocol
// component at all and matched every protocol -- a firewall rule strictly wider
// than the operator wrote.
func TestConfigProtocolNamesResolve(t *testing.T) {
	for name, number := range flowspec.ProtocolNameToNumber {
		matches := parseFlowProtocolMatches("=" + name)
		if len(matches) != 1 {
			t.Errorf("protocol %q: got %d matches, want 1 -- the component is dropped and the rule matches every protocol", name, len(matches))
			continue
		}
		if matches[0].Value != uint64(number) {
			t.Errorf("protocol %q: got value %d, want %d", name, matches[0].Value, number)
		}
	}
}

// TestConfigTCPFlagAliasesResolve drives the config parser over every TCP flag
// spelling, aliases included.
//
// VALIDATES: parseFlowTCPFlagMatches reads TCPFlagNameToValue.
// PREVENTS: the config parser accepting `reset` and `urgent` while the text
// encoder rejected them, which is how the two tables differed.
func TestConfigTCPFlagAliasesResolve(t *testing.T) {
	for name, value := range flowspec.TCPFlagNameToValue {
		matches := parseFlowTCPFlagMatches(name)
		if len(matches) != 1 {
			t.Errorf("tcp flag %q: got %d matches, want 1", name, len(matches))
			continue
		}
		if matches[0].Value != uint64(value) {
			t.Errorf("tcp flag %q: got value 0x%02x, want 0x%02x", name, matches[0].Value, value)
		}
	}
}

// TestConfigFragmentFlagsResolve drives the config parser over every fragment
// flag spelling, the short aliases included.
//
// VALIDATES: parseFlowFragment reads FragmentFlagNameToValue, so the four
// short aliases the text encoder accepted are no longer refused by config.
// PREVENTS: `fragment df` parsing to nothing, which drops the component and
// widens the rule in silence.
func TestConfigFragmentFlagsResolve(t *testing.T) {
	for name, value := range flowspec.FragmentFlagNameToValue {
		flags := parseFlowFragment(name)
		if len(flags) != 1 {
			t.Errorf("fragment flag %q: got %d flags, want 1", name, len(flags))
			continue
		}
		if uint8(flags[0]) != value {
			t.Errorf("fragment flag %q: got 0x%02x, want 0x%02x", name, uint8(flags[0]), value)
		}
	}
}

// TestTrafficClassIsOneComponentInBothPaths drives the two operator paths that
// reach type 11 and asserts they answer the same thing.
//
// VALIDATES: traffic-class and dscp name one component. The text path is what
// `send bgp <selector> flowspec traffic-class 10` reaches, and the config path
// is what a `flow { traffic-class 10 }` route block reaches.
// PREVENTS: the divergence that shipped before this test existed. The config
// builder mapped traffic-class onto NewFlowDSCPComponent while the text encoder
// held no such keyword, so one word was a valid rule in a config file and
// `unknown component: traffic-class` over the API.
func TestTrafficClassIsOneComponentInBothPaths(t *testing.T) {
	dscp, err := EncodeFlowSpecComponents(IPv4FlowSpec, []string{flowspec.KwDSCP, "=10"})
	if err != nil {
		t.Fatalf("text path, %s: %v", flowspec.KwDSCP, err)
	}

	alias, err := EncodeFlowSpecComponents(IPv4FlowSpec, []string{flowspec.KwTrafficClass, "=10"})
	if err != nil {
		t.Fatalf("text path, %s: %v", flowspec.KwTrafficClass, err)
	}
	if !slices.Equal(dscp, alias) {
		t.Errorf("text path: %s encodes %x and %s encodes %x", flowspec.KwDSCP, dscp, flowspec.KwTrafficClass, alias)
	}

	fs, dropped := buildFlowSpecComponents(map[string][]string{flowspec.KwTrafficClass: {"=10"}}, false)
	if len(dropped) != 0 {
		t.Fatalf("config path dropped %v", dropped)
	}
	if fs == nil || len(fs.Components()) != 1 {
		t.Fatalf("config path built %v, want one component", fs)
	}
	if got := fs.Components()[0].Type(); got != flowspec.FlowDSCP {
		t.Errorf("config path built component type %v, want %v", got, flowspec.FlowDSCP)
	}
}
