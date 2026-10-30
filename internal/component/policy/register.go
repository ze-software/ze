// Design: docs/guide/plugins.md -- the policy plugin and its dependents
// Related: registry.go -- the filter instance namespace
// Related: fields.go -- the route field vocabulary

package policy

import (
	"errors"
	"fmt"
	"net"
	"os"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	policyyang "github.com/ze-software/ze/internal/component/policy/yang"
	"github.com/ze-software/ze/internal/core/slogutil"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// PluginName is the registration name every protocol plugin with a filter
// attachment, and every filter type plugin, lists in Dependencies.
const PluginName = "policy"

var logger = slogutil.LazyLogger("policy")

var errPolicyRootNotObject = errors.New("policy: the policy section is not a JSON object")

// The route field vocabulary: one registration per field, declared once here
// (spec "Route Field Vocabulary"). Filter types and attachment points name
// these values; a field nobody registered cannot be named.
var (
	FieldPrefix            = MustRegisterField("prefix")
	FieldFamily            = MustRegisterField("family")
	FieldProtocol          = MustRegisterField("protocol")
	FieldNextHop           = MustRegisterField("next-hop")
	FieldInterface         = MustRegisterField("interface")
	FieldMetric            = MustRegisterField("metric")
	FieldMetricType        = MustRegisterField("metric-type")
	FieldTag               = MustRegisterField("tag")
	FieldLevel             = MustRegisterField("level")
	FieldRouteType         = MustRegisterField("route-type")
	FieldDistance          = MustRegisterField("distance")
	FieldOriginAS          = MustRegisterField("origin-as")
	FieldCommunity         = MustRegisterField("community")
	FieldLargeCommunity    = MustRegisterField("large-community")
	FieldExtendedCommunity = MustRegisterField("extended-community")
	FieldASPath            = MustRegisterField("as-path")
	FieldLocalPreference   = MustRegisterField("local-preference")
	FieldOrigin            = MustRegisterField("origin")
	FieldAIGP              = MustRegisterField("aigp")
	FieldMED               = MustRegisterField("med")
)

// relocatedFromBGP is the container the policy root was a child of before it
// became top-level and shared by every protocol: `bgp { policy { ... } }` is
// refused naming the new root, and `ze config migrate` moves it.
const relocatedFromBGP = "bgp"

func init() {
	config.MustRegisterRelocatedContainer(relocatedFromBGP, RootName)

	if err := registry.Register(registry.Registration{
		Name:        PluginName,
		Description: "Route policy library: the policy root, named filter instances and the route field vocabulary",
		YANG:        policyyang.ZePolicyConfYANG,
		ConfigRoots: []string{RootName},
		RunEngine:   runPolicy,
		CLIHandler:  func(_ []string) int { return 0 },
	}); err != nil {
		fmt.Fprintf(os.Stderr, "policy: registration failed: %v\n", err)
		os.Exit(1)
	}
}

// runPolicy is the policy plugin's engine entry. It answers the config
// transaction for the policy root, so a commit touching it is never refused
// for want of a participant; the instance namespace and the field checks run
// where each attachment's config is loaded (BuildRegistry, CheckChain).
func runPolicy(conn net.Conn) int {
	p := sdk.NewWithConn(PluginName, conn)
	defer p.Close() //nolint:errcheck // best-effort cleanup

	parse := SectionParser(PluginName, errPolicyRootNotObject, func(map[string]any) (bool, error) { return true, nil })
	sdk.NewConfigTx(false, parse, installPolicySection).Register(p)

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, &sdk.Registration{WantsConfig: []string{RootName}}); err != nil {
		logger().Error("policy plugin failed", "error", err)
		return 1
	}
	return 0
}

func installPolicySection(configured bool) error {
	logger().Debug("configured", "policy-root", configured)
	return nil
}
