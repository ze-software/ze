package cli

import (
	_ "github.com/ze-software/ze/internal/component/authz/yang"
	_ "github.com/ze-software/ze/internal/component/bgp/plugin"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/rib/yang"
	_ "github.com/ze-software/ze/internal/component/bgp/yang"
	_ "github.com/ze-software/ze/internal/component/config/system/yang"
	_ "github.com/ze-software/ze/internal/component/hub/yang"
	_ "github.com/ze-software/ze/internal/component/plugin/yang"
	_ "github.com/ze-software/ze/internal/component/ssh/yang"
	_ "github.com/ze-software/ze/internal/plugins/static"
)
