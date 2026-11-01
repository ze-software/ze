# How Ze is developed with AI

Most of Ze's code is written by AI coding agents. I know that makes some people uneasy, and I understand why. Many developers see good software as a craft, the work of a skilled person who cares about every line, and it is hard to see where that care fits when a machine writes the code. Ze is meant to run in networks that people depend on, so I need to be able to explain where the quality of its code comes from when no craftsman wrote it.

The answer came from somewhere I did not expect when I started, which is the way I have run a business for more than twenty years.

## Craft and industry

I understand the craft view from the inside. I wrote ExaBGP by hand over many years, and I know the satisfaction of a piece of code that is exactly right. Seen from there, ISO, which turns the way work is done into a defined and audited process, looks like the opposite of everything good about the job.

I thought so too when I started my career. I was firmly opposed to processes and saw them as useless constraints, and to be fair, a badly designed ISO system can be exactly that. It goes wrong the way a bad KPI does: people learn to satisfy the measure, the measure becomes the goal, and the customer's experience, which it was meant to protect, drops out of sight. I have since found that agents can play a bad measure just as readily as people do, so whatever process I give them has to keep the outcome in view, and I have to correct it whenever it starts rewarding the wrong thing. That means I still look at the code: its structure and the overall implementation. I do not read the details, because the details are what the tests are for. That is what test-driven development gives me, and it is why the number of tests in Ze grows with each commit.

Where I have ended up is that craft and industry suit different work. A bespoke piece of furniture, or a one-off solution for a single customer, benefits from one person's skill and taste. Cars and industrial machinery are not built that way any more, and I would not want to drive a car whose brakes depended on how its maker felt that morning. A network operating system feels much closer to the car to me. Its behaviour is written down in RFCs, it has to behave the same way every time, and it can be tested against what those RFCs say. For that kind of software I have found that craft adds little, while a good process catches what even a gifted engineer misses on a bad day.

The techniques that make software reliable have been written about for decades, and the models have read all of it. When it comes to applying them, I have found that an agent does better than a senior engineer, and I include myself in that. It does not innovate the way a person does, so the design and the decisions about what to build have stayed with me. Inside the right process, though, it produces quality, and building that process has become my part of the job.

## Where it started

I started using AI on ExaBGP, before Ze existed. ExaBGP had some long-standing design issues, and fixing any one of them would have taken weeks of development. With an agent, I only had to formulate the problem and the change I wanted, and it did the work. Some of those fixes came back elegant, and a few were better than what I had initially considered.

The first big job was grunt work: making ExaBGP's code pass type checking with mypy. It was hard going with Opus 4.5. More recently, Opus 5.5 handled fault isolation in ExaBGP very well: ExaBGP already stopped an exception raised while decoding a protocol message from bubbling up into the core engine, and the agent tidied that mechanism up into `with` statements that express it far more elegantly.

## How the models got here

For anyone not following AI development closely, the models have changed a great deal in a short time. In my experience, Claude Opus 4.5 was the first model that made agentic engineering possible, where an agent takes a piece of work from the request through to tested code while a person supervises. Even so, I had to fight Opus 4.5 and 4.6 to get the work done properly, and I hated working with 4.7 and 4.8. Opus 5.0 showed clear signs of progress, but its English was so hard to follow that working with it felt alien.

Opus 5.5 has raised the bar. Like OpenAI's Astra, it is a model I trust to achieve what I set it to do, and it finds problems in the code that earlier models could not spot. Most of the process on this page took shape while I was working with the earlier models, and it still matters with the newer ones, because a model that is good at its job still needs to be told what the job is.

## Agents need managing the way staff do

Running a team taught me that quality cannot depend on who happens to be at work that day. You write down how things are done, make it clear who is responsible for what, have someone other than the author check the work, and when something goes wrong, change the process so that it does not happen again. As a director I cannot do every task myself, and I would not want to. My job is to decide how the work should be done, make sure it was, and fix the process when it was not.

When I started working with agents, they reminded me of a new member of staff who is remarkably fast and knows the trade, but has never set foot in the company. An agent does not know my decisions, or which shortcuts I will not accept, and in two ways it is worse off than a new hire. It forgets everything when its session ends, so whatever I had told one agent in conversation, I had to tell the next one again. Several agents also work at the same time without knowing what the others were told. With people, a written process is good practice. With agents, I found it was the only thing that carried over from one session to the next.

So I ended up running Ze's development as a management system for agents. Its procedures are written down in the repository beside the code, and every agent session reads them, whichever AI tool is running it.

## Where the method comes from

I have run Exa Networks, an ISP, since 2003. Exa is certified to ISO 9001 for quality management and ISO 27001 for information security, and I supervised the implementation of both with our staff. That work taught me how much good work depends on the way it is organised, so when agents started getting things wrong, it was the first place I looked.

