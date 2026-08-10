---
title: The proof is the expensive part
date: 2026-08-06
author: Thomas Mangin
description: Ze uses AI to write code, but an RFC claim only counts after the standard, the tests, the public gap list and the commit process all agree.
---

I ended [AI slop is the wrong test](../ai-slop-is-the-wrong-test/) with this line: the code is cheap, the proof is the expensive part. This post is the missing explanation.

When people hear that Ze is an AI-written network operating system, they usually imagine the worst version of that idea. Ask a model for a routing feature, glance at the diff, run a quick test, merge it. That would be reckless.

A routing daemon can look convincing for a long time. It can parse the usual BGP UPDATEs, pass the happy-path examples, and still break when a real peer sends a malformed attribute, a timer fires in the wrong state, or a policy edge case leaks a route.

I do use Claude to write code. I also use it to write tests, fixtures, small tools and documentation. It is useful because it is fast. It is dangerous for the same reason. It can create a large amount of plausible work before anyone has noticed that the premise was wrong.

So I do not try to make the model more careful by asking nicely. I make Claude show its work, then the tests and checks decide whether that work is good enough.

Here, proof means quite practical things. Which RFC rule is this feature claiming to implement? Which test proves the normal case? Which test proves the bad packet is rejected? Does that test actually run? If Ze has not implemented a part of the RFC, is that gap written down? If the public web page says an RFC is supported, can we trace that statement back to the code and tests?

That is the system Ze is being built around.

*This article was drafted with OpenAI Codex. The ideas, experience and conclusions are mine.*

## Start with the standard

A routing feature often starts with an RFC. That already makes life difficult.

RFCs are prose. They were written by many people, over many years, for human readers. Some are crisp. Some carry old history. Some use the capitalised words `MUST`, `SHOULD` and `MAY`, as described by RFC 2119. Some important rules are in a table, a diagram, a state machine, or an ordinary sentence which never says `MUST` at all. Some describe behaviour for a role Ze does not play. Some quote another RFC, where the real rule actually lives.

This is how compliance becomes vague by accident. You implement the part you remember, add tests for the examples you thought about, and write that the RFC is supported. The code may even be good code. The problem is earlier than the code: nobody wrote down exactly what the code was meant to satisfy.

Ze starts by giving each obligation a name.

The hand-written checklists live under `rfc/short/*.md`. A requirement id such as `RFC7606-7.1-1` is meant to be boring and useful. It says: RFC 7606, section 7.1, first obligation in that section. That lets a test, a known gap, an audit result and a public status row all point to the same rule in the same document.

The gate treats `MUST`, `MUST NOT`, `SHALL`, `SHALL NOT` and `REQUIRED` as obligations. `SHOULD` and `MAY` can still be recorded, but they do not decide whether a commit is allowed to pass. That distinction matters in operations. A nice-to-have and a must-have are different kinds of promise.

