// Design: docs/architecture/testing/ci-format.md — the AS a ze-peer opens with
// Overview: record_parse.go — parseAndAdd calls declarePeerAS at parse time
// Related: internal/test/peer/open.go — the builder that puts the AS on the wire
//
// A ze-peer that states no AS of its own used to open with ZE's, because its
// OPEN was a copy of ze's. That is right for an iBGP session and wrong for every
// eBGP one, and ze answers the wrong one with NOTIFICATION 2/2 Bad Peer AS
// (validateOpenPeerAS, internal/component/bgp/reactor/session_open_as.go).
//
// The AS ze expects from a peer is already declared once, in the ze
// configuration the same .ci embeds. This file derives the peer block's AS from
// that declaration rather than asking every author to repeat it, so the two can
// never disagree (ai/rules/principles.md).

package runner

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/bgp/asn"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/test/ci"
)

// configPeer is one BGP peer of the ze configuration a .ci embeds.
type configPeer struct {
	name string
	// as is the AS ze is configured to expect from this peer.
	as uint32
	// localAS is the AS ze presents to this peer, and localKnown says whether the
	// configuration declared one. The pair decides whether the session is eBGP,
	// which decides whether a missing derivation is a refusal or a no-op: an iBGP
	// peer is answered correctly by the mirror, an eBGP peer is not.
	localAS    uint32
	localKnown bool
	// dialAddr is `connection { remote { ip } }`, the address ze dials the peer
	// at. ze-peer reads it as the LOCAL address of an accepted connection, and it
	// is the key that identifies the session: ze reaches each of its peers at its
	// own address.
	dialAddr string
	// sourceAddr is `connection { local { ip } }`, the address ze speaks from.
	// ze-peer reads it as the REMOTE address, which is the only key it has when
	// ze-peer is the side that dialed. Several peers can legitimately share one,
	// so it is a second key rather than the first.
	sourceAddr string
}

// errASDerivation marks every refusal this file produces.
//
// The corpus gate asks whether a `.ci` was refused BY THE DERIVATION rather than
// by some other directive, and it used to ask by matching the refusal messages
// as strings. That is a second declaration of every message, and it went wrong
// the first time a message was added: a file refused by the new one was counted
// unreached and the gate passed over it. Wrapping once, at the only exit this
// file has, is the declaration the gate reads instead.
var errASDerivation = errors.New("the peer AS derivation refused this file")

// declaresAS reports whether anything in the inheritance chain gave this peer a
// remote AS.
//
// Zero is unambiguous as "undeclared": asLeaf refuses a configured AS 0, because
// RFC 7607 Section 2 reserves it and ze answers NOTIFICATION 2/2 to a peer that
// opens with it. So no readable configuration produces a peer whose as is 0.
func (p configPeer) declaresAS() bool { return p.as != 0 }

// declarePeerAS writes the AS each ze-peer opens with into its stdin block,
// derived from the BGP peers the same .ci configures.
//
// A block that already carries an option=asn line is left alone: the .ci said
// what it wanted, and several files deliberately open with an AS ze will refuse.
// A .ci whose configuration declares no peer AS at all is left alone too, and
// its peer then mirrors ze's own AS, which is what every file got before an AS
// could be derived.
//
// Three things fail the file instead of being guessed, and each one is a way the
// derivation could otherwise answer wrongly in silence: a declared AS the reader
// cannot read (asLeaf), two peers ze dials at ONE address expecting different
// ASNs (addrKeys), and an eBGP peer the result reaches with nothing
// (coversEveryEBGPPeer).
func declarePeerAS(r *Record) error {
	var undeclared []string
	for _, name := range peerBlockNames(r) {
		block, ok := r.StdinBlocks[name]
		if !ok {
			continue
		}
		if blockDeclaresAS(string(block)) {
			continue
		}
		undeclared = append(undeclared, name)
	}
	if len(undeclared) == 0 {
		return nil
	}

	declaration, peers, err := derivationFor(r)
	if err != nil {
		// Wrapped ONCE, here, because this is the only exit this file has. Every
		// refusal below reaches a caller through it, so the gate that asks "did
		// the derivation refuse this file" needs no list of messages.
		var why textbuf.Buffer
		why.Str(r.CIFile).Str(": ").Str(err.Error())
		return fmt.Errorf("%w: %s", errASDerivation, why.String())
	}
	if len(peers) == 0 {
		return nil
	}

	var prefix textbuf.Buffer
	for _, line := range declaration.lines() {
		prefix.Str(line).Byte('\n')
	}
	written := []byte(prefix.String())

	for _, name := range undeclared {
		r.StdinBlocks[name] = append(slices.Clone(written), r.StdinBlocks[name]...)
	}
	return nil
}

