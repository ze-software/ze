// Design: docs/architecture/fleet-interface.md -- the cards of the estate and group dashboards
// Related: handler_dashboard.go -- the card registry and the scope every card counts
// Related: summary.go -- the counts, rankings and lines these cards show
// Related: page_dashboard.templ -- the markup of the cards no shared component draws
//
// Every count a card shows is a link to the rows it counts. A count is taken
// with the functions of summary.go on the value a filter field reads, and
// its link carries that field's term, so the count and the rows behind the
// link cannot differ.

package fleetui

import (
	"context"
	"math"
	"slices"
	"strconv"
	"time"

	"github.com/a-h/templ"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The bounds of the cards.
const (
	// dashboardAlarmsMax is how many open alarms the Alarms card lists.
	dashboardAlarmsMax = 8
	// dashboardChangesMax is how many audit entries Recent changes lists.
	dashboardChangesMax = 8
	// dashboardRolloutsMax is how many rollouts the Rollouts card lists.
	dashboardRolloutsMax = 5
	// dashboardBusiestMax is how many devices Busiest devices ranks.
	dashboardBusiestMax = 5
)

// dashboardSiteKey is the label key whose values are the sites the Estate
// and Group cards count. The interface has no setting naming another one.
const dashboardSiteKey = "site"

// dashboardAlarmDeviceField is the field of an alarms filter that keeps the
// alarms of one device.
const dashboardAlarmDeviceField = "device"

// dashboardMetricDefault is what Busiest devices ranks by when the address
// names nothing.
const dashboardMetricDefault = MetricCPU

// dashboardRankMetrics are the choices of the Busiest devices card, in the
// order its links show them.
var dashboardRankMetrics = []Metric{MetricCPU, MetricMemory, MetricTrafficIn, MetricTrafficOut}

// The words a card shows for a segment that is not a state.
const (
	wordVersionOlder      = "Older"
	wordVersionUnreported = "Not reported"
)

// dashboardDevicesCard is the Devices card: a donut of the scope's devices by
// display state.
func dashboardDevicesCard(_ context.Context, _ *PageRequest, scope *dashboardScope, frame CardView) (templ.Component, error) {
	return DonutCard(devicesCardView(scope, frame)), nil
}

// devicesCardView answers the Devices card. Every state is a segment, a
// state with no device included, so the legend keeps its shape.
func devicesCardView(scope *dashboardScope, frame CardView) DonutCardView {
	counts := CountByState(scope.views)

	segments := make([]DonutSegmentView, 0, len(counts))
	for _, count := range counts {
		segments = append(segments, DonutSegmentView{
			Word:  count.State.String(),
			Href:  scope.devicesWith(fieldState, count.State.Key()),
			Info:  displayStateInfo(count.State),
			Count: count.Count,
			Tone:  DisplayStatus(count.State).Tone,
		})
	}

	frame.TitleHref = scope.tab(dashTabDevices, "").String()

	return DonutCardView{Card: frame, Donuts: []DonutView{{Icon: iconDevice, Segments: segments}}}
}

// displayStateInfo answers what the information mark of a display state
// says, or nothing for a state its word explains.
func displayStateInfo(state DisplayState) string {
	switch state {
	case DisplayStateDiverged:
		return "The device runs a configuration edited on the device, which differs from the hub's."
	case DisplayStateUnknown:
		return "The device is connected but has not reported its health yet."
	case DisplayStateOffline:
		return "The device is not connected to the hub."
	case DisplayStateHealthy, DisplayStateDegraded, DisplayStateDown, DisplayStateUnspecified:
		return ""
	}

	return ""
}

// dashboardConfigCard is the Configuration card: a donut of the scope's
// devices by configuration state.
func dashboardConfigCard(_ context.Context, _ *PageRequest, scope *dashboardScope, frame CardView) (templ.Component, error) {
	return DonutCard(configCardView(scope, frame)), nil
}

// configCardView answers the Configuration card.
func configCardView(scope *dashboardScope, frame CardView) DonutCardView {
	counts := CountByConfig(scope.views)

	segments := make([]DonutSegmentView, 0, len(counts))
	for _, count := range counts {
		segments = append(segments, DonutSegmentView{
			Word:  count.State.String(),
			Href:  scope.devicesWith(fieldConfig, count.State.Key()),
			Info:  configStateInfo(count.State),
			Count: count.Count,
			Tone:  ConfigStatus(count.State).Tone,
		})
	}

	frame.TitleHref = scope.tab(dashTabDevices, "").String()

	return DonutCardView{Card: frame, Donuts: []DonutView{{Icon: iconAdmin, Segments: segments}}}
}

// configStateInfo answers what the information mark of a configuration
// state says, or nothing for a state its word explains.
func configStateInfo(state ConfigState) string {
	switch state {
	case ConfigStatePending:
		return "The hub holds a configuration the device has not applied yet."
	case ConfigStateRejected:
		return "The device refused the configuration the hub sent."
	case ConfigStateDiverged:
		return "The device's configuration was edited on the device."
	case ConfigStateNeverFetched:
		return "The hub has never fetched the device's configuration."
	case ConfigStateInSync, ConfigStateUnspecified:
		return ""
	}

	return ""
}

// dashboardSoftwareCard is the Software card: a donut of the scope's devices
// by software version.
func dashboardSoftwareCard(_ context.Context, _ *PageRequest, scope *dashboardScope, frame CardView) (templ.Component, error) {
	return DonutCard(softwareCardView(scope, frame)), nil
}

// softwareCardView answers the Software card: the newest versions each on
// their own, then the older ones together, then the devices that reported
// none. A version segment has a chart colour, as a version is not a state.
func softwareCardView(scope *dashboardScope, frame CardView) DonutCardView {
	counts := CountByVersion(scope.views)

	segments := make([]DonutSegmentView, 0, len(counts))
	for index, count := range counts {
		segment := DonutSegmentView{Count: count.Count, Href: softwareHref(scope, count)}
		switch count.Bucket {
		case VersionBucketNamed:
			segment.Word = count.Versions[0]
			segment.Color = index + 1
		case VersionBucketOlder:
			segment.Word = wordVersionOlder
			segment.Color = VersionsNamedMax + 1
		case VersionBucketUnreported:
			segment.Word = wordVersionUnreported
			segment.Tone = ToneMuted
		case VersionBucketUnspecified:
			panic("BUG: fleetui: CountByVersion answered a segment that is no bucket")
		}
		segments = append(segments, segment)
	}

	frame.TitleHref = scope.tab(dashTabDevices, "").String()

	return DonutCardView{Card: frame, Donuts: []DonutView{{Icon: iconDevice, Segments: segments}}}
}

// softwareHref answers the devices one segment of the Software card counts:
// the devices of its versions, or for the devices that reported none, the
// devices of no reported version.
func softwareHref(scope *dashboardScope, count VersionCount) string {
	if count.Bucket != VersionBucketUnreported {
		return scope.devicesWith(fieldVersion, textbuf.New().Join(count.Versions, filterListSeparator).Slice())
	}

	reported := Versions(scope.views)
	// No device reported a version, so every device is in the segment, and
	// a negated term with no value is not a filter.
	if len(reported) == 0 {
		return scope.tab(dashTabDevices, "").String()
	}

	return scope.devicesWith(fieldVersion, filterNegation+textbuf.New().Join(reported, filterListSeparator).Slice())
}

// dashboardEstateCard is the Estate card: the figures of the estate.
func dashboardEstateCard(_ context.Context, request *PageRequest, scope *dashboardScope, frame CardView) (templ.Component, error) {
	frame.TitleHref = scope.tab(dashTabDevices, "").String()

	return FiguresCard(FiguresCardView{Card: frame, Figures: estateFigures(request, scope)}), nil
}

// estateFigures answers the figures of the Estate card and of the Group
// card: devices, sites, BGP peers up (established) of those configured, and
// open alarms. The caption is short so it holds on one line and the two by
// two grid keeps its baselines. The peers live on each device's routing tab, so their figure
// links nowhere.
func estateFigures(request *PageRequest, scope *dashboardScope) []FigureView {
	figures := Figures(scope.views, scope.alarms, dashboardSiteKey)
	labels := Address{Kind: ObjectAdmin, Tab: dashTabLabels, View: linkView(request)}

	return []FigureView{
		{Caption: "Devices", Value: strconv.Itoa(figures.Devices), Href: scope.tab(dashTabDevices, "").String()},
		{Caption: "Sites", Value: strconv.Itoa(figures.Sites), Href: labels.String()},
		{
			Caption: "Peers up",
			Value:   strconv.Itoa(figures.PeersEstablished),
			Unit:    "of " + strconv.Itoa(figures.PeersConfigured),
		},
		{Caption: "Open alarms", Value: strconv.Itoa(figures.AlarmsOpen), Href: scope.alarmsHref()},
	}
}

// dashTabLabels is the administration page listing the label values in use,
// sites among them.
const dashTabLabels = "labels"

// dashboardGroupCard is the Group card: the label values that define the
// node, then its figures.
func dashboardGroupCard(_ context.Context, request *PageRequest, scope *dashboardScope, frame CardView) (templ.Component, error) {
	return dashboardGroup(groupCardView(request, scope, frame)), nil
}

// groupCardView answers the Group card.
func groupCardView(request *PageRequest, scope *dashboardScope, frame CardView) DashboardGroupView {
	labels := make([]TagView, 0, len(scope.object.Group))
	for _, level := range scope.object.Group {
		tag := TagView{Key: level.Key, Value: level.Value}
		if level.Missing {
			tag.Value = "(none)"
		}
		labels = append(labels, tag)
	}

	frame.TitleHref = scope.tab(dashTabDevices, "").String()

	return DashboardGroupView{Card: frame, Labels: labels, Figures: estateFigures(request, scope)}
}

// alarmsHref answers the alarms the scope counts: the object's alarms tab,
// or for a device, which has none, the estate's alarms of that device.
func (scope *dashboardScope) alarmsHref() string {
	if scope.device == nil {
		return scope.tab(dashTabAlarms, "").String()
	}

	estate := Address{Kind: ObjectEstate, Tab: dashTabAlarms, View: scope.object.View}
	estate.View.Filter = SetFilterTerm("", dashboardAlarmDeviceField, scope.device.Device.Name)

	return estate.String()
}

// dashboardAlarmsCard is the Alarms card: the most severe open alarms of
// the scope, with their count beside the title.
func dashboardAlarmsCard(_ context.Context, request *PageRequest, scope *dashboardScope, frame CardView) (templ.Component, error) {
	return dashboardAlarms(alarmsCardView(request, scope, frame)), nil
}

// alarmsCardView answers the Alarms card.
func alarmsCardView(request *PageRequest, scope *dashboardScope, frame CardView) DashboardAlarmsView {
	now := request.Snapshot.Now
	top := TopAlarms(scope.alarms, dashboardAlarmsMax)

	rows := make([]AlarmRowView, 0, len(top))
	for index := range top {
		alarm := &top[index]
		summary := alarm.Summary
		if summary == "" {
			summary = alarm.Kind
		}
		rows = append(rows, AlarmRowView{
			Device:     alarm.Device,
			DeviceHref: deviceAddress(request, alarm.Device).String(),
			Summary:    summary,
			Since:      RelativeTime(now.Sub(alarm.Since)),
			SinceExact: dashboardClock(alarm.Since),
			Severity:   AlarmStatus(alarm.Severity),
		})
	}

	href := scope.alarmsHref()
	frame.TitleHref = href
	frame.Counted = true
	frame.Count = len(scope.alarms)
	frame.Link = &LinkView{Label: "See all", Href: href}

	return DashboardAlarmsView{
		Card: frame,
		List: AlarmListView{Rows: rows, Empty: GoodNewsView{First: "No open alarms.", Second: "Every device is quiet."}},
	}
}

// dashboardAttentionCard is the Needs attention card. The estate counts the
// paused rollouts too; a rollout belongs to no group, so a group counts its
// devices only.
func dashboardAttentionCard(ctx context.Context, request *PageRequest, scope *dashboardScope, frame CardView) (templ.Component, error) {
	rollouts := []Rollout{}
	if scope.object.Kind == ObjectEstate {
		answer, err := request.Source.Rollouts(ctx)
		if err != nil {
			return nil, err
		}
		rollouts = answer
	}

	lines, err := NeedsAttention(scope.views, rollouts)
	if err != nil {
		return nil, err
	}

	return dashboardAttention(attentionCardView(scope, lines, frame)), nil
}

// attentionCardView answers the Needs attention card: each line links to the
// view its filter keeps, the rollouts table for the paused rollouts and the
// devices table of the scope for the others.
func attentionCardView(scope *dashboardScope, lines []AttentionLine, frame CardView) DashboardAttentionView {
	views := make([]DashboardLineView, 0, len(lines))
	for _, line := range lines {
		href := scope.tab(dashTabDevices, line.Filter).String()
		if line.Kind == AttentionRolloutPaused {
			href = scope.tab(dashTabRollouts, line.Filter).String()
		}
		views = append(views, DashboardLineView{Count: line.Count, Text: line.Kind.String(), Href: href})
	}

	frame.TitleHref = scope.tab(dashTabDevices, "").String()

	return DashboardAttentionView{
		Card:  frame,
		Lines: views,
		Empty: GoodNewsView{First: "Nothing needs attention.", Second: "No device and no rollout waits for an operator."},
	}
}

// dashboardRolloutsCard is the Rollouts card of the estate.
func dashboardRolloutsCard(ctx context.Context, request *PageRequest, scope *dashboardScope, frame CardView) (templ.Component, error) {
	rollouts, err := request.Source.Rollouts(ctx)
	if err != nil {
		return nil, err
	}

	ordered := slices.Clone(rollouts)
	slices.SortStableFunc(ordered, func(a, b Rollout) int {
		if activeA, activeB := rolloutActive(a.State), rolloutActive(b.State); activeA != activeB {
			if activeA {
				return -1
			}
			return 1
		}

		return b.Started.Compare(a.Started)
	})
	ordered = ordered[:min(len(ordered), dashboardRolloutsMax)]

	rows := make([]DashboardRolloutView, 0, len(ordered))
	for index := range ordered {
		rollout := &ordered[index]
		target := scope.tab(dashTabRollouts, "")
		target.View.Row = rollout.Name
		rows = append(rows, DashboardRolloutView{
			Name:     rollout.Name,
			Href:     target.String(),
			Progress: textbuf.New().Int(int64(rollout.BatchesDone)).Str(" of ").Int(int64(rollout.BatchesTotal)).Str(" batches").Slice(),
			Actor:    rollout.Actor,
			Status:   rolloutStatus(rollout.State),
		})
	}

	frame.TitleHref = scope.tab(dashTabRollouts, "").String()

	return dashboardRollouts(DashboardRolloutsView{Card: frame, Rows: rows, Empty: "No rollout has run yet."}), nil
}

// rolloutActive reports whether a rollout has not finished.
func rolloutActive(state RolloutState) bool {
	switch state {
	case RolloutStatePending, RolloutStateRolling, RolloutStatePaused:
		return true
	case RolloutStateComplete, RolloutStateFailed, RolloutStateAborted, RolloutStateUnspecified:
		return false
	}

	return false
}

// rolloutStatus answers how a rollout state is drawn.
func rolloutStatus(state RolloutState) Status {
	tone := ToneNeutral
	switch state {
	case RolloutStatePending:
		tone = ToneAccent
	case RolloutStateRolling:
		tone = ToneProgress
	case RolloutStatePaused:
		tone = ToneWarning
	case RolloutStateComplete:
		tone = ToneSuccess
	case RolloutStateFailed:
		tone = ToneError
	case RolloutStateAborted:
		tone = ToneMuted
	case RolloutStateUnspecified:
		tone = ToneNeutral
	}

	return Status{Tone: tone, Word: state.String()}
}

// dashboardChangesCard is the Recent changes card of the estate: the last
// audit entries, the newest first.
func dashboardChangesCard(ctx context.Context, request *PageRequest, _ *dashboardScope, frame CardView) (templ.Component, error) {
	entries, err := request.Source.AuditEntries(ctx)
	if err != nil {
		return nil, err
	}

	ordered := slices.Clone(entries)
	slices.SortStableFunc(ordered, func(a, b AuditEntry) int { return b.At.Compare(a.At) })
	ordered = ordered[:min(len(ordered), dashboardChangesMax)]

	now := request.Snapshot.Now
	audit := Address{Kind: ObjectAdmin, Tab: dashTabAudit, View: linkView(request)}

	rows := make([]DashboardChangeView, 0, len(ordered))
	for index := range ordered {
		entry := &ordered[index]
		target := audit
		target.View.Row = entry.ID
		what := entry.Summary
		if what == "" {
			what = textbuf.New().Str(entry.Event).Byte(' ').Str(entry.Target).Slice()
		}
		rows = append(rows, DashboardChangeView{
			When:    RelativeTime(now.Sub(entry.At)),
			Exact:   dashboardClock(entry.At),
			Actor:   entry.Actor,
			What:    what,
			Devices: dashboardDevicesText(entry.Devices),
			Href:    target.String(),
		})
	}

	frame.TitleHref = audit.String()

	return dashboardChanges(DashboardChangesView{Card: frame, Rows: rows, Empty: "Nothing has changed yet."}), nil
}

// dashboardDevicesText answers "1 device" or "4 devices".
func dashboardDevicesText(count int) string {
	if count == 1 {
		return "1 device"
	}

	return textbuf.New().Int(int64(count)).Str(" devices").Slice()
}

// dashboardBusiestCard is the Busiest devices card. What it ranks by is the
// sort parameter of the dashboard's address, so the choice is in the
// address; a metric the last sample does not hold is refused, and the card
// shows the refusal.
func dashboardBusiestCard(_ context.Context, request *PageRequest, scope *dashboardScope, frame CardView) (templ.Component, error) {
	view, err := busiestCardView(request, scope, frame)
	if err != nil {
		return nil, err
	}

	return dashboardBusiest(view), nil
}

// busiestMetric answers the metric the address ranks by. An unknown name is
// the invalid metric, which Busiest refuses with the list of the valid ones.
func busiestMetric(request *PageRequest) Metric {
	key := request.Address.View.Sort.Column
	if key == "" {
		return dashboardMetricDefault
	}

	metric, known := ParseMetric(key)
	if !known {
		return MetricUnspecified
	}

	return metric
}

// busiestCardView answers the Busiest devices card.
func busiestCardView(request *PageRequest, scope *dashboardScope, frame CardView) (DashboardBusiestView, error) {
	metric := busiestMetric(request)

	ranked, err := Busiest(scope.views, metric, dashboardBusiestMax)
	if err != nil {
		return DashboardBusiestView{}, err
	}

	choices := make([]DashboardChoiceView, 0, len(dashboardRankMetrics))
	for _, choice := range dashboardRankMetrics {
		target := request.Address
		target.View.Sort = SortSpec{Column: choice.Key()}
		if choice == dashboardMetricDefault {
			target.View.Sort = SortSpec{}
		}
		choices = append(choices, DashboardChoiceView{Label: choice.String(), Href: target.String(), Current: choice == metric})
	}

	top := 0.0
	if len(ranked) > 0 {
		top = ranked[0].Value
	}

	rows := make([]DashboardBusyView, 0, len(ranked))
	for _, entry := range ranked {
		rows = append(rows, busyView(request, metric, entry, top))
	}

	frame.TitleHref = busiestHref(scope, metric)

	return DashboardBusiestView{
		Card: frame, Choices: choices, Rows: rows, Empty: "No device has reported a sample yet.",
	}, nil
}

// busyView answers one device of the ranking. A percentage is drawn as it
// is; any other figure is drawn as its share of the busiest device's, with
// the figure itself beside the bar.
func busyView(request *PageRequest, metric Metric, entry Ranked, top float64) DashboardBusyView {
	view := DashboardBusyView{Device: entry.Device, Href: deviceAddress(request, entry.Device).String()}
	view.Bar = UsageBarView{Name: metric.String()}

	if metric.Unit() == "%" {
		view.Bar.Percent = int(math.Round(entry.Value))
		view.Bar.Known = true

		return view
	}

	view.Bar.Percent, view.Bar.Known = Percent(entry.Value, top)
	view.Value = metricValueText(metric, entry.Value)

	return view
}

// busiestHref answers the devices table sorted by the ranked metric, the
// busiest first, when the table has that column, and unsorted otherwise.
func busiestHref(scope *dashboardScope, metric Metric) string {
	target := scope.tab(dashTabDevices, "")
	if slices.Contains(DeviceColumns().Keys(), metric.Key()) {
		target.View.Sort = SortSpec{Column: metric.Key(), Descending: true}
	}

	return target.String()
}

// metricValueText writes a value of metric with its unit: a rate in the
// largest unit that keeps it above one, any other value as a number.
func metricValueText(metric Metric, value float64) string {
	if metric.Unit() != metricUnitRate {
		text := textbuf.New().Float(value, 1)
		if unit := metric.Unit(); unit != "" {
			text.Byte(' ').Str(unit)
		}

		return text.Slice()
	}

	return rateText(value)
}

// metricUnitRate is the unit of a traffic metric.
const metricUnitRate = "bit/s"

// rateUnits are the units of a rate, each a thousand times the last.
var rateUnits = []string{"bit/s", "kbit/s", "Mbit/s", "Gbit/s", "Tbit/s"}

// rateText writes a rate in bits per second in the largest unit that keeps
// it at one or above.
func rateText(bps float64) string {
	unit := 0
	for bps >= 1000 && unit < len(rateUnits)-1 {
		bps /= 1000
		unit++
	}

	return textbuf.New().Float(bps, 1).Byte(' ').Str(rateUnits[unit]).Slice()
}

// byteUnits are the units of a size, each 1024 times the last.
var byteUnits = []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB"}

// bytesFigure answers a size as a figure and its unit, in the largest unit
// that keeps it at one or above: 2.3 and TiB.
func bytesFigure(size uint64) (value, unit string) {
	figure := float64(size)
	index := 0
	for figure >= 1024 && index < len(byteUnits)-1 {
		figure /= 1024
		index++
	}

	return textbuf.New().Float(figure, 1).Slice(), byteUnits[index]
}

// dashboardClock writes a time as a card shows it beside a relative age.
func dashboardClock(at time.Time) string {
	return at.UTC().Format(dashboardClockLayout) + " UTC"
}

// dashboardClockLayout is the layout of dashboardClock.
const dashboardClockLayout = "2006-01-02 15:04"
