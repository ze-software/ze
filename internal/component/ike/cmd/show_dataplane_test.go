// VALIDATES: the three `show vpn ipsec dataplane` handlers -- their registration,
// the JSON keys they emit, the one-direction drift comparison, and the rule that
// a dataplane that cannot be read is an ERROR and never an empty table.
// PREVENTS: a dump rendering "no SAs installed" when the truth is that nobody
// asked the kernel.
//
// Design: docs/architecture/ike/ipsec-dataplane-inspection.md -- kernel dataplane read surface
// Related: show_dataplane.go -- the handlers these tests drive

package cmd

import (
	"encoding/json"
	"errors"
	"net"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/engine"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestShowDataplaneRegistered is the wiring test: the three dataplane wire
// methods must reach a non-nil handler through the same registration path the
// CLI dispatches on. A command that is not here is unreachable however complete
// its body is.
func TestShowDataplaneRegistered(t *testing.T) {
	for _, wire := range []string{
		"ze-show:vpn-ipsec-dataplane-sa",
		"ze-show:vpn-ipsec-dataplane-policy",
		"ze-show:vpn-ipsec-dataplane-drift",
	} {
		t.Run(wire, func(t *testing.T) {
			var found bool
			for _, r := range pluginserver.AllBuiltinRPCs() {
				if r.WireMethod == wire {
					require.NotNil(t, r.Handler, "%s handler must not be nil", wire)
					found = true
				}
			}
			require.True(t, found, "%s not registered via pluginserver.RegisterRPCs", wire)
		})
	}
}

// fakeDataplane is a dataplane whose reads are scripted. It exists so the
// handlers can be driven without a kernel and without CAP_NET_ADMIN.
type fakeDataplane struct {
	sas      []dataplane.SAInfo
	policies []dataplane.PolicyInfo
	saErr    error
	polErr   error
	onSAD    func()
}

func (f *fakeDataplane) InstallSA(dataplane.SAParams) error          { return nil }
func (f *fakeDataplane) RemoveSA(uint32, net.IP, uint8) error        { return nil }
func (f *fakeDataplane) InstallPolicy(dataplane.SPParams) error      { return nil }
func (f *fakeDataplane) RemovePolicyParams(dataplane.SPParams) error { return nil }
func (f *fakeDataplane) Close() error                                { return nil }

func (f *fakeDataplane) RemovePolicy(*net.IPNet, *net.IPNet, dataplane.SADir) error { return nil }

func (f *fakeDataplane) ListSAs(uint32) ([]dataplane.SAInfo, error) {
	if f.onSAD != nil {
		f.onSAD()
	}
	return f.sas, f.saErr
}

func (f *fakeDataplane) ListPolicies() ([]dataplane.PolicyInfo, error) {
	return f.policies, f.polErr
}

var fakeBackendSeq int

// useDataplane makes fake the active backend for one test.
func useDataplane(t *testing.T, fake *fakeDataplane) {
	t.Helper()
	fakeBackendSeq++
	name := "test-fake-" + strconv.Itoa(fakeBackendSeq)
	require.NoError(t, dataplane.Register(name, func() (dataplane.Dataplane, error) { return fake, nil }))
	require.NoError(t, dataplane.Load(name))
	t.Cleanup(func() { _ = dataplane.CloseBackend() })
}

// noDataplane clears the active backend for one test.
func noDataplane(t *testing.T) {
	t.Helper()
	require.NoError(t, dataplane.CloseBackend())
}

// usePeerInfo scripts the engine-belief half of the drift comparison.
func usePeerInfo(t *testing.T, peers map[string]engine.PeerInfo) {
	t.Helper()
	for name, info := range peers {
		if info.ChildInID.SPI == 0 {
			info.ChildInID = dataplane.IdentityOf(info.ChildInSPI, nil, 0, info.ChildIfID)
		}
		if info.ChildOutID.SPI == 0 {
			info.ChildOutID = dataplane.IdentityOf(info.ChildOutSPI, nil, 0, info.ChildIfID)
		}
		peers[name] = info
	}
	old := observeDataplane
	observeDataplane = func() (engine.DataplaneSnapshot, error) {
		dp := dataplane.Get()
		if dp == nil {
			return engine.DataplaneSnapshot{Peers: peers}, dataplane.ErrNotRegistered
		}
		sas, err := dp.ListSAs(0)
		return engine.DataplaneSnapshot{Peers: peers, SAs: sas}, err
	}
	t.Cleanup(func() { observeDataplane = old })
}

func responseMap(t *testing.T, resp *plugin.Response) plugin.Map {
	t.Helper()
	m, ok := resp.Data.(plugin.Map)
	require.True(t, ok, "response data is %T, want plugin.Map", resp.Data)
	return m
}

func rowsOf(t *testing.T, resp *plugin.Response, key string) []map[string]any {
	t.Helper()
	rows, ok := responseMap(t, resp)[key].([]map[string]any)
	require.True(t, ok, "response has no %q rows", key)
	return rows
}

func TestShowDataplaneSARendersFields(t *testing.T) {
	useDataplane(t, &fakeDataplane{sas: []dataplane.SAInfo{{
		SPI:               0xc0ffee,
		Src:               net.ParseIP("10.0.0.1"),
		Dst:               net.ParseIP("10.0.0.2"),
		IfID:              7,
		Proto:             50,
		Mode:              dataplane.ModeTunnel,
		ReqID:             42,
		Encryption:        "cbc(aes)",
		EncryptionKeyBits: 256,
		Integrity:         "hmac(sha256)",
		IntegrityKeyBits:  256,
		ReplayWindow:      64,
		BytesCurrent:      4096,
		PacketsCurrent:    12,
		BytesHard:         1 << 40,
		PacketsHard:       9000,
		AddedAt:           time.Unix(1700000000, 0).UTC(),
		UsedAt:            time.Unix(1700000500, 0).UTC(),
	}}})

	resp, err := handleShowVPNIPsecDataplaneSA(nil, nil)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, resp.Status)

	rows := rowsOf(t, resp, "sas")
	require.Len(t, rows, 1)
	row := rows[0]

	// AC-1 names each of these. The keys are kebab-case to match the existing
	// `show vpn ipsec sa` style.
	require.Equal(t, uint32(0xc0ffee), row["spi"])
	require.Equal(t, "10.0.0.1", row["src"])
	require.Equal(t, "10.0.0.2", row["dst"])
	require.Equal(t, uint32(7), row["if-id"])
	require.Equal(t, "esp", row["proto"])
	require.Equal(t, "tunnel", row["mode"])
	require.Equal(t, uint32(42), row["reqid"])
	require.Equal(t, "cbc(aes)", row["encryption"])
	require.Equal(t, 256, row["encryption-keybits"])
	require.Equal(t, "hmac(sha256)", row["integrity"])
	require.Equal(t, 256, row["integrity-keybits"])
	require.Equal(t, uint32(64), row["replay-window"])
	require.Equal(t, uint64(4096), row["bytes"])
	require.Equal(t, uint64(12), row["packets"])
	require.Equal(t, uint64(1)<<40, row["bytes-hard"])
	require.Equal(t, uint64(9000), row["packets-hard"])
	require.Equal(t, "2023-11-14T22:13:20Z", row["added-at"])
	require.Equal(t, "2023-11-14T22:21:40Z", row["used-at"])
}

