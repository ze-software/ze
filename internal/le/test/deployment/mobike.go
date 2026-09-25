// Design: docs/architecture/testing/qemu-integration.md -- native MOBIKE peer proof
// Related: ../interoplab/ipsec/mobike_netns_linux.go -- the two live movement scenarios
package testdeployment

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	interopipsec "github.com/ze-software/ze/internal/le/interoplab/ipsec"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

func runIPsecMOBIKEHere(args leaction.Arguments) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	charon := args.One("charon")
	if charon == "" {
		charon = "/usr/lib/strongswan/charon"
	}
	swanctl := args.One("swanctl")
	if swanctl == "" {
		swanctl = "/usr/sbin/swanctl"
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return interopipsec.RunMOBIKENetns(ctx, root, args.One("daemon"), charon, swanctl)
}
