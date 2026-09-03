// Design: docs/architecture/bgp/filter-path-asn.md -- the reject-asn filter plugin
// Detail: register_command.go -- the YANG command nodes and the in-core forwarders
// Detail: curated.go -- the annotation table and the transit-free set
// Related: filter_path_asn.go -- the SDK entry point that installs handleCommand
//
// The three `show bgp reject-asn` answers, built in the plugin process.
//
// Each one is structured data and nothing else, so `| json`, `| yaml` and
// `| table` all render it (ai/rules/cli.md). The shapes and the column
// orders those operators read are declared in register_command.go, in the
// process that holds the command registry.
//
// Two of the three answer what an operator CONFIGURED. The third answers what
// the curated table holds, and it is the only way the transit-free set reaches a
// config: Ze ships no list that decides anything, so an operator pastes the
// block this command prints and the config then holds NUMBERS (AC-55).
package filter_path_asn

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/bgp/configjson"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	statusDone  = "done"
	statusError = "error"

	cmdShowRejectASN            = "show bgp reject-asn"
	cmdShowRejectASNName        = "show bgp reject-asn name"
	cmdShowRejectASNTransitFree = "show bgp reject-asn known transit-free"
)

// attachment counts the peers whose effective filter chain names one list, per
// direction.
//
// A list attached to nothing is configured and inert, which reads exactly like a
// list that is working. The counts are what tell those two apart, so they are
// answered rather than left for the operator to derive from the peer blocks.
type attachment struct {
	importPeers int
	exportPeers int
}

// attachmentsByList holds the counts the last configure delivery produced,
// keyed by list name. It is replaced whole on every delivery.
//
// A nil pointer means no delivery has arrived; the show command then reports
// zero for every list, which is the truth: no peer chain has been read yet.
//
// Safe for concurrent use. The map it points at MUST NOT be written after the
// Store.
var attachmentsByList atomic.Pointer[map[string]attachment]

// countAttachments counts, for each list name a peer chain references, the peers
// that reference it on import and on export.
//
// A chain reference reaches this plugin as the bare list name, as
// reject-asn:NAME or as bgp-filter-path-asn:NAME, and the engine concatenates
// the bgp-level, group-level and peer-level chains into the peer's effective one
// (concatFilters, internal/component/bgp/config/peers.go). So a peer counts when
// the name appears at any of the three levels, and the instance name is read
// after the prefix rather than compared whole.
//
// A dynamic group's template counts as one attachment. Its members do not exist
// in the config document (configjson.ForEachPeer), so the template is the only
// thing there is to count, and reporting zero for a listen-range group would say
// the list is attached to nothing.
func countAttachments(bgpCfg map[string]any) map[string]attachment {
	counts := make(map[string]attachment)

	configjson.ForEachPeer(bgpCfg, func(_ string, peerMap, groupMap map[string]any, _ configjson.PeerOrigin) {
		for name := range chainNames(directionImport, bgpCfg, groupMap, peerMap) {
			entry := counts[name]
			entry.importPeers++
			counts[name] = entry
		}
		for name := range chainNames(directionExport, bgpCfg, groupMap, peerMap) {
			entry := counts[name]
			entry.exportPeers++
			counts[name] = entry
		}
	})
	return counts
}

// chainNames returns the instance names one direction's chain holds, over every
// config level that contributes to it. It is a set, so a name written at two
// levels counts the peer once.
func chainNames(direction string, levels ...map[string]any) map[string]bool {
	names := make(map[string]bool)

	for _, level := range levels {
		filterBlock, ok := level["filter"].(map[string]any)
		if !ok {
			continue
		}
		for _, ref := range refList(filterBlock[direction]) {
			names[instanceName(ref)] = true
		}
	}
	return names
}

// instanceName strips the prefix a chain reference can carry, leaving the name
// that keys a reject-asn list. It is the reading filterInstanceName takes in the
// BGP peer pipeline, over the JSON the plugin boundary delivers.
func instanceName(ref string) string {
	if _, after, found := strings.Cut(ref, ":"); found {
		return after
	}
	return ref
}

// refList normalises a filter chain leaf-list to []string. The delivered value
// is []any after the JSON round trip, and []string or a bare string on the
// in-process path.
func refList(value any) []string {
	switch list := value.(type) {
	case []any:
		refs := make([]string, 0, len(list))
		for _, item := range list {
			if ref, ok := item.(string); ok {
				refs = append(refs, ref)
			}
		}
		return refs
	case []string:
		return list
	case string:
		if list == "" {
			return nil
		}
		return []string{list}
	}
	return nil
}

