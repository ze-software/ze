# Ze Go Style: Background

This page carries the history and the reasoning behind `ze-go-style.md`. It
holds no rule. Every rule is on the guide, and a reader who applies the guide
needs nothing here.

## Why have style

Another word for style is design. Ze runs on a router that nobody restarts. A
defect drops a session, blackholes a prefix, or leaks a buffer that grows for a
year. Style serves the three design goals: safety, performance and developer
experience. A rule earns its place in the guide only when it makes Ze safer,
faster, or easier to change. Readability is the floor, not the target.

## Why zero technical debt

Code, like steel, is cheaper to change while it is hot. A defect found in design
costs an hour. The same defect found in production costs a week, and it costs it
to somebody else. That is why the guide fixes what it finds when it finds it.

## Why every error is handled

The guide's rule on errors rests on one study of production failures in
distributed data-intensive systems:

> "Specifically, we found that almost all (92%) of the catastrophic system
> failures are the result of incorrect handling of non-fatal errors explicitly
> signaled in software."

## The quotations

> "The lack of back-of-the-envelope performance sketches is the root of all
> evil."

> "There are only two hard things in Computer Science: cache invalidation,
> naming things, and off-by-one errors."

> "The right tool for the job is often the tool you are already using -- adding
> new tools has a higher cost than many people appreciate"

## How the gates choose what they judge

`./le arch compound-guard check` judges only changed lines in shipped Go. It
skips `_test.go`, `vendor/`, `testdata/`, and a dot directory. A changed line
differs between the working tree and the last pushed commit behind HEAD. That
commit is the merge base of HEAD and the branch upstream. When the branch tracks
no upstream, it is the merge base of HEAD and `origin/main`.

The gate is owed before a push, so it judges the unpushed range. The next run
still judges a guard that was committed without a verify. A clean checkout of
pushed code judges nothing, because that code was judged before its push. The
base reads refs only, so the detached worktree that `./le verify worktree` makes
answers the same range. When no base resolves, the check exits 2 and names why.

The tree held about 3,000 such guards when the gate was written. A package or
file scope would make every old guard due when a line next to it changed. A guard
is due when a line of its condition changed, from the `if` keyword to the
opening brace. An edit inside an old guard's body leaves it alone.
`./le arch compound-guard selftest` proves the detection against fixtures.
<!-- source: internal/le/repo/changed/lines.go -- LinesSinceUpstream -->

## Why Ze writes its own lint rules

The toolchain does not know every idiom Ze holds itself to. The `modernize`
suite in `x/tools` carries no rule for the `sort` entry points, and no other
linter does. So 622 call sites stayed invisible until this repository wrote the
rule itself in `.golangci/ruleguard/modern.go`.

The flavors that drop build tags do not lint through the checkout
configuration. They lint through a copy written under `tmp/lint-flavors/`.
golangci-lint expands `${config-path}` against the directory of the
configuration it loaded, which for the copy is that directory. So the derivation
expands the token to the checkout root before it writes the copy.

If the token stays relative, the rules file is not found and gocritic fails to initialize.
`failOn: all` then stops the whole `goanalysis_metalinter` pass. Those flavors
lint with no gocritic at all, and the other findings of the run still print.
<!-- source: internal/le/go/lint/verifylint.go -- deriveTaglessConfig -->

The leading dot on the directory keeps the Go toolchain out of it, so a rules
file is never compiled, vendored, or linted. The dot is a trade, because neither
path gate reaches into the directory. `./le doc index check` accepts a
`<!-- source: -->` anchor into it and does not check it. `./le doc check links`
does not count a dotted path among its broken references. Both were measured by
breaking the path and watching the count stay the same. That is why the guide
tells you to correct it by hand when the file moves.

## Where Ze differs from TigerStyle

The differences are the places where Go, or Ze's own history, gives a different
answer. Each one is deliberate. The Ze column names where the guide states the
rule, and this page adds only why Ze differs.

| Subject | TigerStyle | Ze, and why |
|---------|-----------|-------------|
| Case | `snake_case` for everything | Go casing (`go-conventions.md`). `gofmt` and the standard library set it, and fighting them costs more than it returns |
| Function length | A hard limit of 70 lines | See the guide, "By the numbers" |
| Line length | A hard limit of 100 columns | See the guide, "By the numbers" |
| Indentation | 4 spaces | See the guide, "By the numbers" |
| Allocation | No dynamic allocation after startup | See the guide, "Performance". Go has a garbage collector, so the target is the hot path rather than the whole program |
| Repository tooling | Zig | See the guide, "Simplicity and debt" |
| Dependencies | Zero, apart from the toolchain | See the guide, "Simplicity and debt" |
| Assertion density | At least two for each function | See the guide, "By the numbers" |

## Lineage

The guide is Ze style. It started from TigerStyle, the coding standard of
TigerBeetle, and restates it for Go, for a routing daemon, and for this
repository, with Ze's own examples. Ze style also goes further than TigerStyle.
TigerStyle catches a bad state with an assertion when the program runs. Ze style
prefers a type that cannot hold the bad state, so the compiler refuses it before
the program runs ("Types that cannot lie" in the guide).

Source: `https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/TIGER_STYLE.md`

The quotations on this page come from that document. It credits Rivacindela
Hudsoni for the sketching line, Phil Karlton for the hard things in computer
science, and John Carmack for the tools. The failure statistic comes from
"Simple Testing Can Prevent Most Critical Failures", published at OSDI 2014.
