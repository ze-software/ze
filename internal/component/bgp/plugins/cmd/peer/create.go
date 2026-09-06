// Design: docs/architecture/api/commands.md — runtime BGP peer creation
// RFC: rfc/short/rfc4724.md — the graceful restart time this speaker advertises
// RFC: rfc/short/rfc6286.md — the router id leaf is the BGP Identifier
// RFC: rfc/short/rfc6793.md — the AS number leaves are four octets wide
// RFC: rfc/short/rfc7607.md — AS 0 is refused
// Overview: peer.go — the BGP peer lifecycle handlers and their registration
// Related: internal/component/bgp/reactor/reactor_peers.go — AddDynamicPeer, which this handler calls

package peer

import (
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// cmdBgpPeerCreate is the path an operator types to reach handleBgpPeerAdd.
const cmdBgpPeerCreate = "create bgp peer"

var (
	errCreateAddress = errors.New("create bgp peer takes one peer address")
	errCreateKeyword = errors.New("create bgp peer does not take this keyword")
	errCreateValue   = errors.New("this keyword takes a value")
)

// handleBgpPeerAdd handles `create bgp peer <address> asn <asn> ...`.
//
// It is the runtime counterpart of `delete bgp peer <selector>`
// (handleBgpPeerRemove, peer.go), and it mirrors that command's reach: the peer
// is built in the reactor and started there, and the configuration is not
// touched. So a created peer does not appear in `show config` and a reload
// removes it, exactly as a removed peer comes back on a reload.
//
// Every keyword the command accepts is a leaf of the same name in
// ze-peer-cmd.yang, so the dispatcher types each value before the handler sees
// it. The handler types them AGAIN, because a plugin reaching `ze-bgp:peer-add`
// over the IPC transport sends arguments the CLI never checked, and because a
// keyword this handler cannot honor must be refused BY NAME rather than dropped
// (ai/rules/principles.md).
func handleBgpPeerAdd(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	_, errResp, err := pluginserver.RequireReactor(ctx)
	if err != nil {
		return errResp, err
	}

	// The address is the selector the dispatcher bound from the token after
	// `peer`. It is parsed rather than resolved: the peer does not exist yet, so
	// a name, a glob, an AS pattern and the wildcard each name nothing here, and
	// ResolveSinglePeer would answer "no peer matches selector" for the one
	// address this command is about to create.
	selector := ctx.PeerSelector()
	addr, err := netip.ParseAddr(selector)
	if err != nil {
		var tb textbuf.Buffer
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  tb.Str("create: ").Quoted(selector).Str(" is not a peer address").String(),
		}, fmt.Errorf("%w, got %q: %w", errCreateAddress, selector, err)
	}

	tree, err := peerCreateTree(selector, args)
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, err
	}

	if err := ctx.Reactor().AddDynamicPeer(addr, tree); err != nil {
		var tb textbuf.Buffer
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  tb.Str("create peer failed: ").Str(err.Error()).String(),
		}, fmt.Errorf("create peer %s: %w", addr, err)
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			fieldPeer:     addr.String(),
			fieldRemoteAS: remoteASOf(tree),
			fieldMessage:  "peer created",
		},
	}, nil
}

// peerCreateTree turns the command's keyword-value tail into the peer config
// tree parsePeerFromTree reads.
//
// selector is the address token the dispatcher bound. It arrives again as the
// first argument, because the dispatcher hands the handler every token after
// the command path, so it is skipped here rather than refused.
func peerCreateTree(selector string, args []string) (map[string]any, error) {
	tree := make(map[string]any)

	for i := 0; i < len(args); i++ {
		keyword := args[i]
		if i == 0 && keyword == selector {
			continue
		}

		apply, known := peerCreateKeywords[keyword]
		if !known {
			var tb textbuf.Buffer
			return nil, fmt.Errorf("%w: %q. It takes %s",
				errCreateKeyword, keyword, tb.Join(peerCreateKeywordNames(), ", ").String())
		}
		if i+1 >= len(args) {
			return nil, fmt.Errorf("%q: %w", keyword, errCreateValue)
		}

		i++
		if err := apply(tree, args[i]); err != nil {
			return nil, fmt.Errorf("%s: %w", keyword, err)
		}
	}

	if remoteASOf(tree) == "" {
		return nil, fmt.Errorf("%q: %w", "asn", errCreateValue)
	}
	return tree, nil
}

// remoteASOf reads back the remote AS the tree states. It answers the empty
// string when the command stated none, which is the one keyword that has no
// default and no answer without it.
func remoteASOf(tree map[string]any) string {
	remote, _ := treeReach(tree, "session", "asn")["remote"].(string)
	return remote
}

// peerCreateKeywords maps each keyword `create bgp peer` accepts to the leaf it
// writes in the peer config tree. A table rather than a switch, so the refusal
// above can name the whole set from the code that defines it
// (ai/rules/evidence.md).
//
// Each key is a leaf of the same name under `create bgp peer` in
// ze-peer-cmd.yang, and each path is the one parsePeerFromTree reads
// (internal/component/bgp/reactor/config.go).
var peerCreateKeywords = map[string]func(tree map[string]any, value string) error{
	"asn":               func(t map[string]any, v string) error { return setASN(t, "remote", v) },
	"local-as":          func(t map[string]any, v string) error { return setASN(t, "local", v) },
	"local-address":     setLocalAddress,
	"router-id":         setRouterID,
	"receive-hold-time": func(t map[string]any, v string) error { return setSeconds(t, "receive-hold-time", v) },
	"send-hold-time":    func(t map[string]any, v string) error { return setSeconds(t, "send-hold-time", v) },
	"connect-retry":     func(t map[string]any, v string) error { return setSeconds(t, "connect-retry", v) },
	"connect":           setConnect,
	"accept":            setAccept,
	"family":            setFamilies,
	"graceful-restart":  setGracefulRestart,
	"group-updates":     setGroupUpdates,
	"attach":            setAttach,
}

