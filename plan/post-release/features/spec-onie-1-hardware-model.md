# Spec: onie-1 -- a hardware model the appliance builder takes as input

| Field | Value |
|-------|-------|
| Status | design |
| Scope | cli |
| Priority | P3 |
| Release reason | Naming a hardware model is added platform support for the builder; today's images build and boot without it. |
| RFC gaps | - |
| Depends | - |
| Phase | DESIGN |
| Handoff | - |
| Design tier | strongest tier |
| Updated | 2026-11-28 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Parent: `spec-onie-0-umbrella.md` (owner decisions D-1 to D-5 and the research).

## Task

The appliance builder takes the hardware model as an input. Thomas's words,
voice-transcribed: "we should be able to pass the model to the builder (same
for N100)".

Today an operator who builds for a given box has to know and set the facts of
that box one by one: the architecture, the installer kernel profile, and
nothing at all for the console, which is fixed for every image. Nothing in the
builder knows what an N100 mini PC or a Dell S4048-ON is.

After this work an operator names the box. A model is a small declared file
that the builder discovers. It states the architecture, the installer kernel
profile, the serial console, the firmware modes the box boots in, the kernel
drivers the box cannot work without, and, for a switch, the ONIE platform
strings its installer accepts. The builder fills the image options from the
model, refuses a config that contradicts it, and refuses to build when a
kernel lacks a driver the model requires.

