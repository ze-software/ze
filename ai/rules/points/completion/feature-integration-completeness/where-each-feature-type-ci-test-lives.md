---
kind: directive
level: MUST
stage:
---
**Each feature type MUST have its `.ci` test in the directory its row names:**

| Feature Type | `.ci` Location | What the test does |
|-------------|----------------|-------------------|
| Config option | `test/parse/` | Config with option, ze parses without error |
| API/RPC command | `test/plugin/` | Config + peer, send command, verify wire/JSON output |
| Plugin behavior | `test/plugin/` | Config + plugin, trigger behavior, verify effect |
| CLI subcommand | `test/parse/` or `test/ui/` | Run subcommand, verify stdout/stderr/exit code |
| Wire encoding | `test/encode/` | Config with route, verify hex output |
| Wire decoding | `test/decode/` | Hex input, verify JSON output |
