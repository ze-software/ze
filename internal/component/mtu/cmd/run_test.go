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
	"github.com/ze-software/ze/internal/core/ikeprobe"
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
		// The real leaf: it is empty in this binary, which links no engine, so a
		// test that wants an IKE answer registers a fake through registerFakeIKEProber
		// and every other test sees the leaf's own ErrNotRegistered.
		probeIKE: ikeprobe.Probe,
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

// registerFakeIKEProber stands a prober in the ikeprobe leaf for one test. The
// engine is not linked into this test binary, so the leaf starts empty and every
// request the run makes reaches fn through the real ikeprobe.Probe.
func registerFakeIKEProber(t *testing.T, fn ikeprobe.Prober) {
	t.Helper()
	ikeprobe.ResetForTest()
	t.Cleanup(ikeprobe.ResetForTest)
	ikeprobe.Register(fn)
}

// TestShowMTUUsesIKEProbeWhenSAIsUp pins the wiring row "show mtu with a live SA
// to the peer": a tunnel whose inventory Up is true is confirmed over IKE, the
// request goes through the ikeprobe leaf with the peer's name and the ICMP figure
// as the wire size, one exchange confirms it (no exchange above the ICMP figure,
// A-6), and the measurement row names `ike` as its prober. The reference row,
// which has no SA, names `icmp`.
func TestShowMTUUsesIKEProbeWhenSAIsUp(t *testing.T) {
	f := newFakeDeps()
	f.paths[peerA] = clampedAt(1400)
	f.paths[refV4] = clampedAt(1500)
	f.tunnels = []ipsecinventory.Tunnel{gcmTunnel("site-a", peerA)}
	f.install(t)
	var asked []ikeprobe.Request
	registerFakeIKEProber(t, func(_ context.Context, req ikeprobe.Request) (ikeprobe.Result, error) {
		asked = append(asked, req)
		if req.WireOctets <= 1400 {
			return ikeprobe.Result{Outcome: ikeprobe.OutcomeFits, WireOctets: req.WireOctets}, nil
		}
		return ikeprobe.Result{Outcome: ikeprobe.OutcomeTooBig, WireOctets: req.WireOctets}, nil
	})

	doc := showMTU(t)
	rows := rowsOf(t, doc, fieldMeasurements)
	if got := text(t, rows[0], fieldProber); got != "ike" {
		t.Errorf("the live peer's prober is %q, want ike", got)
	}
	if _, declined := rows[0][fieldIKEDeclined]; declined {
		t.Errorf("the live peer's row says the IKE prober declined: %v", rows[0][fieldIKEDeclined])
	}
	if got := number(t, rows[0], fieldPathMTU); got != 1400 {
		t.Errorf("path MTU %v, want 1400", got)
	}
	if got := number(t, rows[0], fieldIKEConfirmed); got != 1400 {
		t.Errorf("ike-confirmed %v, want the size answered whole, 1400", got)
	}
	if len(asked) != 1 {
		t.Fatalf("the IKE prober was asked %d times (%+v), want the figure once", len(asked), asked)
	}
	for i, req := range asked {
		if req.Peer != "site-a" {
			t.Errorf("request %d names peer %q, want site-a", i, req.Peer)
		}
		if req.DF != probe.DFHonorCache {
			t.Errorf("request %d carries DF mode %v, want honor-cache on a default run", i, req.DF)
		}
	}
	if asked[0].WireOctets != 1400 {
		t.Errorf("the IKE prober was asked %d octets, want the ICMP figure 1400", asked[0].WireOctets)
	}
	reference, ok := doc[fieldReference].(map[string]any)
	if !ok {
		t.Fatalf("the payload carries no reference row: %v", doc[fieldReference])
	}
	if got := text(t, reference, fieldProber); got != "icmp" {
		t.Errorf("the reference row's prober is %q, want icmp", got)
	}
}

