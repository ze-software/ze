// Design: docs/architecture/diagnostics/path-mtu.md -- the run, driven through the handler
//
// Goal: prove every acceptance criterion a unit test can pin, through the
// registered handler and one payload, with the inventory, the wire, the
// interface backend and the kernel state replaced by fakes. Method: fakeDeps
// stands in for liveDeps for one test, each target is a fakePath from
// search_test.go, and the assertions read the rendered document the way a
// pipe does.

package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/ipsecinventory"
	"github.com/ze-software/ze/internal/core/probe"
)

// fakeProber is one fakePath with the close the seam requires.
type fakeProber struct {
	*fakePath
	closed bool
}

func (f *fakeProber) close() error {
	f.closed = true
	return nil
}

// fakeDeps is one test's world: a path per target, the inventory answer, the
// XFRM interfaces, the underlay, the cached PMTUs and the local state. Every
// field has a default a test overrides.
type fakeDeps struct {
	paths       map[netip.Addr]*fakePath
	probers     []*fakeProber
	dfModes     []probe.DFMode
	tunnels     []ipsecinventory.Tunnel
	tunnelsErr  error
	xfrms       []xfrmInterface
	underlay    string
	underlayMTU int
	// underlayType is the netlink link type the fake interface backend reports
	// for the underlay; "device" is what a physical Ethernet port answers.
	underlayType string
	routeErr     error
	cached       map[netip.Addr]uint32
	// cacheErr, when set, is what the kernel cache read answers for every
	// target in place of a value or ErrPathMTUUnknown.
	cacheErr error
	counters fragmentationCounters
	probing  tcpMTUProbing
}

func newFakeDeps() *fakeDeps {
	return &fakeDeps{
		paths:        map[netip.Addr]*fakePath{},
		underlay:     "eth0",
		underlayMTU:  1500,
		underlayType: "device",
		cached:       map[netip.Addr]uint32{},
		probing:      tcpMTUProbingDisabled,
	}
}

// install replaces liveDeps for the test and restores it after.
func (f *fakeDeps) install(t *testing.T) {
	t.Helper()
	previous := liveDeps
	liveDeps = mtuDeps{
		tunnels: func() ([]ipsecinventory.Tunnel, error) {
			if f.tunnelsErr != nil {
				return nil, f.tunnelsErr
			}
			return f.tunnels, nil
		},
		openProber: func(_ context.Context, target netip.Addr, df probe.DFMode) (targetProber, error) {
			f.dfModes = append(f.dfModes, df)
			path, ok := f.paths[target]
			if !ok {
				return nil, errors.New("no fake path to " + target.String())
			}
			p := &fakeProber{fakePath: path}
			f.probers = append(f.probers, p)
			return p, nil
		},
		kernelPathMTU: func(_ context.Context, target netip.Addr) (uint32, error) {
			if f.cacheErr != nil {
				return 0, f.cacheErr
			}
			if v, ok := f.cached[target]; ok {
				return v, nil
			}
			return 0, probe.ErrPathMTUUnknown
		},
		routeInterface: func(_ netip.Addr) (string, error) {
			if f.routeErr != nil {
				return "", f.routeErr
			}
			return f.underlay, nil
		},
		getInterface: func(name string) (*iface.InterfaceInfo, error) {
			return &iface.InterfaceInfo{Name: name, Type: f.underlayType, MTU: f.underlayMTU, State: interfaceStateUp}, nil
		},
		xfrmInterfaces:  func() ([]xfrmInterface, error) { return f.xfrms, nil },
		fragmentation:   func() (fragmentationCounters, error) { return f.counters, nil },
		tcpMTUProbing:   func() (tcpMTUProbing, error) { return f.probing, nil },
		routeMTUExpires: func() (uint32, error) { return 600, nil },
	}
	t.Cleanup(func() { liveDeps = previous })
}

// clampedAt is a v4 path that reports and honors one MTU.
func clampedAt(mtu int) *fakePath {
	return &fakePath{overhead: icmpOverheadIPv4, ifaceMTU: 1500, pathMTU: mtu, reportsMTU: true}
}

// gcmTunnel is an up tunnel over aes128gcm, IPv4 endpoints, no UDP, inner
// IPv6, bound to if_id 1: the AC-2 arithmetic (overhead 52).
func gcmTunnel(peer string, remote netip.Addr) ipsecinventory.Tunnel {
	return ipsecinventory.Tunnel{
		Peer:              peer,
		ConfiguredRemote:  remote,
		Up:                true,
		InstalledRemote:   remote,
		InstalledLocal:    netip.MustParseAddr("192.0.2.1"),
		IfID:              1,
		Mode:              ipsecinventory.ModeTunnel,
		EncryptionName:    "aes128gcm",
		IntegrityName:     "none",
		Encryption:        ipsecinventory.EncryptionID(crypto.ENCR_AES_GCM_16),
		EncryptionKeyBits: 128,
		Integrity:         ipsecinventory.IntegrityID(crypto.AUTH_NONE),
		TSLocal:           netip.MustParsePrefix("2001:db8:1::/48"),
		TSRemote:          netip.MustParsePrefix("2001:db8:2::/48"),
	}
}

