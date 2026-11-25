// VALIDATES: doctor-dataplane-mismatch code registration, dataplane-compatibility
// doctor check phase placement, and end-to-end wiring of evaluateDataPlaneVerdict
// into LoadConfig, ReadConfig, VerifyPluginConfig, and ze doctor.
// PREVENTS: silent divergence between config doors or a doctor check hidden
// behind a ReadConfig refusal in post-config phase.

package config

import (
	"errors"
	"net"
	"slices"
	"strings"
	"testing"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
)

// TestDoctorCodeDataPlaneMismatchRegistered checks that the diagnostic code
// metadata for doctor-dataplane-mismatch is registered at package init.
//
// VALIDATES: diagnostic.Register registered CodeDoctorDataPlaneMismatch.
// PREVENTS: `ze show diagnostic doctor-dataplane-mismatch` answering unknown code.
func TestDoctorCodeDataPlaneMismatchRegistered(t *testing.T) {
	meta := diagnostic.Lookup(CodeDoctorDataPlaneMismatch)
	if meta == nil {
		t.Fatalf("diagnostic.Lookup(%q) = nil, want registered metadata", CodeDoctorDataPlaneMismatch)
	}
	if meta.Code != CodeDoctorDataPlaneMismatch {
		t.Errorf("Code = %q, want %q", meta.Code, CodeDoctorDataPlaneMismatch)
	}
	if meta.Title == "" {
		t.Error("Title is empty")
	}
	if meta.Description == "" {
		t.Error("Description is empty")
	}
	if len(meta.Examples) == 0 {
		t.Error("Examples is empty")
	}
}

// TestDataPlaneDoctorCheckRegisteredInParsedConfigPhase verifies that the
// dataplane-compatibility doctor check runs in DoctorPhaseParsedConfig ahead
// of ReadConfig and carries the expected metadata.
//
// VALIDATES: dataPlaneDoctorCheck registration in DoctorPhaseParsedConfig.
// PREVENTS: a ReadConfig refusal hiding data-plane mismatch diagnostics.
func TestDataPlaneDoctorCheckRegisteredInParsedConfigPhase(t *testing.T) {
	parsedChecks := diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhaseParsedConfig)
	var found *diagnostic.DoctorCheck
	for i := range parsedChecks {
		if parsedChecks[i].Name == dataPlaneCheckName {
			found = &parsedChecks[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("doctor check %q not found in DoctorPhaseParsedConfig", dataPlaneCheckName)
	}
	if found.Order != 100 {
		t.Errorf("Order = %d, want 100", found.Order)
	}
	if found.Component != doctorComponent {
		t.Errorf("Component = %q, want %q", found.Component, doctorComponent)
	}
	if !slices.Equal(found.Dependencies, []string{"config-tree"}) {
		t.Errorf("Dependencies = %v, want [config-tree]", found.Dependencies)
	}
	if !slices.Equal(found.Platforms, []string{diagnostic.DoctorPlatformAny}) {
		t.Errorf("Platforms = %v, want [%s]", found.Platforms, diagnostic.DoctorPlatformAny)
	}
	if !slices.Equal(found.Codes, []string{CodeDoctorDataPlaneMismatch}) {
		t.Errorf("Codes = %v, want [%s]", found.Codes, CodeDoctorDataPlaneMismatch)
	}
	if found.Check == nil {
		t.Fatal("Check callback is nil")
	}

	for _, otherPhase := range []diagnostic.DoctorCheckPhase{
		diagnostic.DoctorPhasePreConfig,
		diagnostic.DoctorPhasePostConfig,
		diagnostic.DoctorPhaseRuntime,
	} {
		for _, check := range diagnostic.DoctorChecksForPhase(otherPhase) {
			if check.Name == dataPlaneCheckName {
				t.Errorf("doctor check %q unexpectedly registered in phase %q", dataPlaneCheckName, otherPhase)
			}
		}
	}
}

// TestDataPlaneVerdictWiredIntoBothDoorsAndDoctor proves that LoadConfig,
// ReadConfig, VerifyPluginConfig, and the registered doctor check all invoke
// evaluateDataPlaneVerdict.
//
// VALIDATES: LoadConfig, ReadConfig, VerifyPluginConfig, and checkDataPlaneCompatibility
// each route through evaluateDataPlaneVerdict, and configs succeed cleanly when
// no primary selector is registered.
// PREVENTS: wiring only one config door or omitting the doctor check path.
func TestDataPlaneVerdictWiredIntoBothDoorsAndDoctor(t *testing.T) {
	isolateRegistry(t)

	// Without any primary selector registered, both doors and doctor succeed cleanly.
	const sampleConfig = "environment {\n}\n"
	if _, err := ReadConfig(sampleConfig, "test.conf", nil); err != nil {
		t.Fatalf("ReadConfig without primary selector failed: %v", err)
	}
	parsed, err := ParseTreeForValidation(sampleConfig)
	if err != nil {
		t.Fatalf("ParseTreeForValidation failed: %v", err)
	}
	if errs := VerifyPluginConfig(parsed); len(errs) != 0 {
		t.Fatalf("VerifyPluginConfig without primary selector = %v, want none", errs)
	}

	// Register a synthetic primary selector whose RegisteredBackends callback counts calls.
	var verdictCalls int
	err = registry.Register(registry.Registration{
		Name:        "synthetic-primary-selector",
		Description: "test primary data-plane selector",
		RunEngine:   func(net.Conn) int { return 0 },
		CLIHandler:  func([]string) int { return 0 },
		DataPlaneSelector: &registry.DataPlaneSelector{
			LeafPath:         "/interface/backend",
			Roots:            []string{"interface"},
			SelectsDataPlane: true,
			RegisteredBackends: func() []string {
				verdictCalls++
				return []string{"kernel"}
			},
		},
	})
	if err != nil {
		t.Fatalf("register synthetic primary selector: %v", err)
	}

	// 1. LoadConfig door.
	if _, err := LoadConfig(sampleConfig, "test.conf", nil); err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if verdictCalls != 1 {
		t.Fatalf("after LoadConfig: verdictCalls = %d, want 1", verdictCalls)
	}

	// 2. ReadConfig door.
	if _, err := ReadConfig(sampleConfig, "test.conf", nil); err != nil {
		t.Fatalf("ReadConfig failed: %v", err)
	}
	if verdictCalls != 2 {
		t.Fatalf("after ReadConfig: verdictCalls = %d, want 2", verdictCalls)
	}

	// 3. VerifyPluginConfig door.
	if errs := VerifyPluginConfig(parsed); len(errs) != 0 {
		t.Fatalf("VerifyPluginConfig = %v, want none", errs)
	}
	if verdictCalls != 3 {
		t.Fatalf("after VerifyPluginConfig: verdictCalls = %d, want 3", verdictCalls)
	}

	// 4. Registered doctor check in DoctorPhaseParsedConfig.
	var doctorFn func(diagnostic.DoctorCheckContext) []diagnostic.Diagnostic
	for _, check := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhaseParsedConfig) {
		if check.Name == dataPlaneCheckName {
			doctorFn = check.Check
			break
		}
	}
	if doctorFn == nil {
		t.Fatalf("doctor check %q not found", dataPlaneCheckName)
	}
	if diags := doctorFn(diagnostic.DoctorCheckContext{Tree: parsed}); len(diags) != 0 {
		t.Fatalf("doctor check returned unexpected diagnostics: %+v", diags)
	}
	if verdictCalls != 4 {
		t.Fatalf("after doctor check: verdictCalls = %d, want 4", verdictCalls)
	}
}

