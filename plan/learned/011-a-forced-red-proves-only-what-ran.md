# Learned: a forced red proves only the assertion that ran

`test/policy/policy-interface-list.ci` was written against code that already
worked, so it owed a forced red. The walk broke the right producer, observed a
red, and recorded it. The red proved one of the file's assertions and nothing
about the two that carry the claim.

## The break has to reach the assertion, and the config decides whether it does

The test asserts that a policy naming `eth0` and `l2tp*` installs two nftables
rules rather than one rule asking for both names. Two matches in one rule and
the same two matches in two rules print the same substrings, so a whole-table
dump cannot tell them apart. What separates the shapes is the negative
assertion: `reject=stdout:pattern=RULE .*eth0.*l2tp`, on a single per-rule line
carrying both names.

The walk reverted `ruleTerms` (`internal/plugins/policyroute/translate.go`) to
the shape that merges the interface matches into one term. That is the right
producer. The `.ci` then configured ONE policy rule, and the fixture
`policyRuleDump` (`internal/test/fixture/netfilter_fixture_policy.go`) waits for
at least two rules in `ze_pr` before it prints anything. The merged shape
programs one kernel rule for one policy rule, so the fixture timed out at
"policy rules were not programmed" and the run died before any output
assertion ran.

The recorded red was real, and it was a red for the wrong reason. It said the
fixture's precondition was unmet. It said nothing about whether either rejection
can fire, so a typo in either pattern would have left the file permanently
green.

**Read the step trace of the red run and name which assertions it reached.** The
verdict line says the file went red. It does not say which claim was tested. The
repair here was to the test's CONFIG rather than to its assertions: two policy
rules, at `order 10` and `order 0`, so the merged shape still programs two
kernel rules and the rejections are reached. The same config gave the ordering
criterion its kernel proof, which a unit test had been carrying alone.

## One break cannot exhibit several reds

Three breaks were needed for one file, and the count came from the assertion
set rather than from the producer.

The runner evaluates every `expect=stdout:pattern` before the first
`reject=stdout:pattern`, so a break that fails a positive assertion stops there
and the rejections stay unreached. The first break therefore reddened the
four-rule sequence assertion, and the second break was the same merge with the
sequence assertion commented out, so the run travelled far enough to trip the
first rejection.

The two rejections are mirrors of each other. For one leaf-list order, exactly
one of them can match, so proving the second needed a third break that appends
the interfaces the other way round.

**Count the breaks from the assertions, not from the producers.** Ask, for each
assertion that carries a claim, what state of the product would make this one
fail, and whether any earlier assertion stops the run before it.

## An assertion no break can redden is a claim with no proof

The file carried a fourth rejection, on a tcp `RULE` line preceding a udp one.
No break turned it red, and the positive sequence assertion already carried the
same claim. It was REMOVED rather than kept.

An assertion that cannot fail is not a stricter test. It is a sentence in the
file that a reader counts as coverage and that discriminates nothing, and it
costs the next session the time to work out which of the two it is. Where a
surviving assertion carries the claim, delete the one that cannot fail. Where
none does, the claim is unproven and the work is not done.

## Where the rest of this spec's lessons went

Only the transferable half is here. The mechanism went to the surfaces that
govern it, per `ai/rules/planning.md`:

- One term for each named interface, and why separate rules are the only OR
  nftables has: `docs/architecture/policyroute/policy-routing.md`.
- What an operator reads back, and why the counter in each row is chain-wide:
  `docs/guide/policy-routing.md`.
- The counter sitting ahead of the match it is named for:
  `plan/journal/counter-counts-the-wrong-packets.md`.
- One owner's unusable table failing `ApplyAll` for every owner:
  `plan/journal/one-members-bad-input-fails-every-member.md`.
- This file's own class:
  `plan/journal/green-that-could-not-have-been-red.md`, row of 2026-09-09.
