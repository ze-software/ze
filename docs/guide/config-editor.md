# Configuration Editor

Ze includes an interactive configuration editor with YANG-driven tab completion, rollback history, and live validation.

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
| `rollback <N>` | Restore revision N |
| `top` | Navigate to config root |
| `up` | Navigate up one level |
| `edit <path>` | Navigate into a section |
| `exit` | Exit editor |

## Other Config Subcommands

| Command | Description |
|---------|-------------|
| `ze config check <file>` | Validate config, check for deprecated patterns |
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

## YANG Completion

Tab completion is driven by registered YANG schemas. The editor suggests:
- Valid config keys at the current level
- Enum values for leaf nodes
- Address family names from registered plugins

## Rollback

The editor automatically saves a rollback revision before each commit. Use `ze config history` to list revisions and `ze config rollback <N>` to restore.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Migration needed (from `check` command) |
| 2 | Error (file not found, parse failure) |

## Example Workflow

```bash
ze config check config.conf         # Pre-flight validation
ze config edit config.conf          # Interactive editing
ze config diff 3 config.conf       # Compare with revision 3
ze config rollback 3 config.conf   # Restore revision 3
ze config archive prod config.conf # Save a named copy
```
