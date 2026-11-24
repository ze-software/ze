// Design: docs/architecture/api/architecture.md -- parsed bus direction and message filter AST
// Related: bus_path.go -- short path validation and segment prefix matching
// Related: ../config/bus_validate.go -- commit-time validation over BusBlock values

package plugin

import (
	"maps"
	"slices"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// Config map keys for the direction-oriented participant-bus YANG grouping.
const (
	busKeyReceive = "receive"
	busKeySend    = "send"
	busKeyAllow   = "allow"
	busKeyFrom    = "from"
	busKeyTo      = "to"
	busKeyMessage = "message"
	busKeyTag     = "tag"
	busKeyValue   = "value"
)

// BusMessageRule holds per-message target and tag overrides within one bus
// direction (send or receive).
type BusMessageRule struct {
	TargetsPresent bool                // True when to (send) or from (receive) was written.
	TargetsNone    bool                // True when to/from was written as "none".
	Targets        []string            // Target paths when TargetsPresent && !TargetsNone.
	Tags           map[string][]string // Tag name -> allowed tag values.
}

// IsEmpty reports whether r carries neither a target override nor any tag rule.
func (r *BusMessageRule) IsEmpty() bool {
	if r == nil {
		return true
	}
	if r.TargetsPresent {
		return false
	}
	return len(r.Tags) == 0
}

// Clone returns a deep copy of r.
func (r *BusMessageRule) Clone() *BusMessageRule {
	if r == nil {
		return nil
	}
	out := &BusMessageRule{
		TargetsPresent: r.TargetsPresent,
		TargetsNone:    r.TargetsNone,
		Targets:        slices.Clone(r.Targets),
	}
	if len(r.Tags) > 0 {
		out.Tags = make(map[string][]string, len(r.Tags))
		for k, v := range r.Tags {
			out.Tags[k] = slices.Clone(v)
		}
	}
	return out
}

// BusDirectionBlock holds one direction (receive or send) of a participant's
// bus filter configuration.
type BusDirectionBlock struct {
	AllowPresent   bool                       // True when allow was written.
	AllowNone      bool                       // True when allow was written as "none".
	Allow          []string                   // Allowed message names when AllowPresent && !AllowNone.
	TargetsPresent bool                       // True when block-level to/from was written.
	TargetsNone    bool                       // True when block-level to/from was written as "none".
	Targets        []string                   // Block-level target paths when TargetsPresent && !TargetsNone.
	Messages       map[string]*BusMessageRule // Per-message overrides keyed by message name.
}

// IsEmpty reports whether d specifies no allow-list, target list, or message
// rule.
func (d BusDirectionBlock) IsEmpty() bool {
	if d.AllowPresent {
		return false
	}
	if d.TargetsPresent {
		return false
	}
	return len(d.Messages) == 0
}

// Clone returns a deep copy of d.
func (d BusDirectionBlock) Clone() BusDirectionBlock {
	out := BusDirectionBlock{
		AllowPresent:   d.AllowPresent,
		AllowNone:      d.AllowNone,
		Allow:          slices.Clone(d.Allow),
		TargetsPresent: d.TargetsPresent,
		TargetsNone:    d.TargetsNone,
		Targets:        slices.Clone(d.Targets),
	}
	if len(d.Messages) > 0 {
		out.Messages = make(map[string]*BusMessageRule, len(d.Messages))
		for k, v := range d.Messages {
			out.Messages[k] = v.Clone()
		}
	}
	return out
}

// BusBlock holds the parsed receive and send filter blocks for one participant
// or override location.
type BusBlock struct {
	Receive BusDirectionBlock
	Send    BusDirectionBlock
}

// IsEmpty reports whether both Receive and Send are empty.
func (b BusBlock) IsEmpty() bool {
	return b.Receive.IsEmpty() && b.Send.IsEmpty()
}

// Clone returns a deep copy of b.
func (b BusBlock) Clone() BusBlock {
	return BusBlock{
		Receive: b.Receive.Clone(),
		Send:    b.Send.Clone(),
	}
}

// BusParticipantOverride associates one parsed BusBlock with the participant
// path it targets and the human-readable config location that wrote it.
type BusParticipantOverride struct {
	Path   string
	Source string
	Block  BusBlock
}

// BusBlockEqual reports whether a and b represent the same bus filter rules,
// comparing allow-lists, target lists, and tag value lists as order-independent
// sets.
func BusBlockEqual(a, b BusBlock) bool {
	if !BusDirectionBlockEqual(a.Receive, b.Receive) {
		return false
	}
	return BusDirectionBlockEqual(a.Send, b.Send)
}

// BusDirectionBlockEqual reports whether a and b represent the same rules for
// one direction, comparing slices as order-independent sets.
func BusDirectionBlockEqual(a, b BusDirectionBlock) bool {
	if !BusAllowEqual(a, b) {
		return false
	}
	if !BusTargetsEqual(a, b) {
		return false
	}
	if len(a.Messages) != len(b.Messages) {
		return false
	}
	for msg, ra := range a.Messages {
		rb, ok := b.Messages[msg]
		if !ok {
			return false
		}
		if !BusMessageRuleEqual(ra, rb) {
			return false
		}
	}
	return true
}

// BusAllowEqual reports whether the allow fields of a and b match as sets.
func BusAllowEqual(a, b BusDirectionBlock) bool {
	if a.AllowPresent != b.AllowPresent {
		return false
	}
	if a.AllowNone != b.AllowNone {
		return false
	}
	return StringSetEqual(a.Allow, b.Allow)
}

// BusTargetsEqual reports whether the block-level target fields of a and b
// match as sets.
func BusTargetsEqual(a, b BusDirectionBlock) bool {
	if a.TargetsPresent != b.TargetsPresent {
		return false
	}
	if a.TargetsNone != b.TargetsNone {
		return false
	}
	return StringSetEqual(a.Targets, b.Targets)
}

// BusMessageRuleTargetsEqual reports whether the per-message target fields of a
// and b match as sets.
func BusMessageRuleTargetsEqual(a, b *BusMessageRule) bool {
	var ap, an, bp, bn bool
	var at, bt []string
	if a != nil {
		ap, an, at = a.TargetsPresent, a.TargetsNone, a.Targets
	}
	if b != nil {
		bp, bn, bt = b.TargetsPresent, b.TargetsNone, b.Targets
	}
	if ap != bp {
		return false
	}
	if an != bn {
		return false
	}
	return StringSetEqual(at, bt)
}

// BusMessageRuleEqual reports whether a and b represent the same per-message
// target and tag rules, comparing slices as order-independent sets.
func BusMessageRuleEqual(a, b *BusMessageRule) bool {
	if a.IsEmpty() && b.IsEmpty() {
		return true
	}
	if !BusMessageRuleTargetsEqual(a, b) {
		return false
	}
	var aTags, bTags map[string][]string
	if a != nil {
		aTags = a.Tags
	}
	if b != nil {
		bTags = b.Tags
	}
	if len(aTags) != len(bTags) {
		return false
	}
	for k, va := range aTags {
		vb, ok := bTags[k]
		if !ok {
			return false
		}
		if !StringSetEqual(va, vb) {
			return false
		}
	}
	return true
}

// StringSetEqual reports whether a and b contain the same unique strings,
// regardless of order or duplicates.
func StringSetEqual(a, b []string) bool {
	sa := uniqueSortedStrings(a)
	sb := uniqueSortedStrings(b)
	return slices.Equal(sa, sb)
}

func uniqueSortedStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := slices.Clone(in)
	slices.Sort(out)
	return slices.Compact(out)
}

