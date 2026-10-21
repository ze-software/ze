//go:build ze_bgp

// Design: docs/architecture/exabgp-bridge.md -- the commands a 4.2 or 5.0 script writes
// Related: old_allowed_test.go -- the differences a 4.2 or 5.0 script sees on upgrade
//
// X-35, AC-37. ExaBGP 6.0 recorded every API command 4.2 and 5.0 accept, with
// the UPDATEs each release packed for it (qa/old/commands-<release>.json at the
// pin, carried here as testdata/exabgp-old/commands-<release>.json.gz). Each
// command is given to the bridge as the script wrote it (Translator.Line), and
// each ze command the bridge writes is parsed by the `update text` handler's
// own parser (update.ParseUpdateText). Both sides are then decoded by ONE
// decoder, the bridge's WireUpdateToExabgpJSON, and compared route by route:
// the routes announced with their attributes, and the routes withdrawn.
//
// The pin's own judge works the same way (qa/bin/test_old_commands): it runs
// the handler on a stand-in reactor, packs what the handler hands to the RIB,
// and decodes both sides with the tree's decoder. The reactor's part is the
// iBGP defaults it adds to an API route (planBatchAttrs, reactor_api_batch.go:
// ORIGIN IGP, an empty AS_PATH, LOCAL_PREF 100) and `next-hop self`, which it
// resolves to the session's local address, 127.0.0.1 in neighbor.conf.

package bridge

import (
	"compress/gzip"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/plugins/cmd/update"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/evpn"     // blank import: the daemon links every NLRI plugin
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/flowspec" // blank import: as above
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/labeled"  // blank import: as above
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/ls"       // blank import: as above
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/mup"      // blank import: as above
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/mvpn"     // blank import: as above
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/rtc"      // blank import: as above
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/srpolicy" // blank import: as above
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/vpls"     // blank import: as above
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/vpn"      // blank import: as above
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// oldRecorded is one entry of commands-<release>.json: what the release made
// of one command line (qa/old/recorder.py at the pin).
type oldRecorded struct {
	Accepted bool     `json:"accepted"`
	Command  string   `json:"command"`
	Messages []string `json:"messages"`
}

// oldVerdict names why a command was not compared, or that it differs.
type oldVerdict uint8

const (
	// The zero value names no verdict; judge never answers it.
	_ oldVerdict = iota
	// oldSame means ze announces and withdraws what the release did.
	oldSame
	// oldUnreadable: the release sent nothing a peer can decode, so refusing
	// it or sending something else loses nobody a route (test_old_commands,
	// UNREADABLE).
	oldUnreadable
	// oldNothingSent: an announce or withdraw the release took and sent no
	// route for (test_old_commands, NOTHING_SENT).
	oldNothingSent
	// oldDiffers: ze refuses the command or sends other routes.
	oldDiffers
)

// oldLocalAddress is the local-address of testdata/exabgp-old/neighbor.conf,
// which `next-hop self` resolves to.
var oldLocalAddress = netip.MustParseAddr("127.0.0.1")

func TestOldCommands50(t *testing.T) { runOldCommands(t, "5.0") }

func TestOldCommands42(t *testing.T) { runOldCommands(t, "4.2") }

// runOldCommands judges every recorded command of one release.
//
// VALIDATES: AC-37, every command the release accepted announces and withdraws
// through the bridge the routes the release did, decoded, or is an ALLOWED row
// carrying its reason (old_allowed_test.go).
// PREVENTS: a 4.2 or 5.0 script losing or changing a route on its move to ze.
func runOldCommands(t *testing.T, release string) {
	t.Helper()
	recorded := loadOldCommands(t, release)
	judge := newOldJudge(t)

	commands := make([]string, 0, len(recorded))
	for command := range recorded {
		commands = append(commands, command)
	}
	slices.Sort(commands)

	counts := map[oldVerdict]int{}
	allowedCount := 0
	var failures []string
	for _, command := range commands {
		if oldCommandAllowed(release, command) {
			allowedCount++
			continue
		}
		verdict, why := judge.judge(command, recorded[command])
		counts[verdict]++
		if verdict == oldDiffers {
			failures = append(failures, command+"\n    "+why)
		}
	}
	t.Logf("%s: %d/%d commands behave as on %s (%d allowed, %d unreadable, %d nothing sent)",
		release, len(commands)-len(failures), len(commands), release,
		allowedCount, counts[oldUnreadable], counts[oldNothingSent])
	for i, failure := range failures {
		if i == 40 && os.Getenv("ZE_OLD_VERBOSE") == "" {
			t.Logf("... %d more", len(failures)-i)
			break
		}
		t.Errorf("%s: %s", release, failure)
	}
}

func loadOldCommands(t *testing.T, release string) map[string]oldRecorded {
	t.Helper()
	file, err := os.Open(filepath.Join("testdata", "exabgp-old", "commands-"+release+".json.gz"))
	if err != nil {
		t.Fatalf("open the %s commands corpus: %v", release, err)
	}
	defer file.Close() //nolint:errcheck // read-only file
	reader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatalf("gunzip the %s commands corpus: %v", release, err)
	}
	recorded := map[string]oldRecorded{}
	if err := json.NewDecoder(reader).Decode(&recorded); err != nil {
		t.Fatalf("decode the %s commands corpus: %v", release, err)
	}
	if len(recorded) == 0 {
		t.Fatalf("the %s commands corpus holds no command", release)
	}
	return recorded
}