// TestDataPlaneVerdictFailClosedAndFindingRendering checks that a finding
// produced by evaluateDataPlaneVerdict renders as an opererr.Error from
// refuseDataPlaneMismatches and as a CodeDoctorDataPlaneMismatch error
// diagnostic with Fix from dataPlaneMismatchDiagnostics.
//
// VALIDATES: fail-closed nil-schema refusal, opererr rendering, and diagnostic
// rendering for single and multiple findings.
// PREVENTS: nil schema silently passing or doctor diagnostics dropping Fix.
func TestDataPlaneVerdictFailClosedAndFindingRendering(t *testing.T) {
	isolateRegistry(t)

	err := registry.Register(registry.Registration{
		Name:        "synthetic-fail-closed-selector",
		Description: "test primary selector",
		RunEngine:   func(net.Conn) int { return 0 },
		CLIHandler:  func([]string) int { return 0 },
		DataPlaneSelector: &registry.DataPlaneSelector{
			LeafPath:           "/interface/backend",
			Roots:              []string{"interface"},
			SelectsDataPlane:   true,
			RegisteredBackends: func() []string { return []string{"kernel"} },
		},
	})
	if err != nil {
		t.Fatalf("register selector: %v", err)
	}

	tree := NewTree()
	refusal := refuseDataPlaneMismatches(tree, nil, nil)
	if refusal == nil {
		t.Fatal("refuseDataPlaneMismatches with nil schema = nil, want fail-closed error")
	}
	opErr, isOperErr := errors.AsType[*opererr.Error](refusal)
	if !isOperErr {
		t.Fatalf("refusal type = %T, want *opererr.Error", refusal)
	}
	if opErr.Fix() == "" {
		t.Errorf("opererr Fix is empty: %v", opErr)
	}

	diags := dataPlaneMismatchDiagnostics(tree, nil, nil)
	if len(diags) != 1 {
		t.Fatalf("dataPlaneMismatchDiagnostics len = %d, want 1", len(diags))
	}
	if diags[0].Code != CodeDoctorDataPlaneMismatch {
		t.Errorf("diagnostic Code = %q, want %q", diags[0].Code, CodeDoctorDataPlaneMismatch)
	}
	if diags[0].Severity != diagnostic.SeverityError {
		t.Errorf("diagnostic Severity = %q, want %q", diags[0].Severity, diagnostic.SeverityError)
	}
	if diags[0].Fix == "" {
		t.Error("diagnostic Fix is empty")
	}

	// Multiple findings join into one error and emit one diagnostic each.
	two := []dataPlaneFinding{
		{problem: "first mismatch", why: "first reason", fix: "set interface backend kernel"},
		{problem: "second mismatch", why: "second reason", fix: "set firewall backend vpp"},
	}
	joined := findingsToError(two)
	if joined == nil || !strings.Contains(joined.Error(), "first mismatch") || !strings.Contains(joined.Error(), "second mismatch") {
		t.Fatalf("findingsToError(two) = %v, want both findings joined", joined)
	}
	twoDiags := findingsToDiagnostics(two)
	if len(twoDiags) != 2 {
		t.Fatalf("findingsToDiagnostics(two) len = %d, want 2", len(twoDiags))
	}
	if twoDiags[0].Fix != "set interface backend kernel" || twoDiags[1].Fix != "set firewall backend vpp" {
		t.Errorf("unexpected diagnostic fixes: %+v", twoDiags)
	}
}

const verdictTestSchemaYANG = `module ze-test-dataplane-verdict-conf {
    yang-version 1.1;
    namespace "urn:ze:test:dataplane-verdict";
    prefix dv;
    container firewall {
        leaf backend {
            type string;
            default "kernel";
        }
    }
    container vpn {
        container ipsec {
            leaf enabled {
                type boolean;
                default "true";
            }
        }
    }
}`

func buildVerdictTestSchema(ifaceDefault, fwDefault, trafficDefault string) *Schema {
	s := NewSchema()
	s.Define("interface", Container(
		Field("backend", &LeafNode{Type: TypeString, Default: ifaceDefault}),
	))
	s.Define("firewall", Container(
		Field("backend", &LeafNode{Type: TypeString, Default: fwDefault}),
	))
	s.Define("traffic", Container(
		Field("control", Container(
			Field("backend", &LeafNode{Type: TypeString, Default: trafficDefault}),
		)),
	))
	s.Define("control-plane-protection", Container(
		Field("enabled", &LeafNode{Type: TypeBool, Default: "true"}),
	))
	s.Define("vpn", Container(
		Field("ipsec", Container(
			Field("enabled", &LeafNode{Type: TypeBool, Default: "true"}),
		)),
	))
	s.Define("sysctl", Container(
		Field("forwarding", &LeafNode{Type: TypeString}),
	))
	s.Define("fib", Container(
		Field("kernel", Container()),
		Field("vpp", Container()),
		Field("p4", Container()),
		Field("custom-vpp", Container()),
		Field("noop", Container()),
		Field("alpha", Container()),
		Field("beta", Container()),
		Field("unmarked", Container()),
	))
	s.Define("bus", Container(
		Field("endpoint", &ListNode{
			KeyName: "name",
			children: map[string]Node{
				"name":     Leaf(TypeString),
				"internal": Leaf(TypeString),
				"process":  Leaf(TypeString),
				"encoding": Leaf(TypeString),
				"external": Container(Field("run", Leaf(TypeString))),
			},
			order: []string{"name", "internal", "process", "encoding", "external"},
		}),
	))
	s.Define("bgp", Container(
		Field("peer", &ListNode{
			KeyName:  "name",
			children: map[string]Node{"name": Leaf(TypeString), "ttl": Leaf(TypeUint16)},
			order:    []string{"name", "ttl"},
		}),
	))
	return s
}

func registerTestSelectors(t *testing.T, ifaceBackends, fwBackends, trafficBackends []string) {
	t.Helper()
	mustRegister := func(reg registry.Registration) {
		t.Helper()
		reg.RunEngine = func(net.Conn) int { return 0 }
		reg.CLIHandler = func([]string) int { return 0 }
		if err := registry.Register(reg); err != nil {
			t.Fatalf("register %q: %v", reg.Name, err)
		}
	}
	mustRegister(registry.Registration{
		Name:        "interface",
		Description: "test interface selector",
		ConfigRoots: []string{"interface"},
		DataPlaneSelector: &registry.DataPlaneSelector{
			LeafPath:           "/interface/backend",
			Roots:              []string{"interface"},
			SelectsDataPlane:   true,
			RegisteredBackends: func() []string { return slices.Clone(ifaceBackends) },
		},
	})
	if fwBackends != nil {
		mustRegister(registry.Registration{
			Name:        "firewall",
			Description: "test firewall selector",
			ConfigRoots: []string{"firewall", "system/conntrack"},
			DataPlaneSelector: &registry.DataPlaneSelector{
				LeafPath:           "/firewall/backend",
				Roots:              []string{"firewall", "system/conntrack"},
				RegisteredBackends: func() []string { return slices.Clone(fwBackends) },
			},
		})
	}
	if trafficBackends != nil {
		mustRegister(registry.Registration{
			Name:        "traffic",
			Description: "test traffic selector",
			ConfigRoots: []string{"traffic/control"},
			DataPlaneSelector: &registry.DataPlaneSelector{
				LeafPath:           "/traffic/control/backend",
				Roots:              []string{"traffic/control"},
				RegisteredBackends: func() []string { return slices.Clone(trafficBackends) },
			},
		})
	}
}

func registerTestFlowspecProducer(t *testing.T) {
	t.Helper()
	err := registry.Register(registry.Registration{
		Name:        "flowspec-firewall",
		Description: "test flowspec firewall producer",
		RunEngine:   func(net.Conn) int { return 0 },
		CLIHandler:  func([]string) int { return 0 },
		DataPlaneFeatures: []registry.DataPlaneFeature{{
			ConfigPath:      "flowspec-firewall",
			Why:             "flowspec-firewall registers firewall tables",
			FollowsSelector: "firewall",
			InUseKind:       registry.DataPlaneInUseSelectedPlugin,
		}},
	})
	if err != nil {
		t.Fatalf("register flowspec-firewall: %v", err)
	}
}

func mustParseWithSchema(t *testing.T, schema *Schema, input string) *Tree {
	t.Helper()
	tree, err := NewParser(schema).Parse(input)
	if err != nil {
		t.Fatalf("parse %q: %v", input, err)
	}
	return tree
}

