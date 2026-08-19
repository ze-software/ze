#!/usr/bin/env python3
"""Unit tests for the ipsec-interop lab helpers.

The parsers here read the output of external commands. Nothing in the lab could
test them before this file existed, and a dead `bytes\\s+(\\d+)` pattern lived in
four copies of the scenarios for that reason: every copy returned zero, and every
`after > before` assertion built on it passed whatever the tunnel did.

CAPTURED is verbatim output of `ip -s xfrm state` from a Linux 6.8 kernel, taken
in a user network namespace holding one AES-GCM SA and one AES-CBC SA. It is
evidence, not a reconstruction. WITH_TRAFFIC and WITH_BYTE_LIMIT keep that layout
and substitute counter values the capture could not produce, which is stated here
so a reader does not mistake them for captured numbers.
"""

import base64
import glob
import importlib.util
import os
import re
import shutil
import sys
import tempfile
import unittest
from unittest import mock

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import lab  # noqa: E402
from lab import (  # noqa: E402
    _PKI_PLACEHOLDER,
    ZE_CLI_PASSWORD_HASH,
    ZE_CLI_PORT,
    ZE_CLI_USER,
    Scenario,
    StrongSwan,
    parse_xfrm_sa_bytes_by_spi,
    pem_to_base64_der,
    resolve_pki_placeholders,
)

LAB_DIR = os.path.dirname(os.path.abspath(__file__))

CAPTURED = """src 10.0.0.2 dst 10.0.0.1
\tproto esp spi 0x9b7e2d51(2608737617) reqid 0(0x00000000) mode tunnel
\treplay-window 0 seq 0x00000000 flag  (0x00000000)
\tauth-trunc hmac(sha256) 0x0011223344556677 (256 bits) 96
\tenc cbc(aes) 0x000102030405060708090a0b0c0d0e0f (128 bits)
\tanti-replay context: seq 0x0, oseq 0x0, bitmap 0x00000000
\tsel src 0.0.0.0/0 dst 0.0.0.0/0 uid 0
\tlifetime config:
\t  limit: soft (INF)(bytes), hard (INF)(bytes)
\t  limit: soft (INF)(packets), hard (INF)(packets)
\t  expire add: soft 0(sec), hard 0(sec)
\t  expire use: soft 0(sec), hard 0(sec)
\tlifetime current:
\t  0(bytes), 0(packets)
\t  add 2026-07-31 03:47:18 use -
\tstats:
\t  replay-window 0 replay 0 failed 0
src 10.0.0.1 dst 10.0.0.2
\tproto esp spi 0x3f2a1c04(1059724292) reqid 0(0x00000000) mode tunnel
\treplay-window 0 seq 0x00000000 flag  (0x00000000)
\taead rfc4106(gcm(aes)) 0x0102030405060708 (288 bits) 128
\tanti-replay context: seq 0x0, oseq 0x0, bitmap 0x00000000
\tsel src 0.0.0.0/0 dst 0.0.0.0/0 uid 0
\tlifetime config:
\t  limit: soft (INF)(bytes), hard (INF)(bytes)
\t  limit: soft (INF)(packets), hard (INF)(packets)
\t  expire add: soft 0(sec), hard 0(sec)
\t  expire use: soft 0(sec), hard 0(sec)
\tlifetime current:
\t  0(bytes), 0(packets)
\t  add 2026-07-31 03:47:18 use -
\tstats:
\t  replay-window 0 replay 0 failed 0
"""

# The captured layout with traffic counters substituted into `lifetime current`.
WITH_TRAFFIC = CAPTURED.replace(
    "\tlifetime current:\n\t  0(bytes), 0(packets)",
    "\tlifetime current:\n\t  846(bytes), 10(packets)",
)

# The captured layout with a real byte lifetime substituted for the INF limits.
# A peer that configures one makes the limit indistinguishable from the traffic
# counter for any reading that is not anchored on the `lifetime current` section.
WITH_BYTE_LIMIT = CAPTURED.replace(
    "\t  limit: soft (INF)(bytes), hard (INF)(bytes)",
    "\t  limit: soft 4194304(bytes), hard 8388608(bytes)",
)


