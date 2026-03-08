package schema

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

func init() {
	yang.RegisterModule("ze-route-refresh.yang", ZeRouteRefreshYANG)
	yang.RegisterModule("ze-route-refresh-api.yang", ZeRouteRefreshAPIYANG)
}
