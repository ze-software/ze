# Third-party credits

Ze is built on other people's work. `ze thanks` lists every third-party work
compiled into the binary, and `ze thanks licence` prints their licence texts,
which the binary carries. This page describes how that record is declared,
how the copy the binary embeds is generated, and which checks keep the two
true.

## The declaration

`website/data/dependencies.json` is the one list a person edits. The website's
Dependencies page reads it, and so does the generator. It holds three lists:

| List | Holds | Each entry carries |
|------|-------|--------------------|
| `categories[].modules[]` | The curated direct modules, grouped as the page shows them. The category name is the group | `module`, `why`, `licence`, `copyright`, `source`, and where needed `licence-files` and `licence-note` |
| `linked` | Every other module a shipped build links. Their group is "Also in the shipped binary" | `module`, `licence`, `copyright`, `source` |
| `works` | Works that are not vendored Go modules: Go itself, the web assets, code copied or ported into Ze | `name`, `version`, `licence`, `copyright`, `source`, `why`, `group`, `files`, and `carried-by` on every work but Go itself, which carries `toolchain: true` instead |

The work marked `toolchain` is the Go toolchain: every binary is built by it,
so every binary lists it, with no package declaring it, at the version its
own build information states. It therefore declares an empty `version` and no
`carried-by`, and only one work carries the mark. The check refuses a
toolchain work with a version, one with a `carried-by` list, a package that
declares it present, a second toolchain work, and a declaration with none.

`licence` is an SPDX expression: one identifier, or identifiers joined by OR
alone or by AND alone. Under both operators the work's own files must state
every identifier: an OR records the choice the work's licence offers, so its
text states each alternative, and an AND names licences that all apply. An
expression that mixes OR and AND, or uses parentheses, `+` or WITH, is refused
rather than half read; no declared work needs one.

`copyright` names the holders, several joined by ", ", each spelled exactly as
the work's own file spells it. An entry whose files state no holder at all
says `copyright-unstated: true`. An entry whose holder is stated in a source
file rather than in a licence file names that file in `copyright-file`, as a
path from the repository root.

A module's licence and notice files are found: every file at the root of
`vendor/<module>/` whose name starts with LICENSE, LICENCE, COPYING, NOTICE,
PATENTS, AUTHORS or COPYRIGHT, in that order. A module whose licence is also
stated in another file lists that file under `licence-files`, as a path from
the repository root: `filippo.io/mlockexe` marks itself CC0 1.0 in its README
and offers 0BSD in its LICENSE, so it declares `CC0-1.0 OR 0BSD` with the
README there. Those files follow the root files: the licence and the holder
are checked against them, and the binary carries them. `licence-note` is a
sentence for the person reading the declaration, saying what the identifier
alone cannot, such as a GPL text that states neither "only" nor "or later";
no tool reads it.

A work lists its files in `files`, as paths from the repository root, in the
order they print, and, unless it is the toolchain work, the Ze packages that
compile it in under `carried-by`, as import paths. A work whose banner in a bundle's companion file does not
name it declares under `banner` a phrase that banner writes (see the check,
below).

The libraries bundled into swagger-ui keep their licence texts in
`third_party/swagger-ui-bundled/`, one `<package>.LICENSE` per library, read
byte for byte from unpkg at the version the work declares. They sit outside
`third_party/web/` because every file there is synced to the asset consumers
and served.

The tool reads unknown keys as an error, so a misspelled key is refused rather
than read as absent.

The file has one schema, `credits.Declaration`, and one reader,
`credits.ReadDeclaration`. The site build reads the declaration through them
too, for the Dependencies page and for `llms.txt`, so the page and the record
cannot read one key under two spellings, and a key neither knows stops the
site build as it stops the check.
<!-- source: internal/le/deps/credits/credits.go -- Declaration, ReadDeclaration, entries, moduleFiles, licencePrefixes -->
<!-- source: internal/le/site/dependencies.go -- readDependencyDeclaration -->

## What the Dependencies page shows

The page at `/reference/dependencies/` renders the declaration. Each curated
module is a row of its group with its version from `go.mod`, its licence, its
copyright holders and its why sentence. A section "Also in the shipped binary"
follows: the `linked` modules under one group, at the version `go.mod`
requires them, then the `works` under their own groups, each name linked to
its `source`. The Go toolchain work shows the release `go.mod`'s `go`
directive pins, and a copied work the declaration gives no version says so.
A `licence-note` follows its identifier in the licence cell. A
`copyright-unstated` entry says that no copyright holder is stated, and an
empty licence or holder says that it is not recorded, so no cell is ever
empty. The lead sentence and the published dependency figure count the direct
modules only. The Markdown mirror carries the same tables, and each module
line of the "Dependency rationale" section of `llms.txt` ends with its licence
identifier.
<!-- source: internal/le/site/dependencies.go -- renderDependencies, shippedGroups, shippedWorkVersion, licenceCellHTML, holderCell -->
<!-- source: internal/le/site/derived.go -- writeLLMSDependencies, llmsLicence -->