var (
	peerA = netip.MustParseAddr("198.51.100.10")
	peerB = netip.MustParseAddr("198.51.100.20")
	refV4 = netip.MustParseAddr("1.1.1.1")
)

// showMTU drives the registered handler and answers the document.
func showMTU(t *testing.T, args ...string) map[string]any {
	t.Helper()
	resp, err := registeredShowMTU(t)(nil, args)
	if err != nil {
		t.Fatalf("show mtu %v: %v", args, err)
	}
	if resp.Status != plugin.StatusDone {
		t.Fatalf("show mtu %v answered status %q, error %q", args, resp.Status, resp.Error)
	}
	return payloadOf(t, resp)
}

func rowsOf(t *testing.T, doc map[string]any, key string) []map[string]any {
	t.Helper()
	list, ok := doc[key].([]any)
	if !ok {
		t.Fatalf("the payload carries no %s list: %v", key, doc)
	}
	rows := make([]map[string]any, 0, len(list))
	for i := range list {
		row, ok := list[i].(map[string]any)
		if !ok {
			t.Fatalf("%s[%d] is not a row: %v", key, i, list[i])
		}
		rows = append(rows, row)
	}
	return rows
}

func stringsOf(t *testing.T, doc map[string]any, key string) []string {
	t.Helper()
	list, ok := doc[key].([]any)
	if !ok {
		t.Fatalf("the payload carries no %s list: %v", key, doc)
	}
	out := make([]string, 0, len(list))
	for i := range list {
		s, ok := list[i].(string)
		if !ok {
			t.Fatalf("%s[%d] is not a string: %v", key, i, list[i])
		}
		out = append(out, s)
	}
	return out
}

// text reads a string field, failing the test when it is absent or not one.
func text(t *testing.T, row map[string]any, key string) string {
	t.Helper()
	v, ok := row[key].(string)
	if !ok {
		t.Fatalf("%s is %v (%T), not a string", key, row[key], row[key])
	}
	return v
}

// number reads a JSON number the way the document decodes it.
func number(t *testing.T, row map[string]any, key string) float64 {
	t.Helper()
	v, ok := row[key].(float64)
	if !ok {
		t.Fatalf("%s is %v (%T), not a number", key, row[key], row[key])
	}
	return v
}

func noteTexts(t *testing.T, doc map[string]any) []string {
	t.Helper()
	var texts []string
	for _, n := range rowsOf(t, doc, fieldNotes) {
		texts = append(texts, text(t, n, fieldText))
	}
	return texts
}

func hasNoteContaining(t *testing.T, doc map[string]any, fragment string) bool {
	t.Helper()
	for _, text := range noteTexts(t, doc) {
		if strings.Contains(text, fragment) {
			return true
		}
	}
	return false
}

// TestShowMTUHostRunHasNoTunnelSection pins the host run (AC-1's payload
// half, AC-18's `host` half): one measurement, the way it was found, and no
// inventory, verdict or reference key at all.
func TestShowMTUHostRunHasNoTunnelSection(t *testing.T) {
	f := newFakeDeps()
	f.paths[peerA] = clampedAt(1400)
	f.install(t)

	doc := showMTU(t, argHost, peerA.String())
	if doc[fieldStatus] != runStatusOK.String() {
		t.Fatalf("status %v, want ok", doc[fieldStatus])
	}
	rows := rowsOf(t, doc, fieldMeasurements)
	if len(rows) != 1 {
		t.Fatalf("%d measurements, want 1", len(rows))
	}
	if got := number(t, rows[0], fieldPathMTU); got != 1400 {
		t.Errorf("path-mtu %v, want 1400", got)
	}
	if rows[0][fieldMethod] != searchViaICMP.String() {
		t.Errorf("method %v, want %s", rows[0][fieldMethod], searchViaICMP)
	}
	if rows[0][fieldLabel] != labelHost {
		t.Errorf("label %v, want host", rows[0][fieldLabel])
	}
	for _, absent := range []string{fieldInventory, fieldVerdict, fieldReference} {
		if _, present := doc[absent]; present {
			t.Errorf("a host run carries %s: %v", absent, doc[absent])
		}
	}
	underlay, ok := doc[fieldUnderlay].(map[string]any)
	if !ok {
		t.Fatalf("no underlay row: %v", doc)
	}
	if underlay[fieldSource] != underlaySourceRoute+peerA.String() {
		t.Errorf("underlay source %v", underlay[fieldSource])
	}
	if _, advised := underlay[fieldAdvice]; advised {
		t.Errorf("a host run advised on the underlay: %v", underlay)
	}
	if len(f.probers) != 1 || !f.probers[0].closed {
		t.Errorf("the prober was not opened once and closed: %+v", f.probers)
	}
	if len(f.dfModes) != 1 || f.dfModes[0] != probe.DFHonorCache {
		t.Errorf("df modes %v, want one DFHonorCache", f.dfModes)
	}
}

