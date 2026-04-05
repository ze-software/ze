#!/usr/bin/env python3
"""Ze BGP stress test runner using BNG Blaster.

Usage:
    python3 test/stress/run.py                     # run all scenarios
    python3 test/stress/run.py 01-bulk-ipv4        # run specific scenario
    VERBOSE=1 python3 test/stress/run.py            # verbose output
    NO_BUILD=1 python3 test/stress/run.py           # skip image builds

Environment:
    VERBOSE         - set to 1 for debug output
    NO_BUILD        - set to 1 to skip image builds
    SESSION_TIMEOUT - BGP session timeout in seconds (default: 120)
"""

import os
import subprocess
import sys

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
PROJECT_ROOT = os.path.abspath(os.path.join(SCRIPT_DIR, "..", ".."))

# Make bngblaster module importable from check.py scripts.
sys.path.insert(0, SCRIPT_DIR)

from bngblaster import Scenario, log_fail, log_pass


def build_images(no_build=False):
    """Build ze-interop and ze-stress-bb images."""
    if no_build:
        print("  skipping image builds (NO_BUILD=1)")
        return

    interop_dir = os.path.join(PROJECT_ROOT, "test", "interop")

    print("Building Ze image...")
    subprocess.run(
        ["docker", "build", "-t", "ze-interop",
         "-f", os.path.join(interop_dir, "Dockerfile.ze"),
         PROJECT_ROOT, "-q"],
        check=True, timeout=600,
    )

    print("Building BNG Blaster image...")
    subprocess.run(
        ["docker", "build", "-t", "ze-stress-bb",
         "-f", os.path.join(SCRIPT_DIR, "Dockerfile.bngblaster"),
         SCRIPT_DIR, "-q"],
        check=True, timeout=600,
    )


def main():
    no_build = os.environ.get("NO_BUILD", "0") == "1"

    # Accept a scenario filter as positional argument.
    scenario_filter = ""
    if len(sys.argv) > 1:
        scenario_filter = sys.argv[1]

    # Check Docker is available.
    result = subprocess.run(
        ["docker", "info"],
        capture_output=True, text=True, timeout=15,
    )
    if result.returncode != 0:
        print("error: Docker is not running or not accessible", file=sys.stderr)
        sys.exit(1)

    build_images(no_build)

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

        scenario = Scenario(scenario_dir)
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
