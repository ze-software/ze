// Design: docs/guide/plugins.md -- the policy plugin and its dependents
// Related: registry.go -- the filter instance namespace
// Related: fields.go -- the route field vocabulary

package policy

import (
	"fmt"
	"net"
	"os"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	policyyang "github.com/ze-software/ze/internal/component/policy/yang"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
	"github.com/ze-software/ze/internal/core/slogutil"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// PluginName is the registration name every protocol plugin with a filter
// attachment, and every filter type plugin, lists in Dependencies.
const PluginName = "policy"

var logger = slogutil.LazyLogger("policy")

// The route field vocabulary: one registration per field, declared once here
// (spec "Route Field Vocabulary"). Filter types and attachment points name
// these values; a field nobody registered cannot be named.
var (
	FieldPrefix    = MustRegisterField("prefix", RouteView{Read: func(r *ribpolicy.Route) any { return r.Prefix }})
	FieldFamily    = MustRegisterField("family", RouteView{Read: func(r *ribpolicy.Route) any { return r.Family }})
	FieldProtocol  = MustRegisterField("protocol", RouteView{Read: func(r *ribpolicy.Route) any { return routeProtocol{r.Source, r.IsBGP, r.IsEBGP} }})
	FieldNextHop   = MustRegisterField("next-hop", RouteView{Read: func(r *ribpolicy.Route) any { return r.NextHop }})
	FieldInterface = MustRegisterField("interface", RouteView{Read: func(r *ribpolicy.Route) any { return r.Interface }})
	FieldMetric    = MustRegisterField("metric", RouteView{
		Read:    func(r *ribpolicy.Route) any { return r.Metric },
		Written: func(s *ribpolicy.Sets) bool { return s.HasMetric },
	})
	FieldMetricType = MustRegisterField("metric-type", RouteView{Written: func(s *ribpolicy.Sets) bool { return s.MetricType != ribpolicy.ExternalMetricTypeUnspecified }})
	FieldTag        = MustRegisterField("tag", RouteView{
		Read:    func(r *ribpolicy.Route) any { return routeTag{r.Tag, r.HasTag} },
		Written: func(s *ribpolicy.Sets) bool { return s.HasTag },
	})
	FieldLevel     = MustRegisterField("level", RouteView{Read: func(r *ribpolicy.Route) any { return r.Level }})
	FieldRouteType = MustRegisterField("route-type", RouteView{Read: func(r *ribpolicy.Route) any { return r.OSPFRouteType }})
	FieldDistance  = MustRegisterField("distance", RouteView{Written: func(s *ribpolicy.Sets) bool { return s.HasDistance }})
	// The BGP path fields: a ribpolicy.Route holds none of them, so only a BGP
	// attachment carries them.
	FieldOriginAS          = MustRegisterField("origin-as", RouteView{})
	FieldCommunity         = MustRegisterField("community", RouteView{})
	FieldLargeCommunity    = MustRegisterField("large-community", RouteView{})
	FieldExtendedCommunity = MustRegisterField("extended-community", RouteView{})
	FieldASPath            = MustRegisterField("as-path", RouteView{})
	FieldLocalPreference   = MustRegisterField("local-preference", RouteView{})
	FieldOrigin            = MustRegisterField("origin", RouteView{})
	FieldAIGP              = MustRegisterField("aigp", RouteView{})
	FieldMED               = MustRegisterField("med", RouteView{})
)

// routeProtocol is the protocol field's value on the route view: the source
// protocol and the BGP class a `protocol [ ebgp ]` condition reads.
type routeProtocol struct {
	source      redistevents.ProtocolID
	isBGP, eBGP bool
}

// routeTag is the tag field's value on the route view: 0 is a real tag, so
// whether the route carries one is part of the value.
type routeTag struct {
	tag    uint32
	hasTag bool
}

// relocatedFromBGP is the container the policy root was a child of before it
// became top-level and shared by every protocol: `bgp { policy { ... } }` is
// refused naming the new root, and `ze config migrate` moves it.
const relocatedFromBGP = "bgp"

func init() {
	config.MustRegisterRelocatedContainer(relocatedFromBGP, RootName)

	// The generic match filter: always built, so a Loc-RIB chain has a
	// filter type in a build without BGP. Its type declares every field a
	// condition can read; each instance is checked against only the
	// conditions it names (match.go, matchConditions).
	MustRegisterFilterType(MatchFilterType,
		[]Field{FieldPrefix, FieldFamily, FieldProtocol, FieldNextHop, FieldInterface, FieldMetric, FieldTag, FieldLevel, FieldRouteType}, nil)
	MustRegisterInstanceFields(MatchFilterType, matchInstanceFields)
	MustRegisterRouteEvaluator(MatchFilterType, compileMatchEvaluator)

	if err := diagnostic.RegisterDoctorCheck(igpImportDoctorCheck); err != nil {
		panic("BUG: policy: doctor check registration refused: " + err.Error())
	}

	if err := registry.Register(registry.Registration{
		Name:        PluginName,
		Description: "Route policy library: the policy root, named filter instances and the route field vocabulary",
		YANG:        policyyang.ZePolicyConfYANG,
		ConfigRoots: []string{RootName},
		RunEngine:   runPolicy,
		CLIHandler:  func(_ []string) int { return 0 },
		// The policy plugin publishes the Loc-RIB import chains, so it owns
		// the metrics of what they hide (locrib/policy_metrics.go).
		ConfigureMetrics: locrib.SetPolicyMetricsRegistry,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "policy: registration failed: %v\n", err)
		os.Exit(1)
	}
}

// runPolicy is the policy plugin's engine entry. It answers the config
// transaction for the policy root and the roots it reads: it compiles every
// Loc-RIB import chain and publishes the snapshot the gate reads
// (rib_compile.go). The BGP chains are checked where bgp loads its config
// (BuildRegistry, CheckChain).
func runPolicy(conn net.Conn) int {
	p := sdk.NewWithConn(PluginName, conn)
	defer p.Close() //nolint:errcheck // best-effort cleanup

	// The transaction compiles the Loc-RIB import chains of every root that
	// declared one (MustRegisterRIBImport) and publishes them to the gate.
	sdk.NewConfigTx(ribImportConfig{}, parseRIBImports, installRIBImports).Register(p)

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, &sdk.Registration{WantsConfig: []string{RootName}}); err != nil {
		logger().Error("policy plugin failed", "error", err)
		return 1
	}
	return 0
}

// addPolicyConfigRead makes the policy plugin read root, a root holding a
// Loc-RIB import attachment. The policy plugin registered in this package's
// init(), which runs before the init() of any package importing policy, so
// the registration exists. A failure is a Ze defect.
func addPolicyConfigRead(root string) {
	if err := registry.AddConfigRead(PluginName, root); err != nil {
		panic("BUG: policy: " + err.Error())
	}
}
