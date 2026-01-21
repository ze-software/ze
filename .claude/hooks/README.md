# Claude Hooks

Automated actions that run in response to Claude Code events.

## Available Hooks

| Hook | Trigger | Action | Mode |
|------|---------|--------|------|
| `session-start.sh` | SessionStart | Check git status, list active specs | Advisory |
| `compaction-reminder.sh` | UserPromptSubmit | Detect context compaction, remind to re-read specs | Advisory |
| `block-destructive-git.sh` | PreToolUse:Bash | Block dangerous git commands | **Blocking** |
| `block-claude-plans.sh` | PreToolUse:Write | Block `.claude/plans/`, remind about docs | **Blocking** |
| `auto_linter.sh` | PostToolUse:Write\|Edit | Format and lint Go files | Advisory |
| `validate-spec.sh` | PostToolUse:Write\|Edit | Validate spec file format | **Blocking** |

## Hook Details

### session-start.sh
- Shows uncommitted changes count
- Lists active specs in `docs/plan/spec-*.md`
- Reminds to re-read specs after compaction

### compaction-reminder.sh
- Detects context compaction via message pattern
- Lists active specs to re-read
- Points to `## Post-Compaction Recovery` section

### block-destructive-git.sh
- Blocks: `git reset --hard`, `git push --force`, `git clean`
- Requires explicit user approval for destructive operations

### block-claude-plans.sh
- **Blocks** writes to `.claude/plans/` (wrong location)
- **Reminds** about required reading when writing to `docs/plan/spec-*.md`

### auto_linter.sh
- Runs `gofmt -w` to format
- Runs `goimports -w` to organize imports
- Runs `golangci-lint` to check issues

**Requirements:**
- `gofmt` (comes with Go)
- `goimports` (optional): `go install golang.org/x/tools/cmd/goimports@latest`
- `golangci-lint` (optional): https://golangci-lint.run/usage/install/

### validate-spec.sh
- Validates spec files have required sections
- Checks for template compliance

## Hook Configuration

Hooks are configured in `settings.json`:

```json
{
  "hooks": {
    "SessionStart": [...],
    "UserPromptSubmit": [...],
    "PreToolUse": [...],
    "PostToolUse": [...]
  }
}
```

## Exit Codes

| Code | Meaning | Effect |
|------|---------|--------|
| 0 | Success | Continue |
| 1 | Warning | Show message, continue |
| 2 | **Blocking** | Reject operation |

## Creating New Hooks

1. Create script in `.claude/hooks/`
2. Make executable: `chmod +x .claude/hooks/your_hook.sh`
3. Add to `settings.json`

### Hook Input Format

Hooks receive JSON on stdin:

```json
{
  "tool_name": "Write",
  "tool_input": {
    "file_path": "/path/to/file.go",
    "content": "..."
  }
}
```

## Debugging

```bash
echo '{"tool_name":"Write","tool_input":{"file_path":"test.go"}}' | .claude/hooks/auto_linter.sh
echo $?
```
