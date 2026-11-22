// Design: ai/rules/plugins.md -- ze_l2tp-off dynamic-CoS stub
// Related: handler.go -- the real dynamic RADIUS-CoS handler (ze_l2tp builds)

//go:build !ze_l2tp

package cos

import (
	"errors"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/ze"
)

// Dynamic per-subscriber CoS is a BNG feature: it reacts to L2TP session
// events and reads RADIUS session metadata, so without ze_l2tp there are no
// sessions and nothing to react to. These stubs keep the always-on static
// CoS profile surface (config, verifier, show class-of-service) compiling
// while the dynamic handler drops with the BNG. The per-session state and the
// subscriber enrichers that read it drop with it too, in session_state.go and
// enricher.go. Same dependent-feature shape as authradius
// (ze_l2tp && ze_radius).

// updateQoSFunc is the signature for updating VLAN QoS maps on an interface.
type updateQoSFunc func(ifaceName string, ingress, egress map[uint32]uint32) error

type cosHandler struct{}

func (h *cosHandler) stop() {}

func sessionCommandDecls() []sdk.CommandDecl { return nil }

func registerSessionCommand(map[string]func([]string, string) (string, any, error)) {}

// BindMetrics is a no-op: the dynamic-CoS counters (applied/reverted/coa)
// count session events, which cannot occur without the BNG.
func BindMetrics(metrics.Registry) {}

type stoppedCosHandler struct{}

func newCosHandlerFromStopped(ze.EventBus, updateQoSFunc, func(rpc.ReportIssueInput) error, *stoppedCosHandler) *cosHandler {
	return &cosHandler{}
}

func (h *cosHandler) stopOwners() *stoppedCosHandler { return &stoppedCosHandler{} }

// No BNG session can retain cleanup work in this build. The same visible
// command still reports its empty pass instead of leaving an unserved schema.
func registerCleanupCommand(handlers map[string]func([]string, string) (string, any, error)) {
	handlers[cleanupCommand] = func(args []string, _ string) (string, any, error) {
		if len(args) != 0 {
			err := opererr.Wrap(errors.New("class-of-service cleanup does not accept arguments"),
				"Class-of-service cleanup did not finish", "The cleanup command takes no arguments",
				"ze request class-of-service cleanup")
			response := plugin.OperatorErrorResponse(err)
			return response.Status, response, err
		}
		return rpc.StatusDone, sharedCoSCleanupResult{}, nil
	}
}