func TestShowDataplaneSASelectsOneSPI(t *testing.T) {
	useDataplane(t, &fakeDataplane{sas: []dataplane.SAInfo{
		{SPI: 100}, {SPI: 200}, {SPI: 300},
	}})

	resp, err := handleShowVPNIPsecDataplaneSA(nil, []string{"spi", "200"})
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, resp.Status)

	rows := rowsOf(t, resp, "sas")
	require.Len(t, rows, 1)
	require.Equal(t, uint32(200), rows[0]["spi"])
}

// TestShowDataplaneSARejectsSPIZero pins RFC 4303 Section 2.1: SPI 0 is reserved
// and never names an installed SA. Treating it as "every SPI" would make a typo
// look like a successful full dump.
func TestShowDataplaneSARejectsSPIZero(t *testing.T) {
	useDataplane(t, &fakeDataplane{sas: []dataplane.SAInfo{{SPI: 100}}})

	resp, err := handleShowVPNIPsecDataplaneSA(nil, []string{"spi", "0"})
	require.NoError(t, err)
	require.Equal(t, plugin.StatusError, resp.Status)
	require.Contains(t, resp.Error, "RFC 4303")
	require.Nil(t, resp.Data, "a refusal must carry no rows")
}

func TestShowDataplaneSARejectsSPIAboveRange(t *testing.T) {
	useDataplane(t, &fakeDataplane{})

	for _, raw := range []string{"4294967296", "-1", "not-a-number"} {
		resp, err := handleShowVPNIPsecDataplaneSA(nil, []string{"spi", raw})
		require.NoError(t, err)
		require.Equal(t, plugin.StatusError, resp.Status, "spi %q must be refused", raw)
	}
}

