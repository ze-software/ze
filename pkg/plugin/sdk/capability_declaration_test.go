package sdk

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
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
		all.Peers = append(all.Peers, one.Peers...)
		if one.Labels != nil {
			all.Labels = one.Labels
		}
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

// verifyCandidate runs the plugin's composed config-verify callback on a
// candidate and returns the answer's status and error text.
func verifyCandidate(t *testing.T, p *Plugin, candidate CapabilityDeclaration) (status, text string) {
	t.Helper()
	params, err := json.Marshal(rpc.ConfigVerifyInput{Sections: declarationSections(t, candidate)})
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
	var decoded struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	}
	if err := json.Unmarshal(answer, &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded.Status, decoded.Error
}

var softverLeaves = map[uint8]string{75: "software-version", 2: "route-refresh"}

// startupDeclaration is the Stage 2 configuration every case starts from:
// peers a and b carry code 75 with their own payload, and code 2 is declared
// for every peer.
var startupDeclaration = CapabilityDeclaration{
	Capabilities: []CapabilityDecl{
		{Code: 75, Encoding: CapEncodingHex, Payload: "01", Peers: []string{"a"}},
		{Code: 75, Encoding: CapEncodingHex, Payload: "02", Peers: []string{"b"}},
		{Code: 2},
	},
	Peers: []string{"a", "b"},
}