// gridProber is a scripted CBC-suite prober: every ask is sent at the largest
// size on the suite's 16-octet grid at or below it (the grid below 1400 is
// 1392), a sent size at or below fitsUpTo fits and one above it is too big.
func gridProber(fitsUpTo uint16, asked *[]ikeprobe.Request) ikeprobe.Prober {
	return func(_ context.Context, req ikeprobe.Request) (ikeprobe.Result, error) {
		*asked = append(*asked, req)
		sent := req.WireOctets - (req.WireOctets-1392)%16
		if sent <= fitsUpTo {
			return ikeprobe.Result{Outcome: ikeprobe.OutcomeFits, WireOctets: sent}, nil
		}
		return ikeprobe.Result{Outcome: ikeprobe.OutcomeTooBig, WireOctets: sent}, nil
	}
}

// TestIKEProbeReportsTheSizeThatFit pins the refuted half of the rule on a
// cipher grid: the ask at the ICMP figure 1400 is sent at 1392 and answered
// too big, so IKE refuted the ICMP figure, the descent finds a smaller grid
// size the peer answered whole, and THAT size is the path MTU, equal to
// ike-confirmed, with a note naming both figures.
func TestIKEProbeReportsTheSizeThatFit(t *testing.T) {
	f := newFakeDeps()
	f.paths[peerA] = clampedAt(1400)
	f.paths[refV4] = clampedAt(1500)
	f.tunnels = []ipsecinventory.Tunnel{gcmTunnel("site-a", peerA)}
	f.install(t)
	var asked []ikeprobe.Request
	registerFakeIKEProber(t, gridProber(1300, &asked))

	doc := showMTU(t)
	rows := rowsOf(t, doc, fieldMeasurements)
	if got := text(t, rows[0], fieldProber); got != "ike" {
		t.Errorf("prober %q, want ike", got)
	}
	if _, declined := rows[0][fieldIKEDeclined]; declined {
		t.Errorf("the row says the IKE prober declined: %v", rows[0][fieldIKEDeclined])
	}
	pathMTU := number(t, rows[0], fieldPathMTU)
	if pathMTU > 1300 || pathMTU < 1280 || (1392-int(pathMTU))%16 != 0 {
		t.Errorf("path MTU %v, want a grid size the path carries, between 1280 and 1300", pathMTU)
	}
	if got := number(t, rows[0], fieldIKEConfirmed); got != pathMTU {
		t.Errorf("ike-confirmed %v, want the path MTU %v: IKE refuted the ICMP figure", got, pathMTU)
	}
	if len(asked) == 0 || asked[0].WireOctets != 1400 {
		t.Fatalf("the IKE prober was asked %+v, want the ICMP figure 1400 first", asked)
	}
	if got := largestAsked(asked); got != 1400 {
		t.Errorf("the largest exchange was %d octets, want nothing above the ICMP figure 1400", got)
	}
	if !hasNoteContaining(t, doc, "octets where ICMP measured 1400") {
		t.Errorf("no note names the two figures: %v", noteTexts(t, doc))
	}
}

// TestIKEFitBelowTheAskKeepsTheICMPFigure pins the confirmed half of the rule:
// an ask at the ICMP figure 1400 sent on the grid at 1392 and answered whole
// tested nothing above 1392, so it refutes nothing. The ICMP figure stands as
// the path MTU, the row says prober ike, ike-confirmed carries 1392, one
// exchange was spent, the caution note says IKE confirmed down to 1392, and no
// note claims a second measurement.
func TestIKEFitBelowTheAskKeepsTheICMPFigure(t *testing.T) {
	f := newFakeDeps()
	f.paths[peerA] = clampedAt(1400)
	f.paths[refV4] = clampedAt(1500)
	f.tunnels = []ipsecinventory.Tunnel{gcmTunnel("site-a", peerA)}
	f.install(t)
	var asked []ikeprobe.Request
	registerFakeIKEProber(t, gridProber(1400, &asked))

	doc := showMTU(t)
	rows := rowsOf(t, doc, fieldMeasurements)
	if got := text(t, rows[0], fieldProber); got != "ike" {
		t.Errorf("prober %q, want ike", got)
	}
	if got := number(t, rows[0], fieldPathMTU); got != 1400 {
		t.Errorf("path MTU %v, want the ICMP figure 1400: a fit below the ask refutes nothing", got)
	}
	if got := number(t, rows[0], fieldIKEConfirmed); got != 1392 {
		t.Errorf("ike-confirmed %v, want the size sent and answered whole, 1392", got)
	}
	if _, declined := rows[0][fieldIKEDeclined]; declined {
		t.Errorf("the row says the IKE prober declined: %v", rows[0][fieldIKEDeclined])
	}
	if len(asked) != 1 || asked[0].WireOctets != 1400 {
		t.Fatalf("the IKE prober was asked %+v, want the ICMP figure 1400 once", asked)
	}
	if got := number(t, rows[0], fieldExchanges); got != 1 {
		t.Errorf("exchanges %v, want 1", got)
	}
	if !hasNoteContaining(t, doc, "asked at 1400 octets was sent at 1392; IKE confirmed the path down to 1392 octets") {
		t.Errorf("no note names the grid rounding as a confirmation: %v", noteTexts(t, doc))
	}
	if hasNoteContaining(t, doc, "where ICMP measured") {
		t.Errorf("a note claims IKE measured a second figure: %v", noteTexts(t, doc))
	}
}

