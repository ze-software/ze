---
title: AI slop is the wrong test
date: 2026-08-03
author: Thomas Mangin
description: Ze is an AI-written NOS. The useful question is whether the code is constrained, reviewed, tested and measured.
---

Ze is an AI-written NOS. I decide the architecture, the tradeoffs, what the code must never break and what gets rejected. Claude turns that into implementation.

That is also how I use it for these articles. I am lazy, and my time is limited. I want the time I do have spent on the judgement, the corrections and the parts only I can supply.

That makes some people uncomfortable. I understand why.

Calling it "AI slop" is easy. It is also too vague to be useful. Slop is code accepted because it was generated, merged because it compiled, or trusted because the demo worked once. AI makes that failure mode cheap. It can produce more bad code in an afternoon than a bad programmer could type in a week.

Manual code can be slop too. AI just removes the production cost.

*This article was drafted with OpenAI Codex. The ideas, experience and conclusions are mine.*

## I have heard this argument before

In 1983, Ed Post published [Real Programmers Don't Use Pascal](https://www.pbm.com/~lindahl/real.programmers.html), a satire of the culture which treated FORTRAN and assembly as real programming and everything else as soft. It was funny because the attitude existed.

The serious version of the argument was stronger than people now remember. Hand-written assembly was faster. It gave direct control over the machine. It avoided weak compilers, poor optimisers and unpredictable output. If you were writing the inner loop of a graphics engine or a codec, a good assembly programmer could beat the compiler.

For a while, that was true.

Then compilers improved and software got larger. The economics changed. The question became less about whether a human could beat the compiler in one loop, and more about whether a team could still understand, port and change the whole program.

The history of id Software shows the transition well. The released [Quake source](https://github.com/id-Software/Quake) still contains hand-written x86 assembly in the rendering path, and Michael Abrash's [Graphics Programming Black Book](https://www.jagregory.com/abrash-black-book/) is still worth reading. By [Doom 3](https://github.com/id-Software/DOOM-3), the engine was a large C++ codebase with isolated SIMD paths and the GPU doing the rasterisation work.

Assembly did not disappear. [FFmpeg still carries pages of x86 SIMD assembly](https://github.com/FFmpeg/FFmpeg/tree/master/libavcodec/x86), because there are places where it still pays, but nobody sane writes a control plane in assembly anymore.

It found its place.

## Generated code has the same problem

Many objections to AI-written code are fair. Models hallucinate APIs. They miss context. They produce code which looks plausible and fails at the edge. Security is a real concern: Veracode's 2025 GenAI Code Security Report says [45% of generated samples failed security tests](https://www.veracode.com/blog/genai-code-security-report/).

Productivity is also less obvious than the marketing suggests. METR's early-2025 study found that experienced open-source developers working in familiar repositories were [19% slower with AI tools](https://metr.org/blog/2025-07-10-early-2025-ai-experienced-os-dev-study/), while believing they had been faster. METR then [updated the picture in 2026](https://metr.org/blog/2026-02-24-uplift-update/), saying the early result no longer reflected current tools and that follow-up measurement had become difficult because many developers did not want to work without AI.

That is a useful warning in both directions. The tools can waste time. The tools also move fast enough that confident claims expire quickly.

I agree with the warning. I disagree with the conclusion some people attach to it.

## Ze assumes the generator is untrusted

Ze does not treat Claude as an engineer who understands the system. It treats Claude as a fast generator which can follow precise constraints and will sometimes fail to follow them. It can explore alternatives, write tests, run tools and revise code until the evidence matches the design.

That distinction matters.

A BGP encoder is not correct because Claude wrote it. It is correct when it emits legal wire bytes, rejects illegal inputs, preserves ownership rules, passes the protocol tests and still behaves correctly when a test starts Ze and talks to it as a peer, rather than calling a Go function directly. A memory optimisation is not correct because a benchmark got faster once. It is correct when the allocation contract is explicit, the benchmark is repeatable and the failure mode is safe.

That is why Ze has a public [quality model](https://ze-software.net/quality/). Local Go tests, fuzz targets, mutation checks, functional transcripts, browser tests, QEMU runs, interop tests, performance gates and release evidence all have different jobs. A parser bug should leave a test or corpus entry. A CLI bug should leave a functional transcript. A Linux behaviour should run on Linux. A performance claim should have a benchmark and a scope.

Claude can write a parser, a protocol encoder, a web page, a benchmark and a hundred test cases in the time a human would spend preparing the first patch. The output earns nothing by existing. It has to match the protocol, preserve the invariants, fit the ownership model, pass the narrow check, then pass the wider gate.

If a piece of generated code cannot be explained, bounded, tested or measured, it does not belong in Ze.

## The skill has moved

The uncomfortable part is that this does not remove expertise. It moves where the expertise is spent.

I know BGP because I have operated it, implemented it, debugged it, broken it and been called to fix my own code late at night. That background matters when Claude proposes a route-selection shortcut, a peer-state transition, a capability rule or an error path. The same kind of knowledge carries across protocol work.

I had not implemented IS-IS or OSPF before Ze. That does not mean starting from nothing. Link-state protocols still have a graph, a database, flooding rules, timers, sequence numbers, checksums, authentication, neighbour state and Dijkstra at the centre. If the implementation behaves strangely, the work is to reduce the failure to a packet, a state transition, a graph invariant or a route calculation.

My maths is very rusty, but enough for that job. I understand Dijkstra's algorithm and enough set theory to reason about reachability, membership, shortest paths and convergence. My networking knowledge is low-level enough to know when the packet, socket or kernel behaviour is the suspect, and my debugging skills come from maintaining my own code in production and being the person called to fix it. That teaches a lot. I still use Wireshark to decode TCP when I need help. That is normal. The important part is knowing what question to ask and when the answer looks wrong.

That is the skill I need when Claude is wrong. I must be able to stop it, inspect the packet, read the RFC, build the reproduction, write the narrow test and explain what rule it broke. If I cannot debug the problem without the model, I cannot safely ask the model to write the code.

## The useful question

In my opinion, authorship is the least interesting part of the argument. What matters is whether the software practice is strong enough to produce good-quality software and surface the bugs which are inevitable. That is the same question a large team has to answer.

For Ze, the useful questions are practical. Which rule must the code never break? Where is that checked? What input breaks it? What happens at capacity? Which test fails if this code is wrong? Which benchmark proves the optimisation? Which source of truth generates the documentation? Which human decision did the model make that it had no authority to make?

Those questions matter for human-written code too. AI makes them harder to avoid because it produces so much code so quickly. It also makes responsibility easier to dodge if people allow it. Claude cannot be responsible for an outage, a route leak or a security hole. Users should not have to care how the code was written. Vendors are expected to have done their work properly. The authors who accept generated code and ship it are responsible, and they must not hide behind the model. I will not.

The old review habit of reading a diff and deciding whether it looks reasonable is weak against generated work. The diff can look reasonable and still be wrong in a way that only a protocol test, fuzz target or integration run will catch, and even that is not a silver bullet.

This is where I think many AI projects will fail. They will use AI to create code volume without changing the verification model. That deserves the word slop.

Ze is trying to do the opposite. The code is cheap. The proof is the expensive part.

## Where I think this goes

AI coding is a young technology. The tooling is immature, the best practices supporting it are still being invented, and the training the models receive is imperfect. A model will still sometimes change a test to make bad code pass. It will still sometimes remove the assertion which was catching the bug, or produce the shape of a fix without understanding the failure. A lot remains to be done before vibe coding can be trusted for serious systems.

That sounds very much like compilers looked to the assembly crowd.

It worked, but good human output was often superior.

There is another pressure this time. The world wants more software than we have good programmers to write and maintain. It is easy to blame AI code because the failure is new and visible. We should also be critical of the industry which existed before AI. Plenty of human software is slow, wasteful and hard to reason about. Some of that comes from treating "Clean Code" as a substitute for taste, and from using high-level languages as permission to stop caring what the program compiles to. We have lived with atrociously performing software for decades.

The skeptics are right about many present failure modes. Their mistake is treating those failure modes as the ceiling. The optimists are right about the trajectory. Their mistake is treating trajectory as permission to skip evidence today.

If I could talk to assembly programmers in 1988, I would say: keep the assembly for the hot loops, and learn C before the economics change around you.

The equivalent now is simple. Learn to work with AI. Keep control of architecture, invariants, review and release. Use it where the evidence says it helps. Push it out of the path where it produces risk.

That is how Ze is being built, and it is why it is taking longer than I expected.

Some people will still call it slop. Fine. The repository is open, the tests are there, the design notes are there, the benchmarks are there. I expected Ze to be finished by now. It is not ready because we see the issues and are fixing them instead of releasing slop.

Good software eventually finds users and earns appreciation, even when it is niche. Bad software just makes up most of GitHub's content, the underwater part of the iceberg that almost nobody sees or discusses, but which still exists. I would rather Ze be late than join that pile.

When Ze is ready, I hope the result speaks for itself. I hope you will like it.
