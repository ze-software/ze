// Design: docs/architecture/ospf/ospf-13-cli-diag-interop.md -- `show ospf` process summary.
// The summary reflects configured state (router-id, areas, ABR/ASBR role, stub-router);
// origination of the max-metric Router-LSA itself is owned by ospf-7.

package ospf

import (
	"github.com/ze-software/ze/internal/plugins/ospf/spf"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// processSummaryView is the `show ospf` payload.
type processSummaryView struct {
	RouterID   string            `json:"router-id"`
	ABR        bool              `json:"abr"`
	ASBR       bool              `json:"asbr"`
	AreaCount  int               `json:"area-count"`
	Areas      []areaSummaryView `json:"areas"`
	StubRouter stubRouterView    `json:"stub-router"`
}

type areaSummaryView struct {
	AreaID         string `json:"area-id"`
	Type           string `json:"type"`
	InterfaceCount int    `json:"interface-count"`
}

// stubRouterView reflects the RFC 6987 max-metric router-lsa configuration. Active is
// true when the router is unconditionally a stub router (`always`); the time-bounded
// on-startup/on-shutdown windows are surfaced as their configured durations.
type stubRouterView struct {
	Always     bool   `json:"always"`
	OnStartup  uint32 `json:"on-startup-seconds,omitempty"`
	OnShutdown uint32 `json:"on-shutdown-seconds,omitempty"`
	Active     bool   `json:"active"`
}

// processSummary renders the `show ospf` process summary from the resolved config.
func (e *engine) processSummary() processSummaryView {
	e.mu.Lock()
	defer e.mu.Unlock()
	cfg := e.cfg

	attached := make(map[types.AreaID]int, len(cfg.Areas))
	for _, ic := range cfg.Interfaces {
		attached[ic.AreaID]++
	}
	attachedAreas := make([]types.AreaID, 0, len(attached))
	for id := range attached {
		attachedAreas = append(attachedAreas, id)
	}
	areas := make([]areaSummaryView, 0, len(cfg.Areas))
	for _, a := range cfg.Areas {
		areas = append(areas, areaSummaryView{
			AreaID:         a.AreaID.String(),
			Type:           string(a.AreaType),
			InterfaceCount: attached[a.AreaID],
		})
	}

	return processSummaryView{
		RouterID:  cfg.RouterID.String(),
		ABR:       spf.IsABR(attachedAreas),
		ASBR:      len(cfg.Redistribute) > 0 || cfg.DefaultInformation.Originate,
		AreaCount: len(cfg.Areas),
		Areas:     areas,
		StubRouter: stubRouterView{
			Always:     cfg.MaxMetric.RouterLSAAlways,
			OnStartup:  cfg.MaxMetric.OnStartupSec,
			OnShutdown: cfg.MaxMetric.OnShutdownSec,
			Active:     cfg.MaxMetric.RouterLSAAlways,
		},
	}
}
