// Design: docs/architecture/chaos-web-dashboard.md -- chaos and route dynamics schedulers

package orchestrator

import (
	"context"
	"fmt"
	"math/rand/v2"
	"os"
	"time"

	"github.com/ze-software/ze/internal/chaos/engine"
	"github.com/ze-software/ze/internal/chaos/guard"
	"github.com/ze-software/ze/internal/chaos/peer"
	"github.com/ze-software/ze/internal/chaos/route"
	"github.com/ze-software/ze/internal/chaos/web"
)

const reconnectBackoff = 2 * time.Second

// RunPeerLoop runs a peer simulator with reconnection after chaos disconnects.
func RunPeerLoop(ctx context.Context, cfg peer.SimulatorConfig, peerIndex int, events chan<- peer.Event) {
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

// RunScheduler runs the chaos scheduler goroutine.
func RunScheduler(ctx context.Context, cfg ChaosConfig, seed uint64, peerCount int, es *EstablishedState, guard *guard.Guard, channels []chan engine.ChaosAction, controlCh <-chan web.ControlCommand, quiet bool) {
	sched := engine.NewScheduler(engine.SchedulerConfig{
		Seed:           seed,
		PeerCount:      peerCount,
		Rate:           cfg.Rate,
		Interval:       cfg.Interval,
		Warmup:         cfg.Warmup,
		EnabledActions: cfg.EnabledActions,
	})

	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()

	paused := false

	dispatchAction := func(a engine.ScheduledAction) {
		if ok, reason := guard.AllowChaos(a.PeerIndex, a.Action.Type); !ok {
			if !quiet {
				fmt.Fprintf(os.Stderr, "ze-chaos | scheduler | blocked %s for peer %d (%s)\n",
					a.Action.Type, a.PeerIndex, reason)
			}
			return
		}
		if !quiet {
			fmt.Fprintf(os.Stderr, "ze-chaos | scheduler | %s -> peer %d\n",
				a.Action.Type, a.PeerIndex)
		}
		select {
		case channels[a.PeerIndex] <- a.Action:
			if a.Action.Type == engine.ActionHoldTimerExpiry {
				guard.OnHoldTimerExpiry(a.PeerIndex)
			}
		default:
			if !quiet {
				fmt.Fprintf(os.Stderr, "ze-chaos | scheduler | dropped %s for peer %d (busy)\n",
					a.Action.Type, a.PeerIndex)
			}
		}
	}

	handleControl := func(cmd web.ControlCommand) bool {
		switch cmd.Type {
		case "pause":
			paused = true
			if !quiet {
				fmt.Fprintf(os.Stderr, "ze-chaos | scheduler | paused\n")
			}
		case "resume":
			paused = false
			if !quiet {
				fmt.Fprintf(os.Stderr, "ze-chaos | scheduler | resumed\n")
			}
		case "rate":
			sched.SetRate(cmd.Rate)
			if !quiet {
				fmt.Fprintf(os.Stderr, "ze-chaos | scheduler | rate -> %.2f\n", cmd.Rate)
			}
		case "trigger":
			if cmd.Trigger != nil {
				HandleManualTrigger(cmd.Trigger, peerCount, es, guard, channels, quiet)
			}
		case "stop":
			if !quiet {
				fmt.Fprintf(os.Stderr, "ze-chaos | scheduler | stopped by dashboard\n")
			}
			return true
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
			actions := sched.Tick(now, es.Snapshot())
			for _, a := range actions {
				dispatchAction(a)
			}
		}
	}
}

// HandleManualTrigger dispatches a manually-triggered chaos action.
func HandleManualTrigger(t *web.ManualTrigger, peerCount int, es *EstablishedState, guard *guard.Guard, channels []chan engine.ChaosAction, quiet bool) {
	actionType, ok := engine.ActionTypeFromString(t.ActionType)
	if !ok {
		if !quiet {
			fmt.Fprintf(os.Stderr, "ze-chaos | scheduler | unknown trigger action: %s\n", t.ActionType)
		}
		return
	}

	targets := t.Peers
	if len(targets) == 0 {
		snap := es.Snapshot()
		var established []int
		for i, est := range snap {
			if est {
				established = append(established, i)
			}
		}
		if len(established) > 0 {
			targets = []int{established[rand.IntN(len(established))]} //nolint:gosec // chaos simulator, not crypto
		}
	}

	for _, idx := range targets {
		if idx < 0 || idx >= peerCount {
			continue
		}
		if ok, reason := guard.AllowChaos(idx, actionType); !ok {
			if !quiet {
				fmt.Fprintf(os.Stderr, "ze-chaos | scheduler | blocked manual %s for peer %d (%s)\n",
					actionType, idx, reason)
			}
			continue
		}
		action := engine.ChaosAction{Type: actionType, Params: t.Params}
		if !quiet {
			fmt.Fprintf(os.Stderr, "ze-chaos | scheduler | manual %s -> peer %d\n",
				actionType, idx)
		}
		select {
		case channels[idx] <- action:
			if actionType == engine.ActionHoldTimerExpiry {
				guard.OnHoldTimerExpiry(idx)
			}
		default:
			if !quiet {
				fmt.Fprintf(os.Stderr, "ze-chaos | scheduler | dropped manual %s for peer %d (busy)\n",
					actionType, idx)
			}
		}
	}
}

// RunRouteScheduler runs the route dynamics scheduler in a goroutine.
func RunRouteScheduler(ctx context.Context, cfg RouteConfig, seed uint64, peerCount int, es *EstablishedState, guard *guard.Guard, channels []chan route.Action, controlCh <-chan web.ControlCommand, quiet bool) {
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
					fmt.Fprintf(os.Stderr, "ze-chaos | route-sched | paused\n")
				}
			case "resume":
				paused = false
				if !quiet {
					fmt.Fprintf(os.Stderr, "ze-chaos | route-sched | resumed\n")
				}
			case "rate":
				sched.SetRate(cmd.Rate)
				if !quiet {
					fmt.Fprintf(os.Stderr, "ze-chaos | route-sched | rate -> %.2f\n", cmd.Rate)
				}
			case "stop":
				if !quiet {
					fmt.Fprintf(os.Stderr, "ze-chaos | route-sched | stopped by dashboard\n")
				}
				return
			}
		case now := <-ticker.C:
			if paused {
				continue
			}
			actions := sched.Tick(now, es.Snapshot())
			for _, a := range actions {
				if ok, reason := guard.AllowRoute(a.PeerIndex, a.Action.Type); !ok {
					if !quiet {
						fmt.Fprintf(os.Stderr, "ze-chaos | route-sched | blocked %s for peer %d (%s)\n",
							a.Action.Type, a.PeerIndex, reason)
					}
					continue
				}
				if !quiet {
					fmt.Fprintf(os.Stderr, "ze-chaos | route-sched | %s -> peer %d\n",
						a.Action.Type, a.PeerIndex)
				}
				select {
				case channels[a.PeerIndex] <- a.Action:
					if a.Action.Type == route.ActionFullWithdraw {
						guard.OnFullWithdraw(a.PeerIndex)
					}
				default:
					if !quiet {
						fmt.Fprintf(os.Stderr, "ze-chaos | route-sched | dropped %s for peer %d (busy)\n",
							a.Action.Type, a.PeerIndex)
					}
				}
			}
		}
	}
}
