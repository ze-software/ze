// Design: docs/architecture/api/commands.md -- the generated invocation form
//
// usage_model_test.go proves both halves of one change: a command whose value
// lived only in a description now DECLARES that value, and the invocation an
// operator already typed still dispatches (plan/spec-generated-command-usage.md,
// R-5).
//
// The two halves have to be proven together, and here. A leaf is only visible
// to the renderer through the whole command tree, and it changes dispatch only
// through the argument definitions the same tree produces, so a fixture module
// proves neither. This package is the one that may import the composition root
// (internal/component/plugin/all) and also holds the Dispatcher.

package server_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"

	// Trigger every module and RPC registration, so the tree under test is the
	// one the daemon assembles.
	_ "github.com/ze-software/ze/internal/component/plugin/all"
)

// commandNode walks the merged command tree to the node at path, or fails the
// test. It answers the node rather than its usage line so a caller can assert
// the argument definitions too.
func commandNode(t *testing.T, path string) *command.Node {
	t.Helper()
	loader, err := yang.DefaultLoader()
	require.NoError(t, err, "load the embedded YANG modules")
	node := yang.BuildCommandTree(loader)
	for segment := range strings.FieldsSeq(path) {
		require.NotNil(t, node, "no node at %q", path)
		node = node.Children[segment]
	}
	require.NotNil(t, node, "no node at %q", path)
	return node
}

