package peer

import (
	"encoding/json"
	"net/netip"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// The children of `show bgp` that another in-tree package declares for itself.
const (
	cmdBgpRib = "show bgp rib"
	cmdBgpIRR = "show bgp irr"
)

// childOwnDeclarations names each child of `show bgp` whose own package
// declares for it, and what that package declares.
//
// Every other child resolves NOTHING, which is what the empty declarations in
// registerShapes give it. A child that declares for itself does not contradict
// that: an empty declaration is a floor and never overrides a value
// (declarationRegistry.declare in
// internal/component/command/column_order.go), so the two agree whatever order
// the packages initialize in. Each entry names the package that owns the value.
var childOwnDeclarations = map[string]struct {
	shape         command.AnswerShape
	addressFields []string
	owner         string
}{
	// internal/component/bgp/plugins/cmd/rib/rib.go
	cmdBgpRib: {command.ShapeTab, []string{"peer", "next-hop"}, "the rib command plugin"},
	// health.go, in this package: `show bgp health` answers peer rows, so it
	// declares for itself on the same path this file's loop blanks.
	cmdBgpHealth: {command.ShapeTab, []string{"peer"}, "health.go"},
	// internal/component/bgp/plugins/filter_irr/cmd_irr.go. Its rows hold
	// addresses only inside an array, which neither transform decorates, so it
	// declares a shape and no address field.
	cmdBgpIRR: {command.ShapeTab, nil, "the IRR filter plugin"},
}

// TestDeclaredShapesReachTheRegistry proves the declaration is wired: the
// command declares its shape in its own register path and the core registry
// answers for it, with no core package naming the command.
func TestDeclaredShapesReachTheRegistry(t *testing.T) {
	// `show bgp` carries its peer rows beside the aggregate keys, read against
	// a declared column order.
	if shape, declared := command.ShapeForCommand(cmdBgp); !declared || shape != command.ShapeTab {
		t.Errorf("%s = %v/%v, want tab/declared", cmdBgp, shape, declared)
	}
	if shape, declared := command.ShapeForCommand(cmdBgpPeerList); !declared || shape != command.ShapeTab {
		t.Errorf("%s = %v/%v, want tab/declared", cmdBgpPeerList, shape, declared)
	}

	// A branch under `show bgp` that declares nothing of its own resolves NONE,
	// so it does not inherit `tab` and get published as supporting row
	// operators over an answer with no rows in it.
	//
	// The branches in childOwnDeclarations resolve a shape, and it is not an
	// inheritance: each is declared on that exact path by the package that owns
	// the command. Those packages reach this test binary through
	// internal/component/plugin/all.
	for _, child := range cmdBgpChildren {
		shape, declared := command.ShapeForCommand(child)
		own, declaresItself := childOwnDeclarations[child]
		if declaresItself {
			if !declared || shape != own.shape {
				t.Errorf("%s = %v/%v, want %v/declared from %s", child, shape, declared, own.shape, own.owner)
			}
			continue
		}
		if declared {
			t.Errorf("%s inherits a shape; a child of show bgp that declares nothing must resolve none", child)
		}
	}
}

// TestShowBgpChildrenDeclareNoAddressField proves no path under `show bgp`
// inherits the "address" field that `show bgp` declares. The population is
// cmdBgpChildren itself plus `show bgp peer list`, so a branch added to that
// list is covered without this test being edited.
//
// An address-field list resolves by the longest registered prefix
// (commandRegistry.lookup in internal/component/command/column_order.go), so a
// child that declares nothing reads its parent's list. `| resolve` and
// `| origin` would then be published and accepted over an answer holding no
// address field, and each would decorate nothing.
func TestShowBgpChildrenDeclareNoAddressField(t *testing.T) {
	paths := make([]string, 0, len(cmdBgpChildren)+1)
	paths = append(paths, cmdBgpChildren...)
	paths = append(paths, cmdBgpPeerList)

	for _, path := range paths {
		fields := command.AddressFieldsForCommand(path)
		// A path whose own package names the fields of its answer that hold an
		// address resolves those. The empty declaration this package makes is a
		// floor and never overrides them (declarationRegistry.declare in
		// internal/component/command/column_order.go).
		if own, declaresItself := childOwnDeclarations[path]; declaresItself {
			if strings.Join(fields, ",") != strings.Join(own.addressFields, ",") {
				t.Errorf("%s = %v, want %v from %s", path, fields, own.addressFields, own.owner)
			}
			continue
		}
		if len(fields) > 0 {
			t.Errorf("%s declares address fields %v; it inherits them from show bgp and its answer holds none", path, fields)
		}
	}

	// The parent still declares its own: its peer rows carry the peer address
	// in an "address" field, so blanking the children must not blank it.
	fields := command.AddressFieldsForCommand(cmdBgp)
	if len(fields) != 1 || fields[0] != "address" {
		t.Errorf("%s = %v, want [address]", cmdBgp, fields)
	}
}

// TestPeerListRefusesResolveAndAnswersCount drives the refusal through the
// surface an operator reaches, ProcessPipesDefaultFormatChecked, rather than
// through the registry alone. It is what proves the declaration changes the
// answer an operator gets.
//
// `show bgp peer list` answers rows keyed BY the peer address, carrying
// remote-as, state, uptime, name and group (handleBgpPeerList). No field holds
// an address, so `| resolve` is refused by name, and `| count` is unaffected
// because it acts on rows rather than on an address field.
func TestPeerListRefusesResolveAndAnswersCount(t *testing.T) {
	if _, _, errMsg := command.ProcessPipesDefaultFormatChecked(cmdBgpPeerList+" | resolve", ""); errMsg == "" {
		t.Errorf("%s | resolve was accepted; it decorates nothing and must be refused", cmdBgpPeerList)
	} else if !strings.Contains(errMsg, "resolve") || !strings.Contains(errMsg, "IP address") {
		t.Errorf("refusal = %q, want it to name resolve and the missing IP address field", errMsg)
	}

	_, format, errMsg := command.ProcessPipesDefaultFormatChecked(cmdBgpPeerList+" | count", "")
	if errMsg != "" {
		t.Fatalf("%s | count = %q, want it accepted: the answer holds rows", cmdBgpPeerList, errMsg)
	}

	// The chain being accepted is not the whole of it: `| count` must still
	// answer the number of peers. The fixture is the shape handleBgpPeerList
	// writes, which is rows keyed by peer address under a "peers" envelope.
	//
	// No address in the fixture holds the digit 3, and no row holds the word
	// the state carries, so an answer that passed the payload through rather
	// than counting it fails both checks.
	answer := format(`{"peers":{"10.0.0.1":{"state":"idle"},"10.0.0.4":{"state":"idle"},"10.0.0.7":{"state":"idle"}}}`)
	if !strings.Contains(answer, "3") {
		t.Errorf("%s | count over three peers = %q, want it to answer 3", cmdBgpPeerList, answer)
	}
	if strings.Contains(answer, "idle") {
		t.Errorf("%s | count = %q, want the count alone rather than the rows", cmdBgpPeerList, answer)
	}
}

// showBgpPaths answers every `show bgp` command an IN-TREE package registers,
// derived from the RPC registry and the YANG command tree rather than listed
// here. A path added later is covered without this file being edited.
//
// Two kinds of `show bgp` command are outside the derived set, and neither is
// dropped by accident:
//
//   - The eleven commands under `show bgp rpki`, `show bgp rs`,
//     `show bgp adj-rib-in` and `show bgp healthcheck`. A plugin PROCESS
//     registers those at runtime as plugin names, so no RPCRegistration carries
//     them and no in-core package can declare for them until CommandDecl
//     carries a shape. That is plan/spec-plugin-declares-answer-shape.md. A
//     path that later gains an in-core shim enters this set on its own.
//   - `show bgp decode` and `show bgp encode`. The CLI registers those with
//     registry.MustRegisterLocal and each prints finished text and returns an
//     exit code, so neither reaches a ResponseData or an operator chain
//     (operatorsFor in cmd/ze/help_command.go publishes nothing for them for
//     the same reason).
func showBgpPaths(t *testing.T) []string {
	t.Helper()

	loader, err := yang.DefaultLoader()
	require.NoError(t, err, "the YANG command tree is what maps a wire method to the path an operator types")

	// Every alias, not the shortest one: the registry resolves the STRING the
	// operator typed, so each spelling has to declare for itself.
	wireToPaths := yang.WireMethodToPaths(loader)

	paths := make([]string, 0, 16)
	for _, rpc := range pluginserver.AllBuiltinRPCs() {
		for _, path := range wireToPaths[rpc.WireMethod] {
			if strings.HasPrefix(path, cmdBgp) {
				paths = append(paths, path)
			}
		}
	}
	sort.Strings(paths)
	require.NotEmpty(t, paths, "no show bgp command was derived; the registry or the YANG tree is not loaded")
	return paths
}

// TestEveryShowBgpPathDeclaresAShape holds every in-tree `show bgp` command to
// declaring what its answer holds.
//
// VALIDATES: AC-20.
// PREVENTS: a command reaching no pre-dispatch refusal and publishing nothing.
// validateDeclaredShape returns at `if !declared`
// (internal/component/command/pipe.go), so an undeclared command accepts every
// operator until its answer is in hand, and `ze help command --json` says
// "with-rows" for each one rather than naming what it supports.
//
// The population is DERIVED (showBgpPaths), so a `show bgp` command added later
// fails this test until it declares.
func TestEveryShowBgpPathDeclaresAShape(t *testing.T) {
	for _, path := range showBgpPaths(t) {
		shape, declared := command.ShapeForCommand(path)
		if !declared {
			t.Errorf("%q declares no answer shape", path)
			continue
		}

		// ShapeTab means "rows read against the column names this command
		// declares", so a tab declaration with no column order names nothing
		// for the renderer to read the rows against.
		if shape == command.ShapeTab && len(command.ColumnsForCommand(path)) == 0 {
			t.Errorf("%q declares %s and no column order", path, shape)
		}
	}
}

// TestDeclaredColumnsExistInPayload holds every column name this package
// declares to being a key its handler writes.
//
// VALIDATES: AC-21.
// PREVENTS: the one failure with no signal. A declared name the payload never
// carries orders nothing, and it publishes a field that does not exist to
// `| display <partial>` completion. Nothing fails when it is wrong.
//
// Each case runs the REAL handler over a peer carrying every optional field, so
// the keys compared against are the keys the producing function writes rather
// than a fixture somebody kept in step by hand.
func TestDeclaredColumnsExistInPayload(t *testing.T) {
	peer := fullyPopulatedPeer()

	cases := []struct {
		path    string
		handler peerRowHandler
		record  func(*testing.T, *plugin.Response) map[string]any
	}{
		{cmdBgpPeerList, handleBgpPeerList, keyedRowOf("peers")},
		{cmdBgpPeerDetail, handleBgpPeerDetail, keyedRowOf("peers")},
		{cmdBgpPeerCapabilities, handleBgpPeerCapabilities, arrayRowOf("peers")},
		{cmdBgpPeerStatistics, handleBgpPeerStatistics, arrayRowOf("peers")},
		{cmdBgpPeerHistory, handlePeerHistory, arrayRowOf("transitions")},
		{cmdBgpHealth, handleShowBGPHealth, arrayRowOf("peers")},
	}

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			reactor := &mockReactor{
				peers:    []plugin.PeerInfo{peer},
				peerCaps: &plugin.PeerCapabilitiesInfo{Families: []string{"ipv4/unicast"}, ASN4: true},
				history: map[string][]plugin.FSMTransitionRecord{
					peer.Address.String(): {{
						From: "OpenConfirm", To: "Established",
						Timestamp: time.Now().UTC(), Reason: "keepalive received",
					}},
				},
			}
			ctx := newTestContext(reactor)
			ctx.Peer = peer.Address.String()

			resp, err := tc.handler(ctx, nil)
			require.NoError(t, err)
			require.Equal(t, plugin.StatusDone, resp.Status, "handler answered %q", resp.Error)

			row := tc.record(t, resp)
			orders := command.ColumnsForCommand(tc.path)
			require.NotEmpty(t, orders, "%s declares no column order", tc.path)

			for _, order := range orders {
				for _, name := range order {
					_, written := row[name]
					assert.True(t, written, "%s declares column %q; the handler writes %v", tc.path, name, sortedKeys(row))
				}
			}
		})
	}
}