// derivationFor reads the .ci's configuration and answers what every peer block
// with no AS of its own should be given.
func derivationFor(r *Record) (asDeclaration, []configPeer, error) {
	peers, err := configuredPeerAS(r)
	if err != nil {
		return asDeclaration{}, nil, err
	}
	if len(peers) == 0 {
		return asDeclaration{}, nil, nil
	}
	declaration, err := deriveAS(peers)
	if err != nil {
		return asDeclaration{}, nil, err
	}
	if err := declaration.coversEveryEBGPPeer(r.CIFile, peers); err != nil {
		return asDeclaration{}, nil, err
	}
	return declaration, peers, nil
}

// blockDeclaresAS reports whether a peer block states its own AS.
func blockDeclaresAS(block string) bool {
	for line := range strings.SplitSeq(block, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "option=asn:") {
			return true
		}
	}
	return false
}

// asDeclaration is what the derivation decided for one .ci: either one AS that
// answers every session, or one AS for each endpoint address.
//
// The lines a peer block carries and the coverage check are both read off this
// one decision, so a peer the lines do not reach cannot be reported as reached.
type asDeclaration struct {
	single    uint32
	oneAS     bool
	byAddr    map[string]uint32
	addrOrder []string
}

// deriveAS turns the configured peers into the declaration a peer block carries.
//
// One AS across every peer needs no key, and that is 489 of the 535 files with a
// peer block. Several ASNs need one line per endpoint address, because a single
// ze-peer process with option=conn_map serves several of ze's peers at once and
// answers each connection with its own OPEN.
//
// Two peers ze dials at ONE address, expecting two different ASNs, is refused
// rather than guessed: nothing on the wire tells their sessions apart. The
// remedy is one option=asn line in the file, which states the fact where a
// reader can see it.
func deriveAS(peers []configPeer) (asDeclaration, error) {
	declared := make([]configPeer, 0, len(peers))
	for _, peer := range peers {
		if peer.declaresAS() {
			declared = append(declared, peer)
		}
	}
	if len(declared) == 0 {
		// Nothing to derive from. The peer blocks keep the mirror, and the guard
		// below reports any peer that needed better.
		return asDeclaration{}, nil
	}

	single := declared[0].as
	shared := true
	for _, peer := range declared[1:] {
		if peer.as != single {
			shared = false
			break
		}
	}
	if shared {
		return asDeclaration{single: single, oneAS: true}, nil
	}
	peers = declared

	byAddr, err := addrKeys(peers)
	if err != nil {
		return asDeclaration{}, err
	}
	return asDeclaration{byAddr: byAddr, addrOrder: slices.Sorted(maps.Keys(byAddr))}, nil
}

// lines is the option=asn text the declaration puts at the top of a peer block.
func (d asDeclaration) lines() []string {
	if d.oneAS {
		var line textbuf.Buffer
		line.Str("option=asn:value=").Uint32(d.single)
		return []string{line.String()}
	}
	out := make([]string, 0, len(d.addrOrder))
	for _, addr := range d.addrOrder {
		var line textbuf.Buffer
		line.Str("option=asn:peer=").Str(addr).Str(":value=").Uint32(d.byAddr[addr])
		out = append(out, line.String())
	}
	return out
}