// TestUsageRendersTheDeclaredValues pins the generated invocation form of every
// command this spec taught to declare its own values.
//
// VALIDATES: the value a description used to spell in prose is now a leaf the
// renderer reads, with the leaf's own name and type.
// PREVENTS: a leaf declared with a name or a mandatory flag that renders a line
// no operator could type, which the gate's difference count alone would not
// distinguish from a line nobody generated at all.
func TestUsageRendersTheDeclaredValues(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"create interface address", "create interface address <name> <prefix>"},
		{"create interface bridge name address", "create interface bridge name <name> address <prefix>"},
		{"create interface bridge name unit", "create interface bridge name <name> unit <vid>"},
		{"create interface dummy name address", "create interface dummy name <name> address <prefix>"},
		{"create interface dummy name unit", "create interface dummy name <name> unit <vid>"},
		{"create interface unit", "create interface unit <name> <vid>"},
		{"create interface veth name", "create interface veth name <name> <peer>"},
		{"delete interface name address", "delete interface name <name> address <prefix>"},
		{"request interface mac", "request interface <name> mac <address>"},
		// The one command here whose values are ALL keyword groups. It acts on
		// two interfaces rather than on the one `interface` names, so it takes
		// no inherited value and every value it does take needs a keyword of
		// its own to say which interface it belongs to.
		{"request interface migrate", "request interface migrate from <source> to <destination> address <prefix> [create <dummy|veth|bridge>] [timeout <duration>]"},
		{"request interface mtu", "request interface <name> mtu <bytes>"},
		{"resolve cymru asn-name", "resolve cymru asn-name <asn>"},
		{"resolve dns a", "resolve dns a <hostname>"},
		{"resolve dns aaaa", "resolve dns aaaa <hostname>"},
		{"resolve dns ptr", "resolve dns ptr <ip-address>"},
		{"resolve dns txt", "resolve dns txt <hostname>"},
		{"resolve peeringdb as-set", "resolve peeringdb as-set <asn>"},
		{"resolve peeringdb max-prefix", "resolve peeringdb max-prefix <asn>"},
		{"resolve ping", "resolve ping <target> [source <source>] [count <count>] [size <size>]"},
		{"resolve traceroute", "resolve traceroute <target> [source <source>] [max-hops <max-hops>] [timeout <timeout>] [probes <probes>]"},
		{"show config cat", "show config cat <id>"},
		{"show data cat", "show data cat <key>"},
		{"show env get", "show env get <name>"},
		{"show firewall ruleset", "show firewall ruleset <name>"},
		{"show interface type", "show interface type <type>"},
		{"show route lookup", "show route lookup <ip>"},
		// `announce` itself carries no command since it became three of them on
		// 2026-08-30, so it renders nothing and each form states its own line.
		{"announce unicast", "announce unicast <prefix> [next-hop <address>] [community <value> ...] [tag <key> <value>] [for <duration>]"},
		{"announce blackhole", "announce blackhole <prefix> [tag <key> <value>] [for <duration>]"},
		{"show announcements", "show announcements [tag <tag>] [selector <selector>] [family <family>]"},
		// The value set reads in the order ze-log-cmd.yang declares it, which
		// for a severity is the progression an operator expects. enumNames
		// (internal/component/config/yang/command.go) sorted it by name until
		// the bare-alternation rule needed the module's own order.
		{"request log level", "request log level <logger> <disabled|debug|info|warn|err>"},
		{"request peer teardown", "request peer <selector> teardown <cease-subcode>"},
		// `filter` and `update` each declare a leaf of their own name, so the
		// line reads the leaf rather than the container, and `update` states
		// its value with no brackets because the group is required.
		{"show policy test peer", "show policy test peer <selector> <import|export> [filter <name>] update <hex> [source-asn4 <true|false>]"},
		{"show metrics name", "show metrics name <name> [label <key> <value> ...]"},

		// The four rendering rules this phase added, on the commands that
		// needed them. A required group renders its keyword and its values with
		// no brackets; a group with no leaf renders as a flag; a choice group
		// renders the words its leaf declares and never its own name.
		{
			"debug ip ospf inject opaque",
			"debug ip ospf inject opaque scope <link|area|as> id <opaque-id> " +
				"[type <type>] [hex <body> ...] [tlv <type> <value-hex> ...] [withdraw]",
		},
		{
			"debug ipv6 ospf inject lsa",
			"debug ipv6 ospf inject lsa type <ls-type> id <link-state-id> " +
				"[scope <link|area|as>] [hex <body> ...] [withdraw]",
		},
		{"show policy chain peer", "show policy chain peer <selector> [import|export]"},
		{"show bgp peer rib", "show bgp peer <selector> rib [sent|advertised|received|sent-received]"},
		// Whole-form alternation: each output form is its own command, so the
		// line an operator reads names the form rather than an alternation the
		// parent's description used to spell. The algorithm is a closed set the
		// operator types one word of, which is a choice group and never a
		// keyword-introduced value.
		{"show pki certificate name", "show pki certificate name <name>"},
		{"show pki certificate name pem", "show pki certificate name <name> pem"},
		{"show pki certificate name bundle pem", "show pki certificate name <name> bundle pem"},
		{
			"show pki certificate name fingerprint",
			"show pki certificate name <name> fingerprint [sha256|sha384|sha512]",
		},
	}

	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			node := commandNode(t, c.path)
			got := command.UsageLine(command.Usage(strings.Fields(c.path), node))
			assert.Equal(t, c.want, got)
		})
	}
}