class TestParseXfrmSABytesBySPI(unittest.TestCase):
    def test_captured_output_yields_one_entry_per_spi(self):
        counters = parse_xfrm_sa_bytes_by_spi(CAPTURED)
        self.assertEqual({"0x9b7e2d51": 0, "0x3f2a1c04": 0}, counters)

    def test_traffic_counters_are_read(self):
        counters = parse_xfrm_sa_bytes_by_spi(WITH_TRAFFIC)
        self.assertEqual({"0x9b7e2d51": 846, "0x3f2a1c04": 846}, counters)

    def test_byte_lifetime_limits_are_not_counted_as_traffic(self):
        counters = parse_xfrm_sa_bytes_by_spi(WITH_BYTE_LIMIT)
        self.assertEqual({"0x9b7e2d51": 0, "0x3f2a1c04": 0}, counters)

    def test_empty_output_yields_no_counters(self):
        self.assertEqual({}, parse_xfrm_sa_bytes_by_spi(""))

    def test_a_dead_pattern_would_fail_this_file(self):
        # The pattern the scenarios once carried was `bytes\s+(\d+)`, which needs
        # the number AFTER the word. It matches nothing in the captured output,
        # so a helper built on it returns an empty mapping for traffic that flowed.
        import re

        dead = re.compile(r"bytes\s+(\d+)")
        self.assertEqual([], dead.findall(WITH_TRAFFIC))
        self.assertNotEqual({}, parse_xfrm_sa_bytes_by_spi(WITH_TRAFFIC))


class TestAssertESPAccepted(unittest.TestCase):
    def test_a_deleted_spi_does_not_fail_the_check(self):
        from lab import assert_esp_accepted

        before = {"0xaaaa": 100, "0xbbbb": 900}
        after = {"0xaaaa": 400}  # 0xbbbb was deleted by a rekey; the sum fell.
        self.assertLess(sum(after.values()), sum(before.values()))
        assert_esp_accepted("ze", before, after, "peer accepted no ESP")

    def test_no_common_spi_grew_raises(self):
        from lab import assert_esp_accepted

        with self.assertRaises(AssertionError):
            assert_esp_accepted(
                "ze", {"0xaaaa": 100}, {"0xaaaa": 100}, "peer accepted no ESP"
            )

    def test_disjoint_spis_raise(self):
        from lab import assert_esp_accepted

        with self.assertRaises(AssertionError):
            assert_esp_accepted(
                "ze", {"0xaaaa": 100}, {"0xcccc": 900}, "peer accepted no ESP"
            )


CERT_PEM = """-----BEGIN CERTIFICATE-----
MIIBhTCCASugAwIBAgIUXM86IRy86IueiG1hNR/7Dsc6aVUwCgYIKoZIzj0EAwIw
GDEWMBQGA1UEAwwNemUtaW50ZXJvcC1jYQ==
-----END CERTIFICATE-----
"""

# openssl writes an EC PARAMETERS block before the key when the caller omits
# -noout. A reader that takes the first block takes the curve parameters, and
# the pki parser then rejects the value.
KEY_WITH_PARAMS_PEM = """-----BEGIN EC PARAMETERS-----
BggqhkjOPQMBBw==
-----END EC PARAMETERS-----
-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIBl9Jw1RcYVaRcVpRy2sLLQe0yFqQ0MJgVJmVQ2vGZ5OoAoGCCqGSM49
-----END EC PRIVATE KEY-----
"""


# openssl writes these two headers when a passphrase encrypts a classic PEM key.
# The body below is the base64 of encrypted DER, and no pki leaf can parse it.
ENCRYPTED_KEY_PEM = """-----BEGIN RSA PRIVATE KEY-----
Proc-Type: 4,ENCRYPTED
DEK-Info: AES-256-CBC,1F2E3D4C5B6A79881F2E3D4C5B6A7988

MIIBhTCCASugAwIBAgIUXM86IRy86IueiG1hNR/7Dsc6aVUwCgYIKoZIzj0EAwIw
-----END RSA PRIVATE KEY-----
"""

# The PKCS#8 spelling of the same thing. It carries no RFC 1421 header, so the
# label is the only signal that the body is encrypted.
PKCS8_ENCRYPTED_PEM = """-----BEGIN ENCRYPTED PRIVATE KEY-----
MIIBhTCCASugAwIBAgIUXM86IRy86IueiG1hNR/7Dsc6aVUwCgYIKoZIzj0EAwIw
-----END ENCRYPTED PRIVATE KEY-----
"""

