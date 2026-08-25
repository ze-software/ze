#!/usr/bin/env python3
"""Single driver for Ze runtime and installer kernel builds.

This is the one place that:
  - reads the canonical kernel version (internal/appliance/kernel.version),
  - resolves profile config fragments (base + `# ze-base:` + `# ze-include:`),
  - maps the target architecture to a docker platform,
  - selects the docker or qemu backend and invokes build.py,
  - records a per-build version-provenance sidecar (build/kernel.version).

It replaces the docker/qemu invocation that used to be copy-pasted across
gokrazy/kernel/Makefile, tools/installer-kernel/Makefile, and
internal/appliance/cmd_kernel.go. The Makefiles and `ze appliance kernel`
all call this driver with repo-relative directories; the driver self-locates
the repo root from its own path, so no caller passes the version file path.

No shell=True anywhere: every subprocess uses list-form argv.
"""

from __future__ import annotations

import argparse
import os
import shutil
import subprocess
import sys
from pathlib import Path

# Many run.py processes run concurrently under the parallel functional-test
# suite; writing __pycache__/*.pyc for the imports below would have them all
# contend on the same bytecode files. Disable bytecode writes before importing.
sys.dont_write_bytecode = True

# build.py lives next to this file; reuse its token validators so the two
# build-time entry points cannot drift on what they accept.
sys.path.insert(0, str(Path(__file__).resolve().parent))
import build  # noqa: E402

# The single arch -> docker platform map. AC-2: this mapping appears in exactly
# one place in the build tree.
DOCKER_PLATFORM = {
    "amd64": "linux/amd64",
    "x86_64": "linux/amd64",
    "arm64": "linux/arm64",
}

VERSION_FILE_REL = Path("internal/appliance/kernel.version")
COMMON_DIR_REL = Path("tools/kernel-builder/common")
RUNTIME_TARGET = "runtime"
INSTALLER_TARGET = "installer"
PROVENANCE_NAME = "kernel.version"


def fatal(message: str) -> None:
    print(f"FATAL: run.py: {message}", file=sys.stderr)
    raise SystemExit(2)


def repo_root() -> Path:
    here = Path(__file__).resolve().parent
    for parent in [here, *here.parents]:
        if (parent / "go.mod").is_file():
            return parent
    fatal("cannot locate repository root (no go.mod found)")
    raise AssertionError  # unreachable, keeps type checkers happy


def docker_platform(arch: str) -> str:
    platform = DOCKER_PLATFORM.get(arch)
    if platform is None:
        fatal(f"unsupported ARCH={arch} (expected amd64, x86_64, or arm64)")
    return platform


def read_version(explicit: str, version_file: Path) -> str:
    """Single build-time reader of the kernel version.

    Precedence: an explicit --version (used by tests, never by the Makefiles)
    wins; otherwise read the canonical kernel.version file. Format is validated
    here so a malformed version fails before any download or build work.
    """
    if explicit:
        return build.validate_version(explicit)
    if not version_file.is_file():
        fatal(f"kernel version file not found: {version_file}")
    raw = version_file.read_text().strip()
    if not raw:
        fatal(f"kernel version file is empty: {version_file}")
    if "\n" in raw:
        fatal(f"kernel version file {version_file} must contain a single N.N.N line")
    return build.validate_version(raw)


def header_values(config: Path, key: str) -> list[str]:
    """Collect validated profile tokens from `# <key>: <token>` header lines."""
    values: list[str] = []
    for line_no, raw in enumerate(config.read_text().splitlines(), start=1):
        line = raw.strip()
        prefix = f"# {key}:"
        if not line.startswith(prefix):
            continue
        token = line[len(prefix) :].strip()
        if not token:
            fatal(f"{config}:{line_no} empty {key} value")
        values.append(build.validate_profile(token))
    return values


