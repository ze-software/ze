// Design: docs/architecture/system-architecture.md -- ze usage

//go:build ze_core

package main

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
)

// TestRootUsageListsSharedNamesOnce counts command rows from the real root
// printer for the live root/YANG intersection, excluding examples and prose.
// VALIDATES: AC-11 of spec-web-cli-config-one-off-defects.
func TestRootUsageListsSharedNamesOnce(t *testing.T) {
	ensureLocalCommandsRegistered()
	verbs := command.HelpEntries(commandTreeForTest(t), nil)
	rows := rootUsageRows(t)
	shared := 0
	for _, root := range registry.ListRoot() {
		for _, verb := range verbs {
			if root.Name != verb.Name {
				continue
			}
			shared++
			count := 0
			for _, row := range rows {
				if row.name == root.Name {
					count++
				}
			}
			if count != 1 {
				t.Errorf("root/YANG command %q appears in %d command rows, want one", root.Name, count)
			}
		}
	}
	if shared == 0 {
		t.Fatal("the shipped registries contain no shared root/YANG name; duplicate proof is empty")
	}
}

// TestRootUsageRetainsCommandMetadata requires every registered root and every
// distinct YANG verb in the printed page, with the root owner's section and
// summary winning a shared name. Root-only and verb-only controls prevent a
// deduplication fix from dropping either command source wholesale.
func TestRootUsageRetainsCommandMetadata(t *testing.T) {
	ensureLocalCommandsRegistered()
	verbs := command.HelpEntries(commandTreeForTest(t), nil)
	rows := rootUsageRows(t)
	roots := registry.ListRootBySection()
	rootNames := make(map[string]bool)
	rootOnly := 0
	for _, section := range roots {
		for _, root := range section.Commands {
			rootNames[root.Name] = true
			want := rootUsageRow{
				name: root.Name, section: registry.SectionTitle(section.Section), desc: root.Meta.ShortHelp,
			}
			if want.section == "" {
				want.section = section.Section
			}
			requireRootUsageRow(t, rows, want)
			inVerbs := false
			for _, verb := range verbs {
				if verb.Name == root.Name {
					inVerbs = true
				}
			}
			if !inVerbs {
				rootOnly++
			}
		}
	}
	verbOnly := 0
	for _, verb := range verbs {
		if rootNames[verb.Name] {
			continue
		}
		verbOnly++
		requireRootUsageRow(t, rows, rootUsageRow{
			name: verb.Name, section: registry.SectionTitle(registry.SectionOperations), desc: verb.Desc,
		})
	}
	if rootOnly == 0 {
		t.Fatal("the shipped registry contains no root-only command; root retention control is empty")
	}
	if verbOnly == 0 {
		t.Fatal("the shipped tree contains no YANG-only verb; verb retention control is empty")
	}
}

// rootUsageRow is one printed command's unconstrained name, section and summary.
// Its zero value is an empty observation, never an expected registry entry.
type rootUsageRow struct {
	name    string
	section string
	desc    string
}

// rootUsageRows reads the real printer's command sections only. Usage, options
// and examples also name commands, so matching the whole output is not a count
// of entries. The page has no width limit, so one entry occupies one line.
func rootUsageRows(t *testing.T) []rootUsageRow {
	t.Helper()
	t.Setenv("NO_COLOR", "1")
	output := captureStderr(t, func() {
		if err := zeUsage(); err != nil {
			t.Errorf("zeUsage: %v", err)
		}
	})
	commandSections := map[string]bool{
		registry.SectionTitle(registry.SectionOperations): true,
	}
	for _, section := range registry.ListRootBySection() {
		title := registry.SectionTitle(section.Section)
		if title == "" {
			title = section.Section
		}
		commandSections[title] = true
	}
	var rows []rootUsageRow
	var section string
	for line := range strings.SplitSeq(output, "\n") {
		if !strings.HasPrefix(line, " ") {
			if title, found := strings.CutSuffix(line, ":"); found {
				section = title
			}
			continue
		}
		if !commandSections[section] {
			continue
		}
		if !strings.HasPrefix(line, "  ") {
			t.Fatalf("command row has no entry indent: %q", line)
		}
		name, desc, found := strings.Cut(line[2:], " ")
		if !found {
			t.Fatalf("command row has no summary column: %q", line)
		}
		rows = append(rows, rootUsageRow{name: name, section: section, desc: strings.TrimSpace(desc)})
	}
	return rows
}

// requireRootUsageRow compares whole rows so a word appearing in another
// command's description cannot supply a missing command or its owner metadata.
func requireRootUsageRow(t *testing.T, rows []rootUsageRow, want rootUsageRow) {
	t.Helper()
	count := 0
	for _, row := range rows {
		if row == want {
			count++
		}
	}
	if count != 1 {
		t.Errorf("command %q section %q summary %q has %d rows, want one",
			want.name, want.section, want.desc, count)
	}
}
