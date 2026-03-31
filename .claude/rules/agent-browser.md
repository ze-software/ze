---
paths:
  - "internal/component/web/**"
  - "internal/component/lg/**"
  - "internal/chaos/web/**"
---

# Agent Browser

Tool: `agent-browser` (CLI, via Bash). Headless browser automation.

## Workflow

1. `agent-browser open <url>` -- navigate
2. `agent-browser wait --load networkidle` -- wait for page load
3. `agent-browser snapshot -i` -- get interactive elements as accessibility tree with @refs
4. Interact using @refs: `agent-browser click @e2`, `agent-browser fill @e3 "text"`
5. `agent-browser screenshot /tmp/page.png` -- take screenshot, then Read to view
6. `agent-browser close` -- clean up when done

## Key Commands

| Action | Command |
|--------|---------|
| Get page text | `agent-browser get text` |
| Get element text | `agent-browser get text @e1` |
| Screenshot with labels | `agent-browser screenshot --annotate /tmp/page.png` |
| Full page snapshot | `agent-browser snapshot` |
| Interactive only | `agent-browser snapshot -i` |
| Chain commands | Use `&&` to chain in one Bash call |

## Patterns

Chain open + wait + snapshot in one call to reduce round-trips:

```
agent-browser open <url> && agent-browser wait --load networkidle && agent-browser snapshot -i
```

Always `snapshot -i` before interacting (refs change after page mutations).

After `click` or `fill`, re-snapshot to get updated refs.

## Rules

- Close browser when done (`agent-browser close`)
- Never upload sensitive project data to external sites
- Use `--annotate` screenshots when visual context helps
- Use `snapshot -i` (interactive only) over full `snapshot` for cleaner output
