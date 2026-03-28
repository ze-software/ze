# Configuration Editor

Ze includes an interactive configuration editor with YANG-driven tab completion, rollback history, and live validation.
<!-- source: cmd/ze/config/cmd_edit.go -- cmdEditWithStorage; internal/component/cli/ -- editor model -->

## Usage

```bash
ze config edit                      # Edit default config
ze config edit myconfig.conf        # Edit specific file
```

The editor starts an ephemeral ze instance in the background for live YANG validation and completion suggestions.

## Editor Commands

| Command | Description |
|---------|-------------|
| `set <path> <value>` | Set a configuration value |
| `delete <path>` | Delete a configuration value or section |
| `show` | Display current configuration |
| `show <path>` | Display a specific section |
| `diff` | Show uncommitted changes |
| `commit` | Save changes and notify daemon |
| `commit confirmed <N>` | Commit with N-second auto-revert window (1-3600) |
| `confirm` | Make a pending confirmed commit permanent |
| `abort` | Roll back a pending confirmed commit immediately |
| `rollback <N>` | Restore revision N |
| `top` | Navigate to config root |
| `up` | Navigate up one level |
| `edit <path>` | Navigate into a section |
| `exit` | Exit editor |
<!-- source: internal/component/cli/editor_commands.go -- editor commands (set, delete, show, diff, commit, rollback) -->

## Other Config Subcommands

| Command | Description |
|---------|-------------|
| `ze config validate <file>` | Validate configuration file |
| `ze config migrate <file>` | Convert ExaBGP config to ze format |
| `ze config fmt <file>` | Normalize formatting (output to stdout) |
| `ze config dump <file>` | Dump parsed config as JSON tree |
| `ze config diff <a> <b>` | Compare two config files |
| `ze config diff <N> <file>` | Compare rollback revision N against current |
| `ze config set <file> <path> <value>` | Set a single value programmatically |
| `ze config history <file>` | List available rollback revisions |
| `ze config rollback <N> <file>` | Restore revision N |
| `ze config archive <name> <file>` | Copy config to named archive |
| `ze config completion <file>` | Query YANG completion engine (debugging) |
<!-- source: cmd/ze/config/main.go -- subcommandHandlers, storageHandlers -->

## YANG Completion

Tab completion is driven by registered YANG schemas. The editor suggests:
- Valid config keys at the current level
- Enum values for leaf nodes
- Address family names from registered plugins

## Commit Confirmed

`commit confirmed <seconds>` writes the configuration and notifies the daemon, but starts a countdown timer. If `confirm` is not issued before the timer expires, the configuration automatically reverts to the previous version. This prevents lockouts when making changes remotely -- if a bad config breaks connectivity, the auto-revert restores access.
<!-- source: internal/component/cli/model_load.go -- cmdCommitConfirmed, handleConfirmCountdown, rollbackConfirmed -->

| Step | What happens |
|------|-------------|
| `commit confirmed 60` | Config saved, daemon notified, 60-second timer starts |
| Verify the change works | BGP sessions come up, routes propagate, etc. |
| `confirm` | Timer stops, config is permanent |
| *or* `abort` | Config reverts immediately |
| *or* timer expires | Config reverts automatically |

The seconds parameter accepts values from 1 to 3600 (one hour).

## Rollback

The editor automatically saves a rollback revision before each commit. Use `ze config history` to list revisions and `ze config rollback <N>` to restore.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Configuration has errors (from `validate` command) |
| 2 | Error (file not found, parse failure) |
<!-- source: cmd/ze/config/main.go -- exitOK, exitError -->

## Example Workflow

```bash
ze config validate config.conf      # Pre-flight validation
ze config edit config.conf          # Interactive editing
ze config diff 3 config.conf       # Compare with revision 3
ze config rollback 3 config.conf   # Restore revision 3
ze config archive prod config.conf # Save a named copy
```