// oldJudge holds the two ways an UPDATE is read: without ADD-PATH, then with
// it on every family. 5.0 packs a Path Identifier for a route given
// `path-information` whether or not the session negotiated it, and the pin's
// judge reads each message both ways for that reason (content, test_old_commands).
type oldJudge struct {
	plain   SessionFacts
	addPath SessionFacts
}

func newOldJudge(t *testing.T) *oldJudge {
	t.Helper()
	plain := SessionFacts{
		Local: oldLocalAddress, Peer: oldLocalAddress,
		LocalAS: 65533, PeerAS: 65533, RouterID: 0x0a000002,
	}
	caps := &capability.EncodingCaps{ASN4: true, AddPathMode: map[capability.Family]capability.AddPathMode{}}
	for _, name := range family.RegisteredFamilyNames() {
		if f, ok := family.LookupFamily(name); ok {
			caps.Families = append(caps.Families, f)
			caps.AddPathMode[f] = capability.AddPathBoth
		}
	}
	withPaths := plain
	withPaths.Encoding = caps
	return &oldJudge{plain: plain, addPath: withPaths}
}

func (j *oldJudge) judge(command string, recorded oldRecorded) (oldVerdict, string) {
	theirs, undecodable := j.recordedRoutes(recorded.Messages)
	if len(theirs) > 0 && undecodable == len(recorded.Messages) {
		return oldUnreadable, ""
	}
	routeVerb := strings.HasPrefix(recorded.Command, "announce") || strings.HasPrefix(recorded.Command, "withdraw")
	nothing := len(theirs) == 0 && routeVerb

	ours, refusal := j.zeRoutes(command)
	if refusal != "" {
		if nothing {
			return oldNothingSent, ""
		}
		return oldDiffers, "refused: " + refusal
	}
	if slices.Equal(ours, theirs) {
		return oldSame, ""
	}
	if nothing {
		return oldNothingSent, ""
	}
	return oldDiffers, "release sent " + strings.Join(firstRows(theirs), " | ") +
		"\n    ze sends " + strings.Join(firstRows(ours), " | ")
}

func firstRows(rows []string) []string {
	if len(rows) > 3 {
		return rows[:3]
	}
	return rows
}

// recordedRoutes decodes the release's messages. A message ze cannot read
// either way is kept as `undecodable:<hex>`, as the pin keeps it, and counted.
func (j *oldJudge) recordedRoutes(messages []string) ([]string, int) {
	var rows []string
	undecodable := 0
	for _, text := range messages {
		raw, err := hex.DecodeString(text)
		if err != nil || len(raw) < 19 {
			rows = append(rows, "undecodable:"+text)
			undecodable++
			continue
		}
		if raw[18] != 2 {
			rows = append(rows, string(rune('0'+raw[18]))+":"+hex.EncodeToString(raw[19:]))
			continue
		}
		decoded, ok := j.decode(raw[19:])
		if !ok {
			rows = append(rows, "undecodable:"+hex.EncodeToString(raw[19:]))
			undecodable++
			continue
		}
		rows = append(rows, decoded...)
	}
	slices.Sort(rows)
	return rows, undecodable
}

// decode reads one UPDATE body without ADD-PATH, then with it.
func (j *oldJudge) decode(body []byte) ([]string, bool) {
	for _, facts := range []SessionFacts{j.plain, j.addPath} {
		rows, ok := decodeOldUpdate(body, facts)
		if ok {
			return rows, true
		}
	}
	return nil, false
}

