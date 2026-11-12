# How Ze is developed with AI

Most of Ze's code is written by AI coding agents. I know that makes some people uneasy, and I understand why.

Some developers object to how AI models were trained. They feel that AI companies took their work without permission, and regard that as theft. I do not share that view: I see no fundamental difference between a person learning from existing information and an AI being trained on it. But I recognise that others feel strongly about this and may refuse to use software written with AI on principle, whatever the process behind it.

Many developers also see good software as a craft: the work of a skilled person who cares about every line. It is hard to see where that care fits when a machine writes the code. Ze is meant to run in networks that people depend on, so I need to explain how I make sure the code is reliable when an agent writes it.

My approach to making AI development reliable draws on more than twenty years of running a business. At Exa Networks, I supervised the implementation of ISO 9001 for quality management and ISO 27001 for information security, and I found that many of those practices also helped me manage AI agents.

## Craft and industry

I understand the craft view from the inside. I put a lot of love and sweat into ExaBGP, and I know the satisfaction of seeing your vision coming to life. From that point of view, ISO can look like the opposite of everything good about the job: it requires work to follow what may feel like a lifeless process.

It’s how I felt about it when I started my career. I was firmly opposed to processes and saw them as useless constraints, and a badly designed ISO system can be exactly that. It can fail in the same way as a bad key performance indicator (KPI): people learn to satisfy the measure, and the customer's experience, which the measure was meant to protect, is forgotten. My experience with agents has confirmed that view: they can also overfocus on meeting a target and lose sight of the result it was meant to achieve. The process I give them has to keep the outcome in view, and I have to correct it whenever it starts rewarding the wrong thing.

I have come to see that craft can exist within an industrial process. A woodworker can use machinery to work faster while still expressing his vision and caring about the finished piece. But the machinery can limit the shapes he can make and take away the tactile pleasure of feeling the wood taking shape under his hands. It is a trade between control and speed. I recognise a similar trade in software development: agents let me realise my vision faster, while I give up some of the direct control and pleasure of working with the code myself.

The techniques that make software reliable have been documented for decades, and agents can draw on that knowledge. In my experience, an agent can apply those techniques more consistently than a senior engineer, myself included. However, this requires the project to define how the work must be done and ensure it is independently reviewed. To provide the equivalent of a fresh pair of eyes, that review must ideally be carried out by another agent starting with a fresh context.

Poor rules can still give both the author and the reviewer the same blind spot, allowing a problem to pass unnoticed. Developers can miss problems for similar reasons. This is why the software architect’s overview of the whole system remains important.

My role is to develop the design and decide how to approach the problem. The agent brings the skills and established practices to realise that vision, guided by the project's instructions and checks. However, this is a different role, and I understand that not everyone will find the same satisfaction in work that can feel more managerial and less creative.

## Where it started

I started using AI on ExaBGP, before Ze existed. ExaBGP had some long-standing design issues, and fixing any one of them would have taken weeks of development. With an agent, I could formulate the problem and the change I wanted, then delegate the implementation. Some of those fixes came back elegant, and a few were better than what I had initially considered.

The first big job was tedious work: making ExaBGP's code pass type checking with mypy. It was hard going with Opus 4.5. More recently, Opus 5.5 handled fault isolation in ExaBGP very well. ExaBGP already caught exceptions raised while decoding a protocol message before they reached the core engine, and the agent reorganised that mechanism using `with` statements that express it far more elegantly.

## How the models got here

In my experience, Claude Opus 4.5 was the first model that made agentic engineering possible: an agent could take a pre-agreed specification through to tested code while a person supervised. However, getting those earlier models to do the work properly was often a struggle. Most of the development process described here took shape during that period.

At the time of writing, I am using Opus 5.5, which has raised the bar. Like OpenAI's Astra, it is a model I trust to achieve what I set it to do, and it finds problems in the code that earlier models could not spot. It also often reports problems outside its assigned task that it discovers while analysing the code. I spend less time fighting the AI nowadays, but it still sometimes misunderstands the objective or leads me down rabbit holes that I have to dig myself out of.

## Where the method comes from

I have run Exa Networks, an ISP, since 2003. Exa is certified to ISO 9001 for quality management and ISO 27001 for information security, and I supervised the implementation of both with our staff. That work taught me how much good work depends on the way it is organised, and gave me a starting point for designing Ze's development framework.

## Agents need managing the way staff do

I am a software engineer by training, and writing software is a passion as well as a profession. I initially had to develop software for the business, but as it grew I had to accept that I could not do everything myself. As a director, my responsibility became setting the culture in which my R&D staff worked, with a clear development process, rules, quality standards and expectations.

From the start, I wanted to take the same approach with Ze: designing a system for developing the project alongside the software itself. The framework gives agents the instructions and checks they need to carry out the work correctly. That intention was there from the beginning; the detailed rules have evolved as problems exposed what was missing.

An agent needs context about the project to carry out its task correctly, including its goals, the decisions already made and the constraints on its design. That includes making sure past mistakes and the lessons learned from them are recorded. I keep that information in the repository beside the code, along with the development rules and procedures. Each session can then start with that context, and agents working in parallel have the same reference.

## The management system, practice by practice

Looking back, many of the practices a business uses to manage its staff now have a counterpart in Ze's repository. I could not anticipate everything or design every procedure up front. As with the ISO processes at Exa, I had to observe how the agents behaved and adapt the procedures as problems emerged.