// TestShowMTUFallsBackToICMPWhenSAIsDown pins the wiring row "show mtu with the
// tunnel down" and AC-2's refusal half: a tunnel whose Up is false is never
// offered to the IKE prober and its row names `icmp` with no reason; a live tunnel
// the engine refuses by name keeps its ICMP figure, names `icmp`, and says why in
// the row and in a note; and a build with no prober registered is named as such
// rather than read as a refusal (AC-9).
func TestShowMTUFallsBackToICMPWhenSAIsDown(t *testing.T) {
	f := newFakeDeps()
	f.paths[peerA] = clampedAt(1400)
	f.paths[peerB] = clampedAt(1300)
	f.paths[refV4] = clampedAt(1500)
	down := gcmTunnel("site-a", peerA)
	down.Up = false
	f.tunnels = []ipsecinventory.Tunnel{down, gcmTunnel("site-b", peerB)}
	f.install(t)
	registerFakeIKEProber(t, func(_ context.Context, req ikeprobe.Request) (ikeprobe.Result, error) {
		if req.Peer != "site-b" {
			t.Errorf("the IKE prober was asked for %q; only the live site-b may be asked", req.Peer)
		}
		return ikeprobe.Result{Outcome: ikeprobe.OutcomeRefused, Refusal: ikeprobe.RefusalSADown}, nil
	})

	doc := showMTU(t)
	rows := rowsOf(t, doc, fieldMeasurements)
	if got := text(t, rows[0], fieldProber); got != "icmp" {
		t.Errorf("the down tunnel's prober is %q, want icmp", got)
	}
	if _, declined := rows[0][fieldIKEDeclined]; declined {
		t.Errorf("the down tunnel was offered to the IKE prober: %v", rows[0][fieldIKEDeclined])
	}
	if got := text(t, rows[1], fieldProber); got != "icmp" {
		t.Errorf("the refused tunnel's prober is %q, want icmp", got)
	}
	if got := text(t, rows[1], fieldIKEDeclined); got != "refused: sa-down" {
		t.Errorf("the refused tunnel's row says %q, want refused: sa-down", got)
	}
	if got := number(t, rows[1], fieldPathMTU); got != 1300 {
		t.Errorf("the refused tunnel's path MTU is %v, want the ICMP figure 1300", got)
	}
	if !hasNoteContaining(t, doc, peerB.String()+" keeps its ICMP figure") {
		t.Errorf("no note says the IKE prober declined for %s: %v", peerB, noteTexts(t, doc))
	}

	ikeprobe.ResetForTest()
	doc = showMTU(t)
	rows = rowsOf(t, doc, fieldMeasurements)
	if got := text(t, rows[1], fieldProber); got != "icmp" {
		t.Errorf("with no prober registered the prober is %q, want icmp", got)
	}
	if got := text(t, rows[1], fieldIKEDeclined); !strings.Contains(got, "not in this build") {
		t.Errorf("with no prober registered the row says %q, want the leaf's own reason", got)
	}
}