// TestShowMTUOversizedTunnelGetsACommand pins AC-2 and AC-4 end to end: an
// aes128gcm tunnel over a 1500 path has a ceiling of 1446, and an interface
// at 1500 is oversized by 54 octets with a command setting 1414.
func TestShowMTUOversizedTunnelGetsACommand(t *testing.T) {
	f := newFakeDeps()
	f.paths[peerA] = clampedAt(1500)
	f.paths[refV4] = clampedAt(1500)
	f.tunnels = []ipsecinventory.Tunnel{gcmTunnel("site-a", peerA)}
	f.xfrms = []xfrmInterface{{name: "xfrm1", ifID: 1, mtu: 1500, up: true}}
	f.install(t)

	doc := showMTU(t)
	if doc[fieldInventory] != inventoryRegistered.String() {
		t.Errorf("inventory %v", doc[fieldInventory])
	}
	tunnels := rowsOf(t, doc, fieldTunnels)
	if len(tunnels) != 1 {
		t.Fatalf("%d tunnels, want 1", len(tunnels))
	}
	row := tunnels[0]
	if row[fieldPeer] != "site-a" {
		t.Errorf("peer %v: the name carries nothing but the name", row[fieldPeer])
	}
	if row[fieldInterface] != "xfrm1" {
		t.Errorf("interface %v", row[fieldInterface])
	}
	if got := number(t, row, fieldCeiling); got != 1446 {
		t.Errorf("ceiling %v, want 1446", got)
	}
	if got := number(t, row, fieldRecommended); got != 1414 {
		t.Errorf("recommended %v, want 1414", got)
	}
	if got := number(t, row, fieldMSS); got != 1354 {
		t.Errorf("mss %v, want 1354", got)
	}
	if row[fieldVerdict] != tunnelVerdictOversized.String() {
		t.Errorf("verdict %v, want oversized", row[fieldVerdict])
	}
	if got := number(t, row, fieldOctets); got != 54 {
		t.Errorf("excess %v, want 54", got)
	}
	if row[fieldAssumed] != false {
		t.Errorf("assumed %v on a measured path", row[fieldAssumed])
	}
	if row[fieldSized] != true {
		t.Errorf("sized %v", row[fieldSized])
	}
	if doc[fieldVerdict] != runVerdictActionNeeded.String() {
		t.Errorf("run verdict %v, want action-needed", doc[fieldVerdict])
	}
	commands := stringsOf(t, doc, fieldCommands)
	if len(commands) != 1 || commands[0] != "set interface xfrm xfrm1 mtu 1414" {
		t.Errorf("commands %v", commands)
	}
	caveats := stringsOf(t, doc, fieldCaveats)
	if len(caveats) != 1 || !strings.Contains(caveats[0], "ICMP") {
		t.Errorf("caveats %v", caveats)
	}
}

// TestShowMTUTightAndOKTunnels pins AC-5: an interface inside the 32-octet
// margin is tight, names the spare, and still earns a command; one at the
// recommended value is ok and earns none.
func TestShowMTUTightAndOKTunnels(t *testing.T) {
	f := newFakeDeps()
	f.paths[peerA] = clampedAt(1500)
	f.paths[peerB] = clampedAt(1500)
	f.paths[refV4] = clampedAt(1500)
	tight := gcmTunnel("site-a", peerA)
	fine := gcmTunnel("site-b", peerB)
	fine.IfID = 2
	f.tunnels = []ipsecinventory.Tunnel{tight, fine}
	f.xfrms = []xfrmInterface{{name: "xfrm1", ifID: 1, mtu: 1420, up: true}, {name: "xfrm2", ifID: 2, mtu: 1414, up: true}}
	f.install(t)

	doc := showMTU(t)
	tunnels := rowsOf(t, doc, fieldTunnels)
	if tunnels[0][fieldVerdict] != tunnelVerdictTight.String() {
		t.Errorf("site-a verdict %v, want tight", tunnels[0][fieldVerdict])
	}
	if got := number(t, tunnels[0], fieldOctets); got != 26 {
		t.Errorf("spare %v, want 26", got)
	}
	if tunnels[1][fieldVerdict] != tunnelVerdictOK.String() {
		t.Errorf("site-b verdict %v, want ok", tunnels[1][fieldVerdict])
	}
	if doc[fieldVerdict] != runVerdictCheck.String() {
		t.Errorf("run verdict %v, want check", doc[fieldVerdict])
	}
	commands := stringsOf(t, doc, fieldCommands)
	if len(commands) != 1 || commands[0] != "set interface xfrm xfrm1 mtu 1414" {
		t.Errorf("commands %v", commands)
	}
}