// TestDataPlaneVerdictRefusesForeignPlaneFeature proves an in-use feature on
// another plane (both ConfigRoot and Predicate with optional ConfigPath and
// SecondFix) is refused with the fix line and passes on its own plane.
//
// VALIDATES: AC-5 / AC-15 fixed-plane feature evaluation for plugin and
// standalone declarations.
// PREVENTS: kernel-only features silently committing under a VPP data plane.
func TestDataPlaneVerdictRefusesForeignPlaneFeature(t *testing.T) {
	isolateRegistry(t)
	registerTestSelectors(t, []string{"kernel", "vpp"}, nil, nil)

	if err := registry.Register(registry.Registration{
		Name:        "ike",
		Description: "test IPsec plugin",
		ConfigRoots: []string{"vpn/ipsec"},
		RunEngine:   func(net.Conn) int { return 0 },
		CLIHandler:  func([]string) int { return 0 },
		DataPlaneFeatures: []registry.DataPlaneFeature{{
			ConfigPath: "vpn ipsec",
			Why:        "IPsec programs kernel XFRM only",
			Planes:     []string{"kernel"},
			InUseKind:  registry.DataPlaneInUseConfigRoot,
		}},
	}); err != nil {
		t.Fatalf("register ike: %v", err)
	}
	if err := registry.RegisterDataPlaneFeature(registry.DataPlaneFeature{
		Name:       "gtsm",
		ConfigPath: "bgp peer ttl",
		Why:        "GTSM programs the Linux kernel only",
		Planes:     []string{"kernel"},
		InUseKind:  registry.DataPlaneInUsePredicate,
		InUse: func(tree map[string]any) registry.DataPlaneFeatureUse {
			bgp, ok := tree["bgp"].(map[string]any)
			if !ok {
				return registry.DataPlaneFeatureUse{}
			}
			peers, ok := bgp["peer"].(map[string]any)
			if !ok {
				return registry.DataPlaneFeatureUse{}
			}
			for peerName, raw := range peers {
				peer, isMap := raw.(map[string]any)
				if !isMap {
					continue
				}
				if ttl, ok := peer["ttl"].(string); ok && ttl != "" && ttl != "0" {
					return registry.DataPlaneFeatureUse{
						InUse:      true,
						ConfigPath: "bgp peer " + peerName + " ttl",
						SecondFix:  "delete bgp peer " + peerName + " ttl",
					}
				}
			}
			return registry.DataPlaneFeatureUse{}
		},
	}); err != nil {
		t.Fatalf("register standalone gtsm feature: %v", err)
	}

	schema := buildVerdictTestSchema("kernel", "kernel", "kernel")

	// Under VPP, both IPsec and active GTSM are refused and joined.
	vppTree := mustParseWithSchema(t, schema, `
interface { backend vpp; }
vpn { ipsec { enabled true; } }
bgp { peer p1 { ttl 1; } }
`)
	err := refuseDataPlaneMismatches(vppTree, schema, nil)
	if err == nil {
		t.Fatal("refuseDataPlaneMismatches(vppTree) = nil, want mismatch refusals")
	}
	msg := err.Error()
	for _, want := range []string{
		`vpn ipsec cannot run with interface backend "vpp"`,
		"IPsec programs kernel XFRM only",
		"fix: set interface backend kernel",
		`bgp peer p1 ttl cannot run with interface backend "vpp"`,
		"GTSM programs the Linux kernel only",
		"delete bgp peer p1 ttl",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("refusal missing %q:\n%s", want, msg)
		}
	}

	// On their declared plane (kernel), both are accepted.
	kernelTree := mustParseWithSchema(t, schema, `
interface { backend kernel; }
vpn { ipsec { enabled true; } }
bgp { peer p1 { ttl 1; } }
`)
	if err := refuseDataPlaneMismatches(kernelTree, schema, nil); err != nil {
		t.Fatalf("kernelTree refused unexpectedly: %v", err)
	}

	// Inactive predicate (ttl 0) and absent vpn/ipsec are accepted under VPP.
	idleTree := mustParseWithSchema(t, schema, `
interface { backend vpp; }
bgp { peer p1 { ttl 0; } }
`)
	if err := refuseDataPlaneMismatches(idleTree, schema, nil); err != nil {
		t.Fatalf("idleTree refused unexpectedly: %v", err)
	}
}

// TestDataPlaneVerdictAcceptsUnusedDefaultBackend verifies AC-3 and AC-4:
// unused secondary selectors with a kernel default refuse nothing under VPP,
// while an active block or producer mismatches until its selector matches.
//
// VALIDATES: AC-3 unused default survivor, AC-3 block/producer mismatch, and
// AC-4 explicit foreign secondary selector with both fixes.
// PREVENTS: refusing a clean VPP config that configures no firewall or QoS.
func TestDataPlaneVerdictAcceptsUnusedDefaultBackend(t *testing.T) {
	isolateRegistry(t)
	registerTestSelectors(t, []string{"kernel", "vpp"}, []string{"kernel", "vpp"}, []string{"kernel", "vpp"})

	if err := registry.Register(registry.Registration{
		Name:        "copp",
		Description: "test copp producer",
		ConfigRoots: []string{"control-plane-protection"},
		RunEngine:   func(net.Conn) int { return 0 },
		CLIHandler:  func([]string) int { return 0 },
		DataPlaneFeatures: []registry.DataPlaneFeature{{
			ConfigPath:      "control-plane-protection",
			Why:             "control-plane protection registers firewall tables",
			FollowsSelector: "firewall",
			InUseKind:       registry.DataPlaneInUseConfigRoot,
		}},
	}); err != nil {
		t.Fatalf("register copp: %v", err)
	}

	schema := buildVerdictTestSchema("kernel", "kernel", "kernel")

	// Unused default firewall and traffic selectors refuse nothing under VPP.
	vppOnly := mustParseWithSchema(t, schema, `interface { backend vpp; }`)
	if err := refuseDataPlaneMismatches(vppOnly, schema, nil); err != nil {
		t.Fatalf("unused default selectors refused under vpp: %v", err)
	}

	// Firewall block without explicit backend defaults to kernel -> refused with fix.
	vppWithFW := mustParseWithSchema(t, schema, `
interface { backend vpp; }
firewall { }
`)
	err := refuseDataPlaneMismatches(vppWithFW, schema, nil)
	if err == nil {
		t.Fatal("vppWithFW = nil, want firewall mismatch")
	}
	if !strings.Contains(err.Error(), "fix: set firewall backend vpp") {
		t.Errorf("vppWithFW error missing fix:\n%s", err)
	}

	// Table producer (copp) without firewall block uses default kernel -> refused.
	vppWithCopp := mustParseWithSchema(t, schema, `
interface { backend vpp; }
control-plane-protection { enabled true; }
`)
	err = refuseDataPlaneMismatches(vppWithCopp, schema, nil)
	if err == nil {
		t.Fatal("vppWithCopp = nil, want producer mismatch")
	}
	if !strings.Contains(err.Error(), "control-plane-protection") || !strings.Contains(err.Error(), "fix: set firewall backend vpp") {
		t.Errorf("vppWithCopp error = \n%s", err)
	}

	// Explicit firewall backend vpp under default kernel interface -> AC-4 both fixes.
	kernelWithVPPFW := mustParseWithSchema(t, schema, `
firewall { backend vpp; }
`)
	err = refuseDataPlaneMismatches(kernelWithVPPFW, schema, nil)
	if err == nil {
		t.Fatal("kernelWithVPPFW = nil, want AC-4 mismatch")
	}
	msg := err.Error()
	for _, want := range []string{"firewall backend", "set firewall backend kernel", "set interface backend vpp"} {
		if !strings.Contains(msg, want) {
			t.Errorf("AC-4 refusal missing %q:\n%s", want, msg)
		}
	}

	// Matching VPP selectors commit cleanly.
	allVPP := mustParseWithSchema(t, schema, `
interface { backend vpp; }
firewall { backend vpp; }
traffic { control { backend vpp; } }
control-plane-protection { enabled true; }
`)
	if err := refuseDataPlaneMismatches(allVPP, schema, nil); err != nil {
		t.Fatalf("allVPP refused unexpectedly: %v", err)
	}
}