// TestDeclaredAddressFieldsHoldAnAddress holds every address field this package
// declares to naming a value `| resolve` and `| origin` can act on.
//
// VALIDATES: AC-12, and the naming half of the Critical Review Checklist.
// PREVENTS: publishing `| resolve` over a field it decorates nothing in.
// resolveJSON and originJSON (internal/component/command/pipe_resolve.go,
// pipe_origin.go) decorate a map value that is a string passing
// netip.ParseAddr, and NOTHING else: an array element is walked past, and a
// prefix string fails to parse. A declaration is a gate rather than a selector,
// so a field the transforms cannot reach accepts the operator and answers the
// payload unchanged.
func TestDeclaredAddressFieldsHoldAnAddress(t *testing.T) {
	peer := fullyPopulatedPeer()

	cases := []struct {
		path    string
		handler peerRowHandler
		record  func(*testing.T, *plugin.Response) map[string]any
	}{
		{cmdBgpPeerDetail, handleBgpPeerDetail, keyedRowOf("peers")},
		{cmdBgpPeerCapabilities, handleBgpPeerCapabilities, arrayRowOf("peers")},
		{cmdBgpPeerStatistics, handleBgpPeerStatistics, arrayRowOf("peers")},
		{cmdBgpHealth, handleShowBGPHealth, arrayRowOf("peers")},
	}

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			reactor := &mockReactor{
				peers:    []plugin.PeerInfo{peer},
				peerCaps: &plugin.PeerCapabilitiesInfo{Families: []string{"ipv4/unicast"}, ASN4: true},
			}
			ctx := newTestContext(reactor)
			ctx.Peer = peer.Address.String()

			resp, err := tc.handler(ctx, nil)
			require.NoError(t, err)
			row := tc.record(t, resp)

			fields := command.AddressFieldsForCommand(tc.path)
			require.NotEmpty(t, fields, "%s declares no address field", tc.path)
			for _, name := range fields {
				value, written := row[name]
				require.True(t, written, "%s declares address field %q; the handler writes %v", tc.path, name, sortedKeys(row))
				text, isString := value.(string)
				require.True(t, isString, "%s: %q holds %T, and only a string value is decorated", tc.path, name, value)
				_, err := netip.ParseAddr(text)
				assert.NoError(t, err, "%s: %q holds %q, which netip.ParseAddr refuses, so neither transform decorates it", tc.path, name, text)
			}
		})
	}
}

