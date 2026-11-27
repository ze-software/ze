// Design: docs/architecture/chaos-web-dashboard.md -- chaos and route dynamics schedulers

package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/ze-software/ze/internal/chaos/engine"
	"github.com/ze-software/ze/internal/chaos/guard"
	"github.com/ze-software/ze/internal/chaos/inprocess"
	chaosmcp "github.com/ze-software/ze/internal/chaos/mcp"
	"github.com/ze-software/ze/internal/chaos/peer"
	"github.com/ze-software/ze/internal/chaos/route"
	"github.com/ze-software/ze/internal/chaos/web"
)

const (
	reconnectBackoff = 2 * time.Second
	// desiredSettleWindow reserves time at the end of a bounded --duration run
	// where automatic fault/churn injection pauses so disconnected peers finish
	// reconnectBackoff, re-synchronize EOR, and let withdrawals/announcements
	// converge before the observation cutoff.
	desiredSettleWindow = 4 * time.Second

	// controlDispatchTimeout bounds synchronous control replies over MCP.
	controlDispatchTimeout = 10 * time.Second

	ctrlTypePause   = "pause"
	ctrlTypeResume  = "resume"
	ctrlTypeRate    = "rate"
	ctrlTypeTrigger = "trigger"
	ctrlTypeStop    = "stop"
)

// schedulerStopTime computes when automatic chaos/route ticks pause before a
// bounded run's deadline. Returns zero for unbounded runs (no deadline) or when
// warmup already covers the run.
func schedulerStopTime(deadline time.Time, hasDeadline bool, start time.Time, warmup time.Duration) time.Time {
	if !hasDeadline {
		return time.Time{}
	}
	active := deadline.Sub(start) - warmup
	if active <= 0 {
		return time.Time{}
	}
	settle := min(desiredSettleWindow, active/2)
	return deadline.Add(-settle)
}

// runPeerLoop runs a peer simulator with reconnection after chaos disconnects.
func runPeerLoop(ctx context.Context, cfg peer.SimulatorConfig, peerIndex int, events chan<- peer.Event) {
	for {
		peer.RunSimulator(ctx, cfg)

		if ctx.Err() != nil {
			return
		}

		select {
		case events <- peer.Event{Type: peer.EventReconnecting, PeerIndex: peerIndex, Time: time.Now()}:
		case <-ctx.Done():
			return
		}

		select {
		case <-time.After(reconnectBackoff):
		case <-ctx.Done():
			return
		}
	}
}

// newControlDispatcher returns a synchronous chaosmcp.ControlDispatcher backed by
// controlCh and schedDone. When wd is non-nil, successful commands also update
// dashboard control state so web and MCP views agree.
func newControlDispatcher(
	ctx context.Context,
	controlCh chan<- web.ControlCommand,
	schedDone <-chan struct{},
	wd *web.Dashboard,
) chaosmcp.ControlDispatcher {
	return func(cmd web.ControlCommand) (web.ControlResult, error) {
		select {
		case <-schedDone:
			return web.ControlResult{}, errors.New("chaos scheduler is stopped")
		case <-ctx.Done():
			return web.ControlResult{}, errors.New("chaos run has ended")
		default:
		}

		reply := make(chan web.ControlResult, 1)
		cmd.Reply = reply

		select {
		case controlCh <- cmd:
		case <-schedDone:
			return web.ControlResult{}, errors.New("chaos scheduler is stopped")
		case <-ctx.Done():
			return web.ControlResult{}, errors.New("chaos run has ended")
		default:
			return web.ControlResult{}, errors.New("chaos control channel is busy")
		}

		timer := time.NewTimer(controlDispatchTimeout)
		defer timer.Stop()

		select {
		case res := <-reply:
			if res.Err == nil && wd != nil {
				wd.ApplyControl(cmd)
			}
			return res, nil
		case <-schedDone:
			select {
			case res := <-reply:
				if res.Err == nil && wd != nil {
					wd.ApplyControl(cmd)
				}
				return res, nil
			default:
				return web.ControlResult{}, errors.New("chaos scheduler stopped before answering")
			}
		case <-ctx.Done():
			return web.ControlResult{}, errors.New("chaos run ended before control completed")
		case <-timer.C:
			return web.ControlResult{}, errors.New("timed out waiting for chaos scheduler response")
		}
	}
}