// TestDataPlaneVerdictSelectorDefaultComesFromSchema proves that selector
// defaults are always read from the YANG schema and never hardcoded literals.
//
// VALIDATES: ResolvePrimaryDataPlane, ResolveDataPlaneSelector, and
// evaluateDataPlaneVerdict read non-standard defaults solely from schema.
// PREVENTS: a hardcoded "kernel" fallback masking schema defaults.
func TestDataPlaneVerdictSelectorDefaultComesFromSchema(t *testing.T) {
	isolateRegistry(t)
	registerTestSelectors(t, []string{"plane-alpha", "plane-beta"}, []string{"plane-alpha", "plane-beta"}, nil)

	schema := buildVerdictTestSchema("plane-alpha", "plane-beta", "plane-alpha")
	tree := mustParseWithSchema(t, schema, `firewall { }`)

	primaryVal, err := ResolvePrimaryDataPlane(tree, schema)
	if err != nil {
		t.Fatalf("ResolvePrimaryDataPlane: %v", err)
	}
	if primaryVal != "plane-alpha" {
		t.Fatalf("ResolvePrimaryDataPlane = %q, want plane-alpha", primaryVal)
	}

	fwVal, fwDef, err := ResolveDataPlaneSelector(tree, schema, "firewall")
	if err != nil {
		t.Fatalf("ResolveDataPlaneSelector(firewall): %v", err)
	}
	if fwVal != "plane-beta" || fwDef != "plane-beta" {
		t.Fatalf("ResolveDataPlaneSelector(firewall) = (%q, %q), want (plane-beta, plane-beta)", fwVal, fwDef)
	}

	refusal := refuseDataPlaneMismatches(tree, schema, nil)
	if refusal == nil {
		t.Fatal("refuseDataPlaneMismatches = nil, want mismatch between plane-alpha and plane-beta")
	}
	if !strings.Contains(refusal.Error(), "fix: set firewall backend plane-alpha") {
		t.Errorf("refusal did not derive fix from schema default:\n%s", refusal)
	}
}

// TestDataPlaneVerdictRefusesUnreadableSelector proves that any unreadable
// primary or active secondary selector fails closed rather than reporting no
// mismatch.
//
// VALIDATES: fail-closed refusals for nil schema, missing leaf, non-leaf path,
// empty schema default, and unreadable active secondary selector.
// PREVENTS: an unreadable selector silently passing as "no mismatch".
func TestDataPlaneVerdictRefusesUnreadableSelector(t *testing.T) {
	isolateRegistry(t)
	registerTestSelectors(t, []string{"kernel", "vpp"}, []string{"kernel", "vpp"}, nil)

	emptyTree := NewTree()

	// 1. Nil schema.
	if err := refuseDataPlaneMismatches(emptyTree, nil, nil); err == nil {
		t.Error("nil schema = nil, want fail-closed refusal")
	}
	if _, err := ResolvePrimaryDataPlane(emptyTree, nil); err == nil {
		t.Error("ResolvePrimaryDataPlane(nil schema) = nil, want error")
	}

	// 2. Primary LeafPath absent from schema.
	missingLeafSchema := NewSchema()
	if err := refuseDataPlaneMismatches(emptyTree, missingLeafSchema, nil); err == nil {
		t.Error("missing primary leaf = nil, want fail-closed refusal")
	}

	// 3. Primary LeafPath points to a container instead of a leaf.
	containerLeafSchema := NewSchema()
	containerLeafSchema.Define("interface", Container(Field("backend", Container())))
	if err := refuseDataPlaneMismatches(emptyTree, containerLeafSchema, nil); err == nil {
		t.Error("non-leaf primary path = nil, want fail-closed refusal")
	}

	// 4. Primary leaf has no written value and no schema default.
	noDefaultSchema := buildVerdictTestSchema("", "kernel", "kernel")
	if err := refuseDataPlaneMismatches(emptyTree, noDefaultSchema, nil); err == nil {
		t.Error("missing primary default = nil, want fail-closed refusal")
	}

	// 5. Active secondary selector leaf has no schema default.
	badSecondarySchema := buildVerdictTestSchema("kernel", "", "kernel")
	fwTree := mustParseWithSchema(t, badSecondarySchema, `firewall { }`)
	if err := refuseDataPlaneMismatches(fwTree, badSecondarySchema, nil); err == nil {
		t.Error("unreadable active secondary selector = nil, want fail-closed refusal")
	}
}

// TestDataPlaneVerdictJudgesSelectedPluginWithoutConfigRoot proves AC-3 for a
// plugin with no config root (flowspec-firewall): it is active when selected
// via `plugin internal`, `plugin external` (under any key), or `--plugin`, and
// inactive otherwise or when an external block runs another program.
//
// VALIDATES: DataPlaneInUseSelectedPlugin matching and refusal/fix wording for
// both plugin blocks and --plugin arguments.
// PREVENTS: rootless firewall table producers bypassing the data-plane guard.
func TestDataPlaneVerdictJudgesSelectedPluginWithoutConfigRoot(t *testing.T) {
	isolateRegistry(t)
	registerTestSelectors(t, []string{"kernel", "vpp"}, []string{"kernel", "vpp"}, nil)
	registerTestFlowspecProducer(t)

	schema := buildVerdictTestSchema("kernel", "kernel", "kernel")
	schema.Define("bus", Container(
		Field("endpoint", &ListNode{
			KeyName: "name",
			children: map[string]Node{
				"name":     Leaf(TypeString),
				"internal": Leaf(TypeString),
				"external": Container(Field("run", Leaf(TypeString))),
			},
			order: []string{"name", "internal", "external"},
		}),
	))

	// Not selected -> accepted under VPP.
	vppOnly := mustParseWithSchema(t, schema, `interface { backend vpp; }`)
	if err := refuseDataPlaneMismatches(vppOnly, schema, nil); err != nil {
		t.Fatalf("unselected flowspec-firewall refused: %v", err)
	}

	// Selected via bus endpoint under a custom key -> refused naming endpoint and both fixes.
	internalTree := mustParseWithSchema(t, schema, `
interface { backend vpp; }
bus {
	endpoint my-fs {
		internal flowspec-firewall;
	}
}
`)
	internalSel, err := ExtractPluginsFromTree(internalTree)
	if err != nil {
		t.Fatalf("ExtractPluginsFromTree(internalTree): %v", err)
	}
	refusal := refuseDataPlaneMismatches(internalTree, schema, internalSel)
	if refusal == nil {
		t.Fatal("endpoint internal selection = nil, want refusal")
	}
	for _, want := range []string{
		"bus endpoint my-fs",
		"set firewall backend vpp",
		"delete bus endpoint my-fs",
	} {
		if !strings.Contains(refusal.Error(), want) {
			t.Errorf("endpoint refusal missing %q:\n%s", want, refusal)
		}
	}

	// Selected via bus endpoint running ze.flowspec-firewall externally -> refused naming endpoint and both fixes.
	externalUseTree := mustParseWithSchema(t, schema, `
interface { backend vpp; }
bus {
	endpoint ext-fs {
		external {
			run "ze.flowspec-firewall";
		}
	}
}
`)
	externalSel, err := ExtractPluginsFromTree(externalUseTree)
	if err != nil {
		t.Fatalf("ExtractPluginsFromTree(externalUseTree): %v", err)
	}
	refusal = refuseDataPlaneMismatches(externalUseTree, schema, externalSel)
	if refusal == nil {
		t.Fatal("external endpoint selection = nil, want refusal")
	}
	for _, want := range []string{
		"bus endpoint ext-fs",
		"set firewall backend vpp",
		"delete bus endpoint ext-fs",
	} {
		if !strings.Contains(refusal.Error(), want) {
			t.Errorf("external endpoint refusal missing %q:\n%s", want, refusal)
		}
	}

	// Selected via --plugin -> refused naming --plugin argument and both fixes.
	cliSel, err := MergeCliPlugins(nil, []string{"ze.flowspec-firewall"})
	if err != nil {
		t.Fatalf("MergeCliPlugins: %v", err)
	}
	refusal = refuseDataPlaneMismatches(vppOnly, schema, cliSel)
	if refusal == nil {
		t.Fatal("--plugin selection = nil, want refusal")
	}
	for _, want := range []string{
		"--plugin ze.flowspec-firewall",
		"set firewall backend vpp",
		"start without --plugin ze.flowspec-firewall",
	} {
		if !strings.Contains(refusal.Error(), want) {
			t.Errorf("--plugin refusal missing %q:\n%s", want, refusal)
		}
	}

	// Genuinely external binary sharing the endpoint key does NOT trigger the compiled-in declaration.
	unrelatedExternal := mustParseWithSchema(t, schema, `
interface { backend vpp; }
bus {
	endpoint flowspec-firewall {
		external {
			run "/usr/local/bin/unrelated-plugin";
		}
	}
}
`)
	unrelatedSel, err := ExtractPluginsFromTree(unrelatedExternal)
	if err != nil {
		t.Fatalf("ExtractPluginsFromTree(unrelatedExternal): %v", err)
	}
	if err := refuseDataPlaneMismatches(unrelatedExternal, schema, unrelatedSel); err != nil {
		t.Fatalf("unrelated external binary falsely triggered flowspec-firewall: %v", err)
	}

	// With firewall backend vpp alongside it -> accepted.
	matchedTree := mustParseWithSchema(t, schema, `
interface { backend vpp; }
firewall { backend vpp; }
`)
	if err := refuseDataPlaneMismatches(matchedTree, schema, cliSel); err != nil {
		t.Fatalf("flowspec-firewall with firewall backend vpp refused: %v", err)
	}
}

