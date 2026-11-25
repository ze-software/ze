// VALIDATES: certificate changes reach the next real TLS handshake on a bound
// listener, while an authenticated client's session still serves requests.
// PREVENTS: a rotated fleet certificate taking effect only after a restart.
package server

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	pluginipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/test/sim"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func TestManagedApplyServingCertificateChangeNextHandshake(t *testing.T) {
	ca := newTestCA(t)
	cert, key := hubCertMaterial(t)
	nextCert, nextKey := hubCertMaterial(t)
	resolve := func(name string) ([]byte, []byte, error) {
		switch name {
		case "fleet-hub":
			return cert, key, nil
		case "replacement":
			return nextCert, nextKey, nil
		default:
			return nil, nil, errors.New("unknown certificate")
		}
	}
	listens := []ManagedListener{{Addr: "127.0.0.1:0", Clients: map[string]string{testClientName: testClientSecret}}}
	cfg := ManagedServerConfig{Authority: ca, Listeners: listens, TLSMaterialResolver: resolve,
		ReadConfig: func(string) ([]byte, error) { return []byte(testClientConfig), nil }}
	s, err := NewManagedServer(cfg)
	require.NoError(t, err)
	require.NoError(t, s.Start(t.Context()))
	t.Cleanup(s.Stop)
	addr := s.Addrs()[0].String()
	mc := dialTestClient(t, addr, testClientName, testClientSecret)
	fetch(t, mc, "")
	since := s.snapshotDevices()[0].ConnectedSince
	foreign, err := (&tls.Dialer{Config: anchoredConfig(t, cert)}).DialContext(t.Context(), "tcp", addr)
	if foreign != nil {
		_ = foreign.Close()
	}
	require.Error(t, err)
	for _, tc := range []struct {
		name   string
		anchor []byte
	}{
		{"fleet-hub", cert}, {"replacement", nextCert}, {"", ca.CertificatePEM()},
	} {
		plan, err := s.PrepareManagedListeners(listens, tc.name)
		require.NoError(t, err)
		s.ApplyManagedListeners(plan)
		require.Equal(t, addr, s.Addrs()[0].String())
		leaf := dialAnchored(t, s, tc.anchor)
		if tc.name == "" {
			require.NoError(t, leaf.CheckSignatureFrom(ca.cert))
		} else {
			want, _ := pem.Decode(tc.anchor)
			require.Equal(t, want.Bytes, leaf.Raw)
		}
		fetch(t, mc, "")
		require.Equal(t, since, s.snapshotDevices()[0].ConnectedSince)
	}
}

func assertManagedCertificateUnbound(t *testing.T, s *ManagedServer, err error, cause string) {
	t.Helper()
	require.NoError(t, err)
	require.NoError(t, s.Start(t.Context()))
	t.Cleanup(s.Stop)
	require.Empty(t, s.Addrs())
	record := s.doctor.snapshot()
	require.Len(t, record.Unbound, 1)
	require.Equal(t, "127.0.0.1:0", record.Unbound[0].Addr)
	require.Contains(t, record.Unbound[0].Error, cause)
}

func applyManaged(t *testing.T, s *ManagedServer, listens []ManagedListener, name string) {
	t.Helper()
	plan, err := s.PrepareManagedListeners(listens, name)
	require.NoError(t, err)
	s.ApplyManagedListeners(plan)
}

func applyTestBlock(addr string) ManagedListener {
	return ManagedListener{Addr: addr, Clients: map[string]string{testClientName: testClientSecret}}
}

func startApplyTest(t *testing.T, cfg ManagedServerConfig) *ManagedServer {
	t.Helper()
	cfg.ReadConfig = func(string) ([]byte, error) { return []byte(testClientConfig), nil }
	s, err := NewManagedServer(cfg)
	require.NoError(t, err)
	require.NoError(t, s.Start(t.Context()))
	t.Cleanup(s.Stop)
	return s
}

