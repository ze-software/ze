// Design: docs/architecture/api/process-protocol.md -- a reload re-sends the capability declaration
// Overview: sdk.go -- plugin SDK core

package sdk

import (
	"encoding/json"
	"fmt"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// CapabilityDeclaration is what a capability extractor computes from one
// configuration: the capabilities to advertise.
type CapabilityDeclaration struct {
	Capabilities []CapabilityDecl
}

// CapabilityExtractor computes a plugin's capability declaration from the
// config sections the plugin receives. It MUST be a pure function of those
// sections, filtering section.Root itself, so the same configuration always
// yields the same declaration.
type CapabilityExtractor func(sections []ConfigSection) (CapabilityDeclaration, error)

// capabilitySource is the extractor a plugin registered.
type capabilitySource struct {
	extract CapabilityExtractor
}

// DeclareCapabilitiesFrom registers the extractor that computes this plugin's
// capability declaration. It MUST be called before Run, and it is the only way
// a plugin declares capabilities.
//
// Run calls the extractor on the Stage 2 configuration and sends its
// capabilities at Stage 3. At config-verify the plugin's own verify handler
// answers first; when it accepts, the extractor runs on the candidate sections
// and the answer carries the candidate's whole declaration
// (rpc.ConfigVerifyOutput.Capabilities). The engine installs it when the
// transaction applies, so a peer a reload adds sends its OPEN with its
// capabilities, and puts the previous one back when the transaction does not
// commit. A runtime configure redelivery reaches OnConfigure and never the
// extractor.
func (p *Plugin) DeclareCapabilitiesFrom(extract CapabilityExtractor) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.capabilitySource = &capabilitySource{extract: extract}
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

// composeCapabilityVerify puts the candidate declaration behind whichever
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
		var output rpc.ConfigVerifyOutput
		if err := json.Unmarshal(answer, &output); err != nil {
			// An answer nobody can read is a refusal: accepting it would let
			// the declaration overrule a handler whose verdict is unknown.
			return marshalStatusError("decode config-verify answer: " + err.Error())
		}
		if output.Status != rpc.StatusOK {
			return answer, nil
		}
		var input rpc.ConfigVerifyInput
		if err := json.Unmarshal(params, &input); err != nil {
			return marshalStatusError("unmarshal config-verify: " + err.Error())
		}
		declared, err := CandidateCapabilities(source.extract, input.Sections)
		if err != nil {
			return marshalStatusError(err.Error())
		}
		output.Capabilities = declared
		return json.Marshal(&output)
	}
}

// CandidateCapabilities is the declaration a config-verify answer carries for
// candidate: the extractor's capabilities, in the shape Stage 3 sends. A
// plugin's own tests call it to prove that the extractor it registers gives a
// peer the candidate adds its capability. Safe for concurrent use when extract
// is.
func CandidateCapabilities(extract CapabilityExtractor, candidate []ConfigSection) (*rpc.DeclareCapabilitiesInput, error) {
	declaration, err := extract(candidate)
	if err != nil {
		return nil, fmt.Errorf("capabilities: %w", err)
	}
	// A non-nil list even when empty: an empty declaration removes every
	// capability the plugin declared, and nil would keep them.
	capabilities := declaration.Capabilities
	if capabilities == nil {
		capabilities = []CapabilityDecl{}
	}
	return &rpc.DeclareCapabilitiesInput{Capabilities: capabilities}, nil
}
