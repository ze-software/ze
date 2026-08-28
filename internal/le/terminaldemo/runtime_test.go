package terminaldemo

import (
	"bytes"
	"strings"
	"testing"
)

func TestRuntimeMainRendersEmbeddedCard(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := RuntimeMain([]string{"card", "launcher", "intro"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.HasPrefix(stdout.String(), "\x1b[H\x1b[2J") {
		t.Fatalf("card did not clear the terminal first: %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "Find a Ze command without memorizing the tree") {
		t.Fatalf("card content is absent: %q", stdout.String())
	}
}

func TestRuntimeMainRefusesUnknownDemoAction(t *testing.T) {
	var stderr bytes.Buffer
	code := RuntimeMain([]string{"run", "unknown", "start"}, &bytes.Buffer{}, &stderr)
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), `unknown demo "unknown"`) {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
