// Design: docs/research/vpp-deployment-reference.md -- registered runtime prerequisites
// Related: startup_runtime.go -- the same live message table gates readiness

package vpp

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"go.fd.io/govpp/api"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

const startupProbeTimeout = 3 * time.Second

func registerStartupDoctor(name string, feature startupFeature) error {
	return diagnostic.RegisterDoctorCheck(diagnostic.DoctorCheck{
		Name: "vpp-startup-" + name, Phase: diagnostic.DoctorPhasePostConfig, Order: 755,
		Component: componentVPP, Platforms: []string{diagnostic.DoctorPlatformAny},
		Dependencies: []string{"vpp-api-socket"},
		Codes:        []string{feature.runtime.code},
		Check: func(input diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
			tree, ok := input.Tree.(*config.Tree)
			if !ok {
				return nil
			}
			if tree == nil {
				return nil
			}
			return checkStartupDoctor(tree, feature, probeStartupRuntime)
		},
	})
}

// checkStartupDoctor uses a delivered configuration only to select the feature
// and socket. A failed connection is unknown, never evidence of a missing
// plugin. An answered message-table mismatch is a concrete incompatibility.
func checkStartupDoctor(tree *config.Tree, feature startupFeature,
	probe func(context.Context, *VPPSettings) error) []diagnostic.Diagnostic {
	enabled, err := feature.enabled(tree)
	if err != nil {
		return []diagnostic.Diagnostic{startupRuntimeDiagnostic(feature, err, diagnostic.SeverityError)}
	}
	if !enabled {
		return nil
	}
	container := tree.GetContainer(componentVPP)
	if container == nil {
		return []diagnostic.Diagnostic{startupRuntimeDiagnostic(feature,
			errors.New("the selected forwarding feature has no VPP configuration"), diagnostic.SeverityError)}
	}
	data, err := json.Marshal(container.ToPluginMap())
	if err != nil {
		return []diagnostic.Diagnostic{startupRuntimeDiagnostic(feature, err, diagnostic.SeverityError)}
	}
	settings, err := ParseSettings(data)
	if err != nil {
		return []diagnostic.Diagnostic{startupRuntimeDiagnostic(feature, err, diagnostic.SeverityError)}
	}
	settings.startup = []startupFeature{feature}
	ctx, cancel := context.WithTimeout(context.Background(), startupProbeTimeout)
	defer cancel()
	if err := probe(ctx, settings); err != nil {
		severity := diagnostic.SeverityWarning
		if _, incompatible := errors.AsType[*api.CompatibilityError](err); incompatible {
			severity = diagnostic.SeverityError
		}
		return []diagnostic.Diagnostic{startupRuntimeDiagnostic(feature, err, severity)}
	}
	return nil
}

func probeStartupRuntime(ctx context.Context, settings *VPPSettings) error {
	connector := NewConnector(settings.APISocket)
	defer connector.Close()
	if err := connector.Connect(ctx, 1, time.Second); err != nil {
		return startupProbeError(err)
	}
	return settings.checkStartupRuntime(ctx, connector)
}

func startupRuntimeDiagnostic(feature startupFeature, err error, severity diagnostic.Severity) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{Code: feature.runtime.code, Severity: severity,
		Message: "VPP plugin " + feature.plugin + " readiness: " + err.Error(), Fix: feature.runtime.fix}
}
