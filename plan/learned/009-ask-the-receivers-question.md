# Learned: an attribute pair is correct only when read the way its receiver reads it

The `as-path-prepend` policy action wrote its AS_SEQUENCE segment with
four-octet AS numbers unconditionally and spliced it in front of a payload
encoded at whatever the session negotiated. On a two-octet session that is not
one decodable AS_PATH. Fixing the width was the spec. Fixing what goes beside it
took two more rounds, and that is where the transferable part is.

## An invariant over each attribute separately proves nothing

RFC 6793 splits one fact, the AS path, across two attributes. AS_PATH carries it
at the session's width with AS_TRANS standing in for what does not fit; AS4_PATH
carries the real four-octet numbers. The receiver's rule, Section 4.2.3, takes
as many AS numbers from the leading part of AS_PATH as make the two counts
equal, then prepends them to AS4_PATH.

The first fix prepended the local AS numbers to BOTH attributes. Stated over the
attributes separately, everything held: the right AS numbers, in the right
order, and the AS number count difference preserved. The spec's AC-4 asserted
exactly that difference, and the code satisfied it.

It is wrong, because prepending to both leaves the difference unchanged, and the
receiver spends that unchanged budget on the head of the AS_PATH ze just wrote,
where the AS_TRANS placeholders now sit. Measured: AS_PATH `[64500, 23456,
64496]` with AS4_PATH `[4200000002, 64496]`, prepend 4200000001, and the peer
reconstructs `[23456, 4200000001, 4200000002, 64496]`. AS_TRANS at the head, and
AS 64500 gone.

**When a fact is split across two artifacts, the assertion belongs on the
recombination, never on either half.** The test that caught it does not compare
bytes to expected bytes. It runs the two attributes ze emits through
`attribute.MergeAS4Path`, which is ze's declaration of the receiver's rule, and
compares the result to the path ze meant to send.

## The rule was already written down, on the other side

`attribute.MergeAS4Path` existed before this work, on the RECEIVE path, where
`reconstructASPath` (`internal/component/bgp/plugins/rib/storage/attrparse.go`)
uses it to intern an arriving route. The send side had its own hand-rolled
derivation for the same question, and only the hand-rolled one was wrong.

The fix was two lines: hand the sender's path to the receiver's function. It
also deleted a parameter, because the merge reads the prepended AS numbers out
of the path itself.

**Before writing a rule about how a peer will read what you send, look for the
code that reads what a peer sent.** It is the same rule, and the copy you are
about to write is the one that will be wrong.

## A wiring test that supplies its own input tests nothing

The spec's Wiring Test table named three rows, each meant to prove that a call
site passes the width of the payload it edits. Two of the three tests called the
extractor directly with a width the test itself chose. They passed. They would
have gone on passing with a literal `true` hard-coded at either call site, which
is the exact defect the spec exists to prevent.

The repair is not a stronger assertion. It is a different entry point: the tests
now call `runIngressPolicyChain` and `runEgressPolicyChainASN4`, let the chain
read its own width, and parse what comes back. Both polarities run, because a
single-polarity proof is red under a literal `true` and green under a literal
`false`.

**A wiring test is defined by what it does not supply.** Anything the test hands
in is not being tested, whatever the assertion says.

## A gate verdict of "killed" is a verdict about the machine

The interop scenario for this spec was written on 2026-09-06 and could not run:
three attempts died to the kernel's OOM killer, and the record blamed the
container fleet. It ran on the first try on 2026-09-08 after colima was resized
from 2 CPUs and 1.9 GiB to 8 and 24, on a host that had 64 GiB the whole time.
Nothing about the lab changed.

`docker info` reports the VM's allocation, not the host's, and the two differ by
more than an order of magnitude on a laptop running colima. The journal row that
recorded the earlier failure quoted 32 CPUs and 31.34 GiB, which was a different
machine.

**Before attributing a kill to the workload, read the number the workload
actually gets.** `plan/journal/gate-verdict-depends-on-the-machine.md` is where
this class of finding lives, and it now has one more instance where the cost was
two days of a scenario being treated as unrunnable.

## The peer's strictness is not the peer's bug, and it is not yours either

Once it ran, the scenario failed on its own `frr.conf`. Two FRR behaviours, both
correct from FRR's side and both invisible from the documentation:

- `dont-capability-negotiate` suppresses only the capabilities FRR SENDS. FRR
  still reads ze's four-octet AS capability, resolves the peer AS to the real
  number, and answers a `remote-as` that disagrees with `2/2 (OPEN Message
  Error/Bad Peer AS)`. Configuring `remote-as 23456` because "that is what the
  peer sees in My AS" never establishes.
- `enforce-first-as` is on by default in FRR 10.3 and compares the leading AS of
  the received AS_PATH with the configured peer AS. A two-octet AS_PATH from a
  non-mappable AS leads with AS_TRANS, exactly as RFC 6793 Section 4.2.2
  requires, and FRR withdraws the route.

Neither is ze being wrong: `(*Session).buildOpen`
(`internal/component/bgp/reactor/session_negotiate.go`) writes AS_TRANS in My AS
and the real ASN in the capability, which is the RFC's own instruction.

**A scenario's peer configuration is a claim about the peer, and it needs the
same evidence as a claim about ze.** Both wrong lines here were written from a
plausible reading of what the peer would do, and both comments in the file
asserted that reading. The comments now carry the measurement instead.
