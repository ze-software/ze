# Learned: command-help-and-description

The owner asked that every command carry two texts, a short one and a long one,
under those names in the code. He also asked for a check that refuses a YANG
edit leaving one of them out. The names arrived the wrong way round, and he
reversed them the same day. The work became a consistency pass rather than an
inversion.

Writing the 3,000 explanations is what made the spec expensive. It is also what
made it valuable. Nobody writes an explanation without first reading the
producer, and 3,196 producer reads found ten defects.

## What the design turned on

**A gate whose population is a list drifts. A gate that walks the tree cannot.**
The first three attempts to count the command nodes owing a long text answered
21, 36 and 21, against a true 107. Each was a regex over YANG source. The tree
the operator meets is MERGED across modules, feature tags and plugin
registrations, so no scan of the files can see it.

The gate now walks the resolved goyang entry tree, and it enumerates from a
binary carrying all 36 feature tags. The number stopped being an argument.

**A shrink-only gate needs no stored baseline when the baseline is the text.**
Arming a gate over 825 breaches turns every session red on a tree it did not
touch. Keying a per-node record on the command PATH makes a rename read as new
debt. One answer avoids both. Read the summary TEXT from HEAD, and judge the
long half only where this tree changed it. Nothing is persisted, a rename
carries its own history, and the gate is green on day one.

**A test that builds its own fixture chooses the shape.** It chooses the shape
the author believes. Three defects here are one defect wearing three faces.

| The code | What it read | What arrives |
|----------|--------------|--------------|
| `parseFIBConfig` | `.(bool)` on a config value | a string, always |
| the same function | the section at the top level | wrapped in its full root path |
| `configvalue.Int` | `value > math.MaxInt64` | that bound rounds up to 2^63 in float64, so it rejects nothing |

Each had a green unit test over a hand-typed fixture. Each stayed invisible for
exactly as long as nobody compared the fixture with the producer. More
assertions would have found none of them. Two things would: build the fixture
from the producer, or route the shape question through one reader that every
caller uses. `internal/core/configvalue` is now that reader.

**Prose is a load-bearing test of the code it describes.** Ten defects were
found by an attempt to write a true sentence about a leaf. The CoPP
`over-limit-policy drop` discarded SSH and every other service the four terms do
not name. A keyless TACACS+ server sent passwords in cleartext under a header
claiming otherwise. The Flowtriq bearer token carried no `ze:sensitive`. The
L2TP pool delegated every subscriber a /56 whatever the operator wrote.

No test, lint or gate found any of them. Each was found because the honest
sentence about the leaf was not the sentence its name implied.

**A claim written from the code you changed is not evidence about the code that
runs it.** Three statements about the TACACS+ key were false, in three different
ways, and each survived a self-review.

| The claim | What is true |
|-----------|--------------|
| A keyless server refuses the config | It refuses a reload. At boot it warns and continues |
| A nil AAA bundle denies every login | It denies authorization. `noBGPAAAWiring` returns a nil authenticator on purpose, so ssh falls back to local users |
| `ze:sensitive` stores the value encoded | It masks the value everywhere and encodes it nowhere the commit path writes |

Every one needed a producer read. The author, having just written the guard,
felt no need to take it.

## What is left open

**The one-line row of a node several modules declare is still chosen by
alphabetical accident.** The long help now carries every module's text, joined.
The row can show one text, and nothing in the schema says which module OWNS a
shared node. So `interface` still reads with the class-of-service summary. Two
repairs exist: an extension that marks a declaration as a merge into another
module, or the convention changing to a real YANG `augment`. Both change how
every plugin attaches leaves, so both are the owner's call.

**The `?` box draws what fits, and no key scrolls it.** A 4096-byte declaration
already overflowed an 80x24 terminal before this spec joined several of them.
`plan/journal/bound-too-small-for-its-own-burst.md` carries the row and the
three possible shapes. Choosing between them is a product decision.

**185 config nodes and 7 RPCs still carry no long text.** Most are leaves no
producer reads, recorded in `plan/journal/unwired-feature.md`. An explanation
for a value nothing consumes would be the first false sentence in 3,000 true
ones.