// TestPeerHistoryRefusesResolve drives AC-7 through the surface an operator
// reaches.
//
// VALIDATES: AC-7, AC-13.
// PREVENTS: a transition row being offered `| resolve`. Its keys are timestamp,
// from, to and reason (handlePeerHistory), none of which holds an address, so
// the operator would decorate nothing in a row. Phase 2 is what makes this a
// refusal rather than an acceptance: without the empty address-field list on
// `show bgp peer`, the shape declared here would publish `| resolve` over the
// "address" field inherited from `show bgp`.
func TestPeerHistoryRefusesResolve(t *testing.T) {
	_, _, errMsg := command.ProcessPipesDefaultFormatChecked(cmdBgpPeerHistory+" | resolve", "")
	require.NotEmpty(t, errMsg, "%s | resolve was accepted; no transition field holds an address", cmdBgpPeerHistory)
	assert.Contains(t, errMsg, "resolve")
	assert.Contains(t, errMsg, "IP address")

	// `| first 1` acts on rows and the transitions ARE rows, so it is accepted
	// and answers the first transition alone.
	_, format, errMsg := command.ProcessPipesDefaultFormatChecked(cmdBgpPeerHistory+" | first 1", "")
	require.Empty(t, errMsg, "%s | first 1 was refused; the answer holds transition rows", cmdBgpPeerHistory)

	answer := format(`{"peer":"10.0.0.1","count":2,"transitions":[` +
		`{"from":"Idle","to":"Connect","reason":"start"},` +
		`{"from":"Connect","to":"Active","reason":"retry"}]}`)
	assert.Contains(t, answer, "start")
	assert.NotContains(t, answer, "retry", "| first 1 kept the second transition")
}

