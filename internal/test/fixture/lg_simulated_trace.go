// Design: docs/architecture/chaos-web-dashboard.md -- the trace graph of a simulated network, compared with the model's traces
// Related: lg_simulated_network.go -- the run this question is asked of, and the list it is one entry of

package fixture

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"

	chaosscenario "github.com/ze-software/ze/internal/chaos/scenario"
)

// The words of the trace graph's text form this file reads
// (internal/component/lg/graph_trace.go, renderTraceGraphText).
const (
	lgSimulatedTraceHeader  = "mode trace"
	lgSimulatedTraceKeyword = "trace"
	lgSimulatedTraceSays    = " says: "
	lgSimulatedTraceRouters = "routers"
	lgSimulatedTraceEnd     = "end"
	lgSimulatedTraceNextHop = "next-hop"
	lgSimulatedTraceASPath  = "as-path"
	lgSimulatedTraceExit    = "exit"
	// lgSimulatedChainRouters is the shortest chain that proves the walk
	// crosses a router between its start and its exit (AC-24).
	lgSimulatedChainRouters = 3
)

// lgSimulatedTraceLine is the fields of one `trace` line. The sentence after
// `says:` is not kept: it may be reworded, and the fields carry the answer.
type lgSimulatedTraceLine struct {
	routers []string
	end     string
	nextHop string
	asPath  []string
}

// lgSimulatedTraceText is what the fixture reads of one trace graph: the
// `trace` line of each starting router, and the routers marked as exits.
type lgSimulatedTraceText struct {
	traces map[string]lgSimulatedTraceLine
	exits  []string
}

// lgSimulatedTraceEndName answers the `end=` value the text form writes for a
// model's end. The model has no equal for the text's four other ends (a loop,
// a router with no route, several routes, an unreadable route): the converged
// network it computes produces none of them, so one in the text is a
// difference.
func lgSimulatedTraceEndName(end chaosscenario.TraceEnd) (string, error) {
	switch end {
	case chaosscenario.TraceEndExitOwnAddress:
		return "exit-own-address", nil
	case chaosscenario.TraceEndExitExternalAddress:
		return "exit-external-address", nil
	case chaosscenario.TraceEndOriginInside:
		return "origin-inside", nil
	case chaosscenario.TraceEndUnspecified:
		return "", errors.New("the model answered a trace with no end: a defect of the topology model")
	}
	return "", fmt.Errorf("the model answered the trace end %d, which this fixture does not know", end)
}

// lgSimulatedReadTrace reads the text form of one trace graph of prefix.
func lgSimulatedReadTrace(text string, prefix netip.Prefix) (lgSimulatedTraceText, error) {
	read := lgSimulatedTraceText{traces: map[string]lgSimulatedTraceLine{}}
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	if lines[0] != lgSimulatedTraceHeader {
		return read, fmt.Errorf("the trace graph of %s does not start with %q:\n%s", prefix, lgSimulatedTraceHeader, text)
	}

	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) >= 4 && fields[0] == "node" && fields[1] == "router" && fields[3] == lgSimulatedTraceExit {
			read.exits = append(read.exits, fields[2])
		}
		if len(fields) == 0 || fields[0] != lgSimulatedTraceKeyword {
			continue
		}

		head, _, stated := strings.Cut(line, lgSimulatedTraceSays)
		if !stated {
			return read, fmt.Errorf("a trace line of %s states no end after %q: %s", prefix, lgSimulatedTraceSays, line)
		}
		fields = strings.Fields(head)
		if len(fields) < 4 {
			return read, fmt.Errorf("a trace line of %s names no family, prefix and router: %s", prefix, line)
		}
		if fields[2] != prefix.String() {
			return read, fmt.Errorf("the trace graph of %s holds a trace of %s: %s", prefix, fields[2], line)
		}
		start := fields[3]
		if _, twice := read.traces[start]; twice {
			return read, fmt.Errorf("the trace graph of %s holds two traces from %s", prefix, start)
		}

		var trace lgSimulatedTraceLine
		for _, field := range fields[4:] {
			key, value, isPair := strings.Cut(field, "=")
			if !isPair {
				return read, fmt.Errorf("a trace line of %s holds %q, which is no key=value: %s", prefix, field, line)
			}
			switch key {
			case lgSimulatedTraceRouters:
				trace.routers = strings.Split(value, ",")
			case lgSimulatedTraceEnd:
				trace.end = value
			case lgSimulatedTraceNextHop:
				trace.nextHop = value
			case lgSimulatedTraceASPath:
				trace.asPath = strings.Split(value, ",")
			default:
				// The text form is this repository's own, so a key this
				// reader does not know is a change it must learn.
				return read, fmt.Errorf("a trace line of %s holds the unknown key %q: %s", prefix, key, line)
			}
		}
		read.traces[start] = trace
	}
	return read, nil
}

