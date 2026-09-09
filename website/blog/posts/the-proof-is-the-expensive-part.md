---
title: The proof is the expensive part
date: 2026-08-06
author: Thomas Mangin
description: I use AI to write Ze, and network change-control instincts to decide what evidence I need before trusting it. The expensive decisions begin with what the standard requires.

deck: Generating a routing feature is quick. Deciding what it owes the standard, whether its tests observe that behaviour and what an operator can rely on takes much longer.

image: assets/blog/the-proof-is-the-expensive-part.svg
image-dark: assets/blog/the-proof-is-the-expensive-part-dark.svg
image-alt: An RFC requirement connects to tagged tests and their runner classification, a partial support declaration with remaining work listed, and a commit snapshot carrying evidence or declared verification debt.

---

When I say that Ze is a network operating system being written with AI, it is easy to imagine a model producing a routing feature, a quick look at the diff and a passing test, then a merge. I would consider that reckless. A routing daemon can handle the usual BGP UPDATEs for a long time before a malformed attribute or an unfortunate sequence of events reveals what the examples never covered.

I use Claude to write code and tests, and to help with fixtures and documentation. Its speed is useful, but it can produce all of those things from the same mistaken premise before the mistake becomes apparent. A convincing explanation beside a convincing implementation gives me very little assurance if they agree for that reason.

In [AI slop is the wrong test](../ai-slop-is-the-wrong-test/), I called the proof the expensive part. This is what I meant: I want a support claim to be traceable to an obligation, to evidence about the behaviour and to the change that evidence belongs to. I have been building Ze's development process around those demands, and deciding whether the connections mean what they claim is where much of my time goes.

*This article was drafted and revised with OpenAI Codex. The ideas, experience and conclusions are mine.*

## Deciding what the code owes

A routing feature often starts with an RFC, and the first difficulty arrives before any implementation. RFCs were written by different people over many years. An obligation may be in a table or a state machine, or in ordinary prose without a capitalised `MUST`; another passage may describe a role Ze does not play or depend on a rule in a different document.

If I start with the part I remember, it is possible to implement it well and still make a false claim about the RFC. Tests for the remembered examples will not find the obligation nobody extracted. I therefore need an explicit account of what Ze thinks it has to satisfy.

Ze's checklists under `rfc/short/` give each requirement an ID. [`RFC7606-7.1-1`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc7606.md) names the first obligation recorded for section 7.1 of RFC 7606. The same name can appear on a test, a known gap or an audit result, so the relationship does not depend on somebody remembering what a test name was meant to cover.

