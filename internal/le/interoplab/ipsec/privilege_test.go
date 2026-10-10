// Design: docs/architecture/testing/interop.md -- the IPsec lab's peers
// Related: ipsec.go -- prepareScenario, the peers these tests read
// Related: nat.go -- natSetupScript and natSysctls
//
// No IPsec lab peer runs privileged (spec-lab-containers-least-privilege,
// AC-1): each is granted what its daemon does. The tests prepare every real
// scenario and read the peers' docker run inputs; no container starts.

package ipsec

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
	"github.com/ze-software/ze/internal/le/interoplab/grantcheck"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// preparedIPsecScenarios prepares every scenario under test/interop-ipsec.
func preparedIPsecScenarios(t *testing.T) map[string]interoplab.PreparedScenario {
	t.Helper()
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	environment := interoplab.Environment{Image: defaultFRRImage, SessionTimeout: 90 * time.Second, Suffix: "privilege-test"}
	sources, err := interoplab.Discover(filepath.Join(root, "test", "interop-ipsec", "scenarios"), "", checkerAdapters())
	if err != nil {
		t.Fatal(err)
	}
	network := interoplab.Network{Name: "ze-ipsec-privilege-test", IPv4: networkPrefix}
	prepared := make(map[string]interoplab.PreparedScenario, len(sources))
	for _, source := range sources {
		plan := scenarioPlan(root, environment, source, &scenarioState{})
		scenario, err := plan.Prepare(t.Context(), interoplab.PrepareContext{Source: source, Network: network})
		if err != nil {
			t.Fatalf("prepare %s: %v", source.Name, err)
		}
		if scenario.Cleanup != nil {
			t.Cleanup(func() {
				if err := scenario.Cleanup(); err != nil {
					t.Errorf("cleanup %s: %v", source.Name, err)
				}
			})
		}
		prepared[source.Name] = scenario
	}
	return prepared
}

// argumentPairs answers every value that follows flag in arguments.
func argumentPairs(arguments []string, flag string) []string {
	var values []string
	for i := 0; i+1 < len(arguments); i++ {
		if arguments[i] == flag {
			values = append(values, arguments[i+1])
		}
	}
	return values
}

// ipsecPeerGrants answers the exact grant set a peer of the named scenario
// holds: FRR the NET_ADMIN and SYS_ADMIN of the shared FRR peer; strongSwan NET_ADMIN, and /proc/sys writable only where its
// checker writes a sysctl; the NAT box NET_ADMIN and its --sysctl settings;
// every other peer NET_ADMIN.
func ipsecPeerGrants(scenario, peer string) grantcheck.Grants {
	switch peer {
	case frrPeer:
		return grantcheck.Grants{Capabilities: []string{"NET_ADMIN", "SYS_ADMIN"}}
	case swanPeer:
		grants := grantcheck.Grants{Capabilities: []string{"NET_ADMIN"}}
		if scenarioCheckers[scenario].writesPeerSysctl {
			grants.SecurityOptions = grantcheck.Parse(nil, interoplab.NetSysctlWriteArguments()).SecurityOptions
			grants.AppArmorProfile = interoplab.NetSysctlAppArmorProfileName
		}
		return grants
	case natPeer:
		return grantcheck.Grants{Capabilities: []string{"NET_ADMIN"}, Sysctls: natSysctls}
	}
	return grantcheck.Grants{Capabilities: []string{"NET_ADMIN"}}
}

