// VALIDATES: the le root resolves local-data handlers and preserves payloads and codes.
// PREVENTS: standalone and tagged dispatch diverging or flattening a tool verdict.
package leroot

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	saved := os.Stdout
	os.Stdout = writer
	done := make(chan string, 1)
	go func() {
		var buffer bytes.Buffer
		_, _ = buffer.ReadFrom(reader)
		done <- buffer.String()
	}()
	fn()
	_ = writer.Close()
	os.Stdout = saved
	out := <-done
	_ = reader.Close()
	return out
}

func TestRegisterShapeDistinguishesFullToolPaths(t *testing.T) {
	const docName = "shape-doc-discriminator"
	const mapName = "shape-map-discriminator"
	RegisterShape(docName, command.ShapeDoc)
	RegisterShape(mapName, command.ShapeMap)

	docShape, docDeclared := command.ShapeForCommand(CommandPath(docName))
	mapShape, mapDeclared := command.ShapeForCommand(CommandPath(mapName))
	if !docDeclared || docShape != command.ShapeDoc {
		t.Errorf("%s shape = %v/%v, want doc/declared", docName, docShape, docDeclared)
	}
	if !mapDeclared || mapShape != command.ShapeMap {
		t.Errorf("%s shape = %v/%v, want map/declared", mapName, mapShape, mapDeclared)
	}
	if shape, declared := command.ShapeForCommand("le"); declared {
		t.Errorf("root le inherited tool shape %v", shape)
	}
}

func TestDispatchReachesRegisteredLocalDataAndPreservesNonzeroPayload(t *testing.T) {
	const name = "dispatch-local-data-probe"
	var got []string
	Register(name, GroupReport, func(args []string) (any, int) {
		got = args
		return map[string]any{"probe": "ran", "code": 3}, 3
	}, registry.Meta{Description: "a test probe", Mode: "offline", Section: registry.SectionTest})
	RegisterShape(name, command.ShapeDoc)

	code := 0
	out := captureStdout(t, func() { code = Dispatch("le", []string{name, "alpha", "beta", "|", "json"}) })
	if code != 3 {
		t.Errorf("Dispatch answered %d, want the handler's 3", code)
	}
	if strings.Join(got, ",") != "alpha,beta" {
		t.Errorf("handler arguments = %q, want [alpha beta]", got)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("nonzero payload is not JSON: %v\n%s", err, out)
	}
	if payload["probe"] != "ran" || payload["code"] != float64(3) {
		t.Errorf("rendered payload = %#v", payload)
	}
}

func TestDispatchUsesSharedPipeRenderers(t *testing.T) {
	const name = "pipe-local-data-probe"
	Register(name, GroupReport, func([]string) (any, int) {
		return map[string]any{
			"actions":        2,
			"native-actions": []string{"tier/check", "repository/check"},
		}, 0
	}, registry.Meta{Description: "a test probe", Mode: "offline", Section: registry.SectionTest})
	RegisterShape(name, command.ShapeMap)

	for _, format := range []string{"json", "yaml", "table"} {
		t.Run(format, func(t *testing.T) {
			code := 0
			out := captureStdout(t, func() { code = Dispatch("le", []string{name, "|", format}) })
			if code != 0 {
				t.Fatalf("%s rendering answered %d: %s", format, code, out)
			}
			if !strings.Contains(out, "actions") || !strings.Contains(out, "tier/check") {
				t.Errorf("%s rendering dropped structured data: %q", format, out)
			}
		})
	}
	out := captureStdout(t, func() {
		if code := Dispatch("le", []string{name, "|", "match", "tier"}); code != 0 {
			t.Errorf("match rendering answered %d", code)
		}
	})
	if !strings.Contains(out, "tier/check") {
		t.Errorf("match rendering dropped the matching row: %q", out)
	}
}

func TestDispatchRefusesTwoFormatOperators(t *testing.T) {
	const name = "pipe-refusal-local-data-probe"
	Register(name, GroupReport, func([]string) (any, int) {
		return map[string]string{"probe": "ran"}, 0
	}, registry.Meta{Description: "a test probe", Mode: "offline", Section: registry.SectionTest})
	RegisterShape(name, command.ShapeDoc)

	code := 0
	stdout := captureStdout(t, func() {
		captureStderr(t, func() {
			code = Dispatch("le", []string{name, "|", "json", "|", "yaml"})
		})
	})
	if code != 1 {
		t.Errorf("two format operators answered %d, want 1", code)
	}
	if stdout != "" {
		t.Errorf("refused pipe chain wrote stdout: %q", stdout)
	}
}

func TestDispatchRefusesUnknownAndHandlesHelp(t *testing.T) {
	if code := Dispatch("le", []string{"no-such-tool"}); code != 1 {
		t.Errorf("unknown tool answered %d, want 1", code)
	}
	if code := Dispatch("le", nil); code != 1 {
		t.Errorf("empty invocation answered %d, want 1", code)
	}
	if code := Dispatch("le", []string{"--help"}); code != 0 {
		t.Errorf("help answered %d, want 0", code)
	}
}

func TestHelpRendersTheNodeWithoutRunningIt(t *testing.T) {
	const name = "help-page-local-data-probe"
	ran := 0
	Register(name, GroupReport, func([]string) (any, int) {
		ran++
		return map[string]string{"probe": "ran"}, 0
	}, registry.Meta{
		Description: "a probe that must not run for a help word",
		Mode:        "offline", Section: registry.SectionTest,
		Subs: "alpha | beta (writes)",
	})
	RegisterShape(name, command.ShapeDoc)

	for _, args := range [][]string{{name, "--help"}, {name, "-h"}, {name, "help"}, {"help", name}} {
		code := 0
		page := captureStderr(t, func() { code = Dispatch("le", args) })
		if code != 0 {
			t.Errorf("%v answered %d, want 0", args, code)
		}
		for _, want := range []string{"le " + name, "must not run", "alpha", "beta (writes)"} {
			if !strings.Contains(page, want) {
				t.Errorf("%v page missing %q: %s", args, want, page)
			}
		}
	}
	if ran != 0 {
		t.Errorf("a help word ran the command %d time(s)", ran)
	}
}

func TestHelpNamesWhatANamespaceHolds(t *testing.T) {
	const member = "help-namespace-probe member"
	Register(member, GroupReport, func([]string) (any, int) { return nil, 0 },
		registry.Meta{Description: "the member's own summary", Mode: "offline", Section: registry.SectionTest})
	RegisterShape(member, command.ShapeDoc)

	code := 0
	page := captureStderr(t, func() { code = Dispatch("le", []string{"help-namespace-probe", "--help"}) })
	if code != 0 {
		t.Errorf("namespace help answered %d, want 0", code)
	}
	for _, want := range []string{"member", "the member's own summary"} {
		if !strings.Contains(page, want) {
			t.Errorf("namespace page missing %q: %s", want, page)
		}
	}
}
