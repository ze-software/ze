// Design: docs/architecture/ddos/cp-survival-5-detect-0-umbrella.md -- leak-probe state machine

package flowspec

type probeAction int

const (
	probeActionNone      probeAction = iota
	probeActionProbe                 // narrow to probe-rate
	probeActionReTighten             // saturated, re-tighten + backoff
	probeActionWithdraw              // attack over, withdraw
)

func (a probeAction) String() string {
	switch a {
	case probeActionNone:
		return "none"
	case probeActionProbe:
		return "probe"
	case probeActionReTighten:
		return "re-tighten"
	case probeActionWithdraw:
		return "withdraw"
	default:
		return "unknown"
	}
}

type probeState int

const (
	probeStateHoldDown probeState = iota
	probeStateWaiting
	probeStateProbing
)

type probe struct {
	holdDown      int
	probeInterval int
	probeWindow   int
	probeRate     float64
	backoffCap    int

	started         bool
	state           probeState
	elapsed         int
	currentHoldDown int
	probeTicks      int
	clearTicks      int
	sinceLastProbe  int
}

func newProbe(holdDown, probeInterval, probeWindow int, probeRate float64, backoffCap int) *probe {
	return &probe{
		holdDown:        holdDown,
		probeInterval:   probeInterval,
		probeWindow:     probeWindow,
		probeRate:       probeRate,
		backoffCap:      backoffCap,
		currentHoldDown: holdDown,
	}
}

func (p *probe) Start() {
	p.started = true
	p.state = probeStateHoldDown
	p.elapsed = 0
	p.probeTicks = 0
	p.clearTicks = 0
	p.sinceLastProbe = 0
	p.currentHoldDown = p.holdDown
}

func (p *probe) Stop() {
	p.started = false
}

func (p *probe) Tick(observedBps float64) probeAction {
	if !p.started {
		return probeActionNone
	}

	p.elapsed++

	switch p.state {
	case probeStateHoldDown:
		if p.elapsed > p.currentHoldDown {
			p.state = probeStateWaiting
			p.sinceLastProbe = 0
		}
		return probeActionNone

	case probeStateWaiting:
		p.sinceLastProbe++
		if p.sinceLastProbe >= p.probeInterval {
			p.state = probeStateProbing
			p.probeTicks = 0
			p.clearTicks = 0
			return probeActionProbe
		}
		return probeActionNone

	case probeStateProbing:
		p.probeTicks++
		saturated := observedBps >= p.probeRate
		if saturated {
			p.clearTicks = 0
		} else {
			p.clearTicks++
		}

		if saturated {
			p.currentHoldDown = min(p.currentHoldDown*2, p.backoffCap)
			p.state = probeStateHoldDown
			p.elapsed = 0
			p.sinceLastProbe = 0
			return probeActionReTighten
		}

		if p.probeTicks >= p.probeWindow {
			p.started = false
			return probeActionWithdraw
		}
		return probeActionNone
	}

	return probeActionNone
}