// ikeAnswers is a scripted IKE prober for one test: fits at or below fitsUpTo,
// too-big above it, and every request recorded. tooBigFirst answers too-big
// that many times at a size before the size's real answer, the lost DF copy
// of AC-3; tooBigAt limits it to one size, and zero applies it to every size.
type ikeAnswers struct {
	fitsUpTo    int
	tooBigFirst int
	tooBigAt    uint16
	seen        map[uint16]int
	asked       []ikeprobe.Request
}

func (a *ikeAnswers) probe(_ context.Context, req ikeprobe.Request) (ikeprobe.Result, error) {
	a.asked = append(a.asked, req)
	if a.seen == nil {
		a.seen = map[uint16]int{}
	}
	a.seen[req.WireOctets]++
	scripted := a.tooBigAt == 0 || a.tooBigAt == req.WireOctets
	if scripted && a.seen[req.WireOctets] <= a.tooBigFirst {
		return ikeprobe.Result{Outcome: ikeprobe.OutcomeTooBig, WireOctets: req.WireOctets}, nil
	}
	if int(req.WireOctets) <= a.fitsUpTo {
		return ikeprobe.Result{Outcome: ikeprobe.OutcomeFits, WireOctets: req.WireOctets}, nil
	}
	return ikeprobe.Result{Outcome: ikeprobe.OutcomeTooBig, WireOctets: req.WireOctets}, nil
}

// largestAsked answers the largest wire size the IKE prober was asked for.
func largestAsked(asked []ikeprobe.Request) int {
	largest := 0
	for _, req := range asked {
		largest = max(largest, int(req.WireOctets))
	}
	return largest
}

// TestIKEProbeConfirmsThenDescends pins the IKE prober's search: the ICMP figure
// is asked first and, when it fits, is the answer with no exchange above it;
// when it is too big the same ladder-then-bisect search the ICMP path runs
// descends below it and the row carries the IKE figure, `prober: ike`, and the
// exchange count; and the budget of ikeExchangesPerRunMax exchanges ends the
// attempt with the ICMP figure kept and the reason named.
func TestIKEProbeConfirmsThenDescends(t *testing.T) {
	f := newFakeDeps()
	f.paths[peerA] = clampedAt(1400)
	f.paths[refV4] = clampedAt(1500)
	f.tunnels = []ipsecinventory.Tunnel{gcmTunnel("site-a", peerA)}
	f.install(t)

	// The path carries 1300 over UDP where ICMP measured 1400.
	answers := &ikeAnswers{fitsUpTo: 1300}
	registerFakeIKEProber(t, answers.probe)
	doc := showMTU(t)
	rows := rowsOf(t, doc, fieldMeasurements)
	if got := text(t, rows[0], fieldProber); got != "ike" {
		t.Errorf("prober %q, want ike", got)
	}
	if got := number(t, rows[0], fieldPathMTU); got != 1300 {
		t.Errorf("path MTU %v, want the IKE figure 1300", got)
	}
	if got := number(t, rows[0], fieldIKEConfirmed); got != 1300 {
		t.Errorf("ike-confirmed %v, want 1300", got)
	}
	if _, declined := rows[0][fieldIKEDeclined]; declined {
		t.Errorf("the row says the IKE prober declined: %v", rows[0][fieldIKEDeclined])
	}
	if got := largestAsked(answers.asked); got != 1400 {
		t.Errorf("the largest exchange was %d octets, want the ICMP figure 1400 and nothing above it", got)
	}
	if answers.asked[0].WireOctets != 1400 {
		t.Errorf("the first exchange was %d octets, want the ICMP figure first", answers.asked[0].WireOctets)
	}
	if got := number(t, rows[0], fieldExchanges); got != float64(len(answers.asked)) {
		t.Errorf("exchanges %v, want the %d exchanges spent", got, len(answers.asked))
	}
	if len(answers.asked) > ikeExchangesPerRunMax {
		t.Errorf("%d exchanges spent, above the budget of %d", len(answers.asked), ikeExchangesPerRunMax)
	}
	if got := number(t, rows[0], fieldProbes); got != float64(len(f.probers[0].sent)) {
		t.Errorf("probes %v, want the %d ICMP probes only", got, len(f.probers[0].sent))
	}
	if !hasNoteContaining(t, doc, "1300 octets where ICMP measured 1400") {
		t.Errorf("no note names the two figures: %v", noteTexts(t, doc))
	}

	// Nothing fits: the budget ends the attempt and the ICMP figure stands.
	answers = &ikeAnswers{fitsUpTo: 0}
	registerFakeIKEProber(t, answers.probe)
	doc = showMTU(t)
	rows = rowsOf(t, doc, fieldMeasurements)
	if got := text(t, rows[0], fieldProber); got != "icmp" {
		t.Errorf("after the budget the prober is %q, want icmp", got)
	}
	if got := number(t, rows[0], fieldPathMTU); got != 1400 {
		t.Errorf("after the budget the path MTU is %v, want the ICMP figure 1400", got)
	}
	if got := text(t, rows[0], fieldIKEDeclined); !strings.Contains(got, "budget of 16") {
		t.Errorf("ike-declined %q, want the budget named", got)
	}
	if len(answers.asked) != ikeExchangesPerRunMax {
		t.Errorf("%d exchanges spent, want exactly the budget of %d", len(answers.asked), ikeExchangesPerRunMax)
	}
	if got := largestAsked(answers.asked); got != 1400 {
		t.Errorf("the largest exchange was %d octets, want nothing above the ICMP figure 1400", got)
	}
	if got := number(t, rows[0], fieldExchanges); got != ikeExchangesPerRunMax {
		t.Errorf("exchanges %v, want %d", got, ikeExchangesPerRunMax)
	}
}

