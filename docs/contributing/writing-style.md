# Ze Writing Style

Ze is read by network operators in many countries. English is a second language
for most of them. A sentence that a reader must read twice costs an
operator time. A sentence that a reader misunderstands becomes a misconfigured
router.

This page is the working standard for every word in the repository. It carries
the whole style layer, so you never need a second document to learn how to write
a Ze sentence. It does not replace Ze's other rules. Naming, config naming, YANG
structure, CLI grammar, and error messages each own their own surface, and this
page names them where they apply.

A second reason to write this way: a controlled vocabulary and one sentence
construction per idea make a text easier to translate. That covers a human
translator, a machine translation engine, and the agents that read this
repository.

The rule that makes this blocking is `ai/rules/writing.md`.
The checker is `internal/le/ste.Answer`.
<!-- source: internal/le/ste/actions.go -- Answer -->

## What this covers

| Surface | Covered |
|---------|---------|
| `docs/` guides, references, comparisons, architecture pages | Yes |
| Code comments and godoc | Yes |
| Error messages, log lines, diagnostic remediation text | Yes |
| CLI output, help text, completions, TUI labels | Yes |
| YANG `description` strings | Yes |
| `ai/` rules and patterns, `plan/` specs and learned summaries | Yes |
| Commit messages and PR text | Yes |
| The website and the wiki | Yes for `website/`. No for generated files in `../gh-pages` |
| Identifiers, keys, tokens, quoted external text, fixture data | No. Read "Words that never change" |

## The six habits to avoid

Learn these six first. They cause most of the confusion in technical prose, and
the checker finds all six.

### 1. Synonym rotation

Give each concept one name. Use that name every time. Repetition is the feature,
and variety is the defect. A reader who meets a second name asks what changed. A
reader who greps for one name must find every instance.

```
Avoid:  The peer sends an UPDATE. The neighbor holds the route. The session
        then withdraws it.
Write:  The peer sends an UPDATE. The peer holds the route. The peer then
        withdraws it.
```

Ze names, and the words that are not Ze names:

| Use | Never |
|-----|-------|
| `peer` | `neighbour` |
| `plugin` | `plug-in` |
| `dataplane` | `data plane`, `data-plane` |
| `hostname` | `host name` |
| `filename` | `file name` |
| `runtime` | `run-time` |
| `startup` | `start-up` |
| `nftables` | `nft tables` |

Words that look like synonyms and are not: `delete` acts on config, `clear` acts
on counters, and `remove` acts on a route. Three operations, three words. Keep
them apart.

**The one name is the plain name.** English gives four or five words for most
actions, and the discipline is to keep the shortest common one and drop the
formal ones. This is the core of the standard, and it does more for a reader with
limited English than any other rule on this page.

| Use | Not |
|-----|-----|
| start | `initiate`, `commence`, `begin` |
| stop | `terminate`, `cease` |
| get | `obtain`, `acquire` |
| use | `utilize` |
| help | `assist`, `facilitate` |
| make sure | `ascertain` |
| remove | `eliminate` |
| more | `additional` |
| through | `via` |
| applicable | `appropriate` |
| permitted | `acceptable` |

Domain verbs stay. `implement`, `negotiate`, `withdraw`, `advertise`, and
`originate` name what Ze does, and no plainer word means the same thing. The
target is the formal word that adds nothing, never the precise word.

**One word, one part of speech.** A word that Ze uses as a noun stays a noun.

```
Avoid:  Peer with the route server, then RIB the prefix.
Write:  Configure a peer to the route server, then install the prefix in the RIB.
```

Three pairs catch most writers, because ordinary English allows both forms and
this discipline allows one:

| Word | Approved as | So write |
|------|-------------|----------|
| `work` | a noun, never a verb | `do work on the parser`, never `work on the parser` |
| `help` | a verb, never a noun | `with the aid of a second operator`, never `with the help of` |
| `damage` | a noun, never a verb | `cause damage to the store`, never `damage the store` |

### 2. Hedging