// TestShowMTUNoUsableMTUHasNoCommand pins AC-6: a 1300 path leaves no value
// at or above 1280 for an inner IPv6 tunnel, the row says so, and no command
// names that interface.
func TestShowMTUNoUsableMTUHasNoCommand(t *testing.T) {
	f := newFakeDeps()
	f.paths[peerA] = clampedAt(1300)
	f.paths[refV4] = clampedAt(1500)
	f.tunnels = []ipsecinventory.Tunnel{gcmTunnel("site-a", peerA)}
	f.xfrms = []xfrmInterface{{name: "xfrm1", ifID: 1, mtu: 1500, up: true}}
	f.install(t)

	doc := showMTU(t)
	row := rowsOf(t, doc, fieldTunnels)[0]
	if row[fieldVerdict] != tunnelVerdictNoUsableMTU.String() {
		t.Errorf("verdict %v, want no-usable-mtu", row[fieldVerdict])
	}
	if _, present := row[fieldRecommended]; present {
		t.Errorf("a recommended value was written: %v", row[fieldRecommended])
	}
	if got := number(t, row, fieldCeiling); got != 1246 {
		t.Errorf("ceiling %v, want 1246", got)
	}
	if commands := stringsOf(t, doc, fieldCommands); len(commands) != 0 {
		t.Errorf("commands %v, want none for a tunnel with no usable MTU", commands)
	}
	if doc[fieldVerdict] != runVerdictActionNeeded.String() {
		t.Errorf("run verdict %v, want action-needed", doc[fieldVerdict])
	}
}

// TestShowMTUUnmeasurablePeerIsAssumed pins AC-7 and the assumed path: a
// peer that answers nothing is reported unmeasurable, the other peer is
// still measured, and the dead peer's tunnel is sized from the tightest
// measured path, marked assumed.
func TestShowMTUUnmeasurablePeerIsAssumed(t *testing.T) {
	f := newFakeDeps()
	dead := clampedAt(1500)
	dead.dead = true
	f.paths[peerA] = dead
	f.paths[peerB] = clampedAt(1400)
	f.paths[refV4] = clampedAt(1500)
	a := gcmTunnel("site-a", peerA)
	b := gcmTunnel("site-b", peerB)
	b.IfID = 2
	f.tunnels = []ipsecinventory.Tunnel{a, b}
	f.xfrms = []xfrmInterface{{name: "xfrm1", ifID: 1, mtu: 1500, up: true}, {name: "xfrm2", ifID: 2, mtu: 1500, up: true}}
	f.install(t)

	doc := showMTU(t)
	if doc[fieldStatus] != runStatusOK.String() {
		t.Errorf("status %v, want ok while one peer measured", doc[fieldStatus])
	}
	measurements := rowsOf(t, doc, fieldMeasurements)
	if measurements[0][fieldOutcome] != searchUnmeasurable.String() {
		t.Errorf("peer A outcome %v, want unmeasurable", measurements[0][fieldOutcome])
	}
	if _, present := measurements[0][fieldPathMTU]; present {
		t.Errorf("an unmeasurable target carries a path-mtu: %v", measurements[0])
	}
	if got := number(t, measurements[1], fieldPathMTU); got != 1400 {
		t.Errorf("peer B path-mtu %v, want 1400", got)
	}
	tunnels := rowsOf(t, doc, fieldTunnels)
	if tunnels[0][fieldAssumed] != true {
		t.Errorf("site-a assumed %v, want true", tunnels[0][fieldAssumed])
	}
	if got := number(t, tunnels[0], fieldPathMTU); got != 1400 {
		t.Errorf("site-a assumed path-mtu %v, want the tightest measured 1400", got)
	}
	if tunnels[1][fieldAssumed] != false {
		t.Errorf("site-b assumed %v, want false", tunnels[1][fieldAssumed])
	}
	if !hasNoteContaining(t, doc, peerA.String()+" answered no probe") {
		t.Errorf("no note names the unmeasurable peer: %v", noteTexts(t, doc))
	}
}

// TestShowMTUExhaustiveBypassesTheCache pins AC-11 through the handler: exhaustive
// opens the prober in DFBypassCache, the cached 1300 does not become the
// answer, and the note names the stale cache.
func TestShowMTUExhaustiveBypassesTheCache(t *testing.T) {
	f := newFakeDeps()
	f.paths[peerA] = clampedAt(1400)
	f.cached[peerA] = 1300
	f.install(t)

	doc := showMTU(t, argHost, peerA.String(), argExhaustive)
	row := rowsOf(t, doc, fieldMeasurements)[0]
	if got := number(t, row, fieldPathMTU); got != 1400 {
		t.Errorf("path-mtu %v, want the wire's 1400", got)
	}
	if got := number(t, row, fieldCachedPathMTU); got != 1300 {
		t.Errorf("cached-path-mtu %v, want 1300", got)
	}
	if f.dfModes[0] != probe.DFBypassCache {
		t.Errorf("exhaustive opened the prober in %v, want DFBypassCache", f.dfModes[0])
	}
	if !hasNoteContaining(t, doc, "stale PMTU of 1300") {
		t.Errorf("no note names the stale cache: %v", noteTexts(t, doc))
	}
}