// TestDataPlaneVerdictMarks proves AC-5 / AC-14 host-side mark semantics: a
// registration setting HostSideDataPlane produces no plane mismatch on another
// plane, while an unmarked registration on the same plane still refuses.
//
// VALIDATES: HostSideDataPlane suppresses fixed-plane mismatch without naming
// any plugin.
// PREVENTS: sysctl being refused under VPP or an unmarked kernel feature being
// silently exempted.
func TestDataPlaneVerdictMarks(t *testing.T) {
	isolateRegistry(t)
	registerTestSelectors(t, []string{"kernel", "vpp"}, nil, nil)

	if err := registry.Register(registry.Registration{
		Name:              "sysctl",
		Description:       "test host-side sysctl",
		ConfigRoots:       []string{"sysctl"},
		HostSideDataPlane: true,
		RunEngine:         func(net.Conn) int { return 0 },
		CLIHandler:        func([]string) int { return 0 },
		DataPlaneFeatures: []registry.DataPlaneFeature{{
			ConfigPath: "sysctl",
			Why:        "sysctl sets Linux kernel tunables",
			Planes:     []string{"kernel"},
			InUseKind:  registry.DataPlaneInUseConfigRoot,
		}},
	}); err != nil {
		t.Fatalf("register sysctl: %v", err)
	}
	if err := registry.Register(registry.Registration{
		Name:        "ike",
		Description: "test unmarked kernel feature",
		ConfigRoots: []string{"vpn/ipsec"},
		RunEngine:   func(net.Conn) int { return 0 },
		CLIHandler:  func([]string) int { return 0 },
		DataPlaneFeatures: []registry.DataPlaneFeature{{
			ConfigPath: "vpn ipsec",
			Why:        "IPsec programs kernel XFRM only",
			Planes:     []string{"kernel"},
			InUseKind:  registry.DataPlaneInUseConfigRoot,
		}},
	}); err != nil {
		t.Fatalf("register ike: %v", err)
	}

	schema := buildVerdictTestSchema("kernel", "kernel", "kernel")

	// Host-side marked feature commits cleanly under VPP.
	sysctlTree := mustParseWithSchema(t, schema, `
interface { backend vpp; }
sysctl { forwarding 1; }
`)
	if err := refuseDataPlaneMismatches(sysctlTree, schema, nil); err != nil {
		t.Fatalf("HostSideDataPlane feature refused under vpp: %v", err)
	}

	// Unmarked feature on the same plane is still refused.
	unmarkedTree := mustParseWithSchema(t, schema, `
interface { backend vpp; }
sysctl { forwarding 1; }
vpn { ipsec { enabled true; } }
`)
	err := refuseDataPlaneMismatches(unmarkedTree, schema, nil)
	if err == nil {
		t.Fatal("unmarked kernel feature under vpp = nil, want refusal")
	}
	if strings.Contains(err.Error(), "sysctl") {
		t.Errorf("HostSideDataPlane sysctl unexpectedly appeared in refusal:\n%s", err)
	}
	if !strings.Contains(err.Error(), "vpn ipsec") {
		t.Errorf("unmarked vpn ipsec missing from refusal:\n%s", err)
	}
}

// TestDataPlaneVerdictRefusesMarkedFIBWriterAlone proves AC-1 and AC-14:
// under VPP, a host-side marked kernel FIB writer alone is refused with a fix
// derived from the registered VPP FIB writer's ConfigRoots; beside the VPP FIB
// writer it is accepted; an unmarked foreign FIB writer gets only the AC-1
// refusal; and a FIB writer declaring no DataPlane (fib-p4) is not judged.
//
// VALIDATES: AC-1 and AC-14 (P-1b) FIB writer rules and derived fix root.
// PREVENTS: booting a VPP router with only fib kernel and no VPP route writer,
// or double-refusing an unmarked foreign FIB writer.
func TestDataPlaneVerdictRefusesMarkedFIBWriterAlone(t *testing.T) {
	isolateRegistry(t)
	registerTestSelectors(t, []string{"kernel", "vpp"}, nil, nil)

	registerFIB := func(name, root, plane string, hostSide bool) {
		t.Helper()
		if err := registry.Register(registry.Registration{
			Name:              name,
			Description:       "test fib writer",
			ConfigRoots:       []string{root},
			ProgramsFIB:       true,
			DataPlane:         plane,
			HostSideDataPlane: hostSide,
			RunEngine:         func(net.Conn) int { return 0 },
			CLIHandler:        func([]string) int { return 0 },
		}); err != nil {
			t.Fatalf("register %s: %v", name, err)
		}
	}
	registerFIB("fib-kernel", "fib/kernel", "kernel", true)
	registerFIB("fib-custom-vpp", "fib/custom-vpp", "vpp", false)
	registerFIB("fib-p4", "fib/p4", "", false)

	schema := buildVerdictTestSchema("kernel", "kernel", "kernel")

	// Marked fib-kernel alone under VPP -> refused with derived fix naming fib/custom-vpp.
	aloneTree := mustParseWithSchema(t, schema, `
interface { backend vpp; }
fib { kernel { } }
`)
	findings := evaluateDataPlaneVerdict(aloneTree, schema, nil)
	if len(findings) != 1 {
		t.Fatalf("marked FIB alone findings = %d, want 1: %+v", len(findings), findings)
	}
	if findings[0].fix != "set fib custom-vpp" {
		t.Errorf("fix = %q, want derived %q", findings[0].fix, "set fib custom-vpp")
	}
	if !strings.Contains(findings[0].problem, "fib kernel") || !strings.Contains(findings[0].why, "fib custom-vpp") {
		t.Errorf("unexpected marked-FIB-alone finding: %+v", findings[0])
	}

	// Marked fib-kernel beside the selected-plane VPP writer -> accepted.
	bothTree := mustParseWithSchema(t, schema, `
interface { backend vpp; }
fib { kernel { } custom-vpp { } }
`)
	if err := refuseDataPlaneMismatches(bothTree, schema, nil); err != nil {
		t.Fatalf("fib kernel beside selected-plane writer refused: %v", err)
	}

	// Unmarked fib-custom-vpp under kernel -> exactly one AC-1 refusal, not two.
	foreignUnmarked := mustParseWithSchema(t, schema, `
interface { backend kernel; }
fib { custom-vpp { } }
`)
	findings = evaluateDataPlaneVerdict(foreignUnmarked, schema, nil)
	if len(findings) != 1 {
		t.Fatalf("unmarked foreign FIB findings = %d, want 1: %+v", len(findings), findings)
	}
	if findings[0].fix != "set interface backend vpp" {
		t.Errorf("AC-1 fix = %q, want %q", findings[0].fix, "set interface backend vpp")
	}

	// FIB plugin declaring no DataPlane (fib-p4) is not judged on either plane.
	p4Tree := mustParseWithSchema(t, schema, `
interface { backend vpp; }
fib { p4 { } }
`)
	if err := refuseDataPlaneMismatches(p4Tree, schema, nil); err != nil {
		t.Fatalf("fib-p4 without DataPlane refused: %v", err)
	}
}