// coversEveryEBGPPeer refuses a .ci whose derivation reaches an eBGP peer with
// nothing.
//
// Failing OPEN here is the whole point. An empty result and a correct result are
// the same value to a caller, so a reader that could not find the AS -- because
// the configuration is on disk rather than embedded, because the AS is declared
// on a group this walk did not follow, because the peer names no address to key
// on -- would leave the peer mirroring ZE's AS, which is the defect this file
// exists to close, reintroduced one layer up (ai/rules/principles.md).
//
// An iBGP peer is left alone: the mirror answers it with ze's own AS, which is
// the AS ze expects from it. Only a peer known to be eBGP is refused, so a
// configuration that declares no local AS states no obligation here.
func (d asDeclaration) coversEveryEBGPPeer(ciFile string, peers []configPeer) error {
	for _, peer := range peers {
		// eBGP is a comparison, so it needs both halves. A peer missing either
		// one has no eBGP verdict, and a peer that is not known to be eBGP is not
		// refused: the mirror answers an iBGP session with ze's own AS, which is
		// the AS ze expects from it. Both halves are READ FACTS now rather than
		// reader shrugs, which is what makes this exemption safe to take
		// (readConfig and asLeaf refuse what they cannot read).
		if !peer.declaresAS() || !peer.localKnown || peer.localAS == peer.as {
			continue
		}
		if d.covers(peer) {
			continue
		}
		var why textbuf.Buffer
		why.Str(ciFile).Str(": peer ").Quoted(peer.name).
			Str(" is eBGP (ze is AS ").Uint32(peer.localAS).Str(", the peer is AS ").Uint32(peer.as).
			Str("), and the AS derivation reached it with nothing, so ze-peer would open with ZE's AS and ze would answer ").
			Str("NOTIFICATION 2/2 Bad Peer AS. Write an option=asn:value=").Uint32(peer.as).
			Str(" line in the peer block, or give the peer a connection > remote > ip to key on")
		return errors.New(why.String())
	}
	return nil
}

// covers reports whether the declaration puts this peer's AS on the wire for
// this peer's session.
func (d asDeclaration) covers(peer configPeer) bool {
	if d.oneAS {
		return d.single == peer.as
	}
	for _, addr := range []string{peer.dialAddr, peer.sourceAddr} {
		if addr == "" {
			continue
		}
		if keyed, ok := d.byAddr[addr]; ok && keyed == peer.as {
			return true
		}
	}
	return false
}

// addrKeys maps each endpoint address to the AS of the peer it identifies.
//
// The dial address is the first key and admits no ambiguity. The source address
// is added only where it names one AS and contradicts no dial address, because
// several peers legitimately speak from one address while ze dials each at its
// own. Leaving such an address out costs nothing: the dial address already
// identifies every accepted connection, and the source address is a second route
// to the same answer rather than the only one.
func addrKeys(peers []configPeer) (map[string]uint32, error) {
	byAddr := make(map[string]uint32, len(peers))
	for _, peer := range peers {
		if peer.dialAddr == "" {
			continue
		}
		claimed, seen := byAddr[peer.dialAddr]
		if seen && claimed != peer.as {
			var why textbuf.Buffer
			why.Str("peer ").Quoted(peer.name).Str(" expects AS ").Uint32(peer.as).
				Str(" at ").Str(peer.dialAddr).Str(" and another peer expects AS ").Uint32(claimed).
				Str(" at the same address, so ze-peer cannot tell the two sessions apart. ").
				Str("Write an option=asn:value=<N> line in the peer block to state which AS it opens with")
			return nil, errors.New(why.String())
		}
		byAddr[peer.dialAddr] = peer.as
	}

	contested := make(map[string]bool, len(peers))
	bySource := make(map[string]uint32, len(peers))
	for _, peer := range peers {
		if peer.sourceAddr == "" {
			continue
		}
		if claimed, seen := bySource[peer.sourceAddr]; seen && claimed != peer.as {
			contested[peer.sourceAddr] = true
			continue
		}
		bySource[peer.sourceAddr] = peer.as
	}
	for addr, as := range bySource {
		if contested[addr] {
			continue
		}
		if claimed, seen := byAddr[addr]; seen && claimed != as {
			continue
		}
		byAddr[addr] = as
	}
	return byAddr, nil
}

// configuredPeerAS reads only configuration consumed by the .ci's daemons.
// Selected input is fail-closed: a syntax error is never an empty peer list.
// Arbitrary tmpfs and stdin sidecars are not configuration, even when their
// contents happen to spell valid BGP declarations.
func configuredPeerAS(r *Record) ([]configPeer, error) {
	inputs, err := peerASConfigInputs(r)
	if err != nil {
		return nil, err
	}
	var peers []configPeer
	for _, path := range slices.Sorted(maps.Keys(inputs)) {
		found, err := peersFromConfig(string(inputs[path]))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		peers = append(peers, found...)
	}
	return peers, nil
}