// TestDeclaredValuesKeepAcceptedInvocations dispatches the invocation each
// command accepted BEFORE its value was declared, through the argument
// definitions the model now produces.
//
// VALIDATES: R-5. Declaring a leaf narrows what the dispatcher admits, so every
// form an operator already typed is re-run against the new definitions.
// PREVENTS: a leaf whose type refuses a value its handler accepts, and a leaf
// whose name makes the dispatcher lift the value out of the argument list the
// handler reads.
func TestDeclaredValuesKeepAcceptedInvocations(t *testing.T) {
	cases := []struct {
		path  string
		input string
		// requiresSelector mirrors the RPCRegistration of the command under
		// test. It changes whether Dispatch adopts a lone trailing positional
		// as the peer selector, so a case that carries one has to declare it.
		requiresSelector bool
		// peer is the selector an operator supplies out of band, which is how
		// `announce` gets one: its own tail is a route, not a selector.
		peer      string
		args      []string
		selectors map[string]string
	}{
		{
			path:      "create interface address",
			input:     "create interface eth0 address 10.0.0.1/24",
			args:      []string{"10.0.0.1/24"},
			selectors: map[string]string{"name": "eth0"},
		},
		{
			// The documented form of `show policy test peer`, which 5f5b73261
			// refused: it declared selector, filter, update and source-asn4 and
			// no direction, so the bare `export` token reached the filter leaf
			// and the enum that should have claimed it did not exist. The
			// mis-binding is the half a passing test hid: `... export update
			// <hex>` was accepted with `export` read as a filter NAME, and only
			// survived because the handler re-parses the raw tokens. Binding
			// `export` to direction rather than to filter is what this asserts.
			path:  "show policy test peer",
			input: "show policy test peer receiver-peer export filter STRIP update 0xffff",
			// The whole tail reaches the handler, selector included, because
			// parsePolicyTestArgs reads it itself. The declared leaves exist so
			// validateCommandArgs can PLACE each token, not so it can consume
			// them: what matters is that `export` is placed on the direction
			// enum rather than offered to the filter string.
			args: []string{"receiver-peer", "export", "filter", "STRIP", "update", "0xffff"},
		},
		{
			path:      "create interface unit",
			input:     "create interface eth0 unit 100",
			args:      []string{"100"},
			selectors: map[string]string{"name": "eth0"},
		},
		{
			path:      "create interface dummy name unit",
			input:     "create interface dummy name zeens0 unit 100",
			args:      []string{"100"},
			selectors: map[string]string{"name": "zeens0"},
		},
		{
			path:      "create interface dummy name address",
			input:     "create interface dummy name zeens0 address 10.0.0.1/24",
			args:      []string{"10.0.0.1/24"},
			selectors: map[string]string{"name": "zeens0"},
		},
		{
			path:      "create interface bridge name unit",
			input:     "create interface bridge name br0 unit 4094",
			args:      []string{"4094"},
			selectors: map[string]string{"name": "br0"},
		},
		{
			path:      "create interface bridge name address",
			input:     "create interface bridge name br0 address 2001:db8::1/64",
			args:      []string{"2001:db8::1/64"},
			selectors: map[string]string{"name": "br0"},
		},
		{
			path:      "create interface veth name",
			input:     "create interface veth name veth0 veth1",
			args:      []string{"veth1"},
			selectors: map[string]string{"name": "veth0"},
		},
		{
			path:      "delete interface name address",
			input:     "delete interface name zetest0 address 10.99.0.1/24",
			args:      []string{"10.99.0.1/24"},
			selectors: map[string]string{"name": "zetest0"},
		},
		{
			path:      "request interface mtu",
			input:     "request interface zetest0 mtu 1400",
			args:      []string{"1400"},
			selectors: map[string]string{"name": "zetest0"},
		},
		{
			path:      "request interface mac",
			input:     "request interface zetest0 mac 02:de:ad:be:ef:01",
			args:      []string{"02:de:ad:be:ef:01"},
			selectors: map[string]string{"name": "zetest0"},
		},
		{
			path:  "resolve cymru asn-name",
			input: "resolve cymru asn-name 64512",
			args:  []string{"64512"},
		},
		{
			path:  "resolve dns a",
			input: "resolve dns a example.com",
			args:  []string{"example.com"},
		},
		{
			path:  "resolve dns ptr",
			input: "resolve dns ptr 192.0.2.1",
			args:  []string{"192.0.2.1"},
		},
		{
			path:  "resolve peeringdb max-prefix",
			input: "resolve peeringdb max-prefix 64512",
			args:  []string{"64512"},
		},
		{
			path:  "resolve ping",
			input: "resolve ping 192.0.2.1 source 10.0.0.1 count 4 size 56",
			args:  []string{"192.0.2.1", "source", "10.0.0.1", "count", "4", "size", "56"},
		},
		{
			path:  "resolve traceroute",
			input: "resolve traceroute example.com source 10.0.0.1 max-hops 30 timeout 2s probes 3",
			args:  []string{"example.com", "source", "10.0.0.1", "max-hops", "30", "timeout", "2s", "probes", "3"},
		},
		{
			path:  "show config cat",
			input: "show config cat 42",
			args:  []string{"42"},
		},
		{
			path:  "show data cat",
			input: "show data cat file/active/etc/ze/router.conf",
			args:  []string{"file/active/etc/ze/router.conf"},
		},
		{
			path:  "show env get",
			input: "show env get ze.cli.format",
			args:  []string{"ze.cli.format"},
		},
		{
			path:  "show firewall ruleset",
			input: "show firewall ruleset filter",
			args:  []string{"filter"},
		},
		{
			path:  "show interface type",
			input: "show interface type ethernet",
			// The leaf and its container are both called `type`, so the value
			// is lifted into the selectors and the tail is empty.
			args:      []string{},
			selectors: map[string]string{"type": "ethernet"},
		},
		{
			path:  "show route lookup",
			input: "show route lookup 192.0.2.1",
			args:  []string{"192.0.2.1"},
		},
		{
			path:             "announce unicast",
			input:            "announce unicast 10.0.0.0/24 next-hop self tag color blue for 300s",
			requiresSelector: true,
			peer:             "edge1",
			args:             []string{"10.0.0.0/24", "next-hop", "self", "tag", "color", "blue", "for", "300s"},
			// A modifier group is a child of the command node, so its leaves are
			// not the command's own argument definitions and the whole tail
			// still reaches parseTrailingOpts unchanged. No selector is adopted:
			// the tail is nine tokens, and the fence in Dispatch takes one only
			// from a LONE spare positional.
		},
		{
			path:  "show announcements",
			input: "show announcements tag color selector edge1 family ipv4",
			args:  []string{"tag", "color", "selector", "edge1", "family", "ipv4"},
		},
		{
			path:  "request log level",
			input: "request log level bgp.reactor debug",
			args:  []string{"bgp.reactor", "debug"},
		},
		{
			path:             "request peer teardown",
			input:            "request peer edge1 teardown 6",
			requiresSelector: true,
			args:             []string{"6"},
			selectors:        map[string]string{"selector": "edge1"},
		},
		{
			path:  "show policy chain peer",
			input: "show policy chain peer edge1 import",
			args:  []string{"edge1", "import"},
		},
		{
			path:  "show policy test peer",
			input: "show policy test peer edge1 import filter my-filter update ffffffffffffffffffffffffffffffff00132 source-asn4 false",
			args:  []string{"edge1", "import", "filter", "my-filter", "update", "ffffffffffffffffffffffffffffffff00132", "source-asn4", "false"},
		},
		// The nineteen commands whose value the operator types between the
		// container that names the object and the action. The leaf moved onto
		// that container, so the model now states the position; each case
		// re-runs the invocation the dispatcher accepted before the move.
		{
			path:      "request interface up",
			input:     "request interface zetest0 up",
			args:      []string{},
			selectors: map[string]string{"name": "zetest0"},
		},
		{
			path:      "request interface down",
			input:     "request interface zetest0 down",
			args:      []string{},
			selectors: map[string]string{"name": "zetest0"},
		},
		{
			path:             "request peer flush",
			input:            "request peer edge1 flush",
			requiresSelector: true,
			args:             []string{},
			selectors:        map[string]string{"selector": "edge1"},
		},
		{
			path:             "request peer pause",
			input:            "request peer edge1 pause",
			requiresSelector: true,
			args:             []string{},
			selectors:        map[string]string{"selector": "edge1"},
		},
		{
			path:             "request peer resume",
			input:            "request peer edge1 resume",
			requiresSelector: true,
			args:             []string{},
			selectors:        map[string]string{"selector": "edge1"},
		},
		{
			// Depth is no limit: three keywords follow the value.
			path:      "request peer plugin session ready",
			input:     "request peer edge1 plugin session ready",
			args:      []string{},
			selectors: map[string]string{"selector": "edge1"},
		},
		{
			// The four commands ze-refresh-cmd.yang merges onto the same
			// container. They declared the selector themselves and now inherit
			// it, so the definition they bind against must be identical.
			path:  "request peer refresh",
			input: "request peer edge1 refresh",
			args:  []string{},
			// applyExtractedSelectors bridges the `selector` leaf onto
			// CommandContext.Peer, which is what every peer-scoped handler
			// reads. The map is the assertion; the bridge is asserted below.
			selectors: map[string]string{"selector": "edge1"},
		},
		{
			path:      "request peer borr",
			input:     "request peer edge1 borr",
			args:      []string{},
			selectors: map[string]string{"selector": "edge1"},
		},
		{
			path:      "request peer eorr",
			input:     "request peer edge1 eorr",
			args:      []string{},
			selectors: map[string]string{"selector": "edge1"},
		},
		{
			path:      "request peer clear soft",
			input:     "request peer edge1 clear soft",
			args:      []string{},
			selectors: map[string]string{"selector": "edge1"},
		},
		{
			path:      "show bgp peer detail",
			input:     "show bgp peer edge1 detail",
			args:      []string{},
			selectors: map[string]string{"selector": "edge1"},
		},
		{
			path:      "show bgp peer capabilities",
			input:     "show bgp peer edge1 capabilities",
			args:      []string{},
			selectors: map[string]string{"selector": "edge1"},
		},
		{
			path:             "show bgp peer history",
			input:            "show bgp peer edge1 history",
			requiresSelector: true,
			args:             []string{},
			selectors:        map[string]string{"selector": "edge1"},
		},
		{
			path:      "show bgp peer statistics",
			input:     "show bgp peer edge1 statistics",
			args:      []string{},
			selectors: map[string]string{"selector": "edge1"},
		},
		{
			path:             "update bgp peer prefix",
			input:            "update bgp peer edge1 prefix",
			requiresSelector: true,
			args:             []string{},
			selectors:        map[string]string{"selector": "edge1"},
		},
		{
			path:  "show metrics name",
			input: "show metrics name ze_bgp_updates_total peer=edge1",
			// The leaf and its container are both called `name`, so the metric
			// name is lifted and only the label filters remain.
			args:      []string{"peer=edge1"},
			selectors: map[string]string{"name": "ze_bgp_updates_total"},
		},
	}

	loader, err := yang.DefaultLoader()
	require.NoError(t, err)
	argDefs := yang.PathToArgDefs(loader)

	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			defs := argDefs[c.path]
			require.NotEmpty(t, defs, "the model declares no argument for %q", c.path)

			var gotArgs []string
			var gotSelectors map[string]string
			handler := func(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
				gotArgs = args
				gotSelectors = ctx.Selectors
				return &plugin.Response{Status: plugin.StatusDone}, nil
			}

			d := pluginserver.NewDispatcher()
			d.RegisterWithOptions(c.path, handler, "under test", pluginserver.RegisterOptions{
				RequiresSelector: c.requiresSelector,
				ArgDefs:          defs,
			})

			ctx := &pluginserver.CommandContext{Peer: c.peer}
			resp, dispatchErr := d.Dispatch(ctx, c.input)
			require.NoError(t, dispatchErr)
			require.NotNil(t, resp)
			assert.Equal(t, "done", resp.Status)
			assert.Equal(t, c.args, gotArgs, "the handler still reads the tail it read before")
			assert.Equal(t, c.selectors, gotSelectors)
		})
	}
}

