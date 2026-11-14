---
title: The proof is the expensive part
date: 2026-08-06
author: Thomas Mangin
description: Ze aims to meet every applicable mandatory RFC requirement. Interoperability labs and tests tied to the RFC text help establish what the running router does and which claims remain unverified.

deck: Tests can pass while the router still does the wrong thing.

---

AI can write a feature and get its first tests passing quickly. If the code and its tests have both been written and reviewed by agents, someone considering the software still needs a reason to trust it. Establishing that the running router does what the protocol requires takes much longer.

*This article was drafted and revised with OpenAI Codex. The ideas, experience and conclusions are mine.*

## The objective is RFC compliance

Ze is the network operating system I am building. I review its structure and overall implementation, while relying on agents for much of the detailed code and test review. The basis for trusting their output has to be something an operator can examine independently.

Our goal is to satisfy every applicable mandatory requirement in the protocol specifications Ze implements. These are published as Requests for Comments, or RFCs, and include `MUST` and `MUST NOT` obligations which define how the router must behave. Establishing that behaviour is part of deciding whether Ze is ready to depend on in a network.

We are building an RFC testing framework to connect each obligation to tests which observe the required behaviour. That is ongoing work, and a claim of support needs to show which requirements have been checked and where evidence is still missing.

## Work with other implementations