This is also why I found [Cloudflare's engineering standards system](https://blog.cloudflare.com/engineering-standards-enforcement/) interesting. Their standards are internal rather than IETF documents, but they give statements stable names so later checks can refer to the same obligation. I discuss the shared-conventions argument in [AI coding has not had its Rails moment](../ai-coding-has-not-had-its-rails-moment/). For Ze, naming the obligation gives us something precise to disagree about when the interpretation is wrong.

The [requirement model](https://github.com/ze-software/ze/blob/main/internal/le/rfc/rfc.go), in `Requirement.Gated`, distinguishes MUST-level obligations from advisory requirements. `MUST`, `MUST NOT`, `SHALL`, `SHALL NOT` and `REQUIRED` enter that gate; `SHOULD` and `MAY` can be recorded without doing so. I need the interpretation to preserve that difference because a recommendation and a mandatory protocol behaviour are different promises to an operator.

The checklist needs its own review. Ze keeps extraction records under `rfc/extraction/`, and the [sign-off checker](https://github.com/ze-software/ze/blob/main/internal/le/rfc/signoff.go), in `evaluateExtraction`, compares a record with the source inventory in both directions. Each detected passage needs a classification, and each gated checklist requirement needs a source mapping or an explicit account of prose which the inventory did not capture.

That can expose an ignored passage or an unsupported checklist entry. It cannot discover an obligation missed by both the extractor and the reviewer, and a `manual-walk` sign-off records a review which the machine cannot reproduce. I still have to decide whether a reason for excluding a passage is legitimate. The point of the record is to preserve that decision where another reader can challenge it, rather than let it disappear into an apparently complete checklist.

## A bad packet gives the decision an observable result

[RFC 7606 section 7.1](https://www.rfc-editor.org/rfc/rfc7606.html#section-7.1) provides a small example. BGP's ORIGIN attribute is malformed if its length is not one octet or its value is undefined, and an UPDATE carrying it SHALL be handled using treat-as-withdraw. Under [section 2](https://www.rfc-editor.org/rfc/rfc7606.html#section-2), the routes in that UPDATE are treated as withdrawn and removed from those held for the peer.

The distinction is operational. Ignoring the packet could leave an older route installed, while resetting the session would disturb other routes as well. I need evidence for the action the RFC requires, including its limit on how much of the session the error affects.

In [`rfc7606_test.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go), `TestRFC7606MalformedOriginLength` supplies an ORIGIN attribute two octets long to `ValidateUpdateRFC7606` and requires exactly `RFC7606ActionTreatAsWithdraw`. Its tag names `RFC7606-7.1-1` and identifies the case as negative. The [validator's `validateOriginAttr`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606.go) checks both length and value, and other tests supply the defined values and require acceptance.

I want both sides because a router which accepts everything can pass positive examples, and one which rejects everything can pass negative ones. Even a negative assertion can be too generous: accepting a session reset as at least as severe as treat-as-withdraw would permit the implementation to overreact. The exact expected outcome is part of the requirement.

Those tests observe the validator's decision. To find out whether the daemon can continue the exchange after the malformed UPDATE, [`test/plugin/rfc7606-withdraw.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc7606-withdraw.ci) starts Ze and a test peer, sends the bad ORIGIN and expects a later valid announcement over the same connection.

The scenario is useful evidence about session survival, and its own comments explain where that evidence stops. It never reads the routing table, so the later announcement cannot establish that a previously installed route was removed. It therefore does not claim `RFC7606-2-1`, the separate withdrawal obligation. I need that distinction retained even though the filename sounds broad enough to cover both.

This is the expensive reading behind an apparently small feature. The RFC sections are related, the validator and daemon tests are related, and still the assertions do not cover the same behaviour. Adding another requirement tag would improve the appearance of coverage without adding an observation.

## A test also needs somewhere to run

The [generated requirement table](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc7606.md) brings those references together. Its `unit/verify` and `functional/verify` labels tell the reader what kind of test carries the tag and which verification machinery can run it. That is useful when deciding how far the evidence reaches.

I also need the agent to put a new test where it will be used. The [carrier scanner](https://github.com/ze-software/ze/blob/main/internal/le/rfc/carriers.go) derives functional classifications from the verifier's suite list and refuses tags in recognised test formats with no automatic runner. A file which looks like a test must not become public reassurance merely because the model gave it a plausible name.

A runner classification is still different from a run record. Functional verification selects suites according to the change, so a green verification result does not establish that this particular scenario ran. When the claim depends on that scenario, its execution has to be established separately.

There is another distinction between a passing test and one which has been shown to detect its intended defect. Ze's [public RFC ledger](../../quality/rfc-compliance/rfc7606/) separates tags from recorded discrimination evidence: an observed failure after deliberately breaking the behaviour claimed. A tagged unit without that record is labelled `unproven`. At this revision, RFC 7606 has tagged tests but no stored discrimination file, and its extraction sign-off is absent too.

I am describing the process I want the evidence to satisfy, with those missing records visible. A table full of links is useful for finding what needs review; treating it as proof that every review and execution has already happened would undo the reason for building it.

## An admitted gap is useful to an operator

The implementation can also fall short of a correctly extracted requirement. I want that represented explicitly. Some obligations do not apply to Ze, and some permit useful evidence on only one side; a known implementation gap is a different answer and needs to remain visible as such.

RFC 7606 is declared `Partial` in its summary metadata. The summary records a deliberate section 5.1 gap, and that gap is published alongside the support claim. A passing ORIGIN test cannot settle the status of the rest of the document.

The connection to the page is generated. [`RequirementRows`](https://github.com/ze-software/ze/blob/main/internal/le/rfc/render.go) assembles the checklist and test references, and the [site's `collectRequirementLedger`](https://github.com/ze-software/ze/blob/main/internal/le/site/rfcledger.go) uses those same rows alongside the stored audits and proof records. The support wording remains a declaration somebody wrote in the summary; generation keeps it with the evidence and does not decide whether it is deserved.

A missing corner of an RFC may be harmless in one deployment and unacceptable in another. If I hide it behind an unqualified claim of support, the operator inherits a risk which the project already knew about. The ledger is the project's account of its implementation, with enough detail for that operator to disagree. It is no IETF certificate.

## The feedback has to protect the claim

Once tests become part of a public claim, changing them to agree with faulty code can make the implementation and the published evidence wrong together. A model faced with a red test sometimes takes exactly that route. I want a boundary before the edit can turn an obligation into an easier assertion.

Ze's [test-edit process](https://github.com/ze-software/ze/blob/main/ai/rules/testing.md) requires approval for weakening tests and a separate record of user approval for changes to RFC-tagged tests. The native edit hook reads the proposed change through the weakening checker. Structural checks cannot detect every altered meaning, so the record gives the reviewer somewhere to see what was authorised rather than treating the author's justification as sufficient approval.

Generated ledgers need protection against drift as well. `./le rfc index-update` rebuilds the requirement tables from their sources, and `./le rfc check` checks their freshness. Editing a stale table by hand would leave the source of the disagreement intact and lose the correction at the next generation.

These failures make AI useful in a way a general request to improve quality does not. A missing negative case or an unknown requirement ID gives Claude a specific discrepancy to investigate. It can follow the reference, propose a case and run the relevant check. I can then spend the review on whether that case captures the obligation, rather than repeatedly searching for which claim it was supposed to support.

## Evidence belongs to the change it checked

The same discipline reaches the commit. Several agents can share one checkout, and the files which passed a check can differ from those which reach a commit. Another session's staged file can join the wrong change, or a later edit can replace the contents which the author thought were being committed.

The [current commit helper](https://github.com/ze-software/ze/blob/main/docs/contributing/committing.md) prepares a script with a private index and the file contents captured during preparation. The [script's `renderPrivateIndex`](https://github.com/ze-software/ze/blob/main/internal/le/commit/script.go) seeds that index from HEAD and adds those captured blobs. Unrelated staged files cannot join it, and later edits to its named files remain in the working tree.

The verification policy has changed since the original version of this article. A normal local commit no longer requires the whole working tree to match a successful verification run. The helper records failed or missing verification as debt, refuses an authorised push while debt remains open, and requires a recorded reason for structural failures charged to the commit.

I still want the evidence to identify the change it describes. A local commit is not a verified result, and a safe snapshot says nothing about the meaning of a test. These mechanisms keep the file population and the outstanding obligations explicit while the rest of the evidence is being assembled.

## Why I am willing to pay for this

My instinct here comes from network change control. I would not accept a network change because its proposed configuration looked plausible. Lab evidence and staged deployment give us a chance to find failures before they reach the whole network; monitoring and rollback planning deal with the fact that we can still be wrong. Writing the configuration is only part of that job.

I apply the same instinct to generated code. Claude's speed helps with repetitive implementation and with following precise failures back to their causes. I decide what role Ze plays, whether a disclosed gap is acceptable and whether the test reaches the behaviour the design requires. Those decisions can stop a feature which already compiles and passes its author's examples.

The system is unfinished. Extraction reviews are incomplete, semantic audits do not cover everything, and a human can approve the wrong exception. A deliberately broken implementation can also reveal that a test needs replacing, so producing more code may be the last useful thing to do at that point.

I am willing to spend another rebuild and review on that discovery. If the result is a requirement Ze does not yet meet, I want it recorded as missing before an operator relies on the feature. That is a useful outcome even when it leaves me with less to claim at the end of the day.

*Last updated: 9 September 2026. The history of this article is available in the [project's Git repository](https://github.com/ze-software/ze/commits/main/website/blog/posts/the-proof-is-the-expensive-part.md).*