// handleCommand answers one command RPC. The command string is the full path the
// YANG node declares, so the switch reads like the command tree.
func handleCommand(command string, args []string) (string, any, error) {
	switch command {
	case cmdShowRejectASN:
		return showRejectASN()
	case cmdShowRejectASNName:
		return showRejectASNName(args)
	case cmdShowRejectASNTransitFree:
		return showKnownTransitFree()
	}
	return statusError, nil, fmt.Errorf("unknown command %q", command)
}

// showRejectASN answers every configured list, sorted by name.
func showRejectASN() (string, any, error) {
	held := listsByName.Load()
	if held == nil {
		return statusError, nil, errors.New("no config delivery has reached the reject-asn filter yet")
	}

	b := textbuf.Get()
	defer b.Release()

	b.Str(`{"lists":[`)
	for i, name := range slices.Sorted(maps.Keys(*held)) {
		if i > 0 {
			b.Byte(',')
		}
		appendList(b, (*held)[name])
	}
	b.Str(`]}`)
	return statusDone, json.RawMessage(b.String()), nil
}

// showRejectASNName answers one list by name.
//
// An unknown name is an error rather than an empty answer: an operator who
// mistyped a list name is asking about something that is not there, and an empty
// entry list would read as a list that holds no ASN.
func showRejectASNName(args []string) (string, any, error) {
	if len(args) == 0 || args[0] == "" {
		return statusError, nil, errors.New("usage: show bgp reject-asn name <name>")
	}
	held := listsByName.Load()
	if held == nil {
		return statusError, nil, errors.New("no config delivery has reached the reject-asn filter yet")
	}
	list, ok := (*held)[args[0]]
	if !ok {
		return statusError, nil, fmt.Errorf("no reject-asn list named %q", args[0])
	}

	b := textbuf.Get()
	defer b.Release()

	appendList(b, list)
	return statusDone, json.RawMessage(b.String()), nil
}

// appendList writes one list record.
//
// Every listed ASN appears exactly once, carrying the UNION of the positions its
// blocks named (AC-17, AC-24): the operator wrote blocks, and what the filter
// acts on is the union, so the union is what the answer prints.
//
// The annotation column is written for every ASN, empty for one the curated
// table does not hold (AC-25). Omitting the key would drop the column from the
// rendered table, and guessing a name would put an invention beside an ASN the
// operator has to make a policy decision about.
func appendList(b *textbuf.Buffer, list *rejectList) {
	counts := attachment{}
	if held := attachmentsByList.Load(); held != nil {
		counts = (*held)[list.name]
	}

	b.Str(`{"name":`).Quoted(list.name)
	b.Str(`,"import-peers":`).Int(int64(counts.importPeers))
	b.Str(`,"export-peers":`).Int(int64(counts.exportPeers))

	b.Str(`,"entries":[`)
	for i, asn := range listedASNs(list) {
		if i > 0 {
			b.Byte(',')
		}
		b.Str(`{"asn":`).Uint32(asn)
		b.Str(`,"positions":[`)
		for j, name := range positionNames(list.positions[asn]) {
			if j > 0 {
				b.Byte(',')
			}
			b.Quoted(name)
		}
		b.Str(`],"nth":[`)
		for j, index := range nthIndexes(list, asn) {
			if j > 0 {
				b.Byte(',')
			}
			b.Uint32(uint32(index))
		}
		b.Str(`],"network":`).Quoted(curatedAnnotation(asn)).Byte('}')
	}

	b.Str(`],"patterns":[`)
	for i, pattern := range list.patterns {
		if i > 0 {
			b.Byte(',')
		}
		b.Quoted(pattern.String())
	}
	b.Str(`]}`)
}

// positionNames names the primitive positions a set holds, in path order:
// direct first, origin last. Path order rather than alphabetical, because the
// reader is looking at a place in an AS_PATH.
func positionNames(set positionSet) []string {
	names := make([]string, 0, 3)
	for _, p := range []position{positionDirect, positionTransit, positionOrigin} {
		if set.holds(p) {
			names = append(names, p.String())
		}
	}
	return names
}

