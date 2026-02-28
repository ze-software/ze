// Design: docs/architecture/chaos-web-dashboard.md — chaos and route dynamics schedulers
// Related: orchestrator_run.go — orchestrator that launches schedulers
// Related: orchestrator.go — ChaosConfig, RouteConfig, establishedState types
// Related: guard.go — peerGuard for action compatibility checks

package main

import (
	"context"
	"fmt"
	"math/rand/v2"
	"os"
	"time"

	"codeberg.org/thomas-mangin/ze/cmd/ze-chaos/chaos"
	"codeberg.org/thomas-mangin/ze/cmd/ze-chaos/peer"
	"codeberg.org/thomas-mangin/ze/cmd/ze-chaos/route"
	"codeberg.org/thomas-mangin/ze/cmd/ze-chaos/web"
)

// reconnectBackoff is the delay before a peer reconnects after a chaos disconnect.
const reconnectBackoff = 2 * time.Second

// runPeerLoop runs a peer simulator with reconnection after chaos disconnects.
// It loops until the context is canceled, restarting the simulator each time.
func runPeerLoop(ctx context.Context, cfg peer.SimulatorConfig, peerIndex int, events chan<- peer.Event) {
	for {
		peer.RunSimulator(ctx, cfg)

		// Exit if context is canceled (clean shutdown).
		if ctx.Err() != nil {
			return
		}

		// Emit reconnecting event.
		select {
		case events <- peer.Event{Type: peer.EventReconnecting, PeerIndex: peerIndex, Time: time.Now()}:
		case <-ctx.Done():
			return
		}

		// Brief backoff before reconnecting.
		select {
		case <-time.After(reconnectBackoff):
		case <-ctx.Done():
			return
		}
	}
}

// runScheduler runs the chaos scheduler goroutine. It ticks at the configured
// interval and dispatches chaos actions to per-peer channels.
// When controlCh is non-nil, it also processes dashboard control commands
// (pause, resume, rate change, manual trigger, stop).
func runScheduler(ctx context.Context, cfg ChaosConfig, seed uint64, peerCount int, es *establishedState, guard *peerGuard, channels []chan chaos.ChaosAction, controlCh <-chan web.ControlCommand, quiet bool) {
	sched := chaos.NewScheduler(chaos.SchedulerConfig{
		Seed:      seed,
		PeerCount: peerCount,
		Rate:      cfg.Rate,
		Interval:  cfg.Interval,
		Warmup:    cfg.Warmup,
	})

	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()

	paused := false

	// dispatchAction sends a chaos action to the peer's channel (non-blocking).
	dispatchAction := func(a chaos.ScheduledAction) {
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
			// Update guard immediately on successful dispatch so the route
			// scheduler sees the new state on its next tick, closing the
			// cross-scheduler race window. Event-loop updates still serve
			// as authoritative correction (establishment, disconnect).
			if a.Action.Type == chaos.ActionHoldTimerExpiry {
				guard.OnHoldTimerExpiry(a.PeerIndex)
			}
		default:
			if !quiet {
				fmt.Fprintf(os.Stderr, "ze-chaos | scheduler | dropped %s for peer %d (busy)\n",
					a.Action.Type, a.PeerIndex)
			}
		}
	}

	// handleControl processes a single control command. Returns true if stop requested.
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
				handleManualTrigger(cmd.Trigger, peerCount, es, guard, channels, quiet)
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

// handleManualTrigger dispatches a manually-triggered chaos action.
func handleManualTrigger(t *web.ManualTrigger, peerCount int, es *establishedState, guard *peerGuard, channels []chan chaos.ChaosAction, quiet bool) {
	actionType, ok := chaos.ActionTypeFromString(t.ActionType)
	if !ok {
		if !quiet {
			fmt.Fprintf(os.Stderr, "ze-chaos | scheduler | unknown trigger action: %s\n", t.ActionType)
		}
		return
	}

	// Determine target peers.
	targets := t.Peers
	if len(targets) == 0 {
		// Pick a random established peer.
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
		action := chaos.ChaosAction{Type: actionType}
		if !quiet {
			fmt.Fprintf(os.Stderr, "ze-chaos | scheduler | manual %s -> peer %d\n",
				actionType, idx)
		}
		select {
		case channels[idx] <- action:
			// Update guard immediately on successful dispatch.
			if actionType == chaos.ActionHoldTimerExpiry {
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

// runRouteScheduler runs the route dynamics scheduler in a goroutine.
// It mirrors runScheduler but dispatches route.Action instead of chaos.ChaosAction.
// When controlCh is non-nil, it processes dashboard control commands
// (pause, resume, rate change, stop).
func runRouteScheduler(ctx context.Context, cfg RouteConfig, seed uint64, peerCount int, es *establishedState, guard *peerGuard, channels []chan route.Action, controlCh <-chan web.ControlCommand, quiet bool) {
	sched := route.NewScheduler(route.SchedulerConfig{
		Seed:       seed + 1, // Different seed from chaos to avoid correlated scheduling.
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
					// Update guard immediately on successful dispatch so the
					// chaos scheduler sees the new state, closing the
					// cross-scheduler race window.
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
