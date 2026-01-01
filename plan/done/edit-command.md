# ZeBGP Interactive Config Editor (`zebgp config edit`)

**Status:** Ready for implementation
**Effort:** ~1,500 lines (implementation + tests)
**Approach:** TDD - tests first

---

## Overview

Interactive config editor using Bubble Tea with:
- VyOS-like set commands with hierarchical context
- Inline ghost text completion (grayed-out suggestion)
- Dropdown list when multiple matches
- Schema-driven completions from existing config schema
- File-based editing with backup/rollback

---

## Package Structure

```
pkg/editor/
├── editor.go         # File management, backup, save/discard
├── model.go          # Bubble Tea model
├── completer.go      # Schema-driven completion engine
├── commands.go       # Command handlers
├── view.go           # View rendering
├── backup.go         # Backup/rollback management
├── diff.go           # Diff generation
└── editor_test.go    # Tests

cmd/zebgp/
└── config_edit.go    # Entry point
```

---

## Core Types

```go
// Editor manages the editing session
type Editor struct {
    originalPath string          // Path to config file
    tempPath     string          // Temp working copy
    original     *config.Tree    // Original parsed config
    working      *config.Tree    // Working copy
    schema       *config.Schema
    dirty        bool
}

// Completer provides schema-driven completions
type Completer struct {
    schema  *config.Schema
    tree    *config.Tree
}

// Completion represents a suggestion
type Completion struct {
    Text        string  // The completion text
    Description string  // Help text
    Type        string  // "keyword", "value", "list-key"
}

// Model is the Bubble Tea model
type Model struct {
    editor      *Editor
    completer   *Completer
    textInput   textinput.Model
    contextPath []string         // Current edit context
    isTemplate  bool             // true when editing with wildcard (*)
    template    *config.Tree     // Template for wildcard edits
    completions []Completion
    selected    int              // Dropdown selection
    ghostText   string           // Inline ghost suggestion
    showDropdown bool
    output      []string
    dirty       bool
}
```

---

## Key Behaviors

### Ghost Text Completion
```
User types: set local
Display:    set local-as     (where "-as" is grayed out)
Tab:        Accepts → "set local-as "
```

### Dropdown on Multiple Matches
```
User types: set local
Tab:        Shows dropdown:
            ▸ local-as       ASN for this router
              local-address  Source address for connections
Tab again:  Cycles selection
Enter:      Accepts selected
```

### Context Navigation
```
zebgp# edit neighbor 192.168.1.1
zebgp[neighbor 192.168.1.1]# set peer-as 65001
zebgp[neighbor 192.168.1.1]# set hold-time 90
zebgp[neighbor 192.168.1.1]# top
zebgp#
```

### Wildcard Template (`edit <list> *`) - Inheritance
```
zebgp# edit neighbor *
zebgp[neighbor *]# set hold-time 180
zebgp[neighbor *]# set capability asn4 true
zebgp[neighbor *]# commit
Template saved.
```

`edit <list> *` creates an **inheritance template** stored in config:

```
# In config file:
neighbor * {
    hold-time 180;
    capability { asn4 true; }
}
neighbor 1.1.1.1 {
    peer-as 65001;
    # inherits hold-time 180, capability asn4 true
}
neighbor 2.2.2.2 {
    peer-as 65002;
    hold-time 90;  # overrides template
    # inherits capability asn4 true
}
```

**Inheritance rules:**
- Template (`*`) provides base values for all entries
- Entry-specific values override template
- Template changes propagate to entries without overrides
- `show` displays effective values (merged)

---

## Commands

| Command | Description |
|---------|-------------|
| `set <path> <value>` | Set configuration value |
| `delete <path>` | Delete configuration value |
| `edit <path>` | Enter subsection context |
| `edit <list> *` | Edit template for all list entries |
| `top` | Return to root context |
| `up` | Go up one level |
| `show [section]` | Display configuration |
| `compare` | Show diff vs original |
| `commit` | Save with backup |
| `discard` | Revert all changes |
| `history` | List backup files |
| `rollback <N>` | Restore backup N |
| `exit` | Exit (prompts if dirty) |

---

## Backup Naming Convention

```
<dir>/<name>-YYYY-MM-DD-<N>.conf

Example:
  /etc/zebgp/config.conf           (original)
  /etc/zebgp/config-2025-01-15-1.conf (first backup today)
  /etc/zebgp/config-2025-01-15-2.conf (second backup today)
```

---

## Implementation Phases

### Phase 1: Core Editor
**Files:** `editor.go`, `backup.go`

- `NewEditor(path)` - Load config, create temp copy
- `Save()` - Create backup, write temp to original
- `Discard()` - Revert working to original
- `ListBackups()` - Find backup files matching pattern
- `Rollback(path)` - Restore from backup

**Tests:**
- `TestEditorLoad` - Load and parse config
- `TestEditorSaveCreatesBackup` - Backup created on save
- `TestEditorBackupNaming` - Correct date/number format
- `TestEditorRollback` - Restore previous config

### Phase 2: Completion Engine
**Files:** `completer.go`

- Navigate schema based on input tokens
- Match partial keywords to schema children
- Generate value hints based on ValueType
- Calculate ghost text (single match or common prefix)

**Tests:**
- `TestCompleterSetKeywords` - Complete "set local" → "local-as"
- `TestCompleterNestedPath` - Complete inside neighbor context
- `TestCompleterValueTypes` - Bool shows true/false, IP shows hint
- `TestCompleterGhostText` - Single match shows remainder

### Phase 3: Bubble Tea Model
**Files:** `model.go`, `view.go`

- Text input with ghost text overlay
- Dropdown rendering below input
- Tab: accept ghost or cycle dropdown
- Context-aware prompt display

**Tests:**
- `TestModelGhostAccept` - Tab accepts ghost text
- `TestModelDropdownCycle` - Tab cycles dropdown
- `TestModelContextPrompt` - Prompt shows context path

### Phase 4: Commands
**Files:** `commands.go`, `diff.go`

- Dispatch input to handlers
- Use SetParser for set/delete
- Context navigation (edit/top/up)
- Diff generation for compare

**Tests:**
- `TestCommandSet` - Modifies working tree
- `TestCommandEdit` - Changes context path
- `TestCommandCompare` - Shows correct diff

### Phase 5: Integration
**Files:** `cmd/zebgp/config_edit.go`

- `cmdEdit()` entry point
- Add "edit" case to config subcommand switch

---

## Critical Files

| File | Purpose |
|------|---------|
| `pkg/config/schema.go` | Schema types for completion |
| `pkg/config/setparser.go` | Reuse for set/delete |
| `pkg/config/serialize.go` | For show/diff output |
| `cmd/zebgp/cli.go` | Reference Bubble Tea patterns |
| `cmd/zebgp/config.go` | Add edit subcommand |

---

## Out of Scope (Second Plan)

- API integration for live config updates
- Applying diff to running daemon
- Real-time validation against running state

---

**Created:** 2025-12-20
