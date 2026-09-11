---
title: The proof is the expensive part
date: 2026-08-06
author: Thomas Mangin
description: Claude can produce a routing feature quickly. My network change-control instincts make me spend much longer establishing what it must do, and whether anything has demonstrated that it does it.

deck: A malformed BGP attribute is a small example of why I spend so much time on proof, even when the implementation is already written and its tests are green.

image: assets/blog/the-proof-is-the-expensive-part.svg
image-dark: assets/blog/the-proof-is-the-expensive-part-dark.svg
image-alt: An RFC requirement connects to tagged tests and their runner classification, a partial support declaration with remaining work listed, and a commit snapshot carrying evidence or declared verification debt.

---

Working in networking gives you a fairly poor appetite for changes which look as though they should work. A proposed configuration can look quite reasonable, and I still expect lab evidence and a staged deployment before it goes across the network. Monitoring and a rollback plan account for what we may have missed anyway, so writing the configuration is only part of the job.

Building Ze, the network operating system I am developing with AI, brings the same problem into software development. Claude can produce an implementation and its tests from one mistaken interpretation, then explain convincingly why they agree. A routing daemon may process ordinary BGP UPDATEs for a long time before a malformed attribute or an unfortunate sequence exposes what they both missed.

In [AI slop is the wrong test](../ai-slop-is-the-wrong-test/), I called the proof the expensive part. Claude's speed is useful, and I have no desire to give it up, but much of my time goes on establishing whether we have tested what the protocol requires at all. Following one malformed BGP attribute from the RFC to a public support claim shows how much remains to be decided after the code is written.

*This article was drafted and revised with OpenAI Codex. The ideas, experience and conclusions are mine.*

## Before writing the first test

An RFC is a less convenient specification than its number suggests. Different people wrote the documents over many years, an obligation can be in a table or state machine, and ordinary prose can require behaviour without a capitalised `MUST` in sight. Some passages concern a role Ze does not play; others depend on a rule explained in another document.

Starting with the part I remember would be a good way to implement that part correctly and make an unjustified claim about the rest. Ze therefore keeps requirement checklists under `rfc/short/`, with an ID for each obligation. [`RFC7606-7.1-1`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc7606.md), for example, names the first obligation recorded for section 7.1 of RFC 7606. A test, a known gap or an audit result can refer to it, so a disagreement about the interpretation at least concerns the same passage.

