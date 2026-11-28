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
that the builder discovers. It states the installer kernel profile, the serial
console, the firmware modes the box boots in, the kernel drivers the box
cannot work without, and, for a switch, the ONIE platform strings its
installer accepts. A model of one particular box also states its
architecture; `generic` states none, so it serves both. The builder fills the
image options from the model and refuses a config that contradicts it. Every
driver a shipped model names is carried by the shared runtime kernel, and the
registry reports a model whose driver no runtime kernel fragment sets.

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
  → Constraint: "there is no second check and no other kernel to check": the built runtime kernel is proven once, by `enforceKernelRequirements`. A model adds no check of a built kernel.
  → Constraint: "Built in or loadable": a driver for physical hardware is loadable (`=m`), and the device loads it at `ze start` ("Loading driver modules on the device"). The requirement manifests accept only `=y`, so they cannot state a NIC driver.
- [ ] `docs/architecture/appliance/builder.md`
  → Constraint: the appliance config is the unexported `applianceConfig`; `ImageConfig` holds arch, size, kernel profile, hugepages, isolated CPUs, crash dump and memory.
- [ ] `docs/architecture/appliance/iso-installer.md`
  → Constraint: names embedded in a kernel command line pass a dedicated pattern of letters, digits, dot, underscore and hyphen. A model name passes that same pattern. A console value holds a comma (`ttyS0,115200n8`), so it has its own pattern: a device name, a comma, a speed from a fixed list, and optional parity and bits.
  → Constraint: today's installer command line carries the screen console as well as the serial one; a model states the serial console only, and the screen console stays as it is.
- [ ] `plan/post-release/features/spec-hardware-capacity-profile.md`
  → Constraint: that skeleton plans a runtime database matched on DMI facts. The model names of this spec are the names that database uses for the same boxes; neither spec defines a second vocabulary.