// TestCandidateSelectionMergesRecordedCommandLinePlugins proves AC-18:
// candidatePluginSelection merges tree blocks with CommandLinePlugins;
// verifyTree and LoadConfig refuse the same tree with the same words under one
// record and accept it under an empty record; an unresolvable recorded plugin
// name is refused at both doors.
//
// VALIDATES: candidatePluginSelection, VerifyPluginConfig, and LoadConfig
// share one recorded-plugin merge and refusal behavior.
// PREVENTS: validate/commit accepting what daemon start refuses.
func TestCandidateSelectionMergesRecordedCommandLinePlugins(t *testing.T) {
	t.Cleanup(ResetDeclaredSchemasForTest())
	t.Cleanup(configyang.RegisterModuleForTest("ze-test-dataplane-verdict-conf.yang", verdictTestSchemaYANG))
	isolateRegistry(t)
	registerTestSelectors(t, []string{"kernel", "vpp"}, []string{"kernel", "vpp"}, nil)
	registerTestFlowspecProducer(t)
	registerProvider(t, "extra-plugin")

	const cfg = "interface {\n  backend vpp;\n}\n"

	// Tree bus endpoint plus recorded command-line plugin both appear in selection.
	treeWithBlock := mustParsePluginVerifyConfig(t, cfg+"bus { endpoint e1 { internal extra-plugin; } }\n")
	RecordCommandLinePlugins([]string{"ze.flowspec-firewall"})
	sel, err := candidatePluginSelection(treeWithBlock, nil)
	if err != nil {
		t.Fatalf("candidatePluginSelection: %v", err)
	}
	if len(sel) != 2 {
		t.Fatalf("candidatePluginSelection len = %d, want 2: %+v", len(sel), sel)
	}

	// Both verifyTree and LoadConfig refuse with the same words under the record.
	tree := mustParsePluginVerifyConfig(t, cfg)
	verifyErrs := VerifyPluginConfig(tree)
	if len(verifyErrs) != 1 {
		t.Fatalf("VerifyPluginConfig errors = %v, want 1", verifyErrs)
	}
	_, loadErr := LoadConfig(cfg, "test.conf", nil)
	if loadErr == nil {
		t.Fatal("LoadConfig with recorded flowspec-firewall = nil, want refusal")
	}
	if verifyErrs[0].Error() != loadErr.Error() {
		t.Errorf("door messages differ:\nverifyTree: %s\nLoadConfig: %s", verifyErrs[0], loadErr)
	}

	// Empty record -> both accept.
	RecordCommandLinePlugins(nil)
	if errs := VerifyPluginConfig(tree); len(errs) != 0 {
		t.Fatalf("VerifyPluginConfig with empty record = %v, want none", errs)
	}
	if _, err := LoadConfig(cfg, "test.conf", nil); err != nil {
		t.Fatalf("LoadConfig with empty record failed: %v", err)
	}

	// Unresolvable recorded name -> refused at both doors.
	RecordCommandLinePlugins([]string{"ze.no-such-plugin"})
	verifyErrs = VerifyPluginConfig(tree)
	if len(verifyErrs) != 1 {
		t.Fatalf("VerifyPluginConfig unresolvable record = %v, want 1 error", verifyErrs)
	}
	_, loadErr = LoadConfig(cfg, "test.conf", nil)
	if loadErr == nil {
		t.Fatal("LoadConfig with unresolvable recorded plugin = nil, want error")
	}
	if !strings.Contains(verifyErrs[0].Error(), "ze.no-such-plugin") || !strings.Contains(loadErr.Error(), "ze.no-such-plugin") {
		t.Errorf("unresolvable plugin not named at both doors:\nverify: %v\nload: %v", verifyErrs[0], loadErr)
	}
	if verifyErrs[0].Error() != loadErr.Error() {
		t.Errorf("unresolvable refusal differs:\nverify: %s\nload: %s", verifyErrs[0], loadErr)
	}
}

// TestLoadConfigVerdictReadsItsCommandLinePlugins proves AC-18 / R-9:
// LoadConfig merges both its cliPlugins parameter (used when no ze dispatcher
// ran, such as le chaos run) and CommandLinePlugins (used when cliPlugins is
// nil, such as ze doctor and managed push), deduplicating when both hold the
// same plugin.
//
// VALIDATES: LoadConfig verdict with parameter-only, record-only, and both.
// PREVENTS: in-process callers with empty records bypassing the verdict or
// double-merging a plugin present in both.
func TestLoadConfigVerdictReadsItsCommandLinePlugins(t *testing.T) {
	t.Cleanup(ResetDeclaredSchemasForTest())
	t.Cleanup(configyang.RegisterModuleForTest("ze-test-dataplane-verdict-conf.yang", verdictTestSchemaYANG))
	isolateRegistry(t)
	registerTestSelectors(t, []string{"kernel", "vpp"}, []string{"kernel", "vpp"}, nil)
	registerTestFlowspecProducer(t)

	const cfg = "interface {\n  backend vpp;\n}\n"

	// 1. Empty record, parameter carries ze.flowspec-firewall (chaos runner shape).
	RecordCommandLinePlugins(nil)
	_, paramErr := LoadConfig(cfg, "test.conf", []string{"ze.flowspec-firewall"})
	if paramErr == nil {
		t.Fatal("LoadConfig(param-only) = nil, want refusal")
	}

	// 2. Record carries ze.flowspec-firewall, parameter is nil (doctor / managed push shape).
	RecordCommandLinePlugins([]string{"ze.flowspec-firewall"})
	_, recordErr := LoadConfig(cfg, "test.conf", nil)
	if recordErr == nil {
		t.Fatal("LoadConfig(record-only) = nil, want refusal")
	}
	if paramErr.Error() != recordErr.Error() {
		t.Errorf("param-only and record-only refusals differ:\nparam:  %s\nrecord: %s", paramErr, recordErr)
	}

	// 3. Both hold ze.flowspec-firewall -> merged once and same refusal.
	tree := mustParsePluginVerifyConfig(t, cfg)
	sel, err := candidatePluginSelection(tree, []string{"ze.flowspec-firewall"})
	if err != nil {
		t.Fatalf("candidatePluginSelection(both): %v", err)
	}
	if len(sel) != 1 {
		t.Fatalf("candidatePluginSelection(both) len = %d, want 1 (merged once): %+v", len(sel), sel)
	}
	_, bothErr := LoadConfig(cfg, "test.conf", []string{"ze.flowspec-firewall"})
	if bothErr == nil {
		t.Fatal("LoadConfig(both) = nil, want refusal")
	}
	if bothErr.Error() != recordErr.Error() {
		t.Errorf("both and record-only refusals differ:\nboth:   %s\nrecord: %s", bothErr, recordErr)
	}
}

