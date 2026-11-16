// Design: docs/labs/pppoe-interop.md -- real-product lab command entry point.
// Related: register.go -- these actions share the established native lab area.
package testvpppppoe

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/ze-software/ze/internal/core/opererr"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

func answerProduct(args leaction.Arguments) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(opererr.Wrap(err, "the product PPPoE lab cannot find the checkout", "Its fresh daemon and independent peers cannot be prepared.", "./le test deployment vpp-pppoe product"))
		return nil, 1
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	report := RunProduct(ctx, root, args.One(imageKeyword))
	if report.Code != 0 {
		leaction.ReportError(opererr.New("the configured Ze PPPoE lab did not establish its IPv4 acceptance result", "Read the named scenario's evidence below; a setup, peer, state or cleanup failure cannot establish the product packet path.", "./le --name session test qemu docker-lab lab \"test deployment vpp-pppoe product\" timeout 3600s"))
	}
	return report, report.Code
}