State the fact. Use CAN for a possibility, MUST for an obligation, and WILL for
a future event. When you do not know, write that you do not know.

```
Avoid:  The reactor may drop the buffer, so the pool should usually recover.
Write:  The reactor drops the buffer. The pool then reclaims it.
```

Every word below is a hedge. Delete it, or replace it with the fact.

| Hedge | Write |
|-------|-------|
| `may`, `might`, `could` | CAN, or state the condition |
| `should` | MUST |
| `generally`, `normally`, `typically` | `usually` |
| `probably`, `possibly`, `perhaps`, `arguably` | the fact, or "I do not know" |
| `seemingly`, `apparently`, `presumably` | what the code does |
| `essentially`, `basically`, `hopefully` | nothing. Delete the word |
| `simply`, `easily` | nothing. Delete the word |
| `somewhat`, `relatively`, `fairly` | the number |
| `in most cases`, `in general` | `usually` |
| `it seems`, `appears to`, `we believe` | what the code does, or the source |

`usually` is a correct word and is the fix for three of the rows above.

**A replacement must keep the meaning.** When the plain word changes what the
sentence claims, change the construction instead. `should` is the common trap:
MUST is right for an obligation, and wrong for a recommendation. Write the fact
("the pool reclaims the buffer") rather than promoting a suggestion into a
requirement.

**Do not put `must` in front of an imperative.** Write `Stop the forwarding
worker`, never "you must stop the forwarding worker". The exception is a safety
instruction, or a condition the reader has to meet first.

### 3. Frozen verbs

An action is a verb. A noun that hides an action makes the reader unpack it.

```
Avoid:  Do the installation of the plugin before the removal of the unit.
Write:  Install the plugin before you remove the unit.

Avoid:  The daemon performs validation of the config.
Write:  The daemon validates the config.
```

The defect is the noun, not the verb that carries it. Look for a noun that ends
in `-tion`, `-ment`, `-ance`, or `-al` behind `do`, `perform`, `make`, `conduct`,
or `carry out`. When a verb for that action exists, use the verb.

When no verb exists for the action, that same construction is correct. `Do a
check of the buffer` is right, because this discipline approves `check` as a noun
and not as a verb.

### 4. Marketing adjectives

Give the number, the limit, or the mechanism. Then delete the adjective. Praise
is not information, and a reader who wants a fact reads past it.

```
Avoid:  Ze has a powerful, seamless forwarding path.
Write:  Ze forwards an UPDATE to 400 peers from one buffer, with no copy.
```

Words to delete: `powerful`, `seamless`, `robust`, `blazing`, `effortless`,
`feature-rich`, `cutting-edge`, `world-class`, `best-in-class`, `battle-tested`,
`enterprise-grade`, `bulletproof`, `rock-solid`, `turnkey`, `next-generation`,
`revolutionary`, `game-changing`, `unparalleled`, `amazing`, `incredible`,
`stunning`.

`docs/comparison.md` has its own rule for this: every capability claim carries
evidence or an explicit "unclear" (`ai/rules/writing.md`).

### 5. Run-ons

Write one topic in each descriptive sentence. When a sentence carries two
topics, split it. When a long sentence carries many items, a vertical list is the
clearer form. A short inline series stays correct.

| Limit | Value |
|-------|-------|
| Words in a procedural sentence: a guide step, a runbook line, a remediation line, a commit message | 20 |
| Words in a descriptive sentence: architecture prose, a comment, a reference page | 25 |
| Sentences in a descriptive paragraph | 6 |
| Topics in a descriptive sentence | 1 |
| Instructions in a procedural sentence | 1, unless two actions happen at the same time |
| Semicolons | 0 |
| Words in each item of a vertical list | Each item is its own sentence, so 20 in a procedure and 25 in a description. The limit applies before the colon AND to every item after it |

The semicolon is banned. It joins two sentences that the reader must hold at
once, and it hides the relation between them. Write two sentences, or write a
vertical list.

