# Spec: netlink-xfrm-upstream-series

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Priority | P3 |
| Release reason | The XFRM corrections already work through the single vendored patch; reshaping them so upstream can take them changes no shipped behavior, so it does not block release correctness or proof. |
| RFC gaps | - |
| Depends | plan/release/fixes/spec-rfc4301-architecture-gaps.md (its uncommitted netlink work, the mapped IPv6 selector fix and the state scope, must be committed first) |
| Phase | - |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-13 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Ze changes the vendored `github.com/vishvananda/netlink` v1.3.1 through one diff,
`internal/le/vendorpatch/patches/netlink-xfrm-fixes.patch`. Its leading text says
"No upstream pull request is opened yet for these XFRM fixes." No pull request can be
opened from it as it stands, for three reasons.

| Problem | Symptom |
|---------|---------|
| One diff mixes three kinds of change | Upstream bug fixes, upstream API additions and Ze's private kernel ABI share hunks in the same files, so no subset can be proposed on its own |
| The private ABI lives in the library | `XFRMA_STATE_SCOPE`, `XFRMA_ZE_CAPS`, `XfrmStateScope` and `XfrmZECapabilities` describe a kernel only Ze runs. Upstream can never take them, so the patch can never be deleted |
| The tests depend on Ze | `TestZeXfrmSelectorUsesPrefixFamily` takes its expected bytes through the private state-scope encoder, and the Ze files carry `// Design:` headers that point at Ze pages |

The goal is a patch series in which every change upstream could accept is a
standalone patch, byte-identical to the pull request that proposes it, with a test
upstream can run. What only Ze needs moves into Ze, and the library keeps the
smallest hook that makes it possible. The owner's preference, stated on 2026-11-13,
frames the split: a problem Ze can fix in its own code is fixed there, and a library
diff is kept for what Ze cannot do otherwise.

Two gaps found while reviewing the selector family fix belong to this work, because
an upstream reviewer will ask about both.

| Gap | What happens today |
|-----|--------------------|
| Source-only selector | The selector writer decides the family from the destination alone. A selector with an IPv4-mapped IPv6 source and no destination is still written as IPv4 with a prefix length above 32 |
| IPv4 address in sixteen bytes with a 128-bit mask | The fix reads it as an IPv6 prefix, which is what the caller wrote. Before the fix it was written as IPv4 by accident. The change is correct and undocumented |

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/patching-the-standard-library.md`, "Patching a vendored module" - the rules every vendor patch obeys
  → Constraint: a patch's leading text names the upstream pull request that carries the change, or says none is opened yet, and quotes the requirement it meets
  → Constraint: one patch changes one module, and carries `Ze-Reviewed-Against: <module path> <version>`
  → Constraint: the set of patches is the files present in `internal/le/vendorpatch/patches/`; adding a patch needs no edit elsewhere
  → Decision: a GitHub fork under ze-software exists only to open the pull request and is never a dependency
  → Constraint: when the upstream release containing a change is required by `go.mod`, the patch is deleted
- [ ] `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md`, "Vendored netlink patch" - the page that lists each correction and names the single patch file
  → Constraint: this section lists every correction against one file name, so it is rewritten in the phase that splits the file
  → Constraint: the section tells a reader to run `git apply` on the single file after a vendor refresh; with a series that instruction is wrong and `./le setup install` is the recovery
- [ ] `ai/doors/access-security.md` - the IKE door
  → Constraint: `ike/dataplane` stays ungated, because OSPF uses it as its XFRM seam for RFC 4552; nothing moved into it may import a gated package
- [ ] `ai/doors/le-tooling.md` - the tooling door
  → Constraint: a shipped `ze` never links an `internal/le` package, so nothing the dataplane needs may live in `internal/le/vendorpatch`
- [ ] `ai/META-DECISIONS.md`, "Code and dependencies" - the owner's rules on third-party modules
  → Decision: a change Ze needs in a third-party Go module is a diff against the upstream release, applied at build time
  → Constraint: `vishvananda/netlink` may be used in full; no approval is owed for using more of its exported API

### RFC Summaries (Scope: protocol)
Not applicable: Scope is tooling. The RFC sentences each correction meets are
already quoted in the existing patch's leading text and travel with their hunks.

**Key insights:** (minimal context to resume after compaction)
- Patches apply in sorted file-name order and are reverted in reverse order by the upstream check, so a series needs names that sort in dependency order
- The dataplane already builds raw XFRM requests for messages the library lacks, so Ze-side encoding of a private attribute follows an existing pattern
- The library carries no general way to add an attribute to a state request, which is the one thing the private ABI needs from it
- Opening a pull request is outward-facing and happens only on the owner's order

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/le/vendorpatch/patches/netlink-xfrm-fixes.patch` - one diff over six targets: `xfrm_policy_linux.go`, `xfrm_state_linux.go`, `nl/xfrm_linux.go`, and three new files (the state scope, its test, the selector family test). Modified and uncommitted by the session that owns `spec-rfc4301-architecture-gaps`
- [ ] `internal/le/vendorpatch/stamp.go` - `Patches` answers every patch file sorted by name; `Stamp` and `CheckStamp` read and check the review stamp
- [ ] `internal/le/vendorpatch/vendor_patch_test.go` - `assertVendorPatchApplied` is the one producer every per-patch guard calls
- [ ] `internal/le/vendorpatch/netlink_xfrm_patch_test.go` - `TestNetlinkXFRMPatchApplied` names the single patch path and a `git apply` recovery command by hand
- [ ] `internal/le/vendorpatch/upstream_test.go` - `TestPatchedVendorModulesMatchUpstream` groups patches by the module their stamp names and reverts them in reverse order
- [ ] `vendor/github.com/vishvananda/netlink/xfrm_policy_linux.go` - `selFromPolicy` writes a selector; the family now comes from the destination mask width, and the source follows the destination's family
- [ ] `vendor/github.com/vishvananda/netlink/xfrm_state_linux.go` - `xfrmStateAddOrUpdate` appends the scope attribute when `StateScope` is set; `parseXfrmState` decodes it
- [ ] `vendor/github.com/vishvananda/netlink/xfrm_state_scope_linux.go` - about 250 lines: the scope type, its validation, writer and parser, and the capability receipt type, parser and getter
- [ ] `vendor/github.com/vishvananda/netlink/xfrm_selector_family_linux_test.go` - five cases; the expected bytes are written by hand and serialized through the private scope writer
- [ ] `internal/component/ike/dataplane/xfrm_forward_default_linux.go` - builds `XFRM_MSG_GETDEFAULT` and `XFRM_MSG_SETDEFAULT` requests itself through the library's exported request builder
- [ ] `internal/component/ike/dataplane/xfrm_migrate_linux.go` - builds `XFRM_MSG_MIGRATE_STATE` requests itself the same way
- [ ] `internal/component/ike/dataplane/state_scope.go` and `xfrm_state_scope_linux.go` - Ze's own canonical scope and its conversion to and from the library type
- [ ] `internal/component/ike/dataplane/xfrm_linux.go` - the only product code that installs states and policies; reads capabilities through `netlink.XfrmZECapabilitiesGet`