// TestShowMTUUnderlayAdvice pins AC-12, AC-13 and AC-14 through the handler:
// the reference at the full MTU says not clamped; the reference at the peers'
// value says circuit clamped and produces the underlay command; a silent
// reference makes the advice undecidable.
func TestShowMTUUnderlayAdvice(t *testing.T) {
	cases := []struct {
		name      string
		reference *fakePath
		outcome   underlayOutcome
		command   string
	}{
		{"reference full: not clamped (AC-12)", clampedAt(1500), underlayNotClamped, ""},
		{"reference equals peers: circuit clamped (AC-13)", clampedAt(1400), underlayCircuitClamped, "set interface ethernet eth0 mtu 1400"},
		{"reference silent: undecidable (AC-14)", &fakePath{overhead: icmpOverheadIPv4, ifaceMTU: 1500, pathMTU: 1500, dead: true}, underlayUndecidable, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeDeps()
			f.paths[peerA] = clampedAt(1400)
			f.paths[refV4] = tc.reference
			f.tunnels = []ipsecinventory.Tunnel{gcmTunnel("site-a", peerA)}
			f.xfrms = []xfrmInterface{{name: "xfrm1", ifID: 1, mtu: 1314, up: true}}
			f.install(t)

			doc := showMTU(t)
			underlay, ok := doc[fieldUnderlay].(map[string]any)
			if !ok {
				t.Fatalf("no underlay row: %v", doc)
			}
			if underlay[fieldAdvice] != tc.outcome.String() {
				t.Errorf("advice %v, want %s", underlay[fieldAdvice], tc.outcome)
			}
			ref, ok := doc[fieldReference].(map[string]any)
			if !ok {
				t.Fatalf("no reference row: %v", doc)
			}
			if ref[fieldHost] != refV4.String() {
				t.Errorf("reference host %v", ref[fieldHost])
			}
			commands := stringsOf(t, doc, fieldCommands)
			if tc.command == "" && len(commands) != 0 {
				t.Errorf("commands %v, want none", commands)
			}
			if tc.command != "" && (len(commands) != 1 || commands[0] != tc.command) {
				t.Errorf("commands %v, want %q", commands, tc.command)
			}
			if !hasNoteContaining(t, doc, "eth0") {
				t.Errorf("no note names the underlay: %v", noteTexts(t, doc))
			}
			if underlay[fieldKind] != "ethernet" {
				t.Errorf("underlay kind %v, want ethernet for a netlink device: %v", underlay[fieldKind], underlay)
			}
		})
	}
}

// TestShowMTUUnderlayCommandNamesTheLinkKind pins that the underlay command
// is spelled with the YANG list the underlay belongs to: a veth underlay (the
// functional tests' sr0) earns `set interface veth`, and a link type the
// schema does not configure earns the value in the note and no command at
// all, never an `ethernet` block for a link that is not one.
func TestShowMTUUnderlayCommandNamesTheLinkKind(t *testing.T) {
	cases := []struct {
		name     string
		linkType string
		kind     string
		command  string
	}{
		{"veth underlay", "veth", "veth", "set interface veth sr0 mtu 1400"},
		{"bridge underlay", "bridge", "bridge", "set interface bridge sr0 mtu 1400"},
		{"ppp underlay: no list in the schema", "ppp", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeDeps()
			f.underlay, f.underlayType = "sr0", tc.linkType
			f.paths[peerA] = clampedAt(1400)
			f.paths[refV4] = clampedAt(1400)
			f.tunnels = []ipsecinventory.Tunnel{gcmTunnel("site-a", peerA)}
			f.xfrms = []xfrmInterface{{name: "xfrm1", ifID: 1, mtu: 1314, up: true}}
			f.install(t)

			doc := showMTU(t)
			underlay, ok := doc[fieldUnderlay].(map[string]any)
			if !ok {
				t.Fatalf("no underlay row: %v", doc)
			}
			if underlay[fieldAdvice] != underlayCircuitClamped.String() {
				t.Fatalf("advice %v, want circuit-clamped: %v", underlay[fieldAdvice], underlay)
			}
			kind, present := underlay[fieldKind]
			if tc.kind == "" && present {
				t.Errorf("underlay kind %v, want none for a %s link", kind, tc.linkType)
			}
			if tc.kind != "" && kind != tc.kind {
				t.Errorf("underlay kind %v, want %s", kind, tc.kind)
			}
			commands := stringsOf(t, doc, fieldCommands)
			if tc.command == "" && len(commands) != 0 {
				t.Errorf("commands %v, want none for a %s link", commands, tc.linkType)
			}
			if tc.command != "" && (len(commands) != 1 || commands[0] != tc.command) {
				t.Errorf("commands %v, want %q", commands, tc.command)
			}
			if tc.command == "" && !hasNoteContaining(t, doc, "no configuration command is listed because sr0") {
				t.Errorf("no note says why the command is missing: %v", noteTexts(t, doc))
			}
		})
	}
}

