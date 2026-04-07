// Design: docs/architecture/core-design.md — text event parsing for route server
// Overview: server.go — route server plugin orchestration

package rs

import (
	"fmt"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/textparse"
)

// Event represents a parsed BGP event (from text format).
// Fields are extracted by parseTextState, parseTextOpen, parseTextRefresh.
type Event struct {
	Type     string    // Event type: "update", "state", "open", "refresh"
	MsgID    uint64    // Message ID (for cache-forward)
	PeerAddr string    // Peer address
	PeerASN  uint32    // Peer ASN
	State    string    // State for state events ("up", "down", "connected")
	Open     *OpenInfo // OPEN: decoded open data
	AFI      string    // Refresh: AFI
	SAFI     string    // Refresh: SAFI
}

// FamilyOperation represents a single add or del operation for a family.
// Text format: "nlri <family> add <prefix>..." or "nlri <family> del <prefix>...".
type FamilyOperation struct {
	Action string // "add" or "del"
	NLRIs  []any  // Prefix strings from text parsing
}

// OpenInfo contains OPEN message details.
type OpenInfo struct {
	ASN          uint32
	RouterID     string
	HoldTime     uint16
	Capabilities []CapabilityInfo
}

// capabilityPresent returns true if any capability in the list has the given name.
func capabilityPresent(caps []CapabilityInfo, name string) bool {
	for _, c := range caps {
		if c.Name == name {
			return true
		}
	}
	return false
}

// CapabilityInfo represents a single capability from the OPEN message.
type CapabilityInfo struct {
	Code  int
	Name  string
	Value string
}

// buildNLRIEntries splits collected tokens into individual NLRI strings.
// Accepts two formats:
//   - Comma: "prefix 10.0.0.0/24,10.0.1.0/24" — type keyword + comma-separated values.
//   - Keyword boundary: "prefix 10.0.0.0/24 prefix 10.0.1.0/24" — repeated type keyword.
func buildNLRIEntries(tokens []string) []any {
	if len(tokens) == 0 {
		return nil
	}

	// Check for comma in any token.
	for i, tok := range tokens {
		if !strings.Contains(tok, ",") {
			continue
		}
		// Prefix = all tokens before the comma token (e.g., "prefix").
		typePrefix := strings.Join(tokens[:i], " ")
		var nlris []any
		for part := range strings.SplitSeq(tok, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				if typePrefix != "" {
					nlris = append(nlris, typePrefix+" "+part)
				} else {
					nlris = append(nlris, part)
				}
			}
		}
		return nlris
	}

	// No commas — check for keyword boundary (repeated type keywords).
	if textparse.NLRITypeKeywords[tokens[0]] {
		var nlris []any
		var current []string
		for _, tok := range tokens {
			if tok == tokens[0] && len(current) > 0 {
				nlris = append(nlris, strings.Join(current, " "))
				current = nil
			}
			current = append(current, tok)
		}
		if len(current) > 0 {
			nlris = append(nlris, strings.Join(current, " "))
		}
		return nlris
	}

	// Single complex NLRI: join all tokens.
	return []any{strings.Join(tokens, " ")}
}

// quickParseTextEvent extracts event type, message ID, peer address, and raw text
// from a text-format event line. Returns the full text as payload for deferred parsing.
//
// Uniform header: "peer <addr> remote as <n> ..."
// State:   "peer <addr> remote as <n> state <state>"       → dispatch="state"
// Message: "peer <addr> remote as <n> <dir> <type> <id>"   → dispatch=<type>.
func quickParseTextEvent(text string) (string, uint64, string, string, error) {
	text = strings.TrimRight(text, "\n")
	s := textparse.NewScanner(text)

	// peer
	if tok, ok := s.Next(); !ok || tok != "peer" {
		return "", 0, "", "", fmt.Errorf("invalid text event: missing peer prefix")
	}
	// <addr>
	peerAddr, ok := s.Next()
	if !ok {
		return "", 0, "", "", fmt.Errorf("invalid text event: missing peer address")
	}
	// remote as <n>
	if tok, ok := s.Next(); !ok || tok != tokenRemote {
		return "", 0, "", "", fmt.Errorf("invalid text event: missing remote keyword")
	}
	if tok, ok := s.Next(); !ok || tok != tokenAS {
		return "", 0, "", "", fmt.Errorf("invalid text event: missing as keyword")
	}
	// <n> (ASN value — consumed but not returned; available from payload)
	if _, ok := s.Next(); !ok {
		return "", 0, "", "", fmt.Errorf("invalid text event: missing as value")
	}

	// Next token: either "state" or <direction>
	dispatchTok, ok := s.Next()
	if !ok {
		return "", 0, "", "", fmt.Errorf("invalid text event: missing dispatch token")
	}
	if dispatchTok == eventState {
		return eventState, 0, peerAddr, text, nil
	}

	// Message events: <direction> was consumed as dispatchTok, next is <type> <id>
	eventType, ok := s.Next()
	if !ok {
		return "", 0, "", "", fmt.Errorf("invalid text event: missing event type")
	}
	idStr, ok := s.Next()
	if !ok {
		return eventType, 0, peerAddr, text, nil
	}
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return eventType, 0, peerAddr, text, nil //nolint:nilerr // Non-numeric ID is valid for some event types
	}

	return eventType, id, peerAddr, text, nil
}

