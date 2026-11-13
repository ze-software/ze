# Handover: netlink XFRM upstream series (2026-11-13)

Spec: `plan/post-release/engineering/spec-netlink-xfrm-upstream-series.md`, status
`design`, untracked. This file records everything known and everything owed, so a
session that starts cold can continue. It was written from a session on Thomas's
Mac that works on this checkout over ssh.

## State of the tree this work depends on

| Thing | State on 2026-11-13 11:45 | Who holds it |
|-------|---------------------------|--------------|
| Forward-apply guard | Committed as `ce3a0e0fd7`: `TestPatchedVendorModulesMatchUpstream` now applies every patch forward | Done |
| `internal/le/vendorpatch/patches/netlink-xfrm-fixes.patch` | HEAD holds a sound two-file patch. The working tree holds a six-file patch that cannot be applied forward | Uncommitted work of `spec-rfc4301-architecture-gaps` |
| Vendored netlink files | Three modified (`xfrm_policy_linux.go`, `xfrm_state_linux.go`, `nl/xfrm_linux.go`) and three untracked (`xfrm_state_scope_linux.go`, its test, `xfrm_selector_family_linux_test.go`) | Same |
| `internal/component/ike/dataplane/xfrm_linux.go` | Modified, and it calls library functions that exist only in the uncommitted vendored files | Shared by the RFC 4301 and dual-form work |
| Dual-form work | Uncommitted. Its commit was refused on one malformed row in `plan/journal/error-names-no-fix-command.md`. Handover: `plan/handover/2026-11-12-dual-form-spd-bypass.md` | A session was still writing its state file at 11:40 |

The RFC 4301 commits `8b3c7a34cd` and `2db581e7d6` are in HEAD and touch no
netlink file. The last commit that touched the vendored library is `e1f42c2ca4`
(2026-11-10). The library part of RFC 4301 (the mapped IPv6 selector fix and the
state scope) is NOT committed, whatever a report says.

## The malformed patch

The generator `tmp/session/2026-11-10-225434/scratch/scope-wire-generate.py`
builds the diff with Python's `difflib` and writes each `diff --git` line itself.
For a file upstream lacks it writes `--- /dev/null` and no `new file mode 100644`
line, so git reads the three added files as a change to a file named `dev/null`.

| Check | Result |
|-------|--------|
| `git apply --summary` on the patch | Lists no file creation (the sibling `netlink-route-probe-ipv4.patch` lists one) |
| `git apply --reverse --check` in the checkout | Passes |
| Forward apply to pristine v1.3.1 | Refused, "error: dev/null: No such file or directory", three times |

Fix: the generator emits `new file mode 100644` between the `diff --git` line and
the `--- /dev/null` line when upstream has no such file, then the patch is
regenerated under a new snapshot name (the generator refuses to overwrite its
existing snapshot). Nothing else in the patch changes. Until then the
vendorpatch package is red in the working tree, which is the guard working.

## Order of work

1. Dual-form lands. It does not need the library work.
2. The RFC 4301 library work lands, with the patch regenerated so it applies
   forward. `xfrm_linux.go` carries hunks of both specs, so each commit goes by
   hunks where the other spec's hunks would not build at HEAD.
3. Only then does the split of this spec begin: its Depends row and risk R-7
   forbid starting on vendored files that carry another session's hunks.

## Design choices

Three were put to Thomas, and he said to continue on 2026-11-13 after the
recommendation to approve all three. The spec has not been moved to `ready`.

| Choice | Decision |
|--------|----------|
| Mapped IPv6 selector fix | Stays in the library and is proposed upstream |
| Private kernel ABI (`XFRMA_STATE_SCOPE`, `XFRMA_ZE_CAPS`, the scope and capability types) | Moves into `internal/component/ike/dataplane`; the library keeps a generic attribute hook |
| The single patch | Becomes an ordered series, one file per change |

## Findings not yet written into the spec

The spec edit was not made. A next session writes these into the spec's
Assumptions and Risks tables.

| Spec item | Finding | Evidence |
|-----------|---------|----------|
| A-1 | Half false. Upstream `main` (at `e0e26cd`) has not fixed the selector family and sets no state flag but ESN, so no patch is redundant. It HAS been reshaped: `pkgHandle` became a `pkgHandle()` call, and `XFRMA_SA_PCPU` is no longer sent on get or delete. No release follows v1.3.1 | `diff` of the three files at v1.3.1 against `raw.githubusercontent.com/vishvananda/netlink/main` |
| A-5 | Confirmed. No Ze producer installs a selector with a source and no destination; an unrestricted side is always an explicit zero prefix | Trace of `childPairPolicy` and its fallback, `newRekeyedChild`, `inboundStateSelector`, `spdPolicyDirection`, the configured traffic selectors, `ikeBypassPolicies`, `spdCatchAllEntries`, `xfrmInterfaceExemptions`, `xfrmStateScopeFromParams`, OSPF's `buildIPsecSA` and `buildIPsecPolicies`, and the helpers `intersectNet`, `rangePrefixes`, `leaseHostIn`. Nothing at the library boundary refuses one, so the fix stays in the series |
| A-6 | Confirmed. The kernel refuses an `AF_INET` selector with a prefix length above 32 with `-EINVAL` and "Invalid prefix length in selector (must be <= 32 for IPv4)", on policy add (`verify_newpolicy_info`) and on state add (`verify_selector_prefixlen` from `verify_newsa_info`) | `net/xfrm/xfrm_user.c` in Linux 7.3, the version `internal/appliance/kernel.version` pins. Present in mainline from v4.19, absent in v4.18. Not read: the introducing commit and stable backports |
| New risk | A patch against v1.3.1 equals its pull request against `main` byte for byte only where `main` left the surrounding lines alone. The spec's goal of byte-identical patches holds for some patches only. Each patch records whether it applies to `main` unchanged; where it does not, the pull request branch is the source and the Ze patch is the same change rebased onto v1.3.1 | The A-1 comparison |
| New risk | Upstream issue 1213 of `vishvananda/netlink` ("xfrm: three bugs in state read-back, replay window and template family"), opened by Thomas on 2026-08-25 with three diffs against `main` at `156a440`, already proposes three fixes this patch carries: the ESN flag and replay window on read-back, the replay window cut to 32, and the template family. It is open with no maintainer answer. The patch's leading text and the spec do not mention it. The series starts from those three diffs and names the issue | The issue and its one comment |
| Bookkeeping | The owned journal row (`green-that-could-not-have-been-red.md`, 2026-11-13, the guard that only reverses) is fixed by `ce3a0e0fd7`, and AC-13 is already met. The spec still lists both as open | The report of the session that made that commit |

## Still unvalidated in the spec

| Item | What settles it |
|------|-----------------|
| A-3: a generic extra-attributes hook is small enough to be the whole Ze-only patch | Write the hook and measure the remaining Ze-only diff |
| A-4: every upstream-candidate test runs unprivileged | Run each new test without privileges |
| A-7: the RFC 4301 library work lands first | `git status` shows the patch and the vendored netlink files clean |
| Which hunks survive unchanged on upstream `main` | Apply each patch of the drafted series to a clone of `main` |

## Working on this server over ssh

- A non-interactive ssh shell has no Go on PATH: export
  `PATH=/usr/local/go/bin:$HOME/go/bin:$PATH` and `GOPATH=$HOME/go` first.
- Other sessions are live in this checkout. Read their `tmp/session/` directory
  for recent writes before touching a file they hold.
- Nothing here orders a push. The server's `main` is ahead of `origin/main`, and
  pushing it is Thomas's order to give.
