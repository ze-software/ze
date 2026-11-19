// Design: docs/architecture/fleet-interface.md -- the live event stream of one page
// Related: source_events.go -- Feed and Subscription, which this stream reads
// Related: assets/fleet.js -- the client, which fetches again the regions an event names
// Related: register.go -- the route, and the prototype-only step route
//
// The prototype-only routes at the end of this file advance a scripted source
// one event at a time, so a browser test decides when the estate changes.
//
// GET /fleet/events?scope=<page path> is a server-sent stream for one open
// page. It carries no data of the estate: each event names the regions of
// the page that are stale, and the script fetches their fragments again.
//
// The stream runs on the goroutine of its request and starts none. It holds
// one subscription, whose buffer the feed bounds, and it ends when the
// browser leaves, when a write does not complete in time, and when the feed
// ends the subscription.

package fleetui

import (
	"context"
	"errors"
	"html"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/a-h/templ"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The bounds of one stream.
const (
	// eventFlushInterval is how often a stream writes what went stale. A
	// burst of events inside one interval is one event on the stream.
	eventFlushInterval = time.Second
	// eventHeartbeatTicks is how many intervals with nothing stale pass
	// before the stream writes a comment line, so a proxy keeps it open and
	// a browser that left is noticed.
	eventHeartbeatTicks = 20
	// eventWriteTimeout is how long one write to the browser may take. A
	// browser that does not read for this long loses its stream.
	eventWriteTimeout = 10 * time.Second
	// eventScopeMax is the longest scope a stream address may carry.
	eventScopeMax = 2048
)

// The parts of the stream's wire format.
const (
	scopeParam     = "scope"
	eventStaleName = "stale"
	frameOpen      = ": open\n\n"
	frameHeartbeat = ": heartbeat\n\n"
)

// regionSet is a set of regions of a page. The zero value is the empty set.
type regionSet uint8

// The regions an event can make stale, in the order a frame names them.
const (
	regionTree regionSet = 1 << iota
	regionContent
	regionTasks
	regionHeader

	regionsAll = regionTree | regionContent | regionTasks | regionHeader
)

// regionNames pairs each region with its name on the stream, in the order a
// frame names them.
var regionNames = []struct {
	region regionSet
	name   string
}{
	{regionTree, fragmentTree},
	{regionContent, fragmentContent},
	{regionTasks, fragmentTasks},
	{regionHeader, fragmentHeader},
}

// frame answers the server-sent event that names the regions of the set. The
// set MUST NOT be empty.
func (regions regionSet) frame() string {
	names := make([]string, 0, len(regionNames))
	for _, pair := range regionNames {
		if regions&pair.region != 0 {
			names = append(names, pair.name)
		}
	}

	return textbuf.New().Str("event: ").Str(eventStaleName).Str("\ndata: ").Join(names, ",").Str("\n\n").Slice()
}

// eventScope is the devices whose changes make the content of one page
// stale: every device for the estate, the task list and the administration,
// the devices of the node for a group, the device itself for a device.
//
// The tree, the task panel and the header show the whole estate on every
// page, so their staleness does not depend on the scope: only the content
// region's does.
type eventScope struct {
	kind   ObjectKind
	device string
	group  []GroupLevel
	// members holds the names of the devices of a group scope.
	members map[string]struct{}
	// recheck reports that a device outside members changed. It may have
	// joined the group, so the members are read again before the next
	// write.
	recheck bool
}

// scopeRefused is the refusal for a stream address whose scope names no page.
func scopeRefused(scope string) error {
	problem := textbuf.New().Str("The live event stream was asked for the page ").Quoted(scope).
		Str(", which is no fleet page.").Slice()

	return opererr.Wrap(ErrAddressUnknown, problem,
		"A stream follows one page, so it cannot start without the address of one, and this page stays as it was read.",
		"reload the page")
}

// parseScope reads the scope of a stream address: the path of the page the
// stream follows.
func parseScope(text string) (Address, error) {
	if len(text) > eventScopeMax {
		return Address{}, scopeRefused(text[:eventScopeMax])
	}

	target, err := url.Parse(text)
	if err != nil {
		return Address{}, scopeRefused(text)
	}

	address, err := ParseAddress(target)
	if err != nil {
		return Address{}, scopeRefused(text)
	}

	return address, nil
}

// groupMembers answers the names of the devices of the group node at path,
// as the source holds them now.
func groupMembers(ctx context.Context, source Source, path []GroupLevel) (map[string]struct{}, error) {
	now, err := source.Now(ctx)
	if err != nil {
		return nil, err
	}

	devices, err := source.Devices(ctx)
	if err != nil {
		return nil, err
	}

	// Membership reads labels and registry fields only, so the join needs
	// neither the alarms nor the tasks.
	members, err := GroupDevices(DeriveDevices(devices, nil, nil, now), path)
	if err != nil {
		return nil, err
	}

	return viewNames(members), nil
}

// newEventScope builds the scope of the page at address. It refuses a group
// no device is in, with the data layer's own error.
func newEventScope(ctx context.Context, source Source, address Address) (*eventScope, error) {
	scope := &eventScope{kind: address.Kind, device: address.Device, group: address.Group}
	if address.Kind != ObjectGroup {
		return scope, nil
	}

	members, err := groupMembers(ctx, source, address.Group)
	if err != nil {
		return nil, err
	}
	scope.members = members

	return scope, nil
}

// holds reports whether a change of the device named name is inside the
// scope. For a group scope it notes a device outside the members, which may
// have joined the group.
func (scope *eventScope) holds(name string) bool {
	switch scope.kind {
	case ObjectEstate, ObjectTasks, ObjectAdmin:
		return true
	case ObjectDevice:
		return name == scope.device
	case ObjectGroup:
		if _, member := scope.members[name]; member {
			return true
		}
		scope.recheck = true

		return false
	case ObjectUnspecified:
		panic(bugAddressKind)
	default:
		panic(bugAddressKind)
	}
}

// stale answers the regions one event makes stale for a page of this scope.
func (scope *eventScope) stale(event Event) regionSet {
	switch event.Kind {
	case EventDeviceChanged, EventAlarmOpened, EventAlarmCleared:
		// The tree shows every device's state and alarm count.
		if scope.holds(event.Device) {
			return regionTree | regionContent
		}

		return regionTree
	case EventTaskCreated, EventTaskProgressed, EventTaskEnded:
		// A task event names its task and no device, so every page that
		// may show the task, or a spinner for it, is stale.
		return regionsAll
	case EventRegistryChanged:
		// A device was added, removed or relabeled: any group may have
		// changed, and the header's counts do not depend on it.
		return regionTree | regionContent
	case EventUnspecified:
		// A source defect. What changed is not known, so everything is
		// fetched again.
		return regionsAll
	default:
		// The set is open to a source built against a later version of this
		// package. What changed is not known, so everything is fetched again.
		return regionsAll
	}
}

// settle reads the members of a group scope again when a device outside them
// changed, and answers the regions that makes stale: the content, when the
// members differ or cannot be read.
func (scope *eventScope) settle(ctx context.Context, source Source) regionSet {
	if !scope.recheck {
		return 0
	}
	scope.recheck = false

	members, err := groupMembers(ctx, source, scope.group)
	if err != nil {
		// The group cannot be read, or holds no device now. Either way
		// what the page shows is no longer known to be current.
		return regionContent
	}

	same := len(members) == len(scope.members)
	for name := range members {
		if _, held := scope.members[name]; !held {
			same = false
		}
	}
	scope.members = members

	if same {
		return 0
	}

	return regionContent
}

// eventTicker answers a channel that delivers once per flush interval, and
// the function that stops it. The caller MUST call stop when it stops
// reading.
type eventTicker func() (ticks <-chan time.Time, stop func())

// wallTicker is the ticker a served stream uses.
func wallTicker() (<-chan time.Time, func()) {
	ticker := time.NewTicker(eventFlushInterval)

	return ticker.C, ticker.Stop
}

// eventStream is the handler of the live event stream. Safe for concurrent
// use: each request holds its own subscription and scope.
type eventStream struct {
	source Source
	// ticker paces the writes. A test supplies its own, so it decides when
	// a stream writes.
	ticker eventTicker
}

// newEventStream builds the stream handler over source.
func newEventStream(source Source) *eventStream {
	return &eventStream{source: source, ticker: wallTicker}
}

// writeFrame writes one frame to the browser and flushes it, under the write
// deadline of a stream.
func writeFrame(w io.Writer, controller *http.ResponseController, frame string) error {
	if err := controller.SetWriteDeadline(time.Now().Add(eventWriteTimeout)); err != nil {
		// A connection that takes no deadline is still written to: the
		// feed ends the subscription of a stream that falls behind.
		if !errors.Is(err, http.ErrNotSupported) {
			return err
		}
	}

	if _, err := io.WriteString(w, frame); err != nil {
		return err
	}

	return controller.Flush()
}

// ServeHTTP runs one stream until the browser leaves. A scope that names no
// page answers 400, a group no device is in answers 404, and a source that
// already serves its maximum of streams answers 503 with the source's own
// operator error: the page then works without live updates.
func (stream *eventStream) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	address, err := parseScope(r.URL.Query().Get(scopeParam))
	if err != nil {
		answerRefusal(w, err, http.StatusBadRequest)
		return
	}

	scope, err := newEventScope(ctx, stream.source, address)
	if err != nil {
		answerRefusal(w, err, failureStatus(err))
		return
	}

	subscription, err := stream.source.Subscribe(ctx)
	if err != nil {
		answerRefusal(w, err, http.StatusServiceUnavailable)
		return
	}
	defer subscription.Close()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	controller := http.NewResponseController(w)
	if writeFrame(w, controller, frameOpen) != nil {
		return
	}

	ticks, stop := stream.ticker()
	defer stop()

	stream.run(ctx, w, controller, subscription, scope, ticks)
}

