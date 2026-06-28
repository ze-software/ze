// Design: plan/learned/907-appliance-install-robust.md -- ze install disk registration

package disk

import (
	"codeberg.org/thomas-mangin/ze/cmd/ze/install"
	"codeberg.org/thomas-mangin/ze/internal/core/subdispatch"
)

func init() {
	install.Register("disk", Run, subdispatch.SubMeta{Desc: "Write disk image and inject database (initrd installer)"})
}