We are fortunate that much of Ze's development consists of implementing RFCs. These provide carefully written specifications with precise requirements for how the software must behave, giving us a clear basis for writing tests. We can turn those requirements into checks of the implementation, with an agreed reference for what the correct result should be.

A change starts with an agreed specification describing what the software must do and how we will check it. An agent writes tests for those requirements and implements the change. Another agent independently reviews the code and tests against the specification. Problems found during review are corrected, and those corrections are reviewed again before the work is considered complete.

<!-- table-columns: off -->
| Management practice | In Ze's repository | Why I introduced it |
| --- | --- | --- |
| Policy set by management | An [owner decisions file](https://github.com/ze-software/ze/blob/main/ai/META-DECISIONS.md) records scope, priorities and the questions that are already settled. | Agents kept asking questions I had already answered, and a question was often a polite way of dropping work that looked hard. An agent may now ask me how to do something, never whether to do it. |
| Documented procedures | The [rules](https://github.com/ze-software/ze/tree/main/ai/rules) live in one place that every AI tool reads. | An instruction kept in one tool's memory, or in one session's notes, was lost to the next session and to every other tool. |
| Responsibility and authority | The rules state which instruction takes priority when two instructions conflict. For example, an instruction to finish the work should never override a ban on destructive actions. | When two rules disagreed, an agent followed whichever suited it. Only I can widen or narrow the scope. |
| Information where the work happens | Every session starts with a small set of essential rules. The agent reads additional rules according to its task, along with the constraints documented for the part of the software it is changing. | Loading every rule for every task buried the one that mattered. |
| Planning before work | A specification is agreed before any code is written. It states the acceptance criteria and names the existing mechanisms the design extends. | Agents started building at once, and sometimes invented a second mechanism next to one that already did the job. |
| Independent audit | A change is reviewed by an agent that did not write it. Any change that could reduce what a test proves must also be approved by an independent agent acting as a judge. | Authors approved their own shortcuts, and failing tests were edited to match broken code. |
| Records as evidence | A test cited for an RFC requirement needs a recorded run in which the code was deliberately broken and the test failed. Every commit goes through one controlled script. | Some tests passed for the wrong reason, and a test like that proves nothing. |
| Corrective action | A defect found in passing is recorded and assigned to the specification that will fix it. Every rule has a written record of why it exists. | Defects found along the way were either forgotten or fixed in the middle of unrelated work, which delayed both. |
| Continual improvement | Rules are corrected when they mislead and removed when they stop helping. Automated checks should base their decisions on the commands being run and the files being changed, so a harmless change in wording does not block the work. | Rules kept out of habit outlived their purpose, and checks that misfired taught agents to ask for exemptions as a matter of routine. |

I still look at the code's structure and overall implementation, while relying on tests to check the details. I trust the development process to ensure those tests are correctly designed, through independent review and checks that catch mistakes in how they are written.

Agents get things wrong, and so do people. The first version of a feature is often not fully correct, and proving it takes many passes, plenty of time and a fair amount of compute. For an RFC, every requirement has to be found and tested, and each test must be shown to fail when the code is broken. That work carries on long after the first implementation is published.

## The limits I have set

A few limits now apply whatever the task, and for several of them the tooling refuses the action outright. An agent may not:

- Weaken a test to make it pass, unless an independent agent acting as a judge has approved the change.
- Review its own work.
- Call work done before a test exercises it through the same interface a user would use: the CLI, the configuration or the wire.
- Run destructive version control commands, or touch work belonging to another session. Several agents work in the same checkout at once.
- Implement protocol behaviour from memory. An agent reads the RFC text and, where the RFC is silent, the source of established open source implementations so that Ze interoperates with them.

## What has stayed my job

When an agent gets something wrong, I look beyond the code fix to the rule or check that allowed the mistake through. I correct the process so that the next session does not repeat it, which can be frustrating. It is much the same work I do at Exa, although the replies I get when I challenge an AI are rather different.

## If you want to look for yourself

Everything I have described is in the repository: the [agent instructions](https://github.com/ze-software/ze/blob/main/AGENTS.md), the [owner decisions](https://github.com/ze-software/ze/blob/main/ai/META-DECISIONS.md) and the [rules](https://github.com/ze-software/ze/tree/main/ai/rules). The [quality pages](https://ze-software.net/quality/) describe how Ze is tested, and the [RFC compliance pages](https://ze-software.net/quality/rfc-compliance/) show which protocol requirements a test proves and which are still missing. Most figures on this site are generated from the code, so they follow the software rather than my opinion of it.

I have written about parts of this at more length on the blog. [The repository is half the AI harness](https://ze-software.net/blog/the-repository-is-the-ai-harness/) covers how the rules let an agent check its own change, and [The proof is the expensive part](https://ze-software.net/blog/the-proof-is-the-expensive-part/) covers why passing tests alone do not show that the software behaves correctly.

## Where it still falls short

This website is probably where you will find the most mistakes. For protocol code, an RFC gives us defined behaviour against which to write tests. The free text of a web page has no equivalent specification, so checking its meaning and accuracy is harder. I am quite happy to let AI turn my ideas into prose, and I sometimes trust the resulting copy a little too readily. If you find something wrong here or in Ze, I would appreciate it if you could tell me on [Discord](https://discord.gg/T8s7CjPDne) or the [issue tracker](https://github.com/ze-software/ze/issues).

*This page was written with Claude and Codex. The method, the decisions and the account of my background are mine.*
