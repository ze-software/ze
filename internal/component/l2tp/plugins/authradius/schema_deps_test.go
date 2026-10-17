// Phase 5 (command-surface-ownership): owner schema packages now carry their
// `show <owner>` / `clear <owner>` command-tree YANG, which augments the
// central show/clear modules. Test binaries that load an owner config schema
// must also register those base modules so the YANG resolver can graft the
// augments. The full `ze` binary gets these via plugin/all; isolated test
// binaries do not, so register them explicitly here.
package l2tpauthradius

import (
	_ "github.com/ze-software/ze/internal/component/cmd/clear/yang"
	_ "github.com/ze-software/ze/internal/component/cmd/show/yang"
)