// parseTextUpdateFamilies extracts family names from a text UPDATE event.
// Scans for "nlri" keyword followed by an afi/safi token (the family).
// Returns a map of family → true for selectForwardTargets compatibility.
func parseTextUpdateFamilies(text string) map[string]bool {
	s := textparse.NewScanner(text)
	families := make(map[string]bool)
	for !s.Done() {
		tok, ok := s.Next()
		if !ok {
			break
		}
		if tok == textparse.KWNLRI {
			if fam, ok := s.Next(); ok {
				if strings.Contains(fam, "/") {
					families[fam] = true
				}
			}
		}
	}
	return families
}

// parseTextNLRIOps extracts family operations (add/del + NLRIs) from a text UPDATE.
// Used by processForward to populate the withdrawal map.
//
// Format: "peer <addr> remote as <n> <dir> update <id> <attrs> [next <nh>] nlri <fam> add|del <nlris> ..."
//
// Key-dispatch loop processes keywords sequentially, resolving aliases via textparse.ResolveAlias:
// - Attribute keywords (origin, path, pref, etc.): skip value(s)
// - "nlri": consume family, extract action (add/del) and collect NLRI tokens until next keyword.
func parseTextNLRIOps(text string) map[string][]FamilyOperation {
	result := make(map[string][]FamilyOperation)
	s := textparse.NewScanner(strings.TrimRight(text, "\n"))

	// Skip header: peer <addr> remote as <n> <dir> update <id>
	for i := 0; i < 8 && !s.Done(); i++ {
		s.Next()
	}

	// Key-dispatch loop — resolve aliases so both short (API) and long (config) forms work.
	for !s.Done() {
		raw, ok := s.Next()
		if !ok {
			break
		}
		tok := textparse.ResolveAlias(raw)

		switch tok {
		case textparse.KWNextHop:
			s.Next() // consume the address

		case textparse.KWNLRI:
			// Family: nlri <family> add|del
			fam, ok := s.Next()
			if !ok || !strings.Contains(fam, "/") {
				continue
			}

			// Optional path-id modifier
			next, ok := s.Peek()
			if !ok {
				continue
			}
			if textparse.ResolveAlias(next) == textparse.KWPathInformation {
				s.Next() // consume "info"/"path-information"
				s.Next() // consume the ID value
				if _, ok = s.Peek(); !ok {
					continue
				}
			}

			// Action: add or del
			action, ok := s.Next()
			if !ok || (action != actionAdd && action != actionDel) {
				continue
			}

			// Collect NLRI tokens until next top-level keyword or end.
			var nlriTokens []string
			for !s.Done() {
				next, ok := s.Peek()
				if !ok || textparse.IsTopLevelKeyword(next) {
					break
				}
				tok, _ := s.Next()
				nlriTokens = append(nlriTokens, tok)
			}

			// Build NLRI entries (handles comma and keyword-boundary formats).
			nlris := buildNLRIEntries(nlriTokens)

			if len(nlris) > 0 {
				result[fam] = append(result[fam],
					FamilyOperation{Action: action, NLRIs: nlris})
			}

		// Attribute keywords: consume their values.
		// Scalar attributes (one value token).
		case textparse.KWOrigin, textparse.KWMED, textparse.KWLocalPreference,
			textparse.KWAggregator, textparse.KWOriginatorID:
			s.Next()
		// Comma-list attributes (one comma-separated value token).
		case textparse.KWASPath, textparse.KWCommunity, textparse.KWLargeCommunity,
			textparse.KWExtendedCommunity, textparse.KWClusterList:
			s.Next()
		case textparse.KWAtomicAggregate:
			// flag, no value
		}
	}

	return result
}