**Key insights:**
- No build-time model exists. The serial console is one fixed value in `gokrazy/ze/config.json`.
- The builder already renders a derived gokrazy parent directory per build to carry kernel arguments (`resolveBuildParentDir`), which is where a per-model console can be written without touching the shared config.
- No kernel fragment builds `ixgbe`; `igb` and `igc` are modules (`=m`) in `gokrazy/kernel/runtime.config`, loaded on the device at start.
- The derived build directory is produced by `internal/appliance/instance` (`deriveConfigJSON`), which today patches only the kernel's extra arguments. The packer writes the serial console from the config's `SerialConsole` field. `docs/architecture/vpp-host-tuning.md` still names `materializeDerivedParent`, which no longer exists; that page is corrected in this work.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/appliance/config.go` - `ImageConfig` fields as listed above; `Validate`, `DefaultConfig`, `LoadConfig`.
- [ ] `internal/appliance/cmd_init.go` - `runInit` takes `--config`, `--cert`, `--key`, `--managed`, `--batch`; the interactive path does not ask for a kernel profile.
- [ ] `internal/appliance/kernelreg.go` - `registeredKernelProfiles` scans a directory for `<name>.config`; `resolveKernelProfileFrom` validates the name and requires the paired manifest.
- [ ] `internal/appliance/kernelargs.go` - `resolveBuildParentDir` asks for a derived parent directory carrying hugepage, CPU isolation and crash-dump kernel arguments.
- [ ] `internal/appliance/instance/prepare.go` - `deriveConfigJSON` patches `KernelExtraArgs` only; `instance.Options` has no console field (read by the design judge, 2026-11-28).
- [ ] `internal/appliance/cmd_kernel.go` - `runKernel` reads `image.kernel-profile` for the installer target.
- [ ] `internal/appliance/kernelreq.go` - `readKernelConfig` counts a symbol as enabled only at `=y`; the manifest grammar refuses any other value (read by the design judge).
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
- The shared runtime kernel carries, as loadable modules, every driver a shipped model names, and the registry reports a model whose driver no runtime fragment sets.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Kernel-profile registry | `internal/appliance/kernelreg.go` `registeredKernelProfiles`, `resolveKernelProfileFrom` | Discovers profile file pairs by directory scan and validates the name | extends: the model registry is a second directory scanned by the same rules, and a model names the profile it uses |
| Kernel requirement manifests and the one kernel check | `internal/appliance/kernelreq.go` `enforceKernelRequirements`, the `.require` files | Prove a built kernel carries the `=y` symbols its manifests name | does not apply: a model adds no check of a built kernel, and NIC drivers are `=m`, which the manifests cannot state. The model check reads the runtime fragment files only |
| Loadable hardware drivers | `gokrazy/kernel/runtime.config` (`CONFIG_IGB=m`); `kernel-profiles.md` "Built in or loadable", "Loading driver modules on the device" | Builds physical NIC drivers as modules and loads them at `ze start` | extends: each driver a shipped model names is added to the runtime fragment as a module, and the existing loader loads it |
| Derived instance config | `internal/appliance/instance/prepare.go` `deriveConfigJSON`, `instance.Options` | Copies the gokrazy instance and patches its kernel arguments, for the appliance build and for `le build gokrazy` | extends: `Options` gains the serial console and the patch sets `SerialConsole`; an empty value leaves the field as the checked-in config has it |
| Installer kernel selection | `internal/appliance/cmd_kernel.go` `runKernel`; `cmd_iso.go` `isoKernelCachePath` | Read `image.kernel-profile` | extends: both read the profile through one function that falls back to the model's |
| Image options | `internal/appliance/config.go` `ImageConfig`, `Validate` | Hold and validate per-image facts | extends: gains `model`; the model supplies defaults and `Validate` refuses a contradiction |
| Kernel arguments of a build | `internal/appliance/kernelargs.go` `resolveBuildParentDir` | Computes per-build kernel arguments from the config and asks `instance` for a derived directory | extends: also passes the model's serial console |
| Installer console choice | `internal/appliance/cmd_iso.go` `isoKernelConsoleArgs` | Chooses the console by architecture | replaces: the console comes from the model, and `generic` states today's per-architecture values |
| Runtime hardware facts | `internal/component/host/dmi_linux.go`; `spec-hardware-capacity-profile.md` part P5 | Reads DMI on the device; plans a matched capacity database | does not apply at build time; the names are shared so the two never diverge |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | The core knows nothing, and everything registers | the door every change reads | "It never needs an edit to a switch, a factory, a field list, a" | agrees | A model is a file found by scan; no list, switch or help string names one |
| `docs/architecture/appliance/builder.md` | Kernel command line | `// Design:` page of `config.go`, `cmd_init.go`, `cmd_build.go`, `main.go`; a page this spec modifies | "pure function of the config in `kernelargs.go`, and each answers nil when its" | agrees | The three fields stay pure functions of the config. The console is not a fourth of them: it comes from the model file and travels as its own field of the derived instance config, where it sets `SerialConsole`. With no console stated (`generic`) nothing is patched and the command line is unchanged. The page gains the model and says where the console comes from |
| `ai/doors/appliance.md` | Boundaries | the door covering every `internal/appliance` file | "The `./ze` launcher binary has no `ze_setup`" | agrees | Every refusal and instruction of this spec names `ze-host` |
| `docs/architecture/appliance/build-artifacts.md` | Decisions | `// Design:` page of `doctor_checks.go` | "Doctor checks test for the artifact, not for the build tool." | agrees | The model check tests the model files and the cached kernel's emitted config, never a tool |
| `docs/architecture/appliance/gokrazy-build-pins.md` | Derived parent instances keep the builddir pins | anchored to `cmd_build.go`, `kernelargs.go` | "An appliance config that asks for hugepages cannot be built from the tracked" | agrees | The console travels through the same derived parent, so the pins this page protects are kept by the existing path |
| `docs/architecture/appliance/ota-push.md` | Decisions | anchored to `cmd_build.go` | | silent | The page covers what a push sends to a device; a model changes nothing a push sends or validates |
| `docs/architecture/vpp-host-tuning.md` | Kernel arguments go through a derived instance config | `// Design:` page of `kernelargs.go` | "The checked-in `gokrazy/ze/config.json` is never edited." | agrees | The model's console is patched into the derived config; the checked-in file keeps its one console value. The page names `materializeDerivedParent`, which no longer exists (the producer is `instance.deriveConfigJSON`); this work corrects the name and adds the console as a second patched field |
| `docs/architecture/appliance/kernel-profiles.md` | Decisions | `// Design:` page of `cmd_kernel.go`; anchored to `kernelargs.go`; a page this spec modifies | "there is no second check and no other kernel to check." | agrees | The built runtime kernel keeps its one check. The model comparison reads the fragment files and never a built kernel, so it is no second check of that kernel. Under "Built in or loadable" a physical NIC driver is a module, and that is how each model's driver is added. The page gains the model registry beside the profile registry, and the rule that a shipped model's drivers are in the runtime fragment |
| `docs/architecture/appliance/iso-installer.md` | Traps | `// Design:` page of `cmd_iso.go`; a page this spec modifies | "Names embedded in the GRUB kernel command line must survive two parsers." | agrees | The page's pattern of letters, digits, dot, underscore and hyphen governs names; the model name passes it. A console value holds a comma, so it is checked by its own stricter-shaped pattern (device, comma, a speed from a fixed list, optional parity and bits), which also admits no space, quote or shell character. The page gains that pattern, the statement that the installer's serial console comes from the model, and the refusal of an ISO for a BIOS-only model, which follows from the page's own UEFI decision |
| `docs/architecture/appliance-serial-login.md` | Appliance Serial Console Login | a page this spec modifies | "The gokrazy serial console (115200 baud on ttyS0) is the emergency access path." | departs | The page states one console for every appliance. After this work the serial console is the one the appliance's model states. See "Documents departed from" |
| `ai/doors/le-tooling.md` | Where it sits | the door covering the `.ci` files | "Functional `.ci` files live under `test/<suite>/` and run through" | agrees | The four new tests are `.ci` files in the existing `test/appliance/` suite |
| `docs/architecture/vpp-host-tuning.md` | Design decisions | decision of 2026-11-23 | "When Ze does not believe the host has enough resources for VPP to remain stable, it warns and still starts VPP. The requirement changes with the configuration and with what else runs on the host (on a general Linux install other programs take memory too), so the check runs at every start against the machine as it is then, and a warning tells the operator they are taking a risk. A measurement the operator recorded on their own hardware stops the warning on that hardware, and the operator can silence the warning with an environment value, set in the configuration or in the OS environment" | agrees | This spec adds no check of host resources and changes no VPP start behaviour; its one refusal is at build time, for a kernel that lacks a driver, which that decision does not cover |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `ze-host appliance init --model <name> <appliance>`, or `image.model` in the JSON given to `--config`.
- Format at entry: a model name, validated against the names found in the model directory.

