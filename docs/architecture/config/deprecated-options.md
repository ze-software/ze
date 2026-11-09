# Configuration Syntax Changes

<!-- source: internal/component/config/cli/cmd_migrate.go -- ze config migrate, form conversion only -->
<!-- source: internal/component/config/loader.go -- parseSetConfig, the unknown-field message -->

Ze has not shipped a release, so there is no deprecation lifecycle for its
config syntax and no path that rewrites an older form of it. When the syntax
changes, the configurations in the repository are converted in the same
change, and no other file written in the old form exists to convert.

A config file records the schema stamp of the release that wrote it, and
recovery after a downgrade chooses the newest rollback file that the running
binary can parse.

## Compatibility rules

| Case | What Ze does |
|------|--------------|
| Current config | `ze config validate <file>` checks it directly. |
| Older pre-release Ze syntax | The parse fails, naming the line and the unknown keyword. Edit the file to the current syntax. |
| ExaBGP syntax | `ze exabgp migrate` converts the shapes it supports and warns about the extensions it does not translate. |
| Newer schema stamp after downgrade | Startup tries rollback files newest-first and writes back the first one this binary can parse. |

## Unsupported ExaBGP extensions

`ze exabgp migrate` translates the capability keywords Ze has a session
setting for, and WARNS about every other keyword the block holds. The warning
goes to stderr, names the peer and names each keyword in one line. The migrated
session negotiates less than the ExaBGP config asked for, so read the warnings
before you deploy the output.

The set is derived, not listed: `migrateCapability` declares the keywords it
translates (`route-refresh`, `extended-message`, `link-local-nexthop`, `asn4`,
`graceful-restart`, `software-version`, `add-path`), and anything else in the
block reaches the warning. A keyword ExaBGP adds later is therefore reported
without an edit here.

| Syntax | Result |
|--------|--------|
| `capability { multi-session; }` | Warns: ``peer <name>: the capability block asks for "multi-session", which `ze exabgp migrate` does not translate, so the migrated config does not ask for it`` |
| `capability { aigp enable; }` | Same warning, naming `aigp`. Ze's AIGP codec is always on and has no per-peer switch, so there is no setting to translate the keyword into |
| Several in one peer | One warning names them all, in the order the block declares them |
| `operational { ... }` under an ExaBGP neighbor | `ze exabgp migrate` stops at the parser: `line N: unknown field in neighbor: operational` |
| `capability { multi-session; }` in Ze syntax | `ze config migrate` stops at the parser: `line N: unknown field in capability: multi-session` |
| `operational { ... }` under a peer, in Ze syntax | `ze config migrate` stops at the parser: `line N: unknown field in peer: operational` |

The capability block is the only place a warning replaces a refusal. An
`operational { ... }` block, in either syntax, is a parse error: the grammar has
no such field, so there is nothing to leave out.

Two of the keywords are worth knowing by name, because each comes from an IETF
draft that expired without becoming an RFC.

| Extension | Draft | Expired | Codepoint |
|-----------|-------|---------|-----------|
| `multi-session` | draft-ietf-idr-bgp-multisession-07 | 2013-03-16 | Section 4 assigns capability code 68 |
| `operational` | draft-ietf-idr-operational-message-00 | 2012-09-01 | Section 9 requests a capability code and a BGP message type from IANA. Neither was allocated |

`operational` therefore has no assigned value to put on the wire. ExaBGP fills
both with private values of its own, capability `0xB9` and message type `0x06`.
Ze does not copy a private codepoint.

<!-- source: internal/exabgp/migration/migrate_unimplemented.go -- untranslatedCapabilityWarning, the warning and the two drafts -->

## Commands

```bash
# Check whether a file matches the current binary
ze config validate config.conf

# Convert an ExaBGP config to Ze syntax
ze exabgp migrate exabgp.conf

# Rewrite a current Ze config in the set or hierarchical form
ze config migrate format hierarchical config.conf
```

See [Configuration Migration](../../config-migration.md) for command details.