func TestShowDataplaneSAAcceptsMaxSPI(t *testing.T) {
	useDataplane(t, &fakeDataplane{sas: []dataplane.SAInfo{{SPI: ^uint32(0)}}})

	resp, err := handleShowVPNIPsecDataplaneSA(nil, []string{"spi", "4294967295"})
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, resp.Status)
	rows := rowsOf(t, resp, "sas")
	require.Len(t, rows, 1)
	require.Equal(t, ^uint32(0), rows[0]["spi"])
}

// TestShowDataplaneUnsupportedBackendIsError is AC-6. A backend that cannot
// enumerate must say so. An empty table would read as "nothing is installed".
func TestShowDataplaneUnsupportedBackendIsError(t *testing.T) {
	useDataplane(t, &fakeDataplane{
		saErr:  dataplane.ErrNotSupported,
		polErr: dataplane.ErrNotSupported,
	})

	for name, handler := range map[string]func(*pluginserver.CommandContext, []string) (*plugin.Response, error){
		"sa":     handleShowVPNIPsecDataplaneSA,
		"policy": handleShowVPNIPsecDataplanePolicy,
		"drift":  handleShowVPNIPsecDataplaneDrift,
	} {
		t.Run(name, func(t *testing.T) {
			resp, err := handler(nil, nil)
			require.NoError(t, err)
			require.Equal(t, plugin.StatusError, resp.Status)
			require.Contains(t, resp.Error, "cannot enumerate")
			require.Nil(t, resp.Data, "an unsupported backend must not also return rows")
		})
	}
}

// TestShowDataplanePermissionErrorNamesCapability is AC-7's EPERM half. R-2:
// a permission failure reads like a bug unless the message names the capability.
func TestShowDataplanePermissionErrorNamesCapability(t *testing.T) {
	useDataplane(t, &fakeDataplane{saErr: syscall.EPERM, polErr: syscall.EPERM})

	resp, err := handleShowVPNIPsecDataplaneSA(nil, nil)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusError, resp.Status)
	require.Contains(t, resp.Error, "CAP_NET_ADMIN")
	require.Nil(t, resp.Data)
}

// TestShowDataplaneNilBackendIsError is AC-7's other half.
func TestShowDataplaneNilBackendIsError(t *testing.T) {
	noDataplane(t)

	for name, handler := range map[string]func(*pluginserver.CommandContext, []string) (*plugin.Response, error){
		"sa":     handleShowVPNIPsecDataplaneSA,
		"policy": handleShowVPNIPsecDataplanePolicy,
		"drift":  handleShowVPNIPsecDataplaneDrift,
	} {
		t.Run(name, func(t *testing.T) {
			resp, err := handler(nil, nil)
			require.NoError(t, err)
			require.Equal(t, plugin.StatusError, resp.Status)
			require.Contains(t, resp.Error, "no ipsec dataplane backend is loaded")
			require.Nil(t, resp.Data)
		})
	}
}

