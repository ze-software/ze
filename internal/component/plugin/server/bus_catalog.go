// Design: docs/architecture/plugin/plugin-system.md -- Stage 1 bus declarations, Stage 2 catalog, Stage 5 receives
// Overview: startup.go -- 5-stage startup handshake driving onRegistration, deliverConfigRPC, onReady
// Related: stage1_checks.go -- Stage 1 check table validating participant-nodes, bus-sends, bus-receives
package server

import (
	"fmt"
	"slices"
	"strings"

	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// refuseBusDeclarationDisagreement verifies that a compiled-in plugin's Stage 1
// ParticipantNodes, Sends, and Receives match its compiled registry.Registration
// so a separate-process instance cannot drift from its compiled declaration.
func refuseBusDeclarationDisagreement(cfg plugin.PluginConfig, input *rpc.DeclareRegistrationInput) error {
	compiled := registryRow(cfg)
	if compiled == nil {
		return nil
	}
	return registry.CheckBusDeclarationsMatch(cfg.Name, compiled, input)
}

// registerDeclaredBusSends records a process's Stage 1 Sends in the server's
// bus message catalog and registers any newly declared non-cli external
// messages into the unified events registry so downstream receivers and Stage 2
// BusCatalog consumers see them ("Sends before receptions").
func (s *Server) registerDeclaredBusSends(pluginName string, sends []rpc.MessageSendDecl) error {
	if len(sends) == 0 {
		return nil
	}

	// Validate compatibility and register newly declared non-cli messages in
	// the events registry before storing into the server's catalog map.
	for i := range sends {
		send := &sends[i]
		ns, name, ok := events.SplitMessageName(send.Message)
		if !ok {
			return fmt.Errorf("plugin %q send %q is not a valid namespace:name", pluginName, send.Message)
		}
		if ns == events.NamespaceCLI {
			continue
		}
		if existing, exists := events.LookupMessage(send.Message); exists {
			if err := checkDeclaredSendTagsCompatible(pluginName, send.Message, existing, send.Tags); err != nil {
				return err
			}
			continue
		}
		def := events.MessageDef{
			Namespace:           ns,
			Name:                name,
			Tags:                rpcTagsToEventTags(send.Tags),
			DefaultDestinations: slices.Clone(send.DefaultDestinations),
		}
		if err := events.RegisterMessageDef(def); err != nil {
			return fmt.Errorf("plugin %q register send %q: %w", pluginName, send.Message, err)
		}
	}

	cloned := cloneMessageSendDecls(sends)
	s.busCatalogMu.Lock()
	if s.busDeclaredSends == nil {
		s.busDeclaredSends = make(map[string][]rpc.MessageSendDecl)
	}
	s.busDeclaredSends[pluginName] = cloned
	s.busCatalogMu.Unlock()
	return nil
}

// unregisterDeclaredBusSends removes a plugin's runtime-declared sends from the
// server's bus catalog when the process rolls back or stops.
func (s *Server) unregisterDeclaredBusSends(pluginName string) {
	s.busCatalogMu.Lock()
	delete(s.busDeclaredSends, pluginName)
	s.busCatalogMu.Unlock()
}

// checkDeclaredSendTagsCompatible checks that a sender referencing an already
// registered message does not declare tag values outside the registered closed
// set. A sender omitting tags inherits the registered message's tags.
func checkDeclaredSendTagsCompatible(pluginName, msg string, existing events.MessageDef, declared []rpc.MessageTagDecl) error {
	if len(declared) == 0 {
		return nil
	}
	if len(existing.Tags) == 0 {
		return nil
	}
	existingByName := make(map[string]events.TagDef, len(existing.Tags))
	for _, t := range existing.Tags {
		existingByName[t.Name] = t
	}
	for _, dt := range declared {
		et, ok := existingByName[dt.Name]
		if !ok {
			return fmt.Errorf("plugin %q send %q declares unknown tag %q", pluginName, msg, dt.Name)
		}
		if len(et.Values) == 0 {
			continue
		}
		for _, v := range dt.Values {
			if !slices.Contains(et.Values, v) {
				return fmt.Errorf("plugin %q send %q tag %q value %q is outside registered values",
					pluginName, msg, dt.Name, v)
			}
		}
	}
	return nil
}

// buildBusCatalog constructs the deterministic Stage 2 BusCatalog combining:
//  1. Every message in the unified events registry (compiled-in and external),
//  2. Every compiled-in plugin registration's Sends (sender kinds and tags),
//  3. Every runtime Stage 1 declared send recorded on this server.
//
// Safe for concurrent use.
func (s *Server) buildBusCatalog() []rpc.BusCatalogEntry {
	byMessage := make(map[string]*rpc.BusCatalogEntry)

	ensure := func(msg string) *rpc.BusCatalogEntry {
		entry, ok := byMessage[msg]
		if !ok {
			entry = &rpc.BusCatalogEntry{Message: msg}
			byMessage[msg] = entry
		}
		return entry
	}
	addSender := func(entry *rpc.BusCatalogEntry, kind string) {
		if kind == "" {
			return
		}
		if !slices.Contains(entry.SenderKinds, kind) {
			entry.SenderKinds = append(entry.SenderKinds, kind)
		}
	}

	for _, def := range events.AllMessages() {
		entry := ensure(def.FullName())
		if len(def.Tags) > 0 {
			entry.Tags = eventTagsToRPCTags(def.Tags)
		}
	}

	for _, reg := range registry.All() {
		for i := range reg.Sends {
			send := &reg.Sends[i]
			entry := ensure(send.Message)
			addSender(entry, reg.Name)
			if len(entry.Tags) == 0 && len(send.Tags) > 0 {
				entry.Tags = cloneMessageTagDecls(send.Tags)
			}
		}
	}

	s.busCatalogMu.Lock()
	for pluginName, sends := range s.busDeclaredSends {
		for i := range sends {
			send := &sends[i]
			entry := ensure(send.Message)
			addSender(entry, pluginName)
			if len(entry.Tags) == 0 && len(send.Tags) > 0 {
				entry.Tags = cloneMessageTagDecls(send.Tags)
			}
		}
	}
	s.busCatalogMu.Unlock()

	out := make([]rpc.BusCatalogEntry, 0, len(byMessage))
	for _, entry := range byMessage {
		slices.Sort(entry.SenderKinds)
		out = append(out, *entry)
	}
	slices.SortFunc(out, func(a, b rpc.BusCatalogEntry) int {
		return strings.Compare(a.Message, b.Message)
	})
	return out
}

// registerReceiveDeclarations activates event subscriptions for proc from its
// Stage 1 Receives declaration, applying wire delivery parameters (Format,
// Encoding, Envelope) from ready. Receptions whose sender namespace or message
// is absent from this build are valid and remain dormant without raising a
// subscription drop.
func (s *Server) registerReceiveDeclarations(proc *process.Process, receives []rpc.MessageReceiveDecl, ready *rpc.ReadyInput) {
	if ready != nil {
		if ready.Format != "" {
			proc.SetFormat(ready.Format)
		}
		if ready.Encoding != "" {
			proc.SetEncoding(ready.Encoding)
		}
		if ready.Envelope {
			proc.SetEnvelope(true)
		}
	}
	if len(receives) == 0 {
		return
	}

	for i := range receives {
		decl := &receives[i]
		ns, eventName, ok := events.SplitMessageName(decl.Message)
		if !ok {
			continue
		}
		if !events.IsValidNamespace(ns) {
			// Dormant reception: sender namespace is not present in this build
			// or not configured yet (spec M-4).
			continue
		}
		nsID := events.LookupNamespaceID(ns)
		etID := events.LookupEventTypeID(eventName)
		if etID == events.EventTypeUnknown {
			// Dormant reception: event type not yet registered by any sender.
			continue
		}
		dir := directionFromTags(decl.Tags)
		if len(decl.From) == 0 {
			s.subscriptions.Add(proc, &Subscription{
				Namespace: nsID,
				EventType: etID,
				Direction: dir,
			})
			continue
		}
		for _, from := range decl.From {
			s.subscriptions.Add(proc, &Subscription{
				Namespace:  nsID,
				EventType:  etID,
				Direction:  dir,
				PeerFilter: senderFilterFromPath(from),
			})
		}
	}

	s.reconcileDelivery(proc)
}

// senderFilterFromPath extracts the leaf selector from a Stage 1 Receive From
// path until Phase 5 replaces Subscription.PeerFilter with compiled bus filters.
func senderFilterFromPath(path string) *PeerFilter {
	if path == "" {
		return nil
	}
	if path == "*" {
		return nil
	}
	if idx := strings.LastIndexByte(path, '/'); idx >= 0 && idx+1 < len(path) {
		return &PeerFilter{Selector: path[idx+1:]}
	}
	return &PeerFilter{Selector: path}
}

// directionFromTags maps a "direction" tag filter on MessageReceiveDecl onto
// the hot-path events.Direction enum. Absent or empty means events.DirBoth.
func directionFromTags(tags []rpc.MessageTagDecl) events.Direction {
	for i := range tags {
		if tags[i].Name != events.TagDirection {
			continue
		}
		vals := tags[i].Values
		if len(vals) == 0 {
			return events.DirBoth
		}
		var recv, sent bool
		for _, v := range vals {
			switch v {
			case events.DirectionReceived:
				recv = true
			case events.DirectionSent:
				sent = true
			case events.DirectionBoth:
				recv = true
				sent = true
			}
		}
		if recv && sent {
			return events.DirBoth
		}
		if recv {
			return events.DirReceived
		}
		if sent {
			return events.DirSent
		}
		return events.DirBoth
	}
	return events.DirBoth
}

func rpcTagsToEventTags(tags []rpc.MessageTagDecl) []events.TagDef {
	if len(tags) == 0 {
		return nil
	}
	out := make([]events.TagDef, len(tags))
	for i := range tags {
		out[i] = events.TagDef{
			Name:   tags[i].Name,
			Values: slices.Clone(tags[i].Values),
		}
	}
	return out
}

func eventTagsToRPCTags(tags []events.TagDef) []rpc.MessageTagDecl {
	if len(tags) == 0 {
		return nil
	}
	out := make([]rpc.MessageTagDecl, len(tags))
	for i := range tags {
		out[i] = rpc.MessageTagDecl{
			Name:   tags[i].Name,
			Values: slices.Clone(tags[i].Values),
		}
	}
	return out
}

func cloneMessageTagDecls(tags []rpc.MessageTagDecl) []rpc.MessageTagDecl {
	if len(tags) == 0 {
		return nil
	}
	out := make([]rpc.MessageTagDecl, len(tags))
	for i := range tags {
		out[i] = rpc.MessageTagDecl{
			Name:   tags[i].Name,
			Values: slices.Clone(tags[i].Values),
		}
	}
	return out
}

func cloneMessageSendDecls(sends []rpc.MessageSendDecl) []rpc.MessageSendDecl {
	if len(sends) == 0 {
		return nil
	}
	out := make([]rpc.MessageSendDecl, len(sends))
	for i := range sends {
		out[i] = rpc.MessageSendDecl{
			Message:             sends[i].Message,
			Tags:                cloneMessageTagDecls(sends[i].Tags),
			DefaultDestinations: slices.Clone(sends[i].DefaultDestinations),
		}
	}
	return out
}

func cloneMessageReceiveDecls(receives []rpc.MessageReceiveDecl) []rpc.MessageReceiveDecl {
	if len(receives) == 0 {
		return nil
	}
	out := make([]rpc.MessageReceiveDecl, len(receives))
	for i := range receives {
		out[i] = rpc.MessageReceiveDecl{
			Message: receives[i].Message,
			From:    slices.Clone(receives[i].From),
			Tags:    cloneMessageTagDecls(receives[i].Tags),
		}
	}
	return out
}
