# Spec: appliance-image-third-party-notices

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Priority | P2 |
| Release reason | A released appliance image redistributes third-party binaries beside `ze` (a shell, boot code, firmware, the gokrazy programs), and only the Linux kernel has a notice in the image today |
| RFC gaps | - |
| Depends | `plan/release/delivery/spec-thanks-third-party-credits.md` |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-20 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The appliance image carries third-party software that is not compiled into
`ze`, and the image says nothing about most of it. The goal is that an image
states every third-party work it contains, with its licence, its copyright
holder and where its source is obtained, and that `ze thanks` on an appliance
lists those works beside the ones compiled into the binary.

This spec was split from `plan/release/delivery/spec-thanks-third-party-credits.md`
by Thomas on 2026-11-20: that spec covers what is compiled into `ze`, this one
what the image carries beside it. It reuses that spec's declaration, its
credits list and its commands.

What research found on 2026-11-20, each to be re-read at the source before the
design is written:

| Work in the image | Evidence | Licence record today |
|-------------------|----------|----------------------|
| The Linux kernel and its modules | `internal/appliance/instance/provenance.go` `Provenance.LinuxNotice` | A notice in the image naming version, source and checksum |
| A shell binary shipped as `ze-recovery-shell`, taken from the gokrazy serial shell package's extra files; its strings read "BusyBox v1.34.1" | `docs/architecture/appliance-serial-login.md`, "ze is the gate, busybox stays the shell" | None |
| The MBR boot code of the gokrazy tooling, whose source header places it under the GNU General Public License, version 3 or later, copyright Sebastian Plotz | The assembly source vendored with the gokrazy internal module | None |
| The systemd-boot EFI binaries and the EDK2 firmware images embedded by the gokrazy tooling | The gokrazy tools module's third-party directory; no licence file beside them | None |
| An Intel graphics firmware blob, on the hardware profile that needs it | `hardwareKMSFirmware` in the kernel builder | None |
| gokrazy's own programs (init, randomd, heartbeat) and the modules they link | The image's package configuration; the linked module set is unverified, because no image was built during research | None |
| The GRUB image of the ISO installer, built from the build host's GRUB | `docs/architecture/appliance/iso-installer.md` | None |
| The installer's kernel | The notice is placed only by the appliance image assembly | None |

The decisions this spec needs from Thomas, to be put to him with the licence
texts quoted, once they have been read:

| Decision | Why it is his |
|----------|---------------|
| How Ze complies for each GPL-licensed binary in the image | He decided the kernel's route on 2026-10-09 (a notice naming the published source); whether the same route fits a binary Ze did not build is a new question |
| Whether the image keeps a binary whose licence or source Ze cannot establish | It is a choice about what ships |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/<doc>.md` - [why relevant]
  → Decision: [specific architectural decision that constrains this spec]
  → Constraint: [specific rule from the doc that applies here]

**Key insights:** (minimal context to resume after compaction)
- [insight from docs]

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/appliance/instance/provenance.go` - `Provenance.LinuxNotice` writes the kernel notice: version, source URL, SHA-256, and where the configuration, patches and build scripts are published

**Behavior to preserve:** (unless the user explicitly said to change it)
- The kernel notice and the decision behind it

**Behavior to change:** (only what the user asked for)
- [to be written with the design]

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| [mechanism] | [file + symbol, or page + section] | [what it does today] | [extends / replaces / does not apply: why] |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| [page] | [section heading] | [what derived it] | "[verbatim span]" | [agrees / departs / silent] | [why] |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- [Where data enters]

### Transformation Path
1. [Stage 1]

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| [boundary] | [how] | No |

### Changed-Wire Census

**Search evidence:** [scope, terms, positive control and source findings]

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| [format] | [role] | [source site] | [search finding] | [change or preserve] |

### Integration Points
- [Existing function/type this connects to] - [how it integrates]

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No duplicated functionality (extends existing, does not recreate) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The works listed in the Task are everything the image carries beside `ze` | Research on 2026-11-20 read the image's package configuration and the gokrazy tooling, and built no image | A work ships uncredited | Build an image and list its files and each binary's linked modules | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A binary in the image has a licence whose terms Ze does not meet today | The licence text, once read | The decision goes to Thomas with the clause quoted |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | [to be written with the design] |
| How is it reverted? | [to be written with the design] |
| Who else touches this path? | [to be written with the design] |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| [entry point] | → | [function that runs] | [test name] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | [what triggers the behavior] | [positive observable outcome] | [what remains permitted and why, or why none] |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestXxx` | `internal/.../xxx_test.go` | [description] | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| [field] | [min-max] | [value] | [value or N/A] | [value or N/A] |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-xxx` | `test/.../*.ci` | [what the user expects to happen] | |

## Files to Modify
- `internal/...` - [feature changes]

## Files to Create
- `internal/...` - [new feature file]

## Design presentation

### What changes

[The design in a paragraph]

### Before and after

[One numbered example per change a user sees]

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| [alternative] | [why] |

### Documents departed from

[none, or each departed page]

### Choices for Thomas

| # | Choice | Options | Decision |
|---|--------|---------|----------|
| C-1 | [choice] | (a) [recommended]; (b) [other] | |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- [to be written with the design]

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Checklist

### Goal Gates (MUST pass)
- [ ] The named runs for this spec pass: `./le test functional appliance`, and the unit tests of the packages the design names

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
