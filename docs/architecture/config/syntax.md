# Configuration Syntax

## TL;DR (Read This First)

| Concept | Description |
|---------|-------------|
| **Style** | JUNOS-like: `{}` blocks, `;` terminators, `#` comments |
| **Top-Level** | `environment`, `plugin`, `bgp`, `ospf` |
| **BGP Block** | `bgp { group <name> { peer <ip> { ... } } }` - wraps all BGP config |
| **OSPF Block** | `ospf { areas { area <id> { ... } } interfaces { interface <name> { area <id> } } }` - native OSPFv2 config root |
| **Inheritance** | 3 levels: bgp globals, group defaults, peer overrides (deep-merge for containers) |
| **Schema** | YANG-driven: parser dispatches by node type (leaf, container, list, leaf-list) |
| **Migration** | `ze config migrate` converts ExaBGP syntax to ze-native |

**When to read full doc:** Config keywords, parsing bugs, new config sections.

---

**Purpose:** Document complete ze configuration file syntax

---

## Overview

Ze configuration uses a JUNOS-like hierarchy of sections, keywords, and values. A value statement can use an explicit semicolon. The tokenizer automatically inserts a semicolon after a value-ending token at a newline, at end of file (EOF), or before `}`.
<!-- source: internal/component/config/tokenizer.go -- tokenizer.scan -->

A `[` opens a list, except straight after a `:` inside a word, where it
brackets an address and stays in the word through its `]`. That is how the
IPv6 route-target redirect is written, `extended-community
[redirect:[2001:db8::1]:100]`, the same form ze prints for a received one. A
`[` glued to any other character, such as `0x48[64,24]`, still opens a list.
<!-- source: internal/component/config/tokenizer.go -- tokenizer.readWord, tokenizer.wordAddressEnd -->

The parser is YANG-driven: each config node's type (leaf, leaf-list, container, list) determines how it is parsed. No custom `ze:syntax` annotations are used in ze-native config.
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- module ze-bgp-conf structure -->

### Retired keywords

A keyword ze once accepted and no longer does parses as an unknown field, and
the field name alone does not tell an operator what to write instead.
`retiredKeywords` maps each one to its replacement, and every unknown-field
refusal in the hierarchical parser, the set parser and the metadata parser
carries that replacement in the message.

The parser accepts ONE spelling, so this table is the message and never a second
grammar. Ze rewrites no file: the operator edits the keyword. Two keywords are
retired today, `process` (write `attach process`) and `certificate-fingerprint`
(write `ca <pki-ca-name>`, naming the certificate authority root exported from
the hub with `ze show pki local-ca pem`).
<!-- source: internal/component/config/retired.go -- retiredKeywords, RetiredKeywordHint -->

### Unknown keys

Every unknown-key refusal, from the hierarchical parser at the root, in a
block, in a list entry or at an editor context, and from the set parser, names
the closest key valid at that position: `unknown field in bgp: router-idd (did
you mean router-id?) (line 2)`. A retired keyword gets its replacement instead,
and a name close to no valid key gets no hint. The distance is the one the CLI
uses for a mistyped command, `suggest.Command`.
<!-- source: internal/component/config/retired.go -- UnknownKeywordHint -->
<!-- source: internal/core/suggest/suggest.go -- Command -->

A block that moved out of a container to become a top-level root gets the
command that rewrites it instead. `bgp { policy { ... } }` is the one such block
today: the `policy` component registers the move from `init()` with
`config.MustRegisterRelocatedContainer("bgp", "policy")`, the schema marks the
old position, and both parsers answer `policy is no longer inside bgp: it is now
the top-level policy { ... } block`, then `fix: ze config migrate -o <new-file>
<file>`. No alias accepts the old position.
<!-- source: internal/component/config/relocated.go -- MustRegisterRelocatedContainer, markRelocations, relocatedHint -->
<!-- source: internal/component/policy/register.go -- init, relocatedFromBGP -->

`ze config migrate` parses the file with `config.MigrationSchema()`, which
grafts each relocated root back under its old parent, and its last
transformation, `relocated-blocks`, moves the block to the top level and merges
it with any top-level block already there. A list entry written at both
positions is refused with both locations named, because no merge can choose
between them.
<!-- source: internal/component/config/relocated.go -- MigrationSchema, ApplyRelocations, refuseRelocationConflict -->
<!-- source: internal/component/config/migration/migrate.go -- relocated-blocks -->

An enumeration refusal includes its original path, the values it accepts, and
the leaf statement to write. A bracket list where the schema expects a container
names that path and renders the container's immediate fields instead, including
enum choices. Hierarchical, set and set-meta parsing derive these corrections
from schema nodes rather than a separate list of feature-specific hints.
<!-- source: internal/component/config/parser.go -- parseNode, parseLeaf, parseContainer -->
<!-- source: internal/component/config/parser_diagnostics.go -- prependSyntaxPath, containerValueFix, setSyntaxRefusal -->

---

## Basic Syntax Patterns

### Leaf (single value)

```
keyword value;
```

A value is a bare word or a quoted string. A bare word ends at whitespace, a
brace, a bracket, `;`, a quote or `#`, and a word that starts with `(` or `)`
is read as that parenthesis instead (`endsWord`, `scan`, `tokenizer.go`). A `[`
straight after a `:` stays in the word through its `]`, as in
`redirect:[2001:db8::1]:100`. A double-quoted string decodes `\"`, `\\`, `\n`
and `\t` (and `\'`); a single-quoted one decodes the same escapes. A quote
with no closing quote before the end of the file is a syntax error naming the
line it opens on: no string runs to the end of the file (`readString`,
`refuseUnterminated` in `parser.go`).

Every printer (`ze config fmt`, `show`, the set writers the editor stores)
writes a value bare only when the tokenizer reads that bare word back as the
same value, and quotes it otherwise (`QuoteIfNeeded`, `list_key.go`). The
decision asks the tokenizer itself (`bareWordReadsBack`), so the quoting rule
cannot drift from what ends a word. `x]y(z`, `a[b`, a value holding a newline or
a carriage return, and `(open` are therefore written quoted.
<!-- source: internal/component/config/tokenizer.go -- bareWordReadsBack, endsWord, readString -->
<!-- source: internal/component/config/list_key.go -- QuoteIfNeeded -->

### Leaf-List (single or multiple values)

A leaf-list accepts either a single value or a bracket list:

```
community 65001:100;                           # single value
community [ 65001:100 65001:200 65001:300 ];   # bracket list
as-path [ 65001 65002 65003 ];                 # bracket list
```

Both forms are equivalent for a single item: `community 65001:100;` and `community [ 65001:100 ];` produce the same result.

A container written as two blocks merges into one, and a leaf-list both
blocks set keeps the members of both, as repeated statements in one block do
(RFC 7950 Section 7.7). A set keeps one copy of each member. An ordered
leaf-list (`ze:ordered`) keeps every copy, in order. A member deactivated in
either block stays deactivated:

```
filter { import { filters [ a b ]; inactive: filters b; final accept; } }
filter { import { filters [ b c ]; final accept; } } # filters is [ a b c ], b inactive
```

An ordered leaf-list whose merged members repeat a deactivated member is
refused, because the deactivation marks a value and cannot say which copy it
means. The error names the leaf, the member and the fix:

```
seq: deactivated member "b" is listed 2 times once the repeated blocks are merged;
seq is an ordered list that keeps repeated members, so ze cannot tell which copy
is deactivated. Fix: write the seq list once, in one block, or remove the line
"inactive: seq b;"
```
<!-- source: internal/component/config/tree.go -- mergeFrom, checkMergedRepeatsLocked, keepsRepeats -->

### Container (block)

```
keyword {
    child1 value1;
    child2 value2;
}
```

A container with one leaf may print inline when that leaf has a one-line
representation. Repeated Flex values keep the container's block, because each
value needs its own statement. Value-or-array leaves retain all their members.
Comments or deactivation markers inside the container also keep the block.
<!-- source: internal/component/config/serialize.go -- CanInlineContainer, writeInlineLeaf -->

### Flex values

A Flex node accepts a value statement, a container or a named list entry.
Repeated value statements in a file keep their written order and duplicates.
Hierarchical parsing records each statement; plain set replay appends each
value line rather than replacing the previous line. Metadata-bearing lines
with the same author, source and session-start time retain that sequence too.
Different identities describe competing assignments of the whole leaf, not
simultaneous occurrences: the tree holds the last assignment and metadata keeps
each owner's value. A metadata printer writes that contested set once, so
reloading or adopting a session cannot multiply its records.

Every printer quotes and escapes each occurrence as one value when needed.
An empty value, a hash, a quote, an embedded newline or a multi-word value
therefore reads back unchanged; quoting never removes duplicates or reorders
the sequence.
The comparison view uses the same token codec, so an escaped newline remains
one displayed statement and validation highlights map to its working line.
<!-- source: internal/component/cli/diff_tree.go -- diffFlexValueText, renderDiffLines -->

An interactive editor `set` replaces the whole Flex value, removing every
previous occurrence while preserving the whole-leaf inactive state and
comments. It is an assignment, not file replay.

<!-- source: internal/component/config/parser_freeform.go -- parseFlexValue -->
<!-- source: internal/component/config/setparser_entries.go -- appendSetFlexValue -->
<!-- source: internal/component/config/setparser_meta.go -- walkAndSetWithMeta -->
<!-- source: internal/component/cli/editor_commands.go -- storeLeaf -->
<!-- source: internal/component/config/serialize.go -- serializeNode -->
<!-- source: internal/component/config/serialize_set.go -- serializeSetFlex, writeFlexLines -->
<!-- source: internal/component/config/list_key.go -- QuoteIfNeeded -->

### List (keyed entries)

```
keyword key1 {
    child1 value1;
}
keyword key2 {
    child1 value2;
}
```

A list entry can also be written on one line, with the key first and the child
values after it. The values are assigned positionally in YANG definition order,
and the LAST child absorbs every remaining token:

```
nlri {
    ipv4/unicast add 10.0.0.0/24
}
```

What the last child RECEIVES depends on its node kind, and the schema decides it
rather than the brackets an author happened to write.