// TestVerdictIsTheSameAtStartAndReload proves AC-16: LoadConfig (used at start
// and reload) refuses an IPsec mismatch and a firewall mismatch with the exact
// same words VerifyPluginConfig gives, with no door-specific input.
//
// VALIDATES: AC-16 single verdict across start, reload, commit, and validate.
// PREVENTS: start accepting an IPsec or firewall mismatch that validate refuses.
func TestVerdictIsTheSameAtStartAndReload(t *testing.T) {
	t.Cleanup(ResetDeclaredSchemasForTest())
	t.Cleanup(configyang.RegisterModuleForTest("ze-test-dataplane-verdict-conf.yang", verdictTestSchemaYANG))
	isolateRegistry(t)
	registerTestSelectors(t, []string{"kernel", "vpp"}, []string{"kernel", "vpp"}, nil)

	if err := registry.Register(registry.Registration{
		Name:        "ike",
		Description: "test IPsec plugin",
		ConfigRoots: []string{"vpn/ipsec"},
		RunEngine:   func(net.Conn) int { return 0 },
		CLIHandler:  func([]string) int { return 0 },
		DataPlaneFeatures: []registry.DataPlaneFeature{{
			ConfigPath: "vpn ipsec",
			Why:        "IPsec programs kernel XFRM only",
			Planes:     []string{"kernel"},
			InUseKind:  registry.DataPlaneInUseConfigRoot,
		}},
	}); err != nil {
		t.Fatalf("register ike: %v", err)
	}

	cases := []struct {
		name string
		cfg  string
		want string
	}{
		{
			name: "ipsec under vpp",
			cfg:  "interface {\n  backend vpp;\n}\nvpn {\n  ipsec {\n    enabled true;\n  }\n}\n",
			want: "set interface backend kernel",
		},
		{
			name: "kernel firewall under vpp",
			cfg:  "interface {\n  backend vpp;\n}\nfirewall {\n}\n",
			want: "set firewall backend vpp",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := mustParsePluginVerifyConfig(t, tc.cfg)
			verifyErrs := VerifyPluginConfig(tree)
			if len(verifyErrs) != 1 {
				t.Fatalf("VerifyPluginConfig errors = %v, want 1", verifyErrs)
			}
			_, loadErr := LoadConfig(tc.cfg, "test.conf", nil)
			if loadErr == nil {
				t.Fatalf("LoadConfig = nil, want refusal containing %q", tc.want)
			}
			_, readErr := ReadConfig(tc.cfg, "test.conf", nil)
			if readErr == nil {
				t.Fatalf("ReadConfig = nil, want refusal containing %q", tc.want)
			}
			if verifyErrs[0].Error() != loadErr.Error() {
				t.Errorf("VerifyPluginConfig vs LoadConfig differ:\nverify: %s\nload:   %s", verifyErrs[0], loadErr)
			}
			if readErr.Error() != loadErr.Error() {
				t.Errorf("ReadConfig vs LoadConfig differ:\nread: %s\nload: %s", readErr, loadErr)
			}
			if !strings.Contains(loadErr.Error(), tc.want) {
				t.Errorf("refusal missing %q:\n%s", tc.want, loadErr)
			}
			if _, ok := errors.AsType[*opererr.Error](loadErr); !ok {
				t.Errorf("refusal type = %T, want *opererr.Error", loadErr)
			}
		})
	}
}

// Compile-time check that evaluateDataPlaneVerdict accepts []plugin.PluginConfig
// without reading any process record.
var _ func(*Tree, *Schema, []plugin.PluginConfig) []dataPlaneFinding = evaluateDataPlaneVerdict

func registerTestNoPlaneFIBSuite(t *testing.T) {
	t.Helper()
	registerFIB := func(name, root, plane string, hostSide, noPlane bool) {
		t.Helper()
		if err := registry.Register(registry.Registration{
			Name:              name,
			Description:       "test fib plugin " + name,
			ConfigRoots:       []string{root},
			ProgramsFIB:       true,
			DataPlane:         plane,
			HostSideDataPlane: hostSide,
			NoDataPlaneChoice: noPlane,
			RunEngine:         func(net.Conn) int { return 0 },
			CLIHandler:        func([]string) int { return 0 },
		}); err != nil {
			t.Fatalf("register %s: %v", name, err)
		}
	}
	registerFIB("fib-noop", "fib/noop", "", false, true)
	registerFIB("fib-alpha", "fib/alpha", "alpha", true, false)
	registerFIB("fib-beta", "fib/beta", "beta", false, false)
	registerFIB("fib-unmarked", "fib/unmarked", "", false, false)
}

// TestVerdictRefusesNoPlaneChoiceBesideConfigRoot verifies that a
// NoDataPlaneChoice FIB plugin configured beside any other FIB plugin's config
// root produces the exact three-line finding derived from registrations, while
// leaving evaluateFIBWriters' foreign-plane findings intact beside it.
//
// VALIDATES: evaluateNoPlaneFIBChoice for same-plane, foreign-plane unmarked,
// foreign-plane host-side, and no-plane unmarked FIB plugins.
// PREVENTS: a no-plane FIB choice committing alongside a real or other FIB writer.
func TestVerdictRefusesNoPlaneChoiceBesideConfigRoot(t *testing.T) {
	isolateRegistry(t)
	registerTestSelectors(t, []string{"alpha", "beta"}, nil, nil)
	registerTestNoPlaneFIBSuite(t)

	schema := buildVerdictTestSchema("alpha", "alpha", "alpha")

	// Beside same-plane writer: exactly one finding from the no-plane rule.
	samePlane := mustParseWithSchema(t, schema, `fib { noop { } alpha { } }`)
	findings := evaluateDataPlaneVerdict(samePlane, schema, nil)
	if len(findings) != 1 {
		t.Fatalf("same-plane findings = %d, want 1: %+v", len(findings), findings)
	}
	if findings[0].problem != "fib noop is configured beside fib alpha" {
		t.Errorf("problem = %q, want %q", findings[0].problem, "fib noop is configured beside fib alpha")
	}
	if findings[0].why != "fib noop is the choice to program no route, and fib alpha programs the alpha data plane" {
		t.Errorf("why = %q", findings[0].why)
	}
	if findings[0].fix != "delete fib noop, or delete fib alpha" {
		t.Errorf("fix = %q, want %q", findings[0].fix, "delete fib noop, or delete fib alpha")
	}

	// Beside foreign-plane unmarked writer: guard's AC-1 finding PLUS this rule's finding.
	foreignUnmarked := mustParseWithSchema(t, schema, `fib { noop { } beta { } }`)
	findings = evaluateDataPlaneVerdict(foreignUnmarked, schema, nil)
	if len(findings) != 2 {
		t.Fatalf("foreign-unmarked findings = %d, want 2 (guard + no-plane): %+v", len(findings), findings)
	}
	if findings[1].problem != "fib noop is configured beside fib beta" {
		t.Errorf("second finding problem = %q", findings[1].problem)
	}
	if findings[1].why != "fib noop is the choice to program no route, and fib beta programs the beta data plane" {
		t.Errorf("second finding why = %q", findings[1].why)
	}
	if findings[1].fix != "delete fib noop, or delete fib beta" {
		t.Errorf("second finding fix = %q", findings[1].fix)
	}

	// Beside foreign-plane host-side writer under beta: guard's AC-14 finding PLUS this rule's finding.
	foreignHost := mustParseWithSchema(t, schema, `
interface { backend beta; }
fib { noop { } alpha { } }
`)
	findings = evaluateDataPlaneVerdict(foreignHost, schema, nil)
	if len(findings) != 2 {
		t.Fatalf("foreign-host findings = %d, want 2 (guard AC-14 + no-plane): %+v", len(findings), findings)
	}
	if findings[1].problem != "fib noop is configured beside fib alpha" {
		t.Errorf("second finding problem = %q", findings[1].problem)
	}

	// Beside unmarked no-plane FIB plugin: exactly one finding.
	unmarkedNoPlane := mustParseWithSchema(t, schema, `fib { noop { } unmarked { } }`)
	findings = evaluateDataPlaneVerdict(unmarkedNoPlane, schema, nil)
	if len(findings) != 1 {
		t.Fatalf("unmarked-no-plane findings = %d, want 1: %+v", len(findings), findings)
	}
	if findings[0].problem != "fib noop is configured beside fib unmarked" {
		t.Errorf("problem = %q", findings[0].problem)
	}
	if findings[0].why != "fib noop is the choice to program no route, and fib unmarked is another FIB plugin" {
		t.Errorf("why = %q", findings[0].why)
	}
	if findings[0].fix != "delete fib noop, or delete fib unmarked" {
		t.Errorf("fix = %q", findings[0].fix)
	}
}

