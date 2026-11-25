// Design: docs/architecture/api/architecture.md -- bus commit refusals 1-5 and 9-11
// Related: bus_validate.go -- ValidateBusConfig and participant discovery
// Related: register_bus.go -- registration seams and external declaration store

package config

import (
	"maps"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// validateParticipantBusBlock enforces Refusals 1-5 and 9-11 on one
// participant's merged BusBlock.
func validateParticipantBusBlock(
	tree *Tree,
	p *BusValidatedParticipant,
	participants map[string]*BusValidatedParticipant,
	templates []participantNodeTemplate,
) error {
	if err := validateDirectionBlock(tree, p, busDirectionSend, busTargetKeyTo, p.Block.Send, participants, templates); err != nil {
		return err
	}
	return validateDirectionBlock(tree, p, busDirectionReceive, busTargetKeyFrom, p.Block.Receive, participants, templates)
}

func validateDirectionBlock(
	tree *Tree,
	p *BusValidatedParticipant,
	dir, targetKey string,
	block plugin.BusDirectionBlock,
	participants map[string]*BusValidatedParticipant,
	templates []participantNodeTemplate,
) error {
	if block.IsEmpty() {
		return nil
	}
	dirLoc := p.Source + " > " + dir
	isSend := dir == busDirectionSend

	// Refusal 4: every message in allow or message <name> must be declared by
	// this participant's kind in this direction.
	if p.HasDeclaration {
		if block.AllowPresent && !block.AllowNone {
			for _, msg := range block.Allow {
				if !participantDeclaresMessage(p, isSend, msg) {
					return refusal4UndeclaredMessage(dirLoc+" > allow", msg, p, isSend)
				}
			}
		}
		for _, msg := range slices.Sorted(maps.Keys(block.Messages)) {
			if !participantDeclaresMessage(p, isSend, msg) {
				return refusal4UndeclaredMessage(dirLoc+" > message "+msg, msg, p, isSend)
			}
		}
	}

	// Refusal 5: per-message to/from written for a message excluded by this
	// direction's allow list.
	if block.AllowPresent {
		for _, msg := range slices.Sorted(maps.Keys(block.Messages)) {
			rule := block.Messages[msg]
			if rule == nil {
				continue
			}
			if !rule.TargetsPresent {
				continue
			}
			excluded := block.AllowNone || !allowListAdmits(block.Allow, msg)
			if excluded {
				return refusal5ExcludedMessageTarget(dirLoc, targetKey, msg)
			}
		}
	}

	// Refusals 1, 2, 3 on block-level to/from.
	if block.TargetsPresent && !block.TargetsNone {
		effectiveMsgs := effectiveDirectionMessages(p, block, isSend)
		line := dirLoc + " > " + targetKey
		for _, target := range block.Targets {
			if err := validateTargetListEntry(tree, line, target, effectiveMsgs, isSend, participants, templates); err != nil {
				return err
			}
		}
	}

	// Refusals 1, 2, 3, 9, 10, 11 on per-message rules.
	for _, msg := range slices.Sorted(maps.Keys(block.Messages)) {
		rule := block.Messages[msg]
		if rule == nil {
			continue
		}
		msgLoc := dirLoc + " > message " + msg
		if rule.TargetsPresent && !rule.TargetsNone {
			line := msgLoc + " > " + targetKey
			for _, target := range rule.Targets {
				if err := validateTargetListEntry(tree, line, target, []string{msg}, isSend, participants, templates); err != nil {
					return err
				}
			}
		}
		if len(rule.Tags) > 0 {
			if err := validateMessageRuleTags(p, msgLoc, msg, isSend, rule.Tags); err != nil {
				return err
			}
		}
	}
	return nil
}

func refusal5ExcludedMessageTarget(dirLoc, targetKey, msg string) *opererr.Error {
	var problem, why, fix textbuf.Buffer
	allowLine := dirLoc + " > allow"
	msgLine := dirLoc + " > message " + msg + " > " + targetKey
	problem.Str(msgLine).Str(" configures ").Str(targetKey).Str(" for ").Quoted(msg).
		Str(", which ").Str(allowLine).Str(" excludes")
	why.Str("a per-message ").Str(targetKey).Str(" rule cannot apply to a message excluded by the same direction's allow list")
	fix.Str("add ").Str(msg).Str(" to ").Str(allowLine).Str(" or remove ").Str(msgLine)
	return opererr.New(problem.String(), why.String(), fix.String())
}

// validateTargetListEntry validates one target path in send > to or receive >
// from against Refusal 1 (existence) and Refusals 2/3 (receiver/sender match).
func validateTargetListEntry(
	tree *Tree,
	line, target string,
	messages []string,
	isSend bool,
	participants map[string]*BusValidatedParticipant,
	templates []participantNodeTemplate,
) error {
	if err := plugin.ValidateBusPathSyntax(target); err != nil {
		return opererr.Wrap(err,
			line+": invalid bus path "+target,
			"bus targets must use short slash form",
			"write a valid participant path in "+line)
	}
	resolvedPaths, _, err := resolveTargetPath(tree, target, line, participants, templates)
	if err != nil {
		return err
	}
	if len(messages) == 0 {
		return nil
	}
	// CLI send messages (cli:<command>) address the CLI/peer selector rather than
	// a bus participant declaring Receives: ["cli:..."], so Refusal 2 does not
	// apply to them (Refusal 1 existence check above still ran).
	if isSend {
		filtered := make([]string, 0, len(messages))
		for _, msg := range messages {
			if strings.HasPrefix(msg, events.NamespaceCLI+":") {
				continue
			}
			filtered = append(filtered, msg)
		}
		messages = filtered
		if len(messages) == 0 {
			return nil
		}
	}

	allDeclared := true
	matchedAny := false
	for _, cp := range resolvedPaths {
		cand := participants[cp]
		if cand == nil {
			continue
		}
		if !cand.HasDeclaration {
			allDeclared = false
			continue
		}
		for _, msg := range messages {
			if isSend {
				if participantDeclaresMessage(cand, false, msg) {
					matchedAny = true
					break
				}
			} else {
				if participantDeclaresMessage(cand, true, msg) {
					matchedAny = true
					break
				}
			}
		}
		if matchedAny {
			break
		}
	}
	if matchedAny {
		return nil
	}
	if !allDeclared {
		return nil
	}

	if isSend {
		return refusal2NoReceiver(line, target, messages, participants)
	}
	return refusal3NoSender(line, target, messages, participants)
}

// resolveTargetPath checks whether target names a configured participant, a
// prefix covering at least one configured participant, or a valid secondary
// lookup path (Refusal 1 and Refusal 8).
func resolveTargetPath(
	tree *Tree,
	target, line string,
	participants map[string]*BusValidatedParticipant,
	templates []participantNodeTemplate,
) (canonicalPaths []string, dormant bool, err error) {
	if resolver, ok := matchingSecondaryResolver(target); ok {
		paths, isDormant, validDesc, resolveErr := resolver.Resolve(tree, target)
		if resolveErr != nil {
			var problem, why, fix textbuf.Buffer
			problem.Str(line).Str(": unknown bus target ").Quoted(target)
			why.Str(resolveErr.Error())
			if validDesc != "" && !strings.Contains(resolveErr.Error(), validDesc) {
				why.Str(" (valid: ").Str(validDesc).Byte(')')
			}
			fix.Str("change ").Str(line).Str(" to name a configured participant or accepted range, or remove ").Str(target)
			return nil, false, opererr.New(problem.String(), why.String(), fix.String())
		}
		return paths, isDormant, nil
	}

	var matched []string
	for _, pPath := range slices.Sorted(maps.Keys(participants)) {
		if plugin.PathCovers(target, pPath) {
			matched = append(matched, pPath)
		}
	}
	if len(matched) > 0 {
		return matched, false, nil
	}

	nodePaths := registeredShortNodePaths(templates)
	configured := slices.Sorted(maps.Keys(participants))
	var problem, why, fix textbuf.Buffer
	problem.Str(line).Str(": ").Quoted(target).Str(" names no configured bus participant")
	why.Str("registered participant nodes: [ ").Join(nodePaths, ", ").Str(" ]")
	if len(configured) > 0 {
		why.Str("; configured participants: [ ").Join(configured, ", ").Str(" ]")
	} else {
		why.Str("; no bus participants are configured")
	}
	fix.Str("update ").Str(line).Str(" to reference a configured participant path or remove ").Str(target)
	return nil, false, opererr.New(problem.String(), why.String(), fix.String())
}

func registeredShortNodePaths(templates []participantNodeTemplate) []string {
	seen := map[string]bool{busEndpointPrefixHint: true}
	out := []string{busEndpointPrefixHint}
	for _, tmpl := range templates {
		if seen[tmpl.shortPath] {
			continue
		}
		seen[tmpl.shortPath] = true
		out = append(out, tmpl.shortPath)
	}
	slices.Sort(out)
	return out
}

func refusal2NoReceiver(line, target string, messages []string, participants map[string]*BusValidatedParticipant) *opererr.Error {
	receivers := collectParticipantsHandling(messages, false, participants)
	var problem, why, fix textbuf.Buffer
	problem.Str(line).Str(": no participant under ").Quoted(target).Str(" registered to receive ").Str(formatMessageList(messages))
	if len(receivers) > 0 {
		why.Str("participants registered to receive ").Str(formatMessageList(messages)).Str(": [ ").Join(receivers, ", ").Str(" ]")
		fix.Str("set ").Str(line).Str(" to a receiver such as ").Str(receivers[0]).Str(" or remove ").Str(target)
	} else {
		why.Str("no configured participant registered to receive ").Str(formatMessageList(messages))
		fix.Str("remove ").Str(target).Str(" from ").Str(line).Str(" or configure a participant that receives ").Str(formatMessageList(messages))
	}
	return opererr.New(problem.String(), why.String(), fix.String())
}

func refusal3NoSender(line, target string, messages []string, participants map[string]*BusValidatedParticipant) *opererr.Error {
	senders := collectParticipantsHandling(messages, true, participants)
	var problem, why, fix textbuf.Buffer
	problem.Str(line).Str(": no participant under ").Quoted(target).Str(" declared sending ").Str(formatMessageList(messages))
	if len(senders) > 0 {
		why.Str("participants that declared sending ").Str(formatMessageList(messages)).Str(": [ ").Join(senders, ", ").Str(" ]")
		fix.Str("set ").Str(line).Str(" to a sender such as ").Str(senders[0]).Str(" or remove ").Str(target)
	} else {
		why.Str("no configured participant declared sending ").Str(formatMessageList(messages))
		fix.Str("remove ").Str(target).Str(" from ").Str(line).Str(" or configure a participant that sends ").Str(formatMessageList(messages))
	}
	return opererr.New(problem.String(), why.String(), fix.String())
}

func refusal4UndeclaredMessage(line, msg string, p *BusValidatedParticipant, isSend bool) *opererr.Error {
	dirVerb := "receiving"
	var declared []string
	if isSend {
		dirVerb = "sending"
		for _, s := range p.Sends {
			declared = append(declared, s.Message)
		}
	} else {
		for _, r := range p.Receives {
			declared = append(declared, r.Message)
		}
	}
	slices.Sort(declared)
	declared = slices.Compact(declared)

	var problem, why, fix textbuf.Buffer
	problem.Str(line).Str(": participant ").Quoted(p.Path).Str(" (kind ").Quoted(p.Kind).Str(") did not declare ").Str(dirVerb).Byte(' ').Quoted(msg)
	if len(declared) > 0 {
		why.Str("kind ").Quoted(p.Kind).Str(" declared ").Str(dirVerb).Str(": [ ").Join(declared, ", ").Str(" ]")
		fix.Str("replace ").Str(msg).Str(" in ").Str(line).Str(" with a declared message such as ").Str(declared[0])
	} else {
		why.Str("kind ").Quoted(p.Kind).Str(" declared no messages in this direction")
		fix.Str("remove ").Str(msg).Str(" from ").Str(line)
	}
	return opererr.New(problem.String(), why.String(), fix.String())
}

// validateMessageRuleTags enforces Refusals 9, 10, and 11 for tag filters under
// one message rule.
func validateMessageRuleTags(p *BusValidatedParticipant, msgLoc, msg string, isSend bool, tags map[string][]string) error {
	declaredTags := lookupMessageTags(msg)
	for _, tagName := range slices.Sorted(maps.Keys(tags)) {
		vals := tags[tagName]
		tagLine := msgLoc + " > tag " + tagName

		tagDef, exists := declaredTags[tagName]
		if !exists {
			knownTags := slices.Sorted(maps.Keys(declaredTags))
			var problem, why, fix textbuf.Buffer
			problem.Str(tagLine).Str(": tag ").Quoted(tagName).Str(" is not declared on message ").Quoted(msg)
			if len(knownTags) > 0 {
				why.Str("declared tags for ").Quoted(msg).Str(": [ ").Join(knownTags, ", ").Str(" ]")
				fix.Str("use a declared tag such as ").Str(knownTags[0]).Str(" or remove ").Str(tagLine)
			} else {
				why.Str("message ").Quoted(msg).Str(" declares no filterable tags")
				fix.Str("remove ").Str(tagLine)
			}
			return opererr.New(problem.String(), why.String(), fix.String())
		}

		if len(tagDef.Values) > 0 {
			for _, v := range vals {
				if !slices.Contains(tagDef.Values, v) {
					var problem, why, fix textbuf.Buffer
					problem.Str(tagLine).Str(": value ").Quoted(v).Str(" is not valid for tag ").Quoted(tagName)
					why.Str("valid values for tag ").Quoted(tagName).Str(" on ").Quoted(msg).Str(": [ ").Join(tagDef.Values, ", ").Str(" ]")
					fix.Str("set ").Str(tagLine).Str(" value to one of [ ").Join(tagDef.Values, " ").Str(" ]")
					return opererr.New(problem.String(), why.String(), fix.String())
				}
			}
		}

		regVals := participantRegisteredTagValues(p, isSend, msg, tagName)
		if len(regVals) > 0 && len(vals) > 0 && !slicesIntersect(vals, regVals) {
			var problem, why, fix textbuf.Buffer
			problem.Str(tagLine).Str(": values ").Str(formatListValue(vals)).
				Str(" share no value with participant ").Quoted(p.Path).
				Str(" registered filter ").Str(formatListValue(regVals))
			why.Str("the participant registration restricts tag ").Quoted(tagName).Str(" on ").Quoted(msg).
				Str(" to ").Str(formatListValue(regVals)).Str(", so ").Str(formatListValue(vals)).Str(" admits nothing")
			fix.Str("write a value admitted by the registration (").Str(formatListValue(regVals)).Str(") in ").Str(tagLine).Str(" or remove the tag line")
			return opererr.New(problem.String(), why.String(), fix.String())
		}
	}
	return nil
}

// lookupMessageTags returns every tag declared for msg across the central
// events message registry and all compiled-in and external sender declarations.
func lookupMessageTags(msg string) map[string]events.TagDef {
	out := make(map[string]events.TagDef)
	mergeTag := func(name string, values []string) {
		existing, ok := out[name]
		if !ok {
			out[name] = events.TagDef{Name: name, Values: slices.Clone(values)}
			return
		}
		if len(existing.Values) == 0 {
			return
		}
		if len(values) == 0 {
			existing.Values = nil
			out[name] = existing
			return
		}
		for _, v := range values {
			if !slices.Contains(existing.Values, v) {
				existing.Values = append(existing.Values, v)
			}
		}
		out[name] = existing
	}

	if def, ok := events.LookupMessage(msg); ok {
		for _, t := range def.Tags {
			mergeTag(t.Name, t.Values)
		}
	}
	for _, reg := range registry.All() {
		for _, s := range reg.Sends {
			if s.Message == msg {
				for _, t := range s.Tags {
					mergeTag(t.Name, t.Values)
				}
			}
		}
	}
	for _, ext := range ExternalBusDeclarations() {
		for _, s := range ext.Sends {
			if s.Message == msg {
				for _, t := range s.Tags {
					mergeTag(t.Name, t.Values)
				}
			}
		}
	}
	return out
}

func participantRegisteredTagValues(p *BusValidatedParticipant, isSend bool, msg, tagName string) []string {
	if isSend {
		for _, s := range p.Sends {
			if s.Message != msg {
				continue
			}
			for _, t := range s.Tags {
				if t.Name == tagName {
					return t.Values
				}
			}
		}
		return nil
	}
	for _, r := range p.Receives {
		if r.Message != msg {
			continue
		}
		for _, t := range r.Tags {
			if t.Name == tagName {
				return t.Values
			}
		}
	}
	return nil
}

func participantDeclaresMessage(p *BusValidatedParticipant, isSend bool, msg string) bool {
	if isSend {
		for _, s := range p.Sends {
			if messageDeclMatches(s.Message, msg) {
				return true
			}
		}
		return false
	}
	for _, r := range p.Receives {
		if r.Message == msg {
			return true
		}
	}
	return false
}

func messageDeclMatches(declared, candidate string) bool {
	if declared == candidate {
		return true
	}
	if strings.HasPrefix(declared, events.NamespaceCLI+":") && strings.HasPrefix(candidate, declared+" ") {
		return true
	}
	return false
}

func allowListAdmits(allow []string, msg string) bool {
	for _, entry := range allow {
		if messageDeclMatches(entry, msg) {
			return true
		}
	}
	return false
}

func effectiveDirectionMessages(p *BusValidatedParticipant, block plugin.BusDirectionBlock, isSend bool) []string {
	if block.AllowPresent {
		if block.AllowNone {
			return nil
		}
		if !isSend {
			return slices.Clone(block.Allow)
		}
		var out []string
		for _, msg := range block.Allow {
			if strings.HasPrefix(msg, events.NamespaceCLI+":") {
				continue
			}
			out = append(out, msg)
		}
		return out
	}
	if !p.HasDeclaration {
		return nil
	}
	var out []string
	if isSend {
		for _, s := range p.Sends {
			if strings.HasPrefix(s.Message, events.NamespaceCLI+":") {
				continue
			}
			out = append(out, s.Message)
		}
		return out
	}
	for _, r := range p.Receives {
		out = append(out, r.Message)
	}
	return out
}

func collectParticipantsHandling(messages []string, isSend bool, participants map[string]*BusValidatedParticipant) []string {
	var out []string
	for _, path := range slices.Sorted(maps.Keys(participants)) {
		cand := participants[path]
		if cand == nil {
			continue
		}
		if !cand.HasDeclaration {
			continue
		}
		for _, msg := range messages {
			if participantDeclaresMessage(cand, isSend, msg) {
				out = append(out, path)
				break
			}
		}
	}
	return out
}

func slicesIntersect(a, b []string) bool {
	for _, va := range a {
		if slices.Contains(b, va) {
			return true
		}
	}
	return false
}

func formatAllowOrTargets(isNone bool, vals []string) string {
	if isNone {
		return "[ " + busLabelNone + " ]"
	}
	return formatListValue(vals)
}

func formatListValue(vals []string) string {
	sorted := slices.Clone(vals)
	slices.Sort(sorted)
	var tb textbuf.Buffer
	return tb.Str("[ ").Join(sorted, ", ").Str(" ]").String()
}

func formatMessageList(messages []string) string {
	if len(messages) == 1 {
		var tb textbuf.Buffer
		return tb.Quoted(messages[0]).String()
	}
	return formatListValue(messages)
}