```
Avoid:  The peer holds the route until the hold timer expires; then the reactor
        withdraws it and the RIB releases the attribute, which returns the
        buffer to the pool.
Write:  The peer holds the route until the hold timer expires. The reactor then
        withdraws the route. The RIB releases the attribute, and the buffer
        returns to the pool.
```

### 6. Phrasal verbs

Use one verb. A verb plus a preposition takes a meaning that neither word
carries, and a reader who knows both words still guesses.

| Avoid | Write |
|-------|-------|
| `spin up`, `kick off` | start |
| `figure out`, `look into` | find, diagnose, examine |
| `get rid of` | delete, remove |
| `put out` | extinguish |
| `give off` | release |
| `come up with` | write, select |
| `end up` | become, result in |
| `sort out`, `work out` | correct, resolve, calculate |
| `take care of` | do, manage |
| `tear down` | stop, remove |
| `set up` | install, prepare, configure |

Ze's one-word nouns are correct and are not this habit: `setup`, `teardown`,
`shutdown`, and `backoff` name things. The command `request peer teardown` is
right. The sentence `tear down the peer` is wrong.

## Choosing words

The habits above say what to avoid. These four say how to choose.

**One word, one meaning.** A word that carries two senses on one page costs the
reader a guess. `follow` means "come after", so `the steps that follow` is right and a
checklist is obeyed, never followed. Keep `check`, `address`, `handle`, `monitor`, and
`state` to one sense everywhere, and pick a different word for the other sense.
A few words carry two approved senses, so the test is the meaning, never a count.

**No slang, no jargon, no regional words.** These are the words a reader with
limited English cannot look up, because a dictionary gives the wrong meaning.

| Avoid | Write |
|-------|-------|
| `brick the appliance` | make the appliance unbootable |
| `bounce the daemon` | restart the daemon |
| `nuke the config`, `blow away the state` | delete the config, delete the state |
| `footgun`, `gnarly`, `hairy` | name the actual risk |
| `sanity check` | validate, or verify |

**Use the plain word unless the technical one earns its place.** Write for a
capable reader who knows computing but not this repository: a good teacher does
not simplify the idea, only the words around it. Reach for a technical term when
it names something exactly and the plain word would lose that meaning. Everywhere
else the plain word is the better one, and a term the reader must decode is a
cost with no return.

The test is whether you would say the sentence out loud to a colleague who has
not read our code. If you would not, write the sentence you would say.

| Avoid | Write |
|-------|-------|
| `N gated requirements` | N requirements the check enforces |
| `the feature is gated behind ze_bgp` | the feature is compiled out unless `ze_bgp` is set |
| `surface`, `artifact`, `mechanism` where a plain noun fits | the page, the file, the check |
| `leverage`, `utilize`, `perform validation of` | use, use, validate |
| `the request is serviced by the handler` | the handler answers the request |

`gated` is the standing example of a word to remove. It tells the reader that
something is restricted, but not by what. Name the thing that restricts.

The opposite mistake is real too. Terms the code defines are exact, and replacing
them breaks the link between the prose and the code. `carrier`, `polarity` and
`disposition` name `CARRIERS`, the `RFC requirement: <ID> <polarity>` tag
grammar, and the classification in `rfc/extraction/*.json`. Keep those, and
expand them on first use.

**Three words in a noun stack, no more.** The head noun is usually the last word,
so every modifier in front of it is one more thing to hold. Break a longer stack
with a preposition. Hyphens bind words into one unit, and three units is still
the cap. `adj-rib-in prefix-limit counter` is three units, and
hyphenating a whole stack into one unit is wrong.

```
Avoid:  forward congestion pool budget calculation
Write:  calculation of the budget for the forward congestion pool
```

**Expand an abbreviation on first use.** Write the full term the first time it
appears in a document, with the short form after it. Then use the short form.
A term of three words or fewer needs no abbreviation at all.

You can also define a shorter NOUN rather than an abbreviation, then use that.
Which abbreviations exist is Ze's choice. Expanding one on first use is not.