// decodeOldUpdate is one message as route rows: test_old_commands `routes`,
// one row per route, `[action, family, nexthop, nlri, attributes]`, the
// attributes shown on an announce only.
func decodeOldUpdate(body []byte, facts SessionFacts) (rows []string, ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	rendered, err := WireUpdateToExabgpJSON(body, facts, rpc.DirectionReceived)
	if err != nil {
		return nil, false
	}
	// The renderer nests its own map types, so the document is read back as
	// plain JSON before it is walked.
	encoded, err := json.Marshal(rendered)
	if err != nil {
		return nil, false
	}
	var document any
	if err := json.Unmarshal(encoded, &document); err != nil {
		return nil, false
	}
	message := findMember(document, "update")
	if message == nil {
		if eor := findMember(document, "eor"); eor != nil {
			return []string{canonicalJSON(map[string]any{"eor": eor})}, true
		}
		return nil, false
	}
	update, isMap := message.(map[string]any)
	if !isMap {
		return nil, false
	}
	attributes, _ := update["attribute"].(map[string]any) //nolint:errcheck // absent on a withdrawal
	if attributes == nil {
		attributes = map[string]any{}
	}
	for _, action := range []string{"announce", "withdraw"} {
		families, _ := update[action].(map[string]any) //nolint:errcheck // absent when the action is
		for name, byNextHop := range families {
			for nexthop, nlris := range nextHopGroups(byNextHop) {
				for _, nlri := range nlris {
					shown := map[string]any{}
					if action == "announce" {
						shown = attributes
					}
					rows = append(rows, oldRow(action, name, nexthop, nlri, shown))
				}
			}
		}
	}
	if eor := findMember(update, "eor"); eor != nil {
		rows = append(rows, canonicalJSON(map[string]any{"eor": eor}))
	}
	return rows, len(rows) > 0
}

// nextHopGroups reads an announce family (keyed by next hop) or a withdraw
// family (a list), as test_old_commands `routes` does.
func nextHopGroups(value any) map[string][]any {
	switch typed := value.(type) {
	case map[string]any:
		groups := make(map[string][]any, len(typed))
		for nexthop, nlris := range typed {
			list, _ := nlris.([]any) //nolint:errcheck // a non-list member carries no route
			groups[nexthop] = list
		}
		return groups
	case []any:
		return map[string][]any{"": typed}
	}
	return nil
}

// findMember is the first member called name, searched breadth first.
func findMember(value any, name string) any {
	queue := []any{value}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		object, isMap := current.(map[string]any)
		if !isMap {
			continue
		}
		if found, present := object[name]; present {
			return found
		}
		for _, member := range object {
			queue = append(queue, member)
		}
	}
	return nil
}

// canonicalJSON orders every list as test_old_commands `_canonical` does, so
// two decoders listing one set in two orders compare equal. Go's encoder
// already sorts object keys.
func canonicalJSON(value any) string {
	encoded, err := json.Marshal(canonicalValue(value))
	if err != nil {
		return "unencodable"
	}
	return string(encoded)
}

func canonicalValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		ordered := make(map[string]any, len(typed))
		for key, member := range typed {
			ordered[key] = canonicalValue(member)
		}
		return ordered
	case []any:
		items := make([]any, len(typed))
		keys := make([]string, len(typed))
		for i, member := range typed {
			items[i] = canonicalValue(member)
			keys[i] = canonicalJSON(items[i])
		}
		order := make([]int, len(items))
		for i := range order {
			order[i] = i
		}
		sort.SliceStable(order, func(a, b int) bool { return keys[order[a]] < keys[order[b]] })
		sorted := make([]any, len(items))
		for i, index := range order {
			sorted[i] = items[index]
		}
		return sorted
	}
	return value
}

// zeRoutes is what ze makes of the command: the bridge's translation, parsed
// by the update text parser, packed, and decoded. The refusal is why ze sends
// nothing, empty when it sends something.
func (j *oldJudge) zeRoutes(command string) ([]string, string) {
	translation, err := Translator{}.Line(command)
	if err != nil {
		return nil, err.Error()
	}
	if len(translation.Commands) == 0 {
		return nil, "the bridge wrote no command"
	}
	var rows []string
	for _, written := range translation.Commands {
		_, args, found := strings.Cut(written.Text, " update text")
		if !found {
			return nil, "not an update text command: " + written.Text
		}
		result, err := update.ParseUpdateText(strings.Fields(args))
		if err != nil {
			return nil, "update text: " + err.Error()
		}
		for _, body := range packOldResult(result) {
			decoded, ok := j.decode(body)
			if !ok {
				return nil, "ze packed an UPDATE its decoder cannot read: " + hex.EncodeToString(body)
			}
			rows = append(rows, decoded...)
		}
	}
	slices.Sort(rows)
	return rows, ""
}

// packOldResult packs one parsed command as the UPDATE bodies a peer would
// read: one announce and one withdrawal per group, and one End-of-RIB per
// family. Grouping does not matter: the comparison is one row per route.
func packOldResult(result *bgptypes.UpdateTextResult) [][]byte {
	var bodies [][]byte
	for i := range result.Groups {
		group := &result.Groups[i]
		if len(group.Announce) > 0 {
			bodies = append(bodies, packOldAnnounce(group))
		}
		if len(group.Withdraw) > 0 {
			bodies = append(bodies, packOldWithdraw(group.Family, oldNLRIBytes(group.Withdraw)))
		}
	}
	for _, f := range result.EORFamilies {
		if f == family.IPv4Unicast {
			bodies = append(bodies, []byte{0, 0, 0, 0})
			continue
		}
		bodies = append(bodies, packOldWithdraw(f, nil))
	}
	return bodies
}