func TestManagedApplyAddsClient(t *testing.T) {
	blk := applyTestBlock("127.0.0.1:0")
	s := startApplyTest(t, ManagedServerConfig{Authority: newTestCA(t), Listeners: []ManagedListener{blk}})
	addr := s.Addrs()[0].String()
	mc := dialTestClient(t, addr, testClientName, testClientSecret)
	fetch(t, mc, "")
	blk.Clients[testClient2Name] = testClient2Secret
	blk.ClientLabels = map[string]map[string]string{testClient2Name: {"site": "east"}}
	applyManaged(t, s, []ManagedListener{blk}, "")
	rows := s.snapshotDevices()
	require.Len(t, rows, 2)
	require.True(t, rows[0].Connected)
	require.False(t, rows[1].Connected)
	require.True(t, rows[1].FirstConnected.IsZero())
	require.Equal(t, "east", rows[1].Labels["site"])
	fetch(t, dialTestClient(t, addr, testClient2Name, testClient2Secret), "")
	fetch(t, mc, "")
}

func TestManagedApplyRemovesConnectedClient(t *testing.T) {
	s, addr := startTestManagedServer(t)
	mc := dialTestClient(t, addr, testClient2Name, testClient2Secret)
	fetch(t, mc, "")
	before := s.snapshotDevices()[1]
	applyManaged(t, s, []ManagedListener{applyTestBlock("127.0.0.1:0")}, "")
	after := s.snapshotDevices()[1]
	require.False(t, after.Connected)
	require.False(t, after.Declared)
	require.False(t, after.RemovedAt.IsZero())
	require.Equal(t, before.LastFetch, after.LastFetch)
	require.Equal(t, before.FirstConnected, after.FirstConnected)
	require.False(t, s.Serves(testClient2Name))
	assertManagedAuthRefused(t, s, addr, testClient2Name, testClient2Secret, ManagedRejectUnknownName)
}

func TestManagedApplySecretChangeClosesSession(t *testing.T) {
	s, addr := startTestManagedServer(t)
	mc := dialTestClient(t, addr, testClientName, testClientSecret)
	fetch(t, mc, "")
	blk := applyTestBlock("127.0.0.1:0")
	blk.Clients[testClientName] = testClient2Secret
	applyManaged(t, s, []ManagedListener{blk}, "")
	require.False(t, s.snapshotDevices()[0].Connected)
	assertManagedAuthRefused(t, s, addr, testClientName, testClientSecret, ManagedRejectBadToken)
	fetch(t, dialTestClient(t, addr, testClientName, testClient2Secret), "")
}

func assertManagedAuthRefused(t *testing.T, s *ManagedServer, addr, name, secret string, cause ManagedRejectCause) {
	t.Helper()
	dialer := &tls.Dialer{Config: &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS13}} //nolint:gosec // test authenticates the device secret.
	conn, err := dialer.DialContext(t.Context(), "tcp", addr)
	require.NoError(t, err)
	defer conn.Close() //nolint:errcheck // test cleanup
	require.NoError(t, pluginipc.SendAuth(t.Context(), conn, secret, name))
	line, err := pluginipc.ReadLineRaw(conn, 512)
	require.NoError(t, err)
	_, verb, _, err := rpc.ParseLine(line)
	require.NoError(t, err)
	require.Equal(t, "error", verb)
	require.Eventually(t, func() bool {
		for _, rejection := range s.doctor.snapshot().Rejected {
			if rejection.Client == name && rejection.Cause == cause {
				return true
			}
		}
		return false
	}, time.Second, time.Millisecond)
}

