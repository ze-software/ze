#!/usr/bin/env python3
"""Install BNG Blaster and dependencies on Ubuntu.

Run once on a fresh Ubuntu VM to set up the stress test environment.
Requires root (sudo).

Usage:
    sudo python3 test/stress/setup.py
    sudo python3 test/stress/setup.py --check   # verify installation
"""

import os
import shutil
import subprocess
import sys


def run(cmd, check=True, **kwargs):
    """Run a command, printing it first."""
    print("  $ %s" % " ".join(cmd))
    return subprocess.run(cmd, check=check, **kwargs)


def check_root():
    if os.geteuid() != 0:
        print("error: must run as root (sudo)", file=sys.stderr)
        sys.exit(1)


def install_build_deps():
    """Install build dependencies for libdict and BNG Blaster."""
    print("Installing build dependencies...")
    run(["apt-get", "update", "-qq"])
    run(
        [
            "apt-get",
            "install",
            "-y",
            "--no-install-recommends",
            "build-essential",
            "cmake",
            "git",
            "libncurses-dev",
            "libssl-dev",
            "libjansson-dev",
            "libpcap-dev",
            "python3",
            "python3-pip",
            "python3-venv",
            "iproute2",
            "jq",
        ]
    )


def build_libdict():
    """Build and install libdict from source."""
    src = "/opt/bngblaster-build/libdict"
    if os.path.isfile("/usr/local/lib/libdict.a") or os.path.isfile(
        "/usr/lib/libdict.a"
    ):
        print("libdict already installed, skipping")
        return

    print("Building libdict from source...")
    os.makedirs("/opt/bngblaster-build", exist_ok=True)
    if os.path.isdir(src):
        shutil.rmtree(src)
    run(["git", "clone", "--depth", "1", "https://github.com/rtbrick/libdict.git", src])
    build = os.path.join(src, "build")
    os.makedirs(build, exist_ok=True)
    run(
        [
            "cmake",
            "..",
            "-DCMAKE_BUILD_TYPE=Release",
            "-DCMAKE_INSTALL_PREFIX=/usr/local",
            "-DLIBDICT_TESTS=OFF",
            "-DLIBDICT_TOOLS=OFF",
        ],
        cwd=build,
    )
    run(["cmake", "--build", ".", "-j%d" % os.cpu_count()], cwd=build)
    run(["cmake", "--install", "."], cwd=build)
    run(["ldconfig"])


def build_bngblaster():
    """Build and install BNG Blaster from source."""
    if shutil.which("bngblaster"):
        print("bngblaster already installed, skipping")
        return

    src = "/opt/bngblaster-build/bngblaster"
    print("Building BNG Blaster from source...")
    os.makedirs("/opt/bngblaster-build", exist_ok=True)
    if os.path.isdir(src):
        shutil.rmtree(src)
    run(
        [
            "git",
            "clone",
            "--depth",
            "1",
            "https://github.com/rtbrick/bngblaster.git",
            src,
        ]
    )
    build = os.path.join(src, "build")
    os.makedirs(build, exist_ok=True)
    run(["cmake", "..", "-DCMAKE_BUILD_TYPE=Release"], cwd=build)
    run(["cmake", "--build", ".", "-j%d" % os.cpu_count()], cwd=build)

    # Install binaries.
    bb = os.path.join(build, "code", "bngblaster", "bngblaster")
    run(["install", "-m", "755", bb, "/usr/local/sbin/bngblaster"])
    run(
        [
            "install",
            "-m",
            "755",
            os.path.join(src, "code", "bngblaster-cli"),
            "/usr/local/sbin/bngblaster-cli",
        ]
    )
    run(
        [
            "install",
            "-m",
            "755",
            os.path.join(src, "code", "bgpupdate"),
            "/usr/local/bin/bgpupdate",
        ]
    )


def install_bird():
    """Install BIRD 2.x for baseline comparison tests."""
    if shutil.which("bird"):
        print("bird already installed, skipping")
        return
    print("Installing BIRD 2.x...")
    run(["apt-get", "install", "-y", "--no-install-recommends", "bird2"])


def install_scapy():
    """Install scapy for bgpupdate."""
    print("Installing scapy...")
    run(["pip3", "install", "--break-system-packages", "scapy"], check=False)


def verify():
    """Verify all tools are available."""
    print("")
    print("Verification:")
    ok = True
    for tool in ["bngblaster", "bngblaster-cli", "bgpupdate", "ip", "bird", "birdc"]:
        path = shutil.which(tool)
        if path:
            print("  %s: %s" % (tool, path))
        else:
            print("  %s: NOT FOUND" % tool)
            ok = False

    # Check scapy.
    result = subprocess.run(
        ["python3", "-c", "from scapy.all import *"],
        capture_output=True,
        text=True,
    )
    if result.returncode == 0:
        print("  scapy: ok")
    else:
        print("  scapy: NOT FOUND")
        ok = False

    if ok:
        print("\nAll tools installed.")
    else:
        print("\nSome tools missing.", file=sys.stderr)
        sys.exit(1)


def main():
    if "--check" in sys.argv:
        verify()
        return

    check_root()
    install_build_deps()
    build_libdict()
    build_bngblaster()
    install_bird()
    install_scapy()
    verify()
    print("\nSetup complete. Run stress tests with: make ze-stress-test")


if __name__ == "__main__":
    main()
