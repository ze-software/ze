# VPP `linux-cp` Plugin -- Vendored Reference Source

**This is FOREIGN code. It is not ze. It is not built, linted, or tested here.**
Nothing in this directory compiles into any ze binary. It exists to be *read*.

Upstream: [FD.io VPP](https://github.com/FDio/vpp), Apache-2.0 (see `LICENSE`).
Copyright Cisco and/or its affiliates and the VPP contributors, not Exa Networks.

---

## Drift warning -- READ THIS BEFORE YOU TRUST A LINE OF IT

**This copy is frozen at VPP v25.10. The VPP an operator actually runs is a
different program.** VPP is a moving target, and the line numbers cited below
move between releases even when the code does not. The netns fallback in
`lcp_itf_pair_create` is byte-identical in `stable/2306`, `stable/2402` and
`v25.10`, yet it sits at line 820, 829 and 854 respectively. **Cite this tree by
function name, not by line number**, unless you also say which version you mean.
A line number quoted without a version has already been wrong here more than
once.

This tree is a reference for reading *semantics*, and it is **NOT the authority
for any given deployment.**

If you are chasing a production bug, or an operator reports behaviour that
contradicts this source, **this file does not settle it.** Check the VPP version
that operator is running, then check that version's source. This copy tells you
what VPP v25.10 does. It cannot tell you what VPP 24.06, or the VPP in a distro
package, or the VPP on the box in front of you does.

The semantics documented below were checked identical across `stable/2306`,
`stable/2402`, and `v25.10`, which is why they are worth writing down. That is
evidence of stability so far. It is not a promise about the next release, and it
says nothing about releases older than 2306, which nobody checked.

---

## Why this exists

Ze talks to VPP's `linux-cp` plugin through generated Go bindings
(`vendor/go.fd.io/govpp/binapi/lcp/lcp.ba.go`). **A generated binding stub tells
you a field exists. It cannot tell you what the foreign system does with it.**
Reading the stub and inferring VPP's behaviour is fabrication, banned by
`ai/rules/evidence.md`.

This is not a hypothetical. Twice in one day (2026-07-16) and across three
earlier sessions, agents read `lcp.ba.go` and inferred VPP's netns semantics from
it. Every one of them was wrong. The stub declares a per-pair `Netns` field and
even carries the upstream comment
`netns - optional tap netns; netns[0] == 0 if none`. That comment is exactly as
far as the stub goes: it says "if none" and never says what *none resolves to*,
nor what a netns string even is. Both answers live only in the C:

| Question the stub cannot answer | The C that answers it |
|---|---|
| What does VPP do when a pair's netns is empty? | `src/lcp_interface.c:854-855` -- `lcp_itf_pair_create` falls back to the **global default**: `if (ns == 0 \|\| ns[0] == 0) ns = lcp_get_default_ns ();` |
| What *is* a netns value to VPP? | `src/lcp.c:73` -- `lcp_set_default_ns` formats `/var/run/netns/%s`, proving it is a literal namespace **name**, resolved as a path. `host` and `root` are not special; they ask for namespaces literally called `host` and `root`. |
| What does "no default netns" mean? | `src/lcp.c:32-35` -- `lcp_get_default_ns` returns `NULL` when unset or empty, i.e. VPP's own OS default, not "the host namespace by name". |

Those facts falsified, at once: a code comment in
`internal/plugins/iface/vpp/lcp.go`, a section of the operator guide
(`docs/guide/vpp.md`, corrected in `c49d36524` against this C), and the
remediation text `ze doctor` prints for `doctor-vpp-lcp-netns`. All three had
told operators to set the netns to `host` or `root`. Per the C above, that asks
VPP for a namespace that normally does not exist.

That is the cost this directory is meant to prevent. **Read the C. Cite the C.**

## What is here, and what is deliberately not

Vendored: the files that answer the questions ze actually asks of `linux-cp`
(how a netns is named, resolved, and defaulted; what the binary API accepts).

| File | Why it is here |
|------|----------------|
| `src/lcp.c` / `src/lcp.h` | Global default netns state: `lcp_set_default_ns` / `lcp_get_default_ns`, and the `/var/run/netns/%s` resolution. |
| `src/lcp_interface.c` / `src/lcp_interface.h` | The keystone file. `lcp_itf_pair_create`: the per-pair netns fallback to the global default, and TAP pair lifecycle. Also `lcp_itf_pair_config` (`VLIB_EARLY_CONFIG_FUNCTION (..., "linux-cp")`), which parses the `startup.conf` stanza ze generates. |
| `src/lcp_api.c` | The binary API handlers, i.e. what actually happens when ze calls `lcp_itf_pair_add_del` over GoVPP. |
| `src/lcp.api` | The API definition `lcp.ba.go` is **generated from**. Useful to see what survived generation and what did not. |
| `src/lcp_cli.c` | The `vppctl` surface (`lcp create ... netns ...`, `lcp default netns ...`), for comparing operator-facing behaviour with the API path. |
| `src/lcp.rst` | Upstream's own plugin documentation. |

Deliberately NOT vendored, to keep this a purposeful reference rather than a
fork of VPP: `lcp_nl.c/h` and `lcp_router.c` (netlink listener and route sync;
ze programs the FIB itself via `fib-vpp`), `lcp_node.c` (packet graph nodes),
`lcp_adj.c/h` (adjacency), `lcp_interface_sync.c`, `lcp_mpls_sync.c`, the
`test/` directory, `CMakeLists.txt`, and `FEATURE.yaml` (build system; nothing
here is built). The rest of VPP is not vendored at all.

If a future question needs one of those files, fetch it at the same pin and add
it here with a row explaining which question it answers. Do not vendor the tree
"for completeness".

## Provenance

| Field | Value |
|-------|-------|
| Upstream repo | `https://github.com/FDio/vpp` (GitHub mirror of `gerrit.fd.io/r/vpp`) |
| Release tag | `v25.10` (annotated tag `52111fc965d60a3f19c897707bed222c18a7d83d`, tagged 2025-10-29 by Andrew Yourtchenko) |
| Commit SHA | `cbba0451bb0af02a3ab8e163f6f99062782258e6` |
| Upstream path | `src/plugins/linux-cp/` -> `src/` here; repo-root `LICENSE` -> `LICENSE` here |
| Fetched | 2026-07-16 |
| Licence | Apache-2.0 (upstream `LICENSE`, verbatim) |

**Why v25.10 and not master, and not `stable/2306`:** a release tag is immutable,
where a branch tip moves under you and makes "the commit we vendored"
unanswerable a week later. v25.10 is also the version ze actually validates
against: `scripts/evidence/effective-vpp-iface.py` drives `ligato/vpp-base`, and
the vendored GoVPP binapi is the 25.10 revision
(`internal/plugins/iface/vpp/wireguard.go:17`). A reference copy should match the
VPP the project tests against, not an arbitrary older branch. The netns
behaviour is identical in `stable/2306` and `stable/2402`, so nothing is lost by
picking the newest.

### Integrity

Verbatim upstream, byte for byte. **Never edit these files** -- a modified copy
is worse than no copy, because its only value is being what upstream really says.

| File | Bytes | SHA-256 |
|------|-------|---------|
| `src/lcp.c` | 3611 | `071ee7964210ebeed477edc2c515b332bed22c9ef9e460e0dce2bf067f51be0d` |
| `src/lcp.h` | 2465 | `14027d2cc86a2319c436b03a58e64348201c4dcdbf6c61163a99a9b9104993bb` |
| `src/lcp_interface.c` | 34174 | `f92fa278e00c50f8942523db46a078a20ccf080e4b7070fa283478fc92a82ad9` |
| `src/lcp_interface.h` | 6313 | `cbd28bed847d3cd78fc364e8518a30b229dd76851c88fa2c4dd972e64ded7432` |
| `src/lcp_api.c` | 8707 | `3a181b7ed55614ec7a247f9cd961a505fe07d6619eab7c5d3c7a06e5b15fb206` |
| `src/lcp.api` | 6693 | `95b30e7413fcab0d5a6618f6f3bb32070b8f94d304df01deeb3cf740fb0af182` |
| `src/lcp_cli.c` | 10929 | `d4a0d3defac30bb9b06fde3821b15c4a617e6cf7a102065c5049d21940ea68b7` |
| `src/lcp.rst` | 3454 | `06ddc72194ce90dabaa1e681cc4f04a4a9e36297682c54d0429383c556ad9bdd` |
| `LICENSE` | 11357 | `58d1e17ffe5109a7ae296caafcadfdbe6a7d176f0bc4ab01e12a689b0499d8bd` |

## Re-fetching

Nothing automated re-fetches or verifies this tree, by design: a reference frozen
at a pin has nothing to drift *against*, and a gate that re-fetches on every
build would trade an offline, reproducible clone for a network dependency. Bump
it by hand when ze starts targeting a new VPP release, and update the pin, the
date, and the hashes above in the same change.

```bash
SHA=cbba0451bb0af02a3ab8e163f6f99062782258e6   # tag v25.10
for f in lcp.c lcp.h lcp_interface.c lcp_interface.h lcp_api.c lcp.api lcp_cli.c lcp.rst; do
  curl -fsS -o "third_party/vpp-linux-cp/src/$f" \
    "https://raw.githubusercontent.com/FDio/vpp/$SHA/src/plugins/linux-cp/$f"
done
curl -fsS -o third_party/vpp-linux-cp/LICENSE \
  "https://raw.githubusercontent.com/FDio/vpp/$SHA/LICENSE"

# Integrity check against the table above:
shasum -a 256 third_party/vpp-linux-cp/LICENSE third_party/vpp-linux-cp/src/*
```

## Related

| Where | What |
|-------|------|
| `vendor/go.fd.io/govpp/binapi/lcp/lcp.ba.go` | The generated stub this source exists to correct. Generated from `src/lcp.api`. |
| `internal/plugins/iface/vpp/lcp.go` | Ze's caller: `SetupLCPPair` via `lcp_itf_pair_add_del`. |
| `internal/component/vpp/startupconf.go` | Emits the `linux-cp { default netns ... }` stanza parsed by `lcp_itf_pair_config` (`src/lcp_interface.c:575-616`), which calls `lcp_set_default_ns` (`src/lcp.c:50`). |
| `docs/guide/vpp.md` | The operator guide, corrected against this source. |
| `ai/rules/evidence.md` | The rule this directory serves. |
</content>
</invoke>
