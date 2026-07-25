// Design: plan/learned/1049-anomaly-2-shape.md -- show anomaly-shape responder status
//
// The responder runs in-process, so the show handler reads its live status
// (mode, armed sources) from the process-global responder.

package shape

import (
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:anomaly-shape",
			Handler:    handleShowAnomalyShape,
		},
	)
}

func handleShowAnomalyShape(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	r := loadGlobalResponder()
	if r == nil {
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   plugin.Map{"enabled": false, "armed": []string{}},
		}, nil
	}
	st := r.statusSnapshot()
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"enabled":     true,
			"mode":        st.Mode,
			"action":      st.Action,
			"kill-switch": st.Killed,
			"armed-count": len(st.ArmedList),
			"armed":       st.ArmedList,
		},
	}, nil
}