Ze has to exchange routes correctly with software people already use. Our [interoperability labs](https://github.com/ze-software/ze/blob/main/docs/architecture/testing/interop.md) run Ze alongside implementations such as FRR, BIRD and GoBGP, with checks on what those peers receive. This exercises both ends of a protocol exchange with independently developed software.

A session reaching the established state is only the beginning. The tests must check the routes exchanged and the effects of the particular feature being exercised. A peer can accept an UPDATE while discarding an attribute Ze should never have sent, so even successful route delivery can leave a protocol defect hidden.

## Observe the exchange in the lab

The lab gives us a controlled setup in which to arrange those exchanges and inspect their results. For example, `LOCAL_PREF` expresses a route's preference within an autonomous system, and normally has to be removed when a route is sent to an external peer.

In a [recorded lab run on 10 November 2026](https://github.com/ze-software/ze/blob/b2b6a36ef0/internal/le/interoplab/bgp/testdata/bgp-local-pref-strip-gobgp/bgp-local-pref-strip-gobgp-UIYSVYU5KP45REAV3CAV7EZJQQ.jsonl), Ze received three routes carrying `LOCAL_PREF` 200 from an internal peer. All three reached the external FRR and GoBGP destinations, and the test observed that both sessions remained established. It also inspected Ze's outgoing messages towards GoBGP through the scenario's declared observation interval and checked that `LOCAL_PREF` was absent.

The wire observation matters because an external receiver is required to ignore `LOCAL_PREF`. Looking only at a receiver's stored routes could therefore hide a sender's mistake. The [lab checker](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go) checks successful delivery as well as the transmitted bytes, so dropping every route cannot satisfy the test either.

Establishing that result required a setup which exercised forwarding after the destination sessions were established, together with observations of both delivered routes and transmitted bytes. If the setup never reaches that forwarding path, or the test looks only at the receiver's stored routes, a passing result leaves the sender's behaviour unchecked. Finding such an omission sends us back to the setup or assertions before the test can tell us whether the implementation is correct.

## Derive the checks from the RFC

Choosing those observations requires reading the protocol's exact requirements. [RFC 4271 section 5.1.5](https://www.rfc-editor.org/rfc/rfc4271.html#section-5.1.5) states:

> A BGP speaker MUST NOT include this attribute in UPDATE messages it sends to external peers, except in the case of BGP Confederations [RFC3065].

That sentence gives the LOCAL_PREF wire check its purpose, including an exception which matters when deciding where it applies. Adding tests only for the examples we happen to think of can leave another required case undiscovered. Ze's [requirement checklists](https://github.com/ze-software/ze/blob/main/rfc/short/rfc4271.md) therefore start from the RFC's obligations and give each one an identifier. A test cites that identifier and states the particular behaviour it checks, so another reviewer can compare its assertions with the requirement.

A test can agree perfectly with our checklist while both omit something required by the RFC. An obligation can appear in a table, a state machine or ordinary prose without using the word `MUST`. Some passages concern roles Ze does not perform, and others depend on another RFC, so the checklist itself needs review against the source text.

We record which passages justify each requirement and why we exclude others. An [automated check](https://github.com/ze-software/ze/blob/main/internal/le/rfc/signoff.go) flags passages left unaccounted for and checklist requirements with no recorded basis. A reviewer still has to read the RFC and decide whether the interpretations and exclusions are justified.

Recommendations remain distinct from mandatory obligations, because recording a recommendation as mandatory would misrepresent the RFC too. The records make those decisions available for someone else to challenge, but cannot reveal an obligation missed by both the tool and the reviewer.

## Cover the whole required response

The required observations can extend beyond whether a session survives. [RFC 7606 section 7.1](https://www.rfc-editor.org/rfc/rfc7606.html#section-7.1) says a BGP UPDATE with a malformed ORIGIN attribute must receive treat-as-withdraw handling. Under [section 2](https://www.rfc-editor.org/rfc/rfc7606.html#section-2), the routes in that UPDATE are treated as withdrawn, removing them from the routes received from that peer.

One of Ze's [functional scenarios](https://github.com/ze-software/ze/blob/main/test/plugin/rfc7606-withdraw.ci) starts the daemon and a peer, sends an UPDATE with a two-octet ORIGIN where one octet is required, then expects a later valid announcement over the same connection. That announcement checks that the session survives. Ignoring the malformed UPDATE would also keep the session alive, so the later announcement alone cannot distinguish that mistake from correct treat-as-withdraw handling.

The scenario does not inspect the receiver's stored routes before or after the malformed UPDATE. Its comments acknowledge that limit, and it does not claim to establish that a previously installed route was removed. Separate [component tests](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7606_rib_structured_test.go) install a route and assert that the malformed re-announcement removes it, but they drive component handlers directly. Establishing the same effect through the complete daemon requires observing route removal there too.

## Check whether the test would notice a bug

A test can observe the expected result for the wrong reason. As I describe in [The repository is half the AI harness](../the-repository-is-the-ai-harness/), a route can arrive through an unintended path and keep a test passing after the feature under test has been disabled.

Ze's [fault-detection checks](https://github.com/ze-software/ze/blob/main/internal/le/rfc/discriminate_observe.go) test this by first requiring the test to pass, then deliberately breaking the producing code and requiring that same test to fail. A build failure or an unrelated failing test cannot supply the required observation. If the selected test still passes, either the break did not affect the claimed behaviour or the test failed to notice it, and no successful fault-detection record is written.

The LOCAL_PREF experiment included that second step. The [10 November record](https://github.com/ze-software/ze/commit/22d430d39b) reports a successful lab run, followed by a failure after the code suppressing `LOCAL_PREF` towards external peers was deliberately disabled. The wire check reported that the attribute had appeared on the actual external destination connection. The record supports the observation for the versions tested and shows one specific fault the test could detect.

The public report keeps that record separate from the requirement citation. The citation says what the test claims to check; the recorded failure shows that it detected a particular deliberately introduced mistake. Other mistakes can still pass unnoticed. This is the practical meaning of proof here, with its scope limited to the observation the record contains.

## Preserve what the evidence established

An old observation can become misleading when the code or test changes. Ze's [stored records](https://github.com/ze-software/ze/blob/main/internal/le/rfc/discriminate.go) identify the test and producing code, together with the claim being checked, and store fingerprints of those inputs. Changes can then be detected, so an old result is not presented as evidence for a different version. Matching fingerprints do not rerun the experiment.

The [test-edit rules](https://github.com/ze-software/ze/blob/main/ai/rules/testing.md) forbid agents from changing expected results, removing assertions or skipping tests simply to obtain a passing run. If an agent believes a test itself is wrong, it must explain the defect and obtain independent approval before changing its expectations. Changes which could weaken a test require that review, and changes to RFC-tagged tests must be checked against the RFC's actual text.

An independent reviewer is another agent which did not write the change, ideally starting with a fresh context. It reads the old and proposed tests, the code they exercise and the relevant requirement, alongside the author's reason for the change. It must decide whether the expectation was wrong or whether the change would conceal a defect. The author cannot approve its own explanation.

Automated checks cannot recognise every change of meaning, and the reviewer can share the author's misunderstanding. The rules give us a way to challenge a convenient passing result against the original requirement.

## Be clear about what we can claim

A missing check and an implementation gap leave different kinds of uncertainty for an operator. Ze might behave correctly without a test observing it. A documented implementation gap identifies behaviour that Ze does not yet provide as required, and can rule it out for a network which needs that behaviour.

Leaving a known gap undisclosed moves the risk to the operator, who may discover the limitation only when trying to use the feature. Naming the missing behaviour gives them a chance to assess it before depending on Ze in their network.

Ze's [public RFC reports](../../quality/rfc-compliance/rfc4271/) publish requirement rows alongside stored reviews and fault-detection records. Someone considering Ze can examine what has been claimed, which tests are cited and what evidence accompanies them. These remain Ze's own records, with no IETF certification, and publishing them does not turn incomplete verification into a guarantee.

Where a test is listed without a recorded fault-detection result, we have not yet established that it catches the mistake it is meant to detect. Where the requirement list has not been checked against the RFC, we may also have missed obligations entirely. Publishing those gaps makes clear what still needs checking before we can claim coverage.

In [AI slop is human slop](../ai-slop-is-the-wrong-test/), I argued that I remain responsible for what Claude produces. I still review the structure and overall implementation, while relying on the development process to provide suitable tests and independent checks of their design. Connecting a requirement to an observed behaviour, then checking that the test can detect a fault, gives that reliance a basis beyond the author's confidence in its own code.

## Smaller tests give faster feedback

Broader scenarios take more setup and time, so we also need smaller checks while developing. Ze has many unit tests, fewer [integration and functional tests](https://github.com/ze-software/ze/blob/main/docs/functional-tests.md), and fewer still of the complete network lab scenarios. Unit tests let Claude and Codex check a small part of the code quickly, while integration tests exercise components together. Retaining cases at each level helps detect behavioural regressions as the project changes.

The BGP hold timer illustrates their different scopes. Routers use it to detect a peer which has stopped sending messages; with a non-zero hold time, expiry requires a notification and session closure. [RFC 4271 section 4.2](https://www.rfc-editor.org/rfc/rfc4271.html#section-4.2) requires the smaller of the two routers' proposed hold times to be used. A unit test can check that the negotiation code selects that value, while a test of the daemon can check what happens when the peer goes silent.

Ze's [functional test](https://github.com/ze-software/ze/blob/main/test/plugin/open-hold-time-peer-lower-wins.ci) proposes 180 seconds, connects a peer proposing three seconds and expects the exact bytes of a hold-timer-expired notification within a 20-second budget. That can expose a daemon using its own longer value even when the negotiation function passes its tests. The test does not separately assert connection closure or expiry at exactly three seconds, and the opposite negotiation case needs its own test.

Continuous integration, or CI, automates repeated runs of these checks. Ze's [GitHub workflow](https://github.com/ze-software/ze/blob/main/.github/workflows/verify.yml) runs checks on pushes and pull requests, while [scheduled jobs](https://github.com/ze-software/ze/blob/main/.github/workflows/evidence-nightly.yml) run further suites, including interoperability labs. The framework's [scanner](https://github.com/ze-software/ze/blob/main/internal/le/rfc/carriers.go) rejects requirement tags in recognised test files which have no automatic runner. That establishes where a test can run; the results for a particular change still have to show that it ran.

Claude can write an implementation much faster than I could, but establishing that the application behaves correctly takes longer. Changes to the implementation, test setup or checker can require us to repeat that investigation before relying on the result.

I expected Ze to be finished by now. Finding what a test failed to observe can send us back to its setup and assertions before it can tell us whether the implementation works, and correcting those checks is part of the development work still to do.

*Originally written: 6 August 2026. Last updated: 13 November 2026. The history of this article is available in the [project's Git repository](https://github.com/ze-software/ze/commits/main/website/blog/posts/the-proof-is-the-expensive-part.md).*