// TestVerdictRefusesNoPlaneChoiceBesideSelectedPlugin proves that a
// NoDataPlaneChoice FIB plugin configured beside another FIB plugin selected
// only by a bus endpoint (in-daemon or separate process) or by --plugin is
// refused with the exact What/Why/Fix lines, while a foreign external endpoint
// sharing the name is not treated as selecting the compiled plugin.
//
// VALIDATES: evaluateNoPlaneFIBChoice selection matching across bus endpoints,
// --plugin entries, and external endpoints.
// PREVENTS: bypassing the no-plane refusal through `bus { endpoint }` or `--plugin`.
func TestVerdictRefusesNoPlaneChoiceBesideSelectedPlugin(t *testing.T) {
	isolateRegistry(t)
	registerTestSelectors(t, []string{"alpha", "beta"}, nil, nil)
	registerTestNoPlaneFIBSuite(t)

	schema := buildVerdictTestSchema("alpha", "alpha", "alpha")

	// Bus endpoint (default in-daemon) selecting fib-alpha.
	inDaemonTree := mustParseWithSchema(t, schema, `
fib { noop { } }
bus {
	endpoint k {
		internal fib-alpha;
	}
}
`)
	inDaemonSel, err := ExtractPluginsFromTree(inDaemonTree)
	if err != nil {
		t.Fatalf("ExtractPluginsFromTree(inDaemonTree): %v", err)
	}
	findings := evaluateDataPlaneVerdict(inDaemonTree, schema, inDaemonSel)
	if len(findings) != 1 {
		t.Fatalf("in-daemon endpoint findings = %d, want 1: %+v", len(findings), findings)
	}
	if findings[0].problem != "fib noop is configured and the plugin fib-alpha is selected" {
		t.Errorf("problem = %q", findings[0].problem)
	}
	if findings[0].why != "fib noop is the choice to program no route, and fib alpha programs the alpha data plane" {
		t.Errorf("why = %q", findings[0].why)
	}
	if findings[0].fix != "delete fib noop, or delete bus endpoint k" {
		t.Errorf("fix = %q", findings[0].fix)
	}

	// Bus endpoint with process separate selecting fib-beta (foreign plane, but
	// selected by endpoint only so evaluateFIBWriters adds no second finding).
	sepTree := mustParseWithSchema(t, schema, `
fib { noop { } }
bus {
	endpoint w {
		internal fib-beta;
		process separate;
	}
}
`)
	sepSel, err := ExtractPluginsFromTree(sepTree)
	if err != nil {
		t.Fatalf("ExtractPluginsFromTree(sepTree): %v", err)
	}
	findings = evaluateDataPlaneVerdict(sepTree, schema, sepSel)
	if len(findings) != 1 {
		t.Fatalf("separate-process endpoint findings = %d, want 1: %+v", len(findings), findings)
	}
	if findings[0].problem != "fib noop is configured and the plugin fib-beta is selected" {
		t.Errorf("problem = %q", findings[0].problem)
	}
	if findings[0].why != "fib noop is the choice to program no route, and fib beta programs the beta data plane" {
		t.Errorf("why = %q", findings[0].why)
	}
	if findings[0].fix != "delete fib noop, or delete bus endpoint w" {
		t.Errorf("fix = %q", findings[0].fix)
	}

	// Selected via --plugin ze.fib-alpha with no fib alpha block.
	noopOnly := mustParseWithSchema(t, schema, `fib { noop { } }`)
	cliSel, err := MergeCliPlugins(nil, []string{"ze.fib-alpha"})
	if err != nil {
		t.Fatalf("MergeCliPlugins: %v", err)
	}
	findings = evaluateDataPlaneVerdict(noopOnly, schema, cliSel)
	if len(findings) != 1 {
		t.Fatalf("--plugin findings = %d, want 1: %+v", len(findings), findings)
	}
	if findings[0].problem != "fib noop is configured and the plugin fib-alpha is selected" {
		t.Errorf("problem = %q", findings[0].problem)
	}
	if findings[0].why != "fib noop is the choice to program no route, and fib alpha programs the alpha data plane" {
		t.Errorf("why = %q", findings[0].why)
	}
	if findings[0].fix != "delete fib noop, or start ze without --plugin ze.fib-alpha" {
		t.Errorf("fix = %q", findings[0].fix)
	}

	// External endpoint reusing a compiled FIB plugin's name does NOT select it.
	extTree := mustParseWithSchema(t, schema, `
fib { noop { } }
bus {
	endpoint fib-alpha {
		external {
			run "/usr/local/bin/unrelated";
		}
	}
}
`)
	extSel, err := ExtractPluginsFromTree(extTree)
	if err != nil {
		t.Fatalf("ExtractPluginsFromTree(extTree): %v", err)
	}
	if findings := evaluateDataPlaneVerdict(extTree, schema, extSel); len(findings) != 0 {
		t.Fatalf("external endpoint falsely triggered no-plane rule: %+v", findings)
	}
}

// TestVerdictWhyNamesThePlaneOrItsAbsence checks that the Why line names the
// other FIB plugin's declared DataPlane when present and says "is another FIB
// plugin" when empty, both for config-root and selection-only matches.
//
// VALIDATES: noPlaneFIBChoiceFinding Why formatting for plane-declaring and
// plane-less FIB registrations.
// PREVENTS: blank plane names or hardcoded plane text in the Why line.
func TestVerdictWhyNamesThePlaneOrItsAbsence(t *testing.T) {
	isolateRegistry(t)
	registerTestSelectors(t, []string{"alpha", "beta"}, nil, nil)
	registerTestNoPlaneFIBSuite(t)

	schema := buildVerdictTestSchema("alpha", "alpha", "alpha")

	// Plane-declaring plugin via --plugin names its plane.
	noopOnly := mustParseWithSchema(t, schema, `fib { noop { } }`)
	alphaSel, err := MergeCliPlugins(nil, []string{"ze.fib-alpha"})
	if err != nil {
		t.Fatalf("MergeCliPlugins(alpha): %v", err)
	}
	findings := evaluateDataPlaneVerdict(noopOnly, schema, alphaSel)
	if len(findings) != 1 || findings[0].why != "fib noop is the choice to program no route, and fib alpha programs the alpha data plane" {
		t.Fatalf("alpha why = %+v", findings)
	}

	// Plane-less FIB plugin via --plugin says "is another FIB plugin".
	unmarkedSel, err := MergeCliPlugins(nil, []string{"ze.fib-unmarked"})
	if err != nil {
		t.Fatalf("MergeCliPlugins(unmarked): %v", err)
	}
	findings = evaluateDataPlaneVerdict(noopOnly, schema, unmarkedSel)
	if len(findings) != 1 || findings[0].why != "fib noop is the choice to program no route, and fib unmarked is another FIB plugin" {
		t.Fatalf("unmarked why = %+v", findings)
	}
}

// TestVerdictAcceptsNoPlaneChoiceAlone verifies that a NoDataPlaneChoice FIB
// plugin commits alone on any data plane and that unmarked FIB pairs remain
// governed solely by evaluateFIBWriters.
//
// VALIDATES: AC-4 permitted survivors for no-plane choice alone and unmarked
// FIB combinations.
// PREVENTS: refusing a standalone no-plane choice or breaking fib kernel + fib vpp.
func TestVerdictAcceptsNoPlaneChoiceAlone(t *testing.T) {
	isolateRegistry(t)
	registerTestSelectors(t, []string{"alpha", "beta"}, nil, nil)
	registerTestNoPlaneFIBSuite(t)

	schema := buildVerdictTestSchema("alpha", "alpha", "alpha")

	for _, cfg := range []string{
		`fib { noop { } }`,
		`interface { backend beta; } fib { noop { } }`,
		`interface { backend beta; } fib { alpha { } beta { } }`,
		`fib { alpha { } unmarked { } }`,
	} {
		tree := mustParseWithSchema(t, schema, cfg)
		if err := refuseDataPlaneMismatches(tree, schema, nil); err != nil {
			t.Errorf("config %q refused unexpectedly: %v", cfg, err)
		}
	}
}