// fullyPopulatedPeer is one peer carrying every optional field the peer
// handlers write, so a declared column name that only a configured peer
// produces is still compared against a payload that has it.
func fullyPopulatedPeer() plugin.PeerInfo {
	return plugin.PeerInfo{
		Address:                 netip.MustParseAddr("192.0.2.1"),
		Name:                    "edge1",
		GroupName:               "transit",
		PeerAS:                  65001,
		LocalAS:                 65000,
		RouterID:                0xC0000201,
		PeerType:                "external",
		State:                   plugin.PeerStateEstablished,
		Uptime:                  5 * time.Minute,
		ReceiveHoldTime:         90 * time.Second,
		SendHoldTime:            90 * time.Second,
		KeepaliveTime:           30 * time.Second,
		ConnectRetry:            5 * time.Second,
		Connect:                 true,
		Accept:                  true,
		LocalAddress:            netip.MustParseAddr("192.0.2.2"),
		LocalPort:               179,
		RemotePort:              54321,
		NextHopMode:             3,
		NextHopAddress:          netip.MustParseAddr("192.0.2.3"),
		UpdatesReceived:         10,
		UpdatesSent:             11,
		KeepalivesReceived:      12,
		KeepalivesSent:          13,
		EORReceived:             1,
		EORSent:                 1,
		OpensReceived:           1,
		OpensSent:               1,
		NotificationsReceived:   0,
		NotificationsSent:       0,
		RefreshReceived:         0,
		RefreshSent:             0,
		ConnectionsEstablished:  2,
		ConnectionsDropped:      1,
		FlapCount:               1,
		ConnectRetryCounter:     3,
		NegotiationComplete:     true,
		NegotiatedASN4:          true,
		NegotiatedRouteRefresh:  true,
		NegotiatedHoldTime:      90 * time.Second,
		NegotiatedKeepaliveTime: 30 * time.Second,
		LastNotifTime:           time.Now().UTC().Add(-time.Hour),
		LastNotifCode:           6,
		LastNotifSubcode:        2,
	}
}