// TestModifierGroupsLeaveDispatchUntouched dispatches the invocation each
// command accepted BEFORE its tail was modeled as modifier groups.
//
// VALIDATES: R-5 for the four rendering rules. A modifier group's leaves live
// on a CHILD node, so the command node's own argument definitions do not grow
// and validateCommandArgs (command.go) neither consumes nor refuses a token it
// used to pass through.
// PREVENTS: the failure a LEAF would have caused instead. Phase 1 of
// validateCommandArgs demands a value after every declared leaf name it meets,
// so a `withdraw` leaf turns the shipped `debug ip ospf inject opaque ...
// withdraw` into "withdraw requires a value"; and an optional `type` leaf
// leaves unmatchedDefCount above zero, so the first token no definition accepts
// becomes positionalError instead of the handler's business.
func TestModifierGroupsLeaveDispatchUntouched(t *testing.T) {
	cases := []struct {
		path      string
		input     string
		args      []string
		selectors map[string]string
		// ownArgDefs is how many argument definitions the COMMAND node carries.
		// A modifier group adds none, and that is the property the dispatch
		// result above depends on.
		ownArgDefs int
	}{
		{
			path:  "debug ip ospf inject opaque",
			input: "debug ip ospf inject opaque scope link id 5 type 200 hex deadbeef tlv 1 aabb withdraw",
			args: []string{
				"scope", "link", "id", "5", "type", "200",
				"hex", "deadbeef", "tlv", "1", "aabb", "withdraw",
			},
		},
		{
			path:  "debug ipv6 ospf inject lsa",
			input: "debug ipv6 ospf inject lsa scope area type 0x2009 id 3 hex ab withdraw",
			args:  []string{"scope", "area", "type", "0x2009", "id", "3", "hex", "ab", "withdraw"},
		},
		{
			path:       "show policy chain peer",
			input:      "show policy chain peer edge1 export",
			args:       []string{"edge1", "export"},
			ownArgDefs: 1,
		},
		{
			path:       "show bgp peer rib",
			input:      "show bgp peer edge1 rib received",
			args:       []string{"received"},
			selectors:  map[string]string{"selector": "edge1"},
			ownArgDefs: 1,
		},
		{
			path:       "show pki certificate name fingerprint",
			input:      "show pki certificate name device fingerprint sha512",
			args:       []string{"sha512"},
			selectors:  map[string]string{"name": "device"},
			ownArgDefs: 1,
		},
		{
			// The one command whose groups are declared by ANOTHER module. The
			// nineteen match components reach this node through the augment in
			// ze-flowspec-cmd.yang, and the property is the same one: they are
			// groups, so the node's own argument definitions stay at zero and
			// handleAnnounceFlowspecCmd
			// (internal/component/bgp/plugins/cmd/announce/announce.go) reads
			// the whole tail. A component lifted out here would never reach
			// splitFlowspecArgs, which is what cuts the tail at the action.
			path:  "announce flowspec",
			input: "announce flowspec destination 192.0.2.0/24 protocol =6 destination-port =80 rate-limit 9600",
			args: []string{
				"destination", "192.0.2.0/24", "protocol", "=6",
				"destination-port", "=80", "rate-limit", "9600",
			},
		},
	}

	loader, err := yang.DefaultLoader()
	require.NoError(t, err)
	argDefs := yang.PathToArgDefs(loader)

	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			defs := argDefs[c.path]
			assert.Len(t, defs, c.ownArgDefs, "the command node's own argument definitions")

			var gotArgs []string
			var gotSelectors map[string]string
			handler := func(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
				gotArgs = args
				gotSelectors = ctx.Selectors
				return &plugin.Response{Status: plugin.StatusDone}, nil
			}

			d := pluginserver.NewDispatcher()
			d.RegisterWithOptions(c.path, handler, "under test", pluginserver.RegisterOptions{ArgDefs: defs})

			resp, dispatchErr := d.Dispatch(&pluginserver.CommandContext{}, c.input)
			require.NoError(t, dispatchErr)
			require.NotNil(t, resp)
			assert.Equal(t, "done", resp.Status)
			assert.Equal(t, c.args, gotArgs, "the handler still reads the tail it read before")
			assert.Equal(t, c.selectors, gotSelectors)
		})
	}
}