## The generated copy

`./le deps credits update` writes `internal/core/credits/data/`, which the
binary embeds. Nobody edits it by hand.

- `credits.json` is the record. It names its writer under `generated-by`, the
  SHA-256 of the declaration it was made from under `declaration-sha256`, the
  group names under `groups` in the order the declaration first names them
  (its categories, then the linked list, then the works), and one object per
  shipped work under `works`, sorted by name. A work object holds `name`,
  `kind` (`module` or `work`), `version` (works only, because a binary reads a
  module's version from its own build information), `toolchain` when set,
  `group`,
  `licence`, `copyright`, `copyright-unstated` when set, `why` when the
  declaration has one, `source`, and `files`.
- Each entry of `files` holds the file's own `name` (`LICENSE`), its `origin`
  in the repository (`vendor/github.com/miekg/dns/LICENSE`), and `text`, the
  stored copy relative to the data directory.
- The texts are stored under `texts/`, each named by the lowercase hex SHA-256
  of its bytes and copied byte for byte. Works that ship a byte-identical text
  share the one copy, and each work still lists its own files in order. The
  hex names are what `go:embed` takes: no name starts with a dot or an
  underscore, and no module path, with its slashes and capitals, becomes a
  path.

A module is written when a shipped flavor links it. A shipped flavor is a row
of the build matrix (`internal/le/repo/compiles/matrix.go`). The tool asks
`go list -e -mod=vendor -deps` for the modules each flavor's anchor package
links, under the flavor's tags and the feature tags. A flavor pinned to an
operating system is asked for that system only, and every other flavor for
both linux and darwin, because each links modules the other does not. A
package go list cannot load is refused by name, because it may hide imports
and leave the set short of a module, with one exception: the
`internal/core/credits` embed pattern `data` matching nothing is accepted,
because that is the tree before the first update writes the data, and the
pattern is resolved after the package's imports are read. Every `works`
entry is written, because the binary decides at run time which of them it
carries.

The record is encoded with a fixed key order, sorted works and a trailing
newline, so a second run over the same tree changes nothing. A file under the
data directory that no work references is removed.
<!-- source: internal/le/deps/credits/credits.go -- record, workRecord, fileRecord, textName, encodeRecord -->
<!-- source: internal/le/deps/credits/update.go -- plan, Update -->
<!-- source: internal/le/deps/credits/golist.go -- linkedByFlavor, linkedModules, dataNotYetWritten -->

## What a carrying package declares

A work that is not a module, other than Go itself, is credited only by a
build that carries it. The package that compiles it in calls `credits.DeclarePresent("<name>")` from its
own `init()`, with the work's name as a literal. The check reads those calls
with the Go parser, never by matching text. A declaration sits in the file
that embeds or holds the work, under that file's build tags, so a build
without the feature does not credit what it does not carry.

| Package | Declares |
|---------|----------|
| `internal/component/web`, `internal/component/lg` | htmx, uPlot: the asset directory both embed. Either alone carries them |
| `internal/component/api/rest` | swagger-ui, normalize.css and every library the bundle's companion file names |
| `internal/plugins/container/seccomp` (linux only) | Moby's default seccomp profile |
| `internal/thirdparty/fat` | the gokrazy FAT writer, from `credits.go`, Ze's own file beside the copy |
| `internal/exabgp/bridge` | ExaBGP, whose encoders the bridge ports |
| `internal/core/ipregistry`, `internal/core/portname` | the IANA protocol registries |

The declaration's `carried-by` names these packages; the table is a reading
aid, and `./le deps credits check` is the comparison.

Go itself is declared by no package: every binary lists it, so a binary that
links no carrying package, the credits package's own test binary included,
still answers a list and a document holding Go.

`DeclarePresent` panics with `BUG:` on a name the embedded record does not
hold as a work, and on the toolchain work, whose declaration would be a
second route nothing reads, so neither reaches an operator. Two packages may
declare the same work.
<!-- source: internal/le/deps/credits/presence.go -- presenceDeclarations -->
<!-- source: internal/core/credits/credits.go -- DeclarePresent, declarePresentIn -->

## What a binary lists

`internal/core/credits` embeds the data directory and decodes the record once,
refusing a key it does not know, a record that names a missing text, and a
record that marks no work toolchain, so the generator and the reader cannot
drift apart in silence and no binary answers an empty list. A binary lists:

- each module its build information (`debug.ReadBuildInfo`) links, at the
  version that information states, a replacement's when the module is
  replaced. The record's version is never used for a module;
- Go itself, in every binary, at the build information's Go version;
- each other work some package declared present, at its declared version;
- ordered by the record's `groups`, then by name.

A linked module with no record is still a row, with its version and the
licence `not recorded`, and the answer carries the names of every such module:
`ze thanks` prints the full list and exits 1 naming them, with the fix
`./le deps credits update`. A binary with no build information cannot tell
what it links, so it answers no list: the refusal names `./ze thanks`, whose
launcher builds `bin/ze` in module mode, which records it.

`TestEveryLinkedModuleHasACredit` (`internal/plugins/thanks`) is the
in-binary proof: its test binary links the product's composition root, and
every module its build information lists must be a recorded row with a
licence document.
<!-- source: internal/core/credits/credits.go -- carried, Works, UnrecordedError, ErrNoBuildInfo -->

## The licence document

`ze thanks licence` prints one document. For each distinct text, in the order
the list and each work's files name them, a heading block names every carried
work that ships it: a line `<name> <version>, <file name>`, then
`  Copyright: <holder>` (or `no copyright holder stated` when the work's files
state none) and `  Source: <location>`. A rule line follows, then the text
byte for byte. Works sharing a byte-identical text share one copy. When
modules have no record, a last block names them, and the command exits 1 as
the list does. `| name <work>` narrows the document to one work, all of its
files in order; a name no carried work has is refused, naming
`show thanks list`.
<!-- source: internal/core/credits/credits.go -- document, writeWorkHeader, licenceOf -->

## The check

`./le deps credits check` judges the whole tree. It runs on demand and in CI,
and never as a commit gate. Each refusal names the work, says what is wrong
and why, and ends with `./le deps credits update` alone on its last line,
because every repair is an edit of the declaration followed by a
regeneration. It refuses:

| Refusal | What it catches |
|---------|-----------------|
| A linked module with no entry | A module a shipped flavor links that the declaration does not name, with the flavors that link it |
| A `linked` entry no flavor links | An entry for a module the shipped binary does not carry |
| A module declared twice | A module in both the curated list and `linked`, or a name twice in one list |
| An entry with no licence, holder or source | A field the record needs left empty |
| A work with no licence file | A module with no licence file at its vendored root, or a work whose `files` is empty or names a missing file |
| Generated data that differs | A record or a text that is not what `update` would write, a text no work references, a record made from another declaration, and so a vendored licence file that changed |
| A holder not in the work's files | A `copyright` holder found verbatim in none of the work's licence, notice or `copyright-file` files. The refusal quotes the holder and names the files it read |
| A false `copyright-unstated` | The marker set on a work one of whose files states a holder line. The refusal quotes the line |
| A licence the text does not state | An identifier of the declared SPDX expression whose title or defining sentence appears in none of the work's files, `licence-files` included. The refusal names the identifier |
| An expression the check cannot read | A licence expression that mixes OR and AND, or uses parentheses, `+` or WITH |
| An identifier the table does not know | An SPDX identifier with no row in the table of defining sentences (`licences.go`), refused rather than passed |
| A work nobody carries | A `carried-by` package that does not declare the work present, a work other than the toolchain with no `carried-by`, a declaration naming no `works` entry, and a declaration in a package the entry does not list |
| A toolchain work missing or declaring too much | A declaration that marks no work toolchain, a toolchain work with a version or a `carried-by` list, a package that declares it present, and a second toolchain work |
| A bundled library with no work | A library banner in a bundle's companion licence file that no declared work claims |
| A banner phrase no banner writes | A work's declared `banner` phrase that no banner of its companion files writes, or a work with a phrase and no companion file. The refusal names the work, the phrase and the files |

A holder line is a line that carries a copyright notice's own marks:
`Copyright` or `copyright` followed by a year, the word followed by `(c)`,
`(C)` or `©` and then a year or a capitalised name, or `(c)`, `(C)` or `©`
followed by a year. The word followed by a capitalised word alone is prose,
such as the title "Copyright Rights in the Protocol Registries" that
`third_party/iana/README.md` quotes. A licence template's `Copyright [yyyy] [name of copyright owner]`, the phrase
"copyright notice", a list item `(c)` and the all-capitals disclaimer do not
qualify. When a file states such a line, the holder is stated, and the
marker cannot hide it.

The licence table maps each identifier to the titles and sentences that state
it. A match ignores case and the layout of white space.

A bundle's companion file is a work file whose name ends in `.js.LICENSE.txt`,
the name webpack gives the banners it extracts. Its banners have no common
field naming the library, so the check does not read a name. It reads each
`/*` comment as one banner, and a banner is claimed by a work listing the file
whose name appears in it as a whole word, ignoring case, or whose declared
`banner` phrase it contains verbatim. A whole word has no letter, digit,
hyphen or underscore on either side, so `buffer` does not claim the banner of
`safe-buffer`. A phrase is for a banner that does not name its package:
deep-extend's says "Recursive object extending". A phrase is checked too: one
that no banner of the work's companion files writes is refused, because it
would tie the work to nothing. The refusal of an unclaimed banner quotes its
first line and its line number.
<!-- source: internal/le/deps/credits/check.go -- Check, Report, Refusal -->
<!-- source: internal/le/deps/credits/licences.go -- licenceSentences, holderLine, banners, unclaimedBanners -->

## The commit gate

`./le commit create` runs the `deps credits` change-set check
(`internal/le/commit/changecheck`). It judges only what the commit adds, and
it runs no go command. It refuses a commit that:

- adds, in a Go file that can ship, an import of a vendored module that the
  generated data the commit leaves does not credit, unless the declaration
  the commit leaves names the module and that data was generated from that
  very declaration;
- carries a changed, added or removed licence or notice file of a credited
  work while the data the commit leaves records other bytes for it;
- carries the declaration or the data while the data was generated from
  another declaration.

A Go file can ship when it is not a test file and sits under `internal/`,
`pkg/`, or the directory of a shipped flavor's anchor under `cmd/`, outside
`internal/le/` and `internal/test/`. This is an approximation of what a build
links, chosen so the gate needs no build: a file inside those directories
that only a tooling tag compiles is judged as if it shipped. Its repair still
works, because once the module is declared and the data regenerated from that
declaration, the gate accepts whether or not the module links.

A commit that adds a module to `go.mod`, `go.sum` and `vendor/` only adds no
import and passes. A module already linked at HEAD without a credit refuses
no commit that leaves its imports alone: the check reports it.
<!-- source: internal/le/deps/credits/gate.go -- judgeCommit, shippable -->

## Design decisions

| Decision | It forbids | Source | Date |
|----------|------------|--------|------|
| Credit what ships: Go's own library, the third-party Go modules linked into a shipped `ze`, the web assets served to browsers, and code copied or ported from another project. Development tooling and lab software are thanked on the website page only | Crediting in the binary a work it does not carry, or leaving out a shipped work | 4096f645dc | 2026-11-20 |
| Each credit carries name, version, licence, copyright holder and the why sentence, and the full licence text of each shipped work is compiled into the binary | A notices file shipped beside the binary, or a credit with no text | 4096f645dc | 2026-11-20 |
| The curated list stays `website/data/dependencies.json`, and the binary carries a generated copy | A second hand-written list of the same works, or moving the list into the product | 4096f645dc | 2026-11-20 |
| The appliance image's own contents are a spec of their own | Crediting the image's BusyBox, boot code, firmware and gokrazy binaries in `ze thanks` | 4096f645dc | 2026-11-20 |
| The router's web interface gets no dedicated page; it reaches the daemon form of the command through the pages that run any command | A web route or page for the credits | 4096f645dc | 2026-11-20 |
| The commands are spelled `ze thanks` and `ze thanks licence` | Another spelling of the command or of licence | 4096f645dc | 2026-11-20 |
| The traffic-usage plugin is Ze's own code: it is not a third-party work | A credit for it, or any text describing it as derived from another program | 7581427e54 | 2026-11-20 |
| The root certificate data inside `golang.org/x/crypto/x509roots/fallback` is credited under that Go module only | A work of its own for the certificate data | 7581427e54 | 2026-11-20 |
| Where upstream does not state all of a licence, the credit states only what is known: `golangci-lint` is `GPL-3.0`, with no note | A `licence-note` or identifier that guesses what upstream does not say | 1b17c0d2f8 | 2026-11-20 |
| What the licence is matters more than who holds the copyright; the holder is still shown, on the Dependencies page and in `ze thanks` | Withholding a licence because its holder is unclear | 7581427e54 | 2026-11-20 |
| `NOTICE` carries no per-work attribution: it points at `ze thanks`, `ze thanks licence` and the Dependencies page, and the upstream NOTICE texts are printed by `ze thanks licence` | Copying a work's attribution or NOTICE text into `NOTICE` | 7581427e54 | 2026-11-20 |
