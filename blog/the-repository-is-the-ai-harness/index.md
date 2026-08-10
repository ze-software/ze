# The repository is half the AI harness

*2026-08-09 by Thomas Mangin*

Ze is a network operating system spread over 634 Go packages. A model can open any one of them in under a second and still have no idea which package a change belongs in, which rule it is about to break, or which test would catch it if it gets that wrong. Nothing in the repository tells it. That is a problem about repository structure, and it is a good deal older than AI.

At the start of the 2000s, Git did not exist. CVS was notorious because you could not rename a folder, a serious limitation when the structure of a project mattered. Subversion was the modern choice if CVS had not already put you off version control.

Project layouts were not very standard either. Our convention was to put each project's configuration, working data and source code under the same three top-level directories:

```text
project/
    etc/<project>/
    data/<project>/
    src/<project>/
```

Project names let us combine or separate trees as needed. The layout was close enough to the Filesystem Hierarchy Standard (FHS) to map onto the operating system easily. During development, the repository acted as the installed root, leaving only a code-test loop.

The same thinking is visible in the [first ExaBGP commit from September 2009](https://github.com/Exa-Networks/exabgp/commit/5490f7baf5981279e2360d88c735570bc9f72532). It had `daemon`, `etc`, `lib` and `test` directories, and a test which announced a route to a Cisco 7204 and kept the connection alive. By 2009 this kind of layout was far more familiar, largely because Ruby on Rails had made project structure part of the framework.

Ze is in good company here. Most repositories on GitHub are well organised for human developers and for the tools we already had, and the reasoning behind that organisation was never written into the tree. People carried it instead, in a wiki nobody updates, a handful of pull request comments and the maintainer's head. An agent arrives with none of that and no way to ask for it.

Rails gave repositories a shape people could learn, and the tooling followed once that shape became predictable. An agent needs that shape as much as a new developer does. It also needs the reasoning behind the shape, and something which checks its work. A developer who misreads a convention usually notices, and a reviewer notices for them when they do not. An agent produces something plausible and moves on. So an AI-ready repository has to teach the agent how the project works and catch it when it gets that wrong. This article describes how Ze is trying to do both.

*This article was co-authored with Claude. The architecture, design decisions and conclusions come from my work on Ze. Claude helped organise the material and draft the text.*

## Rails made the tree an interface

Rails turned project structure into something people argued about. Its [official philosophy](https://guides.rubyonrails.org/getting_started.html#rails-philosophy) calls the idea Convention over Configuration. Running `rails new` creates recognised places for application code, configuration, database changes, libraries and tests.

Rails was probably not the first project to lay a tree out like that, but it was the first one to make the idea popular. It was an idea whose time had come. Many people encounter the same pain, the available tools make an answer possible, and several engineers independently build similar things. One implementation gets the viral effect, receives a memorable name and becomes the example everybody remembers. Twenty years later the same thing is happening around the plain Markdown files an agent reads, and I come back to that at the end.

What Rails did was make its conventions consistent, generate them automatically and teach them to a large community. A developer entering an unfamiliar Rails application already knew where to look, and so did the Rails tools. The directory tree had become an interface shared by people and programs.

That is two problems solved together. Rails improved the framework tools and prescribed the structure those tools could expect, so they never had to rediscover an application's shape.

The idea spread far beyond Rails. Plenty of ecosystems now ship a command which lays out a new project for you, such as `django-admin startproject`, `cargo new` or `ng new`. Scaffolding a tree is completely normal, so the mechanism we would need already exists. What those commands generate is aimed at the compiler, the package manager and the human reader. Not one of them puts anything in the tree for an agent.

An agent needs the same interface a new developer needs, with one extra difficulty: it starts every session knowing nothing at all.

## AI repositories have not had their Rails moment

AI coding has concentrated on the tool side. Harnesses keep getting better at tool use, planning, context management and coordination between agents, and the models improve every few weeks. Still, they walk into arbitrary repositories with no idea where anything is meant to live.

The tooling looks the part now, with its animated terminals and neat progress trees, and underneath the presentation it is still immature. Stencil's [The harness problem](https://stencil.so/blog/the-harness-problem) measured something far more basic than planning or context management, which is whether a model can apply the edit it has already decided to make. Grok 4 failed half of its patches on their benchmark, EDIT-Bench has a single model above sixty per cent on realistic editing tasks, and changing nothing except the format of the edit moved sixteen models by fifteen points on average. Stencil sells that format, so their own figures deserve the usual caution, and an effect of that size is hard to dismiss.

A model which cannot reliably replace a line of text is not being held back by its understanding of the code. A model's ability is fixed the day it ships, and how much of that ability arrives in the repository depends on what we build around it. The edit format is the cheapest thing on that list, and a fair measure of how much better AI coding can get without waiting for new models.

A harness has to support every language and every kind of repository, so it stays general. It can search files, edit text and run commands. It cannot know why one project requires registration while another prefers a switch statement. It cannot know where an architectural decision lives when the project never linked that decision to the code. It cannot recover a rule which exists only in the maintainer's memory.

We keep improving the worker while leaving the workplace unexplained.

Frontier labs are well placed to fix half of this, because they build the model and the harness together and can train one for the other. The other half is not theirs to fix. Nobody at a lab can decide where your architectural decisions live, which patterns your project considers correct, or what counts as evidence that a change works. That has to happen inside the repository, and for now it has to be built by hand.

This is one reason opinions about AI coding are so divided. Some people see the failures of current harnesses and conclude that AI cannot produce serious software. Others use AI successfully but accept the awkwardness of today's tools as a permanent cost. Both positions treat the current state of the art as the final form.

AI-assisted development reminds me of using the Internet at home over 64K between 1994 and 1996. I had already used a faster university connection, so home access was painful. It required technical skill and motivation, and bulletin board systems were still more popular than the Internet. Yet I loved it because the promise was obvious. ADSL and services such as BitTorrent and YouTube later made the Internet useful to far more people. Judging the Internet by 64K would have mistaken a temporary stage for the limits of the technology.

AI coding is at a comparable stage. It is already a large improvement for technical and motivated users who are willing to work around its limitations, and its ADSL moment has not arrived. Many of the AI development best practices which will make it ordinary are still to be invented. Most of that missing work is not intellectually difficult. The edit format above needed no training and around three hundred dollars of benchmarking, which is the sort of gain that sits on the floor until somebody thinks to measure it. Nobody has built and standardised it yet.

In 1999 and 2000, working for an ISP felt like having a superpower, perhaps the way working for a frontier AI lab feels now.

I ended [AI slop is the wrong test](../ai-slop-is-the-wrong-test/) by saying that generated code has to be constrained, reviewed, tested and measured. [The proof is the expensive part](../the-proof-is-the-expensive-part/) explains how Ze ties an RFC claim to requirements, tests, known gaps and commit evidence. This article covers what comes before all of that: helping the agent find the right information before it writes the wrong code.

## Other engineers are finding the same shape

This problem is wider than Ze.

Cloudflare published [How Cloudflare enforces engineering standards using AI](https://blog.cloudflare.com/engineering-standards-enforcement/) in August 2026. Before building what it calls the Cloudflare Codex, its engineering guidance lived in formal documentation, repository files, chat threads and the accumulated knowledge of individual engineers. Developers spent time searching and could not always tell whether an answer was current, authoritative or relevant.

That is almost exactly the problem we found while building Ze.

Cloudflare turned its standards into structured RFCs. Requirements use `MUST` and `SHOULD`. Each statement has a stable name which survives edits to the document around it. Agents receive a compact set of relevant statements first and load the complete RFC only when they need more. AI reviewers check code and technical designs, and rules which ordinary software can recognise are moving into fast linters.

Ze arrived at a remarkably similar shape inside a single repository. Individual instructions have stable path names. A compact index tells an agent which detailed rule applies. Source files point to their design documents. Edit hooks reject common mistakes immediately. Larger checks run before a change can be accepted.

Cloudflare is applying this across a large engineering organisation and Ze is doing it inside an open-source project, which makes the convergence useful evidence. Independent engineers facing the same problem are reaching similar conclusions.

## The project is the other half of the harness

The harness supplies the general abilities: read a file, search the tree, edit code, run a program, keep track of a task and report a failure. The project has to supply the meaning.

Ze carries that meaning in several layers.

`ai/INDEX.md` is a task-oriented entrance. It answers questions such as where to start when adding a plugin, changing configuration, implementing an RFC or adding a command, so the agent does not have to search hundreds of packages to find the first document.

`ai/PACKAGE-MAP.md` gives one short description for each of those 634 packages, which is a great deal of blind exploration to avoid.

Production Go files carry a `// Design:` line near their top, so opening the implementation reveals the document explaining why it exists. Closely connected files also point to each other with `// Detail:`, `// Overview:` and `// Related:` comments. There are around 3,400 of the first kind and 2,900 of the second.

The documents point back into the source through `<!-- source: ... -->` markers. Small programs generate the two reverse indexes, `ai/CODE-TO-DOCS.md` and `ai/DOCS-TO-CODE.md`:

```text
code file       -> documents which explain it
design document -> source files which implement it
```

Those names are boring, which is useful. An agent can guess them without learning a product name for a simple idea.

All of these links are checked. A renamed file, a deleted design or a stale generated index makes verification fail. A hand-written index which quietly becomes fiction is worse than no index at all.

Source files are kept around one concern. Ze uses 1,000 lines as the point at which a production file must be examined for a second concern, which protects the agent's context: changing one thing should not mean paying for several unrelated things on every later step. A coherent file is allowed to stay coherent even when it is large, because mechanical splitting usually makes navigation worse.

Humans benefit from all of this too. We have longer-lived memory than an AI session, though we still forget, change teams and misunderstand old decisions. Structure reduces the amount everybody has to remember.

## A rule needs teeth

Consider command dispatch. A switch is simple and efficient when every possible command is known:

```go
switch name {
case "show":
    runShow()
case "configure":
    runConfigure()
}
```

Ze allows features supplied by plugins, including third-party plugins, so the complete set of commands is open. A central switch would force the core to know every extension and would destroy that separation.

Ze uses a registry instead. A registry is just a table which modules add themselves to:

```go
Register("show", runShow)
Register("configure", runConfigure)
```

The main program looks up the name in the table, and a third party can register another command without touching the core.

A requirement like that needs more than a sentence in an architecture document. The repository provides a chain. `ai/patterns/registration.md` explains why registration is required and what the accepted shape looks like. Nearly four hundred `register.go` files show the normal implementation. `make generate` scans for those files and writes the list of modules loaded at startup, so nobody maintains it by hand. An edit hook rejects hard-coded command lists and registration hidden inside `init()`. A separate check catches dependencies crossing a plugin boundary. Tests prove that registration and discovery actually work.

Whatever the hook prints becomes the agent's next prompt, so it has to point somewhere useful:

```text
✘ BLOCKED: Implicit behavior in init()
  Registration belongs in register.go (or register_*.go), not init().
  Move Register/Subscribe/AddHandler/Hook calls to register.go.
  Global assignments belong in var blocks, not init().
  See ai/patterns/registration.md
```

That is the general shape I want from an AI-ready project. An important rule needs an explanation, a visible example, a mechanical check where one is possible, and a behavioural test.

The check should also run as early as it can. The `init()` mistake above is caught while the file is being written, before anything is compiled, so the agent repairs one file. Caught by a test run half an hour later, the same mistake has a morning's work sitting on top of it, all of it written on the assumption that the first file was acceptable. Some evidence cannot be had that cheaply: a protocol error may need another routing daemon running beside Ze, so it naturally comes later and costs more. Ze's verification command currently runs twenty-five stages in roughly that order. Cheap structure, documentation and generated-file checks come first, then unit, race, allocation, functional and ExaBGP compatibility tests. Release evidence extends further into fuzzing, interoperability, Linux virtual machines, deployment tests, chaos and performance.

Those twenty-five stages are scar tissue. Almost every one exists because something plausible once passed through a weaker process.

## Testing has two directions

Unit testing is one form of testing. It answers a small question, and it can give a dangerously reassuring answer when the real requirement is larger.

The functional direction asks how far through the system a behaviour has been proved:

| Level | Question |
|---|---|
| Function | Does this small piece return the expected result? |
| Subsystem | Do the connected parts work together? |
| Application | Can a user achieve the result through the real program? |
| Interoperability | Does it work with software written by somebody else? |

A door is the easy analogy. At function level the handle turns and operates its latch. At subsystem level the handle is fitted to a door, and the door still opens and closes. At application level the door has to secure the building, because a door which works perfectly is still useless if it has been fitted next to a large hole in the wall. At interoperability level the door has to work with a frame and a lock supplied by somebody else.

For Ze, a parser can return the right values in a Go test while the complete daemon sends the wrong message on the wire. Ze can also agree with its own test peer while FRR, BIRD or another implementation rejects the result. Each level catches a class of mistake the level below cannot see.

The other direction asks what happens under unusual, hostile or expensive conditions:

| Technique | What it tries to reveal |
|---|---|
| Fuzzing | Inputs the developer did not think to write |
| Race detection | Failures caused by operations happening at the same time |
| Mutation testing | Tests which stay green after the code is deliberately broken |
| Allocation checks | Unexpected memory use in work which runs frequently |
| Benchmarks | Important work becoming unacceptably slow |

I group benchmarks with the safety checks rather than with performance work because uncontrolled CPU and memory use becomes an operational failure, and occasionally a denial-of-service weakness.

The two directions cross each other. A parser can be tested for correct output and then fuzzed with malformed input. A subsystem can be checked for its normal result and then run under the race detector. An application test can be checked by deliberately breaking the feature and confirming that the test turns red.

That last one is worth doing by hand when the automated version does not reach far enough. Mutation testing changes production code and checks whether the unit tests notice, which leaves application and interoperability tests uncovered. Claude 5 started closing that gap on its own: remove the fix, rebuild the actual program, confirm the scenario fails, restore the fix and confirm it passes. Nobody asked for it, and it catches tests which observe an unchanged path, along with tests which assert something that was already true before the feature existed.

Test volume alone creates false confidence. Ze's test-health page records volume, but it also looks for tests which nothing runs, requirements with only a positive or a negative case, and tests which never expect a specific error. The useful question is whether a plausible defect would make the evidence fail.

## Protect the proof from its author

An AI which sees a failing test will sometimes change the test to match its implementation. The result is green, internally consistent and wrong. Humans do this too, usually more slowly and with better excuses.

I described that machinery in [The proof is the expensive part](../the-proof-is-the-expensive-part/), so I will not repeat it here beyond the principle it rests on: the authoring process has to make every attempt to weaken the evidence visible, whether the author is a model or a person.

Claude has developed its own vocabulary while helping me build this. For every RFC `MUST`, Ze tests the valid case and the failure case. I called those positive and negative tests. Claude calls them the two polarities. Claude also calls important decisions load-bearing, so often that it has become an online joke. If this continues, I expect the larger ones to become load-banging. I normally cut the phrase from prose. This time the joke has earned its place.

## What this costs

I would rather not pretend this machinery is free.

The generated indexes are large. `ai/CODE-TO-DOCS.md` and `ai/DOCS-TO-CODE.md` are around a quarter of a megabyte each, and the RFC requirement ledger is over a megabyte. Nobody reads those files, but they are regenerated, checked and committed, and they make diffs noisier.

The six thousand cross-reference comments have to stay true. Every renamed file, every moved design document and every merged package is a small maintenance debt. The checks stop them rotting silently, which means they turn into failures somebody has to clear.

A full verification run is slow enough that I do not run it after every edit, so most of the day I run the cheap stages and rely on the full run before a commit is accepted. That is a real gap and I know it.

The hooks fire on code which is correct. A pattern which is right ninety-five times out of a hundred still blocks the other five, and arguing with a hook is more annoying than arguing with a person. When a rule turns out to be wrong, it has already been copied into hundreds of files by an agent which was doing exactly what it was told.

I still think the trade is worth it, because the alternative is reading every diff myself. None of that is automated, and it spends my time instead of machine time. It is a trade though, and a smaller project should pick the parts which pay for themselves rather than copying all of it.

## A project can build this incrementally

Ze's machinery is large because Ze is large and because we have been learning while building it. Another project can start with the useful core, roughly in this order.

1. **Make the project folder runnable.** Keep configuration, fixtures and working data in predictable places, and reduce the differences between development, testing and installation.
2. **Write a short architecture map.** Explain the major directories, what belongs in each one and which dependencies are allowed.
3. **Create one task-oriented index.** Route the common jobs to the relevant design, rule and example.
4. **Link source to design and back again.** Put the link where an agent opening the source will see it, generate the reverse index so no large table is maintained by hand, and fail the build when either side goes stale.
5. **Record the approved patterns.** Show how the project implements registration, configuration, commands, storage and the other repeated concerns.
6. **Turn repeated mistakes into checks.** Start with the cheap ones, for patterns which can be recognised reliably.
7. **Test for depth and under pressure.** Cover function, subsystem, application and third-party software where the feature needs it, then add fuzzing, race detection, mutation checks, memory limits and benchmarks according to the risk.
8. **Provide one verification command.** Run the cheap failures before the expensive evidence, and continue far enough to report every useful failure rather than stopping at the first.
9. **Make every failure useful.** Name the problem, the affected file, the expected pattern and the document which explains the correction.

The point of writing any of it down is retention. A prompt correction lasts for one conversation. A documented pattern, linked from the code and backed by a check, survives the next agent, the next human and the next year.

## The convention will come from a project

AI-assisted development is beginning to discover its Convention over Configuration, and we do not yet know which project will make it viral. The decisions do not have to be the best possible ones. They arrive attached to something people want.

Andrej Karpathy gave a small demonstration of that with his [LLM Wiki gist](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f). It describes an LLM maintaining a wiki made from ordinary Markdown files, with Obsidian as the editor. Raw sources stay separate, the agent maintains cross-linked pages, an `index.md` helps it find the right information, and an `AGENTS.md` or `CLAUDE.md` explains the structure. More complicated search can wait until the collection needs it.

The idea became extremely popular the moment he shared it, and many people had already reached the same answer independently. Ze was already using Markdown for its rules, designs and working memory, maintaining cross-links and generating indexes, all of it in the same text editors we write the code in. What Karpathy added was a clear explanation and a large audience, and for many people his version will become the version they know and copy.

A reference version gets copied whole. Excellent choices spread, awkward ones spread with them, and familiarity turns the whole set into the convention.

Ze may contribute to that shape, or another project may do it much better. I hope the one which becomes viral is a good one.

That creates a responsibility for Ze today. Its structure has to be understandable outside the people who created it. Its checks have to prove something useful. Its documentation has to explain why the machinery exists. Another AI should be able to inspect the repository, reproduce the useful parts and leave behind the complexity which only Ze needs.

ExaBGP could run straight out of its project folder because the design let the program find its environment, with no installation step in between. An AI-ready repository has to do the same thing for an agent, and let it find the project's architecture without already understanding it. That environment includes the source code, its relationships, the designs, the tests, the checks and the failure messages.

Ze has not found the solution, and we are still working on it. An article about AI written six months ago is probably already out of date, and this one describes summer 2026. Model releases arrive almost every month, and each one moves the boundary: a more capable model may make some of this machinery unnecessary, or it may use a richer structure correctly and justify adding more. If you are reading this later, check which of these problems have already been solved before copying anything.