// TestIKEProbeRetriesTooBigBeforeBelievingIt pins AC-3 and the probes-per-size
// boundary: one too-big can be a lost DF copy, so the same size is asked again
// and fits on the retry (two too-bigs then a fit passes), while the third
// too-big is believed and the size fails (RFC 4821 Section 7.6.4, RFC 8899
// Section 5.1.3).
func TestIKEProbeRetriesTooBigBeforeBelievingIt(t *testing.T) {
	f := newFakeDeps()
	f.paths[peerA] = clampedAt(1400)
	f.paths[refV4] = clampedAt(1500)
	f.tunnels = []ipsecinventory.Tunnel{gcmTunnel("site-a", peerA)}
	f.install(t)

	answers := &ikeAnswers{fitsUpTo: 1400, tooBigFirst: probesPerSizeMax - 1}
	registerFakeIKEProber(t, answers.probe)
	doc := showMTU(t)
	rows := rowsOf(t, doc, fieldMeasurements)
	if got := text(t, rows[0], fieldProber); got != "ike" {
		t.Errorf("after two too-bigs and a fit the prober is %q, want ike", got)
	}
	if len(answers.asked) != probesPerSizeMax {
		t.Errorf("%d exchanges at 1400, want %d: two too-bigs retried, the fit believed", len(answers.asked), probesPerSizeMax)
	}

	answers = &ikeAnswers{fitsUpTo: 1400, tooBigFirst: probesPerSizeMax, tooBigAt: 1400}
	registerFakeIKEProber(t, answers.probe)
	doc = showMTU(t)
	rows = rowsOf(t, doc, fieldMeasurements)
	if answers.seen[1400] != probesPerSizeMax {
		t.Errorf("1400 was asked %d times, want %d before the too-big is believed", answers.seen[1400], probesPerSizeMax)
	}
	if got := largestAsked(answers.asked[probesPerSizeMax:]); got >= 1400 {
		t.Errorf("after %d too-bigs the search asked %d octets, want the descent below 1400", probesPerSizeMax, got)
	}
	if got := text(t, rows[0], fieldProber); got != "ike" {
		t.Errorf("the descended figure's prober is %q, want ike", got)
	}
}

