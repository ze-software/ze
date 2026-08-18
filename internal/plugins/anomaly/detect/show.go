// Design: docs/architecture/anomaly/anomaly-1-detect.md -- show anomaly report surface
//
// The detector runs in-process (plugins are goroutines), so the show handler reads
// the recent-incident ring directly from the process-global detector.

package detect

import (
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/anomalyevent"
	"github.com/ze-software/ze/internal/core/textbuf"
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
		e := &incs[i]
		feats := make([]plugin.Map, 0, len(e.FiredFeatures))
		for _, f := range e.FiredFeatures {
			feats = append(feats, plugin.Map{"name": f.Name, "z": f.Z})
		}
		row := plugin.Map{
			"entity":         entityLabel(e),
			"entity-kind":    e.EntityKind.String(),
			"cohort":         e.Cohort,
			"score":          e.Score,
			"severity":       string(e.Severity),
			"at":             e.At.Format(time.RFC3339),
			"fired-features": feats,
		}
		// A port incident names its subject with two numbers rather than an address,
		// so a reader that wants them apart does not have to split the label.
		if e.EntityKind == anomalyevent.EntityKindPort {
			row["port"] = e.Port
			row["proto"] = e.Proto
		}
		list = append(list, row)
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{"enabled": true, "incidents": list},
	}, nil
}

// entityLabel names an incident's subject for display. An address incident renders
// its prefix, as it always has. A port incident has no address, so it renders
// proto/port -- built with textbuf because this is the cold management-plane path and
// nothing here may allocate its way through fmt.
func entityLabel(e *anomalyevent.AnomalyDetected) string {
	if e.EntityKind != anomalyevent.EntityKindPort {
		return e.Entity.String()
	}
	var tb textbuf.Buffer
	tb.Uint8(e.Proto).Str("/").Uint16(e.Port)
	return tb.String()
}
