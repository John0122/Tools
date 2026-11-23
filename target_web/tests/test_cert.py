"""实际调用 OpenSSL，验证证书生命周期与访问地址，而非匹配脚本内容。"""

import os
from pathlib import Path
import ssl
import subprocess
import tempfile
import unittest


GENERATOR = Path(__file__).resolve().parents[1] / "scripts" / "generate-cert.sh"


class CertificateTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="target web cert ")
        self.addCleanup(self.temp.cleanup)
        self.directory = Path(self.temp.name) / "certs"
        self.cert = self.directory / "fullchain.pem"
        self.key = self.directory / "privkey.pem"

    def generate(self, host="192.168.5.10", force=False):
        command = ["bash", str(GENERATOR), host, str(self.directory)]
        if force:
            command.append("--force")
        return subprocess.run(command, capture_output=True, text=True)

    def openssl(self, *args):
        result = subprocess.run(
            ["openssl", *map(str, args)], capture_output=True, text=True
        )
        self.assertEqual(result.returncode, 0, result.stderr or result.stdout)
        return result.stdout

    def assert_generated(self, host="192.168.5.10", force=False):
        result = self.generate(host, force)
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_certificate_matches_ip_and_loopback_with_server_usage(self):
        # SAN 缺失、曲线或用途错误将导致地址校验或握手失败。
        self.assert_generated()
        for host in ("192.168.5.10", "127.0.0.1"):
            self.openssl("verify", "-CAfile", self.cert, "-purpose", "sslserver",
                         "-verify_ip", host, self.cert)
        self.openssl("verify", "-CAfile", self.cert, "-verify_hostname",
                     "localhost", self.cert)
        details = self.openssl("x509", "-in", self.cert, "-text", "-noout")
        self.assertIn("ASN1 OID: prime256v1", details)
        self.assertIn("CA:FALSE", details)
        self.assertIn("Digital Signature", details)
        self.assertIn("TLS Web Server Authentication", details)
        self.openssl("x509", "-in", self.cert, "-checkend", 824 * 86400, "-noout")
        context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
        context.load_cert_chain(self.cert, self.key)
        self.assertEqual(self.key.stat().st_mode & 0o777, 0o600)

    def test_valid_certificate_is_reused(self):
        self.assert_generated()
        before = (self.cert.read_bytes(), self.key.read_bytes())
        os.chmod(self.key, 0o644)
        self.assert_generated()
        self.assertEqual(before, (self.cert.read_bytes(), self.key.read_bytes()))
        self.assertEqual(self.key.stat().st_mode & 0o777, 0o600)

    def test_external_hostname_is_in_certificate(self):
        self.assert_generated()
        self.openssl("verify", "-CAfile", self.cert, "-verify_hostname",
                     "pz-target-web.com", self.cert)

    def test_external_hostname_change_renews_certificate(self):
        self.assert_generated()
        before = self.cert.read_bytes()
        environment = dict(os.environ, TARGET_WEB_EXTERNAL_HOST="other-external.test")
        result = subprocess.run(["bash", str(GENERATOR), "192.168.5.10", str(self.directory)],
                                env=environment, capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertNotEqual(before, self.cert.read_bytes())
        self.openssl("verify", "-CAfile", self.cert, "-verify_hostname", "other-external.test", self.cert)

    def test_changed_ip_replaces_certificate(self):
        self.assert_generated()
        before = self.cert.read_bytes()
        self.assert_generated("10.20.30.40")
        self.assertNotEqual(before, self.cert.read_bytes())
        self.openssl("verify", "-CAfile", self.cert, "-verify_ip",
                     "10.20.30.40", self.cert)

    def test_force_renews_certificate(self):
        self.assert_generated()
        before = self.cert.read_bytes()
        self.assert_generated(force=True)
        self.assertNotEqual(before, self.cert.read_bytes())

    def test_mismatched_key_is_repaired(self):
        self.assert_generated()
        self.openssl("genpkey", "-algorithm", "EC", "-pkeyopt",
                     "ec_paramgen_curve:prime256v1", "-out", self.key)
        self.assert_generated()
        context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
        context.load_cert_chain(self.cert, self.key)

    def test_near_expiry_certificate_is_renewed(self):
        self.directory.mkdir()
        self.openssl(
            "req", "-x509", "-newkey", "ec", "-pkeyopt",
            "ec_paramgen_curve:prime256v1", "-nodes", "-days", "1",
            "-subj", "/CN=192.168.5.10", "-addext",
            "subjectAltName=IP:192.168.5.10,DNS:localhost,IP:127.0.0.1",
            "-keyout", self.key, "-out", self.cert,
        )
        before = self.cert.read_bytes()
        self.assert_generated()
        self.assertNotEqual(before, self.cert.read_bytes())
        self.openssl("x509", "-in", self.cert, "-checkend", 30 * 86400, "-noout")

    def test_corrupt_certificate_is_repaired(self):
        self.assert_generated()
        self.cert.write_text("invalid certificate\n")
        self.assert_generated()
        self.openssl("verify", "-CAfile", self.cert, self.cert)

    def test_invalid_ip_does_not_overwrite_existing_files(self):
        self.assert_generated()
        before = (self.cert.read_bytes(), self.key.read_bytes())
        for host in ("", "localhost", "999.1.1.1", "127.0.0.01", "1.2.3",
                     "192.168.5.10/invalid", "1.2.3.4\nDNS:bad", "::1"):
            with self.subTest(host=host):
                self.assertNotEqual(self.generate(host).returncode, 0)
                self.assertEqual(before, (self.cert.read_bytes(), self.key.read_bytes()))


if __name__ == "__main__":
    unittest.main()
