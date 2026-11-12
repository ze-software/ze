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
	environment := interoplab.Environment{Image: interoplab.FRRImage, SessionTimeout: 90 * time.Second, Suffix: "privilege-test"}
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
// ze NET_ADMIN and BPF, the host's /dev/net/tun and /proc/sys writable under
// the generic net sysctl profile; every other peer NET_ADMIN.
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
	case zePeer:
		// BPF: the default `vpn ipsec application-override deny` loads a cgroup
		// setsockopt program (capabilityBPF in ipsec.go). /dev/net/tun and the
		// writable /proc/sys: an encapsulated Child SA makes Ze create the
		// ze-espform TUN device and set its sysctls (openESPFormTUN).
		return grantcheck.Grants{
			Capabilities:    []string{"NET_ADMIN", "BPF"},
			Devices:         []string{interoplab.TUNDevice},
			SecurityOptions: grantcheck.Parse(nil, interoplab.NetSysctlWriteArguments()).SecurityOptions,
			AppArmorProfile: interoplab.NetSysctlAppArmorProfileName,
		}
	case natPeer:
		return grantcheck.Grants{Capabilities: []string{"NET_ADMIN"}, Sysctls: natSysctls}
	}
	return grantcheck.Grants{Capabilities: []string{"NET_ADMIN"}}
}

// VALIDATES: AC-1. No peer is privileged; the NAT box and strongSwan hold
// NET_ADMIN, ze NET_ADMIN and BPF; the NAT box sets its sysctls through --sysctl and its script writes
// none; strongSwan, where the scenario's checker writes its sysctls at run time
// (cutting its reassembly marks, enabling IPv6 on its lab interface, or turning
// on forwarding;
// writesPeerSysctl on its checker entry) and only there, gets /proc/sys
// writable under the generic net sysctl profile, which grants every sysctl
// those checkers write; ze, in every scenario, gets the host's /dev/net/tun
// and /proc/sys writable under the same profile, because whether a Child SA is
// UDP-encapsulated (and so creates the ze-espform TUN device and writes its
// sysctls) is decided at run time; ze takes no --sysctl: a scenario that
// needs IPv6 on its eth0 gets an IPv6 lab network instead (networkIPv6).
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
			case zePeer:
				if !writable || peer.AppArmorProfile != interoplab.NetSysctlAppArmorProfileName {
					t.Errorf("%s: ze arguments %v profile %q, want /proc/sys writable under %s for the ze-espform sysctls",
						name, peer.Arguments, peer.AppArmorProfile, interoplab.NetSysctlAppArmorProfileName)
				}
				if !slices.Contains(argumentPairs(peer.Arguments, "--device"), interoplab.TUNDevice) {
					t.Errorf("%s: ze arguments %v lack --device %s, which an encapsulated Child SA opens",
						name, peer.Arguments, interoplab.TUNDevice)
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
	for _, key := range vtiIPv6PeerSysctls {
		if !interoplab.NetSysctlGranted(key) {
			t.Errorf("strongSwan's profile does not grant %s, which vtiIPv6PeerInner writes", key)
		}
	}
}

// VALIDATES: a scenario's checker entry sets writesPeerSysctl exactly when its
// checker reaches a function that writes strongSwan's sysctls. The package
// source is parsed; a writer is any function whose body calls
// exec(ctx, swanPeer, "sysctl", "-w", ...) (today dropFragmentsAtPeer and
// vtiIPv6PeerInner), and the calls between functions are followed to a fixed
// point, so a checker that writes directly or through a helper counts.
// PREVENTS: a checker gaining a peer sysctl write while its strongSwan keeps a
// read-only /proc/sys (the scenario fails at run time on a lab host only), or
// a flag left behind that holds /proc/sys writable for nothing.
func TestOnlyCheckersThatWritePeerSysctlsAreFlagged(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	calls := map[string][]string{}
	writers := map[string]bool{}
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
					if callee.Sel.Name == "exec" && writesSwanSysctl(call.Args) {
						writers[function.Name.Name] = true
					}
				}
				return true
			})
		}
	}
	if len(writers) == 0 {
		t.Fatal(`no function calls exec(ctx, swanPeer, "sysctl", "-w", ...): the writer derivation matches nothing`)
	}
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
			t.Errorf("%s: %s reaches a peer sysctl writer = %v, but its entry sets writesPeerSysctl = %v",
				scenario, function, writers[function], entry.writesPeerSysctl)
		}
	}
}

// writesSwanSysctl reports whether exec's arguments are
// (ctx, swanPeer, "sysctl", "-w", ...): a sysctl write on the strongSwan peer.
func writesSwanSysctl(args []ast.Expr) bool {
	if len(args) < 4 {
		return false
	}
	peer, ok := args[1].(*ast.Ident)
	if !ok || peer.Name != "swanPeer" {
		return false
	}
	for i, want := range []string{`"sysctl"`, `"-w"`} {
		literal, ok := args[2+i].(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING || literal.Value != want {
			return false
		}
	}
	return true
}