**Behavior to preserve:** (unless the user explicitly said to change it)
- Every byte Ze sends to the kernel in a state or policy request, for every input the existing tests cover
- Every value Ze reads back from a state or policy dump, the state scope included
- The review stamp, the upstream check and the re-vendor step, unchanged in what they refuse
- The refusal of a state scope when the running kernel does not report the capability

**Behavior to change:** (only what the user asked for)
- The single patch becomes an ordered series, one file per change
- The private kernel ABI's types, validation, encoding and capability read move from the library into `internal/component/ike/dataplane`
- A source-only selector with an IPv4-mapped IPv6 source is written as IPv6
- The selector family test, and every other test in an upstream-candidate patch, no longer depends on Ze

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Vendor patch set | `internal/le/vendorpatch/stamp.go`, `Patches` | The files present in the patch directory are the set, applied in sorted order | Extends: the netlink patch becomes several files in that directory. No change to the mechanism |
| Review stamp | `internal/le/vendorpatch/stamp.go`, `Stamp`, `CheckStamp` | Refuses a patch whose stamp is missing or names another version | Extends: every patch of the series carries the stamp |
| Upstream check | `internal/le/vendorpatch/upstream_test.go`, `TestPatchedVendorModulesMatchUpstream` | Reverts every patch of a module and requires the result to equal upstream | Extends: it already groups by module, so the series is judged with no edit. It is the proof that the split lost and added nothing |
| Per-patch guard | `internal/le/vendorpatch/vendor_patch_test.go`, `assertVendorPatchApplied`, called by three hand-named tests | Checks one named patch is applied | Replaces the three hand-named callers with one test that walks `Patches`: a guard per file written by hand is a second list of the patch set, and the series would add several more rows to it |
| Re-vendor step | `internal/le/setup/install.go`, `applyVendorPatches` | Applies every patch to a staged vendor tree before publishing it | Does not change it. It becomes the recovery command the guard names, replacing the hand-written `git apply` line |
| Raw XFRM requests built by Ze | `internal/component/ike/dataplane/xfrm_forward_default_linux.go` and `xfrm_migrate_linux.go` | Ze encodes the XFRM messages the library does not cover, through the library's exported request builder | Extends: the capability receipt read joins the default-policy read it already sits beside, and the scope attribute is encoded by Ze |
| Ze's canonical state scope | `internal/component/ike/dataplane/state_scope.go`, `SAStateScope` | Owns and validates the negotiated selector set | Extends: it gains the wire encoding and parsing the library type holds today, so the scope has one owner |
| Upstream pull request record | `internal/le/vendorpatch/patches/goyang-enum-negative.patch`, leading text | Names the pull request and the condition for deleting the patch | Extends: each upstream-candidate patch of the series carries the same two statements |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Build side: `./le setup install` re-vendors the modules and applies every patch in sorted order
- Run side: the IKE engine asks the dataplane to install a Child SA; the dataplane fills the library's state and policy values and calls the library

### Transformation Path
1. The patch directory is listed and each stamp is checked against the vendored version
2. Each patch is applied to the staged vendor tree, upstream candidates first, the Ze-only patch last
3. At run time the dataplane converts its canonical scope into attribute bytes itself
4. The dataplane hands the library a state value carrying those bytes as an opaque extra attribute
5. The library writes the state request and appends the extra attribute unchanged
6. On a dump, the library returns the attributes it does not know, and the dataplane parses the scope from them

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Ze ↔ vendored library | The library's exported state and policy values, plus one opaque attribute hook | No |
| Library ↔ kernel | XFRM netlink messages | No |
| Ze ↔ upstream project | One pull request per upstream-candidate patch, opened on the owner's order | No |

### Changed-Wire Census

