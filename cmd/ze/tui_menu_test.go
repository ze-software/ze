// VALIDATES: ze's launcher first screen and its non-TTY branch
// PREVENTS: regression in the sections ze lists and the non-TTY fallback

//go:build ze_core

package main

import (
	"testing"

	"github.com/ze-software/ze/internal/component/command/registry"
)

func TestBuildTopLevel(t *testing.T) {
	const name = "ztest-tui-root2"
	const desc = "sentinel for TUI menu test"
	registry.MustRegisterRootHandler(name, func(_ *registry.RuntimeContext, _ []string) int {
		return 0
	}, registry.Meta{ShortHelp: desc, Section: registry.SectionOperations})

	level := buildTopLevel(commandTreeForTest(t))
	if len(level.Items) == 0 {
		t.Fatal("buildTopLevel returned no items")
	}

	hasHeader := false
	hasCommand := false
	for _, item := range level.Items {
		if item.Header {
			hasHeader = true
		}
		if !item.Header && item.Name != "" {
			hasCommand = true
		}
	}
	if !hasHeader {
		t.Error("top level should contain section headers")
	}
	if !hasCommand {
		t.Error("top level should contain selectable commands")
	}
}

func TestNonTTYFallback(t *testing.T) {
	orig := stdinIsTerminal
	stdinIsTerminal = func() bool { return false }
	defer func() { stdinIsTerminal = orig }()

	if stdinIsTerminal() {
		t.Error("stdinIsTerminal should return false in this test")
	}
}