// lgSimulatedTraceWant answers the fields the model expects on the `trace`
// line of one walk.
func lgSimulatedTraceWant(trace chaosscenario.Trace) (lgSimulatedTraceLine, error) {
	end, err := lgSimulatedTraceEndName(trace.End)
	if err != nil {
		return lgSimulatedTraceLine{}, err
	}
	want := lgSimulatedTraceLine{end: end, nextHop: trace.NextHop.String()}
	for _, router := range trace.Routers {
		want.routers = append(want.routers, router.String())
	}
	// The text writes the AS path of an exit route only: a prefix that
	// originates inside the AS has none.
	if trace.End != chaosscenario.TraceEndOriginInside {
		for _, number := range trace.ASPath {
			want.asPath = append(want.asPath, strconv.FormatUint(uint64(number), 10))
		}
	}
	return want, nil
}

// lgSimulatedTraceProblem compares one `trace` line with the model's walk and
// answers what differs, "" when nothing does.
func lgSimulatedTraceProblem(got, want lgSimulatedTraceLine) string {
	if !slices.Equal(got.routers, want.routers) {
		return fmt.Sprintf("crosses the routers %v, and the model computed %v", got.routers, want.routers)
	}
	if got.end != want.end {
		return fmt.Sprintf("ends as %q, and the model computed %q", got.end, want.end)
	}
	if got.nextHop != want.nextHop {
		return fmt.Sprintf("leaves by the next hop %q, and the model computed %q", got.nextHop, want.nextHop)
	}
	if !slices.Equal(got.asPath, want.asPath) {
		return fmt.Sprintf("carries the AS path %v, and the model computed %v", got.asPath, want.asPath)
	}
	return ""
}

// lgSimulatedTraceTally counts what the walks of the whole network proved, so
// a network that held none of a case fails instead of passing over it.
type lgSimulatedTraceTally struct {
	traces       int
	chains       int // Walks that cross at least lgSimulatedChainRouters routers (AC-24).
	oneExit      int // Prefixes one neighbour announces, every walk ending at one exit (AC-22).
	originInside int // Walks that end where the prefix originates inside the AS (AC-30).
	externalExit int // Walks that leave the AS toward an address of no router (AC-27).
}

// lgSimulatedTraceGraph is the trace graph question of the generated
// network: every walk is the model's (lgSimulatedTraceCompare), and the
// network held each case the generated shape exists to prove.
func lgSimulatedTraceGraph(ctx context.Context, base string, topology *chaosscenario.Topology) error {
	tally, err := lgSimulatedTraceCompare(ctx, base, topology)
	if err != nil {
		return err
	}
	if tally.chains == 0 {
		return fmt.Errorf("no walk of the network crosses %d routers, so the walk across the reflector is unproven", lgSimulatedChainRouters)
	}
	if tally.oneExit == 0 {
		return errors.New("no prefix of the network has one announcer, so the single exit is unproven")
	}
	if tally.originInside == 0 {
		return errors.New("no prefix of the network originates inside the AS, so that end is unproven")
	}
	fmt.Fprintf(os.Stderr,
		"OK: every trace graph traced each router as the model computed: %d traces, %d across three routers or more, "+
			"%d prefixes with one exit, %d ending at an origin inside the AS\n",
		tally.traces, tally.chains, tally.oneExit, tally.originInside)
	return nil
}

