// Property test for ExaBGP -> Ze migration round-trip (spec followup-test-infra L93).
//
// Engine: stdlib testing/quick with a fixed RNG seed (deterministic CI; R-1).
//
// There is no ze->exabgp reverse converter, so "round-trip" is NOT byte
// equality. It is: a randomly generated *valid* ExaBGP config, when parsed ->
// migrated -> serialized, produces text that RE-PARSES as a valid Ze config
// (config.YANGSchema) AND preserves the semantic essentials -- every neighbor
// survives as a peer, and its peer-as is carried through. This catches the
// migration emitting Ze config the Ze parser rejects, or silently dropping
// neighbors, without asserting a converter that does not exist.
package migration

import (
	"fmt"
	"math/rand"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"testing/quick"

	"github.com/ze-software/ze/internal/component/config"
	// Blank imports register the YANG modules config.YANGSchema() must resolve
	// to re-parse the migrated output: ze-bgp-conf provides the top-level `bgp`
	// block, and it imports ze-hub-conf (registered by hub/yang). The generator
	// never emits GR/route-refresh, so no RIB plugin is injected and no other
	// module is needed.
	_ "github.com/ze-software/ze/internal/component/bgp/yang"
	_ "github.com/ze-software/ze/internal/component/hub/yang"
)

// neighborSpec is one generated ExaBGP neighbor. IPs are index-derived at
// render time so every neighbor in a config is distinct and valid.
type neighborSpec struct {
	localAS  uint32
	peerAS   uint32
	routerID bool // emit a router-id leaf
	holdTime bool // emit a hold-time leaf
	ipv4     bool // family ipv4 unicast
	ipv6     bool // family ipv6 unicast
}

// genExaConfig is a testing/quick generator producing a small, always-valid
// ExaBGP config (1..5 neighbors).
type genExaConfig struct {
	neighbors []neighborSpec
}

// Generate implements quick.Generator.
func (genExaConfig) Generate(r *rand.Rand, _ int) reflect.Value {
	n := 1 + r.Intn(5)
	specs := make([]neighborSpec, n)
	for i := range specs {
		specs[i] = neighborSpec{
			localAS:  1 + uint32(r.Intn(65534)), //nolint:gosec // bounded test value
			peerAS:   1 + uint32(r.Intn(65534)), //nolint:gosec // bounded test value
			routerID: r.Intn(2) == 0,
			holdTime: r.Intn(2) == 0,
			ipv4:     r.Intn(2) == 0,
			ipv6:     r.Intn(2) == 0,
		}
	}
	return reflect.ValueOf(genExaConfig{neighbors: specs})
}

// neighborIPs returns the deterministic IPs assigned to each neighbor.
func (g genExaConfig) neighborIPs() []string {
	ips := make([]string, len(g.neighbors))
	for i := range g.neighbors {
		ips[i] = fmt.Sprintf("10.0.%d.%d", i/250, 1+i%250)
	}
	return ips
}

// render produces the ExaBGP config text for this generated set.
func (g genExaConfig) render() string {
	var buf strings.Builder
	ips := g.neighborIPs()
	for i, spec := range g.neighbors {
		buf.WriteString("neighbor ")
		buf.WriteString(ips[i])
		buf.WriteString(" {\n\tlocal-as ")
		buf.WriteString(itoa(spec.localAS))
		buf.WriteString("\n\tpeer-as ")
		buf.WriteString(itoa(spec.peerAS))
		buf.WriteString("\n")
		if spec.routerID {
			buf.WriteString("\trouter-id 1.2.3.")
			buf.WriteString(strconv.Itoa(1 + i%250))
			buf.WriteString("\n")
		}
		if spec.holdTime {
			buf.WriteString("\thold-time 180\n")
		}
		if spec.ipv4 || spec.ipv6 {
			buf.WriteString("\tfamily {\n")
			if spec.ipv4 {
				buf.WriteString("\t\tipv4 unicast\n")
			}
			if spec.ipv6 {
				buf.WriteString("\t\tipv6 unicast\n")
			}
			buf.WriteString("\t}\n")
		}
		buf.WriteString("}\n")
	}
	return buf.String()
}

func itoa(v uint32) string {
	return strconv.FormatUint(uint64(v), 10)
}

