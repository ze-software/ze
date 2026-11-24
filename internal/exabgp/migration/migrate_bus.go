// Design: docs/architecture/exabgp-bridge.md -- bus endpoint emission for migrated ExaBGP processes
// Overview: migrate.go -- ExaBGP migration orchestration
// Related: migrate_process_name.go -- endpoint name sanitization
// Related: migrate_serialize.go -- bus container serialization

package migration

import (
	"maps"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/exabgp/bridge"
)

const (
	busContainerKey         = "bus"
	busEndpointListKey      = "endpoint"
	busInternalLeafKey      = "internal"
	busExternalContainerKey = "external"
	busRunLeafKey           = "run"
	busRespawnLeafKey       = "respawn"
	busOnExitLeafKey        = "on-exit"
	busReceiveContainerKey  = "receive"
	busSendContainerKey     = "send"
	busAllowLeafKey         = "allow"
	busFromLeafKey          = "from"
	busToLeafKey            = "to"
	busMessageListKey       = "message"
	busTagListKey           = "tag"
	busValueLeafKey         = "value"
	busDirectionTagName     = "direction"
	busDirectionReceived    = "received"
	busDirectionSent        = "sent"
	busKeywordNone          = "none"
	onExitKeep              = "keep"
	onExitWithdraw          = "withdraw"
	addPathModeBoth         = "both"
	addPathModeSend         = "send"
	addPathModeReceive      = "receive"
)

// migratedProcess describes one ExaBGP process block ready to become an
// external bus endpoint running the `ze exabgp plugin` wrapper.
type migratedProcess struct {
	OrigName     string
	EndpointName string
	RunCmd       string
	Encoder      bridge.Encoder
	Respawn      bool
	OnExit       string
}

// neighborCaps holds the virtual session capabilities declared by one ExaBGP
// neighbor that feeds a process (E-4, E-7).
type neighborCaps struct {
	Families     []string
	RouteRefresh bool
	AddPathMode  string
}

// busMessageGrant maps one ExaBGP API event key to a canonical Ze bus message
// and optional direction tag value ("received" or "sent").
type busMessageGrant struct {
	Message   string
	Direction string
}

// scriptPeerFeed accumulates the deduplicated ExaBGP API events one neighbor
// grants to one process.
type scriptPeerFeed struct {
	PeerPath string
	PeerAddr string
	Events   []string
}

// scriptEndpointState accumulates neighbor bindings, event grants, and virtual
// session capabilities for one ExaBGP process while neighbors are migrated.
type scriptEndpointState struct {
	feeds        []scriptPeerFeed
	feedIndex    map[string]int
	unscoped     bool
	families     map[string]bool
	routeRefresh bool
	addPathMode  string
	addPathPeer  string
}

// scriptEndpointSet collects per-process scope and virtual session capabilities
// across all migrated neighbors, then writes each process's external endpoint
// under `bus { endpoint <name> { ... } }`.
type scriptEndpointSet struct {
	processes  []migratedProcess
	byOrigName map[string]int
	states     []scriptEndpointState
}

func newScriptEndpointSet(processes []migratedProcess) *scriptEndpointSet {
	byOrig := make(map[string]int, len(processes))
	states := make([]scriptEndpointState, len(processes))
	for i, proc := range processes {
		byOrig[proc.OrigName] = i
		states[i] = scriptEndpointState{
			feedIndex: make(map[string]int),
			families:  make(map[string]bool),
		}
	}
	return &scriptEndpointSet{
		processes:  processes,
		byOrigName: byOrig,
		states:     states,
	}
}

// record merges one neighbor's grants and virtual session capabilities into
// each referenced process's state. Returns an error when two neighbors naming
// the same process state conflicting non-empty ADD-PATH modes (AC-43).
func (s *scriptEndpointSet) record(peerPath, peerAddr string, caps neighborCaps, grants []processGrant) error {
	for _, grant := range grants {
		idx, ok := s.byOrigName[grant.Name]
		if !ok {
			panic("BUG: migration: process grant names an undeclared process: " + grant.Name)
		}
		st := &s.states[idx]
		proc := &s.processes[idx]

		for _, fam := range caps.Families {
			st.families[fam] = true
		}
		if caps.RouteRefresh {
			st.routeRefresh = true
		}
		if caps.AddPathMode != "" {
			if st.addPathMode == "" {
				st.addPathMode = caps.AddPathMode
				st.addPathPeer = peerAddr
			} else if st.addPathMode != caps.AddPathMode {
				var problem, fix textbuf.Buffer
				problem.Str("process ").Str(proc.OrigName).
					Str(": conflicting ADD-PATH modes across neighbors (").
					Str(st.addPathPeer).Str(" states ").Str(st.addPathMode).Str(", ").
					Str(peerAddr).Str(" states ").Str(caps.AddPathMode).Byte(')')
				fix.Str("align the add-path direction across neighbors naming ").Str(proc.OrigName).
					Str(", or declare separate process blocks per ADD-PATH mode")
				return opererr.New(
					problem.String(),
					"one ExaBGP script endpoint declares a single --add-path mode on its run line",
					fix.String(),
				)
			}
		}

		if !grant.Selected {
			st.unscoped = true
			continue
		}

		if existingIdx, seen := st.feedIndex[peerPath]; seen {
			st.feeds[existingIdx].Events = mergeUniqueSorted(st.feeds[existingIdx].Events, grant.Events)
			continue
		}
		st.feedIndex[peerPath] = len(st.feeds)
		st.feeds = append(st.feeds, scriptPeerFeed{
			PeerPath: peerPath,
			PeerAddr: peerAddr,
			Events:   slices.Clone(grant.Events),
		})
	}
	return nil
}