# A root plus an intermediate, which is how a CA bundle is written. A reader
# that takes the first block inlines the root alone and cuts the chain.
BUNDLE_PEM = CERT_PEM + CERT_PEM

# A certificate and a key in one file. A reader that takes the first block puts
# whichever came first into the leaf, and a key in a certificate leaf is wrong.
CERT_PLUS_KEY_PEM = CERT_PEM + KEY_WITH_PARAMS_PEM

# A body holding one character that is outside the base64 alphabet.
CORRUPT_BODY_PEM = """-----BEGIN CERTIFICATE-----
MIIBhTCC*SugAwIBAgIUXM86IRy86IueiG1hNR/7Dsc6aVUwCgYIKoZIzj0EAwIw
-----END CERTIFICATE-----
"""


class TestPEMToBase64DER(unittest.TestCase):
    def test_body_is_returned_without_the_wrapper_or_line_breaks(self):
        value = pem_to_base64_der(CERT_PEM)
        self.assertNotIn("-----", value)
        self.assertNotIn("\n", value)
        # The pki parser runs base64.StdEncoding.DecodeString on this exact
        # string, then x509.ParseCertificate. DER opens with a SEQUENCE tag.
        self.assertEqual(0x30, base64.b64decode(value)[0])

    def test_the_ec_parameters_block_is_stepped_over(self):
        value = pem_to_base64_der(KEY_WITH_PARAMS_PEM)
        self.assertTrue(value.startswith("MHcCAQEEIBl9"))

    def test_text_with_no_pem_block_raises(self):
        with self.assertRaises(RuntimeError):
            pem_to_base64_der("/etc/ze/pki/ca.pem", source="ca.pem")

    def test_an_empty_block_raises(self):
        with self.assertRaises(RuntimeError):
            pem_to_base64_der(
                "-----BEGIN CERTIFICATE-----\n-----END CERTIFICATE-----\n"
            )

    def test_an_rfc1421_header_is_refused(self):
        # Without this guard the two header lines are appended to the body and
        # returned inside the value, and the key itself is encrypted anyway.
        with self.assertRaises(RuntimeError) as caught:
            pem_to_base64_der(ENCRYPTED_KEY_PEM, source="server-key.pem")
        self.assertIn("server-key.pem", str(caught.exception))
        self.assertIn("Proc-Type", str(caught.exception))

    def test_an_encrypted_private_key_label_is_refused(self):
        with self.assertRaises(RuntimeError) as caught:
            pem_to_base64_der(PKCS8_ENCRYPTED_PEM, source="client-key.pem")
        self.assertIn("client-key.pem", str(caught.exception))
        self.assertIn("encrypted", str(caught.exception))

    def test_a_two_certificate_bundle_is_refused(self):
        # The first block once won in silence, so a root-plus-intermediate
        # ca.pem inlined the root alone and the chain was cut.
        with self.assertRaises(RuntimeError) as caught:
            pem_to_base64_der(BUNDLE_PEM, source="ca.pem")
        self.assertIn("ca.pem", str(caught.exception))
        self.assertIn("2 PEM blocks", str(caught.exception))

    def test_a_certificate_beside_a_key_is_refused(self):
        with self.assertRaises(RuntimeError) as caught:
            pem_to_base64_der(CERT_PLUS_KEY_PEM, source="client.pem")
        self.assertIn("client.pem", str(caught.exception))
        self.assertIn("CERTIFICATE", str(caught.exception))
        self.assertIn("EC PRIVATE KEY", str(caught.exception))

    def test_a_body_that_is_not_base64_is_refused(self):
        with self.assertRaises(RuntimeError) as caught:
            pem_to_base64_der(CORRUPT_BODY_PEM, source="ca.pem")
        self.assertIn("ca.pem", str(caught.exception))
        self.assertIn("base64", str(caught.exception))

    def test_a_valid_body_is_returned_byte_for_byte(self):
        # The guard decodes to check, and discards the result. The value that
        # comes back is the original text, joined and stripped of line breaks.
        value = pem_to_base64_der(CERT_PEM)
        self.assertEqual("".join(CERT_PEM.splitlines()[1:-1]), value)