func TestManagedApplyBindsNewAddress(t *testing.T) {
	ca := newTestCA(t)
	s := startApplyTest(t, ManagedServerConfig{Authority: ca})
	require.Empty(t, s.Addrs())
	applyManaged(t, s, []ManagedListener{applyTestBlock("127.0.0.1:0")}, "")
	require.Len(t, s.Addrs(), 1)
	fetch(t, dialTestClient(t, s.Addrs()[0].String(), testClientName, testClientSecret), "")
	applyManaged(t, s, nil, "")
	require.Empty(t, s.Addrs())
	require.False(t, s.snapshotDevices()[0].Connected)
}

func TestManagedApplyNewAddressVerifiesAgainstRoot(t *testing.T) {
	ca := newTestCA(t)
	s := startApplyTest(t, ManagedServerConfig{Authority: ca})
	applyManaged(t, s, []ManagedListener{applyTestBlock("127.0.0.2:0")}, "")
	require.NoError(t, dialAnchored(t, s, ca.CertificatePEM()).VerifyHostname("127.0.0.2"))
}

func TestManagedApplySecondAddressKeepsSessions(t *testing.T) {
	ca := newTestCA(t)
	first := applyTestBlock("127.0.0.1:0")
	s := startApplyTest(t, ManagedServerConfig{Authority: ca, Listeners: []ManagedListener{first}})
	addr := s.Addrs()[0].String()
	mc := dialTestClient(t, addr, testClientName, testClientSecret)
	fetch(t, mc, "")
	before := s.snapshotDevices()[0].ConnectedSince
	second := ManagedListener{Addr: "127.0.0.2:0", Clients: map[string]string{testClient2Name: testClient2Secret}}
	applyManaged(t, s, []ManagedListener{second, first}, "")
	require.Equal(t, addr, s.Addrs()[1].String())
	require.NoError(t, dialAnchored(t, s, ca.CertificatePEM()).VerifyHostname("127.0.0.2"))
	fetch(t, dialTestClient(t, s.Addrs()[0].String(), testClient2Name, testClient2Secret), "")
	fetch(t, mc, "")
	require.Equal(t, before, s.snapshotDevices()[0].ConnectedSince)
}

func TestManagedApplyFirstBindServesNamedCertificate(t *testing.T) {
	cert, key := hubCertMaterial(t)
	s := startApplyTest(t, ManagedServerConfig{Authority: newTestCA(t), TLSMaterialResolver: func(string) ([]byte, []byte, error) { return cert, key, nil }})
	applyManaged(t, s, []ManagedListener{applyTestBlock("127.0.0.1:0")}, "fleet-hub")
	want, _ := pem.Decode(cert)
	require.Equal(t, want.Bytes, dialAnchored(t, s, cert).Raw)
}

func TestManagedApplySameNameNewMaterialNextHandshake(t *testing.T) {
	cert, key := hubCertMaterial(t)
	blk := applyTestBlock("127.0.0.1:0")
	s := startApplyTest(t, ManagedServerConfig{Certificate: "fleet-hub", Listeners: []ManagedListener{blk}, TLSMaterialResolver: func(string) ([]byte, []byte, error) { return cert, key, nil }})
	before := dialAnchored(t, s, cert)
	cert, key = hubCertMaterial(t)
	applyManaged(t, s, []ManagedListener{blk}, "fleet-hub")
	want, _ := pem.Decode(cert)
	require.NotEqual(t, before.Raw, want.Bytes)
	require.Equal(t, want.Bytes, dialAnchored(t, s, cert).Raw)
}

type failingApplyCA struct {
	*testCA
	failure error
}

func (ca *failingApplyCA) IssueLeaf(name string, hosts []string) (tls.Certificate, error) {
	if ca.failure != nil {
		return tls.Certificate{}, ca.failure
	}
	return ca.testCA.IssueLeaf(name, hosts)
}

