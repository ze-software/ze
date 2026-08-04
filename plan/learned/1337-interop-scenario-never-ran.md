# 1337 - an interop scenario that never ran, and the window that ate a Delete

Found 2026-08-04 during the fix of `plan/spec-fixit-ike-request-window.md`. The
spec's BLOCKER was one line of code. The evidence it needed cost more than the fix.

## The defect the spec was open for

`sendDeleteIKE` and `sendDeleteESP` (`internal/component/ike/engine/delete.go`)
returned and sent nothing whenever the request window of RFC 7296 Section 2.3 was
held. Neither ever called `SA.armRequestRetransmit`. Row `RFC7296-2.1-5` [MUST]
asks the initiator to remember each request until its response arrives.

A Delete is the peer's ONLY signal that an SA is gone (Section 1.4). A dropped
one leaves the peer holding an SA Ze destroyed, until its own lifetime ends it.

`writeDelete` now arms the window's retransmit slot, so the owner loop repeats an
unanswered Delete. `serviceRequestWindow` then deems the SA failed once the
repeats run out. That is the second exit the same section names, and Ze took
neither: it freed the window and carried on. A request was forgotten while its
SA still ran.

## The first fix was wrong, and the review is what caught it

The first attempt did two more things. It QUEUED a Delete the window refused, and
it freed the window before the teardown goodbye. Two independent reviewers
refused both, and the RFC text settles it.

<!-- ste: ignore -->
Section 2.3, `rfc/full/rfc7296.txt:1463`: "An IKE endpoint MUST wait for a
response to each of its messages before sending a subsequent message unless it
has received a SET_WINDOW_SIZE Notify message from its peer." **That binds the
SENDER.** `releaseRequestWindow` changes Ze's own bookkeeping. It does not change
what Ze has received, so it cannot make a second unanswered request legal. The
goodbye now WAITS for the answer, bounded at two seconds, and sends nothing when
that answer never comes.

The queue was worse than useless. Every non-test caller of a Delete sender
reaches it with the window free, so no live path can fill it. The one refusal
that remains is an exhausted Message ID space, where the owner loop marks the SA
dead before any drain runs. **Queue fills implies never drains. Queue drains
implies never fills.** It is deleted.

## Traps

- **"Best-effort" in a comment was doing the work of a decision.** The comment
  above the drop said a Delete is best-effort "because the peer removes the old
  Child SA on its own lifetime". That sentence is why the drop survived a review.
  It is its author's belief (`ai/rules/evidence.md`), it names a real mechanism,
  and it still does not authorise missing an enrolled MUST. Ask which requirement
  the label is standing in for.

- **A window mechanism converts every unguarded sender into a droppable one.**
  Adding one shared slot is the correct answer to "two requests went out at once".
  It silently changes what happens to the sender that LOSES the race, and that
  half needs its own answer per sender. Only two answers exist under a window of
  one: wait for the answer, or send nothing. Sending anyway is the violation, and
  a queue is only the first answer with a delay.

- **The reachable case was not the one the finding named.** The ESP Delete's drop
  is unreachable today. Its only caller runs right after the rekey response
  freed the window. The IKE Delete's drop is reached on every operator
  `clear` that lands while a liveness probe is unanswered. Trace each caller
  first, then decide which half of a finding is live. That same trace is what
  showed the queue can never fill.

- **Fixing an RFC MUST by breaking another one is the shape to look for.** The
  first fix traded Section 1.4's goodbye against Section 2.3's wait rule and
  reported it as a fix. Nothing in the change looked like a violation. It deleted
  a `return`, added a queue, and freed a slot. When a fix makes something GO OUT
  that used to be withheld, read the rule that withheld it. Do not assume the
  rule was an oversight.

## The scenario that had never run

`test/ipsec-interop/scenarios/10-clear-reestablish/check.py` carried a note:
"Authored in a parked session that cannot run Docker. Validate at CI." It failed
at its first command, and had failed since the day it was written:

```
ze cli -c "clear vpn ipsec sa"
error: no credentials for 127.0.0.1:2222: no stored username and none supplied
```

Two preconditions were missing, and no scenario in the lab had ever supplied
either. `ze cli` resolves its username from a client-side zefs store that only
`ze init` creates. No `ze.conf` in the lab started an SSH listener.
`lab.py` now carries `ze_cli`, which seeds the store once and passes the password
through `ze.ssh.password`. That is the shape the functional suite uses
(`test/plugin/authz-default.ci`). Scenario 10 passes for the first time.

**A test authored by a session that cannot execute it is not evidence, and it
does not become evidence by being committed.** "Validate at CI" put the run on
nobody. Run it, or say plainly in the report that it is unproven.

## Holding a protocol window on a real wire

The proof needed strongSwan to watch Ze while Ze's own request window was held.
`StrongSwan.break_link` (`lab.py`) drops the peer's OUTBOUND packets in its own
iptables OUTPUT chain, and that is what makes the state constructible. strongSwan
still RECEIVES, so it stays the witness, while its answers never arrive and Ze's
liveness probe holds the window.

The hold is CONFIRMED, never assumed. charon logs `parsed INFORMATIONAL request
N [ ]` for the probe, so a new line after the link broke proves the window is
held.

**Three versions of that scenario were needed, and mutation is what found each
fault.** Version one measured the absence of a rekey to 75 seconds against an
esp-group lifetime of 120. `rekeyLead` (`engine/rekey.go`) is
`max(jitter, min(7*3s, lifetime/2))`, so the soft trigger sits at 99 seconds. Ze
had raised no rekey for a reason that had nothing to do with the window, and
removing `startChildRekey`'s reservation left the scenario GREEN.

Version two fixed the bound and blacked the link out for 104 seconds. That is
long enough for the session to stop and re-establish, and a fresh Child SA
carries a fresh lifetime. Version three uses a 30-second lifetime and a
20-second blackout. The same mutation now reddens it with "Ze sent a
CREATE_CHILD_SA rekey while its own liveness probe was unanswered".

**An absence assertion needs its trigger DERIVED, not estimated.** The number
that made version one vacuous was in a function three files away, and the docstring
that guessed it read plausibly.

## Files

- `internal/component/ike/engine/delete.go`
- `internal/component/ike/engine/established.go`
- `internal/component/ike/engine/msgid.go`
- `internal/component/ike/engine/sa.go`
- `internal/component/ike/engine/rfc7296_window_test.go`
- `internal/component/ike/engine/rfc7296_negotiation_test.go`
- `test/ipsec-interop/lab.py`
- `test/ipsec-interop/scenarios/10-clear-reestablish`
- `test/ipsec-interop/scenarios/24-delete-while-window-held`
- `plan/spec-fixit-ike-request-window.md`
