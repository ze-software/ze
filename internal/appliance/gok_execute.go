// Design: docs/architecture/appliance/builder.md -- full image build (assemble + gok + ext4)
// Related: gok_execute_other.go -- the refusal on a host gokrazy cannot pack on

//go:build linux || darwin

package appliance

import (
	"context"
	"os"

	"github.com/gokrazy/tools/gok"
)

// ExecuteGok runs gok's command line over args, writing to this process's
// stdout and stderr. Both `ze appliance build` and `le build gokrazy` reach gok
// only through here, so the host restriction lives in one place.
func ExecuteGok(ctx context.Context, args []string) error {
	return gok.Context{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Args:   args,
	}.Execute(ctx)
}
