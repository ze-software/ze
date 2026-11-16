// Design: docs/labs/pppoe-interop.md -- source-first native IPv6 acceptance.
// Related: product_plan.go -- one shared Suite/staging owner for both families.
package testvpppppoe

import (
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"syscall"

	"github.com/ze-software/ze/internal/core/opererr"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

func answerProductIPv6(args leaction.Arguments) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		return productIPv6Answer(nil, err)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	report := runProductSuite(ctx, root, args.One(imageKeyword), true)
	if report.Code != 0 {
		leaction.ReportError(opererr.New("the IPv6 product acceptance is incomplete", "The named scenario must establish RA, both DHCPv6 associations, native paths and isolated family closure.", "./le --name session test qemu docker-lab lab \"test deployment vpp-pppoe product-ipv6\" timeout 3600s"))
	}
	return report, report.Code
}

func answerProductFamily(args leaction.Arguments) (any, int) {
	result, err := productFamilyProbe(args.One("interface"), args.One("peer"), args.One("sid"), args.One("operation"), args.One("source"), args.One("target"))
	return productIPv6Answer(result, err)
}

// Prose rendering preserves exactly one JSON value on stdout for Docker's
// receiver. Exit status alone never constitutes an observation.
func productIPv6Answer(value any, err error) (any, int) {
	if err != nil {
		leaction.ReportError(opererr.Wrap(err, "the independent IPv6 wire observation failed", "No packet-path result can be taken from an incomplete peer operation.", "./le test deployment vpp-pppoe product-ipv6"))
		return nil, 1
	}
	wire, err := json.Marshal(value)
	if err != nil {
		leaction.ReportError(opererr.Wrap(err, "the IPv6 wire observation cannot be encoded", "The parent checker requires the exact received bytes.", "./le test deployment vpp-pppoe product-ipv6"))
		return nil, 1
	}
	return productObservation{Data: wire}, 0
}
