// Design: docs/architecture/api/process-protocol.md -- capability declarations fixed for the process life
// Overview: sdk.go -- plugin SDK core
// Related: restart_text.go -- the restart advice a refusal appends

package sdk

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// CapabilityDeclaration is what a capability extractor computes from one
// configuration: the capabilities to advertise, and every peer selector the
// configuration names, whether or not that peer carries a capability.
type CapabilityDeclaration struct {
	Capabilities []CapabilityDecl
	// Peers holds every peer selector the configuration names, in the form
	// CapabilityDecl.Peers uses. A declared selector absent from it is a peer
	// the configuration removed: that peer stops and sends no OPEN again, so its
	// declaration may differ. An all-peer declaration (empty Peers on the
	// CapabilityDecl) needs no selector here.
	Peers []string
	// Labels maps a selector that is not a peer's own name to the config
	// object a refusal names for it, a dynamic group's template selector for
	// one. A selector absent from it is named "peer <selector>".
	Labels map[string]string
}

// CapabilityExtractor computes a plugin's capability declaration from the
// config sections the plugin receives. It MUST be a pure function of those
// sections, filtering section.Root itself, so the same configuration always
// yields the same declaration.
type CapabilityExtractor func(sections []ConfigSection) (CapabilityDeclaration, error)

// capabilitySource is the extractor a plugin registered, with the config leaf
// that sets each capability code, so a refusal names configuration.
type capabilitySource struct {
	extract CapabilityExtractor
	leaves  map[uint8]string
}

// DeclareCapabilitiesFrom registers the extractor that computes this plugin's
// capability declaration, and the config leaf that sets each code. It MUST be
// called before Run, and it is the only way a plugin declares capabilities.
//
// Run calls the extractor once on the Stage 2 configuration and sends its
// capabilities at Stage 3. The engine reads that declaration for every OPEN
// until the plugin process ends, so the declaration is never rewritten: a
// runtime configure redelivery reaches OnConfigure and never the extractor.
// At config-verify the plugin's own verify handler answers first; when it
// accepts, the extractor runs on the candidate sections and a candidate whose
// declaration differs for a peer that stays configured is refused, because ze
// would keep sending the old capability in its OPEN.
func (p *Plugin) DeclareCapabilitiesFrom(extract CapabilityExtractor, leaves map[uint8]string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.capabilitySource = &capabilitySource{extract: extract, leaves: leaves}
}

// handleStartupConfigure serves the Stage 2 configure: the plugin's own
// handler, then the declaration Stage 3 sends. It is the one place the
// extractor's output becomes the declaration.
func (p *Plugin) handleStartupConfigure(params json.RawMessage) error {
	if err := p.handleConfigure(params); err != nil {
		return err
	}
	p.mu.Lock()
	source := p.capabilitySource
	p.mu.Unlock()
	if source == nil {
		return nil
	}
	sections, err := configureSections(params)
	if err != nil {
		return err
	}
	declaration, err := source.extract(sections)
	if err != nil {
		return fmt.Errorf("capabilities: %w", err)
	}
	p.mu.Lock()
	p.capabilities = declaration.Capabilities
	p.mu.Unlock()
	return nil
}

// configureSections reads the sections of a configure callback's parameters.
func configureSections(params json.RawMessage) ([]ConfigSection, error) {
	var input rpc.ConfigureInput
	if params == nil {
		return nil, nil
	}
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, fmt.Errorf("unmarshal configure: %w", err)
	}
	return input.Sections, nil
}

// composeCapabilityVerify puts the declaration comparison behind whichever
// config-verify handler the plugin registered. Run calls it before Stage 1,
// after every On* registration, so the order in which OnConfigVerify and
// DeclareCapabilitiesFrom were called does not matter.
func (p *Plugin) composeCapabilityVerify() {
	p.mu.Lock()
	defer p.mu.Unlock()
	source := p.capabilitySource
	if source == nil {
		return
	}
	own := p.callbacks[callbackConfigVerify]
	p.callbacks[callbackConfigVerify] = func(params json.RawMessage) (json.RawMessage, error) {
		answer, err := own(params)
		if err != nil {
			return answer, err
		}
		if verifyAnswerRefuses(answer) {
			return answer, nil
		}
		var input rpc.ConfigVerifyInput
		if err := json.Unmarshal(params, &input); err != nil {
			return marshalStatusError("unmarshal config-verify: " + err.Error())
		}
		p.mu.Lock()
		declared := p.capabilities
		p.mu.Unlock()
		if err := source.verifyCandidate(p.name, declared, input.Sections); err != nil {
			return marshalStatusError(err.Error())
		}
		return answer, nil
	}
}

// CheckCapabilityReload answers what config-verify answers, once the plugin's
// own verify handler has accepted, for a plugin that registered extract and
// leaves with DeclareCapabilitiesFrom, started on the startup sections, and is
// offered the candidate sections: nil when the candidate keeps every
// declaration a running OPEN carries, else the refusal. It runs the comparison
// the composed config-verify runs, so a plugin's own tests prove that the
// extractor and leaves it registers make the SDK refuse a change of its
// capability and accept an unchanged one. Safe for concurrent use when extract
// is.
func CheckCapabilityReload(plugin string, extract CapabilityExtractor, leaves map[uint8]string, startup, candidate []ConfigSection) error {
	declared, err := extract(startup)
	if err != nil {
		return fmt.Errorf("capabilities: %w", err)
	}
	source := capabilitySource{extract: extract, leaves: leaves}
	return source.verifyCandidate(plugin, declared.Capabilities, candidate)
}