Cloudflare arrived at the same conclusion from the other end. Their standards are internal documents rather than IETF ones, and [How Cloudflare enforces engineering standards using AI](https://blog.cloudflare.com/engineering-standards-enforcement/) describes giving each statement a stable name which survives edits to the text around it, for the same reason: a rule nobody can point at cannot be checked later. Naming the obligation is what makes everything after it possible, whoever wrote the document. I come back to that convergence in [AI coding has not had its Rails moment](../ai-coding-has-not-had-its-rails-moment/).

## Then check the checklist

A checklist can still miss something. Anyone who has worked with standards, audits or change control knows this failure mode. If the obligation was never written down, every later check can be green while the implementation is still wrong.

That is why Ze has extraction sign-offs in `rfc/extraction/<stem>.json`.

The name is dry, but the idea is simple. The checklist says what Ze thinks it must implement. The extraction file says where those requirements came from in the RFC text.

A source location means the place in the RFC where the rule was found: a section, a quoted sentence, a table entry, or another small piece of text. Ze checks the link in both directions.

First, every possible requirement found in the RFC text must either point to a checklist id, or be excluded with a reason. Otherwise Ze may have missed a rule.

Second, every requirement in the checklist must point back to the RFC text, or say that a reviewer found it while reading prose the extractor missed. Otherwise Ze may have invented a rule, or attached a test to the wrong part of the standard.

This does not prove that Ze understands every RFC perfectly. The extractor can miss prose. A `manual-walk` sign-off is a record that a human read the document, not a magical proof of understanding. The useful property is smaller and more honest: a missed obligation becomes a named risk, rather than an invisible green check.

That helps a lot with AI. A model can read an RFC and produce a plausible list of tests. Plausible is not enough. If it invents a requirement, the link back to the RFC fails. If it ignores a sentence the extractor flagged, the link from the RFC to the checklist fails. If it tries to hide behind a hand-written count, the generated inventory disagrees.

The process is boring on purpose. Boring checks are harder to fool.

## Tests have to say what they prove

A normal test name is not enough. `TestBadOriginLength` tells a programmer roughly what is being tested. It does not tell the rest of the system which standard claim depends on that test.

Ze's RFC tests carry tags. A Go test can contain a line like this:

```go
// RFC requirement: RFC7606-7.1-1 negative - ORIGIN length 2 selects treat-as-withdraw.
```

You do not need to read Go to get the point. The tag says that this test is about RFC 7606, section 7.1, first obligation, and that it tests a bad packet. In this case the bad packet has an ORIGIN attribute with the wrong length, and the required result is treat-as-withdraw.

Ze normally wants both sides of a MUST-level rule. A positive test proves the good case still works. A negative test proves the bad case fails in the required way.

Without both, tests can lie while still passing. A router that accepts everything can pass many positive tests. A router that rejects everything can pass many negative tests. The useful behaviour is in the middle: accept the legal packet, reject or contain the illegal one, and do both for the reason the RFC gives.

A tag does not make a weak test strong, but it gives the gate something precise to argue about.

The runner also matters. A test only counts if something actually runs it. Ze records what kind of evidence it has: a small Go test, a command transcript which drives the daemon like an operator would, or an interop scenario with another implementation. A tag in a file that no pipeline runs is refused. It is not weak evidence. It is no evidence.

This is one of the places where the system helps AI a lot. Claude will happily create a good-looking test in the wrong directory if the prompt lets it. The RFC gate does not care that the file looks like a test. It asks whether anything executes it. If nothing does, the tag is rejected instead of becoming a false comfort.

## Gaps are allowed to exist

A useful compliance system has to admit failure.

Some RFC requirements do not apply to Ze. Some can only sensibly be tested on the good side or the bad side. Some are real gaps. Ze records those cases as annotations.

A gap is a promise that the requirement is known and not yet met. It is still counted. It is also tied to the public RFC status page.

That page is not an IETF certificate. It is a support ledger for users. It says which RFCs Ze implements, partially implements, does not implement, or has deferred. If a feature is partial, the page should say which part is partial. If an RFC has no gated obligations, it should show why. If the implementation has a known gap, users should not have to guess from marketing language.

This matters because networks are operated on risk, not slogans. A missing corner of an RFC may be harmless in one deployment and unacceptable in another. Hiding the gap does not make the product better. It only moves the risk from the vendor to the operator.

## The gates push back

The same idea appears while code is being edited. Ze's edit rules reject some changes before they can become part of Ze.

RFC-tagged tests are one example. They are part of a public compliance claim. If Claude tries to change the behaviour of an RFC-tagged test, or remove the tag, the edit hook blocks the change unless the text carries an explicit approval marker. The marker is not magic. It is a human process boundary.

The failure it prevents is common with generated code. The model sees a failing test, changes the test to match the bug, gets a green run, and presents that as progress. For normal code this is already bad. For a public RFC claim it is much worse.

The generated ledger is protected too. `ai/RFC-REQUIREMENTS.md` is built by `make ze-rfc-index`. It maps requirements back to tests, annotations, evidence type and audit state. If the ledger is stale, `make ze-rfc-check` says so. The fix is to regenerate it from source, not edit the table by hand.

Again, this is for the AI as much as for the human. The model gets concrete failure messages: missing bad-packet test, unknown requirement id, tag in a file that no pipeline runs, gap missing from the public status page, stale audit verdict, stale generated ledger. Those messages are much better prompts than "make the quality better".

## Commits have to carry the proof

The final guard is the commit path. A commit should carry evidence that this exact set of files was checked. That matters more when several agents can share one working tree and one git index. A failed commit can leave files staged, and the next commit could accidentally carry them.

Ze does not let an agent type `git add` and `git commit` directly. The approved path goes through `scripts/dev/commit_helper.py`.

Before the helper prepares a commit script, it asks whether the current tree is byte-for-byte identical to the last successful verify run. The fingerprint includes the current commit, tracked changes and untracked file contents. If the tree changed, or the last verify failed, the helper refuses a normal commit.

When the helper does prepare a commit, it writes a message file and an executable script. The script stages only the explicit files listed for that commit. Then it checks the shared index. If any staged file exists which is not part of this commit, the script aborts. Only then does it run `git commit -F`.

There is one extra check after commits containing Go code. `make ze-tracked-build-check` runs after the commit because it tests the source tree git now holds. A pre-commit test can check the working tree. This check catches the case where the committed set itself does not build.

That sounds fussy because it is fussy. It is change control for generated code.

## Why this makes AI useful

The usual way to use an AI coding tool is to ask for a feature and then inspect the diff. That is weak. The diff can look reasonable and still break the protocol.

Network engineers already know this pattern. Nobody serious accepts a network change only because the proposed config looks plausible. We use linting, lab tests, staged rollout, telemetry, rollback plans and change windows because the expensive part is proving that the change behaves in the real system.

Ze applies the same instinct to code.

The model is good at the repetitive work: read the failure, find the requirement, patch the file, rerun the narrow check, update the generated ledger, try again. The rules and generated files supply memory and boundaries. The human supplies judgement when the question is about meaning: what role Ze plays, whether a gap is acceptable, whether the test really captures the behaviour, and whether the implementation is still a good design.

This is the part people miss when they ask whether AI can write production code. The answer depends on the production system around it. If the system accepts plausible output, AI will produce plausible output. If the system demands evidence, AI can help produce evidence.

## This is still not finished

There are limits.

Extraction sign-off is still being expanded. Semantic audits are sampled, not total. A `manual-walk` record is an honest declaration, not a proof of understanding. A model can still produce a bad test, a narrow test or a correct test for the wrong reason. A human can still approve the wrong thing.

The difference is that these are named risks. They are in the system. They are counted, surfaced or blocked. That is much better than trusting a confident paragraph in a pull request.

I do not think the unique part of Ze is that AI writes a lot of the code. Many projects will do that. Some already do. The unique part is that Ze is being built around the assumption that generated code is untrusted until it earns its place.

The code is cheap. The proof is the expensive part.
