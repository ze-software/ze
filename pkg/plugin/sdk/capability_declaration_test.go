package sdk

import (
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// declarationExtractor reads a CapabilityDeclaration straight from each
// section's Data, so a test states the candidate declaration it wants.
func declarationExtractor(sections []ConfigSection) (CapabilityDeclaration, error) {
	var all CapabilityDeclaration
	for _, section := range sections {
		var one CapabilityDeclaration
		if err := json.Unmarshal([]byte(section.Data), &one); err != nil {
			return CapabilityDeclaration{}, err
		}
		all.Capabilities = append(all.Capabilities, one.Capabilities...)
	}
	return all, nil
}

func declarationSections(t *testing.T, declaration CapabilityDeclaration) []ConfigSection {
	t.Helper()
	data, err := json.Marshal(declaration)
	if err != nil {
		t.Fatal(err)
	}
	return []ConfigSection{{Root: "bgp", Data: string(data)}}
}

func configureParams(t *testing.T, declaration CapabilityDeclaration) json.RawMessage {
	t.Helper()
	params, err := json.Marshal(rpc.ConfigureInput{Sections: declarationSections(t, declaration)})
	if err != nil {
		t.Fatal(err)
	}
	return params
}

// verifyCandidate runs the plugin's composed config-verify callback on the
// candidate sections and returns the decoded answer.
func verifyCandidate(t *testing.T, p *Plugin, sections []ConfigSection) rpc.ConfigVerifyOutput {
	t.Helper()
	params, err := json.Marshal(rpc.ConfigVerifyInput{Sections: sections})
	if err != nil {
		t.Fatal(err)
	}
	p.mu.Lock()
	verify := p.callbacks[callbackConfigVerify]
	p.mu.Unlock()
	answer, err := verify(params)
	if err != nil {
		t.Fatalf("config-verify returned a transport error: %v", err)
	}
	var output rpc.ConfigVerifyOutput
	if err := json.Unmarshal(answer, &output); err != nil {
		t.Fatal(err)
	}
	return output
}

// startupDeclaration is the Stage 2 configuration every case starts from:
// peers a and b carry code 75 with their own payload, and code 2 is declared
// for every peer.
var startupDeclaration = CapabilityDeclaration{
	Capabilities: []CapabilityDecl{
		{Code: 75, Encoding: CapEncodingHex, Payload: "01", Peers: []string{"a"}},
		{Code: 75, Encoding: CapEncodingHex, Payload: "02", Peers: []string{"b"}},
		{Code: 2},
	},
}

// startedPlugin returns a plugin whose Stage 2 ran over startupDeclaration
// and whose config-verify is composed as Run composes it.
func startedPlugin(t *testing.T) *Plugin {
	t.Helper()
	p, _ := newTestPair(t)
	p.DeclareAppliesNothing("reads only capability leaves")
	p.DeclareCapabilitiesFrom(declarationExtractor)
	p.composeCapabilityVerify()
	if err := p.handleStartupConfigure(configureParams(t, startupDeclaration)); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestComposedVerifyCarriesCandidateCapabilities checks AC-2 at the SDK: a
// config-verify the plugin's own handler accepts answers OK and carries the
// candidate's whole declaration, whatever it changes against Stage 3, so the
// engine can install it. Method: start a plugin from one declaration, run its
// composed config-verify over candidates that keep, change, remove or add
// capabilities, and compare the answer's declaration with
// CandidateCapabilities over the same candidate and with the candidate itself.
func TestComposedVerifyCarriesCandidateCapabilities(t *testing.T) {
	cases := []struct {
		name      string
		candidate CapabilityDeclaration
	}{
		{name: "unchanged", candidate: startupDeclaration},
		{
			name: "changed payload",
			candidate: CapabilityDeclaration{Capabilities: []CapabilityDecl{
				{Code: 75, Encoding: CapEncodingHex, Payload: "09", Peers: []string{"a"}},
				{Code: 75, Encoding: CapEncodingHex, Payload: "02", Peers: []string{"b"}},
				{Code: 2},
			}},
		},
		{
			name: "code removed from a live peer",
			candidate: CapabilityDeclaration{Capabilities: []CapabilityDecl{
				{Code: 75, Encoding: CapEncodingHex, Payload: "01", Peers: []string{"a"}},
				{Code: 2},
			}},
		},
		{
			name: "peer added with a capability",
			candidate: CapabilityDeclaration{Capabilities: append(slices.Clone(startupDeclaration.Capabilities),
				CapabilityDecl{Code: 9, Encoding: CapEncodingHex, Payload: "03", Peers: []string{"c"}})},
		},
		{
			name: "dynamic group template added",
			candidate: CapabilityDeclaration{Capabilities: append(slices.Clone(startupDeclaration.Capabilities),
				CapabilityDecl{Code: 75, Encoding: CapEncodingHex, Payload: "01", Peers: []string{"group:ix"}})},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := startedPlugin(t)
			sections := declarationSections(t, tc.candidate)
			output := verifyCandidate(t, p, sections)
			if output.Status != rpc.StatusOK {
				t.Fatalf("status %q (%s), want OK: a reload may change a capability", output.Status, output.Error)
			}
			if output.Capabilities == nil {
				t.Fatal("the answer carries no declaration, so the engine would keep the Stage 3 one")
			}
			if !slices.EqualFunc(output.Capabilities.Capabilities, tc.candidate.Capabilities, capabilityDeclEqual) {
				t.Errorf("answer declares %+v, want the candidate's %+v", output.Capabilities.Capabilities, tc.candidate.Capabilities)
			}
			want, err := CandidateCapabilities(declarationExtractor, sections)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.EqualFunc(output.Capabilities.Capabilities, want.Capabilities, capabilityDeclEqual) {
				t.Errorf("config-verify carries %+v, CandidateCapabilities %+v", output.Capabilities.Capabilities, want.Capabilities)
			}
		})
	}
}

// capabilityDeclEqual compares two declarations field by field.
func capabilityDeclEqual(a, b CapabilityDecl) bool {
	return a.Code == b.Code && a.Encoding == b.Encoding && a.Payload == b.Payload && slices.Equal(a.Peers, b.Peers)
}

// TestComposedVerifyEmptyDeclarationRemovesAll checks a candidate that
// declares nothing answers an empty, non-nil list: nil would tell the engine
// to keep the Stage 3 declaration, so the removed capabilities would still be
// sent.
func TestComposedVerifyEmptyDeclarationRemovesAll(t *testing.T) {
	p := startedPlugin(t)
	output := verifyCandidate(t, p, declarationSections(t, CapabilityDeclaration{}))
	if output.Status != rpc.StatusOK {
		t.Fatalf("status %q (%s), want OK", output.Status, output.Error)
	}
	if output.Capabilities == nil {
		t.Fatal("an empty candidate answered no declaration: the engine would keep the old capabilities")
	}
	if output.Capabilities.Capabilities == nil {
		t.Fatal("an empty candidate answered a nil list, which reads as keep")
	}
	if len(output.Capabilities.Capabilities) != 0 {
		t.Errorf("answer declares %+v, want none", output.Capabilities.Capabilities)
	}
}

// TestComposedVerifyExtractorErrorRefuses checks a candidate the extractor
// cannot read is refused with the extractor's error rather than answered with
// a declaration nobody computed.
func TestComposedVerifyExtractorErrorRefuses(t *testing.T) {
	p := startedPlugin(t)
	output := verifyCandidate(t, p, []ConfigSection{{Root: "bgp", Data: "not json"}})
	if output.Status != rpc.StatusError {
		t.Fatalf("status %q, want a refusal", output.Status)
	}
	if output.Error == "" {
		t.Error("the refusal carries no text")
	}
	if output.Capabilities != nil {
		t.Errorf("a refusal carries a declaration: %+v", output.Capabilities)
	}
}

// TestDeclareCapabilitiesFromStageTwoOnly checks the declaration Stage 3 sends
// is the extractor's Stage 2 output, that a runtime configure redelivery
// (AC-2h) leaves it as it was, and that a verify, which only answers the
// candidate's declaration, does not rewrite it either.
func TestDeclareCapabilitiesFromStageTwoOnly(t *testing.T) {
	p := startedPlugin(t)
	p.mu.Lock()
	declared := slices.Clone(p.capabilities)
	p.mu.Unlock()
	if !slices.EqualFunc(declared, startupDeclaration.Capabilities, capabilityDeclEqual) {
		t.Fatalf("Stage 2 declared %+v, want %+v", declared, startupDeclaration.Capabilities)
	}

	p.mu.Lock()
	configure := p.callbacks[callbackConfigure]
	p.mu.Unlock()
	if _, err := configure(configureParams(t, CapabilityDeclaration{})); err != nil {
		t.Fatal(err)
	}
	verifyCandidate(t, p, declarationSections(t, CapabilityDeclaration{}))
	p.mu.Lock()
	after := slices.Clone(p.capabilities)
	p.mu.Unlock()
	if !slices.EqualFunc(after, declared, capabilityDeclEqual) {
		t.Fatalf("a configure redelivery or a verify rewrote the declaration: %+v, want %+v", after, declared)
	}
}

// TestDeclareCapabilitiesFromOwnVerifyAnswersFirst checks the plugin's own
// verify handler answers before the declaration, in both registration orders,
// and that its refusal carries no declaration.
func TestDeclareCapabilitiesFromOwnVerifyAnswersFirst(t *testing.T) {
	errOwn := errors.New("own refusal")
	for _, ownFirst := range []bool{true, false} {
		p, _ := newTestPair(t)
		p.DeclareAppliesNothing("reads only capability leaves")
		own := func([]ConfigSection) error { return errOwn }
		if ownFirst {
			p.OnConfigVerify(own)
			p.DeclareCapabilitiesFrom(declarationExtractor)
		} else {
			p.DeclareCapabilitiesFrom(declarationExtractor)
			p.OnConfigVerify(own)
		}
		p.composeCapabilityVerify()
		if err := p.handleStartupConfigure(configureParams(t, startupDeclaration)); err != nil {
			t.Fatal(err)
		}
		output := verifyCandidate(t, p, declarationSections(t, startupDeclaration))
		if output.Status != rpc.StatusError {
			t.Fatalf("ownFirst=%v: status %q, want the own refusal", ownFirst, output.Status)
		}
		if output.Error != errOwn.Error() {
			t.Errorf("ownFirst=%v: answer %q, want the plugin's own %q", ownFirst, output.Error, errOwn.Error())
		}
		if output.Capabilities != nil {
			t.Errorf("ownFirst=%v: the own refusal carries a declaration", ownFirst)
		}
	}
}

// TestDeclareCapabilitiesFromConstantExtractor checks the external bridge's
// shape: an extractor that ignores the sections answers its one declaration
// for any candidate.
func TestDeclareCapabilitiesFromConstantExtractor(t *testing.T) {
	p, _ := newTestPair(t)
	p.DeclareAppliesNothing("reads only capability leaves")
	constant := CapabilityDeclaration{Capabilities: []CapabilityDecl{{Code: 2}}}
	p.DeclareCapabilitiesFrom(func([]ConfigSection) (CapabilityDeclaration, error) {
		return constant, nil
	})
	p.composeCapabilityVerify()
	if err := p.handleStartupConfigure(configureParams(t, startupDeclaration)); err != nil {
		t.Fatal(err)
	}
	output := verifyCandidate(t, p, []ConfigSection{{Root: "bgp", Data: "{}"}})
	if output.Status != rpc.StatusOK {
		t.Fatalf("a constant extractor was refused: %s", output.Error)
	}
	if output.Capabilities == nil || !slices.EqualFunc(output.Capabilities.Capabilities, constant.Capabilities, capabilityDeclEqual) {
		t.Errorf("answer declares %+v, want the constant %+v", output.Capabilities, constant.Capabilities)
	}
}