The first models are `generic` (exactly today's behaviour, and the default),
`n100`, `dell-s4048-on` and `dell-s5232f-on`.

The ONIE installer (`spec-onie-3-installer.md`) reads the model of an appliance
to learn which switches its file may install on. This spec does not build that
installer.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`
  → Constraint: a feature registers itself and is discovered; adding a model must not need an edit to a switch, a list or a help string.
- [ ] `ai/doors/appliance.md`
  → Constraint: `internal/appliance` is build-host tooling behind `ze_setup`; `./ze appliance` does not exist, so every printed instruction names `ze-host`.
  → Constraint: an image option validated at the helper and not at the CLI is a recorded trap (`image.isolated-cpus`): each new option gets a `.ci` through `ze appliance init`.
- [ ] `docs/architecture/appliance/kernel-profiles.md`
  → Decision: a kernel profile is a pair of files in `tools/installer-kernel/`, found by directory scan (`registeredKernelProfiles`); a model is declared and found the same way.
  → Constraint: `image.kernel-profile` selects the installer kernel only; the runtime kernel is one shared profile, so a driver a model requires at runtime goes into the shared runtime fragments.
- [ ] `docs/architecture/appliance/builder.md`
  → Constraint: the appliance config is the unexported `applianceConfig`; `ImageConfig` holds arch, size, kernel profile, hugepages, isolated CPUs, crash dump and memory.
- [ ] `docs/architecture/appliance/iso-installer.md`
  → Constraint: names embedded in a kernel command line must match a strict pattern; a model name and a console value reach that command line, so both are validated the same way.
- [ ] `plan/post-release/features/spec-hardware-capacity-profile.md`
  → Constraint: that skeleton plans a runtime database matched on DMI facts. The model names of this spec are the names that database uses for the same boxes; neither spec defines a second vocabulary.

**Key insights:**
- No build-time model exists. The serial console is one fixed value in `gokrazy/ze/config.json`.
- The builder already renders a derived gokrazy parent directory per build to carry kernel arguments (`resolveBuildParentDir`), which is where a per-model console can be written without touching the shared config.
- No kernel fragment builds `ixgbe`, and `igb` is a module loaded at start.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/appliance/config.go` - `ImageConfig` fields as listed above; `Validate`, `DefaultConfig`, `LoadConfig`.
- [ ] `internal/appliance/cmd_init.go` - `runInit` takes `--config`, `--cert`, `--key`, `--managed`, `--batch`; the interactive path does not ask for a kernel profile.
- [ ] `internal/appliance/kernelreg.go` - `registeredKernelProfiles` scans a directory for `<name>.config`; `resolveKernelProfileFrom` validates the name and requires the paired manifest.
- [ ] `internal/appliance/kernelargs.go` - `resolveBuildParentDir` writes a derived parent directory carrying hugepage, CPU isolation and crash-dump kernel arguments.
- [ ] `internal/appliance/cmd_iso.go` - `isoKernelConsoleArgs` chooses the installer console by architecture only.
- [ ] `internal/appliance/main.go` - `applianceCommands` is the package's own command table.
- [ ] `gokrazy/ze/config.json` - `SerialConsole` is `ttyS0,115200` for every image.

**Behavior to preserve:**
- A config with no model builds the same image, byte for byte, as before this work.
- Every existing `image.*` option keeps its meaning and its validation.
- `ze appliance kernel --profile` keeps selecting a profile directly.

**Behavior to change:**
- `image.model` exists, with a matching `--model` on `ze appliance init`.
- The console of an image and of its installer follows the model.
- A build refuses when a kernel lacks a driver the model requires.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Kernel-profile registry | `internal/appliance/kernelreg.go` `registeredKernelProfiles`, `resolveKernelProfileFrom` | Discovers profile file pairs by directory scan and validates the name | extends: the model registry is a second directory scanned by the same rules, and a model names the profile it uses |
| Kernel requirement manifests | `internal/appliance/kernelreq.go` `universalKernelRequirements`, the `.require` files | State the kernel options a profile must carry and check a built config against them | extends: a model's required drivers are requirement lines checked by the same code against the runtime and installer kernels |
| Image options | `internal/appliance/config.go` `ImageConfig`, `Validate` | Hold and validate per-image facts | extends: gains `model`; the model supplies defaults and `Validate` refuses a contradiction |
| Derived build directory | `internal/appliance/kernelargs.go` `resolveBuildParentDir` | Renders per-build kernel arguments beside the shared gokrazy config | extends: also renders the model's serial console |
| Installer console choice | `internal/appliance/cmd_iso.go` `isoKernelConsoleArgs` | Chooses the console by architecture | replaces: the console comes from the model, and `generic` states today's per-architecture values |
| Runtime hardware facts | `internal/component/host/dmi_linux.go`; `spec-hardware-capacity-profile.md` part P5 | Reads DMI on the device; plans a matched capacity database | does not apply at build time; the names are shared so the two never diverge |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | The core knows nothing, and everything registers | the door every change reads | "It never needs an edit to a switch, a factory, a field list, a" | agrees | A model is a file found by scan; no list, switch or help string names one |
| `docs/architecture/appliance/builder.md` | Kernel command line | `// Design:` page of `config.go`, `cmd_init.go`, `cmd_build.go`, `main.go`; a page this spec modifies | "pure function of the config in `kernelargs.go`, and each answers nil when its" | agrees | The console is one more pure function of the config in the same file, and it answers nil for `generic`, so today's command line is unchanged. The page gains the model and the console as a fourth field |
| `ai/doors/appliance.md` | Boundaries | the door covering every `internal/appliance` file | "The `./ze` launcher binary has no `ze_setup`" | agrees | Every refusal and instruction of this spec names `ze-host` |
| `docs/architecture/appliance/build-artifacts.md` | Decisions | `// Design:` page of `doctor_checks.go` | "Doctor checks test for the artifact, not for the build tool." | agrees | The model check tests the model files and the cached kernel's emitted config, never a tool |
| `docs/architecture/appliance/gokrazy-build-pins.md` | Derived parent instances keep the builddir pins | anchored to `cmd_build.go`, `kernelargs.go` | "An appliance config that asks for hugepages cannot be built from the tracked" | agrees | The console travels through the same derived parent, so the pins this page protects are kept by the existing path |
| `docs/architecture/appliance/ota-push.md` | Decisions | anchored to `cmd_build.go` | | silent | The page covers what a push sends to a device; a model changes nothing a push sends or validates |
| `docs/architecture/vpp-host-tuning.md` | Kernel arguments go through a derived instance config | `// Design:` page of `kernelargs.go` | "The checked-in `gokrazy/ze/config.json` is never edited." | agrees | The model's console is patched into the derived config; the checked-in file keeps its one console value |
| `docs/architecture/appliance/kernel-profiles.md` | Decisions | `// Design:` page of `kernelreq.go`; a page this spec modifies | "The registry is open. Adding a profile is adding two files, not editing a" | agrees | The model registry is open in the same way, and a model names a profile from this registry. The page gains the statement that a model may add requirement lines to the runtime kernel check |
| `docs/architecture/appliance/iso-installer.md` | Traps | `// Design:` page of `cmd_iso.go`; a page this spec modifies | "Names embedded in the GRUB kernel command line must survive two parsers." | agrees | The console value from a model is pattern-checked before it reaches the GRUB command line. The page gains the statement that the installer console comes from the model |
| `ai/doors/le-tooling.md` | Where it sits | the door covering the four `.ci` files | "Functional `.ci` files live under `test/<suite>/` and run through" | agrees | The four new tests are `.ci` files in the existing `test/appliance/` suite |
| `docs/architecture/vpp-host-tuning.md` | Design decisions | decision of 2026-11-23 | "When Ze does not believe the host has enough resources for VPP to remain stable, it warns and still starts VPP. The requirement changes with the configuration and with what else runs on the host (on a general Linux install other programs take memory too), so the check runs at every start against the machine as it is then, and a warning tells the operator they are taking a risk. A measurement the operator recorded on their own hardware stops the warning on that hardware, and the operator can silence the warning with an environment value, set in the configuration or in the OS environment" | agrees | This spec adds no check of host resources and changes no VPP start behaviour; its one refusal is at build time, for a kernel that lacks a driver, which that decision does not cover |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `ze-host appliance init --model <name> <appliance>`, or `image.model` in the JSON given to `--config`.
- Format at entry: a model name, validated against the names found in the model directory.

### Transformation Path
1. The model directory is scanned; each file is parsed and validated into a model value.
2. `init` resolves the named model and writes the appliance config with `image.model` set and the model's architecture and kernel profile filled in where the operator gave none.
3. `Validate` reloads the model at every later command and refuses a config whose architecture contradicts it.
4. `build` checks the model's required drivers against the runtime kernel's emitted config, then renders the model's console into the derived build directory.
5. `iso` (and later `onie`) read the model for the installer console and the installer kernel profile.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Model file ↔ builder | A declared file parsed at each command | No |
| Builder ↔ gokrazy packer | The derived parent directory's config carries the console | No |

### Changed-Wire Census

**Search evidence:** the one persisted format this spec changes is the appliance config JSON (`image` object). Scope searched: `internal/appliance`, `internal/le`, `test/appliance`, `docs/guide` for `kernel-profile` and `ImageConfig`. Positive control: `kernel-profile` is found in `config.go` (writer and reader), `cmd_iso.go` (reader) and `docs/guide/appliance.md`. The model file is a new format with one writer (a person) and one reader (the registry).

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| appliance config JSON, `image.model` | writer | `internal/appliance/cmd_init.go` `runInit`, `config.go` `saveConfig` | `ImageConfig` is marshalled by `saveConfig` | new optional field; an absent field reads as `generic` |
| appliance config JSON, `image.model` | reader | `internal/appliance/config.go` `LoadConfig`, `Validate` | every command loads through `LoadConfig` | an existing config without the field keeps loading and building unchanged |
| appliance config JSON, `image.model` | writer | an operator's own `--config` and `--batch` files, written by hand | `cmd_init.go` reads both | unchanged files stay valid |
| model file | writer | a file under the model directory, written by hand | new | none to migrate |
| model file | reader | the model registry of this spec | new | none to migrate |

### Integration Points
- `registeredKernelProfiles` and its name validation - the model registry reuses the scan and the name rule.
- `kernelreq` requirement checking - a model's driver list is checked by it.
- `resolveBuildParentDir` - gains the console.

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
| A-1 | The console can be set per build through the derived parent directory, without editing the shared gokrazy config | `docs/architecture/vpp-host-tuning.md`, "Kernel arguments go through a derived instance config": `materializeDerivedParent` writes a patched `config.json`, and `SerialConsole` is a field of that file | The console needs another route into the packer | Read `resolveBuildParentDir` and the packer's `writeCmdline` before phase 3; a unit test reads the built command line | unvalidated |
| A-2 | The S4048-ON and S5232F-ON facts in their model files match the units | Upstream ONIE and SONiC platform files only (umbrella, "The two switches") | A model accepts the wrong platform string or names the wrong console or driver | The switch commands in the umbrella, run by Thomas; the model files are corrected from their output before this spec closes | unvalidated |
| A-3 | The N100 box needs the `hardware-kms` installer profile and boots under UEFI | `tools/installer-kernel/README.md` and `hardware-kms.config` name the N100 display loss | The `n100` model picks the wrong profile | Thomas confirms the profile he uses for the N100 | unvalidated |
| A-4 | The amd64 runtime kernel lacks `ixgbe` | Only an arm64 emitted config was cached where the research ran; no fragment names it | The driver refusal for the S5232F-ON never fires, harmlessly | Read the amd64 runtime kernel's emitted config on the Linux server | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A model silently changes an existing appliance's image | A rebuilt image differs with no config change | `generic` is the default and states today's values; a test compares the build inputs of a config with and without `generic` |
| R-2 | A model and an explicit option disagree and the builder picks one silently | An image built for the wrong architecture | `Validate` refuses the disagreement and names both values |
| R-3 | A model requires a driver and the build succeeds without it | The installed box has no management port | The build refuses, naming the driver, the kernel and the fragment to edit |
| R-4 | A model name reaches a kernel command line unvalidated | A name with a space or quote | The model name uses the kernel-profile name rule; the console value has its own strict pattern |
| R-5 | The model vocabulary drifts from the runtime capacity database | Two names for one box | The capacity spec cites this registry as the source of names |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | An image builds with the wrong console or without a needed driver; a box installs and is unreachable on its console or management port. No running device is affected |
| How is it reverted? | A single commit revert; configs carrying `image.model` then fail to load until the field is removed |
| Who else touches this path? | `spec-appliance-perm-grows-at-install` and `spec-install-9-cloud-init` edit `internal/appliance`; `spec-onie-3-installer` reads the model |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze-host appliance init --model n100 lab` | → | model registry lookup, config written with the model's values | `test/appliance/appliance-model-init.ci` |
| `ze-host appliance init --model no-such-box lab` | → | model registry refusal | `test/appliance/appliance-model-unknown.ci` |
| `ze-host appliance model list` | → | model registry scan | `test/appliance/appliance-model-list.ci` |
| `ze-host appliance build` of a model whose driver the kernel lacks | → | requirement check against the kernel config | `TestBuildRefusesModelDriverTheKernelLacks` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `init --model n100 lab` with no other image option | The written config holds `image.model` `n100` and the architecture and kernel profile the `n100` model states | An operator's explicit kernel profile survives, because a profile is a choice within a model, not a fact of it |
| AC-2 | `init` with no `--model` and a config with no `image.model` | The appliance is created with model `generic`, and its build inputs equal those of the same config before this work | Every existing option survives unchanged, because `generic` states today's values |
| AC-3 | `init --model no-such-box lab` | The command exits non-zero and prints the unknown name, the list of known models, and the `ze-host appliance model list` command | No appliance directory survives, because a config with an unknown model could never build |
| AC-4 | A config whose `image.arch` differs from its model's architecture | `Validate` refuses at every command, naming both values and the edit that fixes it | A config whose architecture agrees survives, as does one that omits the architecture and takes the model's |
| AC-5 | A new model file is dropped into the model directory | `model list` shows it and `init --model` accepts it, with no other file edited | No survivor question: nothing is removed |
| AC-6 | `build` of an appliance whose model requires a driver absent from the runtime kernel's config | The build stops before packing and names the driver, the kernel, and the fragment file to add it to | A build whose kernel carries every required driver proceeds, which is the useful outcome |
| AC-7 | `build` of an appliance whose model states a console other than the default | The built image's kernel command line names the model's console | The default console survives for `generic`, because that is today's behaviour |
| AC-8 | `iso` of an appliance with a model | The installer's kernel command line names the model's console and the ISO uses the model's installer kernel profile | The per-architecture console survives for `generic` |
| AC-9 | A malformed model file (unknown key, bad console, unknown kernel profile) | Every command that scans the registry names the file, the key and the accepted form, and the doctor check reports it | Well-formed models survive and stay usable, because one bad file must not hide the others |
| AC-10 | `model list` and `model show dell-s4048-on` | The list prints each model's name, architecture and description; `show` prints every fact of one model, with the JSON pipe giving the same facts as data | No survivor question: read-only |
| AC-11 | The `dell-s4048-on` and `dell-s5232f-on` model files | Each states the platform strings, firmware mode, console and required drivers read from the unit (assumption A-2) | Values not yet read from a unit are not shipped as facts; the model file marks them and the feature page says the model is unproven on hardware |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Creates an appliance for an N100 by naming it | `init --model` -> registry -> config -> `build` -> image command line | `test/appliance/appliance-model-init.ci`, `TestBuildRendersModelConsole` |
| 2 | Adds a new box by writing one file | model directory -> registry scan -> `model list` | `test/appliance/appliance-model-list.ci` |
| 3 | Is stopped before building an image that cannot work | `build` -> requirement check -> refusal with the fix | `TestBuildRefusesModelDriverTheKernelLacks` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRegisteredModelsShippedSet` | `internal/appliance/modelreg_test.go` | The scan finds exactly the shipped models | |
| `TestModelFileRejectsUnknownKey` | `internal/appliance/modelreg_test.go` | A malformed file is refused by name and key | |
| `TestModelNameRule` | `internal/appliance/modelreg_test.go` | A model name follows the kernel-profile name rule | |
| `TestModelConsolePattern` | `internal/appliance/modelreg_test.go` | A console value outside the pattern is refused | |
| `TestValidateRefusesArchAgainstModel` | `internal/appliance/config_test.go` | AC-4 | |
| `TestGenericModelLeavesBuildInputsUnchanged` | `internal/appliance/config_test.go` | AC-2 | |
| `TestBuildRefusesModelDriverTheKernelLacks` | `internal/appliance/cmd_build_test.go` | AC-6 | |
| `TestBuildRendersModelConsole` | `internal/appliance/kernelargs_test.go` | AC-7 | |
| `TestISOConsoleFollowsModel` | `internal/appliance/cmd_iso_test.go` | AC-8 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| console speed | the speeds the pattern lists | 115200 | 0 | a value not in the list |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `appliance-model-init` | `test/appliance/appliance-model-init.ci` | Naming a model at init writes its facts into the config | |
| `appliance-model-unknown` | `test/appliance/appliance-model-unknown.ci` | An unknown model is refused with the list and the fix | |
| `appliance-model-list` | `test/appliance/appliance-model-list.ci` | The shipped models are listed, and one added file appears | |
| `appliance-model-arch-mismatch` | `test/appliance/appliance-model-arch-mismatch.ci` | A config contradicting its model is refused through the CLI | |

### Interop Tests (Scope: protocol)
N-A: no wire protocol and no peer. The ONIE proof belongs to `spec-onie-3-installer.md`.

## Files to Modify
- `internal/appliance/config.go` - `image.model`, defaults from the model, the contradiction refusal
- `internal/appliance/cmd_init.go` - `--model`, the interactive prompt, batch entries
- `internal/appliance/cmd_build.go` - the required-driver check before packing
- `internal/appliance/kernelargs.go` - the console in the derived build directory
- `internal/appliance/cmd_iso.go` - console and installer profile from the model
- `internal/appliance/kernelreq.go` - requirement lines supplied by a model
- `internal/appliance/main.go` - the `model` command row
- `internal/appliance/doctor_checks.go` - the model registry check
- `gokrazy/kernel/runtime.config`, `gokrazy/kernel/runtime.require`, `tools/installer-kernel/hardware.config`, `tools/installer-kernel/hardware.require` - each driver a shipped model requires and a kernel lacks
- `docs/architecture/appliance/builder.md`, `kernel-profiles.md`, `iso-installer.md` - the model, and the console no longer chosen by architecture
- `docs/guide/appliance.md` - naming a model

## Files to Create
- `internal/appliance/modelreg.go` - the model registry: scan, parse, validate
- `internal/appliance/cmd_model.go` - `model list` and `model show`
- `tools/appliance-models/generic.model`, `n100.model`, `dell-s4048-on.model`, `dell-s5232f-on.model` - the first models
- `tools/appliance-models/README.md` - the file format and how to add a model
- `docs/architecture/appliance/hardware-models.md` - the design page
- `test/appliance/appliance-model-init.ci`, `appliance-model-unknown.ci`, `appliance-model-list.ci`, `appliance-model-arch-mismatch.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | The appliance config is the builder's own JSON, not daemon config |
| YANG validation constraints | N-A | Same reason; validation is in `config.go` `Validate` |
| YANG custom validators | N-A | Same reason |
| CLI commands/flags | Yes | `internal/appliance/cmd_init.go` (`--model`), `internal/appliance/cmd_model.go` |
| CLI grammar (keyword before value) | Yes | `ai/rules/cli.md`; the `ze appliance` surface is flag-based today, so the new flag follows its siblings, and `model list` and `model show <name>` put the keyword first |
| Editor autocomplete | N-A | Not a daemon config leaf |
| Functional test for new RPC/API | Yes | the four `test/appliance/appliance-model-*.ci` files |
| Pipe completeness | Yes | `model list` and `model show` route output through the shared pipes |
| Env var registration | No | No environment variable is added |
| Doctor check for runtime dependencies | Yes | `internal/appliance/doctor_checks.go`: the model directory is a new file path the builder depends on |
| Doctor code per problem class | Yes | `doctor-appliance-model`, build-host phase: a model file that does not parse, and a model whose required driver the cached kernel lacks |
| Prometheus counters/metrics | N-A | Build-host tooling exports no metrics |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/installation.md` |
| 2 | Config syntax changed? | No | The daemon config is untouched |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | No | No RPC |
| 5 | Plugin added/changed? | No | No plugin |
| 6 | Has a user guide page? | Yes | `docs/guide/appliance.md` |
| 7 | Wire format changed? | No | No wire |
| 8 | Plugin SDK/protocol changed? | No | Untouched |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No RFC |
| 10 | Test infrastructure changed? | No | The `.ci` files use the existing appliance suite |
| 11 | Affects daemon comparison? | No | Not a daemon feature |
| 12 | Internal architecture changed? | Yes | `docs/architecture/appliance/hardware-models.md` (new), `builder.md`, `kernel-profiles.md`, `iso-installer.md`, and the README table of `docs/architecture/appliance/` |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | No | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | `ze appliance` is outside the daemon command registry |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived by `./le spec citation anchors`; the pages of row 12 carry them |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/appliance.md` examples of `appliance init` |

## Design presentation

### What changes

An operator names the box an appliance is for. `ze-host appliance init --model
n100 lab` writes a config whose architecture, installer kernel profile and
console come from the `n100` model, and later commands refuse a config that
contradicts its model or a kernel that lacks a driver the model requires. A
model is one file in `tools/appliance-models/`; adding a box is adding a file.
An appliance with no model is `generic` and builds exactly as today.

### Before and after

1. Creating an appliance for an N100. Before: the operator writes a JSON file setting the kernel profile and passes it with `--config`, and the console cannot be chosen. After: `ze-host appliance init --model n100 lab`.
2. Listing what the builder knows. Before: nothing to list. After: `ze-host appliance model list` prints `generic`, `n100`, `dell-s4048-on` and `dell-s5232f-on`, each with its architecture and a one-line description.
3. An unknown model. After: `init --model s4048` exits non-zero and says the model is unknown, lists the four known names, and names `ze-host appliance model list`.
4. A kernel without a needed driver. Before: the image builds and the installed box has no management port. After: `build` stops and says which driver the model requires, which kernel lacks it, and which fragment file to add it to.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| One builder flag per fact (`--console`, `--firmware`, `--onie-platform`) | The operator would still have to know every fact of the box, which is what Thomas asked to stop |
| Models as Go values registered from `init()` | A model is data with no behaviour; the kernel-profile registry already discovers data files by scan, and a second way to register the same kind of thing would sit beside it |
| Detect the model on the device from DMI at install time | The builder must know the model before any device exists, to choose the kernel profile and to stamp the installer; runtime matching is the capacity-profile spec's subject |
| Fold the model into the kernel profile | A profile is a kernel build; one profile serves several models, and a model carries facts no kernel build has (firmware mode, ONIE strings) |

### Documents departed from

none

### Choices for Thomas

| # | Choice | Options | Decision |
|---|--------|---------|----------|
| C-1 | Which installer kernel profile and firmware mode the `n100` model states | (a) `hardware-kms`, UEFI, recommended from the installer-kernel README; (b) another profile | |
| C-2 | Whether `dell-s4048-on` and `dell-s5232f-on` ship before their facts are read from a unit | (a) ship them marked unproven, recommended, so the ONIE spec can be built against them; (b) hold both until the switch commands have been run | |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the `model` command row and `--model` flag exist and reach a registry stub
   - Tests: `appliance-model-list.ci`, `appliance-model-unknown.ci`
   - Files: `main.go`, `cmd_model.go`, `cmd_init.go`, `modelreg.go`
   - Verify: both tests fail because the registry returns nothing
2. **Phase: The registry** -- scan, parse, validate, the four model files and the doctor check
   - Tests: `TestRegisteredModelsShippedSet`, `TestModelFileRejectsUnknownKey`, `TestModelNameRule`, `TestModelConsolePattern`, `appliance-model-list.ci`
   - Files: `modelreg.go`, `tools/appliance-models/`, `doctor_checks.go`, `hardware-models.md`
   - Verify: the list test passes; a file added in the test appears
3. **Phase: The model in the config** -- defaults, the contradiction refusal, `generic` as the default
   - Tests: `TestValidateRefusesArchAgainstModel`, `TestGenericModelLeavesBuildInputsUnchanged`, `appliance-model-init.ci`, `appliance-model-arch-mismatch.ci`
   - Files: `config.go`, `cmd_init.go`, `builder.md`
   - Verify: assumption A-1 is settled by reading the producer before this phase edits `kernelargs.go`
4. **Phase: The model at build** -- the console and the required-driver check, and the drivers the shipped models need
   - Tests: `TestBuildRendersModelConsole`, `TestBuildRefusesModelDriverTheKernelLacks`, `TestISOConsoleFollowsModel`
   - Files: `kernelargs.go`, `cmd_build.go`, `cmd_iso.go`, `kernelreq.go`, the kernel fragments, `kernel-profiles.md`, `iso-installer.md`, `docs/guide/appliance.md`
   - Verify: the main thread runs the scoped appliance tests; a kernel fragment change owes a runtime kernel rebuild on the Linux server

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Each of the three user stories has a working path |
| Correctness | `generic` reproduces today's build inputs exactly; a contradiction names both values |
| Naming | Model names are lower-case with hyphens and follow the kernel-profile name rule; JSON key is `model` |
| Data flow | The registry is the only reader of model files; no command reads one directly |
| Operator errors and doctor codes | Each refusal is an operator error ending in its fix and names `ze-host`, never `./ze`; `doctor-appliance-model` carries its fix |
| Rule: registration | No switch, list or help string names a model; `model list` output comes from the scan |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Four model files | `ze-host appliance model list` prints four rows |
| `--model` at init | `./le test functional appliance` runs `appliance-model-init` |
| The driver refusal | `TestBuildRefusesModelDriverTheKernelLacks` passes |
| The design page | `docs/architecture/appliance/hardware-models.md` exists and the appliance README table lists it |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The model name and console value are pattern-checked before they reach a path or a kernel command line; a model file path never leaves the model directory |
| Resource limits | A model file is read with a size bound |

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

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| A model is a data file found by scan | Go registration; flags | Same shape as kernel profiles; adding a box edits no code |
| `generic` is an explicit model and the default | No model meaning "unset" | One code path; today's behaviour is stated, not implied by an empty value |
| Required drivers refuse the build | A warning | A box with no management port is found only after install, on site |

## Known Limitations
- The model says nothing about front-panel forwarding. A Broadcom switch installed with Ze has a management port only; that needs a datapath and is not this set's work.
- Matching a running device to its model from DMI is `spec-hardware-capacity-profile.md`.

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
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (N-A: no protocol)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove` the spec's path only, in the same `./le commit create` script