// TestShowMTUCacheReadFailureIsNoted pins that a kernel cache read that FAILS
// is reported, not read as "no entry": the caution names the target, and the
// measurement still runs. ErrPathMTUUnknown stays silent, because no entry is
// the common case and earns no note.
func TestShowMTUCacheReadFailureIsNoted(t *testing.T) {
	f := newFakeDeps()
	f.paths[peerA] = clampedAt(1400)
	f.cacheErr = errors.New("netlink: route dump refused")
	f.install(t)

	doc := showMTU(t, argHost, peerA.String())
	if doc[fieldStatus] != runStatusOK.String() {
		t.Fatalf("status %v, want ok: %v", doc[fieldStatus], doc)
	}
	if !hasNoteContaining(t, doc, "the cached path MTU for "+peerA.String()+" could not be read") {
		t.Errorf("no note names the failed cache read: %v", noteTexts(t, doc))
	}

	f = newFakeDeps()
	f.paths[peerA] = clampedAt(1400)
	f.install(t)
	doc = showMTU(t, argHost, peerA.String())
	if hasNoteContaining(t, doc, "could not be read") {
		t.Errorf("a kernel holding no entry was reported as a failed read: %v", noteTexts(t, doc))
	}
}

// TestShowMTUTransportModeUsesTransportArithmetic pins AC-16: a transport
// mode SA's ceiling is the transport figure, larger than the tunnel one for
// the same path, and the row names the mode.
func TestShowMTUTransportModeUsesTransportArithmetic(t *testing.T) {
	f := newFakeDeps()
	f.paths[peerA] = clampedAt(1500)
	f.paths[refV4] = clampedAt(1500)
	transport := gcmTunnel("site-a", peerA)
	transport.Mode = ipsecinventory.ModeTransport
	transport.TSRemote = netip.MustParsePrefix("198.51.100.10/32")
	transport.TSLocal = netip.MustParsePrefix("192.0.2.1/32")
	f.tunnels = []ipsecinventory.Tunnel{transport}
	f.xfrms = []xfrmInterface{{name: "xfrm1", ifID: 1, mtu: 1500, up: true}}
	f.install(t)

	doc := showMTU(t)
	row := rowsOf(t, doc, fieldTunnels)[0]
	if row[fieldMode] != ipsecinventory.ModeTransport.String() {
		t.Errorf("mode %v", row[fieldMode])
	}
	overhead, err := deriveESPOverhead(&transport)
	if err != nil {
		t.Fatalf("overhead: %v", err)
	}
	want := ceiling(1500, &overhead)
	if got := number(t, row, fieldCeiling); got != float64(want) {
		t.Errorf("ceiling %v, want the transport figure %d", got, want)
	}
	if want <= 1446 {
		t.Errorf("transport ceiling %d is not above the tunnel-mode 1446", want)
	}
}

// TestShowMTUProbesTheInstalledEndpoint pins AC-17: the measurement targets
// the installed remote, not the configured one, and the row shows it.
func TestShowMTUProbesTheInstalledEndpoint(t *testing.T) {
	f := newFakeDeps()
	installed := netip.MustParseAddr("203.0.113.77")
	f.paths[installed] = clampedAt(1500)
	f.paths[refV4] = clampedAt(1500)
	tun := gcmTunnel("site-a", peerA)
	tun.InstalledRemote = installed
	f.tunnels = []ipsecinventory.Tunnel{tun}
	f.xfrms = []xfrmInterface{{name: "xfrm1", ifID: 1, mtu: 1414, up: true}}
	f.install(t)

	doc := showMTU(t)
	measurements := rowsOf(t, doc, fieldMeasurements)
	if measurements[0][fieldTarget] != installed.String() {
		t.Errorf("target %v, want the installed %s", measurements[0][fieldTarget], installed)
	}
	if rowsOf(t, doc, fieldTunnels)[0][fieldRemote] != installed.String() {
		t.Errorf("remote %v", rowsOf(t, doc, fieldTunnels)[0][fieldRemote])
	}
	if _, probed := f.paths[peerA]; probed {
		t.Errorf("the configured address was given a path; it must not be probed")
	}
}

