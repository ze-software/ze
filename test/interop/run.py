#!/usr/bin/env python3
"""Ze interoperability test runner.

Usage:
    python3 test/interop/run.py                     # run all scenarios
    python3 test/interop/run.py 01-ebgp-ipv4-frr    # run specific scenario
    VERBOSE=1 python3 test/interop/run.py            # verbose output

Environment:
    FRR_IMAGE   - FRR Docker image (default: quay.io/frrouting/frr:10.3.1)
    VERBOSE     - set to 1 for debug output
    NO_BUILD    - set to 1 to skip image builds
"""

import os
import subprocess
import sys

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
PROJECT_ROOT = os.path.abspath(os.path.join(SCRIPT_DIR, "..", ".."))

# Make interop module importable from check.py scripts.
sys.path.insert(0, SCRIPT_DIR)

from interop import Scenario, log_fail, log_pass


def build_images(frr_image, no_build=False):
    """Build ze-interop, bird-interop images. Pull FRR."""
    if no_build:
        print("  skipping image builds (NO_BUILD=1)")
        return

    print("Building Ze image...")
    subprocess.run(
        ["docker", "build", "-t", "ze-interop",
         "-f", os.path.join(SCRIPT_DIR, "Dockerfile.ze"),
         PROJECT_ROOT, "-q"],
        check=True, timeout=600,
    )

    print("Building BIRD image...")
    subprocess.run(
        ["docker", "build", "-t", "bird-interop",
         "-f", os.path.join(SCRIPT_DIR, "Dockerfile.bird"),
         SCRIPT_DIR, "-q"],
        check=True, timeout=600,
    )

    print("Building GoBGP image...")
    result = subprocess.run(
        ["docker", "build", "-t", "gobgp-interop",
         "-f", os.path.join(SCRIPT_DIR, "Dockerfile.gobgp"),
         SCRIPT_DIR, "-q"],
        capture_output=True, text=True, timeout=600,
    )
    if result.returncode != 0:
        print("  warning: GoBGP image build failed (GoBGP scenarios will fail)")

    print("Pulling FRR image...")
    subprocess.run(
        ["docker", "pull", "-q", frr_image],
        check=True, timeout=600,
    )


def main():
    frr_image = os.environ.get("FRR_IMAGE", "quay.io/frrouting/frr:10.3.1")
    no_build = os.environ.get("NO_BUILD", "0") == "1"

    # Accept a scenario filter as positional argument.
    scenario_filter = ""
    if len(sys.argv) > 1:
        scenario_filter = sys.argv[1]

    # Check Docker is available.
    try:
        result = subprocess.run(
            ["docker", "info"],
            capture_output=True, text=True, timeout=15,
        )
        docker_ok = result.returncode == 0
    except (FileNotFoundError, subprocess.TimeoutExpired):
        docker_ok = False
    if not docker_ok:
        print("Docker unavailable, skipping interop tests")
        sys.exit(0)

    build_images(frr_image, no_build)

    print("")
    print("\u2501" * 40)
    print(" Ze Interoperability Tests")
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

        scenario = Scenario(scenario_dir, frr_image)
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
