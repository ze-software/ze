package lepath

import (
	"strconv"
	"strings"
	"testing"
)

// TestSessionIDLengthIsBounded pins the one length bound every consumer of a
// session or agent id shares: the Bash hook's identity check, the claim
// markers, the commit namespace and the subagent start text all ask
// safeSessionID, so an id one of them accepts is an id all of them can carry
// whole.
// VALIDATES: an id of sessionIDLengthMax bytes is accepted by safeSessionID,
// ValidSessionID and ValidAgentID, and one byte more is refused by all three.
// PREVENTS: an id the start text would cut being printed as an identity line
// the Bash hook refuses.
func TestSessionIDLengthIsBounded(t *testing.T) {
	longest := strings.Repeat("a", sessionIDLengthMax)
	over := longest + "a"
	for _, check := range []struct {
		name  string
		valid func(string) bool
	}{
		{"safeSessionID", safeSessionID},
		{"ValidSessionID", ValidSessionID},
		{"ValidAgentID", ValidAgentID},
	} {
		if !check.valid(longest) {
			t.Errorf("%s refused an id of %d bytes, the bound", check.name, len(longest))
		}
		if check.valid(over) {
			t.Errorf("%s accepted an id of %d bytes, past the bound", check.name, len(over))
		}
	}
	if _, err := AgentFingerprint(longest, over); err == nil {
		t.Error("AgentFingerprint accepted an over-long agent id")
	}
	if _, err := AgentFingerprint(over, longest); err == nil {
		t.Error("AgentFingerprint accepted an over-long session id")
	}
	// The two ids at the bound, joined into the longest name this package
	// composes from them, still fit one file name.
	fingerprint, err := AgentFingerprint(longest, longest)
	if err != nil {
		t.Fatal(err)
	}
	if name := commitSessionPrefix + fingerprint; len(name) > 255 {
		t.Errorf("commit namespace file name is %d bytes, past a file name's 255", len(name))
	}
}

// TestIDRuleStatesTheBound pins the sentence every refusal of an id quotes.
// VALIDATES: IDRule names the characters safeSessionID accepts and states its
// length bound with the number sessionIDLengthMax holds.
// PREVENTS: a refusal of an over-long id that states only the character set,
// a rule the refused id satisfies.
func TestIDRuleStatesTheBound(t *testing.T) {
	rule := IDRule()
	for _, want := range []string{"letters, digits, '.', '_' and '-'", "at most " + strconv.Itoa(sessionIDLengthMax) + " bytes"} {
		if !strings.Contains(rule, want) {
			t.Errorf("IDRule() = %q, missing %q", rule, want)
		}
	}
}
