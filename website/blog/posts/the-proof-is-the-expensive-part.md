---
title: The proof is the expensive part
date: 2026-08-06
author: Thomas Mangin
description: Passing unit tests and CI does not establish that Ze behaves as the protocol requires. Our RFC testing framework connects each requirement to the behaviour its tests observe.

deck: A test can correctly detect a malformed BGP message without checking what happens to the affected route. Ze’s RFC testing framework is intended to expose gaps like this.


---

When a unit test calls a function and checks its return value, a passing result tells us that the function produced the answer the test expected. It tells us nothing about whether the running daemon acts on that answer correctly. Ze can therefore pass its unit tests while still behaving incorrectly as an application, and the gap is harder to see when Claude writes both the code and its tests from the same misunderstanding of the protocol.

Running the tests automatically in continuous integration, or CI, helps us repeat the checks whenever the code changes. But an assertion missing from a test is still missing when CI runs it. Before relying on a routing daemon, I need evidence that the running program responds as the protocol requires, including when another router sends it something malformed.

We are building Ze's RFC testing framework to connect each protocol requirement to a test which observes the required behaviour. The difficulty is in establishing what that test must observe. Our tests for a malformed BGP attribute illustrate how we can check the detection of an error, and even run the daemon, while still leaving part of its response untested.

*This article was drafted and revised with OpenAI Codex. The ideas, experience and conclusions are mine.*

## Detecting an error is only the beginning

