// Design: docs/architecture/testing/interop.md -- native transit checker polarities.
package bgp

import (
	"encoding/json"
	"net/netip"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// bestRouteTransitFRRCheckerBranches requires useful ordinary AS_PATH,
// exact next hop and independent control while refusing unmeasured tables,
// excluded-route leaks and failed withdrawals. This tests the oracle only;
// the scenario runs the real daemon and proves the advertised lifecycle.
func bestRouteTransitFRRCheckerBranches(t *testing.T) {
	hop := netip.MustParseAddr("172.30.0.2")
	route := func(prefix, path, next string) string {
		value := map[string]any{"prefix": prefix, "paths": []any{map[string]any{
			"aspath": map[string]any{"string": path}, "nexthops": []any{map[string]any{"ip": next}}, "valid": true,
		}}}
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return string(encoded)
	}
	table := `{"vrfId":0,"vrfName":"default","tableVersion":1,"routerId":"172.30.0.15","routes":{"10.245.0.0/24":[{}],"10.245.1.0/24":[{}]}}`
	good := bestRouteTransitSnapshot{table: table, subject: route(bestRouteTransitSubject, "65000 65001", hop.String()), control: route(bestRouteTransitControl, "65000 65001", hop.String())}
	for _, question := range []struct {
		name    string
		reading bestRouteTransitSnapshot
		path    string
		valid   bool
	}{
		{"initial", good, "65000 65001", true},
		{"unmeasured", bestRouteTransitSnapshot{table: "{}", subject: good.subject, control: good.control}, "65000 65001", false},
		{"empty query", bestRouteTransitSnapshot{}, "", false},
		{"withdrawal still held", good, "", false},
		{"route-server path", bestRouteTransitSnapshot{table: table, subject: route(bestRouteTransitSubject, "65001", hop.String()), control: good.control}, "65000 65001", false},
		{"invalid native route", bestRouteTransitSnapshot{table: table, subject: strings.Replace(good.subject, `"valid":true`, `"valid":false`, 1), control: good.control}, "65000 65001", false},
		{"wrong next hop", bestRouteTransitSnapshot{table: table, subject: route(bestRouteTransitSubject, "65000 65001", "172.30.0.3"), control: good.control}, "65000 65001", false},
		{"wrong exact prefix", bestRouteTransitSnapshot{table: table, subject: route(bestRouteTransitDenied, "65000 65001", hop.String()), control: good.control}, "65000 65001", false},
		{"denied leak", bestRouteTransitSnapshot{table: strings.Replace(table, `"10.245.1.0/24":[{}]`, `"10.245.1.0/24":[{}],"10.245.2.0/24":[{}]`, 1), subject: good.subject, control: good.control}, "65000 65001", false},
		{"missing control", bestRouteTransitSnapshot{table: strings.Replace(table, `"10.245.1.0/24":[{}]`, `"10.245.9.0/24":[{}]`, 1), subject: good.subject, control: good.control}, "65000 65001", false},
		{"backup", bestRouteTransitSnapshot{table: table, subject: route(bestRouteTransitSubject, "65000 65002 65002 65002", hop.String()), control: good.control}, "65000 65002 65002 65002", true},
		{"final", bestRouteTransitSnapshot{table: strings.Replace(table, `"10.245.0.0/24":[{}],`, "", 1), control: good.control}, "", true},
	} {
		t.Run(question.name, func(t *testing.T) {
			err := requireBestRouteTransitFRR(question.reading, question.path, hop)
			if (err == nil) != question.valid {
				t.Fatalf("native verdict=%v,valid=%t", err, question.valid)
			}
		})
	}
}

// bestRouteTransitBIRDCheckerBranches pairs both useful paths and final
// absence with a native control, and refuses covering routes, policy leaks,
// missing paths, unprepended route-server output and an empty query.
func bestRouteTransitBIRDCheckerBranches(t *testing.T) {
	route := func(prefix, path string) string {
		return prefix + " unicast [ze_peer 00:00:01] * (100)\n bgp_path: " + path + "\n"
	}
	control := route(bestRouteTransitControl, "65000 65001")
	subject := route(bestRouteTransitSubject, "65000 65001")
	good := bestRouteTransitSnapshot{table: "Table master4:\n" + subject + control, subject: subject, control: control}
	for _, question := range []struct {
		name    string
		reading bestRouteTransitSnapshot
		path    string
		valid   bool
	}{
		{"initial", good, "65000 65001", true},
		{"empty", bestRouteTransitSnapshot{}, "", false},
		{"missing control", bestRouteTransitSnapshot{table: subject, subject: subject, control: control}, "65000 65001", false},
		{"denied leak", bestRouteTransitSnapshot{table: good.table + route(bestRouteTransitDenied, "65000 65001"), subject: subject, control: control}, "65000 65001", false},
		{"covering route", bestRouteTransitSnapshot{table: strings.ReplaceAll(good.table, bestRouteTransitSubject, "10.245.0.0/16"), subject: strings.ReplaceAll(subject, bestRouteTransitSubject, "10.245.0.0/16"), control: control}, "65000 65001", false},
		{"route-server path", bestRouteTransitSnapshot{table: good.table, subject: route(bestRouteTransitSubject, "65001"), control: control}, "65000 65001", false},
		{"unreachable", bestRouteTransitSnapshot{table: good.table, subject: strings.Replace(subject, "unicast", "unreachable", 1), control: control}, "65000 65001", false},
		{"unselected", bestRouteTransitSnapshot{table: good.table, subject: strings.Replace(subject, " *", "", 1), control: control}, "65000 65001", false},
		{"withdrawal still held", good, "", false},
		{"backup", bestRouteTransitSnapshot{table: good.table, subject: route(bestRouteTransitSubject, "65000 65002 65002 65002"), control: control}, "65000 65002 65002 65002", true},
		{"final", bestRouteTransitSnapshot{table: "Table master4:\n" + control, control: control}, "", true},
	} {
		t.Run(question.name, func(t *testing.T) {
			err := requireBestRouteTransitBIRD(question.reading, question.path)
			if (err == nil) != question.valid {
				t.Fatalf("native verdict=%v,valid=%t", err, question.valid)
			}
		})
	}
}

// TestBestRouteTransitSourcePopulationRequiresBothOriginals refuses an empty
// receive store and one missing backup, so a later replacement cannot stand
// in for a candidate that the initial setup never delivered.
func TestBestRouteTransitSourcePopulationRequiresBothOriginals(t *testing.T) {
	source, backup := netip.MustParseAddr("172.30.0.3"), netip.MustParseAddr("172.30.0.14")
	good := `{"adj-rib-in":{"172.30.0.3":[{"key":"ipv4/unicast:10.245.0.0/24"}],"172.30.0.14":[{"key":"ipv4/unicast:10.245.0.0/24"}]}}`
	for _, question := range []struct {
		output string
		valid  bool
	}{
		{good, true}, {"{}", false}, {strings.Replace(good, `"172.30.0.14"`, `"172.30.0.9"`, 1), false}, {strings.ReplaceAll(good, "10.245.0.0/24", "10.245.9.0/24"), false},
	} {
		if err := requireBestRouteTransitInputs(question.output, source, backup); (err == nil) != question.valid {
			t.Fatalf("source verdict=%v,valid=%t", err, question.valid)
		}
	}
}

// TestBestRouteTransitExcludedRouteRequiresItsSelectedSource distinguishes a
// real export exclusion from missing input or selection of another route.
func TestBestRouteTransitExcludedRouteRequiresItsSelectedSource(t *testing.T) {
	source := netip.MustParseAddr("172.30.0.3")
	good := `[{"family":"ipv4/unicast","prefix":"10.245.2.0/24","best-peer":"172.30.0.3"}]`
	for _, question := range []struct {
		output string
		valid  bool
	}{
		{good, true}, {"{}", false}, {strings.Replace(good, "10.245.2.0/24", "10.245.0.0/24", 1), false}, {strings.Replace(good, "172.30.0.3", "172.30.0.14", 1), false},
		{"[]", false}, {"null", false}, {"[", false}, {"[null]", false},
		{strings.TrimSuffix(good, "]") + "," + strings.TrimPrefix(good, "["), false},
		{`{"best-path":` + good + `}`, false},
		{`[{"best-peer":"172.30.0.3"}]`, false}, {`[{"prefix":"10.245.2.0/24"}]`, false},
		{strings.Replace(good, "172.30.0.3", "not-an-address", 1), false},
	} {
		if err := requireBestRouteTransitSelected(question.output, source); (err == nil) != question.valid {
			t.Fatalf("selected verdict=%v,valid=%t", err, question.valid)
		}
	}
}

// TestBestRouteTransitPreparationKeepsTheNativeContract copies only immutable
// lab settings, gives each new native daemon its exact private configuration,
// and refuses a missing file or native identity before a container can start.
func TestBestRouteTransitPreparationKeepsTheNativeContract(t *testing.T) {
	directory := t.TempDir()
	for _, file := range []string{"bird-backup.conf", "bird-sink.conf", "frr-backup.conf"} {
		if err := os.WriteFile(filepath.Join(directory, file), []byte("native config"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, question := range []struct {
		name, target string
		prepare      func([]interoplab.PeerConfig, string) ([]interoplab.PeerConfig, error)
		roles        []string
	}{
		{peerFRR, frrConfigPath, prepareBestRouteTransitFRR, []string{bestRouteTransitFRRBackup}},
		{peerBIRD, "/etc/bird/bird.conf", prepareBestRouteTransitBIRD, []string{bestRouteTransitBIRDBackup, bestRouteTransitBIRDSink}},
	} {
		t.Run(question.name, func(t *testing.T) {
			peers := []interoplab.PeerConfig{{Name: question.name, Container: containerName(question.name, "proof"), Image: "actual-pinned-image", Host: 3,
				Mounts: []interoplab.Mount{{Source: "original.conf", Target: question.target, ReadOnly: true}}, Capabilities: []string{"NET_ADMIN"}}}
			before := peers[0]
			result, err := question.prepare(peers, directory)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(result[0], before) {
				t.Fatal("preparation changed original native source")
			}
			if len(result) != len(question.roles)+1 {
				t.Fatalf("prepared %d roles", len(result))
			}
			for i, role := range question.roles {
				peer := result[i+1]
				if peer.Name != role {
					t.Fatalf("role=%s,want%s", peer.Name, role)
				}
				if peer.Image != before.Image {
					t.Fatal("new daemon changed pinned image")
				}
				if peer.Container != containerName(role, "proof") {
					t.Fatal("new daemon lost run identity")
				}
				if peer.Host != uint8(14+i) {
					t.Fatal("new daemon lost independent native address")
				}
				if peer.Mounts[0].Target != question.target {
					t.Fatal("new daemon changed native config target")
				}
				if peer.Mounts[0].Source == before.Mounts[0].Source {
					t.Fatal("new daemon reused original config")
				}
			}
			if _, err := question.prepare(nil, directory); err == nil {
				t.Fatal("missing native source was accepted")
			}
			if _, err := question.prepare(peers, t.TempDir()); err == nil {
				t.Fatal("missing native configuration was accepted")
			}
		})
	}
}
