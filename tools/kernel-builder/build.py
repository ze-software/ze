#!/usr/bin/env python3
"""Build a Ze runtime or installer kernel from resolved config fragments."""

from __future__ import annotations

import argparse
import os
import shutil
import subprocess
import sys
import tarfile
import urllib.request
from pathlib import Path

import ksource

DEFAULT_WORK_DIR = Path("/build")
DEFAULT_BUILD_DIR = Path("/tmp/kbuild")
HARDWARE_KMS_PROFILE = "hardware-kms"
I915_FIRMWARE = "i915/adlp_dmc.bin"


def fatal(message: str) -> None:
    print(f"FATAL: {message}", file=sys.stderr)
    raise SystemExit(2)


def validate_version(value: str) -> str:
    if (
        value == ""
        or value.startswith(".")
        or value.endswith(".")
        or any(ch not in "0123456789." for ch in value)
    ):
        fatal(f"unsupported KERNEL_VERSION={value} (expected digits and dots)")
    major = int(value.split(".", 1)[0])
    if major < 7:
        fatal("kernel >= 7.0 required (L2TP_NETLINK removed, serial 8250 deps changed)")
    return value


def validate_profile(value: str) -> str:
    if not value or value[0] not in "abcdefghijklmnopqrstuvwxyz0123456789":
        fatal(f"unsupported PROFILE={value} (expected ^[a-z0-9][a-z0-9-]*$)")
    for ch in value:
        if ch not in "abcdefghijklmnopqrstuvwxyz0123456789-":
            fatal(f"unsupported PROFILE={value} (expected ^[a-z0-9][a-z0-9-]*$)")
    return value


def validate_arch(value: str) -> tuple[str, str, str]:
    if value == "arm64":
        return "arm64", "arch/arm64/boot/Image", "Image"
    if value in ("amd64", "x86_64"):
        return "x86_64", "arch/x86/boot/bzImage", "bzImage"
    fatal(f"unsupported ARCH={value} (expected arm64, amd64, or x86_64)")


def validate_modules(value: str) -> str:
    if value not in ("yes", "no"):
        fatal(f"unsupported MODULES={value} (expected yes or no)")
    return value


def validate_jobs(value: str) -> str:
    if value and any(ch not in "0123456789" for ch in value):
        fatal(f"unsupported JOBS={value} (expected digits)")
    return value or str(os.cpu_count() or 1)


def require_file(path: Path, label: str) -> None:
    if not path.is_file():
        fatal(f"missing {label}: {path}")


def read_base_header(config: Path) -> str:
    base = ""
    for line_no, raw in enumerate(config.read_text().splitlines(), start=1):
        line = raw.strip()
        if not line.startswith("# ze-base:"):
            continue
        value = line.removeprefix("# ze-base:").strip()
        if not value:
            fatal(f"{config}:{line_no} empty ze-base value")
        if base:
            fatal(f"{config}:{line_no} duplicate ze-base value")
        base = validate_profile(value)
    return base


def resolve_profile_fragments(src_dir: Path, profile: str) -> list[Path]:
    kernel_config = src_dir / "kernel.config"
    kernel_require = src_dir / "kernel.require"
    profile_config = src_dir / f"{profile}.config"
    profile_require = src_dir / f"{profile}.require"
    require_file(kernel_config, "base config fragment")
    require_file(kernel_require, "base require manifest")
    require_file(profile_config, "profile config fragment")
    require_file(profile_require, "profile require manifest")

    fragments = [kernel_config]
    base = read_base_header(profile_config)
    if base:
        base_config = src_dir / f"{base}.config"
        base_require = src_dir / f"{base}.require"
        require_file(base_config, "base profile config fragment")
        require_file(base_require, "base profile require manifest")
        nested = read_base_header(base_config)
        if nested:
            fatal(
                f"profile {profile} base {base} declares ze-base {nested}; "
                "only one level of ze-base is supported"
            )
        fragments.append(base_config)
    fragments.append(profile_config)
    return fragments


def required_symbols_for_fragments(fragments: list[Path]) -> list[str]:
    symbols: list[str] = []
    seen: set[str] = set()
    for fragment in fragments:
        manifest = fragment.with_suffix(".require")
        require_file(manifest, "require manifest")
        for line_no, raw in enumerate(manifest.read_text().splitlines(), start=1):
            line = raw.strip()
            if not line or line.startswith("#"):
                continue
            if line.endswith("=y"):
                line = line.removesuffix("=y")
            elif "=" in line:
                fatal(
                    f"{manifest}:{line_no} require entries must be CONFIG_SYMBOL or CONFIG_SYMBOL=y"
                )
            if not line.startswith("CONFIG_") or any(ch in line for ch in "/\\"):
                fatal(f"{manifest}:{line_no} invalid required symbol {line!r}")
            if line not in seen:
                seen.add(line)
                symbols.append(line)
    return symbols