// run is the loop of one stream. It collects the regions the events make
// stale and writes them once per tick, so a burst is one event per region
// per interval. It never ends on its own account: it returns when the
// request's context ends, when a write fails, and when the feed ends the
// subscription.
func (stream *eventStream) run(
	ctx context.Context, w io.Writer, controller *http.ResponseController,
	subscription *Subscription, scope *eventScope, ticks <-chan time.Time,
) {
	var pending regionSet
	idle := 0

	for {
		select {
		case <-ctx.Done():
			return

		case event, open := <-subscription.Events():
			if !open {
				finishStream(w, controller)
				return
			}
			pending |= scope.stale(event)

		case <-ticks:
			// Everything already published is counted before the write,
			// so an event and the tick after it never part ways.
			drained, open := drainEvents(subscription, scope)
			pending |= drained
			pending |= scope.settle(ctx, stream.source)

			if !open {
				finishStream(w, controller)
				return
			}

			if pending == 0 {
				idle++
				if idle < eventHeartbeatTicks {
					continue
				}
				idle = 0
				if writeFrame(w, controller, frameHeartbeat) != nil {
					return
				}

				continue
			}

			if writeFrame(w, controller, pending.frame()) != nil {
				return
			}
			pending = 0
			idle = 0
		}
	}
}