// TestAnnounceFlowspecUsageStatesTheComponents pins the line `announce flowspec
// help` prints, which is the line that replaced the last authored `Usage:`
// sentence in the tree.
//
// VALIDATES: the whole grammar comes from the model. The seventeen components
// are declared by ze-flowspec-cmd.yang and arrive through an augment, the
// action and the two options are declared by ze-cli-announce-cmd.yang, and one
// line carries both modules' words.
// PREVENTS: an augment that loads without contributing anything. A container
// that omits `config false` is dropped by mergeYANGEntry
// (internal/component/config/yang/command.go) with no diagnostic, so the module
// would still parse, the gate would still count zero authored sentences, and
// the operator would read `announce flowspec` with no grammar at all.
//
// The second render comes from a SECOND tree, because commandNode loads and
// builds one for each call. That is what A-3 asks: Children is a map, every
// augmented container carries ModifierOrder 0, and the answer must not depend
// on which order the map was walked in.
func TestAnnounceFlowspecUsageStatesTheComponents(t *testing.T) {
	const path = "announce flowspec"
	const want = "announce flowspec " +
		"[destination-ipv4 <prefix> ...] [destination-ipv6 <prefix> ...] " +
		"[destination-port <value> ...] [dscp <value> ...] [flow-label <value> ...] " +
		"[fragment <value> ...] [icmp-code <value> ...] [icmp-type <value> ...] " +
		"[next-header <value> ...] [packet-length <value> ...] [port <value> ...] " +
		"[protocol <value> ...] [rd <value>] [source-ipv4 <prefix> ...] " +
		"[source-ipv6 <prefix> ...] [source-port <value> ...] [tcp-flags <value> ...] " +
		"(community <value>|rate-limit <bytes-per-second>|discard) " +
		"[tag <key> <value>] [for <duration>]"

	first := command.UsageLine(command.Usage(strings.Fields(path), commandNode(t, path)))
	assert.Equal(t, want, first)

	second := command.UsageLine(command.Usage(strings.Fields(path), commandNode(t, path)))
	assert.Equal(t, first, second, "a second build of the tree renders the same line")
}

