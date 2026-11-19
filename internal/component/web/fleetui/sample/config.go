// Design: docs/architecture/fleet-interface.md -- the configurations of the sample estate
// Overview: source.go -- the sample data source
// Related: estate.go -- the rows and roles a configuration is written from
// Related: views.go -- the peers a configuration declares
// Related: rollout.go -- the rollout that sent the newest revision
//
// Every device's configuration is written from its role's peers, in the
// syntax of docs/guide/configuration.md. It has been revised three times, and
// a revision is a row of revisions: what changed, who changed it, and when.
// Most devices run the third revision, which the hub holds too. configStages
// names the devices that differ, and the configuration counts of the estate
// follow from it.

package sample

import (
	"slices"
	"time"

	"github.com/ze-software/ze/internal/component/web/fleetui"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// What the estate's configuration events are measured from.
const (
	// rejectedFor is how long ago the rejecting device refused the newest
	// revision, and divergedFor how long ago the diverged device was last
	// edited locally.
	rejectedFor = 4*time.Hour + 28*time.Minute
	divergedFor = 9*time.Hour + 46*time.Minute
	// rolloutEdgeAge is how long ago the newest revision was written and its
	// rollout started.
	rolloutEdgeAge = 5 * time.Hour

	// rejectionAnswer is what the rejecting device answered, in its words.
	rejectionAnswer = "rejected at verify: bgp group transit: prefix maximum 1200000 is above " +
		"the 1000000 routes this device has memory for"

	resultApplied   = "applied"
	resultKeptLocal = "kept the local edit: the hub's version was not applied"

	// fetchDelay is how long after a revision is written a device fetches it.
	fetchDelay = 20 * time.Second
	// localChanges counts the local edits of the diverged device.
	localChanges = 3
)

// configShape is what differs between two texts of one device's
// configuration.
type configShape struct {
	holdTime  uint64
	prefixMax uint64
	// withoutLastPeer leaves out the last peer of the role, when the role
	// has more than one.
	withoutLastPeer bool
	// withProbePeer adds a peer the role does not have.
	withProbePeer bool
}

// revision is one revision of the configurations, the same for every device.
type revision struct {
	// age is how long ago the revision was written.
	age     time.Duration
	actor   string
	summary string
	shape   configShape
}

// The revision numbers. A revision's number is its position in revisions,
// from one.
const (
	revisionFirst   uint8 = 1
	revisionCurrent uint8 = 3
	revisionNewest  uint8 = 4
	// revisionLocal numbers the text the diverged device edited itself. It
	// is no row of revisions.
	revisionLocal uint8 = 255
)

var revisions = []revision{
	{
		age: 6 * day, actor: actorThomas, summary: "First configuration of the device",
		shape: configShape{holdTime: 180, prefixMax: 1000000, withoutLastPeer: true},
	},
	{
		age: 4 * day, actor: actorAmelie, summary: "Added the last peer",
		shape: configShape{holdTime: 180, prefixMax: 1000000},
	},
	{
		age: 2*day + 3*time.Hour, actor: actorThomas, summary: "Lowered the receive hold time to 90 seconds",
		shape: configShape{holdTime: 90, prefixMax: 1000000},
	},
	{
		age: rolloutEdgeAge, actor: actorThomas, summary: "Raised the IPv4 prefix maximum to 1200000",
		shape: configShape{holdTime: 90, prefixMax: 1200000},
	},
}

// localShape is the text the diverged device holds: the current revision
// with three local changes, a shorter hold time, a lower prefix maximum and
// one more peer.
var localShape = configShape{holdTime: 30, prefixMax: 500000, withProbePeer: true}

// probePeer is the peer the diverged device added locally.
var probePeer = peerRow{name: "lab-probe", address: peerAddr("203.0.113.77"), remoteAS: 64499}

// configStage is where one device's configuration stands against the hub's.
type configStage struct {
	state fleetui.ConfigState
	// hub is the revision the hub holds for the device, and device the one
	// the device runs. device means nothing while fetched is false.
	hub     uint8
	device  uint8
	fetched bool
	// local reports that the device runs its own edit of revision device.
	local bool
	// result is what the device answered to its last fetch.
	result string
}

// stageInSync is the stage of every device configStages does not name.
var stageInSync = configStage{
	state: fleetui.ConfigStateInSync, hub: revisionCurrent, device: revisionCurrent,
	fetched: true, result: resultApplied,
}

// configStages names the devices that do not run the current revision in
// sync: the two the rollout already updated, the one that refused it, two
// that have not applied what the hub holds, the one edited locally, and the
// newcomer that never fetched.
var configStages = map[string]configStage{
	deviceEdgeLon01: {
		state: fleetui.ConfigStateInSync, hub: revisionNewest, device: revisionNewest,
		fetched: true, result: resultApplied,
	},
	deviceEdgeLon02: {
		state: fleetui.ConfigStateInSync, hub: revisionNewest, device: revisionNewest,
		fetched: true, result: resultApplied,
	},
	deviceEdgeFra01: {
		state: fleetui.ConfigStateRejected, hub: revisionNewest, device: revisionCurrent,
		fetched: true, result: rejectionAnswer,
	},
	deviceEdgeFra02: {
		state: fleetui.ConfigStatePending, hub: revisionNewest, device: revisionCurrent,
		fetched: true, result: resultApplied,
	},
	deviceCpePar07: {
		state: fleetui.ConfigStatePending, hub: revisionNewest, device: revisionCurrent,
		fetched: true, result: resultApplied,
	},
	deviceRsFra01: {
		state: fleetui.ConfigStateDiverged, hub: revisionCurrent, device: revisionCurrent,
		fetched: true, local: true, result: resultKeptLocal,
	},
	newcomerName: {
		state: fleetui.ConfigStateNeverFetched, hub: revisionFirst,
	},
}

// stageOf answers where the device's configuration stands.
func stageOf(row *deviceRow) *configStage {
	if stage, named := configStages[row.name]; named {
		return &stage
	}

	return &stageInSync
}

// versionID answers the identifier of one revision of one device's
// configuration: four hexadecimal digits drawn from the name and the number.
func versionID(name string, number uint8) string {
	sum := hashBytes(hashText(hashText(hashOffset, historySeed), name), []byte{number})

	high := uint8(unit(sum) * 256)
	low := uint8(unit(sum^mixFirst) * 256)

	return textbuf.StringHex([]byte{high, low})
}

// deviceVersion answers the version the device runs, which is empty when it
// never fetched one.
func deviceVersion(row *deviceRow, stage *configStage) string {
	if !stage.fetched {
		return ""
	}
	if stage.local {
		return versionID(row.name, revisionLocal)
	}

	return versionID(row.name, stage.device)
}

// configFactsAt answers the configuration facts of a device at the snapshot.
func configFactsAt(row *deviceRow, snap *snapshot) fleetui.ConfigFacts {
	stage := stageOf(row)

	facts := fleetui.ConfigFacts{
		State:           stage.state,
		HubVersion:      versionID(row.name, stage.hub),
		DeviceVersion:   deviceVersion(row, stage),
		Template:        roleTable[row.role].template,
		Fetched:         stage.fetched,
		LastFetchResult: stage.result,
	}
	if !stage.fetched {
		return facts
	}

	facts.LastFetch = snap.ago(revisions[stage.device-1].age - fetchDelay)
	if stage.state == fleetui.ConfigStateRejected {
		facts.LastFetch = snap.ago(rejectedFor)
	}

	return facts
}

// blockSession is the block the three levels of a configuration share.
const blockSession = "session"

// The indentation of one level of a configuration text.
const configIndent = "    "

// configWriter writes a configuration text one statement at a time, keeping
// the depth of the open blocks.
type configWriter struct {
	text  *textbuf.Buffer
	depth int
}

// open starts a block.
func (writer *configWriter) open(name string) {
	writer.text.Repeat(configIndent, writer.depth).Str(name).Str(" {\n")
	writer.depth++
}

// openNamed starts a block that has a name, such as a group or a peer.
func (writer *configWriter) openNamed(kind, name string) {
	writer.text.Repeat(configIndent, writer.depth).Str(kind).Byte(' ').Str(name).Str(" {\n")
	writer.depth++
}

// leaf writes one value.
func (writer *configWriter) leaf(key, value string) {
	writer.text.Repeat(configIndent, writer.depth).Str(key).Byte(' ').Str(value).Str(";\n")
}

// end closes the innermost open blocks, count of them.
func (writer *configWriter) end(count int) {
	for range count {
		writer.depth--
		writer.text.Repeat(configIndent, writer.depth).Str("}\n")
	}
}

// shapedPeers answers the peers a text of the device's configuration
// declares.
func shapedPeers(row *deviceRow, shape *configShape) []peerRow {
	peers := slices.Clone(rolePeers[row.role])

	if shape.withoutLastPeer {
		if len(peers) > 1 {
			peers = peers[:len(peers)-1]
		}
	}
	if shape.withProbePeer {
		probe := probePeer
		probe.group = peers[len(peers)-1].group
		peers = append(peers, probe)
	}

	return peers
}

// writeGroupHead writes what a group sets for all its peers.
func writeGroupHead(writer *configWriter, group string, shape *configShape) {
	writer.openNamed("group", group)

	writer.open("timer")
	writer.leaf("receive-hold-time", textbuf.StringUint(shape.holdTime))
	writer.end(1)

	writer.open(blockSession)
	writer.open("family")
	writer.open(familyIPv4)
	writer.open("prefix")
	writer.leaf("maximum", textbuf.StringUint(shape.prefixMax))
	writer.end(4)
}

// writePeer writes one peer of a group.
func writePeer(writer *configWriter, peer *peerRow) {
	writer.openNamed("peer", peer.name)

	writer.open("connection")
	writer.open("remote")
	writer.leaf("ip", peer.address.String())
	writer.end(2)

	writer.open(blockSession)
	writer.open("asn")
	writer.leaf("remote", textbuf.StringUint32(peer.remoteAS))
	writer.end(3)
}

// configText answers one text of the device's configuration.
func configText(row *deviceRow, shape *configShape) string {
	writer := &configWriter{text: textbuf.New()}

	writer.open("bgp")
	writer.leaf("router-id", hostAddr(loopbackBase, row.host).String())

	writer.open(blockSession)
	writer.open("asn")
	writer.leaf("local", textbuf.StringUint32(roleTable[row.role].localAS))
	writer.end(2)

	peers := shapedPeers(row, shape)
	group := ""
	for at := range peers {
		if peers[at].group != group {
			if group != "" {
				writer.end(1)
			}
			group = peers[at].group
			writeGroupHead(writer, group, shape)
		}
		writePeer(writer, &peers[at])
	}

	// The last group, then the bgp block.
	writer.end(2)

	return writer.text.Slice()
}

// deviceConfig answers the configuration the hub holds for a device.
func deviceConfig(row *deviceRow) fleetui.DeviceConfig {
	stage := stageOf(row)

	return fleetui.DeviceConfig{
		Device:  row.name,
		Version: versionID(row.name, stage.hub),
		Text:    configText(row, &revisions[stage.hub-1].shape),
	}
}

// revisionResult answers what the device answered when it was sent revision
// number, and its words for a refusal.
func revisionResult(stage *configStage, number uint8) (fleetui.ConfigVersionResult, string) {
	if stage.fetched {
		if number <= stage.device {
			return fleetui.ConfigVersionResultApplied, ""
		}
	}
	if stage.state == fleetui.ConfigStateRejected {
		return fleetui.ConfigVersionResultRejected, rejectionAnswer
	}

	return fleetui.ConfigVersionResultNotFetched, ""
}

// configVersions answers the versions the hub has held for a device, the
// newest first.
func configVersions(row *deviceRow, snap *snapshot) []fleetui.ConfigVersion {
	stage := stageOf(row)

	versions := make([]fleetui.ConfigVersion, 0, stage.hub)
	for number := stage.hub; number >= revisionFirst; number-- {
		written := &revisions[number-1]
		result, answer := revisionResult(stage, number)
		versions = append(versions, fleetui.ConfigVersion{
			Version: versionID(row.name, number),
			At:      snap.ago(written.age),
			Actor:   written.actor,
			Summary: written.summary,
			Result:  result,
			Answer:  answer,
		})
	}

	return versions
}

// configTextOf answers the text of one version of a device's configuration:
// a revision the hub has held, or the diverged device's own edit.
func configTextOf(row *deviceRow, version string) (string, error) {
	stage := stageOf(row)

	for number := revisionFirst; number <= stage.hub; number++ {
		if versionID(row.name, number) == version {
			return configText(row, &revisions[number-1].shape), nil
		}
	}
	if stage.local {
		if versionID(row.name, revisionLocal) == version {
			return configText(row, &localShape), nil
		}
	}

	problem := textbuf.New().Str("The hub holds no version ").Quoted(version).
		Str(" of the configuration of ").Str(row.name).Str(".").Slice()

	return "", opererr.Wrap(fleetui.ErrObjectUnknown, problem,
		"The version is not one the hub has held for this device, so there is no text to show.",
		devicePage(row.name, tabConfig))
}

// divergenceOf answers both sides of a diverged device's configuration, or
// refuses a device that holds no local edit.
func divergenceOf(row *deviceRow, snap *snapshot) (fleetui.Divergence, error) {
	stage := stageOf(row)
	if !stage.local {
		problem := textbuf.New().Str(row.name).Str(" holds no local edit of its configuration.").Slice()

		return fleetui.Divergence{}, opererr.Wrap(fleetui.ErrObjectUnknown, problem,
			"The device runs a version the hub sent, so there is no difference to show.",
			devicePage(row.name, tabConfig))
	}

	return fleetui.Divergence{
		Device:        row.name,
		HubVersion:    versionID(row.name, stage.hub),
		HubText:       configText(row, &revisions[stage.hub-1].shape),
		DeviceVersion: versionID(row.name, revisionLocal),
		DeviceText:    configText(row, &localShape),
		Changes:       localChanges,
		EditedAt:      snap.ago(divergedFor),
	}, nil
}

// templateRow is one configuration template of the sample hub.
type templateRow struct {
	name   string
	groups []string
	// changed is how long ago the template was last changed.
	changed time.Duration
	actor   string
}

var templateRows = []templateRow{
	{name: templateEdge, groups: []string{groupEdge}, changed: rolloutEdgeAge, actor: actorThomas},
	{name: templateCPE, groups: []string{groupCPE}, changed: 2*day + 3*time.Hour, actor: actorAmelie},
}

// templatesAt answers the templates at the snapshot. A template's device
// count is the number of devices whose role takes its configuration from it.
func templatesAt(snap *snapshot) []fleetui.Template {
	templates := make([]fleetui.Template, 0, len(templateRows))
	for at := range templateRows {
		row := &templateRows[at]

		devices := 0
		for index := range deviceRows {
			if roleTable[deviceRows[index].role].template == row.name {
				devices++
			}
		}

		templates = append(templates, fleetui.Template{
			Name:    row.name,
			Groups:  slices.Clone(row.groups),
			Devices: devices,
			Changed: snap.ago(row.changed),
			Actor:   row.actor,
		})
	}

	return templates
}

// accessProfiles answers what each profile of the sample hub may do.
func accessProfiles() []fleetui.AccessProfile {
	return []fleetui.AccessProfile{
		{Name: "admin", FleetMode: true, Console: true, Diagnostics: true, ConfigPush: true, Rollouts: true},
		{Name: relayProfile, FleetMode: true, Console: true, Diagnostics: true, ConfigPush: true},
		{Name: "read-only", FleetMode: true},
	}
}

// The settings of the sample hub. The offline alarm opens five minutes after
// a device was last seen, so each of the three offline devices has one open.
const (
	offlineAlarmDelay    = 5 * time.Minute
	resourceAlarmPercent = 95
	resourceAlarmDelay   = 5 * time.Minute
)

// settings answers the fleet settings of the sample hub.
func settings() fleetui.Settings {
	return fleetui.Settings{
		OfflineAlarmDelay:    offlineAlarmDelay,
		ResourceAlarmPercent: resourceAlarmPercent,
		ResourceAlarmDelay:   resourceAlarmDelay,
		FinishedTasksKept:    finishedTasksKept,
		EventStreamsMax:      fleetui.EventSubscriptionsMax,
	}
}