// runScheduler runs the chaos scheduler goroutine.
func runScheduler(
	ctx context.Context,
	cfg ChaosConfig,
	seed uint64,
	peerCount int,
	es *establishedState,
	g *guard.Guard,
	channels []chan engine.ChaosAction,
	controlCh <-chan web.ControlCommand,
	stopTicksAt time.Time,
	quiet bool,
	onDone func(),
) {
	defer func() {
		if onDone != nil {
			onDone()
		}
	}()

	sched := engine.NewScheduler(engine.SchedulerConfig{
		Seed:           seed,
		PeerCount:      peerCount,
		Rate:           cfg.Rate,
		Interval:       cfg.Interval,
		Warmup:         cfg.Warmup,
		EnabledActions: cfg.EnabledActions,
		ActionParams:   cfg.ActionParams,
	})

	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()

	paused := false

	dispatchAction := func(a engine.ScheduledAction) {
		if ok, reason := g.AllowChaos(a.PeerIndex, a.Action.Type); !ok {
			if !quiet {
				fmt.Fprintf(os.Stderr, "le chaos run | scheduler | blocked %s for peer %d (%s)\n",
					a.Action.Type, a.PeerIndex, reason)
			}
			return
		}
		if !quiet {
			fmt.Fprintf(os.Stderr, "le chaos run | scheduler | %s -> peer %d\n",
				a.Action.Type, a.PeerIndex)
		}
		select {
		case channels[a.PeerIndex] <- a.Action:
			if a.Action.Type == engine.ActionHoldTimerExpiry {
				g.OnHoldTimerExpiry(a.PeerIndex)
			}
		default:
			if !quiet {
				fmt.Fprintf(os.Stderr, "le chaos run | scheduler | dropped %s for peer %d (busy)\n",
					a.Action.Type, a.PeerIndex)
			}
		}
	}

	handleControl := func(cmd web.ControlCommand) bool {
		switch cmd.Type {
		case ctrlTypePause:
			paused = true
			if !quiet {
				fmt.Fprintf(os.Stderr, "le chaos run | scheduler | paused\n")
			}
			inprocess.ReplyControl(cmd.Reply, web.ControlResult{Action: ctrlTypePause})
		case ctrlTypeResume:
			paused = false
			if !quiet {
				fmt.Fprintf(os.Stderr, "le chaos run | scheduler | resumed\n")
			}
			inprocess.ReplyControl(cmd.Reply, web.ControlResult{Action: ctrlTypeResume})
		case ctrlTypeRate:
			sched.SetRate(cmd.Rate)
			paused = cmd.Rate == 0
			if !quiet {
				fmt.Fprintf(os.Stderr, "le chaos run | scheduler | rate -> %.2f\n", cmd.Rate)
			}
			rate := cmd.Rate
			inprocess.ReplyControl(cmd.Reply, web.ControlResult{Action: ctrlTypeRate, Rate: &rate})
		case ctrlTypeTrigger:
			res := inprocess.DispatchManualTrigger(cmd.Trigger, peerCount, es.Snapshot(), g, channels, cfg.ActionParams, nil, quiet)
			inprocess.ReplyControl(cmd.Reply, res)
		case ctrlTypeStop:
			if !quiet {
				fmt.Fprintf(os.Stderr, "le chaos run | scheduler | stopped by dashboard\n")
			}
			inprocess.ReplyControl(cmd.Reply, web.ControlResult{Action: ctrlTypeStop})
			return true
		default:
			inprocess.ReplyControl(cmd.Reply, web.ControlResult{
				Action: cmd.Type,
				Err:    fmt.Errorf("unknown control command %q", cmd.Type),
			})
		}
		return false
	}

	for {
		select {
		case <-ctx.Done():
			return
		case cmd, ok := <-controlCh:
			if !ok {
				return
			}
			if handleControl(cmd) {
				return
			}
		case now := <-ticker.C:
			if paused {
				continue
			}
			if !stopTicksAt.IsZero() && !now.Before(stopTicksAt) {
				continue
			}
			actions := sched.Tick(now, es.Snapshot())
			for _, a := range actions {
				dispatchAction(a)
			}
		}
	}
}

// runRouteScheduler runs the route dynamics scheduler in a goroutine.
func runRouteScheduler(ctx context.Context, cfg RouteConfig, seed uint64, peerCount int, es *establishedState, guard *guard.Guard, channels []chan route.Action, controlCh <-chan web.ControlCommand, stopTicksAt time.Time, quiet bool) {
	sched := route.NewScheduler(route.SchedulerConfig{
		Seed:       seed + 1,
		PeerCount:  peerCount,
		Rate:       cfg.Rate,
		Interval:   cfg.Interval,
		Warmup:     cfg.Warmup,
		BaseRoutes: cfg.BaseRoutes,
	})

	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()

	paused := false

	for {
		select {
		case <-ctx.Done():
			return
		case cmd, ok := <-controlCh:
			if !ok {
				return
			}
			switch cmd.Type {
			case "pause":
				paused = true
				if !quiet {
					fmt.Fprintf(os.Stderr, "le chaos run | route-sched | paused\n")
				}
			case "resume":
				paused = false
				if !quiet {
					fmt.Fprintf(os.Stderr, "le chaos run | route-sched | resumed\n")
				}
			case "rate":
				sched.SetRate(cmd.Rate)
				if !quiet {
					fmt.Fprintf(os.Stderr, "le chaos run | route-sched | rate -> %.2f\n", cmd.Rate)
				}
			case "stop":
				if !quiet {
					fmt.Fprintf(os.Stderr, "le chaos run | route-sched | stopped by dashboard\n")
				}
				return
			}
		case now := <-ticker.C:
			if paused {
				continue
			}
			if !stopTicksAt.IsZero() && !now.Before(stopTicksAt) {
				continue
			}
			actions := sched.Tick(now, es.Snapshot())
			for _, a := range actions {
				if ok, reason := guard.AllowRoute(a.PeerIndex, a.Action.Type); !ok {
					if !quiet {
						fmt.Fprintf(os.Stderr, "le chaos run | route-sched | blocked %s for peer %d (%s)\n",
							a.Action.Type, a.PeerIndex, reason)
					}
					continue
				}
				if !quiet {
					fmt.Fprintf(os.Stderr, "le chaos run | route-sched | %s -> peer %d\n",
						a.Action.Type, a.PeerIndex)
				}
				select {
				case channels[a.PeerIndex] <- a.Action:
					if a.Action.Type == route.ActionFullWithdraw {
						guard.OnFullWithdraw(a.PeerIndex)
					}
				default:
					if !quiet {
						fmt.Fprintf(os.Stderr, "le chaos run | route-sched | dropped %s for peer %d (busy)\n",
							a.Action.Type, a.PeerIndex)
					}
				}
			}
		}
	}
}