// TestIKEProbeUnregisteredIsNotSilence pins AC-9's half of the leaf: a build
// with no prober registered selects ICMP with the leaf's own reason, spends no
// exchange, and the reason is never read as a silent or an unanswered probe.
func TestIKEProbeUnregisteredIsNotSilence(t *testing.T) {
	f := newFakeDeps()
	f.paths[peerA] = clampedAt(1400)
	f.paths[refV4] = clampedAt(1500)
	f.tunnels = []ipsecinventory.Tunnel{gcmTunnel("site-a", peerA)}
	f.install(t)
	ikeprobe.ResetForTest()

	doc := showMTU(t)
	rows := rowsOf(t, doc, fieldMeasurements)
	if got := text(t, rows[0], fieldProber); got != "icmp" {
		t.Errorf("prober %q, want icmp", got)
	}
	reason := text(t, rows[0], fieldIKEDeclined)
	if !strings.Contains(reason, "not in this build") {
		t.Errorf("ike-declined %q, want the leaf's own reason", reason)
	}
	for _, word := range []string{"silent", "unanswered", "sa-failed"} {
		if strings.Contains(reason, word) {
			t.Errorf("ike-declined %q reads as %s", reason, word)
		}
	}
	if got := number(t, rows[0], fieldExchanges); got != 0 {
		t.Errorf("exchanges %v, want 0 spent on an empty leaf", got)
	}
	if got := number(t, rows[0], fieldProbes); got != float64(len(f.probers[0].sent)) {
		t.Errorf("probes %v, want the %d ICMP probes only", got, len(f.probers[0].sent))
	}
}

// TestIKEProbeSAFailedEndsTheRun pins AC-13 on the MTU side: an sa-failed
// outcome ends the IKE attempt for that tunnel at the first size
// (ikeProbeStopAtFirstSilence), no second size is asked, the row keeps the
// ICMP figure with `prober: icmp` and an ike-declined naming sa-failed and the
// size, a caution note says the figure is unconfirmed, and the other tunnel's
// IKE attempt still runs.
func TestIKEProbeSAFailedEndsTheRun(t *testing.T) {
	f := newFakeDeps()
	f.paths[peerA] = clampedAt(1400)
	f.paths[peerB] = clampedAt(1300)
	f.paths[refV4] = clampedAt(1500)
	f.tunnels = []ipsecinventory.Tunnel{gcmTunnel("site-a", peerA), gcmTunnel("site-b", peerB)}
	f.install(t)
	var asked []ikeprobe.Request
	registerFakeIKEProber(t, func(_ context.Context, req ikeprobe.Request) (ikeprobe.Result, error) {
		asked = append(asked, req)
		if req.Peer == "site-a" {
			return ikeprobe.Result{Outcome: ikeprobe.OutcomeSAFailed, WireOctets: req.WireOctets}, nil
		}
		return ikeprobe.Result{Outcome: ikeprobe.OutcomeFits, WireOctets: req.WireOctets}, nil
	})

	doc := showMTU(t)
	rows := rowsOf(t, doc, fieldMeasurements)
	if got := text(t, rows[0], fieldProber); got != "icmp" {
		t.Errorf("the failed tunnel's prober is %q, want icmp", got)
	}
	if got := number(t, rows[0], fieldPathMTU); got != 1400 {
		t.Errorf("the failed tunnel's path MTU is %v, want the ICMP figure 1400", got)
	}
	reason := text(t, rows[0], fieldIKEDeclined)
	if !strings.Contains(reason, "sa-failed") || !strings.Contains(reason, "1400") {
		t.Errorf("ike-declined %q, want sa-failed and the size 1400", reason)
	}
	if got := number(t, rows[0], fieldExchanges); got != 1 {
		t.Errorf("exchanges %v, want the one that failed the SA", got)
	}
	siteA := 0
	for _, req := range asked {
		if req.Peer == "site-a" {
			siteA++
		}
	}
	if siteA != 1 {
		t.Errorf("site-a was asked %d times (%+v), want once: sa-failed ends the attempt", siteA, asked)
	}
	if !hasNoteContaining(t, doc, peerA.String()+" keeps its ICMP figure, unconfirmed: sa-failed at 1400 octets") {
		t.Errorf("no caution note names the unconfirmed figure and the size: %v", noteTexts(t, doc))
	}
	if got := text(t, rows[1], fieldProber); got != "ike" {
		t.Errorf("the second tunnel's prober is %q, want ike: one failed SA ends only its own attempt", got)
	}
}

