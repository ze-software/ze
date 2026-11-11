---
title: The proof is the expensive part
date: 2026-08-06
author: Thomas Mangin
description: Passing unit tests and CI does not establish that Ze behaves as the protocol requires. Our RFC testing framework connects each requirement to the behaviour its tests observe.

deck: A function can calculate the right BGP hold time while the running daemon uses the wrong one. Ze's RFC testing framework is intended to expose gaps like this.


---

When a unit test calls a function and checks its return value, a passing result tells us that the function produced the answer the test expected. It tells us nothing about whether the running daemon acts on that answer correctly. Ze can therefore pass its unit tests while still behaving incorrectly as an application, and the gap is harder to see when Claude writes both the code and its tests from the same misunderstanding of the protocol.

Running the tests automatically in continuous integration, or CI, repeats the checks whenever the code changes. But CI cannot perform a check that nobody put in the test. Before relying on a routing daemon, I need evidence that the running program responds as the protocol requires when it communicates with another router.

We are building Ze's RFC testing framework to connect each protocol requirement to a test which observes the required behaviour. The difficulty is in establishing what that test must observe. The BGP hold timer gives a simple example of why checking a calculation is not enough to show that the daemon uses its result correctly.

*This article was drafted and revised with OpenAI Codex. The ideas, experience and conclusions are mine.*

## A correct calculation is only the beginning