def resolve_fragments(src_dir: Path, common_dir: Path, profile: str) -> list[Path]:
    """The single python-side profile resolver.

    Mirrors internal/appliance/kernelreg.go: base fragment, an optional
    one-level `# ze-base:` parent, the profile fragment, then any
    `# ze-include:` shared fragments (appended once, first-seen order). The
    Go resolver and this one MUST expand identically; the cross-language
    fixture test/install/kernel-shared-fragment.ci asserts it.
    """
    kernel_config = src_dir / "kernel.config"
    profile_config = src_dir / f"{profile}.config"
    build.require_file(kernel_config, "base config fragment")
    build.require_file(src_dir / "kernel.require", "base require manifest")
    build.require_file(profile_config, "profile config fragment")
    build.require_file(src_dir / f"{profile}.require", "profile require manifest")

    fragments = [kernel_config]
    bases = header_values(profile_config, "ze-base")
    if len(bases) > 1:
        fatal(f"profile {profile} declares multiple ze-base values: {bases}")
    if bases:
        base = bases[0]
        base_config = src_dir / f"{base}.config"
        build.require_file(base_config, "base profile config fragment")
        build.require_file(src_dir / f"{base}.require", "base profile require manifest")
        if header_values(base_config, "ze-base"):
            fatal(
                f"profile {profile} base {base} declares ze-base; only one level is supported"
            )
        fragments.append(base_config)
    fragments.append(profile_config)

    includes: list[str] = []
    for fragment in fragments:
        for name in header_values(fragment, "ze-include"):
            if name not in includes:
                includes.append(name)
    for name in includes:
        shared_config = common_dir / f"{name}.config"
        build.require_file(shared_config, "shared include config fragment")
        build.require_file(
            common_dir / f"{name}.require", "shared include require manifest"
        )
        if header_values(shared_config, "ze-include"):
            fatal(
                f"shared fragment {name} declares ze-include; "
                "nested includes are not supported (one level only)"
            )
        fragments.append(shared_config)
    return fragments


def container_fragment_path(fragment: Path, src_dir: Path, common_dir: Path) -> str:
    if fragment.parent == common_dir:
        return f"/builder/common/{fragment.name}"
    try:
        rel = fragment.relative_to(src_dir)
    except ValueError:
        fatal(
            f"fragment {fragment} is outside src dir {src_dir} and common dir {common_dir}"
        )
        raise AssertionError
    return f"/src/{rel.as_posix()}"


def target_label(explicit: str, modules: str) -> str:
    if explicit:
        return explicit
    return RUNTIME_TARGET if modules == "yes" else INSTALLER_TARGET


def write_provenance(
    out_dir: Path,
    version: str,
    arch: str,
    profile: str,
    modules: str,
    builder: str,
    target: str,
) -> None:
    out_dir.mkdir(parents=True, exist_ok=True)
    record = (
        f"version={version}\n"
        f"target={target}\n"
        f"profile={profile}\n"
        f"arch={arch}\n"
        f"modules={modules}\n"
        f"builder={builder}\n"
    )
    (out_dir / PROVENANCE_NAME).write_text(record)


def docker_available() -> bool:
    return shutil.which("docker") is not None


def qemu_available(arch: str) -> bool:
    binary = (
        "qemu-system-aarch64" if arch in ("arm64", "aarch64") else "qemu-system-x86_64"
    )
    return shutil.which("python3") is not None and shutil.which(binary) is not None


def select_builder(requested: str, arch: str) -> str:
    if requested == "docker":
        if not docker_available():
            fatal("docker builder requested but docker not found")
        return "docker"
    if requested == "qemu":
        if not qemu_available(arch):
            fatal("qemu builder requested but qemu/python3 not found")
        return "qemu"
    if requested:
        fatal(f"unsupported builder {requested} (expected docker or qemu)")
    if docker_available():
        return "docker"
    if qemu_available(arch):
        return "qemu"
    fatal("no builder available; install docker or qemu (brew install qemu)")
    raise AssertionError


