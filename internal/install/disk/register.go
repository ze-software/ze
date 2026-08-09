// Design: docs/architecture/appliance/on-device-installer.md -- ze install disk registration

package disk

import (
	"github.com/ze-software/ze/cmd/ze/install"
	"github.com/ze-software/ze/internal/core/subdispatch"
)

func init() {
	install.Register("disk", Run, subdispatch.SubMeta{Desc: "Write disk image and inject database (initrd installer)"})
}
