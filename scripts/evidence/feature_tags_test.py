"""The evidence scripts' tag derivation agrees with the Makefile's.

The point of the helper is that one fact has one record. A test that only
checked `feature_tags()` against itself would let the two drift and stay
green, so the assertions here read the Makefile's own awk and every evidence
script's build site.
"""

from __future__ import annotations

import ast
import re
import subprocess
import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

from feature_tags import daemon_build_tags, feature_tags  # noqa: E402

ROOT = Path(__file__).resolve().parents[2]
EVIDENCE = ROOT / "scripts" / "evidence"

# A daemon build: these run ./cmd/ze as the product and test a feature, so each
# one must carry every gate. A host driver (ze-host) is deliberately absent --
# see test_host_drivers_take_no_feature_tags.
DAEMON_SCRIPTS = (
    "docker-run.py",
    "effective-l2tp-ppp.py",
    "effective-l2tp-peer.py",
    "effective-vpp-iface.py",
    "effective-vrrp-keepalived.py",
    "effective-pppoe-accel.py",
)

HOST_DRIVER_SCRIPTS = (
    "effective-install-qemu.py",
    "effective-install-iso-qemu.py",
    "effective-vpp-hugepages-qemu.py",
)


class FeatureTagsMatchTheMakefile(unittest.TestCase):
    def test_derivation_matches_the_makefile_awk(self):
        """Run the Makefile's own awk and demand the same answer."""
        awk = subprocess.run(
            ["awk", "$1 ~ /^ze_/ {print $1}", str(ROOT / "feature-gates.txt")],
            capture_output=True,
            text=True,
            check=True,
        )
        want = sorted(set(awk.stdout.split()))
        self.assertEqual(feature_tags(ROOT), want)

    def test_the_gate_list_is_not_empty(self):
        """An empty answer would silently rebuild the defect this file fixes."""
        tags = feature_tags(ROOT)
        self.assertGreater(len(tags), 20, "feature-gates.txt parsed to almost nothing")
        self.assertIn("ze_l2tp", tags)
        self.assertIn("ze_bgp", tags)

    def test_daemon_build_tags_carry_the_base_and_every_gate(self):
        tags = daemon_build_tags(ROOT).split()
        self.assertEqual(tags[:2], ["ze_core", "ze_distro"])
        for gate in feature_tags(ROOT):
            self.assertIn(gate, tags, f"{gate} missing from the daemon build")


class EveryEvidenceScriptDerivesItsTags(unittest.TestCase):
    """No script may go back to a literal. This is the ratchet."""

    def _tag_arguments(self, path: Path) -> list[ast.expr]:
        """Every value passed as the argument after a `-tags` string."""
        tree = ast.parse(path.read_text(encoding="utf-8"))
        found: list[ast.expr] = []
        for node in ast.walk(tree):
            if not isinstance(node, (ast.List, ast.Tuple)):
                continue
            for i, elt in enumerate(node.elts[:-1]):
                if isinstance(elt, ast.Constant) and elt.value == "-tags":
                    found.append(node.elts[i + 1])
        return found

    def test_daemon_scripts_call_the_helper(self):
        for name in DAEMON_SCRIPTS:
            with self.subTest(script=name):
                args = self._tag_arguments(EVIDENCE / name)
                self.assertTrue(args, f"{name} has no -tags argument any more")
                for arg in args:
                    self.assertNotIsInstance(
                        arg,
                        ast.Constant,
                        f"{name} hardcodes its build tags again; call "
                        f"daemon_build_tags(root) so a new gate reaches it",
                    )

    def test_host_drivers_take_no_feature_tags(self):
        """ze-host runs `ze appliance ...` on the build host.

        The appliance verb comes from the blank import in
        cmd/ze/setup_features_setup.go, which is `//go:build ze_setup`, so a
        ze_distro host binary has no appliance command at all. Feature gates
        select daemon features and have nothing to say here, which is why
        mk/build-gokrazy.mk builds ze-host as plain `ze_core ze_setup`.
        """
        for name in HOST_DRIVER_SCRIPTS:
            with self.subTest(script=name):
                text = (EVIDENCE / name).read_text(encoding="utf-8")
                for match in re.finditer(r'"-tags",\s*"([^"]+)"', text):
                    self.assertIn(
                        "ze_setup",
                        match.group(1),
                        f"{name} builds a host driver without ze_setup, so it "
                        f"has no `ze appliance` command",
                    )


if __name__ == "__main__":
    unittest.main()
