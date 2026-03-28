# 460 -- Python Plugin Timeouts

## Objective

Fix flaky `check.ci` (test Q) which timed out waiting for an UPDATE event from ze-peer under parallel test load.

## Decisions

- Increased Python plugin event-wait timeout from 5s to 8s in `check.ci` and `summary-format.ci`
- Kept within the 10s overall test timeout while giving headroom for infrastructure delays
- Applied the same fix to `summary-format.ci` which had the identical pattern

## Patterns

- Python plugin scripts in `.ci` tests have their own internal timeouts independent of the ze infrastructure timeouts (`DefaultAPITimeout`, `ze_plugin_stage_timeout`, `defaultDeliveryTimeout`)
- The plugin's event-wait timer starts when `api.ready()` returns, but the UPDATE event only arrives after ze unblocks `WaitForAPIReady()`, establishes the BGP session, receives the UPDATE from ze-peer, and dispatches it through the delivery chain
- Under parallel test load, this chain (ready signal -> peer connect -> OPEN exchange -> UPDATE dispatch -> event RPC) can consume several seconds of the plugin's budget

## Gotchas

- Hardcoded timeouts in Python plugin scripts are invisible to the Go timeout infrastructure -- `ze_plugin_stage_timeout=10s` was already set for test load, but the Python scripts still used 5s
- The plugin's 5s timer runs concurrently with ze's 5s `DefaultAPITimeout` -- in the worst case, ze consumes most of its API timeout before starting peers, leaving the plugin with almost no time to receive the event
- `summary-format.ci` had the same 5s pattern and was silently vulnerable to the same flake

## Files

- `test/plugin/check.ci` -- event timeout 5s -> 8s
- `test/plugin/summary-format.ci` -- event timeout 5s -> 8s