// write emits every migrated ExaBGP process as an external endpoint under
// `bus > endpoint <name>`.
func (s *scriptEndpointSet) write(dst *config.Tree, apiVersion4 bool) {
	if len(s.processes) == 0 {
		return
	}
	bus := dst.GetOrCreateContainer(busContainerKey)
	for i := range s.processes {
		proc := &s.processes[i]
		st := &s.states[i]

		ep := config.NewTree()
		ext := config.NewTree()

		formSpecs := collectFormSpecs(st.feeds)
		ext.Set(busRunLeafKey, buildWrapperRunCommand(proc, st, formSpecs))

		if !proc.Respawn {
			ext.Set(busRespawnLeafKey, configFalse)
		}
		if onExit := resolveEndpointOnExit(proc.OnExit, apiVersion4); onExit != "" {
			ext.Set(busOnExitLeafKey, onExit)
		}
		ep.SetContainer(busExternalContainerKey, ext)

		if !st.unscoped && len(st.feeds) > 0 {
			populateEndpointBusFilters(ep, st.feeds)
		}
		bus.AddListEntry(busEndpointListKey, proc.EndpointName, ep)
	}
}

// ensureInternalEndpoint declares a compiled-in plugin once under
// `bus { endpoint <name> { internal <name>; } }`.
func ensureInternalEndpoint(dst *config.Tree, name string) {
	bus := dst.GetOrCreateContainer(busContainerKey)
	if bus.GetList(busEndpointListKey)[name] != nil {
		return
	}
	ep := config.NewTree()
	ep.Set(busInternalLeafKey, name)
	bus.AddListEntry(busEndpointListKey, name, ep)
}

// isExaBGPAPIVersion4 reports whether the migration input or environment pins
// the ExaBGP API version to 4 (where an unset process on-exit keeps routes).
func isExaBGPAPIVersion4(tree *config.Tree) bool {
	if tree != nil {
		if envTree := tree.GetContainer("environment"); envTree != nil {
			if exaTree := envTree.GetContainer("exabgp"); exaTree != nil {
				if apiTree := exaTree.GetContainer("api"); apiTree != nil {
					if raw, ok := apiTree.Get("version"); ok && raw != "" {
						v, err := bridge.ParseAPIVersion(strings.Trim(raw, `"';`))
						if err == nil {
							return v == bridge.APIV4
						}
					}
				}
			}
		}
	}
	v, err := bridge.APIVersionFromEnv()
	return err == nil && v == bridge.APIV4
}

// resolveEndpointOnExit returns the `external > on-exit` value to emit, or ""
// when the endpoint's default (`withdraw`) applies (E-2, E-6, Q-E11).
func resolveEndpointOnExit(onExit string, apiVersion4 bool) string {
	switch onExit {
	case onExitKeep:
		return onExitKeep
	case onExitWithdraw:
		return ""
	case "":
		if apiVersion4 {
			return onExitKeep
		}
		return ""
	default:
		// Carry unrecognized words so Ze refuses them by name at config load.
		return onExit
	}
}

// buildWrapperRunCommand builds the `external > run` shell line invoking
// `ze exabgp plugin` with deterministic options followed by the original script
// command line. Never emits --respawn or --on-exit (E-6, Q-E11).
func buildWrapperRunCommand(proc *migratedProcess, st *scriptEndpointState, formSpecs []string) string {
	var tb textbuf.Buffer
	tb.Str("ze exabgp plugin")

	for _, fam := range slices.Sorted(maps.Keys(st.families)) {
		tb.Str(" --family ").Str(fam)
	}
	if st.routeRefresh {
		tb.Str(" --route-refresh")
	}
	if st.addPathMode != "" {
		tb.Str(" --add-path ").Str(st.addPathMode)
	}
	if proc.Encoder == bridge.EncoderJSON {
		tb.Str(" --encoder json")
	}
	for _, spec := range formSpecs {
		tb.Str(" --form ").Str(spec)
	}
	tb.Byte(' ').Str(proc.RunCmd)
	return tb.String()
}

