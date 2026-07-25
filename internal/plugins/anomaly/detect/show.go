// Design: plan/learned/1048-anomaly-1-detect.md -- show anomaly report surface
//
// The detector runs in-process (plugins are goroutines), so the show handler reads
// the recent-incident ring directly from the process-global detector.

package detect

import (
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:anomaly",
			Handler:    handleShowAnomaly,
		},
	)
}

func handleShowAnomaly(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	d := loadGlobalDetector()
	if d == nil {
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   plugin.Map{"enabled": false, "incidents": []plugin.Map{}},
		}, nil
	}

	incs := d.recentIncidents()
	list := make([]plugin.Map, 0, len(incs))
	for i := range incs {
		e := incs[i]
		feats := make([]plugin.Map, 0, len(e.FiredFeatures))
		for _, f := range e.FiredFeatures {
			feats = append(feats, plugin.Map{"name": f.Name, "z": f.Z})
		}
		list = append(list, plugin.Map{
			"entity":         e.Entity.String(),
			"cohort":         e.Cohort,
			"score":          e.Score,
			"severity":       string(e.Severity),
			"at":             e.At.Format(time.RFC3339),
			"fired-features": feats,
		})
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{"enabled": true, "incidents": list},
	}, nil
}
