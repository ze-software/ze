#!/usr/bin/env python3
"""Ze interoperability test runner.

Usage:
    python3 test/interop/run.py                     # run all scenarios
    python3 test/interop/run.py bgp-ebgp-ipv4-frr    # run specific scenario
    VERBOSE=1 python3 test/interop/run.py            # verbose output

Environment:
    FRR_IMAGE                 - FRR Docker image
                                (default: quay.io/frrouting/frr:10.3.1)
    VERBOSE                   - set to 1 for debug output
    NO_BUILD                  - set to 1 to skip image builds
    ZE_INTEROP_SUBNET_INDEX   - force a rendered lab subnet slot
    ZE_INTEROP_SUBNET_PREFIX  - force a rendered lab prefix, e.g. 172.30.44.
"""

import os
import subprocess
import sys

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
PROJECT_ROOT = os.path.abspath(os.path.join(SCRIPT_DIR, "..", ".."))

# Make interop module importable from check.py scripts.
sys.path.insert(0, SCRIPT_DIR)

from interop import (  # noqa: E402  (the sys.path insert above must run first)
    Scenario,
    log_fail,
    log_pass,
    observer_failure_note,
    set_images,
)


def _build_image(name, tag, dockerfile, context, required):
    """Build one image and return the reference this run must use for it.

    The reference is the image ID `docker build -q` prints, never the tag. A tag
    is shared between concurrent runs and an ID is not, so pinning the ID is
    what stops this run consuming an image another run built (`set_images`,
    test/interop/interop.py).

    `required` picks the failure policy, and the two were already different in
    this suite before the table existed: ze, BIRD and keepalived were fatal, and
    GoBGP and StayRTR were tolerated. A required build raises. A tolerated one
    warns and returns the tag, and that is about the blast radius rather than
    the importance: those two builds reach the network for a module, and being
    fatal would let one unreachable proxy take down the 100+ scenarios that
    never touch them. Nothing is waved through by tolerating it, because the
    scenario that wants the image fails at `docker run` with the image name in
    the message, so the absence is reported on the scenario it belongs to.
    """
    print("Building %s image..." % name)
    result = subprocess.run(
        [
            "docker",
            "build",
            "-t",
            tag,
            "-f",
            dockerfile,
            context,
            "-q",
        ],
        capture_output=True,
        text=True,
        timeout=600,
    )
    if result.returncode != 0:
        if required:
            raise RuntimeError(
                "docker build %s failed: %s"
                % (tag, (result.stderr or "").strip() or "no stderr")
            )
        print(
            "  warning: %s image build failed (%s scenarios will fail)" % (name, name)
        )
        return tag
    image_id = (result.stdout or "").strip().splitlines()
    image_id = image_id[-1].strip() if image_id else ""
    if not image_id:
        # `-q` prints the ID and nothing else, so an empty stdout on a zero exit
        # means this docker did not answer the question asked. Falling back to
        # the tag silently would restore the race this function exists to close.
        raise RuntimeError("docker build %s printed no image id" % tag)
    print("  %s -> %s" % (tag, image_id))
    return image_id