// peerASConfigInputs follows configuration consumers, never filename suffixes
// or the contents of unselected blocks. Reload sources become inputs only when
// their rewrite destination is already a configuration input.
func peerASConfigInputs(r *Record) (map[string][]byte, error) {
	inputs := make(map[string][]byte)
	targets := make(map[string]bool)
	if r.ConfigFile != "" {
		// The legacy runner substitutes an embedded file by basename, or
		// copies the named on-disk input to that basename in the work directory.
		path := filepath.Base(r.ConfigFile)
		if file, ok := r.TmpfsFiles[path]; ok {
			inputs[path] = file.Content
		} else {
			content, err := os.ReadFile(r.ConfigFile) //nolint:gosec // option=file was resolved by parseOption
			if err != nil {
				return nil, fmt.Errorf("the configuration option=file names cannot be read: %w", err)
			}
			inputs[path] = content
		}
		targets[path] = true
	}

	// Use the same execution order and filename allocator as runOrchestrated,
	// without changing the record's runtime allocation state during discovery.
	cmds := make([]*RunCommand, len(r.RunCommands))
	for i := range r.RunCommands {
		cmds[i] = &r.RunCommands[i]
	}
	slices.SortStableFunc(cmds, func(a, b *RunCommand) int { return cmp.Compare(a.Seq, b.Seq) })
	var names Record
	for _, cmd := range cmds {
		if cmd.Exec == "" {
			continue // Stop actions launch no process and consume no configuration.
		}
		parts, err := splitCommand(cmd.Exec)
		if err != nil {
			return nil, fmt.Errorf("cmd seq=%d: %w", cmd.Seq, err)
		}
		args := peerASDaemonArgs(parts)
		index := zeDaemonConfigArgIndex(args)
		if index < 0 {
			continue
		}
		path := filepath.Clean(args[index])
		if path == "-" {
			content, ok := r.StdinBlocks[cmd.Stdin]
			if !ok {
				return nil, fmt.Errorf("daemon configuration stdin block %q not found", cmd.Stdin)
			}
			path = zeConfigFileName(&names, cmd.Stdin)
			inputs[path] = content
		}
		targets[path] = true
	}

	rewrites, err := peerASRewriteSources(r)
	if err != nil {
		return nil, err
	}
	// Each distinct source enters the queue once, so even cyclic rewrite
	// declarations terminate after the finite set of referenced paths.
	queue := slices.Sorted(maps.Keys(targets))
	rootCount := len(queue)
	for i := 0; i < len(queue); i++ {
		for _, source := range rewrites[queue[i]] {
			if targets[source] {
				continue
			}
			targets[source] = true
			queue = append(queue, source)
		}
	}
	var generated []string
	for i, path := range queue {
		if _, present := inputs[path]; present {
			continue
		}
		if file, ok := r.TmpfsFiles[path]; ok {
			inputs[path] = file.Content
			continue
		}
		diskPath := path
		if !filepath.IsAbs(diskPath) {
			diskPath = filepath.Join(filepath.Dir(r.CIFile), path)
		}
		content, err := os.ReadFile(diskPath) //nolint:gosec // a configuration path explicitly consumed by this fixture
		if err != nil {
			// A rewrite may create an intermediate before the next action
			// reads it. Initial daemon inputs and source leaves must exist;
			// other failures (permissions, directories) are never deferred.
			if errors.Is(err, os.ErrNotExist) {
				if i >= rootCount {
					if len(rewrites[path]) > 0 {
						generated = append(generated, path)
						continue
					}
				}
			}
			return nil, fmt.Errorf("configuration %s cannot be read: %w", path, err)
		}
		inputs[path] = content
	}
	if len(generated) == 0 {
		return inputs, nil
	}
	// Prove generated paths have actual bytes upstream. Merely having a
	// writer is insufficient: a cycle of absent sources creates nothing.
	consumers := make(map[string][]string)
	for _, dest := range queue {
		for _, source := range rewrites[dest] {
			consumers[source] = append(consumers[source], dest)
		}
	}
	ready := slices.Sorted(maps.Keys(inputs))
	resolved := make(map[string]bool, len(queue))
	for _, path := range ready {
		resolved[path] = true
	}
	// Each path becomes ready once, bounding the growing queue by the
	// selected input population, including rewrite-created intermediates.
	for i := 0; i < len(ready); i++ {
		for _, dest := range consumers[ready[i]] {
			if resolved[dest] {
				continue
			}
			resolved[dest] = true
			ready = append(ready, dest)
		}
	}
	for _, path := range generated {
		if !resolved[path] {
			return nil, fmt.Errorf("configuration %s has no readable rewrite source", path)
		}
	}
	return inputs, nil
}