// startedPlugin returns a plugin whose Stage 2 ran over startupDeclaration
// and whose config-verify is composed as Run composes it.
func startedPlugin(t *testing.T) *Plugin {
	t.Helper()
	p, _ := newTestPair(t)
	p.DeclareAppliesNothing("reads only capability leaves")
	p.DeclareCapabilitiesFrom(declarationExtractor, softverLeaves)
	p.composeCapabilityVerify()
	if err := p.handleStartupConfigure(configureParams(t, startupDeclaration)); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestDeclareCapabilitiesFrom checks the D2 verdict table at the SDK.
// Method: start a plugin from one declaration, then run its composed
// config-verify over candidates that change, remove, add or keep each
// capability, and read the verdict and the refusal text.
func TestDeclareCapabilitiesFrom(t *testing.T) {
	cases := []struct {
		name      string
		candidate CapabilityDeclaration
		refused   []string // the change lines' opening words, in order
	}{
		{
			name:      "unchanged is accepted",
			candidate: startupDeclaration,
		},
		{
			name: "changed payload is refused",
			candidate: CapabilityDeclaration{
				Capabilities: []CapabilityDecl{
					{Code: 75, Encoding: CapEncodingHex, Payload: "09", Peers: []string{"a"}},
					{Code: 75, Encoding: CapEncodingHex, Payload: "02", Peers: []string{"b"}},
					{Code: 2},
				},
				Peers: []string{"a", "b"},
			},
			refused: []string{"peer a: capability software-version"},
		},
		{
			name: "code removed from a live peer is refused",
			candidate: CapabilityDeclaration{
				Capabilities: []CapabilityDecl{
					{Code: 75, Encoding: CapEncodingHex, Payload: "01", Peers: []string{"a"}},
					{Code: 2},
				},
				Peers: []string{"a", "b"},
			},
			refused: []string{"peer b: capability software-version"},
		},
		{
			name: "peer added with a capability is refused",
			candidate: CapabilityDeclaration{
				Capabilities: append(slices.Clone(startupDeclaration.Capabilities),
					CapabilityDecl{Code: 75, Encoding: CapEncodingHex, Payload: "01", Peers: []string{"c"}}),
				Peers: []string{"a", "b", "c"},
			},
			refused: []string{"peer c: capability software-version"},
		},
		{
			name: "peer removed is accepted",
			candidate: CapabilityDeclaration{
				Capabilities: []CapabilityDecl{
					{Code: 75, Encoding: CapEncodingHex, Payload: "01", Peers: []string{"a"}},
					{Code: 2},
				},
				Peers: []string{"a"},
			},
		},
		{
			name: "all-peer declaration removed is refused naming every peer",
			candidate: CapabilityDeclaration{
				Capabilities: startupDeclaration.Capabilities[:2],
				Peers:        []string{"a", "b"},
			},
			refused: []string{"every peer: capability route-refresh"},
		},
		{
			name: "peer added with no capability keeps the all-peer declaration",
			candidate: CapabilityDeclaration{
				Capabilities: startupDeclaration.Capabilities,
				Peers:        []string{"a", "b", "c"},
			},
		},
		{
			name: "a selector the extractor labels is named by its label",
			candidate: CapabilityDeclaration{
				Capabilities: append(slices.Clone(startupDeclaration.Capabilities),
					CapabilityDecl{Code: 75, Encoding: CapEncodingHex, Payload: "01", Peers: []string{"group:ix"}}),
				Peers:  []string{"a", "b", "group:ix"},
				Labels: map[string]string{"group:ix": "peers of dynamic group ix"},
			},
			refused: []string{"peers of dynamic group ix: capability software-version"},
		},
		{
			name: "several changes are one line each, sorted",
			candidate: CapabilityDeclaration{
				Capabilities: []CapabilityDecl{
					{Code: 75, Encoding: CapEncodingHex, Payload: "07", Peers: []string{"b"}},
					{Code: 75, Encoding: CapEncodingHex, Payload: "07", Peers: []string{"a"}},
				},
				Peers: []string{"b", "a"},
			},
			refused: []string{
				"every peer: capability route-refresh",
				"peer a: capability software-version",
				"peer b: capability software-version",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := startedPlugin(t)
			status, text := verifyCandidate(t, p, tc.candidate)
			// CheckCapabilityReload, which plugin tests drive, gives the
			// composed verify's answer for the same pair of configurations.
			checked := CheckCapabilityReload("test-plugin", declarationExtractor, softverLeaves,
				declarationSections(t, startupDeclaration), declarationSections(t, tc.candidate))
			if checked == nil {
				if status != rpc.StatusOK {
					t.Errorf("CheckCapabilityReload accepted what config-verify refused: %s", text)
				}
			} else if checked.Error() != text {
				t.Errorf("CheckCapabilityReload = %q, config-verify = %q", checked.Error(), text)
			}
			if len(tc.refused) == 0 {
				if status != rpc.StatusOK {
					t.Fatalf("refused, want accepted: %s", text)
				}
				return
			}
			if status != rpc.StatusError {
				t.Fatalf("status %q, want a refusal naming %v", status, tc.refused)
			}
			lines := strings.Split(text, "\n")
			for i, want := range tc.refused {
				if !strings.HasPrefix(lines[i], want) {
					t.Errorf("line %d is %q, want it to open with %q", i+1, lines[i], want)
				}
				if !strings.HasSuffix(lines[i], "cannot change while ze runs (plugin test-plugin).") {
					t.Errorf("line %d %q does not say it cannot change and name the plugin", i+1, lines[i])
				}
			}
			if !strings.HasPrefix(lines[len(tc.refused)], "The plugin sends its capabilities only when ze starts") {
				t.Errorf("no reason line after the changes:\n%s", text)
			}
			if !strings.HasSuffix(text, RestartSteps()) {
				t.Errorf("refusal does not end with the restart steps:\n%s", text)
			}
		})
	}
}

// TestDeclareCapabilitiesFromStageTwoOnly checks the declaration Stage 3 sends
// is the extractor's Stage 2 output, and that a runtime configure redelivery
// (AC-2h) leaves it as it was.
func TestDeclareCapabilitiesFromStageTwoOnly(t *testing.T) {
	p := startedPlugin(t)
	p.mu.Lock()
	declared := slices.Clone(p.capabilities)
	p.mu.Unlock()
	if len(declared) != len(startupDeclaration.Capabilities) {
		t.Fatalf("Stage 2 declared %d capabilities, want %d", len(declared), len(startupDeclaration.Capabilities))
	}

	redelivered := CapabilityDeclaration{Peers: []string{"a"}}
	p.mu.Lock()
	configure := p.callbacks[callbackConfigure]
	p.mu.Unlock()
	if _, err := configure(configureParams(t, redelivered)); err != nil {
		t.Fatal(err)
	}
	p.mu.Lock()
	after := p.capabilities
	p.mu.Unlock()
	if len(after) != len(declared) {
		t.Fatalf("a configure redelivery rewrote the declaration: %d capabilities, want %d", len(after), len(declared))
	}

	// A peer removed by an accepted reload and then re-added with the same
	// capability is still accepted, because the declaration still holds it.
	status, text := verifyCandidate(t, p, startupDeclaration)
	if status != rpc.StatusOK {
		t.Fatalf("re-adding a declared peer was refused: %s", text)
	}
}

// TestDeclareCapabilitiesFromOwnVerifyAnswersFirst checks the plugin's own
// verify handler answers before the comparison, in both registration orders.
func TestDeclareCapabilitiesFromOwnVerifyAnswersFirst(t *testing.T) {
	changed := CapabilityDeclaration{Peers: []string{"a", "b"}}
	errOwn := errors.New("own refusal")
	for _, ownFirst := range []bool{true, false} {
		p, _ := newTestPair(t)
		p.DeclareAppliesNothing("reads only capability leaves")
		own := func([]ConfigSection) error { return errOwn }
		if ownFirst {
			p.OnConfigVerify(own)
			p.DeclareCapabilitiesFrom(declarationExtractor, softverLeaves)
		} else {
			p.DeclareCapabilitiesFrom(declarationExtractor, softverLeaves)
			p.OnConfigVerify(own)
		}
		p.composeCapabilityVerify()
		if err := p.handleStartupConfigure(configureParams(t, startupDeclaration)); err != nil {
			t.Fatal(err)
		}
		status, text := verifyCandidate(t, p, changed)
		if status != rpc.StatusError {
			t.Fatalf("ownFirst=%v: status %q, want the own refusal", ownFirst, status)
		}
		if text != errOwn.Error() {
			t.Errorf("ownFirst=%v: answer %q, want the plugin's own %q", ownFirst, text, errOwn.Error())
		}
	}
}

// TestDeclareCapabilitiesFromConstantExtractor checks the external bridge's
// shape: an extractor that ignores the sections always accepts.
func TestDeclareCapabilitiesFromConstantExtractor(t *testing.T) {
	p, _ := newTestPair(t)
	p.DeclareAppliesNothing("reads only capability leaves")
	constant := CapabilityDeclaration{Capabilities: []CapabilityDecl{{Code: 2}}}
	p.DeclareCapabilitiesFrom(func([]ConfigSection) (CapabilityDeclaration, error) {
		return constant, nil
	}, softverLeaves)
	p.composeCapabilityVerify()
	if err := p.handleStartupConfigure(configureParams(t, startupDeclaration)); err != nil {
		t.Fatal(err)
	}
	status, text := verifyCandidate(t, p, CapabilityDeclaration{Peers: []string{"z"}})
	if status != rpc.StatusOK {
		t.Fatalf("a constant extractor was refused: %s", text)
	}
}