### Transformation Path
1. The model directory is scanned; each file is parsed and validated into a model value.
2. `init` resolves the named model and writes the appliance config with `image.model` set and the model's architecture and kernel profile filled in where the operator gave none.
3. `Validate` reloads the model at every later command and refuses a config whose architecture contradicts it.
4. `build` passes the model's console to the derived instance config, where it becomes `SerialConsole`.
5. The registry compares each model's driver symbols with the runtime kernel fragment files and reports a symbol no fragment sets, at every scan and in the doctor check.
6. `iso`, `kernel` (and later `onie`) read the model for the installer console and the installer kernel profile. `iso` refuses a model whose firmware modes do not include UEFI, because the ISO boots under UEFI only.

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
| appliance config JSON, `image.model` | reader | `internal/appliance/config.go` `LoadConfig`, `Validate` | every command loads through `LoadConfig`, which refuses unknown fields | an existing config without the field keeps loading and building unchanged; a config with the field is refused by an older builder, which is the intended outcome |
| appliance config JSON, `image.model` | reader | `internal/appliance/cmd_kernel.go` `runKernel`, `cmd_iso.go` `resolveISOInput` | both read `cfg.Image.KernelProfile` | changed: both take the profile through the model-aware accessor |
| appliance config JSON, `image.model` | writer | `internal/le/test/qemu/install_build.go`, which rewrites the `image` object for the installer proofs | found by the design judge | preserved: it writes no model, so its appliances are `generic` |
| appliance config JSON, `image.model` | reader | the export and import archive (`cmd_export.go`, `cmd_import.go`) | carries the config file whole | preserved: the field travels with the file |
| appliance config JSON, `image.model` | reader | the examples in `docs/guide/appliance.md`, `docs/guide/ze-install.md` and `website/use-cases/appliance-lifecycle/index.md` | prose carriers | checked in the documentation phase; none becomes wrong, since the field is optional |
| appliance config JSON, `image.model` | writer | an operator's own `--config` and `--batch` files, written by hand | `cmd_init.go` reads both | unchanged files stay valid |
| model file | writer | a file under the model directory, written by hand | new | none to migrate |
| model file | reader | the model registry of this spec | new | none to migrate |