func TestShowDataplanePolicyRendersFields(t *testing.T) {
	_, selector, err := net.ParseCIDR("192.168.1.0/24")
	require.NoError(t, err)
	_, remote, err := net.ParseCIDR("192.168.2.0/24")
	require.NoError(t, err)

	useDataplane(t, &fakeDataplane{policies: []dataplane.PolicyInfo{{
		Src:        selector,
		Dst:        remote,
		SrcPort:    dataplane.AnyPortMatch(),
		DstPort:    dataplane.ExactPortMatch(4500),
		Dir:        dataplane.SADirOut,
		UpperProto: 17,
		Priority:   1000,
		IfID:       3,
		TunnelSrc:  net.ParseIP("10.0.0.1"),
		TunnelDst:  net.ParseIP("10.0.0.2"),
		Mode:       dataplane.ModeTunnel,
		ReqID:      77,
		Owner:      "peer-alpha",
		OwnerKnown: true,
	}}})

	resp, err := handleShowVPNIPsecDataplanePolicy(nil, nil)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, resp.Status)

	rows := rowsOf(t, resp, "policies")
	require.Len(t, rows, 1)
	row := rows[0]

	require.Equal(t, "192.168.1.0/24", row["src"])
	require.Equal(t, "192.168.2.0/24", row["dst"])
	require.Equal(t, "any", row["src-port"])
	require.Equal(t, "4500", row["dst-port"])
	require.Equal(t, "out", row["direction"])
	require.Equal(t, uint8(17), row["upper-proto"])
	require.Equal(t, 1000, row["priority"])
	require.Equal(t, uint32(3), row["if-id"])
	require.Equal(t, "protect", row["action"])
	require.Equal(t, "tunnel", row["mode"])
	require.Equal(t, uint32(77), row["reqid"])
	require.Equal(t, "10.0.0.1", row["tunnel-src"])
	require.Equal(t, "10.0.0.2", row["tunnel-dst"])
	require.Equal(t, "peer-alpha", row["owner"])
	require.Equal(t, true, row["owner-known"])
}

// Partial masks must survive the handler and public renderers without becoming
// exact ports. Exact port zero must also remain distinct from a wildcard.
func TestShowDataplanePolicyPreservesPortMasks(t *testing.T) {
	useDataplane(t, &fakeDataplane{policies: []dataplane.PolicyInfo{
		{
			SrcPort: dataplane.PortMatch{Port: 0x1200, Mask: 0xff00},
			DstPort: dataplane.PortMatch{Port: 0x3401, Mask: 0x00f3},
			Dir:     dataplane.SADirOut,
		},
		{
			SrcPort: dataplane.ExactPortMatch(0),
			DstPort: dataplane.AnyPortMatch(),
			Dir:     dataplane.SADirOut,
		},
		{
			SrcPort: dataplane.PortMatch{Port: 500, Mask: 0},
			DstPort: dataplane.ExactPortMatch(4500),
			Dir:     dataplane.SADirOut,
		},
	}})

	resp, err := handleShowVPNIPsecDataplanePolicy(nil, nil)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, resp.Status)
	want := [][2]string{
		{"4608/0xff00", "13313/0x00f3"},
		{"0", "any"},
		{"any", "4500"},
	}
	rows := rowsOf(t, resp, "policies")
	require.Len(t, rows, len(want))
	for i, ports := range want {
		require.Equal(t, ports[0], rows[i]["src-port"], "policy %d source", i)
		require.Equal(t, ports[1], rows[i]["dst-port"], "policy %d destination", i)
	}

	payload, err := json.Marshal(resp.Data)
	require.NoError(t, err)
	_, renderJSON, errMsg := command.ProcessPipesDefaultFormatChecked("show vpn ipsec dataplane policy | json", "")
	require.Empty(t, errMsg)
	var publicRows []map[string]any
	require.NoError(t, json.Unmarshal([]byte(renderJSON(string(payload))), &publicRows))
	require.Len(t, publicRows, len(want))
	for i, ports := range want {
		require.Equal(t, ports[0], publicRows[i]["src-port"], "public policy %d source", i)
		require.Equal(t, ports[1], publicRows[i]["dst-port"], "public policy %d destination", i)
	}

	_, renderTable, errMsg := command.ProcessPipesDefaultFormatChecked("show vpn ipsec dataplane policy | table", "")
	require.Empty(t, errMsg)
	table := renderTable(string(payload))
	require.Contains(t, table, "4608/0xff00")
	require.Contains(t, table, "13313/0x00f3")
}