// ParseBusBlockFromMap extracts a BusBlock from a config map containing
// optional "receive" and "send" submaps (as produced by Tree.ToMap or
// Tree.ToPluginMap). location names the enclosing block for operator errors.
// It enforces Q-1 exclusivity: combining "none" with any other value in allow,
// to, or from returns an *opererr.Error.
func ParseBusBlockFromMap(m map[string]any, location string) (BusBlock, error) {
	if len(m) == 0 {
		return BusBlock{}, nil
	}
	var block BusBlock
	if rawRecv, ok := m[busKeyReceive]; ok {
		recvMap, ok := rawRecv.(map[string]any)
		if !ok {
			return BusBlock{}, invalidBusContainerError(location, busKeyReceive)
		}
		recv, err := parseBusDirectionFromMap(recvMap, location+" > "+busKeyReceive, busKeyFrom)
		if err != nil {
			return BusBlock{}, err
		}
		block.Receive = recv
	}
	if rawSend, ok := m[busKeySend]; ok {
		sendMap, ok := rawSend.(map[string]any)
		if !ok {
			return BusBlock{}, invalidBusContainerError(location, busKeySend)
		}
		send, err := parseBusDirectionFromMap(sendMap, location+" > "+busKeySend, busKeyTo)
		if err != nil {
			return BusBlock{}, err
		}
		block.Send = send
	}
	return block, nil
}