// keyedRowOf answers the first row of an answer whose rows are a map keyed by
// peer address, which is what `show bgp peer list` and `show bgp peer detail`
// answer.
func keyedRowOf(envelope string) func(*testing.T, *plugin.Response) map[string]any {
	return func(t *testing.T, resp *plugin.Response) map[string]any {
		t.Helper()
		rows := decodedEnvelope(t, resp, envelope)
		keyed, isKeyed := rows.(map[string]any)
		require.True(t, isKeyed, "%q holds %T, want a map keyed by peer address", envelope, rows)
		require.NotEmpty(t, keyed)
		for _, row := range keyed {
			record, isRecord := row.(map[string]any)
			require.True(t, isRecord)
			return record
		}
		return nil
	}
}

// arrayRowOf answers the first row of an answer whose rows are an array.
func arrayRowOf(envelope string) func(*testing.T, *plugin.Response) map[string]any {
	return func(t *testing.T, resp *plugin.Response) map[string]any {
		t.Helper()
		rows := decodedEnvelope(t, resp, envelope)
		list, isList := rows.([]any)
		require.True(t, isList, "%q holds %T, want an array of rows", envelope, rows)
		require.NotEmpty(t, list)
		record, isRecord := list[0].(map[string]any)
		require.True(t, isRecord)
		return record
	}
}

// decodedEnvelope takes an answer through the JSON encoding an operator's chain
// reads it after, so a key that only the encoding produces is present and a
// typed value that the encoding changes is compared in its encoded form.
func decodedEnvelope(t *testing.T, resp *plugin.Response, envelope string) any {
	t.Helper()

	encoded, err := json.Marshal(resp.Data)
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	rows, held := decoded[envelope]
	require.True(t, held, "the answer holds no %q: %s", envelope, encoded)
	return rows
}

// sortedKeys names a record's keys for a failure message.
func sortedKeys(row map[string]any) []string {
	names := make([]string, 0, len(row))
	for name := range row {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