type nlriWriter interface {
	PathID() uint32
	Bytes() []byte
}

func oldNLRIBytes[N nlriWriter](routes []N) []byte {
	var packed []byte
	for _, route := range routes {
		if id := route.PathID(); id != 0 {
			packed = binary.BigEndian.AppendUint32(packed, id)
		}
		packed = append(packed, route.Bytes()...)
	}
	return packed
}

func packOldAnnounce(group *bgptypes.NLRIGroup) []byte {
	var attrs []byte
	if group.Wire != nil {
		attrs = append(attrs, group.Wire.Packed()...)
	}
	present := attributeCodes(attrs)
	// The iBGP defaults the reactor adds (planBatchAttrs, reactor_api_batch.go).
	if !present[uint8(attribute.AttrOrigin)] {
		attrs = append(attrs, 0x40, 0x01, 0x01, 0x00)
	}
	if !present[uint8(attribute.AttrASPath)] {
		attrs = append(attrs, 0x40, 0x02, 0x00)
	}
	if !present[uint8(attribute.AttrLocalPref)] {
		attrs = append(attrs, 0x40, 0x05, 0x04, 0x00, 0x00, 0x00, 0x64)
	}

	nexthop := group.NextHop.Addr
	if group.NextHop.Policy == bgptypes.NextHopSelf {
		nexthop = oldLocalAddress
	}
	nlri := oldNLRIBytes(group.Announce)
	if group.Family == family.IPv4Unicast && nexthop.Is4() {
		attrs = append(attrs, 0x40, 0x03, 0x04)
		attrs = append(attrs, nexthop.AsSlice()...)
		return updateBody(nil, attrs, nlri)
	}
	var hops []netip.Addr
	if nexthop.IsValid() {
		hops = []netip.Addr{nexthop}
	}
	reach := attribute.NewMPReachNLRI(attribute.AFI(group.Family.AFI), attribute.SAFI(group.Family.SAFI), hops, nlri)
	value := make([]byte, reach.Len())
	reach.WriteTo(value, 0)
	attrs = appendExtendedAttribute(attrs, 0x80, uint8(attribute.AttrMPReachNLRI), value)
	return updateBody(nil, attrs, nil)
}

func packOldWithdraw(f family.Family, nlri []byte) []byte {
	if f == family.IPv4Unicast && nlri != nil {
		return updateBody(nlri, nil, nil)
	}
	value := binary.BigEndian.AppendUint16(nil, uint16(f.AFI))
	value = append(value, byte(f.SAFI))
	value = append(value, nlri...)
	return updateBody(nil, appendExtendedAttribute(nil, 0x80, uint8(attribute.AttrMPUnreachNLRI), value), nil)
}

func appendExtendedAttribute(attrs []byte, flags, code uint8, value []byte) []byte {
	attrs = append(attrs, flags|0x10, code)
	attrs = binary.BigEndian.AppendUint16(attrs, uint16(len(value)))
	return append(attrs, value...)
}

func updateBody(withdrawn, attrs, nlri []byte) []byte {
	body := binary.BigEndian.AppendUint16(nil, uint16(len(withdrawn)))
	body = append(body, withdrawn...)
	body = binary.BigEndian.AppendUint16(body, uint16(len(attrs)))
	body = append(body, attrs...)
	return append(body, nlri...)
}

// attributeCodes lists the type codes of a packed attribute block.
func attributeCodes(attrs []byte) map[uint8]bool {
	codes := map[uint8]bool{}
	for offset := 0; offset+3 <= len(attrs); {
		flags, code := attrs[offset], attrs[offset+1]
		codes[code] = true
		length, header := int(attrs[offset+2]), 3
		if flags&0x10 != 0 {
			if offset+4 > len(attrs) {
				break
			}
			length, header = int(binary.BigEndian.Uint16(attrs[offset+2:])), 4
		}
		offset += header + length
	}
	return codes
}

// oldRow is one route as test_old_commands `routes` writes it: the row keeps
// its order, and only the NLRI and attributes inside it are canonical.
func oldRow(action, name, nexthop string, nlri any, shown map[string]any) string {
	encoded, err := json.Marshal([]any{action, name, nexthop, canonicalValue(nlri), canonicalValue(shown)})
	if err != nil {
		return "unencodable"
	}
	return string(encoded)
}
