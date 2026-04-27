package cli

import (
	"fmt"
	"strings"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// TestValidationHintPlacement verifies that "missing field" hints appear on the
// correct line in the diff-annotated display. Specifically:
// - Hints must not land on unrelated leaf values (e.g., "asn remote 65099" for missing "local")
// - Hints must not drift when the tree diff expands inline containers.
func TestValidationHintPlacement(t *testing.T) {
	schema, err := config.YANGSchema()
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	origSP := config.NewSetParser(schema)
	origTree, _, err := origSP.ParseWithMeta("#insecure %2026-03-28T12:40:48Z set environment bgp openwait 140")
	if err != nil {
		t.Fatalf("parse original: %v", err)
	}
	originalContent := config.Serialize(origTree, schema)

	workSP := config.NewSetParser(schema)
	workSP.SetPreMigration(true)
	workTree, _, err := workSP.ParseWithMeta(
		"set environment bgp openwait 140\n" +
			"set bgp peer field-test connection remote ip 10.99.0.2\n" +
			"set bgp peer field-test session asn remote 65099")
	if err != nil {
		t.Fatalf("parse working: %v", err)
	}
	workingContent := config.Serialize(workTree, schema)

	v, err := NewConfigValidator()
	if err != nil {
		t.Fatalf("validator: %v", err)
	}
	result := v.Validate(workingContent)

	var localWarn *ConfigValidationError
	for i := range result.Warnings {
		if strings.Contains(result.Warnings[i].Message, "session/asn/local") {
			localWarn = &result.Warnings[i]
			break
		}
	}
	if localWarn == nil {
		t.Fatal("expected warning about session/asn/local")
	}

	workLines := strings.Split(workingContent, "\n")
	if localWarn.Line < 1 || localWarn.Line > len(workLines) {
		t.Fatalf("warning line %d out of range", localWarn.Line)
	}
	hintLine := strings.TrimSpace(workLines[localWarn.Line-1])

	if strings.Contains(hintLine, "remote") {
		t.Errorf("hint for missing 'local' should not appear on a 'remote' line: %q", hintLine)
	}
	if !strings.HasPrefix(hintLine, "session") {
		t.Errorf("hint should appear on 'session {' container, got: %q", hintLine)
	}

	annotated, lineMapping := annotateContentWithTreeDiff(originalContent, workingContent, schema)
	annotatedLines := strings.Split(annotated, "\n")

	for i := range annotatedLines {
		displayLine := i + 1
		var wl int
		if lineMapping != nil {
			wl = lineMapping[displayLine]
		} else {
			wl = displayLine
		}
		if wl == localWarn.Line {
			trimmed := strings.TrimSpace(annotatedLines[i])
			trimmed = strings.TrimLeft(trimmed, "+ *-")
			trimmed = strings.TrimSpace(trimmed)
			if !strings.HasPrefix(trimmed, "session") {
				t.Errorf("display hint should be on 'session {' line, got: %q", annotatedLines[i])
			}
			if strings.Contains(trimmed, "remote") {
				t.Errorf("display hint should not be on a 'remote' value line: %q", annotatedLines[i])
			}
			fmt.Printf("OK: hint on display line %d: %s\n", displayLine, annotatedLines[i])
			return
		}
	}
	t.Error("hint line not found in display mapping")
}
