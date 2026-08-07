---
kind: note
level:
stage:
---
`mk/test-functional.mk` builds an ISOLATED, BARE-NAMED pair into
`$(ZE_ALT_BIN)`, and the daemon it builds carries the **`zetest`** build tag
(`ze_core ze_distro ze_setup zetest $(ZE_FEATURES)`). That same makefile then runs
the suite as `env ZE_TEST_NO_BUILD=1 ZE_BIN=$(ZE_ALT_BIN)/ze ZE_TEST_BIN=$(ZE_ALT_BIN)/ze-test
$(ZE_ALT_BIN)/ze-test ...`. Launched directly the runner rebuilds a ze WITHOUT
`zetest`, so a test needing a zetest-only surface times out as
`server likely failed to start or crashed` -- naming none of this.
