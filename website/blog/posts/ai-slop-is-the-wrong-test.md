---
title: AI slop is the wrong test
date: 2026-08-03
author: Thomas Mangin
description: I expect AI coding to improve, as earlier programming tools did. That makes evidence and engineering judgement more important, and leaves me responsible for the software I accept.

deck: Today's failures deserve scrutiny. They do not establish tomorrow's ceiling, and human authorship has never guaranteed good software.

image: assets/blog/ai-slop-is-the-wrong-test.svg
image-dark: assets/blog/ai-slop-is-the-wrong-test-dark.svg
image-alt: Fast generated output enters a narrowing evidence path through protocol checks, fuzzing, integration and benchmarking before it is accepted or rejected.

---

Ze is an AI-written network operating system. I decide the architecture, the tradeoffs, what the code must never break and what gets rejected. Claude turns that into implementation.

I use AI to help write these articles too. I am lazy, and my time is limited, so I want to spend it on the judgement and the corrections, with help putting them into words. I remain responsible for what goes out under my name.

Calling it "AI slop" is easy, but the label says little about what is wrong. Slop is code accepted without enough scrutiny: merged because it compiled, or trusted because the demo worked once. Human-written code can be slop too, and AI makes it much cheaper to produce.

In many cases, though, AI has written better code than I could have written myself. It has been more meticulous, and it writes far more tests per feature than I ever did. I have good reasons to use it, and I think judging the technology by its current failures repeats a mistake programmers have made before.

*This article was drafted and revised with OpenAI Codex. The ideas, experience and conclusions are mine.*

## I have heard this argument before

