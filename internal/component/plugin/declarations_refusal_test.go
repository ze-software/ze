package plugin

import (
	"testing"

	"github.com/ze-software/ze/internal/component/command/commandtest"
)

// TestShowPluginDeclarationsRefusalIsReturned proves `show plugin declarations`
// given a value answers its refusal in the LocalAnswer, naming the keyword
// that takes a config file, rather than writing it.
// Method: one stray value, the refusal compared whole.
func TestShowPluginDeclarationsRefusalIsReturned(t *testing.T) {
	want := "error: show plugin declarations takes no value, and stray follows it: name a config file with " +
		keywordConfig + " <path>\n"

	answer := dataDeclarations(commandtest.Args("stray"))
	if answer.Payload != nil || answer.Code != 1 {
		t.Errorf("answer = payload %v, code %d; want nil, 1", answer.Payload, answer.Code)
	}
	if answer.Diagnostic != want {
		t.Errorf("diagnostic:\n got %q\nwant %q", answer.Diagnostic, want)
	}
}