A BGP UPDATE carries information about routes, including an ORIGIN attribute. According to [RFC 7606 section 7.1](https://www.rfc-editor.org/rfc/rfc7606.html#section-7.1), an ORIGIN with a length other than one octet or an undefined value is malformed, and the receiver must treat the affected routes as withdrawn. As [section 2](https://www.rfc-editor.org/rfc/rfc7606.html#section-2) explains, that means removing those routes if they were previously received from the peer.

Simply ignoring the malformed UPDATE would leave the old route installed, even though the new information about it is unusable. Closing the connection would remove the affected route along with the peer's other routes. A test of the required behaviour must distinguish it from both mistakes, which means observing what happens to the route as well as the connection.

Ze's [unit test](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go) begins with the detection of the error: it supplies a two-octet ORIGIN attribute to the validator and checks that it returns `RFC7606ActionTreatAsWithdraw`. Other tests supply valid values and expect the validator to accept them.

Both kinds of input matter. A validator which classified every ORIGIN as malformed would pass the bad-length test, while one which accepted every message would pass the valid-input tests. Checking valid and invalid cases together establishes whether the validator distinguishes them for the inputs we supplied, but the observation still stops at its returned action.

To observe more of the response, Ze's [functional scenario](https://github.com/ze-software/ze/blob/main/test/plugin/rfc7606-withdraw.ci) starts the daemon and a peer, sends a malformed ORIGIN, then expects a valid announcement over the same connection. Receiving that announcement establishes that the connection survived. The scenario does not inspect the routing table, however, so it cannot establish that Ze removed an affected route which was already installed.

The unit test checks the returned action and the functional test checks that the connection survives, leaving the route-removal requirement between them unchecked. Completing the test requires establishing that a route was present before the malformed UPDATE and absent afterwards. Without those observations, both tests can pass while the application retains a route it was required to remove.

## Make the missing check visible

Adding more tests does not necessarily reveal an omission like this. We need to start with the behaviour required by the RFC and find the observation that demonstrates it. Ze's [requirement checklists](https://github.com/ze-software/ze/blob/main/rfc/short/rfc7606.md) give each obligation an identifier, which tests cite to state what they check. A reviewer can then compare the assertions with the requirement instead of inferring coverage from the number of passing tests.

In the ORIGIN example, the validator test cites `RFC7606-7.1-1`, the malformed-attribute rule. The functional scenario's comments acknowledge that it does not observe route removal, and it does not cite `RFC7606-2-1`, the separate withdrawal obligation. The withdrawal requirement remains visible without a claim that this scenario verifies it. Adding its identifier to the scenario would conceal the gap without adding the missing observation.

The [generated requirement table](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc7606.md) collects these references so a reviewer can follow an obligation to its tests and inspect what they exercise. Maintaining that connection gives us a way to investigate a support claim: we can see which part of the requirement is asserted and which part still lacks evidence.

A correct assertion is only useful if the test runs. A scenario can remain in the repository after it has been left out of the maintained suites, giving the appearance of coverage without an automatic check. The framework's [scanner](https://github.com/ze-software/ze/blob/main/internal/le/rfc/carriers.go) detects this by rejecting requirement tags in recognised test files which no runner executes.

Having a runner makes the test available, but does not establish that it ran for a particular change. Functional verification selects suites according to the change, so reviewing the result still requires checking which scenarios were executed. The requirement table tells us where the evidence should come from; it cannot replace a test result.

## Check whether the test would notice a bug

Even when the expected result has been observed, we can be wrong about what caused it. In [The repository is half the AI harness](../the-repository-is-the-ai-harness/), I describe route redistribution tests which continued to pass after the intended behaviour was disabled. The route reached the peer through another path, so the test could observe success while the feature it was supposed to protect was broken.

We can expose that weakness by deliberately breaking the behaviour the test claims to check and running it again. A test which still passes cannot demonstrate that the feature works; its setup or assertions must be corrected before we rely on it. Ze calls the stored result of this exercise discrimination evidence, because it records whether the test distinguishes the working implementation from that particular broken one.

The [public RFC ledger](../../quality/rfc-compliance/rfc7606/) reports this evidence separately from a requirement tag. A tag states what a test claims to cover, while a failure after the deliberate break shows that the test can detect that mistake. Without the corresponding evidence, the tagged test is labelled `unproven`. Even with it, other mistakes can go undetected, so the report only supports the particular check we performed.

Changes to the test can undermine that evidence too. If Claude changes an expected result to match a defect, the next run can pass without the required behaviour having been restored. Ze's [test-edit process](https://github.com/ze-software/ze/blob/main/ai/rules/testing.md) requires approval for weakening tests, including a separate user-approval record for RFC-tagged tests. Since automated checks cannot recognise every change of meaning, the reviewer still has to compare the revised assertion with the requirement.

As of 11 September 2026, RFC 7606 had tagged tests but no stored discrimination file. Its extraction sign-off, the record of how the requirements were checked against the RFC, was absent too. Publishing those omissions prevents a reader from mistaking the existence of tagged tests for completed verification.

## The requirements need review too

The extraction review is necessary because a test can agree perfectly with our checklist while both omit something required by the RFC. An obligation can appear in a table, a state machine or ordinary prose without using the word `MUST`. Identifying it also requires understanding where it applies: some passages concern roles Ze does not perform, and others depend on another RFC.

We record how each checklist was extracted so another reviewer can inspect those interpretations. The [sign-off checker](https://github.com/ze-software/ze/blob/main/internal/le/rfc/signoff.go) compares the extraction records with an inventory of detected passages, requiring a classification for each passage. A passage left unclassified can indicate an obligation we overlooked, while an exclusion gives the reviewer a reason to inspect.

The comparison also runs from the checklist back to the source. Each mandatory requirement must have a source passage or an explanation of why the inventory missed it, giving the reviewer a place to check whether we attributed the requirement correctly. This helps expose an invented obligation as well as a missed one. Recommendations remain distinct from mandatory obligations, so reviewers can assess them according to the requirement level in the RFC.

These records make exclusions available for review alongside the requirements we included. They cannot reveal a requirement missed by both the tool and the person reading the RFC, which is why a `manual-walk` sign-off remains a record of someone's review. Another reader can still find an omission or disagree with an interpretation.

## Be clear about what we can claim

An incomplete test and an implementation that departs from the standard leave different kinds of uncertainty for an operator. In the ORIGIN example, an observation is missing from the test. RFC 7606 also has a documented implementation gap: Ze's generated announcements place MP_REACH_NLRI, the attribute carrying multiprotocol routes, after attributes such as ORIGIN, although section 5.1 requires it to come first. The RFC summary therefore declares support `Partial`. Passing the ORIGIN tests cannot resolve this separate difference in how messages are constructed.

Ze's [website generator](https://github.com/ze-software/ze/blob/main/internal/le/site/rfcledger.go) publishes the requirement rows alongside stored audits and proof records so someone considering Ze can examine both kinds of gap. The support declaration remains our judgement, and the page carries no IETF certification. Publishing the records gives the reader a basis for assessing that judgement before relying on it.

An unimplemented requirement can be irrelevant in one network and essential in another. Naming the missing behaviour lets an operator assess it against their intended use of Ze. A broad claim of RFC support could leave them to discover the limitation only when they tried to use the feature.

In [AI slop is the wrong test](../ai-slop-is-the-wrong-test/), I argued that I remain responsible for what Claude produces. The RFC framework helps me act on that responsibility by following a claimed feature from the protocol requirement to the behaviour observed in a test. If the test stops at a helper function or omits a required observation, we have a specific gap to address before trusting the application.

Claude can write an implementation much faster than I could, but establishing that the running application behaves correctly takes longer and sometimes requires correcting the tests as well as the code. I expected Ze to be finished by now. A passing test run can hide unfinished verification such as the missing route observation in this example, and finding those omissions is part of the development still ahead of us.

*Last updated: 21 September 2026. The history of this article is available in the [project's Git repository](https://github.com/ze-software/ze/commits/main/website/blog/posts/the-proof-is-the-expensive-part.md).*