// peerASDaemonArgs recognizes the direct and harness-wrapped daemon launches
// whose configuration runOrchestrated routes into the test's work directory.
func peerASDaemonArgs(parts []string) []string {
	if parts[0] == binNameZe {
		return parts[1:]
	}
	if parts[0] != binNameLE {
		return nil
	}
	if !leHarnessArea(parts[1:]) {
		return nil
	}
	for i := 1; i+1 < len(parts); i++ {
		if parts[i] != "--" {
			continue
		}
		if filepath.Base(parts[i+1]) == binNameZe {
			return parts[i+2:]
		}
		return nil
	}
	return nil
}

// peerASRewriteSources reads only executable peer actions, not arbitrary
// sidecar text that happens to contain an action=rewrite line.
func peerASRewriteSources(r *Record) (map[string][]string, error) {
	rewrites := make(map[string][]string)
	add := func(line string) error {
		after, ok := strings.CutPrefix(strings.TrimSpace(line), "action=rewrite:")
		if !ok {
			return nil
		}
		kv := ci.ParseKVPairs(strings.Split(after, ":"))
		if kv["source"] == "" {
			return errActionRewriteMissingSource
		}
		if kv["dest"] == "" {
			return errActionRewriteMissingDest
		}
		dest := filepath.Clean(kv["dest"])
		rewrites[dest] = append(rewrites[dest], filepath.Clean(kv["source"]))
		return nil
	}
	if len(r.RunCommands) == 0 {
		for _, line := range r.Expects {
			if err := add(line); err != nil {
				return nil, err
			}
		}
	}
	for _, name := range peerBlockNames(r) {
		if _, consumed := blockPeerMode(r, name); !consumed {
			continue
		}
		for line := range strings.SplitSeq(string(r.StdinBlocks[name]), "\n") {
			if err := add(line); err != nil {
				return nil, err
			}
		}
	}
	return rewrites, nil
}

// peersFromConfig reads the BGP peers of one configuration text.
//
// It reads the text rather than loading it through the BGP configuration loader,
// because the loader needs the YANG schema, the plugin registry and a resolved
// tree, none of which the test runner has at parse time.
//
// Only the `peer` blocks INSIDE `bgp { ... }` are read. An IPsec or a WireGuard
// peer is a `peer` block too, and walking the whole text made it indistinguishable
// from a BGP peer that declares no AS: both came back with nothing and both were
// skipped. Scoping the walk means every peer this function returns is a BGP peer,
// so a missing AS is an ERROR here rather than a shrug.
func peersFromConfig(text string) ([]configPeer, error) {
	config, err := readConfig(text)
	if err != nil {
		return nil, err
	}
	bgp := config.topLevel("bgp")

	router, err := readDeclarations("the router", bgp)
	if err != nil {
		var why textbuf.Buffer
		why.Str("the router's own session: ").Str(err.Error())
		return nil, errors.New(why.String())
	}
	inherited, err := inheritedByPeer(bgp, router)
	if err != nil {
		return nil, err
	}

	blocks := bgp.blocks("peer")
	peers := make([]configPeer, 0, len(blocks))
	for _, block := range blocks {
		read, readErr := readDeclarations(block.arg, block.body)
		if readErr != nil {
			// Refused, never dropped. A peer the reader could not read would
			// otherwise leave the population the guard walks, and the guard
			// would then report every remaining peer covered.
			var why textbuf.Buffer
			why.Str("peer ").Quoted(block.arg).Str(": ").Str(readErr.Error())
			return nil, errors.New(why.String())
		}
		base := router
		if container, nested := inherited[block.arg]; nested {
			base = container
		}
		read.inherit(base)
		// A peer that declares no remote AS is KEPT, with as == 0 saying so. It
		// is not dropped, because a peer this function drops is a peer the
		// coverage guard never sees, and it is not refused, because a `.ci` may
		// carry a deliberately invalid configuration to prove ze rejects it:
		// test/reload/tx-bgp-rollback.ci writes `peer broken { session { } }`
		// for exactly that. Which of the two it is stops mattering here, because
		// the reader can no longer confuse "the configuration says nothing" with
		// "I could not read it": readConfig refuses a text it cannot tokenize and
		// asLeaf refuses a leaf it cannot parse. So this is a READ FACT, and the
		// guard below decides what it means.
		peers = append(peers, read)
	}
	return peers, nil
}

