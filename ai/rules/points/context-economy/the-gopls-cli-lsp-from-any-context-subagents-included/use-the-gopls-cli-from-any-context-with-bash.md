---
kind: directive
level: MUST
stage:
---
- **`gopls` is on PATH (`scripts/le/devtools/tools.py`, `REQUIRED_TOOLS`), so ANY context with Bash reaches the capability, whatever its tool registry holds.** This is the fall-back route of the two above, and it needs no session restart. A context whose `ToolSearch` came back empty MUST run the command instead. It MUST NOT read a whole file to hunt for a symbol, and it MUST NOT report back that it could not look.
- **The measured saving is the CLI's, not only the tool's.** `gopls symbols` output against the whole file: `internal/component/bgp/reactor/peer.go` 5,338 bytes against 48,513 (9.1x), `internal/component/ike/engine/fsm.go` 1,297 against 44,164 (34.1x), `internal/component/bgp/reactor/session_prefix.go` 2,254 against 23,395 (10.4x). One `definition` answered in 705 bytes, doc comment included.
- **The two-step recipe, and the only one you need: `gopls symbols <file>` prints `Name Kind <line>:<col>-<line>:<col>`, and that `<line>:<col>` is exactly what `definition`, `references` and `call_hierarchy` take.** MUST find the symbol in step one, ask about it in step two. MUST NOT guess a position.
- **Positions are 1-based and the column is the START of the identifier, never the start of the line.** A `func` declaration puts its name at column 6.
- **Every invocation starts a fresh server and loads the workspace, so MUST budget seconds, not milliseconds.** Measured warm in this repository: `symbols` 3.3s, `workspace_symbol` 3.7s, `references` 4.1s, `definition` 6.5s. A 60s timeout is generous; MUST NOT paste a multi-minute one.
- **MUST batch them like any other Bash call.** Several independent `gopls` questions belong in ONE message, as with any independent calls.
- **The `gopls mcp` server is not registered.** Headless `gopls mcp` watches every directory under the workspace root and holds one open file descriptor per file because fsnotify uses kqueue on macOS. It honors no directory filter: `skipDir` in `golang.org/x/tools/gopls/internal/filewatcher/fsnotify_watcher.go` skips only names that start with `.` or `_`, and `testdata`. MUST use the LSP tool or the CLI above.
