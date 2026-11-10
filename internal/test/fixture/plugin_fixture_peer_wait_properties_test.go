package fixture

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestObserverPeerWaitRegistrationProperties proves that registration metadata
// neither hides a driver's wait nor supplies a wait the driver never makes.
func TestObserverPeerWaitRegistrationProperties(t *testing.T) {
	const source = `package fixture
func Register(name string, driver any, properties ...any) {}
func propertyFrom(callback any) any { return callback }
func waitPeersEOR08(ctx, p any, peers ...string) error { return nil }
func waitPeerEOR08(ctx, p any) error { return waitPeersEOR08(ctx, p, "peer1") }
func registerWithProperties(name string, driver any) {
	Register(name, driver, propertyFrom(waitPeerEOR08))
}
type row struct { name string; driver any; property any }
func init() { REGISTER }
func scenarioX(ctx, p any) error { BODY }
`
	const ci = `stdin=peer:terminator=EOF_PEER
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00170200000000
EOF_PEER
stdin=ze-conf:terminator=EOF_CONF
bgp {
	peer peer1 {
		connection { remote { ip 127.0.0.1; port $PORT; } local { ip 127.0.0.1; } }
	}
}
EOF_CONF
cmd=background:seq=1:exec=le test peer --port $PORT:stdin=peer
cmd=foreground:seq=2:exec=ze -:stdin=ze-conf
`
	for _, tc := range []struct {
		name     string
		register string
		body     string
		want     int
	}{
		{name: "driver waits", register: `Register("plugin/x", scenarioX, propertyFrom(waitPeerEOR08))`, body: "return waitPeerEOR08(ctx, p)", want: 0},
		{name: "property is not a driver", register: `Register("plugin/x", scenarioX, propertyFrom(waitPeerEOR08))`, body: "return nil", want: 1},
		{name: "wrapper retains driver", register: `registerWithProperties("plugin/x", scenarioX)`, body: "return waitPeerEOR08(ctx, p)", want: 0},
		{name: "wrapper property is not a driver", register: `registerWithProperties("plugin/x", scenarioX)`, body: "return nil", want: 1},
		{name: "table retains driver", register: `for _, entry := range []row{{"plugin/x", scenarioX, waitPeerEOR08}} { Register(entry.name, entry.driver, entry.property) }`, body: "return waitPeerEOR08(ctx, p)", want: 0},
		{name: "table property is not a driver", register: `for _, entry := range []row{{"plugin/x", scenarioX, waitPeerEOR08}} { Register(entry.name, entry.driver, entry.property) }`, body: "return nil", want: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			testRoot := t.TempDir()
			content := strings.Replace(source, "BODY", tc.body, 1)
			content = strings.Replace(content, "REGISTER", tc.register, 1)
			writeGuardFile(t, filepath.Join(dir, "plugin_fixture_08.go"), content)
			writeGuardFile(t, filepath.Join(testRoot, "plugin", "x.ci"), ci)
			findings, err := observerPeerWaitFindings(dir, testRoot)
			if err != nil {
				t.Fatal(err)
			}
			if len(findings) != tc.want {
				t.Fatalf("findings = %v, want %d", findings, tc.want)
			}
		})
	}
}

// TestFamily12PeerWaitReadsRegistrationProperties proves the family census
// retains a real observer whose registration declares StopsDaemon.
func TestFamily12PeerWaitReadsRegistrationProperties(t *testing.T) {
	source := parseFixturePackage12(t)
	ref, found := source.registrations["plugin/redistribute-l2tp-announce"]
	if !found {
		t.Fatal("family-12 census dropped the L2TP announce observer with StopsDaemon")
	}
	if ref.function != "p12L2TPAnnounce" {
		t.Fatalf("scenario = %s, want p12L2TPAnnounce", ref.describe())
	}
}