func parseBusDirectionFromMap(m map[string]any, dirLoc, targetKey string) (BusDirectionBlock, error) {
	var dir BusDirectionBlock
	if rawAllow, ok := m[busKeyAllow]; ok {
		vals, err := stringSliceFromAny(rawAllow, dirLoc+" > "+busKeyAllow)
		if err != nil {
			return BusDirectionBlock{}, err
		}
		if len(vals) > 0 {
			isNone, clean, noneErr := parseNoneOrValues(vals, dirLoc+" > "+busKeyAllow)
			if noneErr != nil {
				return BusDirectionBlock{}, noneErr
			}
			dir.AllowPresent = true
			dir.AllowNone = isNone
			dir.Allow = clean
		}
	}
	if rawTargets, ok := m[targetKey]; ok {
		vals, err := stringSliceFromAny(rawTargets, dirLoc+" > "+targetKey)
		if err != nil {
			return BusDirectionBlock{}, err
		}
		if len(vals) > 0 {
			isNone, clean, noneErr := parseNoneOrValues(vals, dirLoc+" > "+targetKey)
			if noneErr != nil {
				return BusDirectionBlock{}, noneErr
			}
			dir.TargetsPresent = true
			dir.TargetsNone = isNone
			dir.Targets = clean
		}
	}
	if rawMsgs, ok := m[busKeyMessage]; ok {
		msgMap, ok := rawMsgs.(map[string]any)
		if !ok {
			return BusDirectionBlock{}, invalidBusContainerError(dirLoc, busKeyMessage)
		}
		if len(msgMap) > 0 {
			dir.Messages = make(map[string]*BusMessageRule, len(msgMap))
			for _, msgName := range slices.Sorted(maps.Keys(msgMap)) {
				rawEntry := msgMap[msgName]
				entryMap, ok := rawEntry.(map[string]any)
				if !ok {
					return BusDirectionBlock{}, invalidBusContainerError(dirLoc+" > "+busKeyMessage, msgName)
				}
				rule, err := parseBusMessageRuleFromMap(entryMap, dirLoc+" > "+busKeyMessage+" "+msgName, targetKey)
				if err != nil {
					return BusDirectionBlock{}, err
				}
				dir.Messages[msgName] = rule
			}
		}
	}
	return dir, nil
}

