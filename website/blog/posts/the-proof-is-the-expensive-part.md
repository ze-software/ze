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

Working in networking gives you a fairly poor appetite for changes which look as though they should work. A proposed configuration can look quite reasonable, and I still expect lab evidence and a staged deployment before it goes across the network. Monitoring and a rollback plan exist because we may have missed something anyway. Writing the configuration is only part of the job.

I bring the same instinct to Ze, the network operating system I am building with AI. Claude writes code and tests, and helps with fixtures and documentation. That is useful, and I have no desire to give up the speed, but it can produce the whole lot from one mistaken interpretation. Reading a convincing explanation beside code which agrees with it is rather less reassuring once the same model has written both.

A routing daemon may process ordinary BGP UPDATEs for a long time before a malformed attribute or an unfortunate sequence exposes the mistake. In [AI slop is the wrong test](../ai-slop-is-the-wrong-test/), I called the proof the expensive part. Much of that expense comes after Claude has something ready to show me, when I have to decide whether we have tested what the protocol requires at all.

*This article was drafted and revised with OpenAI Codex. The ideas, experience and conclusions are mine.*

## Before writing the first test

An RFC is a less convenient specification than its number suggests. Different people wrote the documents over many years, an obligation can be in a table or state machine, and ordinary prose can require behaviour without a capitalised `MUST` in sight. Some passages concern a role Ze does not play; others depend on a rule explained in another document.

Starting with the part I remember is tempting. It is also a good way to implement that part correctly and make an unjustified claim about the rest. Tests built around the same remembered examples will agree with the code, and nobody will have asked about the passage both of them missed.

Ze therefore keeps requirement checklists under `rfc/short/`, with an ID for each obligation. [`RFC7606-7.1-1`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc7606.md), for example, names the first obligation recorded for section 7.1 of RFC 7606. A test can use that name, and so can a known gap or an audit result. When somebody disagrees with the interpretation, we can at least disagree about the same sentence.