// TestDeclaredNumericBoundsHold walks each numeric leaf this spec declared to
// its bound and one past it.
//
// VALIDATES: the range each new leaf states is the range its handler already
// enforced, so declaring the type refuses nothing the handler accepted.
// PREVENTS: a bound copied from the description rather than from the code. The
// VLAN id reads 1-4094 in handleUnitAdd and the MTU 68-65535 in
// handleInterfaceMTU (internal/component/iface/cmd/manage.go); the ping count,
// ping size, traceroute hop count and probe count read theirs from
// internal/component/ping/cmd/resolve.go and
// internal/component/traceroute/cmd/resolve.go.
func TestDeclaredNumericBoundsHold(t *testing.T) {
	cases := []struct {
		path    string
		leaf    string
		lowest  string
		highest string
		below   string
		above   string
	}{
		{path: "create interface dummy name unit", leaf: "vid", lowest: "1", highest: "4094", below: "0", above: "4095"},
		{path: "create interface unit", leaf: "vid", lowest: "1", highest: "4094", below: "0", above: "4095"},
		{path: "delete interface name unit", leaf: "vid", lowest: "1", highest: "4094", below: "0", above: "4095"},
		{path: "request interface mtu", leaf: "bytes", lowest: "68", highest: "65535", below: "67", above: "65536"},
		{path: "resolve ping", leaf: "count", lowest: "1", highest: "100", below: "0", above: "101"},
		{path: "resolve ping", leaf: "size", lowest: "1", highest: "65507", below: "0", above: "65508"},
		{path: "resolve traceroute", leaf: "max-hops", lowest: "1", highest: "64", below: "0", above: "65"},
		{path: "resolve traceroute", leaf: "probes", lowest: "1", highest: "10", below: "0", above: "11"},
	}

	loader, err := yang.DefaultLoader()
	require.NoError(t, err)
	argDefs := yang.PathToArgDefs(loader)

	for _, c := range cases {
		t.Run(c.path+" "+c.leaf, func(t *testing.T) {
			var def *command.ArgDef
			for i, candidate := range argDefs[c.path] {
				if candidate.Name == c.leaf {
					def = &argDefs[c.path][i]
				}
			}
			require.NotNil(t, def, "the model declares no %q leaf on %q", c.leaf, c.path)

			assert.NoError(t, command.ValidateArgString(c.lowest, def), "lowest accepted value")
			assert.NoError(t, command.ValidateArgString(c.highest, def), "highest accepted value")
			assert.Error(t, command.ValidateArgString(c.below, def), "one below the range")
			assert.Error(t, command.ValidateArgString(c.above, def), "one above the range")
		})
	}
}

