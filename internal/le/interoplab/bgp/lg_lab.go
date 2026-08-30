package bgp

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

type lgInjection struct {
	peer, prefix, nextHop, asPath string
}

func runLGLab(name string) error {
	plugin, err := sdk.NewFromEnv(name)
	if err != nil {
		return err
	}
	ctx := context.Background()
	plugin.OnAllPluginsReady(func() error {
		go func() {
			time.Sleep(time.Second)
			for _, injection := range lgInjections {
				command := fmt.Sprintf("request bgp rib inject %s ipv4/unicast %s origin igp nhop %s aspath %s", injection.peer, injection.prefix, injection.nextHop, injection.asPath)
				status, _, dispatchErr := plugin.DispatchCommand(ctx, command)
				if dispatchErr != nil || status != rpc.StatusDone {
					slog.Error("lg graph lab injection failed", "command", command, fieldStatus, status, fieldError, dispatchErr)
					return
				}
			}
			slog.Info("lg graph lab routes injected", "routes", len(lgInjections))
		}()
		return nil
	})
	return plugin.Run(ctx, sdk.Registration{})
}

var lgInjections = []lgInjection{
	{lgRouter1A, lgPrefixFirst, lgRouter1A, lgASPathFirstVia1A},
	{lgRouter3A, lgPrefixFirst, lgRouter3A, lgASPathFirstVia3A},
	{lgRouter1B, lgPrefixFirst, lgRouter1A, lgASPathFirstVia1A},
	{lgRouter4A, lgPrefixFirst, lgRouter1A, lgASPathFirstVia1A},
	{lgRouter4B, lgPrefixFirst, lgRouter1A, lgASPathFirstVia1A},
	{lgRouter5A, lgPrefixFirst, lgRouter1A, lgASPathFirstVia1A},
	{lgRouter5B, lgPrefixFirst, lgRouter1A, lgASPathFirstVia1A},
	{lgRouter2A, lgPrefixFirst, lgRouter3A, lgASPathFirstVia3A},
	{lgRouter2B, lgPrefixFirst, lgRouter3A, lgASPathFirstVia3A},
	{lgRouter3B, lgPrefixFirst, lgRouter3A, lgASPathFirstVia3A},
	{lgRouter6A, lgPrefixFirst, lgRouter3A, lgASPathFirstVia3A},
	{lgRouter6B, lgPrefixFirst, lgRouter3A, lgASPathFirstVia3A},
	{lgRouter1B, lgPrefixSecond, lgRouter1B, lgASPathSecondVia1B},
	{lgRouter1A, lgPrefixSecond, lgRouter1B, lgASPathSecondVia1B},
	{lgRouter2A, lgPrefixSecond, lgRouter1B, lgASPathSecondVia1B},
	{lgRouter2B, lgPrefixSecond, lgRouter1B, lgASPathSecondVia1B},
	{lgRouter3A, lgPrefixSecond, lgRouter1B, lgASPathSecondVia1B},
	{lgRouter3B, lgPrefixSecond, lgRouter1B, lgASPathSecondVia1B},
	{lgRouter4A, lgPrefixSecond, lgRouter1B, lgASPathSecondVia1B},
	{lgRouter4B, lgPrefixSecond, lgRouter1B, lgASPathSecondVia1B},
	{lgRouter5A, lgPrefixSecond, lgRouter1B, lgASPathSecondVia1B},
	{lgRouter5B, lgPrefixSecond, lgRouter1B, lgASPathSecondVia1B},
	{lgRouter6A, lgPrefixSecond, lgRouter1B, lgASPathSecondVia1B},
	{lgRouter6B, lgPrefixSecond, lgRouter1B, lgASPathSecondVia1B},
	{lgRouter1B, lgPrefixThird, lgRouter1B, lgASPathThirdVia1B},
	{lgRouter3B, lgPrefixThird, lgRouter3B, lgASPathThirdVia3B},
	{lgRouter1A, lgPrefixThird, lgRouter1B, lgASPathThirdVia1B},
	{lgRouter4A, lgPrefixThird, lgRouter1B, lgASPathThirdVia1B},
	{lgRouter4B, lgPrefixThird, lgRouter1B, lgASPathThirdVia1B},
	{lgRouter5A, lgPrefixThird, lgRouter1B, lgASPathThirdVia1B},
	{lgRouter5B, lgPrefixThird, lgRouter1B, lgASPathThirdVia1B},
	{lgRouter2A, lgPrefixThird, lgRouter3B, lgASPathThirdVia3B},
	{lgRouter2B, lgPrefixThird, lgRouter3B, lgASPathThirdVia3B},
	{lgRouter3A, lgPrefixThird, lgRouter3B, lgASPathThirdVia3B},
	{lgRouter6A, lgPrefixThird, lgRouter3B, lgASPathThirdVia3B},
	{lgRouter6B, lgPrefixThird, lgRouter3B, lgASPathThirdVia3B},
}