**Prefer the plain verb over the specialist one.** When an ordinary word says the
same thing, use it: `delete` rather than `reap` or `evict`, `remove` rather than
`drain`, `stop` rather than `quiesce`. Drop the specialist verb whenever an
ordinary word CAN say the same thing. Keep it only when no ordinary word carries
the meaning.

## Verbs and voice

| Rule | Detail |
|------|--------|
| Active voice | Name the actor. "The reactor drops the buffer", never "the buffer is dropped" |
| Passive voice | Only when the actor is genuinely unknown, and only in descriptive text |
| A state is not the passive | `the peer is configured` and `the route is installed` describe a condition. The past participle after `is`, `becomes`, or `stays` is an adjective, so keep it |
| Imperative for instructions | "Install the plugin", never `the plugin should be installed` |
| Simple tenses only | Simple present, simple past, simple future. No auxiliary stacks |
| No auxiliary chains | "The peer sends", never "the peer will have been sending" |
| Past participle as an adjective | "the configured peer" is correct |
| `-ing` only as a noun or a modifier | `routing table`, `forwarding worker`, and the heading `Troubleshooting` are correct. A gerund clause is not: write `before you install`, never `before installing` |
| One instruction in a sentence | Two actions in one sentence hide the second one. Two actions that happen at the same time are the exception: `Withdraw and release the route` is correct |

## Articles and demonstratives

| Rule | Detail |
|------|--------|
| Use an article or a demonstrative before a noun | `Set the log level to debug`, never `Set log level to debug`. Telegraphic writing is the default register of CLI help and log lines, and it costs clarity |
| No article in a general statement | `Buffers can cause memory pressure`. An abstract concept takes no article either |
| A long series takes one article, at the front | `Delete the peers, filters, and route maps` |
| A short sentence takes an article before each noun | `Install the plugin and the schema`. In a short sentence this CAN read more clearly |
| Watch what an adjective claims in a series | `the new buffers, slots, and handles` says all three are new. Repeat the article when only the first one is |
| **No definite article before a noun with an alphanumeric identifier** | The identifier makes it a proper noun. Write `peer 10.0.0.1`, `RFC 4271`, `table 254`, and `AS65001`, never `the RFC 4271` |

## Connecting sentences into a paragraph

Short sentences are half of the discipline. Joining them is the other half, and
it is the half that decides whether a page reads as prose or as a list of
assertions.

| Rule | Detail |
|------|--------|
| Give information gradually | Release one subject per sentence, then build on it |
| Repeat the key word in the next sentence | The repeated word IS the link. This is habit 1 used as a construction technique, not as a lint rule |
| Open each paragraph with a topic sentence | It states the paragraph's topic and connects it to what came before. Read the topic sentences alone and you have the page's outline |
| One topic in a paragraph | A second topic starts a second paragraph |
| Join related sentences with a connector | `and`, `but`, `then`, `thus`, `as a result`, `at the same time`. The list is open, and a demonstrative adjective connects too |
| A sentence CAN start with `And` or `But` | Both are correct. This discipline prints them as correct |

```
Avoid:  The reactor drops the buffer. The pool reclaims memory. The peer
        retries.
Write:  The reactor drops the buffer. The pool then reclaims that buffer. As a
        result, the peer retries the UPDATE.
```

## Sentences that carry a condition

Put the condition first. Then a comma. Then the command.

```
Write:  If the session is established, send the End-of-RIB marker.
Write:  Before you delete the peer, stop the forwarding worker.
```

## Notes

| Rule | Detail |
|------|--------|
| A note gives information | It never gives an instruction. When you need an instruction, write a step |
| No imperative inside a note | The imperative marks an instruction, and a note is not one |
| No requirement, no limit, and no result in a note | A limit or a result belongs in the work step it bounds, right after the action |
| Damage or injury information is not a note | It becomes a warning or a caution |
| A note sentence takes the descriptive limit | 25 words, not 20 |
| A note belongs in a procedure | In descriptive text, write a note only when an illustration or a table needs one. An architecture page rarely needs any |
| The test | Delete every note and read the procedure again. A reader who can no longer finish it means the note was carrying a step |