// TestDeclaredASNLeafTakesTheFullWidth pins the one numeric leaf that states no
// range.
//
// VALIDATES: `asn` accepts every 32-bit AS number and refuses one past it,
// which is exactly what requireASN's ParseUint(s, 10, 32) does
// (internal/component/resolve/cmd/resolve.go).
// PREVENTS: a range narrowed to the private or the 16-bit space, which would
// refuse an AS number the resolver looks up today.
func TestDeclaredASNLeafTakesTheFullWidth(t *testing.T) {
	loader, err := yang.DefaultLoader()
	require.NoError(t, err)
	defs := yang.PathToArgDefs(loader)["resolve cymru asn-name"]
	require.Len(t, defs, 1)

	assert.NoError(t, command.ValidateArgString("0", &defs[0]))
	assert.NoError(t, command.ValidateArgString("4294967295", &defs[0]))
	assert.Error(t, command.ValidateArgString("4294967296", &defs[0]))
	assert.Error(t, command.ValidateArgString("64512.1", &defs[0]))
}

// TestCommandsThatTakeNoInheritedValueKeepTheirBareForm dispatches the two
// commands under a container that names an object, which act on no single
// member of that set.
//
// VALIDATES: ze:inherit "none" keeps the container's leaf out of the command's
// argument definitions, so the bare invocation still reaches its handler.
// PREVENTS: the failure the inheritance causes without it. `selector` is
// mandatory, so Phase 3 of validateCommandArgs (command.go) answers "required
// argument missing: selector" for `show bgp peer list`, which every .ci under
// test/ui runs and which cmd/ze prints as its own usage example.
func TestCommandsThatTakeNoInheritedValueKeepTheirBareForm(t *testing.T) {
	loader, err := yang.DefaultLoader()
	require.NoError(t, err)
	argDefs := yang.PathToArgDefs(loader)

	for _, path := range []string{"show bgp peer list", "request interface migrate"} {
		t.Run(path, func(t *testing.T) {
			assert.Empty(t, argDefs[path], "the container's leaf reached a command that takes none")

			var gotArgs []string
			var gotSelectors map[string]string
			handler := func(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
				gotArgs = args
				gotSelectors = ctx.Selectors
				return &plugin.Response{Status: plugin.StatusDone}, nil
			}

			d := pluginserver.NewDispatcher()
			d.RegisterWithOptions(path, handler, "under test", pluginserver.RegisterOptions{ArgDefs: argDefs[path]})

			resp, dispatchErr := d.Dispatch(&pluginserver.CommandContext{}, path)
			require.NoError(t, dispatchErr)
			require.NotNil(t, resp)
			assert.Equal(t, "done", resp.Status)
			assert.Equal(t, []string{}, gotArgs)
			assert.Nil(t, gotSelectors)
		})
	}
}