// TestShowDataplanePolicyUnknownOwnerSaysSo is the fail-closed rendering. A
// policy another daemon installed must not print as a blank cell that reads as
// "unowned" (ai/rules/evidence.md).
func TestShowDataplanePolicyUnknownOwnerSaysSo(t *testing.T) {
	useDataplane(t, &fakeDataplane{policies: []dataplane.PolicyInfo{{
		Dir:        dataplane.SADirIn,
		OwnerKnown: false,
	}}})

	resp, err := handleShowVPNIPsecDataplanePolicy(nil, nil)
	require.NoError(t, err)
	rows := rowsOf(t, resp, "policies")
	require.Len(t, rows, 1)
	require.Equal(t, "unknown", rows[0]["owner"])
	require.Equal(t, false, rows[0]["owner-known"])
	// A wildcard selector is named, not printed as an empty span.
	require.Equal(t, "any", rows[0]["src"])
	require.Equal(t, "any", rows[0]["dst"])
}

func TestShowDataplanePolicyBypassRendersAsBypass(t *testing.T) {
	useDataplane(t, &fakeDataplane{policies: []dataplane.PolicyInfo{{
		Dir:    dataplane.SADirOut,
		Action: dataplane.SPActionBypass,
	}}})

	resp, err := handleShowVPNIPsecDataplanePolicy(nil, nil)
	require.NoError(t, err)
	rows := rowsOf(t, resp, "policies")
	require.Equal(t, "bypass", rows[0]["action"],
		"a bypass policy passes traffic in the clear; reporting it as protect would say the opposite")
}

// TestDriftCleanReportsNothing is AC-4.
func TestDriftCleanReportsNothing(t *testing.T) {
	usePeerInfo(t, map[string]engine.PeerInfo{
		"peer-alpha": {PeerName: "peer-alpha", HasChild: true, ChildInSPI: 100, ChildOutSPI: 200},
	})

	useDataplane(t, &fakeDataplane{sas: []dataplane.SAInfo{{SPI: 100}, {SPI: 200}}})

	resp, err := handleShowVPNIPsecDataplaneDrift(nil, nil)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, resp.Status)
	require.Empty(t, rowsOf(t, resp, "drift"))
}

// TestDriftReportsMissingSPI is AC-3: the engine believes an SPI the kernel does
// not hold, so the command names peer, SPI and direction, and exits non-zero.
func TestDriftReportsMissingSPI(t *testing.T) {
	usePeerInfo(t, map[string]engine.PeerInfo{
		"peer-alpha": {PeerName: "peer-alpha", HasChild: true, ChildInSPI: 100, ChildOutSPI: 200},
	})

	// The kernel holds the inbound SA only. The outbound one is gone.
	useDataplane(t, &fakeDataplane{sas: []dataplane.SAInfo{{SPI: 100}}})

	resp, err := handleShowVPNIPsecDataplaneDrift(nil, nil)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusError, resp.Status,
		"drift must exit non-zero so a script can test it")
	require.Contains(t, resp.Error, "peer-alpha")
	require.Contains(t, resp.Error, "outbound")
	require.Contains(t, resp.Error, "200")
	require.NotContains(t, resp.Error, "inbound",
		"the inbound SA is present, so it is not drift")
}