def build_images(frr_image, no_build=False):
    """Build the lab images and return the reference each one must be run by.

    Keyed by the logical names `set_images` (test/interop/interop.py) reads. An
    empty mapping leaves every reference at its shared tag, which is what
    `NO_BUILD=1` needs: it builds nothing, so it has no ID to pin and the tag it
    reuses is its only possible reference.
    """
    if no_build:
        print("  skipping image builds (NO_BUILD=1)")
        return {}

    built = {}
    for name, tag, dockerfile, context, required in (
        ("ze", "ze-interop", "Dockerfile.ze", PROJECT_ROOT, True),
        ("bird", "bird-interop", "Dockerfile.bird", SCRIPT_DIR, True),
        ("gobgp", "gobgp-interop", "Dockerfile.gobgp", SCRIPT_DIR, False),
        ("keepalived", "keepalived-interop", "Dockerfile.keepalived", SCRIPT_DIR, True),
        ("stayrtr", "stayrtr-interop", "Dockerfile.stayrtr", SCRIPT_DIR, False),
    ):
        built[name] = _build_image(
            name, tag, os.path.join(SCRIPT_DIR, dockerfile), context, required
        )

    print("Pulling FRR image...")
    subprocess.run(
        ["docker", "pull", "-q", frr_image],
        check=True,
        timeout=600,
    )
    return built


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
            capture_output=True,
            text=True,
            timeout=15,
        )
        docker_ok = result.returncode == 0
    except (FileNotFoundError, subprocess.TimeoutExpired):
        docker_ok = False
    if not docker_ok:
        # FAIL CLOSED, and say so on stderr with the remediation. This is the model the
        # three sibling lab runners (ipsec/l2tp/pppoe) were aligned to on 2026-07-29 --
        # they already exited 1, but printed a bare line to stdout with no next step, so
        # the set this pattern was meant to unify stayed divergent on the observable
        # half.
        #
        # Every scenario runs in containers, so an unreachable Docker means nothing was
        # verified -- and a runner that exits 0 over an absence reports success for work it
        # never did (ai/rules/evidence.md). This is load-bearing beyond tidiness: an
        # interop scenario may now carry an `RFC requirement:` tag, and a tag is only
        # evidence if something executes the test (plan/spec-rfcgate-2-evidence.md AC-1).
        print(
            "error: Docker unavailable, cannot run the BGP interop lab -- every "
            "scenario runs in containers. Start Docker (or install it), then re-run: "
            "make ze-interop-test",
            file=sys.stderr,
        )
        sys.exit(1)

    # Pin every container of this run to the image THIS run built, by ID. The
    # shared tags are rebound by any concurrent build, so the pin is what keeps
    # a mutation proof honest: without it a scenario can pass against a daemon
    # the run never built (test/interop/interop.py, `set_images`).
    set_images(build_images(frr_image, no_build))

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
            # Counted BEFORE the note is read, and the interrupt inside the note
            # window is caught below. Both halves are needed and neither works
            # alone: `observer_failure_note` can spend up to 15 seconds inside
            # `docker logs`, and a Ctrl-C in that window lands INSIDE this
            # handler, where it escapes `main` and takes the summary with it.
            # Counting first is what makes the caught interrupt printable; the
            # catch is what gets the summary printed at all. Round 5 moved these
            # two lines and claimed the effect on its own, which round 6
            # measured as inert: the totals were right and nothing printed them.
            failed += 1
            failed_names.append(scenario_name)
            # A process plugin that calls runtime_fail (test/scripts/ze_api.py)
            # stops ze, and every wait the scenario was running then reads as a
            # route that never arrived. This is the site that sees every
            # scenario failure other than the interrupt above, so it is where
            # the plugin's own message is recovered: the containers are still
            # up here, teardown runs in the `finally` below. It never raises;
            # a second exception here would replace the failure being reported.
            #
            # The note is printed AS IT COMES BACK. It words its own claim,
            # because the sentinel case and the unreadable case assert
            # different things, and a prefix added here asserted the wrong one
            # for every failure that precedes ze's container.
            try:
                note = observer_failure_note()
            except KeyboardInterrupt:
                # The operator interrupted the docker read. The scenario is
                # already counted above, so ending the LOOP reports the run;
                # letting this escape would report nothing at all. Teardown
                # runs in the `finally` below, as on the branch above.
                log_fail("INTERRUPTED")
                break
            if note:
                log_fail(note)
        finally:
            scenario.teardown()

        print("")

    # Warn if filter matched nothing.
    if scenario_filter and passed + failed == 0:
        print(
            "error: no scenario matching '%s' found" % scenario_filter, file=sys.stderr
        )
        sys.exit(1)

    # Summary.
    print("\u2501" * 40)
    if failed == 0:
        print("\033[32mPASS  %d scenario(s)\033[0m" % passed)
    else:
        print(
            "\033[31mFAIL  %d passed, %d failed: %s\033[0m"
            % (passed, failed, " ".join(failed_names))
        )
    print("\u2501" * 40)

    sys.exit(0 if failed == 0 else 1)


if __name__ == "__main__":
    main()