## Warnings and cautions

Three parts, in this order: the level word, the command or condition, and then
the consequence whenever you can state it.

| Level | Means |
|-------|-------|
| WARNING | A risk of injury or death |
| CAUTION | A risk of damage to equipment or data |

```
Write:  CAUTION: Before you delete the zefs database, make a backup. A deleted
        database cannot be recovered.
```

Analyze the risk before you pick the level. When a step risks both injury and
damage, it is a WARNING.

A safety instruction is procedural, so the 20-word sentence limit applies.
Uppercase is not part of the discipline. This standard sets no formatting rules,
so the case, color, and symbol of a safety block are Ze's own choice. A symbol
CAN carry the level in place of the word, and other level words are permitted
when the three parts above stay intact. Ze uses WARNING and CAUTION only.

## Punctuation, and how words are counted

Every standard English mark is permitted, with one exception. This discipline
sets no general punctuation rules beyond that, so ordinary US English practice
applies.

| Mark | Use |
|------|-----|
| Semicolon | The one banned mark. Write two sentences, or a vertical list |
| Contractions | Never. Write "do not", never "don't" |
| Hyphen | Five cases. Two or more words acting as one adjective before a noun (`route-server session`, `read-only store`). A two-word number or fraction (`sixty-four`, `one-eighth`). An uppercase letter or a number before a noun, giving shape or configuration (`4-byte ASN`, `64-slot ring`). A verb whose first part is another part of speech (`hot-swap`, `cross-compile`). A prefix ending in a vowel before a root starting with one (`re-enable`, `pre-allocate`) |
| Parentheses | Seven uses. A reference to text or an illustration. An identifier for an item. An identifier for a work step. An abbreviation. The singular and plural at once (`the peer(s)`). An explanation. An alternative |
| Colon | Any standard use, including the stem of a vertical list |

The word limits count some groups as a single word:

| Counted as one word | Example |
|---------------------|---------|
| A number | `4096` |
| A number with its unit | `4096 bytes`, `30 seconds` |
| An abbreviation or an initialism | `NLRI`, `RIB` |
| An alphanumeric identifier | `AS65001`, `ze_core` |
| Quoted text | `"no such peer"` |
| A title, a heading, a label | `show bgp peer list` |
| A proper noun of a person, a group, an organization, or a geopolitical entity | `Thomas Mangin`, `Internet Engineering Task Force`. The category is closed, so a product name is not in it. `IETF` counts, but as an abbreviation |
| A hyphenated word | `route-server` |
| Text inside parentheses | `(see the note above)` |

A colon in a vertical list ends a sentence, exactly as a period does. Two limits
follow from that, and they govern every bulleted list and table row in `docs/`.
The stem before the colon carries the full limit on its own, 20 words or 25. Each
item after the colon is a new sentence and carries the limit again.

Two more counting rules matter. Text in parentheses counts as one word in its
host sentence, and it also forms a sentence of its own that carries its own
limit. A number that identifies a work step or a paragraph is not counted at
all.

## Recommendations that prevent ambiguity

Eight habits sit beside the rules. Each one removes a specific way a reader loses
a sentence.

| # | Recommendation |
|---|----------------|
| 1 | **Keep the conjunction `that`.** Write "Make sure that the peer is established", never "Make sure the peer is established". The dropped `that` forces the reader to re-parse, and it also makes translation harder |
| 2 | **Re-read every `with`.** It can mean an association, help or sharing, or a means or instrument. Name the action instead: `Filter the routes with the community list` beats `use the community list to filter the routes`, because the primary verb comes first |
| 3 | **A pronoun with two candidates becomes a noun.** When `it` points at either the peer or the route, write the noun again. The pronoun set is closed, so `he` and `she` are out (see recommendation 7) |
| 4 | **`This` needs a referent the reader can find.** When `this` points at more than one candidate, give the context again. A commit body that opens "This fixes..." names a diff the reader cannot see |
| 5 | **Watch false friends.** A word that looks like one in your own language often means something else in English. Check the English meaning |
| 6 | **No Latin abbreviations.** Write `for example` rather than `e.g.`, `that is` rather than `i.e.`, and `and so on` rather than `etc.` |
| 7 | **Gender-neutral language.** The operator is `they`. Never `he` or `she`. Use `man` or `woman` only where the context genuinely needs it |
| 8 | **The possessive is permitted, and doubt is the test.** `the peer's route` is fine. When you are unsure the sentence is correct, write it without the possessive |

