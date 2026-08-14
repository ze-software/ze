#!/usr/bin/env python3
"""Both container images build with the default feature set, derived once.

`docker/Dockerfile` built `${ZE_TAGS:+-tags ${ZE_TAGS}}` and the `ze-docker`
target passed `--build-arg ZE_TAGS` only when the caller had already set it. With
`ZE_TAGS` unset -- the documented invocation, `make ze-docker` -- the image held an
UNTAGGED binary: no `ze_core`, so `ze start` is an unknown command, and no `ze_bgp`,
so a `bgp` config root is rejected as an unknown top-level keyword. `docker
compose build` reached the same Dockerfile with no build arg at all.

The fix is the recipe `test/interop/Dockerfile.ze` already proves: derive the
default-on tags from `feature-gates.txt`, the single source of truth every other
consumer reads (Makefile `ZE_FEATURES`, the codegen, `dep_audit.py`). `ZE_TAGS`
stays what it is everywhere else in the build system: EXTRA tags, added to the
derived set, never a replacement for it.

These tests pin that contract for both images -- the scratch deployment image and
the alpine lab image netlab runs -- and refuse a second, hand-written tag list.
A hand-written list is the failure mode this contract exists to refuse
(spec-netlab-integration, R-2 and R-8): the lab image would build a
binary the deployment image does not, and a defect would reproduce in one and not
the other.

Run: python3 scripts/dev/lab_image_tags_test.py
(also picked up automatically by TestPythonUnitTests, scripts/dev/python_tests_test.go)
"""

import pathlib
import re
import unittest

ROOT = pathlib.Path(__file__).resolve().parents[2]
GATES = ROOT / "feature-gates.txt"
MAKEFILE = ROOT / "Makefile"
DEPLOY_DOCKERFILE = ROOT / "docker" / "Dockerfile"
LAB_DOCKERFILE = ROOT / "docker" / "Dockerfile.lab"
INTEROP_DOCKERFILE = ROOT / "test" / "interop" / "Dockerfile.ze"

# The one derivation, as awk writes it. The Makefile doubles every `$` for make,
# so both spellings normalise to this before comparison.
AWK_PROGRAM = "$1 ~ /^ze_/ {print $1}"

# The personality tag. It is not a feature gate, so it is the one `ze_` token a
# Dockerfile is allowed to spell out.
PERSONALITY_TAG = "ze_core"


def feature_tags() -> set[str]:
    """The default-on feature tags, read the way every consumer reads them."""
    tags = set()
    for line in GATES.read_text(encoding="utf-8").splitlines():
        fields = line.split()
        if fields and fields[0].startswith("ze_"):
            tags.add(fields[0])
    return tags


def awk_programs(text: str) -> list[str]:
    """Every awk program in `text` that reads feature-gates.txt, make-normalised."""
    found = []
    for match in re.finditer(r"awk\s+'([^']*)'\s*(\S*feature-gates\.txt)", text):
        found.append(match.group(1).replace("$$", "$"))
    return found


def instructions(text: str) -> str:
    """`text` with its comment lines dropped.

    A comment naming a gate is documentation (both Dockerfiles explain why an
    untagged build has no ze_bgp). An INSTRUCTION naming one is the second list
    this test refuses.
    """
    return "\n".join(
        line for line in text.splitlines() if not line.lstrip().startswith("#")
    )


def go_build_tag_lists(text: str) -> list[str]:
    """The tag list of every `go build -tags <list>` in `text`."""
    return [m.group(1) for m in re.finditer(r"-tags\s+\"([^\"]*)\"", text)]