class TestResolvePKIPlaceholders(unittest.TestCase):
    def test_a_placeholder_becomes_the_base64_der_body(self):
        out = resolve_pki_placeholders(
            'certificate "%%PKI_B64:ca.pem%%";',
            "/pki",
            read=lambda path: CERT_PEM,
        )
        self.assertNotIn("%%", out)
        self.assertIn("MIIBhTCCASug", out)

    def test_content_without_a_placeholder_is_returned_unchanged(self):
        text = 'certificate "already-inlined";'
        self.assertEqual(text, resolve_pki_placeholders(text, None))

    def test_a_missing_pki_directory_raises(self):
        with self.assertRaises(RuntimeError):
            resolve_pki_placeholders('key "%%PKI_B64:ca.pem%%";', None)

    def test_an_unreadable_file_raises(self):
        def refuse(path):
            raise OSError("no such file")

        with self.assertRaises(RuntimeError):
            resolve_pki_placeholders(
                'key "%%PKI_B64:ghost.pem%%";', "/pki", read=refuse
            )


class TestScenarioPKIFixtures(unittest.TestCase):
    """Pin the leaf names the pki schema accepts.

    Scenarios 03, 04 and 08 each wrote a `private-key` leaf holding a file path.
    The schema has `certificate { private { key } }` and every leaf holds
    base64-encoded DER, so ze refused all three configs with "unknown field in
    certificate: private-key" and no EAP packet ever left the container.
    """

    def scenario_configs(self):
        return sorted(glob.glob(os.path.join(LAB_DIR, "scenarios", "*", "ze.conf")))

    @staticmethod
    def read(path):
        with open(path, encoding="utf-8") as fh:
            return fh.read()

    def test_no_fixture_writes_a_private_key_leaf(self):
        for path in self.scenario_configs():
            text = self.read(path)
            self.assertNotIn(
                "private-key", text, "%s: the leaf is private { key }" % path
            )
            self.assertNotIn(
                "%%PKI_DIR%%", text, "%s: a pki leaf holds no file path" % path
            )

    def test_every_pki_scenario_resolves_to_decodable_der(self):
        # The directory comes from the PRODUCER, `Scenario._find_pki_dir`, not
        # from a second copy of its rule here. It prefers the scenario's own
        # `pki/` and falls back to the shared one, and a test that named the
        # shared directory for every scenario asserted against material the lab
        # would never read: 25-responder-eap-tls13 carries `ze.pem` in its own
        # directory, the shared one holds `client.pem`, and the case died with
        # FileNotFoundError rather than failing on anything about the config.
        seen = set()
        checked = 0
        for path in self.scenario_configs():
            text = self.read(path)
            if "%%PKI_B64:" not in text:
                continue
            seen.add(os.path.basename(os.path.dirname(path)))
            wanted = len(_PKI_PLACEHOLDER.findall(text))
            pki_dir = Scenario(os.path.dirname(path))._find_pki_dir()
            self.assertIsNotNone(pki_dir, "%s: no PKI directory resolves" % path)
            out = resolve_pki_placeholders(text, pki_dir)
            self.assertNotIn("%%PKI_B64:", out)
            values = re.findall(r'"([A-Za-z0-9+/=]{64,})"', out)
            # The regex filters on the base64 alphabet. A value that stops
            # looking like base64 matches nothing, the loop below then runs zero
            # times, and the case asserts nothing about any value. Count the
            # matches against the placeholders, and that silence turns red.
            self.assertEqual(
                wanted,
                len(values),
                "%s: %d placeholders resolved to %d base64 values"
                % (path, wanted, len(values)),
            )
            for value in values:
                self.assertEqual(0x30, base64.b64decode(value)[0], path)
            checked += len(values)
        # The expected SET is spelled out, not just its size. A count alone lets
        # two errors cancel: scenario 06 losing all three placeholders while a new
        # PKI scenario appears keeps the total at four and passes green. Naming the
        # scenarios makes each side of that trade visible on its own.
        #
        # Spelled out rather than derived, so a scenario that silently stops
        # carrying PKI material turns this red. Adding a PKI scenario is meant to
        # cost one edit here: that is the prompt to check the new scenario
        # resolves, not an accident of the count.
        self.assertEqual(
            {
                "03-eap-mschapv2",
                "04-eap-tls",
                "06-eap-tls13",
                "08-responder-eap-mschapv2",
                "25-responder-eap-tls13",
            },
            seen,
            "exactly these scenarios carry PKI material",
        )
        self.assertEqual(15, checked, "each of the five scenarios holds 3 pki leaves")