// TestMigrationRoundTripProperty is the L93 property: parse -> migrate ->
// serialize -> re-parse must succeed and preserve every neighbor as a peer.
//
// VALIDATES: AC-1 / L93 -- migration emits re-parseable Ze config and never
// silently drops a neighbor or corrupts its peer-as, for any valid input shape.
// PREVENTS: a migration change that produces Ze-invalid output or loses peers
// slipping through the fixed-example unit tests.
func TestMigrationRoundTripProperty(t *testing.T) {
	t.Parallel()

	zeSchema, err := config.YANGSchema()
	if err != nil {
		t.Fatalf("load ze schema: %v", err)
	}
	zeParser := config.NewParser(zeSchema)

	f := func(g genExaConfig) bool {
		exaText := g.render()

		exaTree, err := ParseExaBGPConfig(exaText)
		if err != nil || exaTree == nil {
			t.Errorf("ParseExaBGPConfig failed for generated config:\n%s\nerr=%v", exaText, err)
			return false
		}

		result, err := MigrateFromExaBGP(exaTree)
		if err != nil || result == nil || result.Tree == nil {
			t.Errorf("MigrateFromExaBGP failed:\n%s\nerr=%v", exaText, err)
			return false
		}

		zeText := SerializeTree(result.Tree)

		// Primary round-trip invariant: migrated config re-parses as valid Ze config.
		zeTree, err := zeParser.Parse(zeText)
		if err != nil || zeTree == nil {
			t.Errorf("re-parse of migrated config failed:\nexabgp:\n%s\nze:\n%s\nerr=%v", exaText, zeText, err)
			return false
		}

		// Semantic invariant: every generated neighbor survives as a peer, with
		// its peer-as preserved.
		gotPeers := collectPeers(zeTree)
		if len(gotPeers) != len(g.neighbors) {
			t.Errorf("peer count mismatch: want %d got %d\nze:\n%s", len(g.neighbors), len(gotPeers), zeText)
			return false
		}
		for i, ip := range g.neighborIPs() {
			peerAS, ok := gotPeers[ip]
			if !ok {
				t.Errorf("neighbor %s missing from migrated peers %v\nze:\n%s", ip, keys(gotPeers), zeText)
				return false
			}
			wantAS := itoa(g.neighbors[i].peerAS)
			if peerAS != wantAS {
				t.Errorf("neighbor %s peer-as: want %s got %s\nze:\n%s", ip, wantAS, peerAS, zeText)
				return false
			}
		}
		return true
	}

	cfg := &quick.Config{
		MaxCount: 500,
		Rand:     rand.New(rand.NewSource(93)), //nolint:gosec // deterministic test seed
	}
	if err := quick.Check(f, cfg); err != nil {
		t.Fatalf("migration round-trip property violated: %v", err)
	}
}

// collectPeers walks the re-parsed Ze tree and returns a map keyed by each
// peer's remote IP (connection > remote > ip -- the ExaBGP neighbor address)
// -> its session remote AS. Migration keys the peer LIST by a derived name
// (peer-N), not the IP, so identity is recovered from connection.remote.ip.
// Migration nests peers as bgp { group NAME { peer NAME { ... } } }; this also
// tolerates peers placed directly under bgp.
func collectPeers(zeTree *config.Tree) map[string]string {
	peers := make(map[string]string)
	bgp := zeTree.GetContainer("bgp")
	if bgp == nil {
		return peers
	}
	record := func(peer *config.Tree) {
		peers[peerRemoteIP(peer)] = peerRemoteAS(peer)
	}
	for _, g := range bgp.GetListOrdered("group") {
		for _, p := range g.Value.GetListOrdered("peer") {
			record(p.Value)
		}
	}
	for _, p := range bgp.GetListOrdered("peer") {
		record(p.Value)
	}
	return peers
}

// peerRemoteIP extracts connection > remote > ip (the ExaBGP neighbor address).
func peerRemoteIP(peer *config.Tree) string {
	conn := peer.GetContainer("connection")
	if conn == nil {
		return ""
	}
	remote := conn.GetContainer("remote")
	if remote == nil {
		return ""
	}
	v, _ := remote.Get("ip")
	return v
}

// peerRemoteAS extracts session > asn > remote (the peer-as) from a peer tree.
func peerRemoteAS(peer *config.Tree) string {
	session := peer.GetContainer("session")
	if session == nil {
		return ""
	}
	asn := session.GetContainer("asn")
	if asn == nil {
		return ""
	}
	v, _ := asn.Get("remote")
	return v
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