Stable names serve a similar purpose in [Cloudflare's engineering standards system](https://blog.cloudflare.com/engineering-standards-enforcement/), although their standards are internal rather than IETF documents. The shared-conventions part belongs in [AI coding has not had its Rails moment](../ai-coding-has-not-had-its-rails-moment/). Here, naming an obligation lets the next reviewer find what the test's author meant to cover, without reconstructing that intention from the implementation.

The obligation's level has to survive that process too. In Ze's [requirement model](https://github.com/ze-software/ze/blob/main/internal/le/rfc/rfc.go), `MUST`, `MUST NOT`, `SHALL`, `SHALL NOT` and `REQUIRED` enter the mandatory gate. `SHOULD` and `MAY` can be recorded without entering it. Treating a recommendation as mandatory changes the promise we make just as surely as overlooking a mandatory behaviour does.

A name and a level can still describe a mistaken reading, so extraction records under `rfc/extraction/` account for how the checklist was derived. The [sign-off checker](https://github.com/ze-software/ze/blob/main/internal/le/rfc/signoff.go) compares a record with the source inventory in both directions: detected passages need a classification, and gated requirements need a source mapping or an explanation of prose the inventory did not capture.

This makes an unclassified passage or an unsupported checklist entry visible to a later check. An obligation missed by both the extractor and the reviewer remains missed, and a `manual-walk` sign-off records a reading the machine cannot reproduce. Somebody still has to judge whether an exclusion makes sense, but keeping its reason beside the checklist gives the next reviewer something to challenge.

## One bad ORIGIN attribute

[RFC 7606 section 7.1](https://www.rfc-editor.org/rfc/rfc7606.html#section-7.1) gives a small enough example to follow. A BGP ORIGIN attribute is malformed when its length is not one octet or its value is undefined. The UPDATE carrying it SHALL receive treat-as-withdraw handling, which [section 2](https://www.rfc-editor.org/rfc/rfc7606.html#section-2) defines as treating the routes in that UPDATE as withdrawn and removing them from those held for the peer.

Ignoring the packet could leave an older route installed, while resetting the session would remove it along with other routes which did nothing wrong. The RFC's chosen outcome has to be distinguished from both, so noticing an error is an inadequate assertion on its own.

In [`rfc7606_test.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go), `TestRFC7606MalformedOriginLength` supplies an ORIGIN two octets long to `ValidateUpdateRFC7606` and requires exactly `RFC7606ActionTreatAsWithdraw`. It carries the `RFC7606-7.1-1` tag as a negative case. The [validator](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606.go) checks length and value, while other tests supply the defined values and require acceptance.

Those positive cases would catch an implementation which rejected everything; the negative case catches acceptance of the malformed attribute. Requiring the exact action also prevents a session reset from passing as a sufficiently severe response. Each assertion has a particular wrong implementation to distinguish from the required one.

The daemon then has to act on the validator's decision without destroying the session. [`test/plugin/rfc7606-withdraw.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc7606-withdraw.ci) starts Ze and a peer, sends the bad ORIGIN, then expects a valid announcement over the same connection. That observes session survival, but the scenario never reads the routing table, so the later announcement cannot establish that a previously installed route was removed.

The scenario's comments acknowledge this limit, and it does not claim `RFC7606-2-1`, the separate withdrawal obligation. Adding that tag would improve the apparent coverage without observing anything new. For an attribute only one octet long when valid, we have already had to separate the validator's decision from what the daemon does with it, then separate session survival from route removal.

## A test file is a start

Once a test has an appropriate assertion, it still needs a route into a run. The [generated requirement table](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc7606.md) gathers the tags and labels them with classifications such as `unit/verify` and `functional/verify`. An arriving agent can find the existing examples there, and a reader can see what kind of test was found and which verification machinery can run it.

That classification comes from the [carrier scanner](https://github.com/ze-software/ze/blob/main/internal/le/rfc/carriers.go), which uses the verifier's functional suite list and refuses tags in recognised test formats with no automatic runner. This prevents a fine-looking scenario which no normal check executes from contributing the same reassurance as a runnable one. Functional verification selects suites according to the change, however, so classification alone cannot establish that this scenario ran for a particular change; the run itself has to show that.

Execution leaves a further question, described in [The repository is half the AI harness](../the-repository-is-the-ai-harness/): a test can run and pass, then stay green when the behaviour it claims to protect is deliberately broken. Ze's [public RFC ledger](../../quality/rfc-compliance/rfc7606/) therefore keeps the tags separate from stored discrimination evidence, meaning an observed failure after breaking the claimed behaviour. A tagged unit without that record is labelled `unproven`.

At this revision, RFC 7606 has tagged tests but no stored discrimination file, and its extraction sign-off is absent too. The table gives us a way to find tests and missing evidence; it does not mean those reviews and experiments have already happened.

## Partial means there is something missing

Missing evidence also needs to be distinguished from missing behaviour. Some requirements do not apply to Ze, and some admit useful evidence on only one side, but neither explanation accounts for an implementation gap. RFC 7606 is declared `Partial` in its summary metadata, with a deliberate section 5.1 gap published beside the support statement. Passing the ORIGIN tests cannot settle that part of the RFC.

The repository's [requirement rows](https://github.com/ze-software/ze/blob/main/internal/le/rfc/render.go) feed the [site ledger producer](https://github.com/ze-software/ze/blob/main/internal/le/site/rfcledger.go), alongside stored audits and proof records. The public support wording comes from the authored summary, so generation keeps the declaration with the material behind it without deciding whether it is deserved. These pages remain Ze's account of Ze and carry no IETF certification.

For an operator, the missing corner may be irrelevant to one deployment and rule out another. Hiding it transfers a risk to the person deploying the software without giving them the information to assess it. I would much rather they find the omission on the support page than through a routing failure.

## Keeping the claim tied to its test

A public claim depends on that relationship surviving later changes. If a model responds to a failure by editing the test to agree with broken code, it damages the support statement as well as the test. Its explanation can be quite convincing unless the reviewer returns to the obligation the assertion was meant to enforce.

The [test-edit process](https://github.com/ze-software/ze/blob/main/ai/rules/testing.md) requires approval for weakening tests and a separate record of user approval for RFC-tagged tests. The native edit hook passes the proposed change through the weakening checker. Structural checks miss changes of meaning, but the approval record gives the reviewer somewhere to establish what was authorised; the agent's justification does not authorise itself.

The same discipline applies to generated tables: a hand edit lasts only until the next generation. `./le rfc index-update` rebuilds them from their sources and `./le rfc check` checks their freshness, so a discrepancy needs correcting at its source.

These specific failures are useful instructions for Claude. An unknown requirement ID or a missing negative case gives it a reference to find and a concrete problem to investigate, instead of a general request to improve quality. It can propose a case and run the relevant check while I concentrate on whether the case exercises the obligation. Finding what the author was trying to prove no longer has to occupy the first part of every review.

## Committing the files which were checked

Even when the review and test are useful, several agents sharing a checkout can break the connection between them and a commit. One session can stage a file for another's commit, or edit a named file after the commit was prepared. The eventual commit may then contain code which the earlier check never saw.

The [current commit helper](https://github.com/ze-software/ze/blob/main/docs/contributing/committing.md) captures file contents during preparation and writes a script which commits them through a private index. The [script](https://github.com/ze-software/ze/blob/main/internal/le/commit/script.go) seeds that index from HEAD and adds the captured blobs, so unrelated staged files stay out and later edits to the named files stay in the working tree.

The policy around verification has changed since the original version of this article. A normal local commit no longer requires the whole working tree to match a successful verification run. Failed or missing verification is recorded as debt, an authorised push is refused while that debt remains open, and structural failures charged to the commit require a recorded reason.

Local commits can therefore preserve a known set of contents while verification remains unfinished and visible. Capturing the files does not judge their tests, but it prevents a careful review of one set of contents from ending in a commit of another.

## The part I cannot delegate away

Claude can write an implementation faster than I could, and it can follow a precise failure through repetitive code without my having to do every step. I still choose what role Ze plays and which compromises I am prepared to make. A feature may compile and pass its author's examples before a review of the RFC reveals that those examples answered the wrong problem.

In network change control, a failed lab exercise is a reason to revise the change before deployment. The same discovery in generated code may call for replacing the test which let it through before asking for another implementation. Extraction reviews and semantic audits remain incomplete, and a human can approve the wrong exception, so further review can still expose an obligation we thought was covered.

I am willing to pay for the extra rebuild and review even when the result is a feature we have to record as missing.

*Last updated: 11 September 2026. The history of this article is available in the [project's Git repository](https://github.com/ze-software/ze/commits/main/website/blog/posts/the-proof-is-the-expensive-part.md).*