**Search evidence:** searched the whole checkout, excluding `.git`, `tmp` and `bin`
and including `vendor/` and the kernel patches, for `XFRMA_STATE_SCOPE`,
`XFRMA_ZE_CAPS`, `StateScope` and `XfrmZECapabilities`, in every language. Positive
control: the search finds the known kernel reader,
`gokrazy/kernel/patches/0003-xfrm-state-scope.patch`, and the known Go writer,
`xfrmStateAddOrUpdate`. No shell, Python or embedded script names any of the four
terms. The wire's bytes do not change: the census records who writes and reads
them, because the writer and reader move from the library into Ze.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| State scope attribute | writer | `vendor/github.com/vishvananda/netlink/xfrm_state_scope_linux.go`, `WriteTo`, called from `xfrmStateAddOrUpdate` | Found by the `XFRMA_STATE_SCOPE` search | Moves to `internal/component/ike/dataplane`; bytes preserved, proven by the existing wire test over the moved encoder |
| State scope attribute | reader | `vendor/github.com/vishvananda/netlink/xfrm_state_scope_linux.go`, `ParseXfrmStateScope`, called from `parseXfrmState` | Found by the same search | Moves to the dataplane; the library returns the raw attribute |
| State scope attribute | reader | `gokrazy/kernel/patches/0003-xfrm-state-scope.patch`, `xfrm_state_scope_from_user` | Found by the same search; named by the design page | Preserved: the kernel side is untouched |
| State scope attribute | writer | `gokrazy/kernel/patches/0003-xfrm-state-scope.patch`, the state dump | Named by the design page beside the reader | Preserved |
| Capability receipt | writer | `gokrazy/kernel/patches/0003-xfrm-state-scope.patch`, `xfrm_get_default` | Found by the `XFRMA_ZE_CAPS` search | Preserved |
| Capability receipt | reader | `vendor/github.com/vishvananda/netlink/xfrm_state_scope_linux.go`, `XfrmZECapabilitiesGet`, `ParseXfrmZECapabilities` | Found by the same search; called from `internal/component/ike/dataplane/xfrm_linux.go` | Moves to the dataplane, beside the default-policy read |
| Selector in a state or policy request | writer | `vendor/github.com/vishvananda/netlink/xfrm_policy_linux.go`, `selFromPolicy` | Read directly | Changes for one input only: a source-only selector with a mapped IPv6 source. Every other input preserved |
| Selector in a state or policy request | reader | The Linux kernel's XFRM user interface | Outside the repository | Preserved: the kernel reads the standard selector layout |

