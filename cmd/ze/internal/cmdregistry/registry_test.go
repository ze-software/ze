package cmdregistry

import (
	"testing"
)

func TestListRootBySection(t *testing.T) {
	ResetForTest()
	defer ResetForTest()

	RegisterRoot("cli", Meta{Description: "CLI", Section: SectionOperations})
	RegisterRoot("config", Meta{Description: "Config", Section: SectionConfiguration})
	RegisterRoot("doctor", Meta{Description: "Doctor", Section: SectionSystem})
	RegisterRoot("schema", Meta{Description: "Schema", Section: SectionConfiguration})
	RegisterRoot("orphan", Meta{Description: "No section set"})

	sections := ListRootBySection()

	if len(sections) != 3 {
		t.Fatalf("expected 3 sections, got %d", len(sections))
	}

	// Verify order: operations, configuration, system.
	wantOrder := []string{SectionOperations, SectionConfiguration, SectionSystem}
	for i, se := range sections {
		if se.Section != wantOrder[i] {
			t.Errorf("section %d: want %q, got %q", i, wantOrder[i], se.Section)
		}
	}

	// Operations has 1 command.
	if len(sections[0].Commands) != 1 || sections[0].Commands[0].Name != "cli" {
		t.Errorf("operations: want [cli], got %v", sections[0].Commands)
	}

	// Configuration has 2 commands, sorted.
	if len(sections[1].Commands) != 2 {
		t.Fatalf("configuration: want 2 commands, got %d", len(sections[1].Commands))
	}
	if sections[1].Commands[0].Name != "config" || sections[1].Commands[1].Name != "schema" {
		t.Errorf("configuration: want [config, schema], got [%s, %s]",
			sections[1].Commands[0].Name, sections[1].Commands[1].Name)
	}

	// System has doctor + orphan (empty section defaults to system).
	if len(sections[2].Commands) != 2 {
		t.Fatalf("system: want 2 commands, got %d", len(sections[2].Commands))
	}
	names := map[string]bool{}
	for _, c := range sections[2].Commands {
		names[c.Name] = true
	}
	if !names["doctor"] || !names["orphan"] {
		t.Errorf("system: want doctor+orphan, got %v", sections[2].Commands)
	}
}

func TestSectionTitle(t *testing.T) {
	if got := SectionTitle(SectionOperations); got == "" {
		t.Error("SectionTitle(SectionOperations) returned empty")
	}
	if got := SectionTitle("nonexistent"); got != "" {
		t.Errorf("SectionTitle(nonexistent) = %q, want empty", got)
	}
}
