// Design: ai/rules/plugins.md -- ze_l2tp-off dynamic-CoS stub
// Related: handler.go -- the real dynamic RADIUS-CoS handler (ze_l2tp builds)

//go:build !ze_l2tp

package cos

import (
	"github.com/ze-software/ze/internal/core/metrics"
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

// resolveStaticFunc returns the static CoS maps for an interface from config.
type resolveStaticFunc func(ifaceName string) (ingress, egress map[uint32]uint32)

type cosHandler struct{}

// newCosHandler is the ze_l2tp-off no-op constructor: no event subscriptions,
// no session state. register.go still stores and stops the handle, so both
// paths stay identical in shape.
func newCosHandler(ze.EventBus, updateQoSFunc, resolveStaticFunc) *cosHandler { return &cosHandler{} }

func (h *cosHandler) stop() {}

// BindMetrics is a no-op: the dynamic-CoS counters (applied/reverted/coa)
// count session events, which cannot occur without the BNG.
func BindMetrics(metrics.Registry) {}
