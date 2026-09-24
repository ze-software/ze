// Design: docs/architecture/wire/nlri-bgpls.md -- native EPE plugin registration

package epe

import (
	"fmt"
	"log/slog"

	epeyang "github.com/ze-software/ze/internal/component/bgp/plugins/epe/yang"
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

var epeLogger = slog.Default()

func init() {
	reg := registry.Registration{Name: Name, Description: "Native BGP Egress Peer Engineering segments",
		RFCs: []string{"9086"}, Features: "yang", YANG: epeyang.ZeBGPEpeConfYANG, ConfigRoots: []string{Name},
		NeedsDataPlane: true, RunEngine: runEPEProducer,
		ConfigureEngineLogger:   func(name string) { epeLogger = slogutil.Logger(name) },
		InProcessConfigVerifier: func(sections []rpc.ConfigSection) error { _, err := ParseConfig(sections); return err }}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.GetYANG = func() string { return epeyang.ZeBGPEpeConfYANG }
		cfg.ConfigLogger = func(level string) { epeLogger = slogutil.PluginLogger(reg.Name, level) }
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		panic(fmt.Sprintf("BUG: register BGP EPE producer: %v", err))
	}
}