## Restricted meanings

An approved word keeps its approved meaning, and its other English meanings stay
out. Three of these appear constantly in Ze prose.

A word also keeps one part of speech. A few words carry two, so the test is the
meaning and never a count.

| Avoid | Write |
|-------|-------|
| `above 4096 bytes`, `below the threshold` | `more than 4096 bytes`, `less than the threshold`. Keep `above` and `below` for physical position |
| `see that the peer is up` | `make sure that the peer is up` |
| `follow the release checklist` | `obey the release checklist`. Keep `follow` for "come after", as in `the steps that follow` |
| `work` as a verb | `work` is a noun. Write `do work on the reactor`, or name the action |
| `help` as a noun | `help` is a verb. Write `the loader helps the plugin start`, not `with the help of the loader` |
| `damage` as a verb | `damage` is a noun. Write `cause damage to the buffer` |

## Words that never change

| Surface | Why |
|---------|-----|
| RFC 2119 keywords (MUST, MUST NOT, SHOULD, MAY) naming an RFC obligation level | The keyword IS the requirement. Every `RFC requirement:` test tag reads the exact word. The `should` to MUST replacement never applies to a quoted normative term |
| Quoted external text: RFC prose, vendor output, peer daemon logs | A quotation is evidence. An edited quotation is false evidence |
| Go identifiers, YANG leaf names, JSON keys, env var keys, CLI tokens | `ai/rules/go-standards.md`, `ai/rules/config.md`, `ai/rules/config.md`, and `ai/rules/cli.md` own these names |
| Technical nouns of this domain: `peer`, `prefix`, `NLRI`, `netlink`, `teardown` | A domain noun is precise. Keep it, and keep one spelling |
| Fenced code blocks, hex dumps, config samples, fixture data | These are data, not prose |

## Per-surface notes

| Writing | Extra requirement |
|---------|-------------------|
| A `docs/` page | Verify each factual claim against the source, then add a `<!-- source: -->` anchor (`ai/rules/writing.md`) |
| An error message | Name what failed, the offending value, and the next action. Keep the leading phrase stable so it stays greppable (`ai/rules/cli.md`) |
| A code comment | Say why, and state a caller obligation with MUST (`ai/rules/go-standards.md`) |
| A YANG `description` | One sentence on what the leaf MEANS. Never prescribe a CLI spelling (`ai/rules/config.md`) |
| A YANG `description` on a COMMAND node or an RPC | The one-line summary: one sentence, 25 words at most, one line, no semicolon, and it ends in a full stop. `./le docvalid help-shape` names the path and the rule when it does not |
| A YANG `ze:help` | The long explanation the help page of that one command or rpc prints. It can be several sentences over several lines, and STE reviews it as prose |
| A `registry.Meta.Description` on an offline local command | The same one-line summary a command node owes, judged by the same gate. The catalog publishes it where the command reaches no YANG node |
| A `registry.Meta.LongHelp` | The long explanation, in the same shape as a `ze:help` |
| A commit message | Procedural limits apply: 20 words in a sentence, and an imperative subject (`ai/rules/completion.md`) |
| A spec or a learned summary | The same limits. A spec is read by the next agent under time pressure |
| A guide step | Imperative, one instruction, 20 words |

## Spelling