// readDeclarations reads one block's OWN declarations, with no inheritance
// applied. The same reader serves a peer, a group, a template and the router
// itself, because all four spell the session and the connection the same way.
//
// Every lookup is scoped to the block's own top level, which is what keeps a
// container's defaults separate from those of the peers written inside it.
// `bgp { session { asn { local N } } }` is the router's own AS, and
// test/plugin/bgp-local-as-options.ci states the rule in its own comment: "Every
// peer below inherits it unless the peer names its own". An unscoped read took a
// nested peer's declarations as the container's whenever the container's own
// session was written below that peer, so a sibling peer could inherit another
// peer's AS, another peer's dial address, and with them the eBGP verdict the
// whole guard turns on.
func readDeclarations(name string, block configFile) (configPeer, error) {
	peer := configPeer{name: name}
	asn := block.topLevel("session").topLevel("asn")

	as, declared, err := asLeaf(asn, "remote")
	if err != nil {
		return peer, err
	}
	if declared {
		peer.as = as
	}
	peer.localAS, peer.localKnown, err = asLeaf(asn, "local")
	if err != nil {
		return peer, err
	}

	connection := block.topLevel("connection")
	peer.dialAddr, _ = connection.blockIP("remote")
	peer.sourceAddr, _ = connection.blockIP("local")
	return peer, nil
}

// inherit fills what this peer did not declare from the group or template that
// holds it. A peer's own value always wins, which is what the daemon's own
// inheritance does.
func (p *configPeer) inherit(base configPeer) {
	if p.as == 0 {
		p.as = base.as
	}
	if !p.localKnown {
		p.localAS, p.localKnown = base.localAS, base.localKnown
	}
	if p.dialAddr == "" {
		p.dialAddr = base.dialAddr
	}
	if p.sourceAddr == "" {
		p.sourceAddr = base.sourceAddr
	}
}

// inheritedByPeer maps each peer NAME to what the group or template holding it
// declares.
//
// A peer inside a container inherits the container's session, so a group that
// names the AS gives it to every peer under it and the peers themselves name
// none. Reading only `peer` blocks made those peers invisible, and an eBGP one
// would have opened with ze's AS. Keyed by name because a peer name is a list
// key and unique in a configuration, which costs no byte offsets to track.
func inheritedByPeer(bgp configFile, router configPeer) (map[string]configPeer, error) {
	out := make(map[string]configPeer)
	for _, keyword := range []string{"group", "template"} {
		for _, container := range bgp.blocks(keyword) {
			base, err := readDeclarations(container.arg, container.body)
			if err != nil {
				var why textbuf.Buffer
				why.Str(keyword).Byte(' ').Quoted(container.arg).Str(": ").Str(err.Error())
				return nil, errors.New(why.String())
			}
			base.inherit(router)
			for _, nested := range container.body.blocks("peer") {
				out[nested.arg] = base
			}
		}
	}
	return out, nil
}

// asLeaf reads an AS leaf that may be absent.
//
// It answers three states, and the third is the point. A leaf that is not there
// is ABSENT, which is what an IPsec peer and a peer whose container declares the
// AS both look like. A leaf that IS there and does not read is an ERROR, because
// a value the reader could not read must never come back looking like a value
// nobody wrote: that is the shape the whole derivation exists to remove, and
// answering absent here would exempt the peer from the guard below.
func asLeaf(block configFile, keyword string) (uint32, bool, error) {
	value, declared := block.leaf(keyword)
	if !declared {
		return 0, false, nil
	}
	// asn.Parse reads the asdot spellings beside the decimal one, exactly as
	// the daemon's config parser does. A harness that read fewer spellings
	// than the daemon would refuse a config the daemon accepts.
	parsed, err := asn.Parse(value)
	if err != nil {
		var why textbuf.Buffer
		why.Str("the configured ").Str(keyword).Str(" AS ").Quoted(value).
			Str(" is not an AS number, so nothing can be derived from it")
		return 0, false, errors.New(why.String())
	}
	if parsed == 0 {
		var why textbuf.Buffer
		why.Str("the configured ").Str(keyword).
			Str(" AS is 0, which RFC 7607 Section 2 reserves and ze refuses on the wire")
		return 0, false, errors.New(why.String())
	}
	return parsed, true, nil
}
