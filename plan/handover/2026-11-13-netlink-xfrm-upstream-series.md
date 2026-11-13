# Handover: netlink XFRM upstream series (2026-11-13)

Spec: `plan/post-release/engineering/spec-netlink-xfrm-upstream-series.md`, status
`design`, untracked. This file records everything known and everything owed, so a
session that starts cold can continue. It was written from a session on Thomas's
Mac that works on this checkout over ssh.

## State of the tree this work depends on

| Thing | State on 2026-11-13 11:45 | Who holds it |
|-------|---------------------------|--------------|
| Forward-apply guard | Committed as `ce3a0e0fd7`: `TestPatchedVendorModulesMatchUpstream` now applies every patch forward | Done |
| `internal/le/vendorpatch/patches/netlink-xfrm-fixes.patch` | Committed in `352f231413`, six files, applies forward | Done |
| Vendored netlink files | Committed in `352f231413`. Clean in git. `route_probe_ipv4_linux.go` and its patch `netlink-route-probe-ipv4.patch` are untracked work of another piece and touch no XFRM file | Done |
| `internal/component/ike/dataplane/xfrm_linux.go` | Modified and uncommitted: the callers of the new library fields | The RFC 4301 work |
| Dual-form work | Landed as `abeb7d6535` on 2026-11-12; later rounds of that spec are uncommitted. Handover: `plan/handover/2026-11-12-dual-form-spd-bypass.md` | The dual-form spec |

The RFC 4301 commits `8b3c7a34cd` and `2db581e7d6` are in HEAD and touch no
netlink file. The last commit that touched the vendored library is `e1f42c2ca4`
(2026-11-10) before this work. The library part of RFC 4301 (the mapped IPv6
selector fix and the state scope) was committed on its own as `352f231413` on
2026-11-13, by the session that wrote this file, because its own session had
stopped. The Ze callers of it are not in that commit.

## The malformed patch (repaired and committed)

The generator `tmp/session/2026-11-10-225434/scratch/scope-wire-generate.py`
builds the diff with Python's `difflib` and writes each `diff --git` line itself.
For a file upstream lacks it writes `--- /dev/null` and no `new file mode 100644`
line, so git reads the three added files as a change to a file named `dev/null`.

| Check | Result |
|-------|--------|
| `git apply --summary` on the patch | Lists no file creation (the sibling `netlink-route-probe-ipv4.patch` lists one) |
| `git apply --reverse --check` in the checkout | Passes |
| Forward apply to pristine v1.3.1 | Refused, "error: dev/null: No such file or directory", three times |

Done on 2026-11-13 and committed in `352f231413`:

- The three `new file mode 100644` lines were inserted into the patch, and nothing
  else in it changed. `git apply --summary` now lists three creations, and
  `./le test unit pkg path ./internal/le/vendorpatch/` is green (a real run, 1.4s).
- The generator now emits that line when upstream has no such file, so a later
  regeneration keeps it.
- The patch and the generator as they were before are kept in
  `tmp/session/2026-11-13-112345/scratch/` as
  `netlink-xfrm-before-new-file-mode.patch` and
  `scope-wire-generate.before-new-file-mode.py`. They can be removed now.

In the working tree the ike dataplane package has one red,
`TestVPPInstallPolicyPartialPortMaskRefused`, in the uncommitted VPP policy work
of the RFC 4301 spec. It is not caused by the library.

## Progress and what is next

Landed in `b43120b371`: the single patch is a series of eleven files, the guard
judges a patch at its place in the stack, and every page names the series. The
spec's Design Insights section lists the series, its order and what each patch
needs. The spec is `in-progress`, claimed one above the WIP cap on the owner's
one-off exception.

| Next | State | Blocked by |
|------|-------|------------|
| Upstream-style tests, and the two selector gaps (AC-3, AC-7, AC-8) | Not started | Nothing |
| Each upstream-candidate patch checked against a clone of upstream `main` (AC-2) | Not started | Nothing; needs a clone outside the checkout |
| Private kernel ABI moved into `internal/component/ike/dataplane` (AC-5, AC-6) | Not started | The RFC 4301 session's uncommitted dataplane callers (`xfrm_linux.go`, `state_scope.go`, `xfrm_state_scope_linux.go`, `xfrm_forward_default_linux.go`): the move edits those files |
| Pull requests (AC-11) | Not started | The owner's order |

Also in the working tree, not this spec's: `rfc7296_vpp_message_test.go` carries
an approved repair of `TestVPPInstallPolicyPartialPortMaskRefused` and a new
`TestVPPInstallPolicyAnyProtocolPortRefused`. They pass only with the RFC 4301
session's uncommitted change to `vpp_policy.go`, so they land in that session's
commit. The judge's approval row is in `tmp/commit-weakening-approved-ea3b8173.md`.