def repair_out_dir_ownership(out_dir: Path, image: str, platform: str) -> None:
    """Give the container's output back to the invoking user.

    build.py runs as uid 0 inside the container, so everything it writes through
    the `-v {out_dir}:/out` bind mount lands on the host owned root:root. The
    caller is an unprivileged make recipe: `ze-kernel-vmlinuz-stage`
    (mk/build-gokrazy.mk) then cannot `rm -rf` its own scratch view, the
    materialize step leaves an orphaned .copytree-* staging dir, no kernel is
    staged, and ze-qemu-kernel-guard refuses every QEMU target with a message
    about permissions rather than about the kernel. Nothing but `sudo rm` clears
    it, and no automated caller can issue that
    (plan/journal/container-build-leaves-root-owned-scratch.md).

    Running the build container with `--user` does not fix it. The build also
    writes two named volumes, /build and /tmp/kbuild, and docker creates a named
    volume owned root:root 0755. Measured 2026-08-24 on a volume created fresh
    for the probe: an unprivileged uid is denied there on a clean machine
    exactly as on one that has built before, so `--user` denies the build its
    source tree instead of denying it the output. /out is the one mount that
    reaches the host, so repairing /out is the whole of what the host needs.

    Runs after a FAILED build too: a build that dies during modules_install
    leaves a partial root-owned tree, which is the same landmine.
    """
    subprocess.run(
        [
            "docker",
            "run",
            "--rm",
            "--platform",
            platform,
            "-v",
            f"{out_dir}:/out",
            image,
            "chown",
            "-R",
            f"{os.getuid()}:{os.getgid()}",
            "/out",
        ],
        check=True,
    )


def run_docker(
    *,
    version: str,
    arch: str,
    profile: str,
    modules: str,
    jobs: str,
    fragments: list[Path],
    src_dir: Path,
    common_dir: Path,
    out_dir: Path,
    builder_dir: Path,
    patches_dir: Path | None,
    firmware_dir: Path | None,
    image: str,
) -> None:
    platform = docker_platform(arch)
    out_dir.mkdir(parents=True, exist_ok=True)
    subprocess.run(
        ["docker", "build", "--platform", platform, "-t", image, str(builder_dir)],
        check=True,
    )
    args = [
        "docker",
        "run",
        "--rm",
        "--platform",
        platform,
        "-e",
        f"KERNEL_VERSION={version}",
        "-v",
        f"{builder_dir}:/builder:ro",
        "-v",
        f"{src_dir}:/src:ro",
        # The shared common dir is referenced by build.py at /builder/common.
        # When --common-dir is the default (a subdir of builder_dir) this mount
        # is redundant with the /builder mount above; when it points elsewhere
        # this is what makes the shared fragments reachable, so mount it either way.
        "-v",
        f"{common_dir}:/builder/common:ro",
        "-v",
        f"{out_dir}:/out",
        "-v",
        "ze-kernel-build:/tmp/kbuild",
        "-v",
        "ze-kernel-work:/build",
    ]
    if firmware_dir is not None:
        args.extend(["-v", f"{firmware_dir}:/firmware:ro"])
    args.extend(
        [
            image,
            "python3",
            "/builder/build.py",
            "--version",
            version,
            "--arch",
            arch,
            "--profile",
            profile,
            "--modules",
            modules,
            "--out-dir",
            "/out",
        ]
    )
    if jobs:
        args.extend(["--jobs", jobs])
    if patches_dir is not None:
        # patches_dir lives under src_dir (mounted at /src); map it by its
        # path relative to src_dir rather than assuming a direct child.
        patches_rel = patches_dir.relative_to(src_dir).as_posix()
        args.extend(["--patches-dir", f"/src/{patches_rel}"])
    if firmware_dir is not None:
        args.extend(["--firmware-dir", "/firmware"])
    for fragment in fragments:
        args.extend(
            ["--fragment", container_fragment_path(fragment, src_dir, common_dir)]
        )
    try:
        subprocess.run(args, check=True)
    finally:
        repair_out_dir_ownership(out_dir, image, platform)