// TestShowMTUNoInventoryIsNotZeroTunnels pins AC-18: an unregistered
// inventory is named as such, distinct from a registered one holding no
// tunnel, and the host run does not consult it.
func TestShowMTUNoInventoryIsNotZeroTunnels(t *testing.T) {
	f := newFakeDeps()
	f.paths[refV4] = clampedAt(1500)
	f.tunnelsErr = ipsecinventory.ErrNotRegistered
	f.install(t)

	doc := showMTU(t)
	if doc[fieldInventory] != inventoryNotRegistered.String() {
		t.Errorf("inventory %v, want not-registered", doc[fieldInventory])
	}
	if _, present := doc[fieldVerdict]; present {
		t.Errorf("a verdict was written with no inventory: %v", doc[fieldVerdict])
	}
	if !hasNoteContaining(t, doc, "no IPsec inventory is registered") {
		t.Errorf("no note says the inventory is unregistered: %v", noteTexts(t, doc))
	}

	f.tunnelsErr = nil
	doc = showMTU(t)
	if doc[fieldInventory] != inventoryRegistered.String() {
		t.Errorf("inventory %v, want registered", doc[fieldInventory])
	}
	if doc[fieldVerdict] != runVerdictNoTunnels.String() {
		t.Errorf("verdict %v, want no-tunnels", doc[fieldVerdict])
	}

	f.tunnelsErr = ipsecinventory.ErrNotRegistered
	f.paths[peerA] = clampedAt(1400)
	doc = showMTU(t, argHost, peerA.String())
	if doc[fieldStatus] != runStatusOK.String() {
		t.Errorf("host run status %v with no inventory, want ok", doc[fieldStatus])
	}
}

// TestShowMTUEveryPipeRendersOnePayload pins AC-19: `| json`, `| yaml` and
// `| table` each accept the document, and the verdict is its own field with
// the peer name carrying no marker.
func TestShowMTUEveryPipeRendersOnePayload(t *testing.T) {
	f := newFakeDeps()
	dead := clampedAt(1500)
	dead.dead = true
	f.paths[peerA] = dead
	f.paths[peerB] = clampedAt(1400)
	f.paths[refV4] = clampedAt(1500)
	a := gcmTunnel("site-a", peerA)
	b := gcmTunnel("site-b", peerB)
	b.IfID = 2
	f.tunnels = []ipsecinventory.Tunnel{a, b}
	f.xfrms = []xfrmInterface{{name: "xfrm1", ifID: 1, mtu: 1500, up: true}, {name: "xfrm2", ifID: 2, mtu: 1500, up: true}}
	f.install(t)

	resp, err := registeredShowMTU(t)(nil, nil)
	if err != nil {
		t.Fatalf("show mtu: %v", err)
	}
	encoded, err := json.Marshal(resp.Data)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	for _, pipe := range []string{"json", "yaml", "table"} {
		_, ops := command.ParsePipe("show mtu | " + pipe)
		rendered, refusal := command.ApplyPipes(string(encoded), ops, nil, nil)
		if refusal != "" {
			t.Errorf("`| %s` refused the payload: %s", pipe, refusal)
		}
		if !strings.Contains(rendered, "site-a") {
			t.Errorf("`| %s` lost the tunnel rows:\n%s", pipe, rendered)
		}
	}
	doc := payloadOf(t, resp)
	for _, row := range rowsOf(t, doc, fieldTunnels) {
		peer := text(t, row, fieldPeer)
		if strings.ContainsAny(peer, "*!") {
			t.Errorf("peer %q carries a marker; the verdict is its own field", peer)
		}
		if strings.Contains(peer, text(t, row, fieldVerdict)) {
			t.Errorf("peer %q carries a marker; the verdict is its own field", peer)
		}
		if row[fieldAssumed] == true && strings.Contains(peer, "assumed") {
			t.Errorf("peer %q carries the assumed marker", peer)
		}
	}
}

// TestShowMTUFragmentationCountersAreAbsolute pins AC-20 through the
// handler: non-zero outbound fragmentation counters reach the notes with
// their absolute values.
func TestShowMTUFragmentationCountersAreAbsolute(t *testing.T) {
	f := newFakeDeps()
	f.paths[peerA] = clampedAt(1400)
	f.counters = fragmentationCounters{ipFragOKs: 1234, ipFragFails: 7}
	f.install(t)

	doc := showMTU(t, argHost, peerA.String())
	if !hasNoteContaining(t, doc, "1234") {
		t.Errorf("no note carries the absolute IpFragOKs 1234: %v", noteTexts(t, doc))
	}
	if !hasNoteContaining(t, doc, "7") {
		t.Errorf("no note carries IpFragFails 7: %v", noteTexts(t, doc))
	}
	if doc[fieldTCPMTUProbing] != tcpMTUProbingDisabled.String() {
		t.Errorf("tcp-mtu-probing %v", doc[fieldTCPMTUProbing])
	}
}