### Integration Points
- `vendorpatch.Patches` - the series is discovered through it, with no registration
- `assertVendorPatchApplied` - the derived guard calls it once per patch
- The library's exported request builder - the dataplane's capability read uses it, as the default-policy read does

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Upstream's main branch has not already fixed or reshaped the code these patches touch | Compared 2026-11-13: the three patched files at v1.3.1 against upstream `main` at `e0e26cd`. Not fixed: `selFromPolicy` on `main` still takes the family from the destination alone, and `main` sets no state flag but ESN. Reshaped: `main` replaced the package handle `pkgHandle` with a `pkgHandle()` call, and no longer sends `XFRMA_SA_PCPU` on a get or delete request. No release follows v1.3.1 | No patch is redundant. A patch against v1.3.1 equals its pull request against `main` byte for byte only where `main` left the surrounding lines alone (R-9) | `diff` of each file at v1.3.1 against `raw.githubusercontent.com/vishvananda/netlink/main` | half false: not fixed, reshaped |
| A-2 | The library's exported request builder lets the dataplane read the capability receipt without any library change | `readXFRMDefaults` in `xfrm_forward_default_linux.go` issues the same request with the same three-byte body as the library's `XfrmZECapabilitiesGet`, and the receipt is an attribute after those three bytes in the same answer | The capability read stays in the Ze-only patch | Read both functions side by side: done 2026-11-13, the requests are identical | confirmed |
| A-3 | A generic "extra attributes" hook on a state request and its readback is small enough to be the whole Ze-only patch | `xfrmStateAddOrUpdate` appends attributes one after another; `parseXfrmState` already loops over them | The Ze-only patch keeps more of the private ABI | Write the hook and measure the remaining Ze-only diff | confirmed 2026-11-13: 151 lines, see Design Insights |
| A-4 | Every upstream-candidate test can run unprivileged, as a pure encode or decode check | The selector family test already runs that way | A test needs a network namespace, which upstream's suite runs as root and Ze agents do not | Run each new test without privileges | unvalidated |
| A-5 | Ze never installs a selector with a source and no destination | Traced 2026-11-13 over every non-test producer of an `XfrmPolicy` or `XfrmState` selector: `childPairPolicy` and its fallback (`newChildSA` fills an absent side with `ipToFullNet`), `newRekeyedChild`, `inboundStateSelector`, `spdPolicyDirection` (`SPDPolicy.validate` refuses an absent prefix), the configured traffic selectors (`traffic_selector.go` refuses either side absent), `ikeBypassPolicies`, `spdCatchAllEntries`, `xfrmInterfaceExemptions`, `xfrmStateScopeFromParams`, and OSPF's `buildIPsecSA` and `buildIPsecPolicies`. The narrowing helpers `intersectNet`, `rangePrefixes` and `leaseHostIn` answer ok only with a prefix. An unrestricted side is always an explicit zero prefix | The source-only gap is a live Ze defect and moves to a release fix | The trace named in Basis. The producers hold it; nothing at the library boundary refuses a source-only selector, so a new caller can still produce one | confirmed |
| A-6 | The kernel refuses an IPv4 selector whose prefix length exceeds 32, so the upstream bug is a refused install and not a wrong match | Read 2026-11-13 in Linux 7.3 (`internal/appliance/kernel.version`), `net/xfrm/xfrm_user.c`: `verify_newpolicy_info` for a policy and `verify_selector_prefixlen`, called by `verify_newsa_info`, for a state both answer `-EINVAL` with "Invalid prefix length in selector (must be <= 32 for IPv4)" when either prefix length is above 32 under `AF_INET`. Both checks are present in mainline from v4.19 and absent in v4.18 | The pull request text would misstate the bug's effect | The two functions named in Basis. Not read: the commit that introduced the checks, and stable backports | confirmed |
| A-7 | The uncommitted library work of `spec-rfc4301-architecture-gaps` lands before this spec starts | That session's state file lists the green run and patch regeneration as owed | The split would be made under another session's edits | Check the patch and the vendored files are clean in git before the first edit | confirmed 2026-11-13: commit `352f231413` carries the library work and its patch, and both are clean in git |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The split changes a vendored byte | `TestPatchedVendorModulesMatchUpstream` or a product test goes red | The vendored tree after the split must be byte-identical to the tree before it, for every file the move does not touch; compare before continuing |
| R-2 | Patches of the series depend on each other's context lines, so upstream cannot take one alone | A patch does not apply to upstream's main branch without its predecessors | Order the series so each upstream candidate applies to upstream alone where the code allows, and record in its leading text which earlier patch it needs where it does not |
| R-3 | Upstream declines an API addition | A pull request is closed unmerged | The patch stays as a Ze patch with the refusal recorded in its leading text; nothing in Ze depends on the merge |
| R-4 | Upstream changes the shape of a change during review | Review comments ask for another name or signature | The Ze patch is regenerated from the reviewed branch, and Ze's callers follow in the same commit |
| R-5 | Moving the scope out of the library breaks the readback a product test relies on | A state scope readback test goes red | The moved parser is proven on the same fixtures before the library type is deleted |
| R-6 | A careless upstream caller relied on a sixteen-byte IPv4 address with a 128-bit mask being sent as IPv4 | An upstream reviewer raises it | The pull request text states the change and its reason, and a test pins the new reading |
| R-8 | The patch this spec starts from is malformed: on 2026-11-13 the regenerated single patch added three files without a creation header, and a forward apply to pristine upstream v1.3.1 was refused with "error: dev/null: No such file or directory" while the reverse check passed | The forward apply in phase 1's guard goes red on the starting patch | The owning session regenerates it with the creation header before this spec starts; the split never begins from a patch that does not apply forward. Done in the working tree on 2026-11-13: the patch carries the three `new file mode 100644` lines and the vendorpatch package is green. Committed in `352f231413` |
| R-7 | The series is edited while another session holds uncommitted hunks in the same vendored files | `git status` shows the files modified before the first edit | Wait for that session's commit; this spec depends on it |
| R-9 | Upstream `main` has moved since v1.3.1 (A-1) and Ze vendors v1.3.1, so one change can need two forms: the patch Ze applies and the pull request upstream reviews. The goal of a patch byte-identical to its pull request then holds only for some patches | A patch of the series does not apply to upstream `main` | Each upstream-candidate patch records in its leading text whether it applies to `main` unchanged; where it does not, the pull request branch is the source and the Ze patch is the same change rebased onto v1.3.1, compared hunk by hunk |
| R-10 | Upstream issue 1213 ("xfrm: three bugs in state read-back, replay window and template family"), opened 2026-08-25 with its three diffs inlined against `main` at `156a440`, already proposes three of the fixes this patch carries, and has had no maintainer answer. The patch's leading text names no upstream issue, and this spec was written without it | A series patch duplicates or contradicts a diff already posted upstream | The series starts from the three diffs of issue 1213, each patch's leading text names the issue, and a new pull request refers to it. The silence is evidence the patches are carried for a long time, so each one stays small and independent |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | IPsec states or policies installed with wrong bytes, or a state scope that no longer reaches the kernel. Site-to-site tunnels and OSPF over IPsec use this path |
| How is it reverted? | A single commit revert per phase: the vendored tree and the patch files move together |
| Who else touches this path? | `spec-rfc4301-architecture-gaps` (uncommitted work in the same vendored files), `spec-xfrm-outer-df-set-dscp` and `spec-ipsec-opaque-selector-port-mask` (both name the single patch file) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A patch file present in the patch directory | → | `assertVendorPatchApplied` over every file `Patches` answers | `TestEveryVendorPatchIsApplied` |
| The vendored netlink tree | → | Every patch of the series reverted in reverse order | `TestPatchedVendorModulesMatchUpstream` |
| A Child SA installed with a negotiated scope | → | The dataplane's scope encoder and the library's extra attribute hook | `TestXFRMStateScopeWirePreservesOrderedSelectors` |
| A state dump from Ze's kernel | → | The dataplane's scope parser over the attribute the library returns | The state scope readback integration test in `internal/component/ike/engine/rfc4301_state_scope_readback_integration_linux_test.go` |
| A selector installed through the library | → | `selFromPolicy` | `TestXFRMSinglePortSelectorReachesTheKernel` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | The patch directory after the split | Each change upstream could take is one patch file, and the Ze-only change is one further file that sorts last | The other modules' patches are untouched: they are not part of this work |
| AC-2 | Each upstream-candidate patch, with its path prefix removed | It applies as written to a checkout of upstream, alone or after the earlier patch its leading text names | A patch may name one predecessor where both change the same function: the code leaves no other order |
| AC-3 | Each upstream-candidate patch | Its code carries a test upstream can run unprivileged, with expected values written by hand and no identifier, header or dependency that belongs to Ze | Ze names remain in the Ze-only patch and in the leading text of every patch: neither goes upstream |
| AC-4 | The vendored tree after the split, with nothing moved yet | It equals the vendored tree before the split, byte for byte | No difference is permitted: the split is a regrouping |
| AC-5 | The private kernel ABI after the move | Its types, validation, encoding, parsing and capability read live in `internal/component/ike/dataplane`, and the dataplane sends the same attribute bytes as before | The attribute numbers stay declared where the dataplane reads them. The library keeps only a hook that names no Ze concept |
| AC-6 | The Ze-only patch after the move | It holds only the generic extra-attribute hook on a state request and its readback | The hook remains because the library builds the state request and exposes no other way to add an attribute |
| AC-7 | A selector with an IPv4-mapped IPv6 source, a 128-bit mask and no destination | The library writes it with the IPv6 family and all sixteen address bytes | A source-only selector with a 32-bit mask stays IPv4: the mask says so |
| AC-8 | A selector whose address is IPv4 held in sixteen bytes, with a 128-bit mask | The library writes it as an IPv6 prefix, and a test and the pull request text both say so | The same address with a 32-bit mask stays IPv4, as the existing test case proves |
| AC-9 | A patch file added to the patch directory and then reverted in the vendored tree | The one derived guard reports that file by name as not applied, and reports every other patch as applied, with no test edited for the new file | No hand-named per-patch guard remains: each was a second list of the set |
| AC-13 | A patch whose added file lacks its creation header, so it reverses cleanly and cannot be applied to upstream | The guard refuses it and names the file, because it applies every patch forward to the upstream module as well as reversing it | A patch that both applies forward and reverses passes: that is the only shape the re-vendor step can use |
| AC-10 | Each upstream-candidate patch's leading text | It names its pull request, or says none is opened yet, quotes the requirement it meets, and states the condition under which the patch is deleted | The Ze-only patch says it is never proposed upstream, and why |
| AC-11 | The owner orders the pull requests opened | Each is opened from a branch of a ze-software fork that carries exactly its patch, and the patch's leading text is updated with its address | No pull request is opened without that order: it is outward-facing |
| AC-12 | Every page and comment that named the single patch file | It names the series, or the patch that carries the change it describes | Closed specs and historical records keep the old name: they describe the tree of their day |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEveryVendorPatchIsApplied` | `internal/le/vendorpatch/vendor_patch_test.go` | Every patch file present is applied, found from the directory | |
| `TestPatchedVendorModulesMatchUpstream` | `internal/le/vendorpatch/upstream_test.go` | The series reverts to upstream exactly (existing test, unchanged) | |
| `TestXfrmSelectorFamilyFromMask` | `vendor/github.com/vishvananda/netlink/xfrm_selector_family_linux_test.go` | The selector family follows the mask width, for the five existing cases, with expected bytes serialized directly | |
| `TestXfrmSelectorFamilySourceOnly` | same file | A source-only mapped IPv6 selector is written as IPv6 | |
| `TestXfrmSelectorFamilySixteenByteIPv4` | same file | An IPv4 address in sixteen bytes follows its mask: 32 bits gives IPv4, 128 bits gives IPv6 | |
| One upstream-style test per remaining candidate patch | beside the code each patch changes, in `vendor/github.com/vishvananda/netlink/` | The template family, the readback by message family, the replay window, the truncated message refusal, the port masks and the flags, each proven without Ze | |
| `TestXfrmStateExtraAttributes` | `vendor/github.com/vishvananda/netlink/`, in the Ze-only patch | An extra attribute is appended unchanged, and an unknown attribute is returned on readback | |
| `TestXFRMStateScopeWirePreservesOrderedSelectors` | `internal/component/ike/dataplane/kernel_scope_wire_linux_test.go` | The moved encoder produces the bytes the library encoder produced (existing test, re-pointed) | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Selector prefix length, IPv4 family | 0-32 | 32 | N/A | 33 |
| Selector prefix length, IPv6 family | 0-128 | 128 | N/A | 129 |
| State scope selector count | 1-64 | 64 | 0 | 65 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-sa-installed` | `test/ipsec/ipsec-sa-installed.ci` | An operator's tunnel comes up and its SA is installed, with the series applied and the scope moved | |
| `ipsec-icmp-type-selector` | `test/ipsec/ipsec-icmp-type-selector.ci` | A selector with an explicit port mask still reaches the kernel through the port-mask patch | |
| `ipsec-sa-options-kernel` | `test/ipsec/ipsec-sa-options-kernel.ci` | The state flags the flag patches add still reach the kernel | |