// TestIKEProbeNeverExceedsTheCeiling pins AC-6 on the MTU side: an ICMP figure
// above ikeWireMax (RFC 7296 Section 2's 3000) is not offered to the IKE
// prober at all, the reason names the ceiling, and no exchange is spent.
func TestIKEProbeNeverExceedsTheCeiling(t *testing.T) {
	f := newFakeDeps()
	f.underlayMTU = 9000
	f.paths[peerA] = &fakePath{overhead: icmpOverheadIPv4, ifaceMTU: 9000, pathMTU: 3200, reportsMTU: true}
	f.paths[refV4] = clampedAt(1500)
	f.tunnels = []ipsecinventory.Tunnel{gcmTunnel("site-a", peerA)}
	f.install(t)
	answers := &ikeAnswers{fitsUpTo: 9000}
	registerFakeIKEProber(t, answers.probe)

	doc := showMTU(t)
	rows := rowsOf(t, doc, fieldMeasurements)
	if len(answers.asked) != 0 {
		t.Errorf("the IKE prober was asked %+v, want nothing above %d octets", answers.asked, ikeWireMax)
	}
	if got := text(t, rows[0], fieldProber); got != "icmp" {
		t.Errorf("prober %q, want icmp", got)
	}
	if got := number(t, rows[0], fieldPathMTU); got != 3200 {
		t.Errorf("path MTU %v, want the ICMP figure 3200", got)
	}
	if got := text(t, rows[0], fieldIKEDeclined); !strings.Contains(got, "3000") {
		t.Errorf("ike-declined %q, want the ceiling named", got)
	}
}

// TestMeasurementRowNamesTheProber pins the payload rules: every measurement
// row carries prober and its own caveats; the ICMP row carries the ICMP
// optimism caveat; the IKE row over a NAT-T SA carries none; the IKE row
// without UDP encapsulation carries the different-path caveat (UDP/500 against
// protocol 50); and the run's caveats carry the ICMP caveat only while an ICMP
// figure is in the payload.
func TestMeasurementRowNamesTheProber(t *testing.T) {
	f := newFakeDeps()
	f.paths[peerA] = clampedAt(1400)
	f.paths[peerB] = clampedAt(1300)
	f.paths[refV4] = clampedAt(1500)
	natT := gcmTunnel("site-b", peerB)
	natT.UDPEncap = true
	f.tunnels = []ipsecinventory.Tunnel{gcmTunnel("site-a", peerA), natT}
	f.install(t)
	answers := &ikeAnswers{fitsUpTo: 1500}
	registerFakeIKEProber(t, answers.probe)

	doc := showMTU(t)
	rows := rowsOf(t, doc, fieldMeasurements)
	for i, row := range rows {
		if got := text(t, row, fieldProber); got != "ike" {
			t.Errorf("row %d prober %q, want ike", i, got)
		}
	}
	plain := stringsOf(t, rows[0], fieldCaveats)
	if len(plain) != 1 || !strings.Contains(plain[0], "UDP/500") {
		t.Errorf("the non-NAT IKE row's caveats are %v, want the different-path caveat alone", plain)
	}
	for _, caveat := range plain {
		if strings.Contains(caveat, "ICMP") {
			t.Errorf("the IKE row carries the ICMP caveat: %q", caveat)
		}
	}
	if encap := stringsOf(t, rows[1], fieldCaveats); len(encap) != 0 {
		t.Errorf("the NAT-T IKE row's caveats are %v, want none", encap)
	}
	// The reference is ICMP-measured, so the run still carries the ICMP caveat.
	if run := stringsOf(t, doc, fieldCaveats); len(run) != 1 || !strings.Contains(run[0], "ICMP") {
		t.Errorf("the run's caveats are %v, want the ICMP caveat for the reference", run)
	}

	// A host run is ICMP only: the row and the run both carry the ICMP caveat.
	f.paths[netip.MustParseAddr("203.0.113.9")] = clampedAt(1400)
	doc = showMTU(t, "host", "203.0.113.9")
	rows = rowsOf(t, doc, fieldMeasurements)
	if got := text(t, rows[0], fieldProber); got != "icmp" {
		t.Errorf("the host row's prober is %q, want icmp", got)
	}
	if got := stringsOf(t, rows[0], fieldCaveats); len(got) != 1 || !strings.Contains(got[0], "ICMP") {
		t.Errorf("the host row's caveats are %v, want the ICMP caveat", got)
	}
}