// VALIDATES: AC-1. No peer is privileged; the NAT box, strongSwan and ze hold
// NET_ADMIN; the NAT box sets its sysctls through --sysctl and its script writes
// none; strongSwan, where the scenario's checker cuts its reassembly marks at
// run time (writesPeerSysctl on its checker entry) and only there, gets
// /proc/sys writable under the generic net sysctl profile, which grants both
// marks; ze gets neither.
// PREVENTS: a lab peer silently running --privileged again, or a grant drifting
// from the writes the lab makes.
func TestIPsecLabPeersRunWithoutPrivilege(t *testing.T) {
	for name, scenario := range preparedIPsecScenarios(t) {
		for _, peer := range scenario.Peers {
			for _, problem := range grantcheck.Problems(grantcheck.ParsePeer(peer, grantcheck.StagedRoots(t)...), ipsecPeerGrants(name, peer.Name)) {
				t.Errorf("%s: peer %s %s", name, peer.Name, problem)
			}
			if peer.Name == frrPeer {
				continue
			}
			if !slices.Contains(peer.Capabilities, "NET_ADMIN") {
				t.Errorf("%s: peer %s lacks NET_ADMIN: %v", name, peer.Name, peer.Capabilities)
			}
			writable := slices.Contains(peer.Arguments, "systempaths=unconfined")
			switch peer.Name {
			case swanPeer:
				if !scenarioCheckers[name].writesPeerSysctl {
					if writable || peer.AppArmorProfile != "" {
						t.Errorf("%s: strongSwan writes no sysctl here, yet gets %v %q", name, peer.Arguments, peer.AppArmorProfile)
					}
					continue
				}
				if !writable || peer.AppArmorProfile != interoplab.NetSysctlAppArmorProfileName {
					t.Errorf("%s: strongSwan arguments %v profile %q, want /proc/sys writable under %s",
						name, peer.Arguments, peer.AppArmorProfile, interoplab.NetSysctlAppArmorProfileName)
				}
			case natPeer:
				sysctls := argumentPairs(peer.Arguments, "--sysctl")
				for _, sysctl := range natSysctls {
					if !slices.Contains(sysctls, sysctl) {
						t.Errorf("%s: NAT box --sysctl %v lacks %s", name, sysctls, sysctl)
					}
				}
				if strings.Contains(peer.Command[len(peer.Command)-1], "sysctl") {
					t.Errorf("%s: the NAT script still writes a sysctl on a read-only /proc/sys", name)
				}
				if writable || peer.AppArmorProfile != "" {
					t.Errorf("%s: the NAT box writes no sysctl at run time, yet gets %v %q", name, peer.Arguments, peer.AppArmorProfile)
				}
			default:
				if writable || peer.AppArmorProfile != "" {
					t.Errorf("%s: peer %s writes no sysctl, yet gets %v %q", name, peer.Name, peer.Arguments, peer.AppArmorProfile)
				}
			}
		}
	}
	for _, mark := range peerReassemblyMarks {
		if !interoplab.NetSysctlGranted(mark) {
			t.Errorf("strongSwan's profile does not grant %s, which dropFragmentsAtPeer writes", mark)
		}
	}
}

// VALIDATES: a scenario's checker entry sets writesPeerSysctl exactly when its
// checker reaches dropFragmentsAtPeer, the one helper that writes strongSwan's
// sysctls. The package source is parsed and the calls between its functions
// followed to a fixed point, so a checker that writes through a helper counts.
// PREVENTS: a checker gaining a peer sysctl write while its strongSwan keeps a
// read-only /proc/sys (the scenario fails at run time on a lab host only), or
// a flag left behind that holds /proc/sys writable for nothing.
func TestOnlyCheckersThatWritePeerSysctlsAreFlagged(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	calls := map[string][]string{}
	fileSet := token.NewFileSet()
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fileSet, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil {
				continue
			}
			ast.Inspect(function.Body, func(node ast.Node) bool {
				call, isCall := node.(*ast.CallExpr)
				if !isCall {
					return true
				}
				switch callee := call.Fun.(type) {
				case *ast.Ident:
					calls[function.Name.Name] = append(calls[function.Name.Name], callee.Name)
				case *ast.SelectorExpr:
					calls[function.Name.Name] = append(calls[function.Name.Name], callee.Sel.Name)
				}
				return true
			})
		}
	}
	writers := map[string]bool{"dropFragmentsAtPeer": true}
	for grew := true; grew; {
		grew = false
		for caller, callees := range calls {
			if writers[caller] {
				continue
			}
			if slices.ContainsFunc(callees, func(callee string) bool { return writers[callee] }) {
				writers[caller] = true
				grew = true
			}
		}
	}
	for scenario, entry := range scenarioCheckers {
		qualified := runtime.FuncForPC(reflect.ValueOf(entry.run).Pointer()).Name()
		function := qualified[strings.LastIndexByte(qualified, '.')+1:]
		if writers[function] != entry.writesPeerSysctl {
			t.Errorf("%s: %s reaches dropFragmentsAtPeer = %v, but its entry sets writesPeerSysctl = %v",
				scenario, function, writers[function], entry.writesPeerSysctl)
		}
	}
}