## The management system, practice by practice

Looking back, most of the practices a business uses to manage its staff now have a counterpart in Ze's repository. I did not design them up front. Each one was added after agents went wrong in a way I did not want to see twice.

<!-- table-columns: off -->
| Management practice | In Ze's repository | What happened before it |
| --- | --- | --- |
| Policy set by management | An [owner decisions file](https://github.com/ze-software/ze/blob/main/ai/META-DECISIONS.md) records scope, priorities and the questions that are already settled. | Agents kept asking questions I had already answered, and a question was often a polite way of dropping work that looked hard. An agent may now ask me how to do something, never whether to do it. |
| Documented procedures | The [rules](https://github.com/ze-software/ze/tree/main/ai/rules) live in one place that every AI tool reads. | An instruction kept in one tool's memory, or in one session's notes, was lost to the next session and to every other tool. |
| Responsibility and authority | A precedence order decides between rules: irreversible actions first, then correctness owed to people outside the project, then the agreed scope, then the current phase of work. | When two rules disagreed, an agent followed whichever suited it. Only I can widen or narrow the scope. |
| Information where the work happens | A short core of rules is always loaded, and the others load when the task matches their trigger. Each subsystem has a page listing the constraints any change to it must respect. | Loading every rule for every task buried the one that mattered. |
| Planning before work | A specification is agreed before any code is written. It states the acceptance criteria and names the existing mechanisms the design extends. | Agents started building at once, and sometimes invented a second mechanism next to one that already did the job. |
| Independent audit | A change is reviewed by an agent that did not write it, and a separate judge must approve any change that could reduce what a test proves. | Authors approved their own shortcuts, and failing tests were edited to match broken code. |
| Records as evidence | A test cited for an RFC requirement needs a recorded run in which the code was deliberately broken and the test failed. Every commit goes through one controlled script. | Some tests passed for the wrong reason, and a test like that proves nothing. |
| Corrective action | A defect found in passing is recorded and assigned to the specification that will fix it. Every rule has a written record of why it exists. | Defects found along the way were either forgotten or fixed in the middle of unrelated work, which delayed both. |
| Continual improvement | Rules are corrected when they mislead and removed when they stop helping. Automated checks look at what a command or file is, never at how a message is worded. | Rules kept out of habit outlived their purpose, and checks that misfired taught agents to ask for exemptions as a matter of routine. |

## The limits I have set

A few limits now apply whatever the task, and for several of them the tooling refuses the action outright. An agent may not:

- Weaken a test to make it pass, unless an independent judge has approved the change.
- Review its own work.
- Call work done before a test exercises it the way a user reaches it, through the CLI, the configuration or the wire.
- Run destructive version control commands, or touch work belonging to another session. Several agents work in the same checkout at once.
- Implement protocol behaviour from memory. It reads the RFC text, and where the RFC is silent, it reads the source of established open source implementations so that Ze interoperates with them.

## What has stayed my job

I still decide Ze's architecture and its scope, and I write the rules the agents work under. When an agent gets something wrong, I have learned that fixing the code is only half the job: I also look for the rule or the check that let the mistake through and correct it, so that the next session does not repeat it. It is much the same work I do at Exa, with a team that types a good deal faster.

## If you want to look for yourself

Everything I have described is in the repository: the [agent instructions](https://github.com/ze-software/ze/blob/main/AGENTS.md), the [owner decisions](https://github.com/ze-software/ze/blob/main/ai/META-DECISIONS.md) and the [rules](https://github.com/ze-software/ze/tree/main/ai/rules). The [quality pages](https://ze-software.net/quality/) describe how Ze is tested, and the [RFC compliance pages](https://ze-software.net/quality/rfc-compliance/) show which protocol requirements a test proves and which are still missing. Most figures on this site are generated from the code, so they follow the software rather than my opinion of it.

I have written about parts of this at more length on the blog. [The repository is half the AI harness](https://ze-software.net/blog/the-repository-is-the-ai-harness/) covers how the rules let an agent check its own change, and [The proof is the expensive part](https://ze-software.net/blog/the-proof-is-the-expensive-part/) covers why passing tests alone do not show that the software behaves correctly.

## Where it still falls short

Agents get things wrong, and so do people. The first version of a feature is often not fully correct, and proving it takes many passes, plenty of time and a fair amount of compute. For an RFC, every requirement has to be found, tested and shown to fail when the code is broken, and that work carries on long after the first implementation is published.

This website is probably where you will find the most mistakes. An RFC defines behaviour strictly enough for a test to check it, but nothing checks the free text of a web page in the same way. If you find something wrong here or in Ze, I would appreciate it if you could tell me on [Discord](https://discord.gg/T8s7CjPDne) or the [issue tracker](https://github.com/ze-software/ze/issues).

*This page was written with Claude. The method, the decisions and the account of my background are mine.*
