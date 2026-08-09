// Design: docs/architecture/ospf/ospf-4-component-config.md -- per-area state scaffolding
// Related: config.go -- areaConfig and interfaceConfig inputs
package ospf

import "github.com/ze-software/ze/internal/plugins/ospf/types"

type area struct {
	id         types.AreaID
	areaType   areaType
	interfaces map[string]interfaceConfig
}

func newAreas(cfg ospfConfig) map[types.AreaID]*area {
	areas := make(map[types.AreaID]*area, len(cfg.Areas))
	for _, ac := range cfg.Areas {
		areas[ac.AreaID] = &area{id: ac.AreaID, areaType: ac.AreaType, interfaces: make(map[string]interfaceConfig)}
	}
	for _, ic := range cfg.enrolledInterfaces() {
		a := areas[ic.AreaID]
		if a == nil {
			continue
		}
		a.interfaces[ic.Name] = ic
	}
	return areas
}
