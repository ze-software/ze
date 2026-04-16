#!/usr/bin/env python3
"""Install stress-test dependencies on Ubuntu.

Run once on a fresh Ubuntu VM. Requires root (sudo).

The stress test harness uses:
  - `bin/ze-test peer --mode inject` (in-tree, built via `make ze-test`)
  - BIRD 2.x (for the scenario-04 baseline)
  - iproute2, ethtool (for netns / veth setup)

No external BGP traffic generator is required; the BNG Blaster /
libdict / scapy dependencies were removed in favour of the in-tree Go
injector.

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
    """Install runtime dependencies (iproute2, ethtool, jq)."""
    print("Installing runtime dependencies...")
    run(["apt-get", "update", "-qq"])
    run(
        [
            "apt-get",
            "install",
            "-y",
            "--no-install-recommends",
            "iproute2",
            "ethtool",
            "tcpdump",
            "jq",
        ]
    )


def install_bird():
    """Install BIRD 2.x for the scenario-04 baseline."""
    if shutil.which("bird"):
        print("bird already installed, skipping")
        return
    print("Installing BIRD 2.x...")
    run(["apt-get", "install", "-y", "--no-install-recommends", "bird2"])


def verify():
    """Verify all tools are available."""
    print("")
    print("Verification:")
    ok = True
    for tool in ["ip", "ethtool", "bird", "birdc"]:
        path = shutil.which(tool)
        if path:
            print("  %s: %s" % (tool, path))
        else:
            print("  %s: NOT FOUND" % tool)
            ok = False

    if ok:
        print("\nAll runtime tools installed.")
        print("Build the harness binary with: make ze-test")
    else:
        print("\nSome tools missing.", file=sys.stderr)
        sys.exit(1)


def main():
    if "--check" in sys.argv:
        verify()
        return

    check_root()
    install_build_deps()
    install_bird()
    verify()
    print("\nSetup complete. Run stress tests with: make ze-stress-test")


if __name__ == "__main__":
    main()