// TestInheritedSelectorReachesThePeerBridge dispatches one command whose
// selector now comes from its container and reads the field every peer-scoped
// handler uses.
//
// VALIDATES: applyExtractedSelectors (command.go) still bridges the `selector`
// leaf onto CommandContext.Peer when the leaf is declared by the container
// rather than by the command.
// PREVENTS: an inherited leaf that arrives under a different name or with a
// different case, which would leave ctx.Peer empty and send every peer command
// to every peer.
func TestInheritedSelectorReachesThePeerBridge(t *testing.T) {
	loader, err := yang.DefaultLoader()
	require.NoError(t, err)
	defs := yang.PathToArgDefs(loader)["request peer flush"]
	require.Len(t, defs, 1)
	assert.Equal(t, "peer", defs[0].Anchor, "the selector is anchored to the container that declares it")

	var gotPeer string
	handler := func(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
		gotPeer = ctx.Peer
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}

	d := pluginserver.NewDispatcher()
	d.RegisterWithOptions("request peer flush", handler, "under test", pluginserver.RegisterOptions{
		RequiresSelector: true,
		ArgDefs:          defs,
	})

	_, dispatchErr := d.Dispatch(&pluginserver.CommandContext{}, "request peer edge1 flush")
	require.NoError(t, dispatchErr)
	assert.Equal(t, "edge1", gotPeer)
}