// drainEvents reads the events a subscription already holds, without
// waiting, and answers the regions they make stale. open is false when the
// feed ended the subscription. A subscription holds at most EventBufferSize
// events, which bounds the loop.
func drainEvents(subscription *Subscription, scope *eventScope) (stale regionSet, open bool) {
	for range EventBufferSize {
		select {
		case event, more := <-subscription.Events():
			if !more {
				return stale, false
			}
			stale |= scope.stale(event)
		default:
			return stale, true
		}
	}

	return stale, true
}

// finishStream ends a stream whose subscription the feed ended. Events may have
// been lost, or the hub stopped sending them, so the page is told that every
// region is stale: it fetches them again and subscribes anew.
func finishStream(w io.Writer, controller *http.ResponseController) {
	writeFrame(w, controller, regionsAll.frame()) //nolint:errcheck // the stream ends here whether or not the browser still reads
}

// Stepper is implemented by a data source whose events follow a script that
// can be advanced one event at a time, which the sample source does. It lets
// a browser test decide when the estate changes.
type Stepper interface {
	// Step applies the next event of the script, publishes it to the
	// source's subscribers and answers it. It refuses, with an operator
	// error, once the source has stopped.
	Step() (Event, error)
	// Hold stops the source from advancing its script on its own, for good.
	// A browser test calls it before it opens a page, so no event arrives
	// that it did not step.
	Hold()
}

// holdAnswer is what the hold route answers.
const holdAnswer = "held\n"

// newHoldHandler answers POST /fleet/prototype/hold: it takes the script of
// stepper away from its timer, and changes nothing of the estate. Asked by
// htmx, from the states page, it answers the line and a toast saying so.
func newHoldHandler(stepper Stepper) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stepper.Hold()

		if r.Header.Get(headerHTMXRequest) == htmxTrue {
			renderAnswer(r.Context(), w, http.StatusOK, scriptAnswer(holdAnswer, ToastView{
				Level:   ToastInformational,
				Message: "The sample script is held.",
				Detail:  "The estate changes only when Step is pressed, one event at a time.",
			}))

			return
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		io.WriteString(w, holdAnswer) //nolint:errcheck // the answer is committed; a failed write means the browser left
	})
}

// newStepHandler answers POST /fleet/prototype/step: it advances the script
// of stepper by one event and names the event it published, as its kind and
// the device it is about, on one line.
func newStepHandler(stepper Stepper) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		event, err := stepper.Step()
		if err != nil {
			answerRefusal(w, err, http.StatusServiceUnavailable)
			return
		}

		answer := textbuf.New().Str(event.Kind.String())
		if event.Device != "" {
			answer.Byte(' ').Str(event.Device)
		}

		// Asked by htmx, from the states page, the line goes in the page and
		// a toast names the event published.
		// String empties the buffer, so the line is read once.
		if r.Header.Get(headerHTMXRequest) == htmxTrue {
			line := answer.String()
			renderAnswer(r.Context(), w, http.StatusOK, scriptAnswer(line, ToastView{
				Level:   ToastInformational,
				Message: "The sample script published " + line + ".",
				Detail:  "Every page showing what the event changed fetches it again.",
			}))

			return
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		io.WriteString(w, answer.Byte('\n').Slice()) //nolint:errcheck // the answer is committed; a failed write means the browser left
	})
}

// scriptAnswer answers what the step and hold routes send to htmx: the line,
// as text, for the region the control targets, then the toast out of band.
func scriptAnswer(line string, toast ToastView) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		if _, err := io.WriteString(w, html.EscapeString(strings.TrimSpace(line))); err != nil {
			return err
		}

		return ToastOOB(toast).Render(ctx, w)
	})
}