| Last child | `keyword key [ a b ];` stores |
|------------|-------------------------------|
| A leaf-list (`ze:syntax "bracket"`, `ze:syntax "value-or-array"`, or a plain YANG leaf-list) | `a` and `b` as two MEMBERS, through the same store the block form uses |
| Any other leaf (a leaf, a multi-word leaf, a flex value, a presence container's value) | the single string `"[ a b ]"`, joined with any other tokens on the line |
| A container, a list, an inline list or a freeform block | nothing: the line is refused, naming the child and the block form to write instead |

So `nth 2 [ 3491 ]` and `nth 2 { asn [ 3491 ]; }` produce one tree, and the NLRI
entry above is untouched, because its last child is a plain leaf. A leaf-list
before the last child takes its one value as a member the same way, and a
value that falls on a container or a list (`peer A "x"` gives `x` to the
peer's first child, the `attach` container) is refused, because no printer
could write it back (`refuseInlineSlot`).

The serializer writes the one-line form back for a list whose schema has exactly
ONE child and that child is a leaf-list. Every other list keeps the block form,
because the one-line form drops the child NAME and a reader could not tell which
leaf the members landed in. A list with an entry whose key is one of its child
names is written one block per entry, never as `name { key value }`, which
reads back as an anonymous entry setting that child (`entriesArePositional`).
<!-- source: internal/component/config/parser_list.go -- parseListInlineEntry, setInlineLastChild, isInlineLeafListChild, refuseInlineSlot, inlineSlotTakesValue -->
<!-- source: internal/component/config/serialize.go -- entriesArePositional -->
<!-- source: internal/component/config/serialize.go -- inlineLeafListEntry, writeInlineLeafListMembers -->

### What a reader receives

`(*Tree).ToMap` lowers the parsed tree to `map[string]any` for delivery, and the
shape it emits for a leaf-list or a list is not the one the node type suggests.
An in-process component receives that map unchanged. A plugin receives it
marshaled to JSON, which turns a `[]string` into a `[]any`.

| Node | Members | In process | After JSON |
|------|---------|------------|------------|
| leaf | set | `string`, whatever the YANG type | `string`, whatever the YANG type |
| leaf-list | none active | key absent | key absent |
| leaf-list | exactly one | bare `string` | bare `string` |
| leaf-list | two or more | `[]string` | `[]any` |
| list | any count, one included | `map[string]any` keyed by the list key | `map[string]any` keyed by the list key |

Three consequences decide how a reader is written. A plain leaf is a STRING at
both ends, whatever its YANG type says. `Tree.values` is a `map[string]string`
and the lowering copies it through, so a `boolean` leaf arrives as `"true"` and
a `uint16` leaf as `"30"`. A leaf-list is not always a slice, so asserting one
drops the value at exactly one member. A list is never a slice at any count.
Its key leaf is the map key rather than a field inside the entry, so asserting a
slice drops it at every count.

A list whose YANG key names several leaves (`key "peer vrf interface"`, the
BFD `single-hop-session` list) is keyed by the entry's canonical ADDRESS, not
by its first word: the first key leaf's value, then `name value` for each other
key leaf in YANG key order whose value is not its YANG `default`. A key leaf
with no default is omitted only when absent. So the entries
`single-hop-session fe80::b { interface zbl0; }` and
`single-hop-session fe80::b { interface zbl1; }` are stored, and delivered to
the plugin, under `fe80::b interface zbl0` and `fe80::b interface zbl1`, and
`vrf default` written or omitted is the same entry. The other key leaves stay
leaves of the entry as written. A key value of such a list that is empty or
holds whitespace is refused at parse, because it would make the address
ambiguous, and two entries with one address are refused as a duplicate list
key. Brace form writes the first value alone on the opening line, because the
body carries the other key leaves. Set form writes the whole address on every
line of the entry (`set bfd single-hop-session fe80::b interface zbl1 profile
fast`), each token quoted on its own, plus one line holding the address alone,
and never a line that sets a key leaf the address carries. The set parser
(with and without meta) reads an address left to right: the first token is the
first key value, then a `name value` pair is consumed while the next token
names another key leaf of the list and a token follows it; a key leaf named
twice is refused. The CLI editor reads every path by the same rule, so
`set bfd single-hop-session fe80::b interface zbl1` creates the entry on
`zbl1` (with its `interface` leaf written), a leaf after the address lands on
that entry only, `delete` of a full address removes one entry, and a set-form
`show` of `fe80::b` matches the address as one segment and prints none of the
link entries' lines. A rename or copy changes only the first key value: the
other key leaves carry over, a destination that changes one is refused with
the delete and set lines that move the entry, and the session change file
writes each key as one quoted token. A rename or copy of an entry of a keyless
list, or of an entry under one, is refused. A list whose key names one leaf
keeps its key exactly as written.

An empty single-key entry is still configuration. For example,
`interface { dummy zetrk1 {} }` saves as `set interface dummy zetrk1`,
and parsing that line recreates the entry. Plain and metadata-bearing set
files preserve it without requiring a description or another placeholder
leaf. A nonempty single-key entry needs no extra address-only line: its
children already recreate it. An inactive empty entry uses `nop` instead
of `set`, preserving its inactive state. Annotated set displays use the same
entry lines with their metadata gutter.
<!-- source: internal/component/config/serialize_set.go -- serializeSetList, serializeSetMetaList -->
<!-- source: internal/component/config/serialize_annotated.go -- serializeAnnotatedSetList -->

The web editor addresses the entry by its address as path segments
(`/show/bfd/single-hop-session/fe80::b/interface/zbl1/`); its list table heads
the key column with the first key leaf and links each row to its own segments,
and its add form posts the other key leaves as fields that complete the
address. Neither the add form nor the rename form changes the case of a key.
gNMI names the keys (`single-hop-session[peer=fe80::b][interface=zbl1]`): a
path must name the first key leaf, `[vrf=default]` names the same entry as no
`vrf`, a response path names every key leaf from the schema (an absent one at
its default), and a Set of a key leaf is refused as naming it twice.
<!-- source: internal/component/config/list_key.go -- BuildListKey, ParseListAddress, canonicalParsedListKey, ListKeyOpeningValue, setFormEntry, emitAddressLine, ListPathStep, ListAddressSpan, WalkListEntryPath, ResolveListEntryTarget, ListEntryDestination, splitQuotedTokens -->
<!-- source: internal/component/config/serialize_set.go -- FilterSetByPath, setPathSegments -->
<!-- source: internal/component/cli/model_commands_edit.go -- cmdSet, listEntryMove -->
<!-- source: internal/component/web/handler_config_walk.go -- walkSchema, walkTree, isListEntryPath -->
<!-- source: internal/component/web/fragment.go -- buildListTable, buildContextHeading -->
<!-- source: internal/component/gnmi/path.go -- pathToSegments, segmentsToPath -->
<!-- source: internal/component/config/parser_list.go -- addParsedListEntry -->

Read all three through `internal/core/configvalue` (`Bool`, `Int`, `LeafList`,
`ListEntries`) rather than asserting a type. The obligation is stated in the
configuration rule.

A `.(bool)` or `.(float64)` assertion on a delivered leaf never succeeds, and it
fails the way an absent value fails. The reader keeps its default and nothing is
logged, so the operator's setting is discarded in silence. Three shipped call
sites read that way, in `fib/kernel` and the L2TP pool. One of them delegated
every subscriber a /56 whatever `delegation-length` said. A hand-typed test
fixture hides it, because a fixture carrying `"delegation-length":48` as a JSON
number is a payload the producer never emits.

The lowering is ONE WAY. A bare string is a plain leaf and a one-member
leaf-list alike, so no lowered map can be rebuilt into a Tree without the schema
that declared the node. A consumer that needs a Tree takes the PARSED one
(`preparePKIConfig`, `cmd/ze/hub/main_pki.go`), never a rebuild of the map.

A list is delivered as a map, and a map states no order, so the operator's ENTRY
ORDER needs a second key to survive lowering. There are two lowerings.

| Lowering | Reads | Carries the order |
|----------|-------|-------------------|
| `(*Tree).ToMap` | gNMI, `ze config show`, config validation, the CLI editor | no |
| `(*Tree).ToPluginMap` | the map delivered to a plugin, at startup and at reload | yes |

`ToPluginMap` emits the order of every list of two or more entries as a sibling
key: the list name with an `@` prefix, holding the entry keys in the operator's
order. A YANG node name never starts with `@`, so the sidecar can never collide
with a declared sibling. A list of one entry carries no sidecar.

`ToMap` carries no sidecar at any count, because its consumers are schema-bound:
gNMI would publish a path no YANG module declares, `ze config show | json` would
show the key to the operator, and whole-config validation would refuse it as an
unknown node.

A `list` declared `ordered-by user` MUST be read with `configorder.Entries`
(`internal/core/configorder`), which returns the entries in the delivered order
and refuses a list of two or more entries that arrives with none. Read an
unordered list with `configvalue.ListEntries`. The obligation is stated in the
configuration rule.

Every printer writes every list's entries in the order the tree recorded,
including plain, positional multi-block, Flex and InlineList entries. This
applies to hierarchical output (`ze config fmt`, `ze config migrate format
hierarchical`, `show`), set and set-with-metadata files, annotated views and
blame. Entries absent from the record print afterward, sorted by key; removed
or repeated keys in the record do not print twice or lose another entry.
`ListEntryKeys` is the shared ordering producer.
<!-- source: internal/component/config/serialize.go -- ListEntryKeys -->

This display rule does not change how consumers read a list. The schema still
carries `ordered-by user` as `ListNode.UserOrdered`, and the readers above keep
their contracts.

Order in a display is distinct from order in a comparison. Reordering a plain
list is no configuration change, including when the list is nested or uses
Flex or InlineList syntax. `PruneUnchanged` removes that order-only difference
without leaving empty ancestor blocks. Reordering a list declared `ordered-by
user` is a change: pruning retains the list as order context, and `show |
compare` marks moved entries. Comments alone are not changes; inactive state
and intentionally present empty containers still are.
Scalar comparisons use the printer's canonical spelling, so replaying the same
value from hierarchical, set or metadata-bearing set form does not invent a
change. Ephemeral nodes remain omitted; hidden persisted nodes still count.
<!-- source: internal/component/config/prune.go -- PruneUnchanged, SubtreesEqual -->

The visual diff uses the same entry writers as the hierarchical printer,
including grouped positional rows and inline leaf-list entries.
<!-- source: internal/component/config/serialize.go -- ListUsesMultiBlock, WriteListEntry, WritePositionalListEntry, WriteFlexListEntry, WriteInlineListEntry -->

`ze config diff` keeps its resolved BGP value comparison and also compares
schema-declared user-ordered lists from the parsed configuration. It reports
their entry-key sequence under the existing `@<list-name>` metadata convention,
at the full configuration path (for example,
`firewall/table/wan/chain/input/@term`). Both text and JSON carry the old and
new sequences. A list's first appearance or disappearance is an addition or
removal; a membership or order change in an existing list is a changed sequence.
Firewall-only inputs need no BGP block. This order comparison does not add
general value comparison for other non-BGP configuration.

<!-- source: internal/component/config/cli/cmd_diff.go -- resolveDiff, diffListOrders -->

<!-- source: internal/component/config/tree.go -- (*Tree).ToMap, (*Tree).ToPluginMap -->
<!-- source: internal/core/configvalue/configvalue.go -- Bool, Int, LeafList, ListEntries -->
<!-- source: internal/core/configorder/configorder.go -- Entries, OrderKey -->

### Inactive prefix (deactivate / activate)

Any structural statement may be prefixed with `inactive: ` to mark it
inactive. The node stays in the file and round-trips through save/load,
but is removed by `PruneInactive` before consumers see the tree, so the
runtime treats it as absent.

```
bgp {
    inactive: router-id 10.0.0.1;       # leaf -- value preserved verbatim
    inactive: peer scratch { ... }      # list entry -- whole subtree skipped
    inactive: filter { ... }            # container -- whole subtree skipped
    filter {
        import {
            filters [ no-self-as reject-bogons ];
            inactive: filters no-self-as;    # single leaf-list member
            final accept;
        }
    }
}
```

A single deactivated **leaf-list member** keeps its bare value in the
leaf line and is declared inactive by a follow-up `inactive: <leaf>
<member>` statement (above: `no-self-as` stays listed, marked
inactive). Both kinds of leaf-list write it this way: a plain one
(`name-server 8.8.8.8 1.1.1.1;`) and a bracket one (`address [ ... ];`).
`inactive: <leaf> <member>;` with one bare value is the hierarchical
spelling of the set format's `nop <leaf> <member>` (below), on both kinds:
it deactivates the member, and a member the list does not hold yet is
added, deactivated. `inactive: <leaf> [ ... ];` is the whole leaf, so a
whole inactive leaf-list is written in brackets even when it holds one
member.
The internal
`inactive:<member>` token never appears in serialized output -- it
would fail per-item type validation (e.g. `ip-address`) on reparse.
<!-- source: internal/component/config/parser.go -- parseNodeInactive, parseInactiveValueOrArray, parseInactiveBracketMember -->
<!-- source: internal/component/config/serialize.go -- serializeNode -->

The CLI verbs `ze config deactivate <file> <path>` and `ze config
activate <file> <path>` add and remove the prefix. The TUI accepts the
same `deactivate <path>` / `activate <path>` commands while editing.
The mechanism is engine-level: every YANG node is deactivatable, no
schema annotation is required.

In the **set / single-line format**, `nop` is a `set` line marked
inactive, the Junos way: the keyword replaces `set` on a deactivated
line, where the hierarchical format writes the `inactive:` prefix. So
`nop <leaf-list> <member>` adds the member deactivated, as the `set`
line would add it active. "nop" means "no operation": the config entry
exists in the file but produces no operational effect. Toggling
activation is a 3-byte in-place edit (`set` <-> `nop`):

`set` and `delete` in this format are the canonical CLI verbs in their engine
path form, and the parser takes their spelling from `command.Verbs` rather than
writing the words again, so renaming a verb renames the file keyword with it.
`nop` and `inactive` belong to this format alone and the verb registry holds
neither.
<!-- source: internal/component/config/setparser.go -- cmdNop, cmdSet, cmdDelete, parseNop, nopBracketMember, setBracketLine, bracketMembers, isMemberLeafList -->
<!-- source: internal/component/config/serialize_set.go -- writeInactiveMemberLines -->
<!-- source: internal/component/command/verbs.go -- VerbSet, VerbDelete -->

```
nop bgp router-id 10.0.0.1
```

For leaf-lists with per-member deactivation, a deactivated member is
written `nop <path> <member>`, for both kinds of leaf-list. A plain
leaf-list gives each member its own line: active members use `set`,
deactivated members use `nop`:

```
set system name-server 8.8.8.8
nop system name-server 1.1.1.1
```

A bracket leaf-list sets the whole list on one line, every member listed,
and follows it with one `nop <path> <member>` line per deactivated member.
A `nop` line that names one value is always one member: written before the
list line, or naming a value the list does not hold yet, it adds the member
deactivated, and a later list line that names the member keeps it
deactivated. A later list line still replaces the members, so it drops a
member it does not name. Only a `nop` line that names a list in brackets
(`nop <path> [ ... ]`) deactivates the whole leaf:

```
set interface ethernet eth0 unit 0 ipv4 address [ 192.0.2.1/24 192.0.2.3/30 ]
nop interface ethernet eth0 unit 0 ipv4 address 192.0.2.3/30
```

`delete <path> <member>` removes that one member, of a plain or a bracket
leaf-list alike; `delete <path>` removes the whole leaf.

Container deactivation uses a structural `nop <container-path>` line
before its children. Children retain their own `set`/`nop` state:

```
nop bgp
set bgp router-id 10.0.0.1
set bgp neighbor 192.0.2.1 peer-as 65001
```

**Backward compatibility:** the `inactive <path>` keyword from older
configs is still accepted by the parser, and so is `inactive <path>
<member>` after a bracket leaf-list's list line, the spelling an older
release wrote for a deactivated bracket member. On save, `inactive` lines
are migrated to the `nop` form automatically. An older draft could also
spell a bracket list as one quoted value (`address "192.0.2.1/24
192.0.2.3/30"`): when that value fails as one member but each of its words
is a valid member, the parser reads the words as the members.

Coexists with per-feature `disable` semantics (e.g. a peer's
`admin-state disable` leaf), which are protocol-aware and operationally
distinct from "no such node". Use `inactive:` when you want the node
absent at apply time; use the per-feature disable when you want the
protocol to know about the node but treat it as administratively down.

---

### Comments

A comment runs from `#` to the end of the line, anywhere whitespace may stand,
and is the only comment form. It changes no value. The parser keeps each one on
the statement it was written beside, and the hierarchical printer
(`config.Serialize`, so `ze config fmt` and `ze config migrate format
hierarchical`) writes it back beside that statement, wherever the canonical
order moves the statement:

| Written | Belongs to | Printed |
|---------|------------|---------|
| On its own line above a statement | That statement (leaf, container, list entry, `inactive:` line) | On its own line directly above it, at its indent |
| After a statement on its line, after `{`, after `}` | That statement, its opening line, its closing brace | After the same line, one space before `#` |
| On its own lines after a block's last statement | The block | Inside the block, last before `}`; at the end of the file for the root |
| A group before a block's first statement, a blank line after it | The block's head | First in the block, then one blank line |
| After `[` in a bracket leaf-list | The leaf | Above the leaf |
| Inside a freeform, flex or inline-list body, or a container written without braces | The enclosing statement | Above that statement |

A statement written twice (a leaf set twice, two blocks of one container)
keeps the comments of both. The last same-line comment stays on the line, and
the earlier ones print above the statement.

A container whose inside carries a comment, or whose entries carry one when it
is `ze:flatten`, prints as a block rather than on one line, because a one-line
spelling has no line for the comment. The container's own comment above it
keeps the one-line spelling.

Blank lines are presentation: fmt keeps only the one after a head group.
A comment never makes `PruneUnchanged` keep an unchanged node. The set and
set-meta printers do not print comments yet, so a commit from an editing
session, which writes the set-meta form, drops them; an editor with no session
writes the hierarchical form and keeps them, its stamp replaced rather than
stacked (`SerializeStamped`).

The editor's displays print the operator's comments where the tree carries
them: `show` prints through `config.Serialize`, or `SerializeSubtree` below
the root (the set display prints set lines, which carry none). The
editor's change view ignores comments when it compares configuration, so a
comment never marks a line changed. It prints no comment line; its validation
highlight still lands on the statement's line in the working text, below any
comment above it.
<!-- source: internal/component/config/parser_comments.go -- commentScope -->
<!-- source: internal/component/config/serialize_comments.go -- writeBlock, writeCommentsAbove -->
<!-- source: internal/component/config/serialize.go -- CanInlineContainer -->
<!-- source: internal/component/config/flatten.go -- canFlattenContainer -->
<!-- source: internal/component/config/prune.go -- PruneUnchanged -->
<!-- source: internal/component/cli/editor.go -- Editor.stampedWorkingContent -->
<!-- source: internal/component/cli/model_commands_show.go -- Model.renderTreeAtPath -->
<!-- source: internal/component/cli/diff_tree.go -- computeTreeAnnotatedDiff, renderDiffLines -->

### Schema Stamp

Committed config files carry a schema stamp as the first line, naming the Ze
release that wrote them (`YY.MM.DD`):

```
# ze-schema: 26.10.07
```

The stamp is a comment (ignored by all parsers). On the first line it is always
the file's head, so `ze config fmt` keeps it on line 1. `FormatSchemaStamp` builds it
from `version.Release()` and every persistence site re-emits it: the editor
commit, schema evolution at startup, and downgrade recovery. It is not stored
in the YANG tree. A writer of the hierarchical form goes through
`SerializeStamped`, which replaces the stamp the parsed tree kept as its head
rather than adding a second one, so a file names one release, the one that
wrote it. `ScanStampRelease` reads it back from the first line, and a
file without a stamp yields an empty release. `version.CompareReleases` sorts
an empty or unparseable release (such as a dev build's `dev`) as older than
every dated one, so an unstamped file is never "newer" than the binary.

<!-- source: internal/component/config/stamp.go -- FormatSchemaStamp, SerializeStamped, ScanStampRelease -->
<!-- source: internal/core/version/version.go -- CompareReleases, IsNewerRelease -->

#### Schema Evolution

At start, after the config loads, the hub runs every registered
`migration.Evolution` whose `Release` is newer than the file's stamp
(`applyEvolutions`). Each one's `Detect` decides whether the tree needs it, and
`Apply` changes the tree; when any applied, the result is re-stamped with this
binary's release and saved back. An unstamped or `dev`-stamped file runs every
evolution, except one marked `StampedOnly`.

`StampedOnly` exists for an evolution that writes an old default into a file.
A shape change is safe on any file, because `Detect` sees the old shape. An
unset leaf looks the same under both defaults, so only a dated stamp says which
one the file was written under, and a file without one keeps the current
default. `ipsec-unmatched-keeps-bypass` is one: it writes `vpn ipsec unmatched
bypass` into a file saved while bypass was the default
([IPsec guide](../../guide/ipsec.md)).

The stamp is the one on the content the config loader parsed
(`LoadConfigResult.StampRelease`). After a downgrade recovery (below) that is
the rollback's own stamp, not the one recovery published over it, so a
recovered rollback still gets the evolutions it is owed.

An evolution that changes how the router behaves carries a `Notice`: what it
wrote, why, and the command that undoes it. `applyEvolutions` logs each one at
Warn, the default level, naming the config file (or stdin) and whether the
change was saved back. An evolution that only changes a shape leaves `Notice`
empty and is logged at Info with the others, so the Warn is kept for the
changes an operator must hear about.

<!-- source: internal/component/config/migration/evolution.go -- Evolution, Evolve, applicableEvolutions -->
<!-- source: cmd/ze/hub/main_evolve.go -- applyEvolutions, warnEvolutionNotices -->
<!-- source: internal/component/config/loader.go -- LoadConfigResult.StampRelease -->
<!-- source: internal/component/ike/ipsec/unmatched_evolution.go -- unmatchedBypassEvolution -->

#### Downgrade Recovery

When ze starts and `LoadConfig` fails on the active config, the hub calls
`RecoverConfig`, unless the failure is a `ze:validate` refusal
(`ErrCustomValidation`): a value the rules refuse is reported, never answered
by starting on an older config. Recovery needs the persistent store, so a
config read from stdin is not recovered.

`RecoverConfig` acts only when the stamp on the failing config names a newer
release than the binary's own, meaning the binary was downgraded. It then walks
the config's history in the store (`ListVersions`, newest first), skips each
version whose stamp is also newer than the binary, and attempts a full parse on
the rest. The first version that parses becomes the running config.

| Step | What happens |
|------|-------------|
| 1 | `LoadConfig` fails, the failure is not `ErrCustomValidation`, and a store is open |
| 2 | The stamp on the failing config is newer than `version.Release()`; otherwise ze refuses to start with the load error |
| 3 | Walk history newest first, skip versions stamped newer than the binary |
| 4 | Full parse on each candidate (the stamp is a hint, the parse is the gate) |
| 5 | First success: the failing config is written into history (`WriteVersion`), so it is kept, not lost |
| 6 | The recovered tree, re-stamped with this binary's release, is published through the store as a candidate and promoted to active; with an explicit config file the commit also writes that file |
| 7 | If no version parses, or publication fails, ze refuses to start with a clear error |

Publication refuses when the active config changed while recovery ran
("configuration changed externally during schema recovery").

Recovery runs at startup only, not on SIGHUP reload. A reload failure during
runtime surfaces as an error rather than silently reverting to old config.

<!-- source: internal/component/config/stamp.go -- RecoverConfig -->
<!-- source: cmd/ze/hub/main.go -- recoverableLoadError, startup LoadConfig failure branch -->
<!-- source: cmd/ze/hub/config_source.go -- publishRecoveredConfig, promoteConfigCandidate -->

---

## Top-Level Structure

```
# Comment
plugin {
    external <name> {
        ...
    }
}

bgp {
    router-id <ip>;                         # BGP-level global (inherited by all peers)
    session { asn { local <asn>; } }        # BGP-level local AS (inherited by all peers)

    group <name> {
        <peer-fields>                       # Group-level defaults (shared by all peers in group)
        peer <name> {
            <peer-fields>                   # Peer-level overrides
        }
    }

    peer <name> {                           # Standalone peer (no group inheritance)
        <peer-fields>
    }
}
```
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- container bgp, list peer -->

---

## Section Types

### environment

Ze-specific block for setting environment configuration from the config file.
See [environment-block.md](environment-block.md) for full documentation.

```
environment {
    log { level DEBUG; }
    tcp { attempts 3; }
}
```
<!-- source: internal/component/config/environment.go -- ListenEndpoint, ParseCompoundListen, parseOneEndpoint -->

### plugin

Container for plugin definitions. Supports `external` for subprocess plugins.
Future: `builtin` and `wasm` for other plugin types.

```
plugin {
    external <name> {
        run <path>;
        encoder json;           # or text (v4 only)
        timeout 10s;            # stage timeout (default: 5s)
    }
}
```

| Keyword | Type | Description |
|---------|------|-------------|
| run | string | Command to execute |
| encoder | string | `json` or `text` |
| timeout | duration | Startup stall timeout (e.g., `5s`, `1m`, `500ms`): how long a startup stage may go with no plugin completing one, not a budget for the stage itself. Default: 5s. 0 = use default. Negative rejected. |
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- list process, leaf run, leaf encoding -->

**Timeout semantics:** During startup, all plugins synchronize at each stage. The timeout controls how long this plugin waits for all plugins to complete each stage. With multiple plugins, use the same timeout for all, or set the longest timeout on all plugins to avoid fast plugins timing out while waiting for slow ones.

### ospf

Top-level native OSPFv2 configuration. The `ospf` container is a YANG config
root and auto-loads the OSPF edge plugin. Interfaces bind to areas explicitly by
setting `interfaces/interface/area`; Ze does not use FRR-style `network <prefix>
area <id>` matching. `network-type` accepts `broadcast`, `point-to-point`, and
`loopback`; passive and loopback records do not open raw sockets. The NSM
consumes `mtu-ignore`, `retransmit-interval`, `transmit-delay`, and dead timers
from the same interface list.

```
ospf {
    router-id 10.0.0.1
    areas {
        area 0.0.0.0 { area-type normal; }
    }
    interfaces {
        interface eth0 {
            area 0.0.0.0
            network-type point-to-point
            mtu-ignore true
        }
        interface lo0 {
            area 0.0.0.0
            network-type loopback
        }
    }
}
```

`router-id` uses `ze:validate "ospf-router-id"`. `area-id` and interface `area`
use `ze:validate "ospf-area-id"` and are checked again by the in-process config
verifier so an interface cannot reference an undeclared area.
<!-- source: internal/plugins/ospf/yang/ze-ospf-conf.yang -- ospf container, area and interface lists -->
<!-- source: internal/plugins/ospf/register.go -- InProcessConfigVerifier -->

### filter import (Loc-RIB attachments)

`isis`, `ospf`, `static`, `connected` and `rib` each carry the same
`filter { import { ... } }` block, declared once as the `rib-import-filter`
grouping in `ze-policy-conf`. `filters` is an ordered list of names of filter
instances defined under the top-level `policy {}` root. `final` is mandatory.

```
policy {
    prefix-list ISIS-IN {
        entry 10.0.0.0/24 { action accept; }
    }
}
isis {
    filter {
        import {
            filters [ ISIS-IN ];
            final accept;
        }
    }
}
```

| Keyword | Type | Description |
|---------|------|-------------|
| filters | ordered leaf-list of string | Filter instance names, run in order. A filter that rejects stops the chain. A filter that does not reject passes the route on |
| final | `accept` or `reject` | Decides a route that passed every filter. Required, even with no filters |
<!-- source: internal/component/policy/yang/ze-policy-conf.yang -- grouping rib-import-filter -->
<!-- source: internal/component/policy/rib_compile.go -- ribChain.Evaluate -->

The protocol's chain runs first, then the `rib` chain. A rejected route stays
in the Loc-RIB as a hidden path that is never selected. A commit that changes
a chain judges every stored path again. Load refuses these chains
([Route Policy at the Loc-RIB](../../guide/route-policy.md)):

- a chain with no `final`
- a `<plugin>:<filter>` reference
- a name that is not a `policy` instance
- a filter that reads a field the routes do not carry
- a filter type with no Loc-RIB route evaluator
<!-- source: internal/core/rib/policy/policy.go -- Snapshot.Evaluate -->
<!-- source: internal/component/policy/rib_compile.go -- readRIBChain, compileRIBChain, installRIBImports -->

### Peer Groups

Peers are organized into named groups under `bgp`. Groups provide shared defaults inherited by all member peers.

#### Group Structure

```
bgp {
    group <name> {
        <peer-fields>               # Group-level defaults
        peer <name> {
            <peer-fields>           # Peer-level overrides
        }
    }
}
```

| Element | Type | Purpose |
|---------|------|---------|
| `group` | list (key: name) | Named collection of peers with shared defaults |
| `peer` inside group | list (key: name) | Peer with optional overrides of group defaults |
| `peer` at bgp level | list (key: name) | Standalone peer (no group inheritance) |
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- list group, list peer -->

#### 3-Level Inheritance

| Priority | Source | Scope |
|----------|--------|-------|
| Lowest | `bgp` block globals | `session { asn { local; } }`, `router-id` -- inherited by all peers |
| Middle | Group defaults | All `peer-fields` set on the group |
| Highest | Peer overrides | All `peer-fields` set on the peer |
<!-- source: internal/component/bgp/config/resolve.go -- ResolveBGPTree, deepMergeMaps -->

Containers (like `capability`, `timer`) deep-merge at key level -- both group and peer capabilities are combined. Leaves (like `hold-time` inside `timer`) override -- peer value wins over group value.

#### Example

Every peer below gives `remote` and `local` DIFFERENT addresses, and that is a
property of BGP rather than a style choice: the two ends of one session are two
speakers, so they hold two addresses. A configuration naming one address for both
describes a session that cannot exist, and Ze refuses to advertise a route whose
NEXT_HOP is the peer's own address (RFC 4271 Section 5.1.3), so such a peer
receives nothing while looking established. The refusal is
`originatedNextHopIsPeerOwn` and `egressNextHopIsPeerOwn`, both in
`internal/component/bgp/reactor/forward_next_hop.go`.

```
bgp {
    router-id 1.2.3.4
    session { asn { local 65000; } }

    group rr-clients {
        timer { receive-hold-time 180; }
        session { capability { route-refresh enable; } }

        peer router-east {
            connection {
                remote { ip 10.0.0.1; }
                local { ip 10.0.0.254; }
            }
            session { asn { remote 65001; } }
        }
        peer client-b {
            connection {
                remote { ip 10.0.0.2; }
                local { ip 10.0.0.254; }
            }
            session { asn { remote 65002; } }
            timer { receive-hold-time 90; }    # Overrides group's 180
        }
    }

    group edge-peers {
        timer { receive-hold-time 30; }
        peer edge-gw {
            connection {
                remote { ip 192.168.1.1; }
                local { ip 192.168.1.254; }
            }
            session { asn { remote 64500; } }
        }
    }
}
```
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- peer-fields grouping -->

#### Peer Name Rules

| Rule | Detail |
|------|--------|
| Required | Every peer must have a name (it is the list key) |
| Unique | Two peers with the same name produce a config validation error |
| First character | ASCII letter, digit, or underscore (`[a-zA-Z0-9_]`) |
| Subsequent characters | ASCII letter, digit, underscore, hyphen, or dot (`[a-zA-Z0-9_.-]`) |
| CLI usage | `peer <name> <command>` selects the peer |
<!-- source: internal/component/bgp/config/resolve.go -- validatePeerName -->

#### Migration

`ze config migrate` converts old syntax:
- `template { bgp { peer <pattern> { inherit-name X; } } }` becomes `group X { }`
- `inherit X` in peers moves the peer into `group X`
- `neighbor` blocks become `peer` blocks inside groups

### bgp

Container for all BGP-related configuration (peers, groups, global settings).

```
bgp {
    router-id <ip>;           # Global router-id (inherited by all peers)
    session { asn { local <asn>; } }  # Global local AS (inherited by all peers)

    group <name> {
        # Group-level defaults (inherited by all peers in group)
        session { asn { remote <asn>; } }
        timer { receive-hold-time <seconds>; connect-retry <seconds>; }
        session { capability { ... } }
        session { family { ... } }

        peer <name> {
            # Connection (transport-level)
            connection {
                remote { ip <ip>; port <port>; connect <bool>; }
                local { ip <ip>; port <port>; accept <bool>; }
                md5 { password <string>; ip <ip>; }
                ttl { max <0-255>; set <0-255>; min <0-255>; }
            }

            # Session (BGP protocol)
            session {
                asn { local <asn>; remote <asn>; }
                router-id <ip>;
                link-local <ipv6>;
                family { ... }
                capability { ... }
            }

            # Behavior (operational knobs)
            behavior {
                group-updates <bool>;
                manual-eor <bool>;
            }

            # Other
            description <string>;
            timer { receive-hold-time <seconds>; send-hold-time <seconds>; connect-retry <seconds>; }
            rib { adj { in <bool>; out <bool>; } out { ... } }
            attach process <plugin-name> { ... }
            update { ... }
        }
    }
}
```
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- container bgp structure -->

**Migration:** `ze config migrate` converts old syntax:
- `neighbor` to `bgp { group <name> { peer } }`
- `template { inherit-name X }` to `group X { }`
- `inherit X` in peers moves peer into `group X`

---

## Peer Keywords

Peer configuration is organized into nested containers by concern.
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- peer-fields grouping -->

### connection (transport-level)

| Keyword | Type | Description |
|---------|------|-------------|
| `connection { local { ip; port; accept; } }` | container | Local address (`auto` or IP), listen port, accept inbound connections (default: true) |
| `connection { remote { ip; port; connect; } }` | container | Peer IP address, dial port, initiate outbound connections (default: true) |
| `connection { md5 { password; ip; } }` | container | TCP MD5 authentication (RFC 2385) |
| `connection { ttl { max; set; min; } }` | container | GTSM max (RFC 5082), outgoing TTL, minimum incoming TTL |

### session (BGP protocol)

| Keyword | Type | Description |
|---------|------|-------------|
| `session { asn { local; remote; } }` | container | Local and remote AS numbers |
| `session { router-id; }` | IP | BGP router ID override |
| `session { link-local; }` | IPv6 | IPv6 link-local address for MP_REACH_NLRI next-hop (RFC 2545) |
| `session { family { ... } }` | list | Address families with per-family prefix enforcement |
| `session { capability { ... } }` | container | BGP capabilities (see below) |
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- add-path inside capability container -->

### behavior (operational knobs)

| Keyword | Type | Description |
|---------|------|-------------|
| `behavior { group-updates; }` | bool | Group UPDATE messages for efficiency (default: true) |
| `behavior { manual-eor; }` | bool | Manual End-of-RIB control |

### Other peer-level keywords

| Keyword | Type | Description |
|---------|------|-------------|
| `description` | string | Peer description |
| `timer { receive-hold-time; send-hold-time; connect-retry; }` | container | Timer settings (defaults: 90s, 0/auto, 120s). A non-zero `send-hold-time` must be greater than `receive-hold-time` (RFC 9687 Section 4.4) |
| `rib { adj { in; out; } out { ... } }` | container | RIB configuration (adj-rib-in/out, outbound batching) |
| `attach process <name> { ... }` | list | Plugin process bindings |
| `update { ... }` | container | Route announcements |
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- grouping peer-fields, containers connection, session, behavior -->

### Capability Section

All capabilities support a four-mode vocabulary, declared once as the
`capability-mode` typedef and reused by every mode leaf under `capability`:

| Mode | Advertise? | Enforcement |
|------|------------|-------------|
| `enable` | Yes | None |
| `disable` | No | None |
| `require` | Yes | Reject peer if capability missing |
| `refuse` | No | Reject peer if capability present |
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- typedef capability-mode -->

**Simple capabilities** -- mode is the value:

```
capability {
    asn4 require;                  # Reject peers without 4-byte ASN
    route-refresh enable;          # Advertise, no enforcement
    graceful-restart enable;       # Advertise GR support
    extended-message require;      # Reject peers without extended message
    software-version disable;      # Don't advertise
}
```

**Removed capabilities** -- these are not supported and will be rejected:
`multi-session`, `operational`, `aigp`. These were ExaBGP-era capabilities with no ze runtime implementation.

**Block capabilities** -- `mode` key inside block (for capabilities with sub-parameters):

```
capability {
    graceful-restart {
        mode require;              # Reject peers without GR
        restart-time 120;
        long-lived-stale-time 3600;  # RFC 9494 LLGR period (0-16777215 seconds)
    }
}
```

**Nexthop** -- structured inline list keyed by family:

```
capability {
    nexthop {
        ipv4/unicast ipv6;         # IPv4 unicast with IPv6 next-hop (enable)
        ipv4/mpls-vpn ipv6 require; # IPv4 VPN with IPv6 next-hop (require mode)
    }
}
```

Each line is parsed as: `<family> <next-hop-afi> [<mode>]` where family is the list key, `next-hop-afi` is `ipv4` or `ipv6`, and mode defaults to `enable`.

**ADD-PATH** -- unified block with default direction, per-family overrides, and PATHS-LIMIT:

```
capability {
    add-path {
        direction send/receive;           # default for all families
        limit { receive 10; }             # default PATHS-LIMIT
        family {
            ipv4/unicast { mode require; } # per-family mode
        }
    }
}
```

The `direction` and `limit` on the container are inherited by all negotiated families. Per-family entries override direction, limit, or mode.
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- add-path capability container -->

**Defaults:** ASN4 defaults to `enable`. All other capabilities are absent (opt-in) -- they only participate in negotiation when explicitly configured.
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- leaf asn4 default enable -->

**Mode leaves:** `asn4`, `route-refresh` and `extended-message` are
`capability-mode` leaves, written `route-refresh enable` in a file and
`set ... capability route-refresh enable` in the editor. They take the four
modes only: a bare name (`route-refresh;`), a block (`route-refresh { }`) and
the words `true` and `false` are refused, at parse and at commit. A mode word
outside that vocabulary, on any capability, is a config error that names the
word; the reactor never defaults it to `enable`. `link-local-nexthop` is a leaf
too, and takes `enable` or `disable` only, because its plugin enforces neither
`require` nor `refuse`.
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- leaf route-refresh, leaf extended-message -->
<!-- source: internal/component/bgp/reactor/config_capabilities.go -- parseCapMode, capModeLeaf -->
<!-- source: internal/component/bgp/plugins/llnh/yang/ze-link-local-nexthop.yang -- leaf link-local-nexthop -->

### Family Section

```
family {
    ipv4/unicast;
    ipv4/multicast;
    ipv4/nlri-mpls;
    ipv4/mpls-vpn;
    ipv4/mup;
    ipv6/unicast;
    ipv6/multicast;
    ipv6/nlri-mpls;
    ipv6/mpls-vpn;
    ipv6/mup;
    l2vpn/vpls;
    l2vpn/evpn;
}
```

Block syntax is also supported: `ipv4 { unicast; multicast require; }`.
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- list family -->

### ADD-PATH Section

ADD-PATH and PATHS-LIMIT are configured inside `session > capability > add-path`:

```
capability {
    add-path {
        direction send/receive;      # default for all families
        limit { receive 10; }        # default PATHS-LIMIT (inherited)
        family {
            ipv4/unicast {
                direction receive;   # per-family override
                limit { receive 5; } # per-family limit override
                mode require;       # reject peer if missing
            }
            ipv6/unicast             # inherits direction + limit from container
        }
    }
}
```

The `direction` leaf and the `limit` container's `receive` leaf on the add-path container apply to all negotiated families unless overridden per-family. A `receive` limit is advertised only for a family whose direction includes receive, and the old scalar `limit N;` is refused with the replacement line. Per-family `mode` supports `enable` (default), `require`, `refuse`, `disable`.
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- container add-path, list family, leaf direction, container limit, leaf receive, leaf mode -->

### Process Section

```
# Named process binding (preferred)
attach process <plugin-name> {
    content {
        encoding json;       # json | text (default: inherit from plugin)
        format parsed;       # parsed | raw | full (default: parsed)
    }
    receive [ update state negotiated ];  # enum list of message types
    send [ update ];                      # enum list of sendable types
}
```

#### Receive enum values

| Value | Description |
|-------|-------------|
| `update` | Route announcements |
| `open` | Session open messages |
| `notification` | Error notifications |
| `keepalive` | Keepalive messages |
| `refresh` | Route refresh requests |
| `state` | Peer up/down events |
| `negotiated` | Capability negotiation results |
| `*` | Every event type, in both directions |
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- leaf-list receive, ze:validate receive-event-type -->

Plugins may register additional event types (e.g., `rpki`, `update-rpki`) that can also appear in receive lists. These are validated at runtime against the plugin registry.

**Direction is part of the type.** A plain type means both directions. A `-received` or `-sent` suffix means one: `receive [ update-received ]` asks for the UPDATEs the peer sends to ze, and `receive [ update-sent ]` asks for the ones ze sends to the peer. The two together mean both, which is what the plain type already says.

A token is resolved against the event registry whole before any suffix is cut, so a plugin type whose name ends in `-sent` keeps its name and is never read as a direction.

**`sent` on its own is not a type** and is refused. It named a direction, and the direction now belongs to the type it applies to, so `receive [ sent state ]` becomes `receive [ update-sent state ]`.

**`all` is not accepted.** `*` says the same thing without reading like a type name, and it says it deliberately: a config that names `*` accepts every event type a plugin registers later, which is exactly what the word `all` used to do by accident.

**`receive` decides delivery, and an absent block feeds nothing.** ze builds a
peer-to-process index from the resolved settings of every peer and consults it
on each peer-scoped event. A process is handed an event when the peer's block
grants that type in that direction AND the process subscribed to it. A peer
that states no block for a running process feeds it nothing, whatever the
process asked for at startup. The two halves are reported when they disagree,
at plugin ready and after every config apply.

<!-- source: internal/component/plugin/server/delivery_graph.go -- DeliveryGraph, PeerScopedProcs -->
<!-- source: internal/component/plugin/server/delivery_reconcile.go -- deliveryDisagreements -->

**A group's block reaches its members, and a member's list replaces it.**
`attach process` lives in the `peer-fields` grouping, so it is legal at
`bgp/group`, `bgp/group/peer` and `bgp/peer`. `receive` and `send` are
leaf-lists outside `cumulativePaths`, so a member that restates a process block
REPLACES the group's list for that member rather than adding to it. The index
is built after that merge, which is why a peer a dynamic group creates carries
its group's blocks under the address its connection arrived from: no config
document names its generated `dyn-<address>` identity, and none needs to.

<!-- source: internal/component/bgp/config/resolve.go -- ResolveBGPTree -->
<!-- source: internal/component/bgp/reactor/reactor_dynamic.go -- buildDynamicPeerSettings -->

#### Send enum values

| Value | Description |
|-------|-------------|
| `update` | Can inject routes |
| `refresh` | Can request route refresh |
| `raw` | Can send a whole BGP message it built itself, an OPEN or a NOTIFICATION included |
| `*` | Every message type, including every one a plugin registers later |
<!-- source: internal/core/bgp/events/events.go -- BaseSendTypes -->
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- leaf-list send, ze:validate send-message-type -->

`update`, `refresh` and `raw` are the three BASE types. A plugin registers more through
`Registration.SendTypes`, and naming one in a send list auto-loads the plugin
that enables it: `send [ enhanced-refresh ]` starts bgp-route-refresh, which
sends the RFC 7313 BoRR and EoRR markers. `*` names no type, so it auto-loads
nothing.
<!-- source: internal/component/plugin/registry/registry.go -- Registration.SendTypes -->

A direction suffix has no meaning in a send list, because every send type is sent.

**`all` is not accepted.** List send types explicitly, or write `*`.

Invalid enum values are rejected at parse time.

**`send` is enforced, not documentation.** A process reaches only the peers that
attach it with the type the command puts on the wire. A peer the command names
but the permission refuses is dropped and reported; a command whose every peer
refuses fails. `update` covers an announce, a withdrawal, an End-of-RIB marker, a
named commit, a cache forward and a stored-route relay; `refresh` covers a route
refresh, a BoRR, an EoRR and a soft clear. A command an operator types is not a
process and is not gated.

The reactor applies it in two places, and a peer-naming command reaches one of
them. Six commands resolve a peer SELECTOR and are gated there. Four more name
their peers directly, without a selector, and each applies the same check:
`send bgp <selector> cached <id>`, the `forward-cached` and
`relay-stored-route` plugin RPCs, and
`send bgp <selector> raw`.

**Raw has a word of its own: `raw`.** `send bgp <selector> raw ...` carries a whole
BGP message the caller chose, an OPEN or a NOTIFICATION included, so no other
send type describes it and `send [ update ]` does not imply it. Write
`attach process <name> { send [ raw ] }` for any program that injects raw
messages. Until 2026-08-30 the rail was gated on attachment alone, and a bare
`attach process <name> { }` was enough; that spelling no longer permits it.

<!-- source: internal/component/bgp/reactor/send_permission.go -- Peer.maySend, sendOrigin, filterPermittedPeers -->
<!-- source: internal/component/bgp/reactor/reactor_api.go -- getMatchingPeersSel, SendRawMessage -->

**A truthful `send [ update ]` or `send [ raw ]` moves the peer's End-of-RIB.**
The session's initial sync counts the bindings that can put a route on the wire
by either rail, and holds the marker until each one has sent `plugin session
ready`, so the marker means what RFC 4724 Section 4 says it means: this
speaker's initial routing update is complete, plugin-injected routes included. A
program granted one of those permissions that never sends the signal delays the
marker by the sync timeout instead. Every ze plugin that can be counted sends it
(bgp-rib, bgp-watchdog, bgp-rs, bgp-adj-rib-in), and a plugin author who
originates routes at peer-up must. A plugin with nothing to send still sends it,
on every peer-up: the barrier cannot tell "finished with nothing to send" from
"still working".

The wait does not queue anything. The initial sync has written every route it
owns before it starts waiting, so a route a program pushes during the wait goes
straight to the wire, in front of the marker, where a route belonging to the
initial update belongs.

**Two bindings are DERIVED, and only two.** Both come from the `redistribute`
root, and each is granted only when a rule implies it. A peer that already names
the process keeps its own binding unchanged.

| The rule | The derived binding |
|----------|---------------------|
| a source registered under the BGP protocol, such as `import bgp` | `receive [ update state refresh ]` toward the process `bgp-rib` runs under, because the Loc-RIB is the source and reads a peer's UPDATEs only through an `attach process` grant |
| `destination bgp` | `receive [ state ]` and `send [ update ]` toward the process `redistribute-orchestrator` runs under, because that plugin puts the route on the peer's wire and `send` is what permits it |

<!-- source: internal/component/bgp/config/redistribute_binding.go -- wireRedistributeDelivery -->

---

## Redistribute Block

`redistribute { destination <proto> { import <source> } }` is read once at load
and again on every SIGHUP reload, and a config it cannot turn into rules
REFUSES the load, and `ze config validate` and a commit refuse it too. Five
shapes fail, each naming what the operator typed: a source no component
registered, a family name that is not a registered address family, a
`destination` that imports nothing, a `destination` no protocol declared (the
refusal lists `redistribute.DestinationNames`, the declared set), and a rule
naming a source or a destination whose protocol has no block in the config (`import bgp` or `destination bgp`
with no `bgp { }` block, and the same for `isis` and `ospf`). The root each
source or destination needs is declared from the protocol's own `init()`
(`RouteSource.ConfigRoot`, `Destination.ConfigRoot`), before any plugin starts,
so the check names no protocol.

The rules are installed by the redistribute orchestrator, the plugin that owns
the `redistribute` root, from its own config section at startup and at each
reload. The block needs no `bgp` block: redistribution between IS-IS, OSPF,
static and connected runs on a router with no BGP.

The refusal replaced a warning. The warning left the global evaluator nil, which
disabled every redistribution rule in the file rather than the one line that was
wrong, and its only trace was a startup log line. `ze doctor` on a config file
reports either fault through its config-parse diagnostic, because the load
refuses both. Its post-config check names them under
`doctor-redistribute-unknown-source` and
`doctor-redistribute-unknown-destination` for a tree that reaches it.

An empty rule list installs an EMPTY evaluator rather than leaving the previous
one standing, so a reload that removed the last `redistribute` block stops
redistributing.
<!-- source: internal/component/config/loader_redistribute.go -- ExtractRedistributeRules -->
<!-- source: internal/component/config/loader_redistribute.go -- RefuseRedistributeRules -->
<!-- source: internal/component/config/redistribute/destination.go -- Destination, RegisterDestination -->
<!-- source: internal/plugins/redistribute/config.go -- ruleState -->
<!-- source: internal/component/config/doctor_redistribute.go -- checkRedistributeRules -->

---

## Filter Block

Route filtering via named filter instances. Appears in `peer-fields` grouping
(group and peer levels) and the `bgp` container (global level).

```
filter {
    import { filters [ rpki:validate community:scrub ]; final accept; }
    export { filters [ aspath:prepend ]; final reject; }
}
```

| Keyword | Type | Description |
|---------|------|-------------|
| `import`, `export` | presence container | Configure one direction's policy |
| `filters` | ordered leaf-list of string | Named filter instances or `<plugin>:<filter>` references; may be empty |
| `final` | mandatory enumeration | `accept` or `reject`, with no default |

Chains are cumulative across config levels (bgp > group > peer). A member can
be written three ways, and all three resolve to the same dispatch target: the
plain instance name `NO-TRANSIT`, the filter-type form `reject-asn:NO-TRANSIT`,
and the plugin form `bgp-filter-path-asn:NO-TRANSIT`.

The most-specific present direction supplies the final disposition; an absent
child inherits it. Each original bgp, group and peer layer is validated before
inheritance, even when it has no peers. A child cannot repair a parent's missing
final. Dynamic group templates take the cumulative bgp/group filters and the
group's final when present.

Filter accept continues and reject stops. On exhaustion, including an explicitly
empty policy, the final decides. Full payload modifications end filter traversal
but still receive the final decision. With no explicit policy at any inherited
level, EBGP rejects at exhaustion and IBGP accepts. Automatic and inactive
references do not establish explicit policy presence (RFC 8212 Section 3).
<!-- source: internal/component/bgp/filterapi/policy.go -- Policy.AcceptsAtEnd -->
The old `import [ NAME ];` and `export [ NAME ];` forms are refused: write
`import { filters [ NAME ]; final accept; }` or the corresponding export policy.

The plain `MY-LOOP`, type-qualified `loop-detection:MY-LOOP`, and canonical
`loop:MY-LOOP` forms identify the same configured instance. The effective chain
retains the canonical reference, but its execution belongs to the registered
in-process protocol stage, not to a plugin process named after the instance.
The hook runs once at that stage; it is not called again by the per-peer RPC
chain. This also applies to dynamic group templates.
Active named internal references are supported only on import. Config refuses
one on export rather than treating an unexecuted check as acceptance; an
inactive reference remains an explicit no-execution decision.
The refusal identifies the original bgp, group, or peer layer and prints a
`deactivate ... filter export filters <reference>` command using the spelling
written there, with identifiers quoted for the editor. Run the printed `top`,
`deactivate`, and `commit now` commands in the configuration editor. An inherited
reference must be deactivated where it was written, not at the affected peer.
This keeps the inactive member, other references, and the direction's final.
<!-- source: internal/component/bgp/config/peers.go -- validateExportPolicyReferences -->

One filter is injected rather than written: `prependDefaultFilters`
(`internal/component/bgp/config/peers.go`) puts each `loop-detection` entry
defined under the top-level `policy` block at the head of every peer's import chain. An operator
suppresses that injection by naming the entry in the chain, and deactivating a
named member turns the filter off for that peer:

```
filter {
    import {
        filters [ MY-LOOP ];
        inactive: filters MY-LOOP;
        final accept;
    }
}
```

A deactivated member stays in the chain and never executes. It is how an
operator records a decision to run a session without a check, and the config
layer reads it as a decision rather than as an absence: `filterChainContains`
counts it, and so does the transit-leak obligation below.

There is no mandatory filter an operator cannot configure, and no `overrides`
keyword. RFC 9234 Only-To-Customer marking is applied by the `bgp-role` plugin
from the peer's `role` block, not by a filter named in a chain.
Automatic loop references do not invent an operator policy or change an explicit
final. Config and runtime carry a single validated policy value so references
and disposition belong to the same generation.
<!-- source: internal/component/bgp/filterapi/policy.go -- Policy, NewPolicy, InternalPolicy -->

### The transit-leak obligation

A peer that declares an RFC 9234 role must name, in the chains that role binds,
a filter of a type that can refuse a path through a transit provider
(RFC 7454 Section 9). `reject-asn` is such a type. A config that omits it is
REFUSED, naming the peer, the role and the chain.

| Local role | Remote is | Import chain | Export chain |
|------------|-----------|--------------|--------------|
| `peer` | a settlement-free peer | REQUIRED | REQUIRED |
| `provider` | our customer | REQUIRED | not required |
| `customer` | our transit provider | not required | REQUIRED |
| `rs` | a client of the route server we run | REQUIRED | REQUIRED |
| `rs-client` | the route server we are a client of | REQUIRED | REQUIRED |
| absent | undeclared | not required | not required |

RFC 9234 names the LOCAL speaker's position, so `customer` means the remote is
our upstream. A deactivated member satisfies the obligation: naming the filter
and switching it off records the decision. A peer or a dynamic group that declares
no role is accepted, and `ze doctor` enumerates them under
`doctor-bgp-peer-no-role`.

Validation checks local policy filter names against the filter types of the
top-level `policy` root (the lists marked `ze:filter`, `policy.BuildRegistry`),
canonicalizes short filter-type references through the plugin registry, and leaves explicit `<plugin>:<filter>`
references for runtime dispatch. Runtime calls use the `filter-update` callback
on the selected plugin.

<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- filter containers -->
<!-- source: internal/component/policy/yang/ze-policy-conf.yang -- policy container -->
<!-- source: internal/component/bgp/config/peers.go -- policyRegistryOf, validateChainNames -->
<!-- source: internal/component/bgp/config/redistribution.go -- redistribution config parsing -->
<!-- source: internal/component/policy/registry.go -- BuildRegistry, ValidateName -->
<!-- source: internal/component/plugin/registry/registry.go -- FilterTypes registry -->
<!-- source: internal/component/bgp/config/peers.go -- validateLeakFilterObligations -->

---

## Update Block (Ze-Native Routes)

The `update { attribute {} nlri {} }` block is the ze-native way to announce routes in configuration files. All ze-native route config uses this format.

### Attribute Block

Path attributes shared by all NLRI in the update block.

```
update {
    attribute {
        origin igp;
        next-hop 10.0.0.1;
        local-preference 100;
        med 200;
        as-path [ 65001 65002 ];
        community [ 65001:100 65001:200 ];
        extended-community [ target:65001:100 ];
        large-community [ 65001:100:200 ];
        aggregator 65001:10.0.0.1;
        atomic-aggregate enable;
        originator-id 10.0.0.1;
        cluster-list [ 3.3.3.3 192.168.201.1 ];
        path-information 1.2.3.4;
        label 16000;
        labels [ 100 200 300 ];
        split /25;
        attribute [ 0x04 0x80 0x00000064 ];   # generic hex attributes
    }
    nlri {
        ...
    }
}
```
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- container attribute, leaf origin, leaf next-hop, leaf-list community -->

### NLRI Grammar

```
<nlri-line> := <family> [rd <rd>] [label <label>] <op> <payload> ;
<op>        := add | del | eor
<payload>   := <bracket-list> | <structured-payload>
```

- `<family>` -- address family, e.g. `ipv4/unicast`, `ipv4/mpls-vpn`, `ipv4/flow`
- `rd` / `label` -- optional VPN qualifiers, placed before the operation keyword
- `<op>` -- **mandatory** operation: `add` (announce), `del` (withdraw), `eor` (end-of-rib, no payload)
- `<bracket-list>` -- `[ prefix1 prefix2 ... ]` -- for prefix-based families, one route per entry
- `<structured-payload>` -- for complex families (FlowSpec, VPLS, EVPN), one route per line
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- list nlri, leaf content -->

### Payload dispatch

| Family Category | Bracket List? | Example |
|----------------|---------------|---------|
| Prefix (unicast, multicast, mpls, mpls-vpn) | Yes | `ipv4/unicast add [ 10.0.0.0/24 10.0.1.0/24 ];` |
| FlowSpec (flow, flow-vpn) | No -- one per line | `ipv4/flow add source-ipv4 10.0.0.2/32;` |
| VPLS | No -- one per line | `l2vpn/vpls rd X add ve-id 5 ve-block-offset 1 ...;` |
| EVPN | No -- one per line | `l2vpn/evpn add <route-type-specific>;` |

After `add`, if the next token is `[` the parser reads a bracket list of single-token entries (one route per token). Otherwise it reads one structured NLRI until `;`.

### NLRI Examples

```
update {
    attribute {
        origin igp;
        next-hop 10.0.0.1;
        local-preference 100;
    }
    nlri {
        # Simple unicast
        ipv4/unicast add 10.0.0.0/24;
        ipv4/unicast add [ 10.0.1.0/24 10.0.2.0/24 10.0.3.0/24 ];

        # Withdrawal
        ipv4/unicast del 10.0.99.0/24;

        # End-of-RIB
        ipv4/unicast eor;
    }
}

# VPN with rd and label qualifiers before add
update {
    attribute { origin igp; next-hop 192.168.0.1; }
    nlri {
        ipv4/mpls-vpn rd 100:100 label 20012 add 10.0.0.0/24;
        ipv4/mpls-vpn rd 100:100 label 20012 add [ 10.0.1.0/24 10.0.2.0/24 ];
    }
}

# VPLS
update {
    attribute { origin igp; next-hop 192.168.201.1; }
    nlri {
        l2vpn/vpls rd 192.168.201.1:123 add ve-id 5 ve-block-offset 1
               ve-block-size 8 label-base 10702;
    }
}
```

### FlowSpec

FlowSpec routes (RFC 8955) use the same `update { nlri { } }` syntax. The legacy `flow { route { match {} then {} } }` block is not supported -- use `ze config migrate` to convert ExaBGP-format FlowSpec configs.

FlowSpec actions are expressed as extended-community attributes. Match criteria are inline after the family and `add` keyword.

```
# Simple discard rule
update {
    attribute {
        extended-community [ rate-limit:0 ];
    }
    nlri {
        ipv4/flow add source-ipv4 10.0.0.1/32;
    }
}

# Complex match with redirect
update {
    attribute {
        extended-community [ redirect:65500:12345 ];
    }
    nlri {
        ipv4/flow add destination-ipv4 192.168.0.1/32 source-ipv4 10.0.0.2/32
               protocol [ =tcp =udp ] destination-port [ >8080&<8088 =3128 ]
               source-port >1024;
    }
}

# FlowSpec VPN (with rd qualifier before add)
update {
    attribute {
        extended-community [ rate-limit:0 ];
    }
    nlri {
        ipv4/flow-vpn rd 65535:65536 add source-ipv4 10.0.0.1/32;
    }
}
```

#### Match Criteria

| Criterion | Description |
|-----------|-------------|
| `source-ipv4` / `source-ipv6` | Source prefix |
| `destination-ipv4` / `destination-ipv6` | Destination prefix |
| `protocol` | IP protocol, a number (0 to 255) or a protocol name, under any match operator: `=tcp`, `!=udp`, `>5`, `[ >=6&<=17 ]`, `>=tcp&<=udp`. Config and update text read it with one grammar (`parseNumericTermsText`), and a malformed value is refused with the value and the fix |
| `port` | Source or destination port, 0 to 65535 |
| `destination-port` | Destination port, 0 to 65535 |
| `source-port` | Source port, 0 to 65535 |
| `next-header` | IPv6 next header, the same component and grammar as `protocol` |
| `tcp-flags` | TCP flags (`syn`, `rst`, `fin`, `ack`, `urg`, `push`) |
| `icmp-type` | ICMP type, 0 to 255 or a type name (`echo-request`, `unreachable`, ...) |
| `icmp-code` | ICMP code, 0 to 255 or a code name (`host-unreachable`, ...) |
| `fragment` | Fragment flags (`dont-fragment`, `is-fragment`, `first-fragment`, `last-fragment` and their short aliases), each optionally led by `!` (not), `=` (match) or `&` (AND with the term before). Config and update text read it with one bitmask grammar (`parseBitmaskValue`) |
| `dscp` | DSCP value, 0 to 63 |
| `packet-length` | Packet length, 0 to 65535 for both families (RFC 8955 Section 4.2.2.10, which RFC 8956 Section 3 applies to IPv6) |
| `traffic-class` | The ExaBGP spelling of `dscp`. RFC 8956 Section 8 registers component type 11 under one name for both families, so the two words build one component |
| `flow-label` | IPv6 flow label, 0 to 1048575 (the 20-bit field of RFC 8956 Section 3.7) |

#### Match Operators

| Operator | Meaning |
|----------|---------|
| `=` | Equal |
| `>` | Greater than |
| `>=` | Greater or equal |
| `<` | Less than |
| `<=` | Less or equal |
| `!=` | Not equal |
| `&` | AND with previous |

Every numeric criterion (`protocol`, `next-header`, the three ports,
`packet-length`, `icmp-type`, `icmp-code`, `dscp`, `traffic-class`,
`flow-label`) takes these operators, in config and in update text, through one
parser (`parseNumericTermsText`) with one width table (`componentMaxValue`) and
one name table (`componentValueNames`). A value wider than its field, such as
`destination-port 65536`, a term with no value after `&`, a leading `&` with no
term before it, and a value that is neither a number nor a known name are each
refused at load with the criterion, the value and the fix. A bare value means
`=`. Successive terms are ORed; `&` inside a token ANDs the next term with the
one before it, so `[ >8080&<8088 =3128 ]` is (>8080 AND <8088) OR =3128.
The bitmask criteria, `tcp-flags` and `fragment`, take one grammar in config
and in update text through one parser (`parseBitmaskTermsText`), ExaBGP 6.0's.
`&` ANDs the next term with the one before it and successive terms are ORed,
as for numeric criteria: `syn&ack` is SYN set AND ACK set, `[ syn ack ]` is SYN
set OR ACK set. `+` names several bits of one operand: `syn+ack` is SYN or ACK
set, and `=syn+ack` both set. A term may lead with `=` (all the operand's bits
set, RFC 8955 Section 4.2.1.2 match bit), `!` (none set) or `!=` (not all set),
and its operand is a flag name or a number from 0 to 255. A leading `&` with no
term before it, an `&` or `+` with nothing after it, and an unknown flag are
refused at load with the criterion and the fix.

#### FlowSpec Actions (Extended Communities)

| Extended Community | Action |
|--------------------|--------|
| `rate-limit:0` | Discard (rate-limit to zero) |
| `rate-limit:<bps>` | Rate limit in bytes per second |
| `rate-limit:<bps>:bytes` | Explicit bytes-per-second form, canonicalized to `rate-limit:<bps>` |
| `rate-limit:<pps>:packets` | Rate limit in packets per second |
| `redirect:<asn>:<value>` | Redirect to the VRF that imports this route-target. A 2-octet AS takes type 0x80 with a 4-octet value; a 4-octet AS takes type 0x82 with a 2-octet value |
| `redirect:<ipv4>:<value>` | The same redirect with an IPv4-address route-target: type 0x81 with a 2-octet value |
| `redirect-to-nexthop <ipv4>` | Redirect to one IPv4 address: the draft-ietf-idr-flowspec-redirect-ip community, type 0x01 sub-type 0x0c, copy bit clear |
| `redirect-to-nexthop <ipv6>` | Redirect to one IPv6 address. This community is 20 octets, so it travels in IPV6_EXTENDED_COMMUNITIES (attribute 25, RFC 5701) instead of attribute 16 |
| `copy-to-nexthop <ipv4>`, `copy-to-nexthop <ipv6>` | Copy the matched traffic to one address: the same community as `redirect-to-nexthop <address>` with the copy ("C") bit of its local administrator set (draft-ietf-idr-flowspec-redirect-ip Section 2) |
| `redirect-to-nexthop-draft` | Redirect to the route's next hop: community 0x0800 (draft-simpson-idr-flowspec-redirect), which carries the target in the MP_REACH_NLRI next hop |
| `copy-to-nexthop` | Copy to the route's next hop: community 0x0800 with its copy bit. Only an address after the word selects the IETF form above |
| `action sample`, `action terminal`, `action sample-terminal`, `action none` | Sampling and terminal action bits. A word outside this set is refused |
| `mark <dscp>` | Set DSCP value, 0 to 63. RFC 8955 Section 7.5 gives the DSCP six bits, so a larger value is refused instead of truncated |

The three redirect encodings are the three RFC 8955 Section 7.4 names: the administrator you write selects the type, and the type fixes how large the value can be. A value too large for its form is refused, because a truncated route-target names a different VRF.

A rate must be a finite number. A negative rate is refused because RFC 8955 Section 7.1 forbids encoding one, and `NaN` and `Inf` are refused because the RFC gives a rate or the zero that means discard, and a decoder reads `NaN` back as that zero.

Legacy ExaBGP 5.0 packet-rate syntax `rate-limit-packets:<pps>` is still accepted during parsing. ExaBGP's explicit byte form `rate-limit:<bps>:bytes` is also accepted and normalized to bare `rate-limit:<bps>`.

### Watchdog

Routes can be tagged with a watchdog group and held until explicitly announced:

```
update {
    attribute { origin igp; next-hop 10.0.0.1; }
    watchdog { name mypool; withdraw true; }
    nlri {
        ipv4/unicast add 10.0.0.0/24;
    }
}
```

Standalone watchdog commands (via API):
```
request bgp watchdog announce <name>   # send all routes in pool to peers
request bgp watchdog withdraw <name>   # withdraw all routes in pool from peers
```
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- container watchdog, leaf name, leaf withdraw -->

---

## Value Types

### IP Address

```
192.168.1.1
2001:db8::1
```

### Prefix

```
10.0.0.0/8
2001:db8::/32
```

### ASN

```
65001           # 2-byte
4200000001      # 4-byte
auto            # Use local-as
```

### Community

```
65001:100       # Standard (AS:value)
0x12345678      # Numeric
no-export
no-advertise
no-export-subconfed
nopeer
```

### Extended Community

```
target:65001:100
origin:65001:100
redirect:65001:100
l2info:19:0:1500:111
0x0002fde800000001      # Raw hex (8 bytes)
```

### Large Community

```
65001:100:200   # ASN:value:value
```

### Route Distinguisher

```
65001:100       # Type 0 (ASN2:value)
192.168.1.1:100 # Type 1 (IP:value)
4200000001:100  # Type 2 (ASN4:value)
```

### Boolean

```
enable          # or true
disable         # or false
```

A boolean leaf takes these four words and nothing else. The parser normalizes
`enable` to `true` and `disable` to `false` before the tree stores the value.
`require` and `refuse` are capability modes, an enumeration of their own
(`capability-mode` above), and a boolean leaf given either is refused with
`invalid bool`.
<!-- source: internal/component/config/schema.go -- ValidateValue, normalizeEnableBool -->

### Origin

```
igp
egp
incomplete
```

---

## Inheritance

### Peer Group Inheritance

```
bgp {
    router-id 1.2.3.4                   # BGP-level global
    session { asn { local 65000; } }     # BGP-level global

    group rr-clients {
        timer { receive-hold-time 90; }  # Group default
        session {
            family { ipv4/unicast { prefix { maximum 1000000; } } }
        }

        peer client-a {
            connection {
                remote { ip 192.168.1.2; }
                local { ip 192.168.1.1; }
            }
            session { asn { remote 65002; } }
        }
    }
}
```

### Key Concepts

| Concept | Description |
|---------|-------------|
| BGP globals | `session { asn { local; } }` and `router-id` at bgp level, inherited by all peers |
| Group defaults | Any `peer-fields` set on the group, inherited by all peers in that group |
| Peer overrides | Any `peer-fields` set on the peer, takes highest precedence |
| Deep merge | Containers (e.g., `session`, `timer`) merge keys from both group and peer |
| Leaf override | Scalar values (e.g., `receive-hold-time` inside `timer`) at peer level replace group values |
<!-- source: internal/component/bgp/config/resolve.go -- ResolveBGPTree, deepMergeMaps -->

### Standalone Peers

Peers directly under `bgp` (not inside a group) inherit only BGP-level globals:

```
bgp {
    router-id 192.0.2.1
    session { asn { local 65000; } }

    peer my-peer {
        connection {
            remote { ip 10.0.0.5; }
            local { ip 10.0.0.1; }
        }
        session { asn { remote 65001; } }
        timer { receive-hold-time 180; }
    }
}
```

---

## YANG Defaults Do Not Reach a Config On Their Own

A `default` statement on a YANG leaf is SCHEMA METADATA. It is not a value in
the parsed config tree, and a component that reads its config section directly
sees the leaf as absent.

`config.ApplyDefaults` (`internal/component/config/schema_defaults.go`) is what
puts a declared default into a tree, and it must be called with the schema node
for the subtree being filled. Two things follow, and both have produced defects:

- A component that never calls it reads every unwritten leaf as missing. If it
  then treats missing as zero, or as "no configuration", the declared default is
  documentation that no code obeys.
- Where a component DOES call it, the pattern is three steps: `config.YANGSchema()`,
  `schema.Lookup("<container path>")`, then `config.ApplyDefaults(subtree, node)`.
  `applyPeerSchemaDefaults` (`internal/component/bgp/config/peers.go`) fills peer
  entries this way, and `parseAdminDistanceConfig`
  (`internal/component/sysrib/sysrib.go`) fills the protocol distance table.

**A failure to load the schema is not a reason to carry on with an empty tree.**
`applyPeerSchemaDefaults` returns silently on both error paths; the distance
parser returns the error instead, because that value decides which route the
kernel installs and a silent empty answer is indistinguishable from a
deliberate one. Which behavior is right depends on what the missing defaults
would cause, and the choice is made deliberately rather than by copying.

## An Absent Config Root Delivers No Section At All

A component that declares a config root is called with that root's section, and
it is NOT called when the operator wrote no block for it.
`BuildPluginConfigSections` (`internal/component/config/plugin_verify.go`) asks
`ExtractConfigSubtree` for each declared root, and skips the root when the answer
is nil, so the configure callback never runs.

<!-- source: internal/component/config/plugin_verify.go -- BuildPluginConfigSections, ExtractConfigSubtree -->

The consequence is the trap: state a component initializes only inside its
configure callback stays at its Go zero value on every config that omits the
block. Where one config in the tree carries the block and the rest do not, every
test passes and the feature is inert almost everywhere. `runSysRIBPlugin`
(`internal/component/sysrib/register.go`) shipped that way on 2026-09-04: the
administrative distance table was resolved in `OnConfigure` alone, so a config
with no `rib` block left every protocol on its producer's constant permanently.

<!-- source: internal/component/sysrib/register.go -- runSysRIBPlugin, publishDistances -->

**Seed the declared state at start, then let a section override it.** The seed
is the same parse over an empty tree, so the schema stays the one declaration:
`parseAdminDistanceConfig("{}")` fills every leaf from its YANG default. The
rollback path needs the same seed, because a callback that never fired leaves no
previous value to restore and an empty map would strip what the seed installed.

`runSysRIBPlugin` now seeds two tables this way. `parseFIBImportConfig("{}")`
fills the FIB permission set beside the distance table, and a `rib {
fib-withhold }` list overrides it. Both tables take one route through the
callbacks: `OnConfigVerify` parses them, `OnConfigure` and `OnConfigApply`
install them, and the rollback restores the seed when the previous value is
empty.

<!-- source: internal/component/sysrib/register.go -- runSysRIBPlugin -->
<!-- source: internal/component/sysrib/fibimport.go -- parseFIBImportConfig, publishFIBImport -->

## Custom YANG Extensions

Ze defines custom extensions in `ze-extensions.yang` that control config parsing, validation, and UI behavior:

| Extension | Argument | Applied to | Purpose |
|-----------|----------|------------|---------|
| `ze:syntax` | mode | any node | Parser syntax mode (flex, freeform, inline-list, etc.) |
| `ze:key-type` | type | inline-list | Key type for inline lists |
| `ze:sensitive` | -- | leaf | Marks leaf as containing sensitive data |
| `ze:validate` | name | leaf, leaf-list | References a custom validator function |
| `ze:cumulative` | -- | leaf-list | Accumulate values across config inheritance levels |
| `ze:decorate` | name | leaf | Attaches display-time decorator for web UI |
| `ze:required` | path | list | Descendant field must have value after config inheritance resolution. Path argument is mandatory; bare `ze:required;` is rejected at schema load (use `mandatory true` for leaf-level required). Enforced generically for any list at validate, editor, and startup. |
| `ze:suggest` | path | list | Field shown in creation form with inherited defaults |
| `ze:listener` | -- | list | Marks a named `list server { key "name"; }` as a network listener. `CollectListeners` walks every enabled service with this extension at parse time and feeds `ValidateListenerConflicts`, which rejects overlapping `ip:port` pairs with a message naming both services. Covers web, ssh, mcp, looking-glass, prometheus, plugin hub, and api-server rest/grpc. |
| `ze:command` | handler | container | Marks a config-false container as executable CLI command |
| `ze:edit-shortcut` | -- | command | Available in edit mode as shortcut |
| `ze:display-key` | -- | leaf | Display label for keyless list entries in web UI |
| `ze:hidden` | value | leaf | Hidden from config display and web editor |
| `ze:allow-unknown-fields` | -- | container | Container accepts arbitrary key-value pairs |
| `ze:route-attributes` | -- | inline-list | Node accepts standard BGP route attributes |
| `ze:os` | goos | any node | Restricts node to a specific OS (any GOOS value); excluded from schema on other platforms |

<!-- source: internal/component/config/yang/modules/ze-extensions.yang -- all ze: extensions -->

---

## Related

- [ExaBGP Legacy Syntax](exabgp-syntax.md) -- `static`, `announce`, `flow` blocks accepted for migration
- [API Update Syntax](../api/update-syntax.md) -- API command syntax for route injection (not yet implemented)

---

**Last Updated:** 2026-04-02