def enforce_required_symbols(
    build_tree: Path, profile: str, fragments: list[Path]
) -> None:
    config = build_tree / ".config"
    require_file(config, "resolved kernel config")
    required = required_symbols_for_fragments(fragments)
    enabled: set[str] = set()
    for raw in config.read_text().splitlines():
        if not raw.startswith("CONFIG_"):
            continue
        key, sep, value = raw.partition("=")
        if sep and value == "y":
            enabled.add(key)
    for symbol in required:
        if symbol not in enabled:
            fatal(
                f"kernel profile {profile}: {symbol} did not resolve to =y in {config}"
            )


def safe_extract(tar: tarfile.TarFile, dest: Path) -> None:
    root = dest.resolve()
    for member in tar.getmembers():
        target = (dest / member.name).resolve()
        if target != root and root not in target.parents:
            fatal(f"kernel tarball contains unsafe path: {member.name}")
    tar.extractall(dest)


def download_tarball(version: str, work_dir: Path) -> Path:
    tarball = ksource.tarball_name(version)
    path = work_dir / tarball
    if path.is_file():
        print(f">>> using pre-downloaded {tarball}")
        return path

    url = ksource.tarball_url(version)
    tmp = path.with_suffix(path.suffix + ".part")
    print(f">>> downloading linux {version}")
    try:
        with urllib.request.urlopen(url) as response, tmp.open("wb") as out:
            shutil.copyfileobj(response, out)
        tmp.replace(path)
    except Exception:
        tmp.unlink(missing_ok=True)
        raise
    return path


def restore_or_extract_tree(
    version: str,
    profile: str,
    modules: str,
    patches_dir: Path | None,
    work_dir: Path,
    build_dir: Path,
    tarball: Path,
) -> Path:
    build_dir.mkdir(parents=True, exist_ok=True)
    build_tree = build_dir / f"linux-{version}-{profile}-{modules}"
    cache_tar = work_dir / f"linux-{version}-{profile}-{modules}.built.tar"
    if patches_dir is not None:
        shutil.rmtree(build_tree, ignore_errors=True)
    elif (build_tree / "scripts/Kbuild.include").is_file():
        print(f">>> reusing existing source tree {build_tree}")
    elif cache_tar.is_file():
        print(f">>> restoring cached build tree from {cache_tar.name}")
        with tarfile.open(cache_tar, "r") as tar:
            safe_extract(tar, build_dir)

    if not (build_tree / "scripts/Kbuild.include").is_file():
        shutil.rmtree(build_tree, ignore_errors=True)
        shutil.rmtree(build_dir / f"linux-{version}", ignore_errors=True)
        print(f">>> extracting to {build_tree}")
        with tarfile.open(tarball, "r:xz") as tar:
            safe_extract(tar, build_dir)
        (build_dir / f"linux-{version}").rename(build_tree)
    return build_tree


def apply_patches(build_tree: Path, patches_dir: Path | None) -> None:
    if patches_dir is None:
        return
    series = patches_dir / "series"
    require_file(series, "patch series")
    print(f">>> applying patches from {patches_dir}")
    for raw in series.read_text().splitlines():
        patch_name = raw.strip()
        if not patch_name or patch_name.startswith("#"):
            continue
        if "/" in patch_name or ".." in patch_name:
            fatal(f"invalid patch name in series: {patch_name}")
        patch_file = patches_dir / patch_name
        require_file(patch_file, "patch")
        with patch_file.open("rb") as stdin:
            subprocess.run(["patch", "-p1"], cwd=build_tree, stdin=stdin, check=True)


def run_make(build_tree: Path, *args: str) -> None:
    subprocess.run(["make", *args], cwd=build_tree, check=True)


def merge_config(
    build_tree: Path, kernel_arch: str, arch: str, profile: str, fragments: list[Path]
) -> None:
    print(f">>> configuring (defconfig + resolved {profile} profile) for {arch}")
    run_make(build_tree, f"ARCH={kernel_arch}", "defconfig")
    merge = build_tree / "scripts/kconfig/merge_config.sh"
    subprocess.run(
        [str(merge), "-m", ".config", *[str(p) for p in fragments]],
        cwd=build_tree,
        check=True,
    )


def embed_firmware(build_tree: Path, profile: str, firmware_dir: Path | None) -> None:
    if profile != HARDWARE_KMS_PROFILE:
        return
    if firmware_dir is None:
        fatal(
            f"profile {HARDWARE_KMS_PROFILE} requires --firmware-dir with {I915_FIRMWARE}"
        )
    blob = firmware_dir / I915_FIRMWARE
    require_file(blob, "firmware")
    print(f">>> embedding firmware from {firmware_dir}")
    print(f"  {I915_FIRMWARE}: {blob.stat().st_size} bytes")
    with (build_tree / ".config").open("a") as config:
        config.write(f'CONFIG_EXTRA_FIRMWARE="{I915_FIRMWARE}"\n')
        config.write(f'CONFIG_EXTRA_FIRMWARE_DIR="{firmware_dir}"\n')


def build_kernel(
    build_tree: Path, kernel_arch: str, make_target: str, jobs: str, modules: str
) -> None:
    print(f">>> building {make_target} with -j{jobs} (modules={modules})")
    if modules == "yes":
        run_make(build_tree, f"ARCH={kernel_arch}", f"-j{jobs}", make_target, "modules")
    else:
        run_make(build_tree, f"ARCH={kernel_arch}", f"-j{jobs}", make_target)


