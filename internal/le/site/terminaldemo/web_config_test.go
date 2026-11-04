// Design: docs/contributing/terminal-demos.md -- shared browser journey consumers.
package siteterminaldemo

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestWebConfigConsumersRequireRunnableJourney invokes both real runtime entries
// and proves unreadable, malformed and skipped journeys refuse before daemon
// startup, rather than falling back to a separate validation/recording workflow.
func TestWebConfigConsumersRequireRunnableJourney(t *testing.T) {
	for _, entry := range [][]string{{"validate", "web-config"}, {"run", "web-config", "record"}} {
		for _, kind := range []string{"missing", "malformed", "skipped"} {
			t.Run(strings.Join(entry, "-")+"/"+kind, func(t *testing.T) {
				root := t.TempDir()
				t.Setenv("ZE_DEMO_ROOT", root)
				marker := filepath.Join(root, "started")
				bin := filepath.Join(root, "bin")
				mustWrite(t, filepath.Join(bin, "ze"), "#!/bin/sh\ntouch \"$JOURNEY_START_MARKER\"\nexit 1\n")
				if err := os.Chmod(filepath.Join(bin, "ze"), 0o700); err != nil {
					t.Fatal(err)
				}
				t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
				t.Setenv("JOURNEY_START_MARKER", marker)
				switch kind {
				case "missing":
				case "malformed":
					mustWrite(t, filepath.Join(root, recordedWebConfigJourney), "not-a-directive\n")
				case "skipped":
					mustWrite(t, filepath.Join(root, recordedWebConfigJourney), "option=skip:reason=owned test refusal\n")
				}
				var output, diagnostic bytes.Buffer
				if code := RuntimeMain(entry, &output, &diagnostic); code == 0 {
					t.Fatalf("%v accepted %s journey: %s", entry, kind, output.String())
				}
				if !strings.Contains(diagnostic.String(), recordedWebConfigJourney) {
					t.Fatalf("%v did not identify the refused shared journey: %s", entry, diagnostic.String())
				}
				if _, err := os.Stat(marker); !os.IsNotExist(err) {
					t.Fatalf("%v started daemon setup for %s journey: %v", entry, kind, err)
				}
			})
		}
	}
}

// TestWebConfigDigestFollowsSharedJourney proves source freshness changes when
// either the authored journey or its browser executor changes outside the demo
// directory. The test judges the actual producer's content digest.
func TestWebConfigDigestFollowsSharedJourney(t *testing.T) {
	fixture := newDemoFixture(t)
	driver := filepath.Join(fixture.root, "demos", "terminal", "web-config", "demo.cjs")
	journey := filepath.Join(fixture.root, recordedWebConfigJourney)
	runner := filepath.Join(fixture.root, "internal", "component", "web", "testing", "runner.go")
	mustWrite(t, driver, "capture driver\n")
	mustWrite(t, journey, "action=open:path=/\n")
	mustWrite(t, runner, "browser executor\n")
	engine := fixture.engine(&bytes.Buffer{})
	demo := Demo{ID: "web-config", Kind: "browser", Source: "web-config/demo.cjs"}
	before, err := engine.sourceDigest(demo)
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range []struct{ path, content string }{
		{journey, "action=open:path=/show/\n"},
		{runner, "changed browser executor\n"},
	} {
		mustWrite(t, change.path, change.content)
		after, err := engine.sourceDigest(demo)
		if err != nil {
			t.Fatal(err)
		}
		if after == before {
			t.Fatalf("recording remained current after shared input %s changed", change.path)
		}
		before = after
	}
}
