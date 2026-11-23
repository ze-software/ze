---
spec: plugin-external-yang-at-startup
topic: Plugins
feature: plugins
---
## Before

External plugins could send a YANG module in their Stage 1 declaration, but Ze stored the text without joining it to the daemon's configuration schema or command tree. Any configuration block under an external plugin's own root failed with an unknown top-level keyword before the plugin started, external plugins could not define their own CLI commands or argument checks in YANG, and running `ze schema` attempted to execute external binaries with `--yang`.

## After

External plugins written in Go or any other language can now declare their own `-conf`, `-cmd`, and `-api` YANG modules in their Stage 1 `schemas` list. At `ze start`, Ze reads declared external plugins first, collects their Stage 1 modules before parsing the full configuration, joins and publishes them once for the daemon's lifetime, and delivers each plugin's configuration section at start and on commit. External plugin commands appear in CLI completion, help, pipes, shell forwarding (`ze <verb> ...`), and MCP, REST, and gRPC parameter schemas with full YANG argument validation before the plugin handler runs. Invalid, clashing, or unheld-registration modules refuse startup and report clear doctor diagnostics (`doctor-plugin-schema-refused`, `doctor-plugin-schema-undeclared`, `doctor-plugin-schema-changed`), reloads that change external plugin declarations are refused until `ze` restarts, and offline commands never execute external plugin programs — instead asking a running `ze` first for `validate`, `fmt`, `dump`, `diff`, and `graph`, or reporting external sections as unavailable with `ze start` as the fix.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