In July 1983, Ed Post's [Real Programmers Don't Use Pascal](https://www.pbm.com/~lindahl/real.programmers.html) satirised programmers who treated FORTRAN and assembly as evidence of competence. I recognise the attitude, but I also think the serious objections behind it deserve more credit than they get in hindsight.

An assembly programmer could have good reasons to distrust a compiler. In an inner loop, knowledge of the processor and careful use of its instructions could produce something faster than the compiler managed. Michael Abrash's [Graphics Programming Black Book](https://www.jagregory.com/abrash-black-book/) describes that kind of engineering, and the released [Quake source](https://github.com/id-Software/Quake/tree/master/WinQuake) contains hand-written x86 assembly alongside its C rendering code.

Compilers improved, and the programs people wanted to build grew larger. The advantage of winning in one loop had to be weighed against the cost of understanding and changing the whole program, or moving it to another machine. A criticism of the compiler could be correct at the time and still be a poor reason to spend the following decade writing everything in assembly.

Assembly still has a place. [FFmpeg's x86 codec code](https://github.com/FFmpeg/FFmpeg/tree/master/libavcodec/x86) contains extensive assembly implementations, and knowing when that effort is justified remains a skill. I would not choose assembly for a control plane. The expertise survived the change in tools because someone still had to decide where direct control paid for its cost.

A model is different from a compiler: it can misunderstand the requested behaviour or invent an API, while a compiler translates a program under language rules. The history does not establish how reliable AI coding will become. It does explain why I am reluctant to treat a tool's present weaknesses as permanent limits, especially when I already find it useful.

## The failures are reasons to check the output

Models miss context and produce code which looks plausible until it reaches an edge case. Security is a concern too. Veracode's [2025 GenAI Code Security Report](https://www.veracode.com/blog/genai-code-security-report/) reported that 45 per cent of generated samples failed its security tests across Java, Python, C# and JavaScript. That describes its evaluation, rather than the vulnerable share of every AI-written codebase, and it is enough to reject the assumption that plausible output is safe output.

Even the feeling of getting more done can be misleading. In [METR's early-2025 study](https://metr.org/blog/2025-07-10-early-2025-ai-experienced-os-dev-study/), 16 experienced open-source developers took 19 per cent longer on tasks in familiar repositories when AI tools were allowed, while believing they had been faster. In its [February 2026 update](https://metr.org/blog/2026-02-24-uplift-update/), METR believes developers are likely to gain more from newer tools, but says the follow-up data gives only very weak evidence for the size of that improvement. Developers' reluctance to work without AI introduced selection effects, and concurrent agent use made time spent harder to measure.

I cannot turn either study into a productivity figure for Ze. I can take the warning seriously without concluding that there is no progress to be made. The tools can waste time today, and improvements to them can change which objections remain justified.

That is why I treat Claude as a generator which can follow precise constraints and will sometimes fail to follow them. It can explore alternatives and run tools, then revise an implementation against the design. I have to decide whether the result is acceptable, including whether the tests are asking the right thing.

For a BGP encoder, I want evidence that the bytes meet the protocol's requirements and that the implementation respects the ownership rules of the surrounding code. A test which calls a Go function checks a different boundary from one which starts Ze and talks to it as a peer. Both can pass while a case we did not think of remains broken.

Ze's public [quality model](https://ze-software.net/quality/) sets out which kinds of evidence belong at those boundaries. A parser failure should leave a test or a fuzz corpus entry, and a claim about Linux behaviour needs an exercise on Linux. For a memory optimisation, I want a repeatable benchmark with a stated allocation contract. These are acceptance requirements, and listing them is no claim that every part of Ze has met them.

The tests need the same suspicion as the implementation. A model may change the expected result to match bad code or remove the assertion which exposed a bug, and a green run then conceals the failure. [The proof is the expensive part](../the-proof-is-the-expensive-part/) describes what it costs to connect requirements to evidence and check that the connection means anything.

## I spend my expertise differently

I know BGP because I have operated it, implemented it, broken it and been called to fix my own code late at night. That background matters when Claude proposes a route-selection shortcut or a peer-state transition. I can compare its suggestion with something beyond the explanation the model supplies.

I had not implemented IS-IS or OSPF before Ze, but I could bring my experience of protocol debugging to them. With link-state routing I have to reason about the topology database and the route calculation, as well as how information reaches neighbouring routers. If an implementation behaves strangely, I need to reduce the failure to a packet or a state transition that I can explain.

My maths is very rusty, but I understand Dijkstra's algorithm and enough set theory to reason about reachability and shortest paths. My networking knowledge is low-level enough to know when the packet, socket or kernel behaviour is the suspect, and my debugging skills come from maintaining my own code in production. I still use Wireshark to decode TCP when I need help, and I need to recognise when its answer contradicts what I expected.

Claude lets me spend more of that experience on deciding what should be built and recognising when the implementation is wrong. It does not make the experience unnecessary. I must be able to stop it and read the RFC, then build a reproduction which demonstrates the rule it broke. If I cannot debug the problem without the model, I cannot safely ask the model to write the code.

That changes what I need from review as well. A reasonable-looking diff can conceal a failure which appears only across a peer connection or at capacity. I need to know which test would fail if the change were wrong, and which decisions the model took that it had no authority to take. Reading faster cannot compensate for leaving those questions unanswered.

## I expect this to improve

AI coding is young, and many of the practices needed to use it well are still being invented. Generating more code while keeping the same weak acceptance process will produce more bad software. I expect plenty of projects to do that, and I think the criticism they receive will be deserved. It is also a problem we can do something about, through better tools and a development process which gives the generator constraints and checks its output.

We should be just as critical of the software industry that existed before AI. We have lived for decades with human-written software which is slow, wasteful and difficult to understand. I have little patience for treating "Clean Code" as a substitute for taste, or for using a high-level language as permission to stop caring what the program compiles to. A person typing every line has never been a quality guarantee.

The demand for software gives us another reason to improve the process. In my view, we want more software than we have good programmers available to write and maintain, and much of what already exists needs attention. I am interested in tools which let an experienced engineer attempt more, while still understanding and controlling what gets shipped. My experience with Ze gives me reason to think AI can do that.

I expect both the models and the tools around them to improve. That expectation is why I am learning to use them now, rather than waiting for them to become ordinary. It gives me no permission to accept weak evidence today, any more than a promising compiler once justified ignoring the machine code it produced.

Claude cannot be responsible for an outage, a route leak or a security hole. The people who accept generated code and ship it are responsible, and I will not ask users to excuse a defect because of how Ze was written. They are entitled to judge the result and the engineering behind it.

I expected Ze to be finished by now. It is not ready, and the issues we keep finding have taken longer to fix than I expected. The repository is open, with the tests and design notes available for scrutiny, but I would rather delay Ze than release it with problems I already know about. When it is ready, I hope you will like it.

*Last updated: 9 September 2026. The history of this article is available in the [project's Git repository](https://github.com/ze-software/ze/commits/main/website/blog/posts/ai-slop-is-the-wrong-test.md).*