No new functional test is owed: the work changes no behavior an operator reaches.
These existing files, run through `./le test functional ipsec`, are the proof
through the daemon that the states and policies still install.

## Files to Modify
- `internal/le/vendorpatch/patches/netlink-xfrm-fixes.patch` - removed, replaced by the series
- `internal/le/vendorpatch/vendor_patch_test.go` - gains the derived guard
- `internal/le/vendorpatch/netlink_xfrm_patch_test.go`, `bubbles_cursor_patch_test.go`, `gokrazy_packer_patch_test.go` - the hand-named guards are removed
- `vendor/github.com/vishvananda/netlink/xfrm_policy_linux.go` - the source-only family case
- `vendor/github.com/vishvananda/netlink/xfrm_state_linux.go` - the scope field and its handling give way to the generic hook
- `vendor/github.com/vishvananda/netlink/nl/xfrm_linux.go` - the private attribute numbers leave
- `vendor/github.com/vishvananda/netlink/xfrm_state_scope_linux.go` and its test - removed, their content moves to the dataplane
- `vendor/github.com/vishvananda/netlink/xfrm_selector_family_linux_test.go` - rewritten without Ze
- `internal/component/ike/dataplane/state_scope.go`, `xfrm_state_scope_linux.go`, `xfrm_linux.go` - gain the scope wire encoding, parsing and the capability read
- `internal/component/ike/dataplane/xfrm_forward_default_linux.go` - the capability receipt is read from the answer it already requests
- `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` - "Vendored netlink patch" names the series and the new owner of the scope
- `docs/contributing/patching-the-standard-library.md` - the guard test row and the series naming rule
- `docs/architecture/linux-limitations.md`, `ai/INDEX.md` - the patch file name