// listedASNs names every ASN the list holds, sorted, from BOTH arms that carry
// one. An ASN written only under an `nth` keyword is in the list exactly as much
// as one written under `indirect`, so leaving it out of the answer would show an
// operator a list that is not the list they configured (AC-24).
func listedASNs(list *rejectList) []uint32 {
	asns := make(map[uint32]struct{}, len(list.positions)+len(list.nth))
	for asn := range list.positions {
		asns[asn] = struct{}{}
	}
	for key := range list.nth {
		asns[key.asn] = struct{}{}
	}
	return slices.Sorted(maps.Keys(asns))
}

// nthIndexes names the collapsed positions one ASN is rejected at, sorted. It is
// empty for an ASN no `nth` keyword names, which the answer prints as an empty
// array rather than omitting the key.
func nthIndexes(list *rejectList, asn uint32) []uint8 {
	var indexes []uint8
	for key := range list.nth {
		if key.asn == asn {
			indexes = append(indexes, key.index)
		}
	}
	slices.Sort(indexes)
	return indexes
}

// showKnownTransitFree answers the curated table as a config block an operator
// pastes, and as the same set in structured form.
//
// The block is the point of the command (AC-53). Ze ships no ASN set that
// decides anything, so this is how the well-known transit-free ASNs reach a
// config, and after the paste the config holds numbers that no later edit to the
// table can change (AC-55).
//
// "block" is an array of lines rather than one string with newlines in it, so
// every renderer prints it as lines an operator can select. The comments carry
// the provenance the table cannot otherwise show: no authority publishes this
// set, so the curated date is the only staleness signal there is.
func showKnownTransitFree() (string, any, error) {
	b := textbuf.Get()
	defer b.Release()

	b.Str(`{"curated":`).Quoted(curatedDate)

	b.Str(`,"sources":[`)
	for i, source := range curatedSources {
		if i > 0 {
			b.Byte(',')
		}
		b.Quoted(source)
	}

	b.Str(`],"networks":[`)
	for i, network := range curatedTransitFree {
		if i > 0 {
			b.Byte(',')
		}
		b.Str(`{"asn":`).Uint32(network.asn)
		b.Str(`,"name":`).Quoted(network.name)
		b.Str(`,"contested":`).Bool(network.contested).Byte('}')
	}

	b.Str(`],"block":[`)
	for i, line := range transitFreeBlock() {
		if i > 0 {
			b.Byte(',')
		}
		b.Quoted(line)
	}
	b.Str(`]}`)

	return statusDone, json.RawMessage(b.String()), nil
}

// transitFreeBlock renders the pasteable config block: the provenance as `#`
// comments, then one `indirect [ ... ];` leaf-list line.
//
// The keyword is `indirect` rather than any of the other six, because RFC 7454
// Section 9 asks for the ASNs that must not appear anywhere except as the peer
// itself, which is exactly what `indirect` covers. An operator who wants another
// position changes the one word.
//
// One line for the ASNs rather than one per ASN, because the operator pastes it
// straight under `reject-asn NAME { ... }` and a single line is what a leaf-list
// looks like everywhere else in a ze config.
// TestKnownTransitFreePrintsPasteableBlock feeds the whole block back through the
// config parser, so a format that stops being pasteable is a red test rather than
// an operator's problem.
func transitFreeBlock() []string {
	lines := make([]string, 0, len(curatedSources)+len(curatedTransitFree)+2)

	var head textbuf.Buffer
	lines = append(lines, head.Str("# transit-free ASNs, curated ").Str(curatedDate).String())

	for _, source := range curatedSources {
		var tb textbuf.Buffer
		lines = append(lines, tb.Str("# source: ").Str(source).String())
	}

	// A contested entry says so before the operator pastes it. The dispute is
	// what the curated table records instead of resolving, so hiding it here
	// would put the editorial judgement back (curated.go).
	for _, network := range curatedTransitFree {
		if !network.contested {
			continue
		}
		var note textbuf.Buffer
		lines = append(lines, note.Str("# AS").Uint32(network.asn).
			Str(" is contested: ").Str(network.note).String())
	}

	var block textbuf.Buffer
	block.Str(positionKeyPasteable + " [")
	for _, network := range curatedTransitFree {
		block.Byte(' ').Uint32(network.asn)
	}
	block.Str(" ];")

	return append(lines, block.String())
}
