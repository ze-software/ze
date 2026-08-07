---
kind: table
level:
stage:
---
| Change type | Required functional test | Directory |
|-------------|------------------------|-----------|
| New/changed BGP wire behavior | `.ci` with `expect=bgp:` hex match | `test/encode/` or `test/decode/` |
| New/changed plugin behavior | `.ci` with API commands + expectations | `test/plugin/` |
| New/changed config option | `.ci` with parse success/failure | `test/parse/` |
| New/changed CLI subcommand | `.ci` with `cmd=foreground` + `expect=stdout` | `test/ui/` |
| New/changed web endpoint | `.ci` with HTTP expectations | `test/web/` |
| New/changed editor behavior | `.et` with input/expect directives | `test/editor/` |
| Config reload behavior | `.ci` with `action=sighup` | `test/reload/` |
| Managed/fleet behavior | `.ci` | `test/managed/` |
| Cross-component integration | `.ci` | `test/integration/` |
| Interoperability | `.ci` | `test/interop/` |