// TestIKEExchangeBudgetBoundary pins the exchanges-per-run boundary on the
// adapter itself: ikeExchangesPerRunMax exchanges are asked, the next is
// refused with errIKEExchangeBudget before the leaf is asked, and the count is
// what the adapter reports.
func TestIKEExchangeBudgetBoundary(t *testing.T) {
	answers := &ikeAnswers{fitsUpTo: 1400}
	p := &ikeProber{peer: "site-a", overhead: icmpOverheadIPv4, df: probe.DFHonorCache, ceiling: 1400, ask: answers.probe}
	for i := range ikeExchangesPerRunMax {
		if _, err := p.probe(context.Background(), 1400-icmpOverheadIPv4); err != nil {
			t.Fatalf("exchange %d: %v", i+1, err)
		}
	}
	_, err := p.probe(context.Background(), 1400-icmpOverheadIPv4)
	if !errors.Is(err, errIKEExchangeBudget) {
		t.Errorf("exchange %d answered %v, want errIKEExchangeBudget", ikeExchangesPerRunMax+1, err)
	}
	if len(answers.asked) != ikeExchangesPerRunMax {
		t.Errorf("the leaf was asked %d times, want %d: the refusal happens before the ask", len(answers.asked), ikeExchangesPerRunMax)
	}
	if p.exchanges != ikeExchangesPerRunMax {
		t.Errorf("the adapter counts %d exchanges, want %d", p.exchanges, ikeExchangesPerRunMax)
	}
}

// TestIKEProbeRekeyedIsRetriedAtTheSameSize pins AC-10 on the module side: an
// exchange the peer's IKE rekey retired is answered `refused: rekeyed` with the
// size it sent, and that is neither a refusal that ends the search nor an answer
// about the path. The same size is asked again on the new SA, the fit confirms the
// ICMP figure, both exchanges are counted, and the row says nothing declined.
func TestIKEProbeRekeyedIsRetriedAtTheSameSize(t *testing.T) {
	f := newFakeDeps()
	f.paths[peerA] = clampedAt(1400)
	f.paths[refV4] = clampedAt(1500)
	f.tunnels = []ipsecinventory.Tunnel{gcmTunnel("site-a", peerA)}
	f.install(t)
	var asked []ikeprobe.Request
	registerFakeIKEProber(t, func(_ context.Context, req ikeprobe.Request) (ikeprobe.Result, error) {
		asked = append(asked, req)
		if len(asked) == 1 {
			return ikeprobe.Result{Outcome: ikeprobe.OutcomeRefused, Refusal: ikeprobe.RefusalRekeyed, WireOctets: req.WireOctets}, nil
		}
		return ikeprobe.Result{Outcome: ikeprobe.OutcomeFits, WireOctets: req.WireOctets}, nil
	})

	doc := showMTU(t)
	rows := rowsOf(t, doc, fieldMeasurements)
	if got := text(t, rows[0], fieldProber); got != "ike" {
		t.Errorf("prober %q, want ike: %v", got, rows[0])
	}
	if _, declined := rows[0][fieldIKEDeclined]; declined {
		t.Errorf("the row says the IKE prober declined across the rekey: %v", rows[0][fieldIKEDeclined])
	}
	if got := number(t, rows[0], fieldPathMTU); got != 1400 {
		t.Errorf("path MTU %v, want the ICMP figure 1400 confirmed on the retry", got)
	}
	if len(asked) != 2 || asked[0].WireOctets != 1400 || asked[1].WireOctets != 1400 {
		t.Fatalf("the IKE prober was asked %+v, want 1400 twice: the retired exchange and its retry", asked)
	}
	if got := number(t, rows[0], fieldExchanges); got != 2 {
		t.Errorf("exchanges %v, want 2: the retired exchange counts", got)
	}
}