// peerCreateKeywordNames answers every keyword the command takes, sorted, so a
// refusal states the set the operator can choose from.
func peerCreateKeywordNames() []string {
	names := make([]string, 0, len(peerCreateKeywords))
	for name := range peerCreateKeywords {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func setASN(tree map[string]any, leaf, value string) error {
	// RFC 6793 Section 2: "BGP speakers that support four-octet Autonomous
	// System numbers". AS 0 names no speaker: RFC 7607 Section 1 states "AS 0
	// is reserved and MUST NOT be used", so it is refused here rather than
	// carried into an OPEN.
	asn, err := strconv.ParseUint(value, 10, 32)
	if err != nil || asn == 0 {
		return fmt.Errorf("an AS number is 1 to 4294967295, got %q", value)
	}
	treeReach(tree, "session", "asn")[leaf] = value
	return nil
}

func setLocalAddress(tree map[string]any, value string) error {
	if _, err := netip.ParseAddr(value); err != nil {
		return fmt.Errorf("not an address: %q", value)
	}
	treeReach(tree, "connection", "local")["ip"] = value
	return nil
}

func setRouterID(tree map[string]any, value string) error {
	// RFC 6286 Section 2.1: "the BGP Identifier is a 4-octet, unsigned,
	// non-zero integer". Ze writes it as a dotted quad on every surface, and
	// parseRouterID refuses a zero one, so the shape is what this checks.
	id, err := netip.ParseAddr(value)
	if err != nil || !id.Is4() {
		return fmt.Errorf("a router id is a dotted quad, got %q", value)
	}
	treeReach(tree, "session")["router-id"] = value
	return nil
}

func setSeconds(tree map[string]any, leaf, value string) error {
	if _, err := strconv.ParseUint(value, 10, 16); err != nil {
		return fmt.Errorf("a time in seconds is 0 to 65535, got %q", value)
	}
	treeReach(tree, "timer")[leaf] = value
	return nil
}

func setConnect(tree map[string]any, value string) error {
	if err := checkBoolean(value); err != nil {
		return err
	}
	treeReach(tree, "connection", "remote")["connect"] = value
	return nil
}

func setAccept(tree map[string]any, value string) error {
	if err := checkBoolean(value); err != nil {
		return err
	}
	treeReach(tree, "connection", "local")["accept"] = value
	return nil
}

func setGroupUpdates(tree map[string]any, value string) error {
	if err := checkBoolean(value); err != nil {
		return err
	}
	treeReach(tree, "behavior")["group-updates"] = value
	return nil
}

// setFamilies enables each named family in the session's OPEN.
//
// The value is comma separated because the dispatcher binds exactly ONE token
// to a keyword, so a space-separated list would reach the handler as a run of
// tokens that name no keyword and be refused.
func setFamilies(tree map[string]any, value string) error {
	families := treeReach(tree, "session", "family")
	for name := range strings.SplitSeq(value, ",") {
		if _, known := family.LookupFamily(name); !known {
			return fmt.Errorf("no such address family: %q", name)
		}
		families[name] = "enable"
	}
	return nil
}

// setGracefulRestart states the Restart Time this speaker advertises.
func setGracefulRestart(tree map[string]any, value string) error {
	// RFC 4724 Section 3: "Restart Time: This is the estimated time (in
	// seconds) it will take for the BGP session to be re-established after a
	// restart. This can be used to speed up routing convergence by its peer in
	// case that the BGP speaker does not come back after a restart." The field
	// is 12 bits wide in the same section's figure, so 4095 is its maximum.
	seconds, err := strconv.ParseUint(value, 10, 16)
	if err != nil || seconds > 4095 {
		return fmt.Errorf("a graceful restart time is 0 to 4095 seconds, got %q", value)
	}
	treeReach(tree, "session", "capability", "graceful-restart")["restart-time"] = value
	return nil
}

// setAttach binds the peer to each named plugin process.
//
// The binding grants the WHOLE surface. The process receives every message from
// this peer, and it can send every type toward it. A binding is the peer's
// receive AUTHORIZATION and its send permission both (Server.PeerScopedProcs,
// internal/component/plugin/server/delivery_graph.go). So a binding with no
// grant attaches the process and permits it nothing. That is an attach that
// looks done and does nothing (ai/rules/principles.md).
//
// The whole surface is also what the ExaBGP construct this carries means: a
// neighbor's `api <process>` gives that process the neighbor's events and lets
// it announce to the neighbor. `ze exabgp migrate` writes the same grant for
// the bridge (addProcessBinding, internal/exabgp/migration/migrate.go). A
// narrower grant is expressed in the configuration file, where the receive and
// send lists are written out.
func setAttach(tree map[string]any, value string) error {
	processes := treeReach(tree, "attach", "process")
	for name := range strings.SplitSeq(value, ",") {
		if name == "" {
			return fmt.Errorf("a process name is not empty: %q", value)
		}
		processes[name] = map[string]any{"receive": "*", "send": "*"}
	}
	return nil
}

func checkBoolean(value string) error {
	if value == "true" || value == "false" {
		return nil
	}
	return fmt.Errorf("true or false, got %q", value)
}

// treeReach answers the container at the named path, creating each level the
// tree does not hold yet.
func treeReach(tree map[string]any, path ...string) map[string]any {
	node := tree
	for _, key := range path {
		child, ok := node[key].(map[string]any)
		if !ok {
			child = make(map[string]any)
			node[key] = child
		}
		node = child
	}
	return node
}
