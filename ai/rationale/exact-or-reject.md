# Rationale: Exact Or Reject

Short rule: `ai/rules/exact-or-reject.md`.

## Why

Ze's configuration is the operator's contract with the system. If the operator
writes `qdisc htb { class fast { rate 10mbit ceil 20mbit } }`, they expect a
policer running at 10 Mbps CIR with 20 Mbps EIR. They do NOT expect a "best
guess" that the backend picked because HTB doesn't quite match VPP's native
types. A silent approximation is worse than a rejection because:

1. **The operator cannot tell whether their QoS is working.** The config
   parses, the daemon applies, no errors appear. But the policer at 8 Mbps
   CIR (because the backend silently rounded down to a supported step) is
   not what they asked for. Debugging this requires reading the source.
2. **The system lies to its own operator.** Commit says "applied"; reality
   says "approximated". The trust contract is broken the first time this
   happens, and every config review from then on must assume the daemon is
   silently reinterpreting inputs.
3. **Approximations compound.** Backend A approximates one way; later work
   depends on that approximation; backend B approximates differently; the
   two backends produce measurably different behavior for the same config.
   Two backends, two silent divergences, no operator-visible signal.

Rejection, in contrast, is a loud, specific signal: "your config asks for
something I cannot do; here is exactly what's wrong". The operator fixes it
or picks a different backend. The invariant "config applied == config
active" holds.

## Real incidents in this repo

These are failures this rule was written to prevent. Each one was caught
in review, AFTER being written, proving the rule is needed as an up-front
constraint.

### `egressMapFromPrioClasses` (fw-7, 2026-04-18)
Initial translation of prio qdisc to VPP QoS egress maps used
`Outputs[class_index] = class.Priority`. The mapping has no
operator-facing semantics (class index is arbitrary; DSCP input value
is semantic) -- so the backend was going to apply a map that bore no
resemblance to what `qdisc prio` means. The translation function also
had `if i >= 256 { break }` which silently discarded classes beyond the
256th -- two layers of silent mis-behavior in one function.

Fix: prio rejected at verify; the translation skeleton retained for a
future spec returns an error on the 256+ case rather than discarding.

### `applyFilter` DSCP path (fw-7, 2026-04-18)
Every DSCP filter built a fresh `QosEgressMapUpdate` with zeroed rows,
added its single entry, and pushed. `QosEgressMapUpdate` replaces the
whole map, so the second filter on the same interface blanked the first.
Two DSCP filters on one interface = only the last one programmed. The
code comment even said "A future optimization can batch them" -- silent
correctness bug disguised as an optimization opportunity.

Fix: accumulate all per-interface DSCP entries into one `QosEgressMap`
and push once per interface.

### Policer name truncation (fw-7, 2026-04-18)
VPP policer names are `string[64]`. The backend composes
`ze/<iface>/<class>` and truncates on overflow. Two classes with long
names sharing a 64-char prefix would produce the same policer name; the
second `PolicerAddDel` would silently upsert the first.

Fix: verifier rejects class names that would produce a name longer than
64 chars, with a message telling the operator to shorten the interface
or class name.

## What "exactly" means

"Exactly" means the backend state, when applied, matches every operator-
visible property of the config. Not:
- A state that is "close" in a metric the operator did not define.
- A state that shares some properties with the desired state.
- A state that is "good enough for testing" or "good enough for MVP".

If the backend cannot deliver the exact state, the path is: reject at
verify, record a deferral in `plan/deferrals.md` pointing at a concrete
receiving spec, let the operator choose a different backend or wait for
the deferred work.

## When exactness is truly unattainable (rare)

Sometimes the operator-visible property is, by design, probabilistic or
approximate (e.g. a fair-queue scheduler is inherently non-deterministic;
a token bucket's burst depends on clock precision). In those cases the
YANG semantics themselves must admit the approximation -- document it in
the leaf's description, reject configs that ask for unsupported degrees
of precision, and make the allowed approximation the contract.

The rule of thumb: if the operator, reading the config and the YANG
description, could reasonably expect behavior X but gets behavior Y,
there is a silent approximation. Fix it.