class TestLabImageTagsMatchFeatureGates(unittest.TestCase):
    """AC-6 and AC-7: both images carry the shipped default feature set."""

    def test_lab_image_recipe_exists(self):
        self.assertTrue(
            LAB_DOCKERFILE.is_file(),
            "docker/Dockerfile.lab is missing: netlab and containerlab exec sh, ip "
            "and cat inside a node, which a scratch image cannot answer",
        )

    def test_both_dockerfiles_derive_the_tags_from_feature_gates(self):
        for path in (DEPLOY_DOCKERFILE, LAB_DOCKERFILE):
            with self.subTest(dockerfile=str(path.relative_to(ROOT))):
                programs = awk_programs(path.read_text(encoding="utf-8"))
                self.assertEqual(
                    programs,
                    [AWK_PROGRAM],
                    f"{path.name} must derive its default-on tags from "
                    "feature-gates.txt exactly once",
                )

    def test_the_derivation_is_the_same_everywhere(self):
        """One derivation, four readers: Makefile, both images, the interop image."""
        for path in (MAKEFILE, DEPLOY_DOCKERFILE, LAB_DOCKERFILE, INTEROP_DOCKERFILE):
            with self.subTest(reader=str(path.relative_to(ROOT))):
                self.assertIn(
                    AWK_PROGRAM,
                    awk_programs(path.read_text(encoding="utf-8")),
                    f"{path.name} reads feature-gates.txt differently from the rest",
                )

    def test_neither_dockerfile_hand_writes_a_feature_tag(self):
        """A second tag list is the drift R-2 and R-8 name, so no gate is spelled out."""
        gates = feature_tags()
        for path in (DEPLOY_DOCKERFILE, LAB_DOCKERFILE):
            text = instructions(path.read_text(encoding="utf-8"))
            spelled = {t for t in re.findall(r"\bze_[a-z0-9_]+\b", text) if t in gates}
            with self.subTest(dockerfile=str(path.relative_to(ROOT))):
                self.assertEqual(
                    spelled,
                    set(),
                    f"{path.name} hand-writes feature tags {sorted(spelled)}; they "
                    "must come from feature-gates.txt",
                )

    def test_both_images_build_one_identical_tag_list(self):
        """The lab image and the deployment image build the same binary features."""
        expected = f"{PERSONALITY_TAG} $ZE_FEATURES $ZE_TAGS"
        for path in (DEPLOY_DOCKERFILE, LAB_DOCKERFILE):
            with self.subTest(dockerfile=str(path.relative_to(ROOT))):
                self.assertEqual(
                    go_build_tag_lists(path.read_text(encoding="utf-8")),
                    [expected],
                    f'{path.name} must build -tags "{expected}": the personality '
                    "tag, the derived feature set, then the caller's extra tags",
                )

    def test_bgp_is_in_the_derived_set(self):
        """AC-7's symptom: without ze_bgp a `bgp` config root is an unknown keyword."""
        self.assertIn("ze_bgp", feature_tags())


class TestDockerTargetsAreDistinct(unittest.TestCase):
    """R-4: the lab image must not be mistaken for the deployment image."""

    def target_body(self, name: str) -> str:
        text = MAKEFILE.read_text(encoding="utf-8")
        match = re.search(rf"^{re.escape(name)}:.*?\n((?:\t.*\n|\n)*)", text, re.M)
        # `if match is None` rather than assertIsNotNone: both fail the test, and
        # only this form narrows the Optional for a type checker reading .group.
        if match is None:
            self.fail(f"Makefile has no {name} target")
        return match.group(1)

    def test_ze_docker_lab_builds_the_lab_recipe(self):
        body = self.target_body("ze-docker-lab")
        self.assertIn("-f docker/Dockerfile.lab", body)
        self.assertIn("-t $(ZE_LAB_IMAGE):$(ZE_LAB_TAG)", body)
        defaults = dict(
            re.findall(
                r"^(ZE_LAB_\w+) \?= (\S+)$", MAKEFILE.read_text(encoding="utf-8"), re.M
            )
        )
        self.assertEqual(
            defaults,
            {"ZE_LAB_IMAGE": "netlab/ze", "ZE_LAB_TAG": "latest"},
            "the lab image defaults to netlab/ze:latest",
        )

    def test_ze_docker_builds_the_deployment_recipe(self):
        body = self.target_body("ze-docker")
        self.assertIn("-f docker/Dockerfile", body)
        self.assertNotIn("Dockerfile.lab", body)
        self.assertIn("-t $(ZE_DOCKER_IMAGE):$(ZE_DOCKER_TAG)", body)
        self.assertNotIn("ZE_LAB_IMAGE", body)

    def test_both_targets_pass_ze_tags_as_extra_tags(self):
        """`make ze-docker ZE_TAGS=maprib` keeps ze_core and the feature set."""
        for name in ("ze-docker", "ze-docker-lab"):
            with self.subTest(target=name):
                self.assertIn("--build-arg ZE_TAGS=", self.target_body(name))


if __name__ == "__main__":
    unittest.main()