class TestPrepareZeConf(unittest.TestCase):
    """The CLI account and the SSH listener are rendered, never hand-copied.

    The daemon starts no SSH listener unless its config asks for one
    (`infraSetup`, cmd/ze/hub/infra_setup.go), and `ze_cli` reaches it over SSH.
    Two of the sixteen scenarios carried the block by hand, so the next author to
    add a `ze_cli` call would have met a credential error that says nothing about
    what their scenario tests. `Scenario._prepare_ze_conf` appends it to the
    rendered copy instead, the way `_render_scenario_dir` does in
    test/interop/interop.py.
    """

    def setUp(self):
        self.tmp = tempfile.mkdtemp(prefix="ze-lab-test-")
        self.addCleanup(shutil.rmtree, self.tmp, True)

    def write_conf(self, text):
        path = os.path.join(self.tmp, "ze.conf")
        with open(path, "w", encoding="utf-8") as fh:
            fh.write(text)
        return path

    def test_a_config_needing_no_pki_still_gains_the_account_and_listener(self):
        # No placeholder to resolve, so the resolver returns the text unchanged.
        # That used to short-circuit the render and hand the container the source
        # file, which is how fourteen of the sixteen ended up with no listener.
        path = self.write_conf("vpn {\n\tipsec {\n\t}\n}\n")
        out = Scenario(self.tmp)._prepare_ze_conf(path, None)
        self.assertNotEqual(path, out, "the container must mount a rendered copy")
        with open(out, encoding="utf-8") as fh:
            text = fh.read()
        self.assertIn("user %s {" % ZE_CLI_USER, text)
        self.assertIn(ZE_CLI_PASSWORD_HASH, text)
        self.assertIn("ssh {", text)
        self.assertIn("port %s;" % ZE_CLI_PORT, text)
        self.assertIn("vpn {", text, "the scenario's own config survives")

    def test_no_scenario_config_carries_the_boilerplate(self):
        # `grep -l authentication` answers sixteen and means nothing: every
        # scenario carries the IKE peer's own authentication block. The account
        # name and the hash are what only the render may hold.
        pattern = os.path.join(LAB_DIR, "scenarios", "*", "ze.conf")
        for path in sorted(glob.glob(pattern)):
            with open(path, encoding="utf-8") as fh:
                text = fh.read()
            self.assertNotIn(
                "user %s {" % ZE_CLI_USER,
                text,
                "%s: the CLI account is appended by _prepare_ze_conf" % path,
            )
            self.assertNotIn(
                ZE_CLI_PASSWORD_HASH,
                text,
                "%s: the CLI password hash is appended by _prepare_ze_conf" % path,
            )


class TestXfrmStateFailsClosed(unittest.TestCase):
    """A failed XFRM read must raise, not answer "no SAs".

    10-clear-reestablish snapshots strongSwan's ESP SPIs BEFORE its clear and
    passes when a SPI absent from that snapshot appears after. A reader that
    answered "" for a failed command made the snapshot empty, and then the SA
    that already existed satisfied the comparison on the first poll: the
    scenario passed with the clear having done nothing.

    Empty output is a real answer here -- `ip xfrm state` prints nothing when
    the kernel holds no SA -- so the fault is read from the EXIT STATUS.
    """

    def test_xfrm_state_propagates_a_failed_command(self):
        with mock.patch.object(lab, "docker_exec_quiet", return_value=""):
            with mock.patch.object(
                lab, "docker_exec", side_effect=RuntimeError("rc=1")
            ):
                with self.assertRaises(RuntimeError):
                    StrongSwan().xfrm_state()

    def test_scenario_10_refuses_an_empty_before_snapshot(self):
        check = self.load_scenario_10()
        swan = mock.Mock()
        swan.xfrm_state.return_value = ""
        with mock.patch.object(check, "StrongSwan", return_value=swan):
            with mock.patch.object(check, "ze_cli") as cli:
                with self.assertRaises(AssertionError):
                    check.check()
        cli.assert_not_called()

    @staticmethod
    def load_scenario_10():
        path = os.path.join(
            LAB_DIR, "scenarios", "10-clear-reestablish", "check.py"
        )
        spec = importlib.util.spec_from_file_location("scenario10_check", path)
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        return module


if __name__ == "__main__":
    unittest.main()
