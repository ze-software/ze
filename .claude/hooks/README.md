# Claude Hooks

Automated actions that run in response to Claude Code tool usage.

## Available Hooks

### auto_linter.sh

**Trigger:** After Write or Edit on `.go` files

**Actions:**
1. Runs `gofmt -w` to format the file
2. Runs `goimports -w` to organize imports (if available)
3. Runs `golangci-lint` to check for issues

**Mode:** Advisory (shows warnings but doesn't block)

**Requirements:**
- `gofmt` (comes with Go)
- `goimports` (optional): `go install golang.org/x/tools/cmd/goimports@latest`
- `golangci-lint` (optional): https://golangci-lint.run/usage/install/

## Hook Configuration

Hooks are configured in `settings.local.json`:

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Write|Edit",
        "hooks": [
          {
            "type": "command",
            "command": "\"$CLAUDE_PROJECT_DIR\"/.claude/hooks/auto_linter.sh",
            "timeout": 60
          }
        ]
      }
    ]
  }
}
```

## Creating New Hooks

1. Create script in `.claude/hooks/`
2. Make executable: `chmod +x .claude/hooks/your_hook.sh`
3. Add to `settings.local.json`

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

### Hook Exit Codes

- `0`: Success (silent)
- `1`: Warning (shown to user, non-blocking)
- Other: Error

## Debugging Hooks

Test manually:

```bash
echo '{"tool_name":"Write","tool_input":{"file_path":"pkg/test.go"}}' | .claude/hooks/auto_linter.sh
```

Check output and exit code:

```bash
echo $?
```