// TestDriftSilentDuringRekeyWindow is R-3, and it is the reason the comparison
// runs in one direction only.
//
// RFC 7296 Section 2.8: the old and the new Child SA coexist until the old one
// is deleted. The kernel therefore holds SPIs the engine no longer names, and a
// set-EQUALITY comparison would report drift on every rekey.
func TestDriftSilentDuringRekeyWindow(t *testing.T) {
	usePeerInfo(t, map[string]engine.PeerInfo{
		"peer-alpha": {PeerName: "peer-alpha", HasChild: true, ChildInSPI: 300, ChildOutSPI: 400},
	})

	// Both the retired pair (100/200) and the replacement pair (300/400) are in
	// the kernel, which is what a rekey window looks like.
	useDataplane(t, &fakeDataplane{sas: []dataplane.SAInfo{
		{SPI: 100}, {SPI: 200}, {SPI: 300}, {SPI: 400},
	}})

	resp, err := handleShowVPNIPsecDataplaneDrift(nil, nil)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, resp.Status,
		"a kernel that holds MORE than the engine expects is a rekey window, not drift")
	require.Empty(t, rowsOf(t, resp, "drift"))
}

// TestDriftIgnoresPeersWithNoChild keeps a peer whose IKE SA is up but whose
// Child SA has not been negotiated out of the report. It has no SPI to miss.
func TestDriftIgnoresPeersWithNoChild(t *testing.T) {
	usePeerInfo(t, map[string]engine.PeerInfo{
		"peer-alpha": {PeerName: "peer-alpha", HasChild: false, ChildInSPI: 100},
	})

	useDataplane(t, &fakeDataplane{sas: nil})

	resp, err := handleShowVPNIPsecDataplaneDrift(nil, nil)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, resp.Status)
}

// TestDriftNamesEveryDriftingPeer proves the message carries every finding, not
// just the first. The dispatcher discards Data on an error response, so a fact
// missing from the message is a fact the operator never sees.
func TestDriftNamesEveryDriftingPeer(t *testing.T) {
	usePeerInfo(t, map[string]engine.PeerInfo{
		"peer-alpha": {PeerName: "peer-alpha", HasChild: true, ChildInSPI: 100, ChildOutSPI: 200},
		"peer-beta":  {PeerName: "peer-beta", HasChild: true, ChildInSPI: 300, ChildOutSPI: 400},
	})

	useDataplane(t, &fakeDataplane{sas: nil})

	resp, err := handleShowVPNIPsecDataplaneDrift(nil, nil)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusError, resp.Status)
	for _, want := range []string{"peer-alpha", "peer-beta", "100", "200", "300", "400"} {
		require.Contains(t, resp.Error, want)
	}
	require.Equal(t, 1, strings.Count(resp.Error, "ipsec dataplane drift"),
		"one summary line, then one clause per finding")
}

// TestSAToMapCarriesCounters is AC-8: the `show vpn ipsec sa` child object
// reports the byte and packet counters its YANG description has advertised since
// 2026-06-03, sourced from the kernel SAD.
func TestSAToMapCarriesCounters(t *testing.T) {
	peers := map[string]engine.PeerInfo{
		"peer-alpha": {
			PeerName:    "peer-alpha",
			HasChild:    true,
			ChildInSPI:  100,
			ChildOutSPI: 200,
			ChildInID:   dataplane.IdentityOf(100, nil, 0, 0),
			ChildOutID:  dataplane.IdentityOf(200, nil, 0, 0),
		},
	}
	kernel := sadCounters{known: true, byID: map[dataplane.SAIdentity]dataplane.SAInfo{
		{SPI: 100}: {SPI: 100, BytesCurrent: 4096, PacketsCurrent: 12},
		{SPI: 200}: {SPI: 200, BytesCurrent: 8192, PacketsCurrent: 24},
	}}

	row := saToMap(&engine.SA{PeerName: "peer-alpha", State: engine.StateEstablished},
		time.Now(), peers, kernel)

	child, ok := row["child-sa"].(map[string]any)
	require.True(t, ok, "no child-sa object")
	require.Equal(t, uint64(4096), child["bytes-in"])
	require.Equal(t, uint64(12), child["packets-in"])
	require.Equal(t, uint64(8192), child["bytes-out"])
	require.Equal(t, uint64(24), child["packets-out"])
	require.Equal(t, true, child["counters-known"])
}