### Integration Points
- `registeredKernelProfiles` and its name validation - the model registry reuses the scan and the name rule.
- `instance.Options` and `deriveConfigJSON` - gain the console.
- The runtime kernel fragments - gain each named driver as a module.

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
| A-1 | The console can be set per build through the derived parent directory, without editing the shared gokrazy config | `internal/appliance/instance/prepare.go` `deriveConfigJSON` writes a patched `config.json` (read by the design judge); the packer's `writeCmdline` emits the console from that file's `SerialConsole` | The console needs another route into the packer | A unit test of `deriveConfigJSON` with a console, and the built image's command line in `TestBuildRendersModelConsole` | unvalidated |
| A-2 | The S4048-ON and S5232F-ON facts in their model files match the units | Upstream ONIE and SONiC platform files only (umbrella, "The two switches") | A model accepts the wrong platform string or names the wrong console or driver | The switch commands in the umbrella, run by Thomas; the model files are corrected from their output before this spec closes | unvalidated |
| A-3 | The N100 box needs the `hardware-kms` installer profile and boots under UEFI | `tools/installer-kernel/README.md` and `hardware-kms.config` name the N100 display loss | The `n100` model picks the wrong profile | Thomas confirms the profile he uses for the N100 | unvalidated |
| A-4 | The amd64 runtime kernel lacks `ixgbe` | Only an arm64 emitted config was cached where the research ran; no fragment names it | The driver refusal for the S5232F-ON never fires, harmlessly | Read the amd64 runtime kernel's emitted config on the Linux server | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A model silently changes an existing appliance's image | A rebuilt image differs with no config change | `generic` is the default and states today's values; a test compares the build inputs of a config with and without `generic` |
| R-2 | A model and an explicit option disagree and the builder picks one silently | An image built for the wrong architecture | `Validate` refuses the disagreement and names both values |
| R-3 | A model names a driver that no runtime fragment sets | The installed box has no management port | The registry test fails for a shipped model, and the doctor check and `model show` report it for any model, naming the symbol and the fragment file to add it to as a module |
| R-6 | A named module is built but never loaded on the device | The interface is absent after boot | The existing loader loads modules for the hardware it finds; the QEMU proofs cannot show a physical NIC, so each unit's first boot is the evidence, recorded on the feature page |
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
| A shipped model names a driver | → | the registry's comparison with the runtime fragments | `TestEveryShippedModelDriverIsInTheRuntimeFragments` |
| `ze-host appliance iso` for a BIOS-only model | → | the firmware-mode refusal | `test/appliance/appliance-model-iso-needs-uefi.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `init --model n100 lab` with no other image option | The written config holds `image.model` `n100`, and every later command uses the architecture and the installer kernel profile the `n100` model states | An operator's explicit kernel profile survives, because a profile is a choice within a model, not a fact of it |
| AC-2 | `init` with no `--model` and a config with no `image.model` | The appliance is created with model `generic`, which states no architecture, no profile and no console, so `DefaultConfig`'s values apply and the build inputs equal those of the same config before this work, on amd64 and on arm64 | Every existing option survives unchanged, because `generic` adds nothing |
| AC-3 | `init --model no-such-box lab` | The command exits non-zero and prints the unknown name, the list of known models, and the `ze-host appliance model list` command | No appliance directory survives, because a config with an unknown model could never build |
| AC-4 | A config whose `image.arch` differs from the architecture its model states | `Validate` refuses at every command, naming both values and the edit that fixes it | A config whose architecture agrees survives; so does any architecture under a model that states none (`generic`) |
| AC-5 | A new model file is dropped into the model directory | `model list` shows it and `init --model` accepts it, with no other file edited | No survivor question: nothing is removed |
| AC-6 | A model names a driver symbol that no runtime kernel fragment sets to `=y` or `=m` | `model show` and the doctor check name the model, the symbol, and the fragment file to add it to as a module; for a shipped model the registry test fails | A model whose every symbol is set is reported clean, and the built kernel is still proven once, by the existing requirement check, which this spec leaves alone |
| AC-12 | `iso` of an appliance whose model lists firmware modes without UEFI | The command refuses, names the model and its modes, and says the ISO boots under UEFI only | A model that lists UEFI, and `generic`, which lists both, build an ISO as today |
| AC-13 | `ze-host appliance kernel` for an appliance whose config holds a model and no profile | The command builds the installer kernel profile the model states, the same one `iso` then uses | An explicit `--profile` or `image.kernel-profile` survives and wins |
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
| `TestEveryShippedModelDriverIsInTheRuntimeFragments` | `internal/appliance/modelreg_test.go` | AC-6 for the shipped set | |
| `TestModelReportsADriverNoFragmentSets` | `internal/appliance/modelreg_test.go` | AC-6 for an added model | |
| `TestDeriveConfigSetsSerialConsole` | `internal/appliance/instance/prepare_test.go` | The console reaches `SerialConsole`; an empty value leaves the field | |
| `TestKernelCommandUsesTheModelProfile` | `internal/appliance/cmd_kernel_test.go` | AC-13 | |
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
| `appliance-model-iso-needs-uefi` | `test/appliance/appliance-model-iso-needs-uefi.ci` | An ISO for a BIOS-only model is refused with the reason | |

### Interop Tests (Scope: protocol)
N-A: no wire protocol and no peer. The ONIE proof belongs to `spec-onie-3-installer.md`.

## Files to Modify
- `internal/appliance/config.go` - `image.model`, defaults from the model, the contradiction refusal
- `internal/appliance/cmd_init.go` - `--model`, the interactive prompt, batch entries
- `internal/appliance/kernelargs.go` - the console passed to the derived instance
- `internal/appliance/instance/prepare.go` - `Options` gains the console; the patch sets `SerialConsole`
- `internal/appliance/cmd_iso.go` - console and installer profile from the model; the UEFI refusal
- `internal/appliance/cmd_kernel.go` - the installer profile from the model
- `internal/appliance/main.go` - the `model` command row
- `internal/appliance/doctor_checks.go` - the model registry check
- `gokrazy/kernel/runtime.config` - each driver a shipped model names, as a module
- `docs/architecture/appliance/builder.md`, `kernel-profiles.md`, `iso-installer.md` - the model, and the console no longer chosen by architecture
- `docs/architecture/vpp-host-tuning.md` - the stale `materializeDerivedParent` name, and the console as a second patched field
- `docs/architecture/appliance-serial-login.md` - the console is the model's, no longer one fixed value
- `docs/guide/appliance.md` - naming a model

## Files to Create
- `internal/appliance/modelreg.go` - the model registry: scan, parse, validate
- `internal/appliance/cmd_model.go` - `model list` and `model show`
- `tools/appliance-models/generic.model`, `n100.model`, `dell-s4048-on.model`, `dell-s5232f-on.model` - the first models
- `tools/appliance-models/README.md` - the file format and how to add a model
- `docs/architecture/appliance/hardware-models.md` - the design page
- `test/appliance/appliance-model-init.ci`, `appliance-model-unknown.ci`, `appliance-model-list.ci`, `appliance-model-arch-mismatch.ci`, `appliance-model-iso-needs-uefi.ci`

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
| Doctor code per problem class | Yes | `doctor-appliance-model`, build-host phase: a model file that does not parse, and a model that names a driver no runtime fragment sets |
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
contradicts its model. The shared runtime kernel carries the drivers the
shipped models name, and the builder reports a model whose driver it lacks. A
model is one file in `tools/appliance-models/`; adding a box is adding a file.
An appliance with no model is `generic` and builds exactly as today.

### Before and after

1. Creating an appliance for an N100. Before: the operator writes a JSON file setting the kernel profile and passes it with `--config`, and the console cannot be chosen. After: `ze-host appliance init --model n100 lab`.
2. Listing what the builder knows. Before: nothing to list. After: `ze-host appliance model list` prints `generic`, `n100`, `dell-s4048-on` and `dell-s5232f-on`, each with its architecture and a one-line description.
3. An unknown model. After: `init --model s4048` exits non-zero and says the model is unknown, lists the four known names, and names `ze-host appliance model list`.
4. A model whose driver the runtime kernel does not carry. Before: nothing says so, and the installed box has no management port. After: `ze-host appliance model show <name>` and the doctor check name the driver and the fragment file to add it to as a module.
5. An ISO for a box that boots under legacy BIOS only. After: `ze-host appliance iso` refuses and says the ISO boots under UEFI only.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| One builder flag per fact (`--console`, `--firmware`, `--onie-platform`) | The operator would still have to know every fact of the box, which is what Thomas asked to stop |
| Models as Go values registered from `init()` | A model is data with no behaviour; the kernel-profile registry already discovers data files by scan, and a second way to register the same kind of thing would sit beside it |
| Detect the model on the device from DMI at install time | The builder must know the model before any device exists, to choose the kernel profile and to stamp the installer; runtime matching is the capacity-profile spec's subject |
| Fold the model into the kernel profile | A profile is a kernel build; one profile serves several models, and a model carries facts no kernel build has (firmware mode, ONIE strings) |
| Check each model's drivers against the built runtime kernel at `build` | The kernel-profiles page states there is one check of the runtime kernel and no second. NIC drivers are also modules, which the requirement manifests cannot state. Comparing the model with the fragment files reports the same mistake earlier and adds no second kernel check |

### Documents departed from

`docs/architecture/appliance-serial-login.md`, opening paragraph. Before: "The gokrazy serial console (115200 baud on ttyS0) is the emergency access path." After: the serial console is the emergency access path, on the device and at the speed the appliance's model states, which is ttyS0 at 115200 for `generic` and for every appliance built before this work. What an operator sees: nothing changes unless they name a model whose console differs; then the login gate appears on that model's console instead of on a port the box may not have.

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
   - Tests: `TestBuildRendersModelConsole`, `TestDeriveConfigSetsSerialConsole`, `TestEveryShippedModelDriverIsInTheRuntimeFragments`, `TestModelReportsADriverNoFragmentSets`, `TestISOConsoleFollowsModel`, `TestKernelCommandUsesTheModelProfile`, `appliance-model-iso-needs-uefi.ci`
   - Files: `kernelargs.go`, `instance/prepare.go`, `cmd_iso.go`, `cmd_kernel.go`, `runtime.config`, `kernel-profiles.md`, `iso-installer.md`, `vpp-host-tuning.md`, `appliance-serial-login.md`, `docs/guide/appliance.md`
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
| The driver report | `TestEveryShippedModelDriverIsInTheRuntimeFragments` and `TestModelReportsADriverNoFragmentSets` pass |
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
| A model's drivers are compared with the runtime fragments, and the shipped kernel carries them | A check of the built kernel at build | One kernel check exists already and the page forbids a second; the fragment comparison reports the gap without building anything |

## Known Limitations
- The model says nothing about front-panel forwarding. A Broadcom switch installed with Ze has a management port only; that needs a datapath and is not this set's work.
- Matching a running device to its model from DMI is `spec-hardware-capacity-profile.md`.
- The PXE installer's console is chosen per architecture by the image server (`internal/plugins/imageserver/handler.go`) and does not follow the model: a PXE server serves many appliances and knows none of them. A box whose console differs from the default is installed from the ISO or from ONIE.
- The installer kernel's own driver set is the profile's business and is proven by the profile's existing manifest; a model names runtime drivers only.

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