BGP routers exchange messages to keep a session alive. If one router stops hearing from the other for the agreed hold time, it closes the session. Each router proposes a hold time when the session starts, and [RFC 4271 section 4.2](https://www.rfc-editor.org/rfc/rfc4271.html#section-4.2) says they must use the smaller of the two values.

A unit test could give a function two proposed values and check that it returns the smaller one. That would tell us about the calculation, but the daemon could still start the timer with the wrong value. The unit test would pass even though a silent peer remained connected longer than intended.

Ze's [functional test](https://github.com/ze-software/ze/blob/main/test/plugin/open-hold-time-peer-lower-wins.ci) starts the daemon with a hold time of 180 seconds and connects a peer that proposes three seconds. The peer then stays silent. The test expects Ze to send a hold-timer-expired notification within the test's time limit, well before the 180 seconds Ze proposed.

The functional test observes what the daemon does with the negotiated value, rather than stopping at the result of a calculation. It checks one case: the peer proposes the smaller value and then stops sending messages. Other cases need their own tests, including one where Ze proposes the smaller value.

## Connect tests to requirements

Adding more tests does not necessarily reveal a missing case. We need to start with the behaviour required by the RFC and find the observation that demonstrates it. Ze's [requirement checklists](https://github.com/ze-software/ze/blob/main/rfc/short/rfc4271.md) give each obligation an identifier, which tests cite to state what they check. A reviewer can then compare the assertions with the requirement instead of inferring coverage from the number of passing tests.

An identifier gives a reviewer a route from an RFC requirement to the tests that cite it. The reviewer can then examine what each test sends and observes. The identifier alone cannot establish that a test covers every case the requirement describes, and it must name the right requirement for the link to be useful.

The [public RFC page](../../quality/rfc-compliance/rfc4271/) brings requirements and their test references together so a reviewer can follow an obligation to its tests and inspect what they exercise. Maintaining that connection gives us a way to investigate a support claim: we can see which part of the requirement is asserted and which part still lacks evidence.

A correct assertion is only useful if the test runs. A scenario can remain in the repository after it has been left out of the maintained suites, giving the appearance of coverage without an automatic check. The framework's [scanner](https://github.com/ze-software/ze/blob/main/internal/le/rfc/carriers.go) detects this by rejecting requirement tags in recognised test files which no runner executes.

Having a runner makes the test available, but does not establish that it ran for a particular change. Functional verification selects suites according to the change, so reviewing the result still requires checking which scenarios were executed. The requirement table tells us where the evidence should come from; it cannot replace a test result.

## Check whether the test would notice a bug

Even when the expected result has been observed, we can be wrong about what caused it. In [The repository is half the AI harness](../the-repository-is-the-ai-harness/), I describe route redistribution tests which continued to pass after the intended behaviour was disabled. The route reached the peer through another path, so the test could observe success while the feature it was supposed to protect was broken.

If the test still passes after breaking the behaviour it is meant to check, it cannot be relied on to detect that fault. Its setup or assertions must be corrected before we rely on it. We store the result of this exercise so a reviewer can see whether the test detected that particular fault.

The [public RFC report](../../quality/rfc-compliance/rfc4271/) shows this evidence separately from the requirement cited by the test. That citation states what the test claims to cover, while a failure after the deliberate break shows that the test can detect that mistake. Without the corresponding evidence, that ability remains unproven. Even with it, other mistakes can go undetected, so the report only supports the particular check we performed.

Changes to the test can undermine that evidence too. If Claude changes an expected result to match a defect, the next run can pass without the required behaviour having been restored. Ze's [test-edit process](https://github.com/ze-software/ze/blob/main/ai/rules/testing.md) requires an independent agent to judge changes that could weaken a test, as well as changes to RFC-tagged tests. Since automated checks cannot recognise every change of meaning, the reviewer still has to compare the revised assertion with the requirement.

The public record can also show that evidence is missing. As of 11 September 2026, for example, RFC 7606 had tests citing its requirements but no stored result showing that those tests detected a deliberate fault. There was also no record confirming that the requirement list had been checked against the RFC. Publishing those omissions prevents a reader from mistaking the existence of test references for completed verification.

## The requirements need review too

Reviewing the requirement list against the RFC is necessary because a test can agree perfectly with our checklist while both omit something required by the RFC. An obligation can appear in a table, a state machine or ordinary prose without using the word `MUST`. Identifying it also requires understanding where it applies: some passages concern roles Ze does not perform, and others depend on another RFC.

We record where each checklist requirement came from so another reviewer can inspect those interpretations. An [automated check](https://github.com/ze-software/ze/blob/main/internal/le/rfc/signoff.go) compares these records with passages identified in the RFC text. Each passage must either link to a requirement or have a reason for leaving it out. A passage with neither may contain an obligation we overlooked, and the recorded reasons let a reviewer question our exclusions.

The comparison also runs from the checklist back to the source. Each mandatory requirement must have a source passage or an explanation of why the inventory missed it, giving the reviewer a place to check whether we attributed the requirement correctly. This helps expose an invented obligation as well as a missed one. Recommendations remain distinct from mandatory obligations, so reviewers can assess them according to the requirement level in the RFC.

These records make exclusions available for review alongside the requirements we included. They cannot reveal a requirement missed by both the tool and the reviewer, so recording that someone read the RFC does not establish that they found every requirement. Another reader can still find an omission or disagree with an interpretation.

## Be clear about what we can claim

An incomplete test and an implementation that departs from the standard leave different kinds of uncertainty for an operator. A test might leave a required behaviour unchecked even though the implementation is correct. A documented implementation gap, on the other hand, identifies behaviour that Ze does not yet provide as required.

Ze's [website generator](https://github.com/ze-software/ze/blob/main/internal/le/site/rfcledger.go) publishes the requirement rows alongside stored audits and proof records so someone considering Ze can examine both kinds of gap. The support declaration remains our judgement, and the page carries no IETF certification. Publishing the records gives the reader a basis for assessing that judgement before relying on it.

An unimplemented requirement can be irrelevant in one network and essential in another. Naming the missing behaviour lets an operator assess it against their intended use of Ze. A broad claim of RFC support could leave them to discover the limitation only when they tried to use the feature.

In [AI slop is the wrong test](../ai-slop-is-the-wrong-test/), I argued that I remain responsible for what Claude produces. The RFC framework helps me act on that responsibility by following a claimed feature from the protocol requirement to the behaviour observed in a test. If the test stops at a helper function or omits a required observation, we have a specific gap to address before trusting the application.

Claude can write an implementation much faster than I could, but establishing that the running application behaves correctly takes longer and sometimes requires correcting the tests as well as the code. I expected Ze to be finished by now. A passing test run can hide unfinished verification, and finding those omissions is part of the development still ahead of us.

*Last updated: 21 September 2026. The history of this article is available in the [project's Git repository](https://github.com/ze-software/ze/commits/main/website/blog/posts/the-proof-is-the-expensive-part.md).*