This was also what interested me in [Cloudflare's engineering standards system](https://blog.cloudflare.com/engineering-standards-enforcement/). Their standards are internal rather than IETF documents, but stable names let later checks refer back to a particular obligation. The shared-conventions part belongs in [AI coding has not had its Rails moment](../ai-coding-has-not-had-its-rails-moment/); here the immediate benefit is removing the guess about what a test's author meant it to cover.

The checklist preserves the RFC's levels of obligation. In Ze's [requirement model](https://github.com/ze-software/ze/blob/main/internal/le/rfc/rfc.go), `MUST`, `MUST NOT`, `SHALL`, `SHALL NOT` and `REQUIRED` enter the mandatory gate. `SHOULD` and `MAY` can be recorded without entering it. Treating a recommendation as mandatory changes the promise we make just as surely as overlooking a mandatory behaviour does.

Of course, giving a mistaken interpretation an ID does not improve it. Extraction records under `rfc/extraction/` are there to account for how the checklist was derived. The [sign-off checker](https://github.com/ze-software/ze/blob/main/internal/le/rfc/signoff.go) compares a record with the source inventory in both directions: detected passages need a classification, and gated requirements need a source mapping or an explanation of prose the inventory did not capture.

An ignored passage or an unsupported checklist entry can then be found mechanically. If the extractor and reviewer both miss an obligation, the check will miss it too, and a `manual-walk` sign-off records a reading which the machine cannot reproduce. Somebody still has to judge whether an exclusion makes sense. Keeping the reason beside the checklist at least gives the next reviewer something to challenge, instead of an apparently finished list.

## One bad ORIGIN attribute

[RFC 7606 section 7.1](https://www.rfc-editor.org/rfc/rfc7606.html#section-7.1) gives a small enough example to follow. A BGP ORIGIN attribute is malformed when its length is not one octet or its value is undefined. The UPDATE carrying it SHALL receive treat-as-withdraw handling, which [section 2](https://www.rfc-editor.org/rfc/rfc7606.html#section-2) defines as treating the routes in that UPDATE as withdrawn and removing them from those held for the peer.

Ignoring the packet would be convenient, but it could leave an older route installed. Resetting the session would remove it, along with other routes which did nothing wrong. The RFC specifies an outcome between those two, and a test has to distinguish it from both.

In [`rfc7606_test.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go), `TestRFC7606MalformedOriginLength` supplies an ORIGIN two octets long to `ValidateUpdateRFC7606`. It requires exactly `RFC7606ActionTreatAsWithdraw`, and carries the `RFC7606-7.1-1` tag as a negative case. The [validator](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606.go) checks length and value; other tests supply the defined values and require acceptance.

Both sides are needed. Accepting everything can pass the positive examples, and rejecting everything can pass the negative ones. Even an assertion which accepts a session reset as sufficiently severe would let this implementation overreact. The test must insist on the particular action the RFC asks for, rather than be pleased that an error was noticed.

So far, this exercises the validator's choice. The daemon also has to use that choice without destroying the session, and [`test/plugin/rfc7606-withdraw.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc7606-withdraw.ci) tests that by starting Ze and a peer, sending the bad ORIGIN, then expecting a valid announcement over the same connection.

The scenario checks that the connection survives. That still leaves the route removal unobserved: it never reads the routing table, so receiving the later announcement cannot establish that a previously installed route was removed. Its comments say so, and it does not claim `RFC7606-2-1`, the separate withdrawal obligation. The filename sounds more generous than the test is entitled to be.

Adding that requirement tag would be easy. It would also make the coverage look better without observing anything new, which is precisely the sort of improvement I am trying to prevent. Two related RFC sections and two related kinds of test have already required several different judgements, for an attribute only one octet long when it is valid.

## A test file is a start

The [generated requirement table](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc7606.md) gathers the tags and labels them with classifications such as `unit/verify` and `functional/verify`. This tells a reader what kind of test has been found and which verification machinery can run it. It also gives an arriving agent somewhere to look before writing another example of the same behaviour.

A plausible filename is not enough to get into that table as runnable reassurance. The [carrier scanner](https://github.com/ze-software/ze/blob/main/internal/le/rfc/carriers.go) derives functional classifications from the verifier's suite list and refuses tags in recognised test formats which have no automatic runner. Otherwise a model could give us a fine-looking scenario that no normal check ever executes.

Being attached to a runner does not mean it ran for a particular change either. Functional verification selects suites according to the change, so a green overall result may not include this scenario. If the judgement depends on its execution, the run has to be established separately.

Then comes the failure described in [The repository is half the AI harness](../the-repository-is-the-ai-harness/): a test can run, pass, and stay green when the behaviour it claims to protect is deliberately broken. Ze's [public RFC ledger](../../quality/rfc-compliance/rfc7606/) keeps test tags separate from stored discrimination evidence, meaning an observed failure after breaking the claimed behaviour. A tagged unit without that record is labelled `unproven`.

At this revision, RFC 7606 has tagged tests but no stored discrimination file, and its extraction sign-off is absent too. Those are missing pieces of the record. It would be rather pointless to build machinery for exposing them and then describe the table as though every review and experiment had already happened.

## Partial means there is something missing

Even a correctly extracted requirement with an appropriate test can remain unimplemented. Some requirements do not apply to Ze, and some admit useful evidence on only one side; a known implementation gap needs its own answer, rather than being filed under either of those explanations.

RFC 7606 is declared `Partial` in its summary metadata, with a deliberate section 5.1 gap. That gap is published beside the support statement. Passing the ORIGIN tests does not settle the rest of the RFC, however convenient it would be to give the whole document one green box.

The repository's [requirement rows](https://github.com/ze-software/ze/blob/main/internal/le/rfc/render.go) are also used by the [site ledger producer](https://github.com/ze-software/ze/blob/main/internal/le/site/rfcledger.go), alongside the stored audits and proof records. The public wording comes from the summary somebody authored. Generation keeps that declaration with the material behind it; it cannot decide whether the declaration is deserved.

From an operator's side, a missing corner of the RFC may be irrelevant to one deployment and rule out another. Hiding a known gap transfers a risk to the person deploying the software without giving them the information to assess it. I would much rather they find the omission on the support page than through a routing failure. These pages remain Ze's account of Ze, and carry no IETF certification.

## The red test has to survive the repair

Once a test contributes to a public statement, changing it to agree with broken code damages both at once. A model faced with a failure sometimes takes that shortcut, and the explanation can be quite convincing if nobody goes back to the obligation it was meant to enforce.

The [test-edit process](https://github.com/ze-software/ze/blob/main/ai/rules/testing.md) requires approval for weakening tests and a separate record of user approval for RFC-tagged tests. The native edit hook passes the proposed change through the weakening checker. Structural checks miss changes of meaning, but the approval record gives the reviewer somewhere to establish what was authorised; the agent's justification does not authorise itself.

Generated tables can be made to agree by hand too, until the next generation loses the edit. `./le rfc index-update` rebuilds them from their sources and `./le rfc check` checks their freshness. A discrepancy belongs back at its source, where correcting it will survive another build.

This is where I find AI particularly useful. An unknown requirement ID or a missing negative case gives Claude a specific problem to follow, and it can find the reference, propose a case and run the relevant check. I still have to decide whether the case exercises the obligation, but I can spend the review on that instead of searching again for what the author was trying to prove. A general request to improve quality gives the model much less to work with.

## Committing the files which were checked

Several agents sharing a checkout introduce another fairly mundane way to lose the connection. One session can stage a file for another's commit, or edit a named file after the commit was prepared. Even if the earlier check was useful, the eventual commit may contain different code.

The [current commit helper](https://github.com/ze-software/ze/blob/main/docs/contributing/committing.md) prepares a script with a private index and captures file contents during preparation. The [script](https://github.com/ze-software/ze/blob/main/internal/le/commit/script.go) seeds its index from HEAD and adds those captured blobs. Unrelated staged files stay out, and later edits to the named files stay in the working tree.

The policy around verification has changed since the original version of this article. A normal local commit no longer requires the whole working tree to match a successful verification run. Failed or missing verification is recorded as debt, an authorised push is refused while that debt remains open, and structural failures charged to the commit require a recorded reason.

That permits a local commit while keeping the unfinished verification visible. Capturing the right files cannot tell us whether a test is any good, of course; it prevents a different mistake, where we carefully review one set of contents and commit another.

## The part I cannot delegate away

Claude can write the implementation faster than I could, and it can follow a precise failure back through repetitive code without my having to do every step. I choose what role Ze plays and which compromises I am prepared to make. A feature may already compile and pass its author's examples when a review of the RFC shows that those examples were answering the wrong problem.

That can be irritating, but it is a useful discovery. In network change control, a failed lab exercise is a reason to revise the change before deployment. Generated code deserves the same response. Producing another implementation is sometimes less useful than replacing the test which let the first one through.

There is plenty unfinished here. Extraction reviews and semantic audits do not cover everything, and a human can approve the wrong exception. I am willing to pay for the extra rebuild and review which exposes one of those holes, even when the result is a feature we have to record as missing.

*Last updated: 11 September 2026. The history of this article is available in the [project's Git repository](https://github.com/ze-software/ze/commits/main/website/blog/posts/the-proof-is-the-expensive-part.md).*