def run_qemu(
    *,
    version: str,
    arch: str,
    profile: str,
    modules: str,
    jobs: str,
    fragments: list[Path],
    src_rel: str,
    out_rel: str,
    builder_rel: str,
    patches_rel: str,
    firmware_dir: Path | None,
    root: Path,
) -> None:
    script = root / builder_rel / "qemu-build.py"
    args = [
        "python3",
        str(script),
        "--arch",
        arch,
        "--profile",
        profile,
        "--version",
        version,
        "--src-dir",
        src_rel,
        "--out-dir",
        out_rel,
        "--builder-dir",
        builder_rel,
        "--modules",
        modules,
    ]
    if jobs:
        args.extend(["--jobs", jobs])
    if patches_rel:
        args.extend(["--patches-dir", patches_rel])
    if firmware_dir is not None:
        args.extend(["--firmware-dir", str(firmware_dir)])
    for fragment in fragments:
        args.extend(["--fragment", fragment.relative_to(root).as_posix()])
    subprocess.run(args, check=True)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Drive a Ze kernel build (docker or qemu)."
    )
    parser.add_argument("--arch", default="arm64")
    parser.add_argument("--profile", default="qemu")
    parser.add_argument(
        "--src-dir",
        required=True,
        help="repo-relative dir with kernel.config + profile fragments",
    )
    parser.add_argument("--out-dir", required=True, help="repo-relative output dir")
    parser.add_argument(
        "--builder-dir",
        default="tools/kernel-builder",
        help="repo-relative dir with build.py",
    )
    parser.add_argument(
        "--common-dir",
        default=str(COMMON_DIR_REL),
        help="repo-relative dir with shared # ze-include fragments",
    )
    parser.add_argument("--modules", default="no")
    parser.add_argument("--jobs", default="")
    parser.add_argument(
        "--builder",
        default="",
        help="docker or qemu (default: docker if available, else qemu)",
    )
    parser.add_argument(
        "--patches-dir", default="", help="repo-relative patch series dir"
    )
    parser.add_argument(
        "--firmware-dir", default="", help="host path to firmware dir for embedding"
    )
    parser.add_argument(
        "--target",
        default="",
        help="provenance label (installer or runtime); derived from --modules if unset",
    )
    parser.add_argument(
        "--version",
        default="",
        help="explicit version (tests only; Makefiles must not pass this)",
    )
    parser.add_argument(
        "--version-file",
        default="",
        help="override the kernel.version path (tests only)",
    )
    parser.add_argument("--image", default="ze-kernel-builder", help="docker image tag")
    parser.add_argument(
        "--fragment",
        action="append",
        default=[],
        help="explicit fragment list (skips resolution)",
    )
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    root = repo_root()

    arch = args.arch
    if arch not in DOCKER_PLATFORM:
        fatal(f"unsupported ARCH={arch} (expected amd64, x86_64, or arm64)")
    profile = build.validate_profile(args.profile)
    modules = build.validate_modules(args.modules)
    jobs = build.validate_jobs(args.jobs) if args.jobs else ""

    version_file = (
        Path(args.version_file) if args.version_file else (root / VERSION_FILE_REL)
    )
    version = read_version(args.version, version_file)

    src_dir = (root / args.src_dir).resolve()
    common_dir = (root / args.common_dir).resolve()
    out_dir = (root / args.out_dir).resolve()
    builder_dir = (root / args.builder_dir).resolve()
    patches_dir = (root / args.patches_dir).resolve() if args.patches_dir else None
    firmware_dir = Path(args.firmware_dir).resolve() if args.firmware_dir else None

    if args.fragment:
        fragments = [
            Path(p) if Path(p).is_absolute() else (root / p).resolve()
            for p in args.fragment
        ]
    else:
        fragments = resolve_fragments(src_dir, common_dir, profile)
    for fragment in fragments:
        build.require_file(fragment, "config fragment")

    builder = select_builder(args.builder, arch)
    target = target_label(args.target, modules)
    print(
        f">>> building {target} kernel: version={version} arch={arch} profile={profile} builder={builder}"
    )

    if builder == "docker":
        run_docker(
            version=version,
            arch=arch,
            profile=profile,
            modules=modules,
            jobs=jobs,
            fragments=fragments,
            src_dir=src_dir,
            common_dir=common_dir,
            out_dir=out_dir,
            builder_dir=builder_dir,
            patches_dir=patches_dir,
            firmware_dir=firmware_dir,
            image=args.image,
        )
    else:
        run_qemu(
            version=version,
            arch=arch,
            profile=profile,
            modules=modules,
            jobs=jobs,
            fragments=fragments,
            src_rel=args.src_dir,
            out_rel=args.out_dir,
            builder_rel=args.builder_dir,
            patches_rel=args.patches_dir,
            firmware_dir=firmware_dir,
            root=root,
        )

    write_provenance(out_dir, version, arch, profile, modules, builder, target)
    print(
        f">>> {target} kernel ready (version={version}, target={target}, profile={profile})"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
