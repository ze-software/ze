#!/usr/bin/env python3
"""Ze BGP stress test runner using BNG Blaster.

Uses network namespaces and veth pairs. Requires root (sudo) on Linux.
BNG Blaster must be installed first (see setup.py).

Usage:
    sudo python3 test/stress/run.py                     # run all scenarios
    sudo python3 test/stress/run.py 01-bulk-ipv4        # run specific scenario
    sudo VERBOSE=1 python3 test/stress/run.py            # verbose output

Environment:
    VERBOSE         - set to 1 for debug output
    SESSION_TIMEOUT - BGP session timeout in seconds (default: 120)
    ZE_BINARY       - path to ze binary (default: auto-build)
"""

import os
import shutil
import subprocess
import sys

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
PROJECT_ROOT = os.path.abspath(os.path.join(SCRIPT_DIR, "..", ".."))

# Make bngblaster module importable from check.py scripts.
sys.path.insert(0, SCRIPT_DIR)

from bngblaster import Scenario, log_fail, log_pass, log_info


def check_prerequisites():
    """Verify tools are available, run setup if needed."""
    if os.geteuid() != 0:
        print("error: must run as root (sudo) for network namespaces", file=sys.stderr)
        sys.exit(1)

    missing = [t for t in ["bngblaster", "bngblaster-cli", "bgpupdate"]
               if not shutil.which(t)]
    if missing:
        print("Installing BNG Blaster (first run)...")
        setup = os.path.join(SCRIPT_DIR, "setup.py")
        subprocess.run([sys.executable, setup], check=True, timeout=600)
        # Verify after install.
        still_missing = [t for t in missing if not shutil.which(t)]
        if still_missing:
            print("error: setup completed but still missing: %s" % ", ".join(still_missing),
                  file=sys.stderr)
            sys.exit(1)


def build_ze():
    """Build Ze binary, return path."""
    ze_binary = os.environ.get("ZE_BINARY", "")
    if ze_binary and os.path.isfile(ze_binary):
        log_info("using Ze binary: %s" % ze_binary)
        return ze_binary

    # Check for pre-built binary (normal path: Makefile builds before sudo).
    ze_binary = os.path.join(PROJECT_ROOT, "bin", "ze")
    if os.path.isfile(ze_binary):
        log_info("using Ze binary: %s" % ze_binary)
        return ze_binary

    # Build from source (only works when go is in PATH).
    if not shutil.which("go"):
        print("error: bin/ze not found and 'go' not in PATH", file=sys.stderr)
        print("hint: run 'make ze-stress-test' which builds Ze before sudo", file=sys.stderr)
        sys.exit(1)

    print("Building Ze...")
    subprocess.run(
        ["go", "build", "-o", ze_binary, "./cmd/ze"],
        cwd=PROJECT_ROOT, check=True, timeout=120,
    )
    log_pass("Ze built: %s" % ze_binary)
    return ze_binary


def main():
    check_prerequisites()

    # Accept a scenario filter as positional argument.
    scenario_filter = ""
    if len(sys.argv) > 1:
        scenario_filter = sys.argv[1]

    ze_binary = build_ze()

    print("")
    print("\u2501" * 40)
    print(" Ze BGP Stress Tests (BNG Blaster)")
    print("\u2501" * 40)
    print("")

    scenarios_dir = os.path.join(SCRIPT_DIR, "scenarios")
    passed = 0
    failed = 0
    failed_names = []

    for scenario_name in sorted(os.listdir(scenarios_dir)):
        scenario_dir = os.path.join(scenarios_dir, scenario_name)
        if not os.path.isdir(scenario_dir):
            continue

        # Filter if a specific scenario was requested.
        if scenario_filter and scenario_name != scenario_filter:
            continue

        # Skip scenarios without check.py.
        check_path = os.path.join(scenario_dir, "check.py")
        if not os.path.isfile(check_path):
            continue

        print("\u2500\u2500 %s \u2500\u2500" % scenario_name)

        scenario = Scenario(scenario_dir, ze_binary)
        try:
            scenario.setup()
            scenario.run_check()
            log_pass("PASS")
            passed += 1
        except BaseException as e:
            if isinstance(e, KeyboardInterrupt):
                log_fail("INTERRUPTED")
                failed += 1
                failed_names.append(scenario_name)
                scenario.teardown()
                break
            log_fail("FAIL: %s" % e)
            failed += 1
            failed_names.append(scenario_name)
        finally:
            scenario.teardown()

        print("")

    # Warn if filter matched nothing.
    if scenario_filter and passed + failed == 0:
        print("error: no scenario matching '%s' found" % scenario_filter, file=sys.stderr)
        sys.exit(1)

    # Summary.
    print("\u2501" * 40)
    if failed == 0:
        print("\033[32mPASS  %d scenario(s)\033[0m" % passed)
    else:
        print("\033[31mFAIL  %d passed, %d failed: %s\033[0m"
              % (passed, failed, " ".join(failed_names)))
    print("\u2501" * 40)

    sys.exit(0 if failed == 0 else 1)


if __name__ == "__main__":
    main()