// TestSAToMapCountersUnknownWhenSADUnreadable is the fail-closed half of AC-8.
//
// The noop backend and an unprivileged process both leave the SAD unreadable.
// Reporting zero there would say the tunnel has carried no traffic, which is a
// wrong answer rather than a missing one (ai/rules/evidence.md). The keys stay
// present and carry null, and counters-known says why.
func TestSAToMapCountersUnknownWhenSADUnreadable(t *testing.T) {
	peers := map[string]engine.PeerInfo{
		"peer-alpha": {PeerName: "peer-alpha", HasChild: true, ChildInSPI: 100, ChildOutSPI: 200},
	}

	row := saToMap(&engine.SA{PeerName: "peer-alpha", State: engine.StateEstablished},
		time.Now(), peers, sadCounters{})

	child, ok := row["child-sa"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, false, child["counters-known"])
	for _, key := range []string{"bytes-in", "bytes-out", "packets-in", "packets-out"} {
		require.Contains(t, child, key, "%s must still be present", key)
		require.Nil(t, child[key], "%s must be null, never 0, when the SAD could not be read", key)
	}
}

// TestSAToMapCountersUnknownWhenSPIAbsent covers the drift case: the SAD was
// read, and it does not hold this child's SPI. The counters are unknown for that
// SA specifically, and zero would claim it carried nothing.
func TestSAToMapCountersUnknownWhenSPIAbsent(t *testing.T) {
	peers := map[string]engine.PeerInfo{
		"peer-alpha": {PeerName: "peer-alpha", HasChild: true, ChildInSPI: 100, ChildOutSPI: 200,
			ChildInID: dataplane.IdentityOf(100, nil, 0, 0), ChildOutID: dataplane.IdentityOf(200, nil, 0, 0)},
	}
	kernel := sadCounters{known: true, byID: map[dataplane.SAIdentity]dataplane.SAInfo{
		{SPI: 100}: {SPI: 100, BytesCurrent: 4096, PacketsCurrent: 12},
	}}

	row := saToMap(&engine.SA{PeerName: "peer-alpha", State: engine.StateEstablished},
		time.Now(), peers, kernel)

	child, ok := row["child-sa"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, uint64(4096), child["bytes-in"])
	require.Nil(t, child["bytes-out"], "the kernel does not hold the outbound SPI, so its counter is unknown")
	require.Nil(t, child["packets-out"])
	require.Equal(t, true, child["counters-known"],
		"the SAD WAS read; an SPI missing from it is drift, not an unread dataplane")
}

// TestReadSADCountersUnreadableIsNotKnown drives readSADCounters itself, which
// is where the dump error is classified.
//
// Without this the error arm is unreachable from a test: an unreadable SAD and a
// successfully-read empty SAD both produce no counters, so the two are
// indistinguishable downstream. They are NOT the same answer, and `known` is
// what keeps them apart.
func TestReadSADCountersUnreadableIsNotKnown(t *testing.T) {
	useDataplane(t, &fakeDataplane{saErr: dataplane.ErrNotSupported})
	_, counters := readSADCounters()
	require.False(t, counters.known,
		"a dump that failed must not be recorded as a successful read of an empty SAD")

	useDataplane(t, &fakeDataplane{sas: nil})
	_, counters = readSADCounters()
	require.True(t, counters.known,
		"a dump that succeeded and returned nothing IS a known, empty SAD")
}

func TestReadSADCountersNoBackendIsNotKnown(t *testing.T) {
	noDataplane(t)
	_, counters := readSADCounters()
	require.False(t, counters.known)
}

// TestDataplaneReadErrorKeepsCausesDistinct proves the three failure causes do
// not collapse into one message. An operator fixes each differently.
func TestDataplaneReadErrorKeepsCausesDistinct(t *testing.T) {
	unsupported := dataplaneReadError("SAD", dataplane.ErrNotSupported)
	perm := dataplaneReadError("SAD", syscall.EPERM)
	other := dataplaneReadError("SAD", errors.New("netlink socket closed"))

	require.Contains(t, unsupported.Error, "cannot enumerate")
	require.Contains(t, perm.Error, "CAP_NET_ADMIN")
	require.Contains(t, other.Error, "netlink socket closed")
	require.NotContains(t, other.Error, "CAP_NET_ADMIN")
	require.NotContains(t, unsupported.Error, "CAP_NET_ADMIN")
}

