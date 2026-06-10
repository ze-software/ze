package cli

import (
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugin"
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/yang"
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/yang"
	_ "codeberg.org/thomas-mangin/ze/internal/component/config/system/yang"
	_ "codeberg.org/thomas-mangin/ze/internal/component/hub/yang"
	_ "codeberg.org/thomas-mangin/ze/internal/component/plugin/yang"
	_ "codeberg.org/thomas-mangin/ze/internal/component/ssh/yang"
	_ "codeberg.org/thomas-mangin/ze/internal/plugins/static"
)