func TestManagedApplyLeafIssueFailureLeavesAddressUnbound(t *testing.T) {
	ca := &failingApplyCA{testCA: newTestCA(t)}
	first := applyTestBlock("127.0.0.1:0")
	next := applyTestBlock("127.0.0.2:0")
	s := startApplyTest(t, ManagedServerConfig{Authority: ca, Listeners: []ManagedListener{first}})
	before := dialAnchored(t, s, ca.CertificatePEM())
	ca.failure = errors.New("authority is offline")
	applyManaged(t, s, []ManagedListener{first, next}, "")
	require.Len(t, s.Addrs(), 1)
	require.Equal(t, before.Raw, dialAnchored(t, s, ca.CertificatePEM()).Raw)
	unbound := s.doctor.snapshot().Unbound
	require.Len(t, unbound, 1)
	require.Equal(t, next.Addr, unbound[0].Addr)
	require.Contains(t, unbound[0].Error, "authority is offline")
	ca.failure = nil
	applyManaged(t, s, []ManagedListener{first, next}, "")
	require.Len(t, s.Addrs(), 2)
	require.Empty(t, s.doctor.snapshot().Unbound)
}

func TestManagedStartUnresolvedCertificateBindsNothingThenReloadBinds(t *testing.T) {
	var cert, key []byte
	blk := applyTestBlock("127.0.0.1:0")
	blk.Block, blk.Certificate = "central", "fleet-hub"
	s := startApplyTest(t, ManagedServerConfig{Authority: newTestCA(t), Certificate: "fleet-hub", Listeners: []ManagedListener{blk},
		TLSMaterialResolver: func(string) ([]byte, []byte, error) {
			if cert == nil {
				return nil, nil, errors.New("undefined")
			}
			return cert, key, nil
		}})
	require.Empty(t, s.Addrs())
	require.Len(t, s.doctor.snapshot().Unbound, 1)
	_, err := s.PrepareManagedListeners([]ManagedListener{blk}, "fleet-hub")
	require.ErrorContains(t, err, "bus hub server central certificate fleet-hub")
	require.ErrorContains(t, err, "define pki certificate fleet-hub")
	cert, key = hubCertMaterial(t)
	applyManaged(t, s, []ManagedListener{blk}, "fleet-hub")
	want, _ := pem.Decode(cert)
	require.Equal(t, want.Bytes, dialAnchored(t, s, cert).Raw)
	require.Empty(t, s.doctor.snapshot().Unbound)
}

func TestManagedReloadKeepsLeafThatCoversHosts(t *testing.T) {
	ca := newTestCA(t)
	clk := sim.NewFakeClock(time.Now())
	first := applyTestBlock("127.0.0.1:0")
	second := applyTestBlock("127.0.0.2:0")
	s := startApplyTest(t, ManagedServerConfig{Authority: ca, Clock: clk, Listeners: []ManagedListener{first}})
	before := dialAnchored(t, s, ca.CertificatePEM())
	applyManaged(t, s, []ManagedListener{first}, "")
	require.Equal(t, before.Raw, dialAnchored(t, s, ca.CertificatePEM()).Raw)
	applyManaged(t, s, []ManagedListener{first, second}, "")
	added := dialAnchored(t, s, ca.CertificatePEM())
	require.NoError(t, added.VerifyHostname("127.0.0.2"))
	require.NotEqual(t, before.Raw, added.Raw)
	applyManaged(t, s, []ManagedListener{first}, "")
	require.Equal(t, added.Raw, dialAnchored(t, s, ca.CertificatePEM()).Raw)
	clk.Add(2 * time.Hour)
	renewed := dialAnchored(t, s, ca.CertificatePEM())
	require.NotEqual(t, added.Raw, renewed.Raw)
	require.NoError(t, renewed.VerifyHostname("127.0.0.2"))
}