def copy_runtime_outputs(
    build_tree: Path, image_path: str, out_dir: Path, kernel_arch: str
) -> None:
    shutil.copy2(build_tree / image_path, out_dir / "vmlinuz")
    run_make(
        build_tree,
        f"ARCH={kernel_arch}",
        f"INSTALL_MOD_PATH={out_dir}",
        "modules_install",
    )
    modules_dir = out_dir / "lib/modules"
    if modules_dir.is_dir():
        for name in ("build", "source"):
            for path in modules_dir.glob(f"*/{name}"):
                path.unlink(missing_ok=True)
    dts_dir = build_tree / "arch/arm64/boot/dts"
    if kernel_arch == "arm64" and dts_dir.is_dir():
        for dtb in dts_dir.rglob("*.dtb"):
            shutil.copy2(dtb, out_dir / dtb.name)
    for overlay_src in (build_tree / "overlays", dts_dir / "overlays"):
        if overlay_src.is_dir():
            overlay_dst = out_dir / "overlays"
            shutil.rmtree(overlay_dst, ignore_errors=True)
            shutil.copytree(overlay_src, overlay_dst)
            break
    print(f">>> done: {out_dir / 'vmlinuz'}")


def copy_installer_outputs(
    build_tree: Path,
    image_path: str,
    out_dir: Path,
    work_dir: Path,
    build_dir: Path,
    version: str,
    profile: str,
    modules: str,
    patches_dir: Path | None,
) -> None:
    shutil.copy2(build_tree / image_path, out_dir / "Image")
    if patches_dir is None:
        cache_tar = work_dir / f"linux-{version}-{profile}-{modules}.built.tar"
        print(f">>> caching build tree to {cache_tar}")
        with tarfile.open(cache_tar, "w") as tar:
            tar.add(build_tree, arcname=build_tree.name)
    size_mib = (out_dir / "Image").stat().st_size // (1024 * 1024)
    print(f">>> done: {out_dir / 'Image'} ({size_mib} MiB, profile={profile})")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Build a Ze kernel from config fragments."
    )
    parser.add_argument(
        "--version",
        default=os.environ.get("KERNEL_VERSION"),
        required="KERNEL_VERSION" not in os.environ,
    )
    parser.add_argument("--arch", default=os.environ.get("ARCH", "arm64"))
    parser.add_argument("--profile", default=os.environ.get("PROFILE", "qemu"))
    parser.add_argument("--jobs", default=os.environ.get("JOBS", ""))
    parser.add_argument("--src-dir", default=os.environ.get("SRC_DIR", "/src"))
    parser.add_argument("--out-dir", default=os.environ.get("OUT_DIR", "/out"))
    parser.add_argument("--modules", default=os.environ.get("MODULES", "no"))
    parser.add_argument("--patches-dir", default=os.environ.get("PATCHES_DIR", ""))
    parser.add_argument("--firmware-dir", default=os.environ.get("FIRMWARE_DIR", ""))
    parser.add_argument("--fragment", action="append", default=[])
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    version = validate_version(args.version)
    kernel_arch, image_path, make_target = validate_arch(args.arch)
    profile = validate_profile(args.profile)
    modules = validate_modules(args.modules)
    jobs = validate_jobs(args.jobs)
    src_dir = Path(args.src_dir)
    out_dir = Path(args.out_dir)
    patches_dir = Path(args.patches_dir) if args.patches_dir else None
    firmware_dir = Path(args.firmware_dir) if args.firmware_dir else None
    fragments = [Path(p) for p in args.fragment]
    if not fragments:
        fragments = resolve_profile_fragments(src_dir, profile)
    for fragment in fragments:
        require_file(fragment, "config fragment")
    if patches_dir is not None:
        require_file(patches_dir / "series", "patch series")

    work_dir = Path(os.environ.get("WORK_DIR", str(DEFAULT_WORK_DIR)))
    build_dir = Path(os.environ.get("BUILD_DIR", str(DEFAULT_BUILD_DIR)))
    work_dir.mkdir(parents=True, exist_ok=True)
    tarball = download_tarball(version, work_dir)
    build_tree = restore_or_extract_tree(
        version, profile, modules, patches_dir, work_dir, build_dir, tarball
    )
    apply_patches(build_tree, patches_dir)
    merge_config(build_tree, kernel_arch, args.arch, profile, fragments)
    embed_firmware(build_tree, profile, firmware_dir)
    run_make(build_tree, f"ARCH={kernel_arch}", "olddefconfig")
    enforce_required_symbols(build_tree, profile, fragments)
    build_kernel(build_tree, kernel_arch, make_target, jobs, modules)

    out_dir.mkdir(parents=True, exist_ok=True)
    shutil.copy2(build_tree / ".config", out_dir / "config")
    if modules == "yes":
        copy_runtime_outputs(build_tree, image_path, out_dir, kernel_arch)
    else:
        copy_installer_outputs(
            build_tree,
            image_path,
            out_dir,
            work_dir,
            build_dir,
            version,
            profile,
            modules,
            patches_dir,
        )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