// verifyCandidate is the comparison behind config-verify: the candidate
// sections' declaration against the Stage 3 declaration. It returns the
// extractor's error, or the refusal text when a peer that stays configured
// would need a different OPEN.
func (source *capabilitySource) verifyCandidate(plugin string, declared []CapabilityDecl, sections []ConfigSection) error {
	candidate, err := source.extract(sections)
	if err != nil {
		return err
	}
	changes := capabilityChanges(declared, &candidate)
	if len(changes) == 0 {
		return nil
	}
	return errors.New(capabilityRefusal(plugin, source.leaves, candidate.Labels, changes))
}

// verifyAnswerRefuses reports whether a config-verify answer is a refusal.
// An answer that does not decode is treated as one: accepting it would let the
// comparison overrule a handler whose answer nobody can read.
func verifyAnswerRefuses(answer json.RawMessage) bool {
	var status struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(answer, &status); err != nil {
		return true
	}
	return status.Status != rpc.StatusOK
}

// allPeersSelector is the selector an all-peer declaration (empty Peers) is
// compared under. No configured selector is empty: a peer name and a group
// key are both non-empty node names.
const allPeersSelector = ""

// capabilityChange is one capability whose declaration a candidate changes for
// one selector.
type capabilityChange struct {
	selector string
	code     uint8
}

// capabilityChanges compares the Stage 3 declaration with a candidate's, per
// selector and code, and returns each change the running OPEN cannot carry,
// sorted by selector (every peer first), then code.
//
// A selector the candidate no longer names is skipped: that peer is removed and
// sends no OPEN again. Every other selector, and the all-peer selector, must
// declare the same encodings and payloads for every code: a changed payload, a
// code removed from a live peer, and a code added (a peer added on reload
// included) are each a change.
func capabilityChanges(declared []CapabilityDecl, candidate *CapabilityDeclaration) []capabilityChange {
	before := declarationIndex(declared)
	after := declarationIndex(candidate.Capabilities)

	live := map[string]bool{allPeersSelector: true}
	for _, selector := range candidate.Peers {
		live[selector] = true
	}

	var changes []capabilityChange
	for selector := range live {
		codes := map[uint8]bool{}
		for code := range before[selector] {
			codes[code] = true
		}
		for code := range after[selector] {
			codes[code] = true
		}
		for code := range codes {
			if slices.Equal(before[selector][code], after[selector][code]) {
				continue
			}
			changes = append(changes, capabilityChange{selector: selector, code: code})
		}
	}
	slices.SortFunc(changes, func(a, b capabilityChange) int {
		if c := strings.Compare(a.selector, b.selector); c != 0 {
			return c
		}
		return int(a.code) - int(b.code)
	})
	return changes
}

// declarationIndex maps selector, then code, to the sorted encodings and
// payloads declared for it, so two declarations compare without regard to the
// order the extractor listed them in.
func declarationIndex(decls []CapabilityDecl) map[string]map[uint8][]string {
	index := map[string]map[uint8][]string{}
	add := func(selector string, code uint8, value string) {
		if index[selector] == nil {
			index[selector] = map[uint8][]string{}
		}
		index[selector][code] = append(index[selector][code], value)
	}
	for i := range decls {
		decl := &decls[i]
		value := string(decl.Encoding) + ":" + decl.Payload
		if len(decl.Peers) == 0 {
			add(allPeersSelector, decl.Code, value)
			continue
		}
		for _, selector := range decl.Peers {
			add(selector, decl.Code, value)
		}
	}
	for _, codes := range index {
		for code := range codes {
			slices.Sort(codes[code])
		}
	}
	return index
}

// capabilityRefusal is the text of a refused reload: one line per change, the
// reason once, then the restart advice. labels is the candidate's
// CapabilityDeclaration.Labels.
func capabilityRefusal(plugin string, leaves map[uint8]string, labels map[string]string, changes []capabilityChange) string {
	var text strings.Builder
	selectors := map[string]bool{}
	for _, change := range changes {
		selectors[change.selector] = true
		text.WriteString(selectorLabel(labels, change.selector))
		text.WriteString(": capability ")
		text.WriteString(capabilityLeaf(leaves, change.code))
		text.WriteString(" cannot change while ze runs (plugin ")
		text.WriteString(plugin)
		text.WriteString(").\n")
	}
	text.WriteString("The plugin sends its capabilities only when ze starts, so ze would keep sending the old value in its OPEN to ")
	text.WriteString(refusalAudience(selectors))
	text.WriteString(".\n")
	text.WriteString(RestartSteps())
	return text.String()
}

// capabilityLeaf names the config leaf that sets code. A code the plugin
// registered no leaf for is named by its number, which is still true.
func capabilityLeaf(leaves map[uint8]string, code uint8) string {
	if leaf, ok := leaves[code]; ok {
		return leaf
	}
	return "code " + strconv.Itoa(int(code))
}

// selectorLabel names the config object a change is about: every peer for an
// all-peer declaration, the extractor's label where it gave one, else the peer.
func selectorLabel(labels map[string]string, selector string) string {
	if selector == allPeersSelector {
		return "every peer"
	}
	if label, ok := labels[selector]; ok {
		return label
	}
	return "peer " + selector
}

// refusalAudience is who would keep receiving the old OPEN.
func refusalAudience(selectors map[string]bool) string {
	if selectors[allPeersSelector] {
		return "every peer"
	}
	if len(selectors) == 1 {
		return "this peer"
	}
	return "these peers"
}
