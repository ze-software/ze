package schema

import _ "embed"

// ZeRouteRefreshYANG is the embedded YANG schema for route-refresh.
//
//go:embed ze-route-refresh.yang
var ZeRouteRefreshYANG string

// ZeRouteRefreshAPIYANG is the embedded YANG schema for route-refresh API RPCs.
//
//go:embed ze-route-refresh-api.yang
var ZeRouteRefreshAPIYANG string
