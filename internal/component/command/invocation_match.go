// Design: docs/architecture/api/commands.md -- one matcher for typed command declarations.
// Related: invocation_scan.go -- decoded operational words; arguments.go -- placement.
package command

import "strings"

// MatchInvocationPath matches decoded words against a registered path and its
// original typed definitions. It returns the unconsumed arguments and bound
// inline selectors, without invoking a handler. Explicit and anchored slots
// are bound here and judged by ValidateArgs at execution; the existing implicit
// selector fallback validates its guess before accepting it. No definitions
// means literal path matching, as for a plugin with no CLI declaration.
// Definitions MUST be the immutable registered values, never reconstructed
// unrestricted placeholders. Input words and the declaration bound all loops.
func MatchInvocationPath(tokens []string, key string, defs []ArgDef) ([]string, map[string]string, bool) {
	keyTokens := strings.Fields(key)
	if len(keyTokens) == 0 || len(tokens) < len(keyTokens) {
		return nil, nil, false
	}
	defByName := make(map[string]*ArgDef, len(defs))
	for i := range defs {
		defByName[strings.ToLower(defs[i].Name())] = &defs[i]
	}
	inIdx := 0
	selectors := make(map[string]string)
	for keyIdx, keyTok := range keyTokens {
		if inIdx >= len(tokens) || !strings.EqualFold(tokens[inIdx], keyTok) {
			return nil, nil, false
		}
		if def, ok := defByName[strings.ToLower(keyTok)]; ok && def.Kind() != ArgFlag && inIdx+1 < len(tokens) {
			if keyIdx+1 >= len(keyTokens) || !strings.EqualFold(tokens[inIdx+1], keyTokens[keyIdx+1]) {
				// The keyword declares the slot. ValidateArgs later names any
				// bound it breaks instead of reporting an unknown command.
				selectors[def.Name()] = tokens[inIdx+1]
				inIdx += 2
				continue
			}
		}
		if keyIdx+1 < len(keyTokens) && inIdx+1 < len(tokens) && !strings.EqualFold(tokens[inIdx+1], keyTokens[keyIdx+1]) {
			if def := AnchoredInvocationArg(keyTok, defs, selectors); def != nil {
				selectors[def.Name()] = tokens[inIdx+1]
				inIdx += 2
				continue
			}
			if def := ImplicitInvocationArg(keyTokens, defs, selectors); def != nil {
				value := tokens[inIdx+1]
				if err := ValidateArgString(value, def); err != nil {
					return nil, nil, false
				}
				selectors[def.Name()] = value
				inIdx += 2
				continue
			}
		}
		inIdx++
	}
	if len(selectors) == 0 {
		selectors = nil
	}
	return tokens[inIdx:], selectors, true
}

// AnchoredInvocationArg answers the one unmatched leaf whose declared anchor
// names keyTok. Two leaves on one keyword are ambiguous and name no slot.
// It borrows the immutable definitions supplied to MatchInvocationPath.
func AnchoredInvocationArg(keyTok string, defs []ArgDef, matched map[string]string) *ArgDef {
	var found *ArgDef
	for i := range defs {
		def := &defs[i]
		if _, ok := matched[def.Name()]; ok {
			continue
		}
		if def.Anchor() == "" {
			continue
		}
		if !strings.EqualFold(def.Anchor(), keyTok) {
			continue
		}
		if found != nil {
			return nil
		}
		found = def
	}
	return found
}

// ImplicitInvocationArg retains the dispatcher's fallback for an unmatched
// mandatory string leaf between path words. One pattern-less candidate wins
// over patterned candidates; otherwise exactly one patterned candidate wins.
// Keyword-only leaves and leaves named in the path are never candidates.
// It borrows the immutable definitions supplied to MatchInvocationPath.
func ImplicitInvocationArg(keyTokens []string, defs []ArgDef, matched map[string]string) *ArgDef {
	var loose, patterned *ArgDef
	looseCount, patternedCount := 0, 0
	for i := range defs {
		def := &defs[i]
		if _, ok := matched[def.Name()]; ok {
			continue
		}
		if def.Kind() != ArgString {
			continue
		}
		if !def.Mandatory() {
			continue
		}
		if def.KeywordOnly() {
			continue
		}
		if invocationKeyTokenPresent(keyTokens, def.Name()) {
			continue
		}
		if Constraint(def) == ConstraintPattern {
			patterned, patternedCount = def, patternedCount+1
			continue
		}
		loose, looseCount = def, looseCount+1
	}
	if looseCount == 1 {
		return loose
	}
	if looseCount == 0 && patternedCount == 1 {
		return patterned
	}
	return nil
}

func invocationKeyTokenPresent(keyTokens []string, name string) bool {
	for _, token := range keyTokens {
		if strings.EqualFold(token, name) {
			return true
		}
	}
	return false
}