func parseBusMessageRuleFromMap(m map[string]any, msgLoc, targetKey string) (*BusMessageRule, error) {
	rule := &BusMessageRule{}
	if rawTargets, ok := m[targetKey]; ok {
		vals, err := stringSliceFromAny(rawTargets, msgLoc+" > "+targetKey)
		if err != nil {
			return nil, err
		}
		if len(vals) > 0 {
			isNone, clean, noneErr := parseNoneOrValues(vals, msgLoc+" > "+targetKey)
			if noneErr != nil {
				return nil, noneErr
			}
			rule.TargetsPresent = true
			rule.TargetsNone = isNone
			rule.Targets = clean
		}
	}
	if rawTags, ok := m[busKeyTag]; ok {
		tagMap, ok := rawTags.(map[string]any)
		if !ok {
			return nil, invalidBusContainerError(msgLoc, busKeyTag)
		}
		if len(tagMap) > 0 {
			rule.Tags = make(map[string][]string, len(tagMap))
			for _, tagName := range slices.Sorted(maps.Keys(tagMap)) {
				vals, err := parseTagEntryValues(tagMap[tagName], msgLoc+" > "+busKeyTag+" "+tagName)
				if err != nil {
					return nil, err
				}
				rule.Tags[tagName] = vals
			}
		}
	}
	return rule, nil
}

func parseTagEntryValues(raw any, tagLoc string) ([]string, error) {
	if entryMap, ok := raw.(map[string]any); ok {
		rawVal, hasVal := entryMap[busKeyValue]
		if !hasVal {
			var problem, fix textbuf.Buffer
			problem.Str(tagLoc).Str(": tag filter requires at least one value")
			fix.Str("add value [ <val> ]; inside ").Str(tagLoc).Str(" or remove the empty tag block")
			return nil, opererr.New(
				problem.String(),
				"a tag filter with no values cannot match any message",
				fix.String())
		}
		return stringSliceFromAny(rawVal, tagLoc+" > "+busKeyValue)
	}
	return stringSliceFromAny(raw, tagLoc)
}

func parseNoneOrValues(vals []string, loc string) (isNone bool, clean []string, err error) {
	hasNone := false
	others := make([]string, 0, len(vals))
	for _, v := range vals {
		if v == BusKeywordNone {
			hasNone = true
			continue
		}
		others = append(others, v)
	}
	if hasNone && len(others) > 0 {
		var problem, fix textbuf.Buffer
		problem.Str(loc).Str(": cannot combine ").Quoted(BusKeywordNone).
			Str(" with other entries [ ").Join(vals, " ").Str(" ]")
		fix.Str("write either ").Str(loc).Str(" [ ").Str(BusKeywordNone).
			Str(" ] alone, or list explicit entries without ").Str(BusKeywordNone)
		return false, nil, opererr.New(
			problem.String(),
			"the keyword none explicitly empties a bus list and cannot be combined with explicit entries",
			fix.String())
	}
	if hasNone {
		return true, nil, nil
	}
	return false, deduplicatePreserveOrder(others), nil
}

func deduplicatePreserveOrder(vals []string) []string {
	seen := make(map[string]bool, len(vals))
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

func stringSliceFromAny(raw any, loc string) ([]string, error) {
	switch v := raw.(type) {
	case string:
		if v == "" {
			return nil, nil
		}
		return []string{v}, nil
	case []string:
		return slices.Clone(v), nil
	case []any:
		out := make([]string, 0, len(v))
		for _, elem := range v {
			s, ok := elem.(string)
			if !ok {
				var tb textbuf.Buffer
				return nil, opererr.New(
					tb.Str(loc).Str(": expected string list entry").String(),
					"bus filter list entries must be strings",
					"write string entries in "+loc)
			}
			if s != "" {
				out = append(out, s)
			}
		}
		return out, nil
	default:
		var tb textbuf.Buffer
		return nil, opererr.New(
			tb.Str(loc).Str(": expected string or list of strings").String(),
			"bus filter list entries must be strings",
			"write string entries in "+loc)
	}
}

func invalidBusContainerError(loc, child string) *opererr.Error {
	var tb textbuf.Buffer
	return opererr.New(
		tb.Str(loc).Str(" > ").Str(child).Str(": expected a configuration block").String(),
		"this bus section must be a container block",
		"write "+child+" { ... } under "+loc)
}