Scratch kept in `tmp/session/2026-11-13-112345/scratch/`: `xfrm-series/` (the
scripts and stages that built the series) and `series-commit/`. Both can be
removed once the upstream-style tests phase has regenerated the patches.

## Design choices

Three were put to Thomas, and he said to continue on 2026-11-13 after the
recommendation to approve all three. The spec has not been moved to `ready`.

| Choice | Decision |
|--------|----------|
| Mapped IPv6 selector fix | Stays in the library and is proposed upstream |
| Private kernel ABI (`XFRMA_STATE_SCOPE`, `XFRMA_ZE_CAPS`, the scope and capability types) | Moves into `internal/component/ike/dataplane`; the library keeps a generic attribute hook |
| The single patch | Becomes an ordered series, one file per change |

## Findings, now in the spec

Written into the spec on 2026-11-13 as A-1, A-5, A-6, R-9, R-10 and two Design
Insights lines. The table stays here as the evidence trail.

| Spec item | Finding | Evidence |
|-----------|---------|----------|
| A-1 | Half false. Upstream `main` (at `e0e26cd`) has not fixed the selector family and sets no state flag but ESN, so no patch is redundant. It HAS been reshaped: `pkgHandle` became a `pkgHandle()` call, and `XFRMA_SA_PCPU` is no longer sent on get or delete. No release follows v1.3.1 | `diff` of the three files at v1.3.1 against `raw.githubusercontent.com/vishvananda/netlink/main` |
| A-5 | Confirmed. No Ze producer installs a selector with a source and no destination; an unrestricted side is always an explicit zero prefix | Trace of `childPairPolicy` and its fallback, `newRekeyedChild`, `inboundStateSelector`, `spdPolicyDirection`, the configured traffic selectors, `ikeBypassPolicies`, `spdCatchAllEntries`, `xfrmInterfaceExemptions`, `xfrmStateScopeFromParams`, OSPF's `buildIPsecSA` and `buildIPsecPolicies`, and the helpers `intersectNet`, `rangePrefixes`, `leaseHostIn`. Nothing at the library boundary refuses one, so the fix stays in the series |
| A-6 | Confirmed. The kernel refuses an `AF_INET` selector with a prefix length above 32 with `-EINVAL` and "Invalid prefix length in selector (must be <= 32 for IPv4)", on policy add (`verify_newpolicy_info`) and on state add (`verify_selector_prefixlen` from `verify_newsa_info`) | `net/xfrm/xfrm_user.c` in Linux 7.3, the version `internal/appliance/kernel.version` pins. Present in mainline from v4.19, absent in v4.18. Not read: the introducing commit and stable backports |
| R-9 | A patch against v1.3.1 equals its pull request against `main` byte for byte only where `main` left the surrounding lines alone. The spec's goal of byte-identical patches holds for some patches only. Each patch records whether it applies to `main` unchanged; where it does not, the pull request branch is the source and the Ze patch is the same change rebased onto v1.3.1 | The A-1 comparison |
| R-10 | Upstream issue 1213 of `vishvananda/netlink` ("xfrm: three bugs in state read-back, replay window and template family"), opened by Thomas on 2026-08-25 with three diffs against `main` at `156a440`, already proposes three fixes this patch carries: the ESN flag and replay window on read-back, the replay window cut to 32, and the template family. It is open with no maintainer answer. The patch's leading text and the spec do not mention it. The series starts from those three diffs and names the issue | The issue and its one comment |
| Bookkeeping | The owned journal row (`green-that-could-not-have-been-red.md`, 2026-11-13, the guard that only reverses) is fixed by `ce3a0e0fd7`, and AC-13 is already met. The spec no longer lists the row as owned, and Design Insights says AC-13 is met | The report of the session that made that commit |

## Still unvalidated in the spec

| Item | What settles it |
|------|-----------------|
| A-3: a generic extra-attributes hook is small enough to be the whole Ze-only patch | Write the hook and measure the remaining Ze-only diff |
| A-4: every upstream-candidate test runs unprivileged | Run each new test without privileges |
| A-7: the RFC 4301 library work lands first | Confirmed: `352f231413` |
| Which hunks survive unchanged on upstream `main` | Apply each patch of the drafted series to a clone of `main` |

## Working on this server over ssh

- A non-interactive ssh shell has no Go on PATH: export
  `PATH=/usr/local/go/bin:$HOME/go/bin:$PATH` and `GOPATH=$HOME/go` first.
- Other sessions are live in this checkout. Read their `tmp/session/` directory
  for recent writes before touching a file they hold.
- Nothing here orders a push. The server's `main` is ahead of `origin/main`, and
  pushing it is Thomas's order to give.