## Files to Create
- `internal/le/vendorpatch/patches/netlink-xfrm-<order>-<change>.patch` - one per upstream-candidate change, and one last for the Ze-only hook
- Upstream-style test files in `vendor/github.com/vishvananda/netlink/`, one per candidate patch that has none

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config or RPC changes |
| YANG validation constraints | N-A | No leaf changes |
| YANG custom validators | N-A | No leaf changes |
| CLI commands/flags | N-A | No command changes |
| CLI grammar (keyword before value) | N-A | No command changes |
| Editor autocomplete | N-A | No leaf changes |
| Functional test for new RPC/API | N-A | No RPC or API is added |
| Pipe completeness | N-A | No command output changes |
| Env var registration | N-A | No env var changes |
| Doctor check for runtime dependencies | N-A | No new runtime dependency: the kernel capability the scope needs is already checked before a scope is installed, and that check moves with its code |
| Doctor code per problem class | N-A | No new refusal or log line: the existing capability refusal keeps its text |
| Prometheus counters/metrics | N-A | No observable state changes |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | No operator-visible behavior changes |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | The kernel attribute bytes are preserved; only their Go owner moves |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | Yes | `docs/contributing/patching-the-standard-library.md`: one derived guard replaces the per-patch guards |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md`: the scope's wire encoding is owned by the dataplane, and the library patch is a series |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived at implementation with `./le spec citation anchors`; the design page's anchors on `xfrm_state_scope_linux.go` in `vendor/` move to the dataplane file |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | The `git apply` command in the design page's patch section is replaced by `./le setup install` |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- one derived guard over the patch directory
   - Tests: `TestEveryVendorPatchIsApplied`
   - Files: `internal/le/vendorpatch/vendor_patch_test.go`, the three hand-named guard files
   - Verify: the derived guard is red when a patch is reverted in a scratch copy, then the hand-named guards are removed
2. **Phase: Upstream state** -- settle A-1 and A-6 before anything is split
   - Tests: none; this phase reads
   - Files: none in Ze. A clone of upstream under the owner's source directory, and Ze's kernel tree
   - Verify: a table, in this spec, of each change against upstream's main branch: absent, already fixed, or reshaped
3. **Phase: Split** -- the single patch becomes the series, with no vendored byte changed
   - Tests: `TestPatchedVendorModulesMatchUpstream`, `TestEveryVendorPatchIsApplied`
   - Files: the patch directory
   - Verify: the vendored tree is byte-identical before and after (AC-4)
4. **Phase: Upstream-style tests** -- each candidate patch carries a test without Ze, and the two selector gaps are closed
   - Tests: the selector family tests and one per remaining candidate
   - Files: `vendor/github.com/vishvananda/netlink/`, each patch regenerated
   - Verify: each new test is red against the code without its patch and green with it
5. **Phase: Private ABI into Ze** -- the scope and the capability read move to the dataplane; the Ze-only patch shrinks to the hook
   - Tests: `TestXFRMStateScopeWirePreservesOrderedSelectors`, `TestXfrmStateExtraAttributes`, the scope readback integration test, the capability tests in `kernel_scope_caps_linux_test.go`
   - Files: the dataplane files and the vendored state files
   - Verify: the moved encoder is proven on the existing fixtures first, then the library type is deleted (no period with both)
6. **Phase: Pages** -- every page that names the single patch
   - Tests: `./le doc check all`
   - Files: the four pages in Files to Modify
   - Verify: no page or comment outside closed records names the removed file
7. **Phase: Pull requests** -- on the owner's order only
   - Tests: upstream's own test run on each branch
   - Files: each patch's leading text gains its pull request address
   - Verify: each branch's diff equals its patch

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | The vendored tree after the split equals the tree before it; the attribute bytes after the move equal the bytes before it |
| Naming | No identifier, comment or header in an upstream-candidate patch names Ze; patch file names sort in the order they must apply |
| Data flow | The library names no Ze concept after phase 5; the dataplane is the only owner of the scope's wire form |
| Rule: no layering | The library scope type is deleted in the same phase the dataplane encoder replaces it |
| Rule: registration | No test, page or tool lists the netlink patches by name where the directory answers |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The series replaces the single patch | List the patch directory; the single file is absent |
| The series reverts to upstream | `TestPatchedVendorModulesMatchUpstream` green |
| Each candidate applies to upstream | Apply each, with its prefix removed, in the upstream clone |
| No Ze name in a candidate patch's code | Search the added lines of each candidate patch for Ze's name and for `Design:` |
| The private ABI is out of the library | Search `vendor/github.com/vishvananda/netlink/` for the four census terms; no hit |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The moved scope parser still refuses a truncated, duplicate or malformed attribute from a dump, as the library parser did |
| Fail-open | The capability refusal still runs before a scope is installed: a kernel that ignores the attribute must not be treated as enforcing it |
| Hook misuse | The extra-attribute hook cannot replace an attribute the library itself writes |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The move of the private ABI landed in `8afd1ff560` (2026-11-13): AC-5 and AC-6 are met. Every code phase of this spec is done. Open: the pull requests, on the owner's order (AC-11), and closure. The integration tests that install and read back a scope on Ze's kernel were not run for the move; closure owes them (`rfc4301_state_scope_integration_linux_test.go`, `rfc4301_state_scope_readback_integration_linux_test.go`, `rfc4301_forward_default_integration_linux_test.go` in the engine, `xfrm_readback_integration_linux_test.go` in the dataplane).
- A-3 is confirmed: the Ze-only patch `netlink-xfrm-90-state-extra-attributes.patch` is 151 lines, of which about 30 are leading text, 55 its test and 27 the hook in `xfrm_state_linux.go` (a field, the `XfrmAttr` type, `addXfrmExtraAttrs`, one call and a `default:` arm). The patch it replaces was 396 lines. Whether the hook applies to upstream `main` was not checked.
- The capability read does not share the forward-default read's answer: both build the same request through `xfrmGetDefaultAnswer`, at different times (the forward default at policy setup, the capability before each scoped state is installed).
- The library returns on read-back only the attributes above its own `XFRMA_MAX` (35 at v1.3.1). A compile-time assertion in `xfrm_state_scope_abi_linux.go` fails the build if a re-vendor raises it to the scope's number, 42. The upstream IPTFS attributes 36 to 41 also arrive in `ExtraAttrs`; the dataplane reads only type 42 and keeps none of them.
- The upstream-style tests phase landed in `c9ce6ba81d` (2026-11-13): AC-2, AC-3, AC-7 and AC-8 are met. Each of patches 01 to 10 adds its own test file, and breaking one line of each patch in turn turned that patch's test red. Open: the move of the private ABI into the dataplane (AC-5, AC-6) and the pull requests on the owner's order (AC-11).
- A-4 holds with three small extractions. Patches 02, 03 and 09 had no seam that could be tested without sending a request, so each gained a few-line function: `xfrmReplayWindowInESN`, `xfrmUserTmplFamily`, `xfrmUserpolicyFlags`. Patch 02's test covers the decision; that the attribute is then written and the legacy field zeroed still needs root to prove. Patch 03 no longer matches the diff posted in upstream issue 1213 word for word.
- R-9 did not materialise. Applied to upstream `main` at `e0e26cd`, patches 01 to 10 apply unchanged in file-name order; 01, 02, 03, 05 and 10 also apply on their own; 04, 06, 07, 08 and 09 need the predecessors their leading text names. The sixteen new tests pass there as an unprivileged user. Each patch file is therefore its pull request. Patch 90 does not build on `main`, where the package handle is a function call, and is never proposed. A clone of upstream is at `~/Code/github.com/vishvananda/netlink` on the server.
- Phases landed in `b43120b371` (2026-11-13): the split into eleven files (AC-1, AC-4, AC-10), the guard (AC-9 as amended below, AC-13) and the pages (AC-12). Open: the upstream-style tests and the two selector gaps (AC-3, AC-7, AC-8), the check of each patch against upstream `main` (AC-2), the move of the private ABI into the dataplane (AC-5, AC-6), and the pull requests on the owner's order (AC-11).
- AC-9 is amended by the independent test judge's refusal: the hand-named guards stay. A guard derived from the directory cannot notice a patch file that is gone, so each named guard pins that its patch is present, and `TestNetlinkXFRMPatchApplied` names the eleven files. `TestEveryVendorPatchIsApplied` guards every file present beside them. Returning to one list needs a check that refuses the removal of a patch file, such as the commit gate treating it as it treats a removed test; that is separate work and the owner's choice.
- A patch of a series does not reverse out of the final vendored tree when a later patch sits on its lines, so `assertVendorPatchApplied` judges a patch at its place in its module's stack: the later patches of the same module are reversed first, in a copy.
- The already-applied probe of the re-vendor step (`applyVendorPatches`) and of the Dependabot workflow asks the same standalone question. Both run on a freshly vendored tree, where applying the series in order works. Run on an already patched tree, the probe would misjudge a stacked patch as not applied and then fail its forward check. Nothing does that today; it is a limit to remove before anything does.
- Patch 05 carries `xfrm_state_scope_linux_test.go` (it tests only the truncated-header refusal) and patch 90 carries `xfrm_selector_family_linux_test.go` (it calls the private scope writer). Both are rewritten without Ze in the upstream-style tests phase, where the selector test moves to patch 10.
- The series order and each patch's predecessors: 01 state read-back (ESN flag, replay window), 02 replay window above 32, 03 template family with no destination (these three are upstream issue 1213); 04 read-back by message family (needs 01); 05 truncated state header; 06 selector port masks (needs 01, 04); 07 AFUnspec (needs 01); 08 NoPMTUDisc (needs 07); 09 ICMP flag (needs 06, 08); 10 selector family from the prefix mask; 90 Ze state scope (needs 09).
- The owner granted a one-off exception to the WIP cap for this spec on 2026-11-13: it was claimed one above the cap, on his order to implement it. The cap itself is unchanged.
- AC-13 is already met: commit `ce3a0e0fd7` made `TestPatchedVendorModulesMatchUpstream` apply every patch forward, and the journal row this spec owned for it is `fixed:`. The remaining guard work of this spec is the one derived guard over the patch directory.
- The starting state, the order of work and the findings behind A-1, A-5 and A-6 are recorded below, moved from the session's handover of 2026-11-13. The order of work is the series order above.
- Starting state (2026-11-13): the RFC 4301 commits `8b3c7a34cd` and `2db581e7d6` are in HEAD and touch no netlink file. The last commit that touched the vendored library is `e1f42c2ca4` (2026-11-10) before this work. The library part of RFC 4301 (the mapped IPv6 selector fix and the state scope) was committed on its own as `352f231413` on 2026-11-13, by the session that wrote the handover, because its own session had stopped. The Ze callers of it are not in that commit.
- The evidence trail behind A-1, A-5, A-6, R-9 and R-10:

| Spec item | Finding | Evidence |
|-----------|---------|----------|
| A-1 | Half false. Upstream `main` (at `e0e26cd`) has not fixed the selector family and sets no state flag but ESN, so no patch is redundant. It HAS been reshaped: `pkgHandle` became a `pkgHandle()` call, and `XFRMA_SA_PCPU` is no longer sent on get or delete. No release follows v1.3.1 | `diff` of the three files at v1.3.1 against `raw.githubusercontent.com/vishvananda/netlink/main` |
| A-5 | Confirmed. No Ze producer installs a selector with a source and no destination; an unrestricted side is always an explicit zero prefix | Trace of `childPairPolicy` and its fallback, `newRekeyedChild`, `inboundStateSelector`, `spdPolicyDirection`, the configured traffic selectors, `ikeBypassPolicies`, `spdCatchAllEntries`, `xfrmInterfaceExemptions`, `xfrmStateScopeFromParams`, OSPF's `buildIPsecSA` and `buildIPsecPolicies`, and the helpers `intersectNet`, `rangePrefixes`, `leaseHostIn`. Nothing at the library boundary refuses one, so the fix stays in the series |
| A-6 | Confirmed. The kernel refuses an `AF_INET` selector with a prefix length above 32 with `-EINVAL` and "Invalid prefix length in selector (must be <= 32 for IPv4)", on policy add (`verify_newpolicy_info`) and on state add (`verify_selector_prefixlen` from `verify_newsa_info`) | `net/xfrm/xfrm_user.c` in Linux 7.3, the version `internal/appliance/kernel.version` pins. Present in mainline from v4.19, absent in v4.18. Not read: the introducing commit and stable backports |
| R-9 | A patch against v1.3.1 equals its pull request against `main` byte for byte only where `main` left the surrounding lines alone. The spec's goal of byte-identical patches holds for some patches only. Each patch records whether it applies to `main` unchanged; where it does not, the pull request branch is the source and the Ze patch is the same change rebased onto v1.3.1 | The A-1 comparison |
| R-10 | Upstream issue 1213 of `vishvananda/netlink` ("xfrm: three bugs in state read-back, replay window and template family"), opened by Thomas on 2026-08-25 with three diffs against `main` at `156a440`, already proposes three fixes this patch carries: the ESN flag and replay window on read-back, the replay window cut to 32, and the template family. It is open with no maintainer answer. The patch's leading text and the spec do not mention it. The series starts from those three diffs and names the issue | The issue and its one comment |

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One patch per upstream change, in an ordered series | Keep one patch and extract pull requests from it by hand | A patch that is byte-identical to its pull request can be deleted the day upstream releases it, and review comments map onto one file. A hand extraction drifts from the patch Ze actually builds with |
| Move the private kernel ABI into the dataplane, leaving a generic attribute hook in the library | Keep the private ABI in the library as a second, Ze-only patch | The owner's preference is a Ze fix where Ze can make it. The dataplane already encodes the XFRM messages the library lacks, and already owns the canonical scope, so the library copy is a second owner of one fact. The hook names no Ze concept, so it can itself be proposed upstream |
| Keep the selector family fix in the library and propose it upstream | Refuse IPv4-mapped IPv6 selectors in Ze, or have Ze encode policy and state requests itself | The defect is in the library's contract: nothing Ze passes produces the right bytes. Refusal removes support for a legal selector, and Ze's own encoder would re-implement the library's state and policy requests. A merged pull request removes the patch, which is how this change ends up outside the library diff. Owner decision owed: see Known Limitations |
| One derived guard over the patch directory | One hand-named guard per patch, as today | The series would add a guard per file; the directory already is the set |
| Upstream-candidate tests are pure encode and decode checks | Tests that install into a network namespace, as much of upstream's suite does | Upstream's namespace tests need root, which Ze agents do not use, and a byte check is what proves an encoding fix |

## Known Limitations
- The selector family fix stays a library patch until upstream merges it. The owner may instead choose refusal in Ze; that choice is put to him at the design gate and changes only the first patch of the series.
- Whether upstream accepts the API additions is outside Ze's control. A declined change stays a Ze patch with the refusal recorded.
- Pull requests are opened only on the owner's order. Until then the work ends at branches that are ready.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test (N-A when Scope is tooling or docs, which delete that section)
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit, and the full verification is Thomas's release check. A red CI reports gets a fix commit
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)