// The same SPI on another destination must keep its own counters and cannot
// conceal deletion of the expected state.
func TestShowIPsecSameSPIKeepsCountersAndDriftSeparate(t *testing.T) {
	local := net.ParseIP("192.0.2.1")
	remote := net.ParseIP("192.0.2.2")
	other := net.ParseIP("192.0.2.3")
	usePeerInfo(t, map[string]engine.PeerInfo{
		"peer-alpha": {
			HasChild: true, ChildInSPI: 100, ChildOutSPI: 200, ChildIfID: 7,
			ChildInID:  dataplane.IdentityOf(100, local, dataplane.ProtoESP, 7),
			ChildOutID: dataplane.IdentityOf(200, remote, dataplane.ProtoESP, 7),
		},
	})
	table := engine.NewSATable()
	table.Insert(&engine.SA{PeerName: "peer-alpha", State: engine.StateEstablished})
	engine.SetActiveTableForTest(table)
	t.Cleanup(func() { engine.SetActiveTableForTest(nil) })
	fake := &fakeDataplane{sas: []dataplane.SAInfo{
		{SPI: 100, Dst: local.To4(), Proto: dataplane.ProtoESP, IfID: 7},
		{SPI: 200, Dst: remote.To4(), Proto: dataplane.ProtoESP, IfID: 7, BytesCurrent: 4096, PacketsCurrent: 12},
		{SPI: 200, Dst: other, Proto: dataplane.ProtoESP, IfID: 7, BytesCurrent: 8192, PacketsCurrent: 24},
	}}
	useDataplane(t, fake)
	resp, err := handleShowVPNIPsecSA(nil, nil)
	require.NoError(t, err)
	child := rowsOf(t, resp, "peers")[0]["child-sa"].(map[string]any)
	require.Equal(t, uint64(4096), child["bytes-out"])
	require.Equal(t, uint64(12), child["packets-out"])
	require.Equal(t, uint64(0), child["bytes-in"], "a measured zero remains numeric")
	require.Equal(t, true, child["counters-known"])

	fake.sas = append(fake.sas[:1], fake.sas[2])
	resp, err = handleShowVPNIPsecSA(nil, nil)
	require.NoError(t, err)
	child = rowsOf(t, resp, "peers")[0]["child-sa"].(map[string]any)
	require.Nil(t, child["bytes-out"])
	require.Nil(t, child["packets-out"])
	require.Equal(t, true, child["counters-known"], "the missing SA was observed")
	resp, err = handleShowVPNIPsecDataplaneDrift(nil, nil)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusError, resp.Status)
	require.Contains(t, resp.Error, "peer-alpha")
	require.Contains(t, resp.Error, "outbound")
	require.NotContains(t, resp.Error, "inbound")
}

// A roster publication inside ListSAs exercises the real engine observation
// guard through the CLI. Raw readback remains available during the transition.
func TestDriftRejectsGenerationChangeDuringDump(t *testing.T) {
	engine.SetActivePeersForTest(nil)
	t.Cleanup(func() { engine.SetActivePeersForTest(nil) })
	fake := &fakeDataplane{
		sas:   []dataplane.SAInfo{{SPI: 100, Proto: dataplane.ProtoESP}},
		onSAD: func() { engine.SetActivePeersForTest(nil) },
	}
	useDataplane(t, fake)
	resp, err := handleShowVPNIPsecDataplaneDrift(nil, nil)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusError, resp.Status)
	require.NotContains(t, resp.Error, "ipsec dataplane drift:")
	_, counters := readSADCounters()
	require.False(t, counters.known)
	resp, err = handleShowVPNIPsecDataplaneSA(nil, nil)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, resp.Status)
	require.Equal(t, uint32(100), rowsOf(t, resp, "sas")[0]["spi"])
}