// collectFormSpecs returns the sorted `--form` flag arguments for one endpoint.
// When every neighbor naming the process agrees on the same form tokens (or
// there is only one neighbor), unqualified `<token>` values are returned. When
// neighbors disagree, each non-default neighbor contributes `<addr>:<token>`.
func collectFormSpecs(feeds []scriptPeerFeed) []string {
	if len(feeds) == 0 {
		return nil
	}
	perFeed := make([][]string, len(feeds))
	anyForm := false
	for i, feed := range feeds {
		var tokens []string
		for _, ev := range feed.Events {
			if isFormToken(ev) {
				tokens = append(tokens, ev)
			}
		}
		slices.Sort(tokens)
		tokens = slices.Compact(tokens)
		if len(tokens) > 0 {
			anyForm = true
		}
		perFeed[i] = tokens
	}
	if !anyForm {
		return nil
	}

	uniform := true
	for i := 1; i < len(perFeed); i++ {
		if !slices.Equal(perFeed[0], perFeed[i]) {
			uniform = false
			break
		}
	}
	if uniform {
		return perFeed[0]
	}

	var specs []string
	for i, tokens := range perFeed {
		for _, tok := range tokens {
			var tb textbuf.Buffer
			specs = append(specs, tb.Str(feeds[i].PeerAddr).Byte(':').Str(tok).String())
		}
	}
	slices.Sort(specs)
	return slices.Compact(specs)
}

// populateEndpointBusFilters writes the endpoint's `receive` and `send` bus
// filter blocks derived from the ExaBGP neighbor `api` blocks (Q-E4).
func populateEndpointBusFilters(ep *config.Tree, feeds []scriptPeerFeed) {
	sendPeers := make([]string, 0, len(feeds))
	receivePeers := make(map[string]bool)
	allMessages := make(map[string]bool)
	msgPeers := make(map[string]map[string]bool)
	msgDirs := make(map[string]map[string]bool)

	for _, feed := range feeds {
		sendPeers = append(sendPeers, feed.PeerPath)
		grantedAny := false
		for _, ev := range feed.Events {
			grants, ok := mapExaBGPEventToBus(ev)
			if !ok {
				continue
			}
			grantedAny = true
			for _, g := range grants {
				allMessages[g.Message] = true
				if msgPeers[g.Message] == nil {
					msgPeers[g.Message] = make(map[string]bool)
				}
				msgPeers[g.Message][feed.PeerPath] = true
				if g.Direction != "" {
					if msgDirs[g.Message] == nil {
						msgDirs[g.Message] = make(map[string]bool)
					}
					msgDirs[g.Message][g.Direction] = true
				}
			}
		}
		if grantedAny {
			receivePeers[feed.PeerPath] = true
		}
	}

	slices.Sort(sendPeers)
	sendPeers = slices.Compact(sendPeers)

	recv := config.NewTree()
	if len(allMessages) == 0 {
		recv.SetSlice(busFromLeafKey, []string{busKeywordNone})
	} else {
		sortedRecvPeers := slices.Sorted(maps.Keys(receivePeers))
		sortedAllow := slices.Sorted(maps.Keys(allMessages))
		recv.SetSlice(busFromLeafKey, sortedRecvPeers)
		recv.SetSlice(busAllowLeafKey, sortedAllow)

		for _, msg := range sortedAllow {
			peersForMsg := slices.Sorted(maps.Keys(msgPeers[msg]))
			needFrom := len(peersForMsg) < len(sortedRecvPeers)
			dirs := slices.Sorted(maps.Keys(msgDirs[msg]))
			needDirTag := len(dirs) == 1
			if !needFrom && !needDirTag {
				continue
			}
			msgTree := config.NewTree()
			if needFrom {
				msgTree.SetSlice(busFromLeafKey, peersForMsg)
			}
			if needDirTag {
				tagTree := config.NewTree()
				tagTree.SetSlice(busValueLeafKey, dirs)
				msgTree.AddListEntry(busTagListKey, busDirectionTagName, tagTree)
			}
			recv.AddListEntry(busMessageListKey, msg, msgTree)
		}
	}
	ep.SetContainer(busReceiveContainerKey, recv)

	send := config.NewTree()
	send.SetSlice(busToLeafKey, sendPeers)
	ep.SetContainer(busSendContainerKey, send)
}