US English, with no exceptions in the repository: `color`, `behavior`,
`initialize`, `license`, `analyze`. The one exception is prose in Thomas's own
voice, which keeps UK English and which this page does not govern
(`ai/rules/writing.md`).

## Detail budget

Write what changes the reader's next action, and write nothing else. Detail is a
cost the reader pays, not proof that you did the work. A fact the reader can
recover in seconds by opening the code does not get written down.

- Cite a location so the reader can NAVIGATE, never to show that you looked.
  Name the file and the symbol: `session.go` `Session.Run`. A line number is
  correct when the line IS the fact, or when a gate or a generator pins it.
- One example for one point. A second example earns its place only when it shows
  a different reading.
- When a directive can be read two ways, write both readings and name the one
  that governs. More examples hide an ambiguity.
- Do not make the same cut twice. When a table and a paragraph draw the same
  distinction, keep the table and delete the paragraph.
- State the obligation, name the gate, and stop. How a gate is implemented lives
  in the code and its fixtures.
- Report the conclusion, not the search. What you tried, in what order, and how
  long it took are yours.
- Give a count plus the exceptions, not a row per item. "12 call sites updated,
  2 refused and listed below" is complete.
- A pointer line points, and it does not summarise. An entry in `ai/INDEX.md`,
  or in any other index, says what the target answers and then stops, under 120
  characters after the link.
- Ask for a decision the way you would ask a colleague. Say the thing, then the
  reason. Drop the machinery from the sentence. Do not stack qualifiers. Plain
  is not loose: say exactly what happened, in the words a person would use.

Over budget means CUT. Do not split a long record into two documents.

| Artifact | Contains | Budget |
|----------|----------|--------|
| Subagent report to the main thread | the conclusion, the evidence that would overturn it, open questions | under 40 lines |
| Review finding | the claim, where it lives, how it fails | 3 lines |
| Commit subject | what changed, imperative | one line |
| Commit body | the defect, its cause, what the fix does | under 15 lines |
| Known-failure shard | the failing output, the repro command, the next step | under 20 lines |
| Learned summary | what the code cannot tell a future reader | 25 to 35 lines |
| Index or pointer line | what the target answers | under 120 characters after the link |
| Rule file | trigger, directives, one example for each | under 150 lines |

These patterns are banned:

| Banned | Why |
|--------|-----|
| Recounting dead ends, wrong hypotheses, or the order you tried things | The reader needs the answer, not the route to it |
| Any sentence about the difficulty or the size of the work | It changes no action |
| Restating a fact in the next paragraph, or "as noted above" | Say it once, in the place the reader looks first |
| A line number for a claim the symbol name already locates | It rots at the next edit and forces a re-index |
| Pasting a whole file, table, or log when the answer is one row | Quote the row |
| A third example to settle an ambiguity that two readings would settle | The ambiguity survives, now hidden |

## Comparison honesty

A product comparison is advice, never marketing. A comparison can create tension
between projects, so every claim helps the reader choose the right tool rather
than make Ze look better.

1. Cite every capability claim with a durable source. Prefer upstream source code
   links for implemented behavior, and official feature documentation when the
   code is not practical to cite. For an integrated product, cite the wrapper
   surface and the integrated project where the runtime behavior lives. For VyOS
   routing features, cite the VyOS config and templates plus FRR documentation
   or source.
2. State the comparison scope before the matrix. Name the inspected checkout,
   release, branch, commit, docs page, or upstream feature page. Say that
   `not found` means not found in the inspected scope.
3. Label uncertainty instead of turning it into a gap. Use `Unclear` when the
   evidence is incomplete, and `Partial` when a narrower feature exists that is
   not equivalent. Separate similarly named features, such as IS-IS L1/L2 route
   leaking against cross-VRF route leaking.
4. Do not cherry-pick categories to favor Ze. Include the other products'
   equivalent strengths, say where Ze is behind and cite the evidence, and
   describe delegation to an integrated daemon neutrally.
5. Make a wide table user-controllable. A page with three or more product
   columns provides keyboard-accessible controls to hide products, and those
   controls never delete evidence from the source document.

