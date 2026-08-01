# 754: Makefile Split into Focused Includes

## Context
The Makefile had grown to 1,233 lines mixing build, seven testing categories, inventory management, gokrazy appliance building, and kernel compilation. Navigation was difficult, and contributors could not easily find available targets.

## Decision
Split the monolithic Makefile into `mk/` includes organized by function: `test-unit.mk`, `test-functional.mk`, `test-fuzz.mk`, `test-chaos.mk`, `test-integration.mk`, `perf.mk`, `inventory.mk`, `gokrazy.mk`. Core Makefile reduced to 427 lines.

Key choices:
- **Tiered help**: `make help` shows a 20-line quick reference with "Start here" for new contributors. `make help-test`, `help-deploy`, `help-dev` expand each area. Avoids the wall-of-text problem.
- **Component groups**: `ze-test-bgp`, `ze-test-core`, `ze-test-plugins`, `ze-test-config`, `ze-test-cli`, `ze-test-rest` for scoped unit testing. `rest` is the catch-all for everything not in a named group.
- **Missing targets added**: `ze-static-test`, `ze-traffic-test`, `ze-vpp-test`, `ze-l2tp-wire-test` were test suites that existed but had no Makefile entry.
- **Contributor docs**: `docs/contributing/testing.md` with escalation ladder (unit -> functional -> integration -> chaos -> perf) and cheat sheet.

## Consequences
- Each `mk/*.mk` file is self-contained with its own `.PHONY` declarations and group variables.
- Adding a new test category means creating a new `mk/test-*.mk` and one `include` line.
- `docs/functional-tests.md` updated to remove stale claims about missing targets.

## Gotchas
- The `ZE_GROUP_REST` variable uses `go list` with `grep -v` exclusions, so adding a new named group requires adding the exclusion to `rest`.
- `mk/` files cannot define `GO_TEST` or other shared variables; those stay in the root Makefile.

## Files

None recorded.
