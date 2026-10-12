// Design: docs/architecture/appliance/builder.md -- full image build (assemble + gok + ext4)
// Related: gok_execute.go -- the gok run on Linux and macOS

//go:build !linux && !darwin

package appliance

import (
	"context"
	"errors"
	"runtime"
)

// ExecuteGok refuses: gokrazy's packer reads the size of a block device only on
// Linux and macOS, so on this host it cannot write an image at all.
func ExecuteGok(context.Context, []string) error {
	return errors.New("building a gokrazy image is not supported on " + runtime.GOOS +
		": gokrazy's packer reads block device sizes only on Linux and macOS; build the appliance image on a Linux or macOS host")
}