// parseTextOpen extracts OPEN event data from text format.
// Format: "peer <addr> remote as <n> <dir> open <id> router-id <ip> hold-time <t> cap <code> <name> [<value>]...".
// ASN is extracted from the uniform header.
func parseTextOpen(text string) *Event {
	s := textparse.NewScanner(strings.TrimRight(text, "\n"))

	// peer <addr>
	s.Next()
	addr, ok := s.Next()
	if !ok {
		return nil
	}

	event := &Event{
		Type:     eventOpen,
		PeerAddr: addr,
		Open:     &OpenInfo{},
	}

	// remote as <n>
	s.Next() // "remote"
	s.Next() // "as"
	if asnStr, ok := s.Next(); ok {
		if n, err := strconv.ParseUint(asnStr, 10, 32); err == nil {
			event.PeerASN = uint32(n)  //nolint:gosec // bounded by ParseUint bitSize=32
			event.Open.ASN = uint32(n) //nolint:gosec // bounded by ParseUint bitSize=32
		}
	}

	// <dir> open <id>
	s.Next() // direction
	s.Next() // "open"
	s.Next() // message ID

	// Keyword loop for remaining body
	for !s.Done() {
		tok, ok := s.Next()
		if !ok {
			break
		}
		switch tok {
		case tokenRouterID:
			if v, ok := s.Next(); ok {
				event.Open.RouterID = v
			}
		case tokenHoldTime:
			if v, ok := s.Next(); ok {
				if n, err := strconv.ParseUint(v, 10, 16); err == nil {
					event.Open.HoldTime = uint16(n) //nolint:gosec // bounded by ParseUint bitSize=16
				}
			}
		case tokenCap:
			// cap <code> <name> [<value>]
			codeStr, ok := s.Next()
			if !ok {
				continue
			}
			code, _ := strconv.Atoi(codeStr)
			name, ok := s.Next()
			if !ok {
				continue
			}
			value := ""
			if next, ok := s.Peek(); ok && next != tokenCap && next != tokenRouterID && next != tokenHoldTime {
				value, _ = s.Next()
			}
			event.Open.Capabilities = append(event.Open.Capabilities,
				CapabilityInfo{Code: code, Name: name, Value: value})
		}
	}

	return event
}

// parseTextState extracts state event data from text format.
// Format: "peer <addr> remote as <n> state <state>".
func parseTextState(text string) *Event {
	s := textparse.NewScanner(strings.TrimRight(text, "\n"))

	// peer <addr>
	s.Next()
	addr, ok := s.Next()
	if !ok {
		return nil
	}

	event := &Event{
		Type:     eventState,
		PeerAddr: addr,
	}

	// Keyword loop
	for !s.Done() {
		tok, ok := s.Next()
		if !ok {
			break
		}
		switch tok {
		case "remote":
			// remote as <n>
			if as, ok := s.Next(); ok && as == "as" {
				if v, ok := s.Next(); ok {
					if n, err := strconv.ParseUint(v, 10, 32); err == nil {
						event.PeerASN = uint32(n) //nolint:gosec // bounded by ParseUint bitSize=32
					}
				}
			}
		case "state":
			if v, ok := s.Next(); ok {
				event.State = v
			}
		}
	}

	return event
}

// parseTextRefresh extracts refresh event data from text format.
// Format: "peer <addr> remote as <n> <dir> refresh|borr|eorr <id> family <family>".
func parseTextRefresh(text string) *Event {
	s := textparse.NewScanner(strings.TrimRight(text, "\n"))

	// peer <addr>
	s.Next()
	addr, ok := s.Next()
	if !ok {
		return nil
	}

	// remote as <n>
	s.Next() // "remote"
	s.Next() // "as"
	s.Next() // ASN value

	// <dir> <type> <id>
	s.Next() // direction
	refreshType, ok := s.Next()
	if !ok {
		return nil
	}
	s.Next() // message ID

	event := &Event{
		Type:     refreshType,
		PeerAddr: addr,
	}

	// Keyword loop for family
	for !s.Done() {
		tok, ok := s.Next()
		if !ok {
			break
		}
		if tok == tokenFamily {
			if fam, ok := s.Next(); ok {
				if parts := strings.SplitN(fam, "/", 2); len(parts) == 2 {
					event.AFI = parts[0]
					event.SAFI = parts[1]
				}
			}
			break
		}
	}

	return event
}