func TestManagedReloadCertificateMatchesStart(t *testing.T) {
	ca := newTestCA(t)
	cert, key := hubCertMaterial(t)
	for _, addr := range []string{"127.0.0.1:0", "127.0.0.2:0"} {
		for _, name := range []string{"", "fleet-hub"} {
			cfg := ManagedServerConfig{Authority: ca, TLSMaterialResolver: func(string) ([]byte, []byte, error) { return cert, key, nil }}
			reloaded := startApplyTest(t, cfg)
			cfg.Certificate = name
			cfg.Listeners = []ManagedListener{applyTestBlock(addr)}
			boot := startApplyTest(t, cfg)
			applyManaged(t, reloaded, cfg.Listeners, name)
			startPair, err := boot.getCertificate(nil)
			require.NoError(t, err)
			reloadPair, err := reloaded.getCertificate(nil)
			require.NoError(t, err)
			if name != "" {
				require.Equal(t, startPair.Certificate, reloadPair.Certificate)
				continue
			}
			startLeaf, err := x509.ParseCertificate(startPair.Certificate[0])
			require.NoError(t, err)
			reloadLeaf, err := x509.ParseCertificate(reloadPair.Certificate[0])
			require.NoError(t, err)
			require.Equal(t, startLeaf.IPAddresses, reloadLeaf.IPAddresses)
			require.NoError(t, reloadLeaf.CheckSignatureFrom(ca.cert))
		}
	}
}

// Two configured blocks contending for one address must keep their admission
// secrets separate, including across an otherwise unchanged reload.
func TestManagedApplyConflictingBlocksKeepAdmissionScoped(t *testing.T) {
	first := applyTestBlock("127.0.0.1:0")
	first.Block = "east"
	s := startApplyTest(t, ManagedServerConfig{Authority: newTestCA(t), Listeners: []ManagedListener{first}})
	// Claim the actual port twice. Port zero itself asks for a fresh port
	// on every bind and does not conflict with another port-zero request.
	addr := s.Addrs()[0].String()
	first.Addr = addr
	second := ManagedListener{Block: "west", Addr: addr, Clients: map[string]string{testClient2Name: testClient2Secret}}
	listens := []ManagedListener{first, second}
	applyManaged(t, s, listens, "")
	require.Len(t, s.Addrs(), 1)
	require.Equal(t, addr, s.Addrs()[0].String())
	mc := dialTestClient(t, addr, testClientName, testClientSecret)
	fetch(t, mc, "")
	plan, err := s.PrepareManagedListeners(listens, "")
	require.NoError(t, err)
	delete(first.Clients, testClientName)
	s.ApplyManagedListeners(plan)
	fetch(t, mc, "")
	require.Len(t, s.doctor.snapshot().Unbound, 1)
	assertManagedAuthRefused(t, s, addr, testClient2Name, testClient2Secret, ManagedRejectUnknownName)
}

func TestManagedApplyEphemeralSocketsFollowNamedBlocks(t *testing.T) {
	first := applyTestBlock("127.0.0.1:0")
	first.Block = "east"
	second := ManagedListener{Block: "west", Addr: first.Addr, Clients: map[string]string{testClient2Name: testClient2Secret}}
	s := startApplyTest(t, ManagedServerConfig{Authority: newTestCA(t), Listeners: []ManagedListener{first, second}})
	addrs := s.Addrs()
	require.Len(t, addrs, 2)
	east, west := addrs[0].String(), addrs[1].String()
	require.NotEqual(t, east, west)
	eastClient := dialTestClient(t, east, testClientName, testClientSecret)
	westClient := dialTestClient(t, west, testClient2Name, testClient2Secret)
	fetch(t, eastClient, "")
	fetch(t, westClient, "")
	applyManaged(t, s, []ManagedListener{second, first}, "")
	addrs = s.Addrs()
	require.Len(t, addrs, 2)
	require.Equal(t, west, addrs[0].String())
	require.Equal(t, east, addrs[1].String())
	fetch(t, eastClient, "")
	fetch(t, westClient, "")
	require.False(t, authAs(t, east, testClient2Name, testClient2Secret))
	require.False(t, authAs(t, west, testClientName, testClientSecret))
}