Every row carries its evidence or an explicit caveat, and a row with neither is
removed. A public comparison opens with its inspected scope, near the top:

    Scope: inspected <projects/versions/paths>. Claims cite code or official
    docs. Integrated products cite their integration surface and the integrated
    implementation when relevant. `Not found` means not found in this inspected
    scope.

## Config examples in docs

Write every config example one statement per line, and do not collapse it onto
one line. The multi-line form is the house style. An agent that reflows an
example onto one line, followed by an agent that reflows it back, costs two
diffs and tells the reader nothing.

The one-line form is also a syntax error unless the last statement carries its
semicolon. Automatic semicolon insertion fires at a newline, so a closing brace
on the same line as the statement it closes ends the block before the statement
ends. Measured with a built `ze config validate`:
`attach process bgp-rr { receive [ update ] }` is refused with
"expected ';' after receive, got RBRACE", and
`attach process bgp-rr { receive [ update ]; }` is accepted. An operator copies
what a guide shows, so a guide never shows the refused form.

An inline mention inside a sentence or a table cell can stay on one line, and it
then carries the semicolon: `internal rib { use bgp-rib; }`. Reflowing a phrase
into a block would break the sentence around it.

Every config example in `docs/` parses, and an excerpt parses inside the
smallest complete config that carries it. Build the binary and run
`ze config validate` over that config before you publish the example.

## How to check your writing

| Command | What it does |
|---------|--------------|
| `./le ste review-changed` | Every finding in the files you changed, with the line and the replacement |
| `./le ste check` | The gate. It fails when a habit grew in a file you changed |
| `./le ste review` | The whole tree, for a rewriting session |
| `./le ste check file <path>...` | Named files |
| `./le ste check file <scratch>/msg.txt` | A commit message or a PR body, written to a file first. `namedFiles` in `internal/le/ste/actions.go` takes paths only, and the package reads no stdin |

The gate compares each file against its own version at HEAD. Legacy prose in a
file you touch costs nothing, and the sentence you add is what goes red. No
baseline file exists, so the one way to green is to rewrite the prose.

When the checker is wrong, correct the checker and add the case to
`internal/le/ste.Answer`. A checker that flags a code span, an RFC 2119
MUST, or the noun `setup` teaches its readers to ignore it.

When a document must quote non-conforming text at length, exempt it with
`<!-- ste: ignore-file <reason> -->` on its own line. The reason is required.

A document that is DELETED when its work closes is out of scope, and rewriting
its prose is wasted work. `plan/spec-*.md`, `plan/deferrals/` and
`plan/known-failures/` are excluded in `internal/le/ste/ste.go`.
`plan/journal/`, `plan/learned/` and `plan/TEMPLATE.md` stay in scope: they
outlive every spec and are read by sessions that were not there.
<!-- source: internal/le/ste/ste.go -- excluded plan trees -->

## Lineage

This standard follows the discipline of Simplified Technical English, the
controlled-language standard ASD-STE100. The aerospace industry wrote it for
maintenance documentation that a non-native reader must understand the first
time. The rules are restated in Ze's words, and every example is written for Ze's own
surfaces. Where a question is not answered here, the
published standard is the authority.

ASD gives Issue 9 (2025-01-15) at no cost from `https://www.asd-ste100.org`, and
the direct file is
`https://www.asd-ste100.org/assets/files/ASD-STE100_ISSUE9.pdf`. It carries 53
writing rules in 9 sections, about 900 approved words, and about 1200
unapproved words with their alternatives.

The document is copyright (c) ASD 2025, and Ze holds no reproduction right. The
special usage rights cover ASD, AIA and ICCAIA members and their customers,
ministries of defense, airworthiness authorities, and universities. Ze is in
none of those categories, so the PDF, its text and its dictionary stay out of
the tracked repository. Keep the local copy in `tmp/asd-ste100/`, which
`.gitignore` excludes. Naming the standard is free, and copying its text is
not.
