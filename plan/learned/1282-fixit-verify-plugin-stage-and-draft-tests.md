# 1282 -- The Verify Plugin Stage: Nine Reds, and Why Tests Had Nowhere to Be Written

## Context

`make ze-verify` had been red on every `main` run for days (15 consecutive runs
back to 2026-07-25), each with a DIFFERENT set of failing plugin tests. Run
30343424675 failed 9 of 512: `[37, 64, 100, 130, 213, 287, 400, 403, 476]`. The
rotating cast is what made it look unownable -- no single session had caused it,
so no session cleared it, and a genuinely new regression would have hidden under
the existing red.

Two of the nine reproduced deterministically in isolation. Six only under
`scripts/dev/stress-repro.py`. One was environmental on every unprivileged host.

## Decisions

- **Distinguish "not yet up" from "down" in bgp-rs.** `processForward` dropped a whole UPDATE on `!peer.Up`, but `handleOpen` creates the `PeerState`, so `!Up` is also true in the window before the state event lands. `dispatchStructured` has already advanced `seenMsgID`, so `handleStateUp`'s cut excludes the message from the replay too: the route was lost permanently on a healthy session. `PeerState.StateSeen` makes `Up == false` readable, and the guard now means DOWN. The mirror case -- a DESTINATION peer that loses the same race and never receives the WITHDRAW, because `buildReplayRoutes` structurally carries announcements only -- is untouched and stays owned by `plan/spec-fixit-stored-route-relay-hardening.md`.
- **A relayed MP_REACH takes the position of the one it replaces.** `writeRelayPayload` appended it after every surviving attribute, so anything the source placed after MP_REACH (OTC, code 35) ended up in front of it. The live forward relays the source bytes untouched, so the same route left the daemon as two different byte strings depending only on whether the destination was an established forward target yet. RFC 4271 leaves attribute order free, so nothing downstream complains; what it breaks is any consumer comparing the two rails.
- **`ze-test` is built with the daemon's feature tags.** The runner built the helper with a bare `ze_test` while building `ze` from `TestBuildTags()`. `registry.Lookup` then missed every feature-gated plugin, so `ze-test plugin-external as112` exited 1 with "unknown registered plugin" before the refusal the test awaits could be logged. Two build recipes for one binary had drifted; both now derive their feature set from `feature-gates.txt`.
- **A port an RFC fixes cannot be partitioned by unique config.** All 14 BFD tests co-bind `0.0.0.0:3784/3785` under `SO_REUSEPORT`, and the kernel hashes each inbound datagram to one socket in the group, so a reflected echo meant for one daemon reaches a sibling's. They now share `option=exclusive:group=bfd-ports`. Same class as `ddos-flood` and `cos-vlan`, and joins the same ratchet.
- **Tests are drafted in `test/draft/<suite>/` before they are live.** Motivated by the above: every one of these tests had to be edited repeatedly, and each edit was live in a checkout shared with other sessions.

## Consequences

- The plugin stage went from `503/512, failed 9` to `514/514`, and stayed there across three consecutive full-suite runs plus the full gate.
- Six previously load-dependent tests no longer reproduce under `stress-repro.py` at 80-120 stressed invocations; several had reproduced on invocation 1.
- Writing or changing a `.ci` now starts with `test/draft/<suite>/` and `--draft`, enforced by `ai/rules/testing.md` and the `/ze-test` skill.
- `test/draft/` is gitignored AND skipped by six recursive gates. Both, deliberately: the ignore makes the CI guarantee absolute, the skips make local verify usable. `TestDraftDirIsInvisibleToRepoGates` fails if a gate stops skipping.

## Gotchas

- **"Passes in isolation" is a starting point, not a conclusion.** Every one of the six load-dependent failures passed alone. `stress-repro.py` needs `--any-failure` for assertion flakes -- without it only a crash counts as a reproduction and the evidence is discarded.
- **An observer that owns `request shutdown` owns the whole test's clock.** Five separate tests failed because the observer finished its own assertion and tore the daemon down while a peer was still waiting on the wire. `api.quiesce()` is not the barrier: it drains the forward pool and says nothing about a peer that was not yet an established forward target, whose route then arrives on the replay rail. `api.wait_peer_eor_sent()` exists for exactly this and its docstring names the symptom.
- **Budget the polls against the test timeout.** Every `dispatch_until` attempt is a full engine RPC. A 60-attempt poll on a starved daemon outlasted a 20s test budget and turned a run whose wire assertions had ALREADY passed into an opaque timeout. Put `quiesce()` first so the poll is a safety net, not the mechanism.
- **`dispatch_until` returns its LAST result when attempts run out.** `community-strip.ci` waited for `total-routes >= 1` and then guarded with `if total < 0`, so an exhausted poll arrived as `0` and sailed through to an unconditional `print('OK: dest peer wire assertion verified')` for an assertion that had never run.
- **A test can assert an ordering the product does not define.** `mup6.ci` expected announce, EOR, withdraw; whether the plugin's route joins the initial dump (and so precedes the marker) or follows it is scheduling-decided, and both are correct BGP. `eor.ci` was worse: its explicit `update text nlri <family> eor` can NEVER reach the wire, because `sendInitialRoutes` emits one per negotiated family unconditionally and claims the per-family slot RFC 4724 Section 2 allows, so `AnnounceEOR` refuses the explicit one at both `peer.ShouldQueue()` and `!peer.ClaimInitialSyncEOR(fam)`. The file's comment claiming graceful-restart removal prevents the automatic marker was simply false.
- **A guard that returns early must still report.** The `await=stderr` fence's timeout path skipped output collection, so the report showed "expected 0 / received 0" and nothing the daemon had said -- a daemon that died in startup looked identical to one that was merely slow. Fixing the report is what exposed the build-tag drift, after two hours of reading the wrong code.
- **Verify what a red actually belongs to before attributing it.** `test/web/commit-flow.wb` failed the full gate; a clean worktree at HEAD without any of these changes failed it identically. Two other plugin tests (`flowspec`, and this session's `eor`) surfaced only in the full run and were NOT in the reported CI failure set.
- **Suite discovery is a non-recursive glob** (`record_parse.go` `Discover`). That is what makes a subdirectory free to use as an incubator, and `TestDiscoverIgnoresSubdirectories` pins it so a future recursive rewrite fails there rather than through a stranger's red verify.

## Files

None recorded.