// lgSimulatedTraceCompare asks the trace graph of every prefix of the model,
// in its text form, and compares the walk from every router with the model's
// trace: the routers crossed in order, the end, the exit next hop and the AS
// path (AC-21). The model computed the traces apart from the looking glass
// and from the RIB, so the two cannot agree on a shared mistake (R-10).
//
// It answers what the walks proved. The caller MUST refuse a tally that lacks
// the case its network exists to prove: a network that held none of a case
// compares equal and proves nothing about it.
func lgSimulatedTraceCompare(ctx context.Context, base string, topology *chaosscenario.Topology) (lgSimulatedTraceTally, error) {
	var routers []netip.Addr
	for _, speaker := range topology.Speakers() {
		if speaker.Session == chaosscenario.SessionKindInternal {
			routers = append(routers, speaker.Address)
		}
	}

	var tally lgSimulatedTraceTally
	for _, prefix := range topology.Prefixes() {
		target := base + "/lg/graph?mode=trace&format=text&prefix=" + url.QueryEscape(prefix.String())
		text, err := lgCoveringRequest(ctx, http.MethodGet, target, "")
		if err != nil {
			return tally, err
		}
		read, err := lgSimulatedReadTrace(text, prefix)
		if err != nil {
			return tally, err
		}
		if len(read.traces) != len(routers) {
			return tally, fmt.Errorf("the trace graph of %s holds %d traces, and the network has %d routers:\n%s",
				prefix, len(read.traces), len(routers), text)
		}

		lastRouters := map[string]bool{}
		keptExternalNextHop := false
		for _, router := range routers {
			model, err := topology.Trace(router, prefix)
			if err != nil {
				return tally, err
			}
			want, err := lgSimulatedTraceWant(model)
			if err != nil {
				return tally, err
			}
			got, drawn := read.traces[router.String()]
			if !drawn {
				return tally, fmt.Errorf("the trace graph of %s holds no trace from %s:\n%s", prefix, router, text)
			}
			if problem := lgSimulatedTraceProblem(got, want); problem != "" {
				return tally, fmt.Errorf("the trace graph of %s: the trace from %s %s", prefix, router, problem)
			}

			tally.traces++
			lastRouters[want.routers[len(want.routers)-1]] = true
			if len(want.routers) >= lgSimulatedChainRouters {
				tally.chains++
			}
			if model.End == chaosscenario.TraceEndOriginInside {
				tally.originInside++
			}
			if model.End == chaosscenario.TraceEndExitExternalAddress {
				tally.externalExit++
				keptExternalNextHop = true
			}
		}

		announcers, err := topology.Announcers(prefix)
		if err != nil {
			return tally, err
		}
		if len(announcers) != 1 {
			continue
		}
		// A border router that keeps the neighbour's next hop gives every
		// router that address, so each walk leaves the AS at its own start and
		// the prefix has no single exit router to end at. The comparison with
		// the model above is the whole proof of such a prefix.
		if keptExternalNextHop {
			continue
		}
		// One neighbour announces the prefix: one exit router is drawn, and
		// every router's chain ends at it (AC-22).
		if len(read.exits) != 1 || len(lastRouters) != 1 || !lastRouters[read.exits[0]] {
			return tally, fmt.Errorf("the trace graph of %s, which one neighbour announces, marks the exits %v and its walks end at %v:\n%s",
				prefix, read.exits, lastRouters, text)
		}
		tally.oneExit++
	}

	return tally, nil
}