// mapExaBGPEventToBus maps one ExaBGP API key (from apiBlockEvents) to its
// canonical Ze bus message name(s) and optional direction tag (Q-E4 table).
// Operational events (unsupported) and form words return (nil, false).
func mapExaBGPEventToBus(apiKey string) ([]busMessageGrant, bool) {
	switch apiKey {
	case "neighbor-changes":
		return []busMessageGrant{
			{Message: "bgp:state"},
			{Message: "bgp:connected"},
		}, true
	case "negotiated":
		return []busMessageGrant{{Message: "bgp:negotiated"}}, true
	case "fsm":
		return []busMessageGrant{{Message: "bgp:fsm"}}, true
	case "signal":
		return []busMessageGrant{{Message: "bgp:signal"}}, true
	case "receive-open":
		return []busMessageGrant{{Message: "bgp:open", Direction: busDirectionReceived}}, true
	case "send-open":
		return []busMessageGrant{{Message: "bgp:open", Direction: busDirectionSent}}, true
	case "receive-update":
		return []busMessageGrant{{Message: "bgp:update", Direction: busDirectionReceived}}, true
	case "send-update":
		return []busMessageGrant{{Message: "bgp:update", Direction: busDirectionSent}}, true
	case "receive-notification":
		return []busMessageGrant{{Message: "bgp:notification", Direction: busDirectionReceived}}, true
	case "send-notification":
		return []busMessageGrant{{Message: "bgp:notification", Direction: busDirectionSent}}, true
	case "receive-keepalive":
		return []busMessageGrant{{Message: "bgp:keepalive", Direction: busDirectionReceived}}, true
	case "send-keepalive":
		return []busMessageGrant{{Message: "bgp:keepalive", Direction: busDirectionSent}}, true
	case "receive-refresh":
		return []busMessageGrant{{Message: "bgp:refresh", Direction: busDirectionReceived}}, true
	case "send-refresh":
		return []busMessageGrant{{Message: "bgp:refresh", Direction: busDirectionSent}}, true
	default:
		// receive-operational / send-operational (unsupported) and form words.
		return nil, false
	}
}

// isFormToken reports whether apiKey is one of ExaBGP's message-rendering form
// words (`receive-packets`, `receive-consolidate`, `receive-parsed`, and their
// `send-` counterparts) carried via `--form` on the wrapper run line.
func isFormToken(apiKey string) bool {
	switch apiKey {
	case "receive-packets", "receive-consolidate", "receive-parsed",
		"send-packets", "send-consolidate", "send-parsed":
		return true
	default:
		return false
	}
}

// extractNeighborCaps reads the virtual session capabilities stated by one
// ExaBGP neighbor (family block, route-refresh, and ADD-PATH direction).
func extractNeighborCaps(src *config.Tree) neighborCaps {
	var caps neighborCaps

	if srcFam := src.GetContainer("family"); srcFam != nil {
		keys := srcFam.Values()
		if len(keys) > 0 && !slices.Contains(keys, exabgpFamilyAll) {
			caps.Families = make([]string, 0, len(keys))
			for _, k := range keys {
				caps.Families = append(caps.Families, convertFamilySyntax(k))
			}
			slices.Sort(caps.Families)
			caps.Families = slices.Compact(caps.Families)
		}
	}

	if srcCap := src.GetContainer("capability"); srcCap != nil {
		if v, ok := srcCap.GetFlex("route-refresh"); ok && exabgpBoolean(v) {
			caps.RouteRefresh = true
		}
	}

	caps.AddPathMode = neighborAddPathMode(src)
	return caps
}

// neighborAddPathMode extracts the effective ADD-PATH wrapper mode ("receive",
// "send", "both", or "") from an ExaBGP neighbor tree.
func neighborAddPathMode(src *config.Tree) string {
	var direction string
	if srcCap := src.GetContainer("capability"); srcCap != nil {
		if container := srcCap.GetContainer("add-path"); container != nil {
			s, _ := container.Get("send")
			r, _ := container.Get("receive")
			switch {
			case s == configTrue && r == configTrue:
				direction = dirSendReceive
			case s == configTrue:
				direction = addPathModeSend
			case r == configTrue:
				direction = addPathModeReceive
			}
		} else if v, ok := srcCap.GetFlex("add-path"); ok {
			switch strings.ToLower(strings.Trim(v, `"';`)) {
			case "", configTrue, "enable", dirSendReceive, addPathModeBoth:
				direction = dirSendReceive
			case configFalse, "disable", "no":
				direction = ""
			case addPathModeSend:
				direction = addPathModeSend
			case addPathModeReceive:
				direction = addPathModeReceive
			}
		}
	}
	if direction == "" && src.GetContainer("add-path") != nil {
		direction = dirSendReceive
	}
	switch direction {
	case dirSendReceive:
		return addPathModeBoth
	case addPathModeSend, addPathModeReceive:
		return direction
	default:
		return ""
	}
}

func mergeUniqueSorted(a, b []string) []string {
	merged := make([]string, 0, len(a)+len(b))
	merged = append(merged, a...)
	merged = append(merged, b...)
	slices.Sort(merged)
	return slices.Compact(merged)
}