// TestShowMTUDownAndUnboundTunnelsAreListed pins the three not-sized
// shapes: a down tunnel whose configured remote is a hostname is listed with
// the down verdict and no target; a policy-based SA is listed unsized; a
// refused transform is a fault that makes the run verdict action-needed.
func TestShowMTUDownAndUnboundTunnelsAreListed(t *testing.T) {
	f := newFakeDeps()
	f.paths[peerA] = clampedAt(1500)
	f.paths[refV4] = clampedAt(1500)
	down := ipsecinventory.Tunnel{Peer: "site-down"}
	policy := gcmTunnel("site-policy", peerA)
	policy.IfID = 0
	refused := gcmTunnel("site-refused", peerA)
	refused.IfID = 2
	refused.Encryption = ipsecinventory.EncryptionID(crypto.ENCR_AES_CCM_16)
	f.tunnels = []ipsecinventory.Tunnel{down, policy, refused}
	f.xfrms = []xfrmInterface{{name: "xfrm2", ifID: 2, mtu: 1500, up: true}}
	f.install(t)

	doc := showMTU(t)
	rows := rowsOf(t, doc, fieldTunnels)
	if len(rows) != 3 {
		t.Fatalf("%d tunnels, want 3", len(rows))
	}
	if rows[0][fieldVerdict] != tunnelVerdictDown.String() || rows[0][fieldSized] != false {
		t.Errorf("down tunnel row %v", rows[0])
	}
	if _, present := rows[0][fieldRemote]; present {
		t.Errorf("a hostname remote was written as an address: %v", rows[0][fieldRemote])
	}
	if rows[1][fieldSized] != false || !strings.Contains(text(t, rows[1], fieldReason), "policy-based") {
		t.Errorf("policy-based row %v", rows[1])
	}
	if rows[2][fieldSized] != false || !strings.HasPrefix(text(t, rows[2], fieldReason), refusedTransformReason) {
		t.Errorf("refused row %v", rows[2])
	}
	if !hasNoteContaining(t, doc, "site-refused") {
		t.Errorf("no fault note names the refused peer: %v", noteTexts(t, doc))
	}
	// The ladder puts a down tunnel's CHECK above a fault outside the table.
	if doc[fieldVerdict] != runVerdictCheck.String() {
		t.Errorf("run verdict %v, want check while a tunnel is down", doc[fieldVerdict])
	}
	if len(rowsOf(t, doc, fieldMeasurements)) != 1 {
		t.Errorf("measurements %v, want the shared peer once; the reference has its own row", rowsOf(t, doc, fieldMeasurements))
	}

	// Alone, the refused transform is the fault that makes the run ACTION NEEDED.
	f.tunnels = []ipsecinventory.Tunnel{refused}
	doc = showMTU(t)
	if doc[fieldVerdict] != runVerdictActionNeeded.String() {
		t.Errorf("run verdict %v, want action-needed for a refused transform", doc[fieldVerdict])
	}
}

// TestShowMTUDFGateFailureStopsTheRun pins the run status: a target that
// answers the 10000-octet gate ends the run as df-gate-failed and the later
// targets are not probed.
func TestShowMTUDFGateFailureStopsTheRun(t *testing.T) {
	f := newFakeDeps()
	f.paths[peerA] = &fakePath{overhead: icmpOverheadIPv4, ifaceMTU: 65000, pathMTU: 65000}
	f.paths[peerB] = clampedAt(1400)
	a := gcmTunnel("site-a", peerA)
	b := gcmTunnel("site-b", peerB)
	b.IfID = 2
	f.tunnels = []ipsecinventory.Tunnel{a, b}
	f.install(t)

	doc := showMTU(t)
	if doc[fieldStatus] != runStatusDFGateFailed.String() {
		t.Errorf("status %v, want df-gate-failed", doc[fieldStatus])
	}
	rows := rowsOf(t, doc, fieldMeasurements)
	if rows[0][fieldOutcome] != searchDFGateFailed.String() {
		t.Errorf("first outcome %v", rows[0][fieldOutcome])
	}
	if rows[1][fieldOutcome] != "error" {
		t.Errorf("second target was probed after the gate failed: %v", rows[1])
	}
	if len(f.probers) != 1 {
		t.Errorf("%d probers opened, want 1", len(f.probers))
	}
}

// TestShowMTUDetailListsEveryProbe pins the detail view: every probe sent,
// with its payload and wire size.
func TestShowMTUDetailListsEveryProbe(t *testing.T) {
	f := newFakeDeps()
	f.paths[peerA] = clampedAt(1400)
	f.install(t)

	doc := showMTU(t, argHost, peerA.String(), argDetail)
	row := rowsOf(t, doc, fieldMeasurements)[0]
	sent := rowsOf(t, row, fieldProbesSent)
	if len(sent) != int(number(t, row, fieldProbes)) {
		t.Fatalf("%d probe rows for %v probes", len(sent), row[fieldProbes])
	}
	if got := number(t, sent[0], fieldWire); got != float64(sanityPayload+icmpOverheadIPv4) {
		t.Errorf("first wire size %v, want the gate %d", got, sanityPayload+icmpOverheadIPv4)
	}
	if sent[0][fieldOutcome] == nil {
		t.Errorf("a probe row carries no outcome: %v", sent[0])
	}
}
